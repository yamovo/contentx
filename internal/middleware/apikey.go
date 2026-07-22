package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// APIKeyMiddleware authenticates requests using API keys.
func APIKeyMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := extractAPIKey(c)
		if key == "" {
			c.Next()
			return
		}

		keyHash := auth.HashAPIKey(key)

		var apiKey auth.APIKey
		if err := db.Where("key_hash = ?", keyHash).First(&apiKey).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		if apiKey.IsExpired() {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key has expired"})
			c.Abort()
			return
		}

		// Load user.
		var user models.User
		if err := db.Preload("Role").Preload("Role.Permissions").
			Where("id = ?", apiKey.UserID).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		if !user.IsActive() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Account is disabled"})
			c.Abort()
			return
		}

		// Update last used.
		db.Model(&apiKey).Update("last_used", gorm.Expr("CURRENT_TIMESTAMP"))

		c.Set(ContextKeyUser, &user)
		c.Set("api_key", &apiKey)
		c.Next()
	}
}

// RequireScope checks if the API key has a required scope.
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKeyValue, exists := c.Get("api_key")
		if !exists {
			c.Next() // Not an API key request, skip
			return
		}

		apiKey, ok := apiKeyValue.(*auth.APIKey)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid API key type"})
			c.Abort()
			return
		}
		if !apiKey.HasScope(scope) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":          "Insufficient API key scope",
				"required_scope": scope,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// extractAPIKey gets the API key from header or query.
func extractAPIKey(c *gin.Context) string {
	// Check X-API-Key header.
	if key := c.GetHeader("X-API-Key"); key != "" {
		return key
	}
	// Check Authorization: ApiKey xxx.
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "ApiKey ") {
		return auth[7:]
	}
	// Check query parameter.
	return c.Query("api_key")
}
