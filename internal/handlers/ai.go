package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/middleware"
	"github.com/yamovo/contentx/internal/services"
)

// AIHandler handles AI/RAG HTTP requests: semantic search, RAG Q&A, and
// vector index management.
type AIHandler struct {
	svc           *services.RAGService
	audit         services.AuditLogger
	enabled       bool
	allowOutbound bool // when false, blocks endpoints that call external LLM APIs
}

// NewAIHandler creates a new AIHandler.
func NewAIHandler(svc *services.RAGService, enabled, allowOutbound bool) *AIHandler {
	return &AIHandler{
		svc:           svc,
		audit:         services.NoopAuditLogger{},
		enabled:       enabled,
		allowOutbound: allowOutbound,
	}
}

// SetAuditLogger wires the business-level audit logger.
func (h *AIHandler) SetAuditLogger(l services.AuditLogger) {
	if l != nil {
		h.audit = l
	}
}

func (h *AIHandler) guard(c *gin.Context) bool {
	if !h.enabled || h.svc == nil {
		Error(c, http.StatusServiceUnavailable, "AI_DISABLED", "AI features are not enabled")
		return false
	}
	return true
}

// logAudit records an AI operation in the audit trail.
func (h *AIHandler) logAudit(c *gin.Context, action string, tenantID uint, details map[string]any) {
	var uid *uint
	if u := middleware.GetCurrentUser(c); u != nil {
		id := u.ID
		uid = &id
	}
	tid := tenantID
	h.audit.Log(services.AuditEvent{
		UserID:   uid,
		TenantID: &tid,
		Action:   action,
		Entity:   "ai",
		IP:       c.ClientIP(),
		Details:  details,
	})
}

// Search performs a semantic search using vector similarity.
// GET /api/v1/ai/search?q=...&limit=5
//
//	@Summary      Semantic search
//	@Description  Vector similarity search across published content chunks
//	@Tags         AI
//	@Produce      json
//	@Param        q      query  string  true   "Search query"
//	@Param        limit  query  int     false  "Max results"  default(5)
//	@Success      200  {object}  APIResponse
//	@Router       /ai/search [get]
func (h *AIHandler) Search(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	q := c.Query("q")
	if q == "" {
		BadRequest(c, "query parameter 'q' is required")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	if limit <= 0 || limit > 50 {
		limit = 5
	}

	tenantID := getCurrentTenant(c)
	result, err := h.svc.Search(c.Request.Context(), q, tenantID, limit)
	if err != nil {
		Error(c, http.StatusInternalServerError, "AI_SEARCH_FAILED", "Semantic search failed: "+err.Error())
		return
	}

	h.logAudit(c, "ai.search", tenantID, map[string]any{
		"query": q, "results": result.Total,
	})
	Success(c, result)
}

// Ask performs a RAG query: retrieves relevant context and optionally
// synthesises an answer using the configured LLM.
// POST /api/v1/ai/rag/ask  {"query": "...", "top_k": 5}
//
//	@Summary      RAG query
//	@Description  Retrieve relevant content and optionally generate an answer
//	@Tags         AI
//	@Accept       json
//	@Produce      json
//	@Param        body  body  object  true  "RAG query"
//	@Success      200  {object}  APIResponse
//	@Router       /ai/rag/ask [post]
func (h *AIHandler) Ask(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	if !h.allowOutbound {
		Error(c, http.StatusForbidden, "AI_OUTBOUND_DISABLED", "Outbound AI API calls are disabled")
		return
	}
	var req struct {
		Query string `json:"query" binding:"required"`
		TopK  int    `json:"top_k"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request body: "+err.Error())
		return
	}
	if req.TopK <= 0 || req.TopK > 20 {
		req.TopK = 5
	}

	tenantID := getCurrentTenant(c)
	answer, err := h.svc.Ask(c.Request.Context(), req.Query, tenantID, req.TopK)
	if err != nil {
		Error(c, http.StatusInternalServerError, "RAG_FAILED", "RAG query failed: "+err.Error())
		return
	}

	h.logAudit(c, "ai.rag_ask", tenantID, map[string]any{
		"query": req.Query, "context_chunks": len(answer.Context),
	})
	Success(c, answer)
}

// Reindex rebuilds the vector index for the current tenant.
// POST /api/v1/ai/reindex
//
//	@Summary      Reindex vector embeddings
//	@Description  Rebuild all vector embeddings from published articles for the current tenant
//	@Tags         AI
//	@Produce      json
//	@Success      200  {object}  APIResponse
//	@Router       /ai/reindex [post]
func (h *AIHandler) Reindex(c *gin.Context) {
	if !h.guard(c) {
		return
	}

	tenantID := getCurrentTenant(c)
	h.logAudit(c, "ai.reindex", tenantID, nil)

	// Use context.Background() — not the request context — because the
	// goroutine outlives the HTTP response.
	go func() {
		n, err := h.svc.ReindexTenant(context.Background(), tenantID)
		if err != nil {
			return
		}
		_ = n
	}()

	Success(c, gin.H{"message": "reindex started in background", "tenant_id": tenantID})
}

// Status returns the current AI/RAG service status.
// GET /api/v1/ai/status
//
//	@Summary      AI service status
//	@Description  Returns embedding provider, vector store, and index statistics
//	@Tags         AI
//	@Produce      json
//	@Success      200  {object}  APIResponse
//	@Router       /ai/status [get]
func (h *AIHandler) Status(c *gin.Context) {
	if !h.enabled || h.svc == nil {
		Success(c, gin.H{"enabled": false})
		return
	}
	llmName := "none"
	if l := h.svc.LLM(); l != nil {
		llmName = l.Name()
	}
	Success(c, gin.H{
		"enabled":            true,
		"embedding_provider": h.svc.Embedder().Name(),
		"llm_provider":       llmName,
		"vector_store":       h.svc.Store().Name(),
		"vector_count":       h.svc.Store().Count(getCurrentTenant(c)),
		"allow_outbound":     h.allowOutbound,
	})
}
