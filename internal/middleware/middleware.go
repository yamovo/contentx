package middleware

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/config"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// LoggerMiddleware logs HTTP requests.
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		userAgent := c.Request.UserAgent()

		if query != "" {
			path = path + "?" + query
		}

		gin.DefaultWriter.Write([]byte(fmt.Sprintf("[API] %s | %3d | %13v | %15s | %-7s %s | %s\n",
			time.Now().Format("2006-01-02 15:04:05"),
			status,
			latency,
			clientIP,
			method,
			path,
			userAgent,
		)))
	}
}

// CORSMiddleware handles Cross-Origin Resource Sharing.
func CORSMiddleware(cfg config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		// Check if origin is allowed.
		allowed := false
		for _, o := range cfg.AllowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
			// Support wildcard subdomain matching.
			if strings.HasPrefix(o, "*.") {
				suffix := o[1:] // e.g., ".example.com"
				if strings.HasSuffix(origin, suffix) {
					allowed = true
					break
				}
			}
		}

		if !allowed {
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
		c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))

		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight.
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// SecurityHeaders adds common security headers.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		if gin.Mode() == gin.ReleaseMode {
			c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		} else {
			c.Header("Strict-Transport-Security", "max-age=0")
		}
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; font-src 'self' data:; connect-src 'self' ws: wss:")
		c.Next()
	}
}

// RequestID adds a unique request ID to each request.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// ActivityLogger logs user actions to the activity_log table.
func ActivityLogger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Only log mutations.
		method := c.Request.Method
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return
		}

		// Don't log if there were errors.
		if len(c.Errors) > 0 {
			return
		}

		var userID *uint
		if user := GetCurrentUser(c); user != nil {
			uid := user.ID
			userID = &uid
		}

		// Use entity set by handler, or fall back to URL parsing.
		entity := GetActivityEntity(c)
		if entity == "" {
			entity = extractEntity(c.FullPath())
		}

		log := models.ActivityLog{
			UserID:    userID,
			Action:    methodToAction(method),
			Entity:    entity,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		}

		// Non-blocking write.
		go func() {
			if err := db.Create(&log).Error; err != nil {
				println("[ACTIVITY] failed to log:", err)
			}
		}()
	}
}

// RecoverMiddleware handles panics gracefully.
func RecoverMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Sprintf("Panic recovered: %v", r)
				fmt.Println("[RECOVER]", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

// ContentTypeJSON sets JSON content type for API responses.
func ContentTypeJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Next()
	}
}

// ETag adds ETag support for caching.
func ETag() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer = &etagWriter{ResponseWriter: c.Writer}
		c.Next()
	}
}

// Helper functions.

func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

func methodToAction(method string) string {
	switch method {
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return strings.ToLower(method)
	}
}

// SetActivityEntity sets the entity name for activity logging on this request.
func SetActivityEntity(c *gin.Context, entity string) {
	c.Set("activity_entity", entity)
}

// GetActivityEntity returns the entity name set by the handler, or "" if not set.
func GetActivityEntity(c *gin.Context) string {
	if v, ok := c.Get("activity_entity"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func extractEntity(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 {
		return parts[1] // e.g., /api/v1/articles -> articles
	}
	return "unknown"
}

// etagWriter wraps gin.ResponseWriter to support ETag.
type etagWriter struct {
	gin.ResponseWriter
	body []byte
}

func (w *etagWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

func (w *etagWriter) WriteHeader(code int) {
	if len(w.body) > 0 {
		hash := sha256.Sum256(w.body)
		etag := fmt.Sprintf(`"%x"`, hash[:8])
		w.Header().Set("ETag", etag)
	}
	w.ResponseWriter.WriteHeader(code)
}
