package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/middleware"
	"github.com/vortexcms/go-cms/internal/services"
)

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	svc *services.AuthService
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(svc *services.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Login authenticates a user and returns tokens.
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req services.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokenPair, user, err := h.svc.Login(req.Username, req.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": tokenPair,
		"user": user,
	})
}

// Register creates a new user account.
// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req services.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokenPair, user, err := h.svc.Register(req, c.ClientIP())
	if err != nil {
		handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data": tokenPair,
		"user": user,
	})
}

// RefreshToken refreshes an access token.
// POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req services.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokenPair, err := h.svc.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tokenPair})
}

// Logout invalidates the current token.
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims != nil {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			if err := h.svc.Logout(authHeader[7:], claims.UserID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// Me returns the current authenticated user.
// GET /api/v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	safeUser, permissions, err := h.svc.Me(user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        safeUser,
		"permissions": permissions,
	})
}

// UpdateProfile updates the current user's profile.
// PUT /api/v1/auth/profile
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	var req struct {
		DisplayName *string `json:"display_name"`
		Bio         *string `json:"bio"`
		Website     *string `json:"website"`
		Avatar      *string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fields := map[string]interface{}{}
	if req.DisplayName != nil {
		fields["display_name"] = *req.DisplayName
	}
	if req.Bio != nil {
		fields["bio"] = *req.Bio
	}
	if req.Website != nil {
		fields["website"] = *req.Website
	}
	if req.Avatar != nil {
		fields["avatar"] = *req.Avatar
	}

	updated, err := h.svc.UpdateProfile(user.ID, fields)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": services.SanitizeUser(updated)})
}

// ChangePassword changes the current user's password.
// PUT /api/v1/auth/password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	var req services.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.ChangePassword(user.ID, req.OldPassword, req.NewPassword); err != nil {
		handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}
