package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

const (
	// ContextKeyUser is the gin context key for the authenticated user.
	ContextKeyUser = "currentUser"
	// ContextKeyClaims is the gin context key for JWT claims.
	ContextKeyClaims = "claims"
)

// AuthMiddleware validates JWT tokens, checks revocation, and injects user into context.
func AuthMiddleware(jwtMgr *auth.JWTManager, db *gorm.DB, store auth.TokenStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
			c.Abort()
			return
		}

		// Check if token has been revoked.
		if store != nil && store.IsRevoked(token) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token has been revoked"})
			c.Abort()
			return
		}

		claims, err := jwtMgr.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Load user from database.
		var user models.User
		if err := db.Preload("Role").Preload("Role.Permissions").
			Where("id = ?", claims.UserID).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		if !user.IsActive() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Account is disabled"})
			c.Abort()
			return
		}

		c.Set(ContextKeyUser, &user)
		c.Set(ContextKeyClaims, claims)
		c.Next()
	}
}

// OptionalAuthMiddleware tries to authenticate but doesn't block.
func OptionalAuthMiddleware(jwtMgr *auth.JWTManager, db *gorm.DB, store auth.TokenStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.Next()
			return
		}

		// Skip revoked tokens silently.
		if store != nil && store.IsRevoked(token) {
			c.Next()
			return
		}

		claims, err := jwtMgr.ValidateToken(token)
		if err != nil {
			c.Next()
			return
		}

		var user models.User
		if err := db.Preload("Role").Preload("Role.Permissions").
			Where("id = ?", claims.UserID).First(&user).Error; err != nil {
			c.Next()
			return
		}

		if user.IsActive() {
			c.Set(ContextKeyUser, &user)
			c.Set(ContextKeyClaims, claims)
		}
		c.Next()
	}
}

// RequirePermission checks if the authenticated user has a specific permission.
func RequirePermission(permissionSlug string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Get(ContextKeyUser)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		u, ok := user.(*models.User)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid user type"})
			c.Abort()
			return
		}
		if !hasPermission(u, permissionSlug) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":      "Insufficient permissions",
				"required":   permissionSlug,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireRole checks if the user has one of the specified roles.
func RequireRole(roleSlugs ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Get(ContextKeyUser)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		u, ok := user.(*models.User)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
			c.Abort()
			return
		}
		for _, slug := range roleSlugs {
			if u.Role.Slug == slug {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient role"})
		c.Abort()
	}
}

// RequireAdmin is a shortcut for admin-only routes.
func RequireAdmin() gin.HandlerFunc {
	return RequireRole("admin")
}

// RequireEditor checks for editor or admin role.
func RequireEditor() gin.HandlerFunc {
	return RequireRole("admin", "editor")
}

// hasPermission checks if a user has a specific permission.
func hasPermission(user *models.User, slug string) bool {
	// Admins have all permissions.
	if user.Role.Slug == "admin" {
		return true
	}
	for _, perm := range user.Role.Permissions {
		if perm.Slug == slug {
			return true
		}
	}
	return false
}

// extractToken gets the JWT token from Authorization header.
func extractToken(c *gin.Context) string {
	bearer := c.GetHeader("Authorization")
	if len(bearer) > 7 && strings.HasPrefix(bearer, "Bearer ") {
		return bearer[7:]
	}
	// Also check query parameter (for WebSocket, etc.)
	return c.Query("token")
}

// GetCurrentUser retrieves the authenticated user from context.
func GetCurrentUser(c *gin.Context) *models.User {
	user, exists := c.Get(ContextKeyUser)
	if !exists {
		return nil
	}
	u, ok := user.(*models.User)
	if !ok {
		return nil
	}
	return u
}

// GetClaims retrieves the JWT claims from context.
func GetClaims(c *gin.Context) *auth.Claims {
	claims, exists := c.Get(ContextKeyClaims)
	if !exists {
		return nil
	}
	cl, ok := claims.(*auth.Claims)
	if !ok {
		return nil
	}
	return cl
}
