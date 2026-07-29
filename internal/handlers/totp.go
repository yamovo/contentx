package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/services"
)

// TOTPHandler manages the current user's TOTP two-factor authentication.
// All routes are mounted under the JWT-protected /auth/totp group.
type TOTPHandler struct {
	svc *services.TOTPService
}

// NewTOTPHandler creates a new TOTP handler.
func NewTOTPHandler(svc *services.TOTPService) *TOTPHandler {
	return &TOTPHandler{svc: svc}
}

// Status reports whether TOTP is enabled for the current user.
// GET /api/v1/auth/totp/status
//
//	@Summary      TOTP status
//	@Description  Returns whether two-factor authentication is enabled for the current user
//	@Tags         Auth
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object{enabled=bool}}
//	@Failure      401  {object}  APIResponse
//	@Router       /auth/totp/status [get]
func (h *TOTPHandler) Status(c *gin.Context) {
	user := getCurrentUser(c)
	if user == nil {
		return
	}
	enabled, err := h.svc.Status(user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, gin.H{"enabled": enabled})
}

// Setup generates a pending TOTP secret for the current user.
// POST /api/v1/auth/totp/setup
//
//	@Summary      Begin TOTP setup
//	@Description  Generates a TOTP secret and otpauth URI. TOTP stays disabled until confirmed via /auth/totp/enable.
//	@Tags         Auth
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=services.TOTPSetupResponse}
//	@Failure      401  {object}  APIResponse
//	@Failure      409  {object}  APIResponse
//	@Router       /auth/totp/setup [post]
func (h *TOTPHandler) Setup(c *gin.Context) {
	user := getCurrentUser(c)
	if user == nil {
		return
	}
	resp, err := h.svc.Setup(user.ID, user.Username)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, resp)
}

// Enable confirms the pending secret and activates TOTP.
// POST /api/v1/auth/totp/enable
//
//	@Summary      Enable TOTP
//	@Description  Verifies the code from the authenticator app, enables TOTP, and returns one-time backup codes.
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Param        body  body      object{code=string}  true  "6-digit code from the authenticator app"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse{data=object{backup_codes=[]string}}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Router       /auth/totp/enable [post]
func (h *TOTPHandler) Enable(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}
	user := getCurrentUser(c)
	if user == nil {
		return
	}
	codes, err := h.svc.Enable(user.ID, req.Code)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, gin.H{"backup_codes": codes})
}

// Disable turns off TOTP after re-verifying the password.
// POST /api/v1/auth/totp/disable
//
//	@Summary      Disable TOTP
//	@Description  Disables two-factor authentication. Requires the account password for confirmation.
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Param        body  body      object{password=string}  true  "Account password"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Router       /auth/totp/disable [post]
func (h *TOTPHandler) Disable(c *gin.Context) {
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}
	user := getCurrentUser(c)
	if user == nil {
		return
	}
	if err := h.svc.Disable(user.ID, req.Password); err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, gin.H{"message": "Two-factor authentication disabled"})
}
