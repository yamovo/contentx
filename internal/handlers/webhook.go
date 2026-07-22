package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/services"
)

// WebhookHandler manages webhooks.
type WebhookHandler struct {
	svc *services.WebhookService
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(svc *services.WebhookService) *WebhookHandler {
	return &WebhookHandler{svc: svc}
}

// List returns all webhooks.
// GET /api/v1/webhooks
//
//	@Summary      List webhooks
//	@Description  Returns all configured webhooks
//	@Tags         Webhooks
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=[]models.Webhook}
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /webhooks [get]
func (h *WebhookHandler) List(c *gin.Context) {
	webhooks, err := h.svc.List()
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, webhooks)
}

// Create creates a new webhook.
// POST /api/v1/webhooks
//
//	@Summary      Create webhook
//	@Description  Create a new webhook endpoint
//	@Tags         Webhooks
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.CreateWebhookRequest  true  "Webhook config"
//	@Security     BearerAuth
//	@Success      201   {object}  APIResponse{data=models.Webhook}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /webhooks [post]
func (h *WebhookHandler) Create(c *gin.Context) {
	var req services.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	wh, err := h.svc.Create(req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, wh)
}

// Delete deletes a webhook.
// DELETE /api/v1/webhooks/:id
//
//	@Summary      Delete webhook
//	@Description  Delete a webhook by ID
//	@Tags         Webhooks
//	@Produce      json
//	@Param        id  path      int  true  "Webhook ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /webhooks/{id} [delete]
func (h *WebhookHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid webhook ID")
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Webhook deleted"})
}

// Logs returns delivery logs for a webhook.
// GET /api/v1/webhooks/:id/logs
//
//	@Summary      Webhook logs
//	@Description  Returns delivery logs for a webhook
//	@Tags         Webhooks
//	@Produce      json
//	@Param        id     path      int  false  "Webhook ID"
//	@Param        limit  query     int  false  "Max logs"  default(50)
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=[]models.WebhookLog}
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /webhooks/{id}/logs [get]
func (h *WebhookHandler) Logs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid webhook ID")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	logs, err := h.svc.GetLogs(uint(id), limit)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, logs)
}
