package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/services"
)

// TokenHandler manages API tokens.
type TokenHandler struct {
	svc *services.TokenService
}

// NewTokenHandler creates a new token handler.
func NewTokenHandler(svc *services.TokenService) *TokenHandler {
	return &TokenHandler{svc: svc}
}

// List returns all API tokens.
// GET /api/v1/system/tokens
//
//	@Summary      List API tokens
//	@Description  Returns all API tokens (admin only)
//	@Tags         System
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=[]models.APIToken}
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /system/tokens [get]
func (h *TokenHandler) List(c *gin.Context) {
	tokens, err := h.svc.List()
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, tokens)
}

// Create creates a new API token.
// POST /api/v1/system/tokens
//
//	@Summary      Create API token
//	@Description  Creates a new API token with specified permissions
//	@Tags         System
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.CreateTokenRequest  true  "Token config"
//	@Security     BearerAuth
//	@Success      201   {object}  APIResponse{data=services.TokenCreatedResponse}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /system/tokens [post]
func (h *TokenHandler) Create(c *gin.Context) {
	var req services.CreateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	user := getCurrentUser(c)
	if user == nil {
		return
	}

	result, err := h.svc.Create(req, user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, result)
}

// Delete deletes an API token.
// DELETE /api/v1/system/tokens/:id
//
//	@Summary      Delete API token
//	@Description  Deletes an API token by ID
//	@Tags         System
//	@Produce      json
//	@Param        id   path      int  true  "Token ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /system/tokens/{id} [delete]
func (h *TokenHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid token ID")
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Token deleted"})
}
