package handlers

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/middleware"
	"github.com/yamovo/contentx/internal/services"
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
//
//	@Summary      User login
//	@Description  Authenticate with username/email and password, returns JWT token pair
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.LoginRequest  true  "Login credentials"
//	@Success      200   {object}  APIResponse{data=object}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req services.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	tokenPair, user, err := h.svc.Login(req.Username, req.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{
		"token": tokenPair,
		"user":  user,
	})
}

// Register creates a new user account.
// POST /api/v1/auth/register
//
//	@Summary      User registration
//	@Description  Create a new user account and return JWT token pair
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.RegisterRequest  true  "Registration data"
//	@Success      201   {object}  APIResponse{data=object}
//	@Failure      400   {object}  APIResponse
//	@Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req services.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	tokenPair, user, err := h.svc.Register(req, c.ClientIP())
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, gin.H{
		"token": tokenPair,
		"user":  user,
	})
}

// RefreshToken refreshes an access token.
// POST /api/v1/auth/refresh
//
//	@Summary      Refresh access token
//	@Description  Exchange a refresh token for a new token pair
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.RefreshRequest  true  "Refresh token"
//	@Success      200   {object}  APIResponse{data=object}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Router       /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req services.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	tokenPair, err := h.svc.RefreshToken(req.RefreshToken)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, tokenPair)
}

// Logout invalidates the current token.
// POST /api/v1/auth/logout
//
//	@Summary      Logout
//	@Description  Invalidate the current access token
//	@Tags         Auth
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims != nil {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			if err := h.svc.Logout(authHeader[7:], claims.UserID); err != nil {
				handleServiceError(c, err)
				return
			}
		}
	}

	Success(c, gin.H{"message": "Logged out successfully"})
}

// Me returns the current authenticated user.
// GET /api/v1/auth/me
//
//	@Summary      Get current user
//	@Description  Returns the authenticated user profile and permissions
//	@Tags         Auth
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object{user=services.SafeUser,permissions=[]string}}
//	@Failure      401  {object}  APIResponse
//	@Router       /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		Unauthorized(c, "Not authenticated")
		return
	}

	safeUser, permissions, err := h.svc.Me(user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{
		"user":        safeUser,
		"permissions": permissions,
	})
}

// UpdateProfile updates the current user's profile.
// PUT /api/v1/auth/profile
//
//	@Summary      Update profile
//	@Description  Update display_name, bio, website, avatar of current user
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Param        body  body      object  true  "Fields to update"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse{data=services.SafeUser}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Router       /auth/profile [put]
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		Unauthorized(c, "Not authenticated")
		return
	}

	var req struct {
		DisplayName *string `json:"display_name"`
		Bio         *string `json:"bio"`
		Website     *string `json:"website"`
		Avatar      *string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
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

	Success(c, services.SanitizeUser(updated))
}

// ChangePassword changes the current user's password.
// PUT /api/v1/auth/password
//
//	@Summary      Change password
//	@Description  Change the current user's password
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.ChangePasswordRequest  true  "Old and new password"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Router       /auth/password [put]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		Unauthorized(c, "Not authenticated")
		return
	}

	var req services.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.ChangePassword(user.ID, req.OldPassword, req.NewPassword); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Password changed successfully"})
}
