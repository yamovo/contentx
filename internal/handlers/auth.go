package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/auth"
	"github.com/vortexcms/go-cms/internal/middleware"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	db       *gorm.DB
	jwtMgr   *auth.JWTManager
	blacklist *auth.Blacklist
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(db *gorm.DB, jwtMgr *auth.JWTManager, blacklist *auth.Blacklist) *AuthHandler {
	return &AuthHandler{db: db, jwtMgr: jwtMgr, blacklist: blacklist}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username    string `json:"username" binding:"required,min=3,max=64"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Login authenticates a user and returns tokens.
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := h.db.Preload("Role").Where("username = ? OR email = ?", req.Username, req.Username).
		First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if !user.IsActive() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is disabled"})
		return
	}

	if err := auth.CheckPassword(user.Password, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate tokens.
	tokenPair, err := h.jwtMgr.GenerateTokenPair(
		user.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Record login.
	user.RecordLogin(c.ClientIP())
	h.db.Model(&user).Updates(map[string]interface{}{
		"last_login_at": user.LastLoginAt,
		"last_login_ip": user.LastLoginIP,
		"login_count":   user.LoginCount,
	})

	// Log activity.
	h.db.Create(&models.ActivityLog{
		UserID:    &user.ID,
		Action:    "login",
		Entity:    "user",
		EntityID:  user.ID,
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})

	c.JSON(http.StatusOK, gin.H{
		"data":          tokenPair,
		"user":          sanitizeUser(&user),
	})
}

// Register creates a new user account.
// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if registration is enabled.
	var setting models.SiteSetting
	if h.db.Where("key = ?", "enable_registration").First(&setting).Error == nil {
		if setting.Value == "false" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Registration is currently disabled"})
			return
		}
	}

	// Check uniqueness.
	var count int64
	h.db.Model(&models.User{}).Where("username = ? OR email = ?", req.Username, req.Email).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Username or email already exists"})
		return
	}

	// Hash password.
	hashedPw, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find default role.
	var defaultRole models.Role
	if err := h.db.Where("is_default = ?", true).First(&defaultRole).Error; err != nil {
		// Fallback to subscriber.
		h.db.Where("slug = ?", "subscriber").First(&defaultRole)
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}

	user := models.User{
		Username:    req.Username,
		Email:       req.Email,
		Password:    hashedPw,
		DisplayName: displayName,
		RoleID:      defaultRole.ID,
		Status:      models.UserStatusActive,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Generate tokens immediately.
	h.db.Preload("Role").First(&user, user.ID)
	tokenPair, err := h.jwtMgr.GenerateTokenPair(
		user.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User created but token generation failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data": tokenPair,
		"user": sanitizeUser(&user),
	})
}

// RefreshToken refreshes an access token.
// POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokenPair, err := h.jwtMgr.RefreshAccessToken(req.RefreshToken)
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
		h.blacklist.Revoke(
			c.GetHeader("Authorization")[7:],
			claims.ExpiresAt.Time,
		)
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

	// Reload with full associations.
	h.db.Preload("Role").Preload("Role.Permissions").First(&user, user.ID)

	c.JSON(http.StatusOK, gin.H{
		"data": sanitizeUser(user),
		"permissions": func() []string {
			slugs := make([]string, len(user.Role.Permissions))
			for i, p := range user.Role.Permissions {
				slugs[i] = p.Slug
			}
			return slugs
		}(),
	})
}

// UpdateProfile updates the current user's profile.
// PUT /api/v1/auth/profile
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

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

	updates := map[string]interface{}{}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Bio != nil {
		updates["bio"] = *req.Bio
	}
	if req.Website != nil {
		updates["website"] = *req.Website
	}
	if req.Avatar != nil {
		updates["avatar"] = *req.Avatar
	}

	if len(updates) > 0 {
		h.db.Model(user).Updates(updates)
	}

	h.db.Preload("Role").First(&user, user.ID)
	c.JSON(http.StatusOK, gin.H{"data": sanitizeUser(user)})
}

// ChangePassword changes the current user's password.
// PUT /api/v1/auth/password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := auth.CheckPassword(user.Password, req.OldPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Current password is incorrect"})
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Model(user).Update("password", newHash)
	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// Helper to sanitize user output (remove sensitive fields).
type SafeUser struct {
	ID          uint                 `json:"id"`
	Username    string               `json:"username"`
	Email       string               `json:"email"`
	DisplayName string               `json:"display_name"`
	Avatar      string               `json:"avatar"`
	Bio         string               `json:"bio"`
	Website     string               `json:"website"`
	Role        models.Role          `json:"role"`
	Status      models.UserStatus    `json:"status"`
	LastLoginAt *models.User         `json:"-"`
	LoginCount  int                  `json:"login_count"`
	Preferences models.UserPreferences `json:"preferences"`
	CreatedAt   models.User          `json:"-"`
}

func sanitizeUser(u *models.User) gin.H {
	return gin.H{
		"id":           u.ID,
		"username":     u.Username,
		"email":        u.Email,
		"display_name": u.DisplayName,
		"avatar":       u.AvatarURL(),
		"bio":          u.Bio,
		"website":      u.Website,
		"role":         u.Role,
		"status":       u.Status,
		"login_count":  u.LoginCount,
		"preferences":  u.Preferences,
		"created_at":   u.CreatedAt,
	}
}
