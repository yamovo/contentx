package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
)

// PublicContentHandler serves the RFC-002 public content delivery routes:
// read-only, published-only, allowlist-gated, fixed to the default tenant.
// All miss conditions (feature disabled, type not allowlisted, unknown type,
// unknown entry, unpublished entry) return the same 404 so callers cannot
// probe internal state.
type PublicContentHandler struct {
	svc         *services.PublicContentService
	allowedUIDs map[string]bool
}

// NewPublicContentHandler builds the handler with the UID allowlist.
func NewPublicContentHandler(svc *services.PublicContentService, allowedUIDs []string) *PublicContentHandler {
	allowed := make(map[string]bool, len(allowedUIDs))
	for _, uid := range allowedUIDs {
		allowed[uid] = true
	}
	return &PublicContentHandler{svc: svc, allowedUIDs: allowed}
}

// publicContentEntry is the public DTO (RFC-002 §3). It intentionally omits
// tenant ID, content type internal ID, creator/updater IDs, and management
// status fields; until field-level visibility exists, the allowlist entry for
// a type means all of its Data fields are public.
type publicContentEntry struct {
	DocumentID  string         `json:"document_id"`
	Data        models.JSONMap `json:"data"`
	Locale      string         `json:"locale"`
	PublishedAt *time.Time     `json:"published_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func toPublicContentEntry(e *models.ContentEntry) publicContentEntry {
	return publicContentEntry{
		DocumentID:  e.DocumentID,
		Data:        e.Data,
		Locale:      e.Locale,
		PublishedAt: e.PublishedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

// noStore sets Cache-Control: no-store. Unpublishing must take effect
// immediately; public delivery does not rely on caller-side cache guessing.
func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
}

func notFound(c *gin.Context) {
	noStore(c)
	Error(c, http.StatusNotFound, "NOT_FOUND", "Resource not found")
}

// List godoc
//
//	@Summary		List published content entries (public)
//	@Description	Public read-only list for an allowlisted content type. Published entries of the default tenant only; pagination is capped; unknown or non-allowlisted types return 404.
//	@Tags			Public Content
//	@Produce		json
//	@Param			uid			path		string	true	"Content type UID (allowlisted)"
//	@Param			page		query		int		false	"Page number (>= 1)"	default(1)
//	@Param			page_size	query		int		false	"Page size (1-50)"		default(20)
//	@Param			locale		query		string	false	"Filter by BCP-47 locale"
//	@Success		200			{object}	map[string]interface{}
//	@Header			200			{string}	Cache-Control	"no-store"
//	@Failure		400			{object}	map[string]interface{}	"Illegal pagination"
//	@Failure		404			{object}	map[string]interface{}	"Type not allowlisted, unknown, or has no published entries"
//	@Router			/public/content/{uid} [get]
func (h *PublicContentHandler) List(c *gin.Context) {
	uid := c.Param("uid")
	if !h.allowedUIDs[uid] {
		notFound(c)
		return
	}

	page := 1
	if s := c.Query("page"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 1 {
			Error(c, http.StatusBadRequest, "BAD_REQUEST", "page must be a positive integer")
			return
		}
		page = v
	}
	pageSize := 20
	if s := c.Query("page_size"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 1 || v > services.PublicContentPageSizeMax {
			Error(c, http.StatusBadRequest, "BAD_REQUEST", "page_size must be between 1 and 50")
			return
		}
		pageSize = v
	}

	entries, total, serr := h.svc.List(uid, services.PublicContentListParams{
		Page:     page,
		PageSize: pageSize,
		Locale:   c.Query("locale"),
	})
	if serr != nil {
		handleServiceError(c, serr)
		return
	}

	items := make([]publicContentEntry, 0, len(entries))
	for i := range entries {
		items = append(items, toPublicContentEntry(&entries[i]))
	}

	noStore(c)
	paginate := paginateFrom(page, pageSize, total)
	Success(c, listResponse(items, paginate))
}

// Get godoc
//
//	@Summary		Get a published content entry (public)
//	@Description	Public read-only fetch of one published entry by its document ID. Published entries of the default tenant only; unpublished or unknown documents return 404.
//	@Tags			Public Content
//	@Produce		json
//	@Param			uid			path	string	true	"Content type UID (allowlisted)"
//	@Param			documentId	path	string	true	"Entry document ID (UUID)"
//	@Success		200			{object}	publicContentEntry
//	@Header			200			{string}	Cache-Control	"no-store"
//	@Failure		404			{object}	map[string]interface{}	"Type not allowlisted, unknown, unpublished, or missing entry"
//	@Router			/public/content/{uid}/{documentId} [get]
func (h *PublicContentHandler) Get(c *gin.Context) {
	uid := c.Param("uid")
	if !h.allowedUIDs[uid] {
		notFound(c)
		return
	}

	entry, err := h.svc.Get(uid, c.Param("documentId"))
	if err != nil {
		notFound(c)
		return
	}

	noStore(c)
	Success(c, toPublicContentEntry(entry))
}
