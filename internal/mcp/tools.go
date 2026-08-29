package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/permissions"
	"github.com/yamovo/contentx/internal/services"
)

// registerTools attaches all read-only MCP tools to the server. It is
// transport-agnostic and reused by any transport (stdio today, HTTP later).
func registerTools(s *mcpsdk.Server, deps Deps) {
	ts := &toolset{deps: deps}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "search_content",
		Description: "Full-text search across published ContentX articles and pages. Returns ranked hits with title, excerpt, slug and absolute URL.",
	}, ts.searchContent)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_articles",
		Description: "List published articles with pagination and optional category ID, tag slug and sort order.",
	}, ts.listArticles)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_article",
		Description: "Fetch a single published article by numeric ID, including its full content body, author, category and tags.",
	}, ts.getArticle)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_content_types",
		Description: "List the custom content types (collections) defined in ContentX with their field schema. Use this to discover the content model before querying.",
	}, ts.listContentTypes)

	// RAG tools are exposed only when a RAG service is configured.
	if deps.RAG != nil {
		mcpsdk.AddTool(s, &mcpsdk.Tool{
			Name:        "rag_search",
			Description: "Semantic search across ContentX content using vector similarity. Unlike full-text search, this understands meaning and context. Returns ranked content chunks with similarity scores.",
		}, ts.ragSearch)

		mcpsdk.AddTool(s, &mcpsdk.Tool{
			Name:        "rag_ask",
			Description: "Ask a question and get an answer based on ContentX content using RAG (Retrieval-Augmented Generation). Retrieves relevant content chunks and optionally synthesises an answer. Returns both the answer and the supporting context.",
		}, ts.ragAsk)
	}

	// Write tools are exposed only when an Authorizer is configured (HTTP mode).
	if deps.Authorizer != nil {
		registerWriteTools(s, ts)
	}
}

// toolset carries the shared dependencies for the tool handlers.
type toolset struct {
	deps Deps
}

// requestHeader extracts the transport-provided HTTP headers from an MCP tool
// request. In-memory and stdio requests legitimately have no headers.
func requestHeader(req *mcpsdk.CallToolRequest) http.Header {
	if req == nil || req.Extra == nil {
		return nil
	}
	return req.Extra.Header
}

// verifiedHTTPPrincipal re-resolves the current request token through the
// Authorizer and binds it to the principal captured when this MCP session was
// authenticated. This both applies permission revocations immediately and
// prevents changing identities or tenants inside an existing HTTP session.
// A nil Authorizer identifies local stdio mode, which remains read-only and
// backward compatible without an HTTP principal.
func (t *toolset) verifiedHTTPPrincipal(ctx context.Context, header http.Header) (*Principal, error) {
	if t.deps.Authorizer == nil {
		return nil, nil
	}

	sessionPrincipal, ok := PrincipalFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("verified MCP principal is required")
	}
	if header == nil {
		return nil, fmt.Errorf("verified MCP request headers are required")
	}
	current, err := t.deps.Authorizer.Resolve(header)
	if err != nil || current == nil || current.UserID == 0 || current.TenantID == 0 {
		return nil, fmt.Errorf("invalid MCP principal")
	}
	if current.UserID != sessionPrincipal.UserID || current.TenantID != sessionPrincipal.TenantID {
		return nil, fmt.Errorf("MCP principal changed within the session")
	}

	return &Principal{
		UserID:      current.UserID,
		TenantID:    current.TenantID,
		Permissions: append([]string(nil), current.Permissions...),
	}, nil
}

// requirePermission enforces an effective permission only in authenticated
// HTTP mode. Stdio has no Authorizer and keeps its existing local read-only
// behavior. HTTP always fails closed on a missing principal/header.
func (t *toolset) requirePermission(ctx context.Context, header http.Header, want string) error {
	principal, err := t.verifiedHTTPPrincipal(ctx, header)
	if err != nil {
		return err
	}
	if principal == nil {
		return nil
	}
	if !permissions.Grants(principal.Permissions, want) {
		return fmt.Errorf("token lacks permission: %s", want)
	}
	return nil
}

// auditRAG records a successful RAG tool call for HTTP sessions, mirroring the
// REST AI audit actions. Stdio sessions have no principal and stay silent, and
// a nil Audit logger keeps deployments that opted out of auditing unaffected.
func (t *toolset) auditRAG(ctx context.Context, action, query string, topK, results int) {
	if t.deps.Audit == nil {
		return
	}
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return // stdio mode: local trusted use, nothing to attribute
	}
	userID := principal.UserID
	tenantID := principal.TenantID
	t.deps.Audit.Log(services.AuditEvent{
		UserID:   &userID,
		TenantID: &tenantID,
		Action:   action,
		Entity:   "article",
		Details: map[string]any{
			"query":   query,
			"top_k":   topK,
			"results": results,
		},
	})
}

// status returns the article status filter the read tools should enforce.
// Empty string means "any status" and is only used when drafts are explicitly
// allowed; otherwise callers only ever see published content.
func (t *toolset) status() string {
	if t.deps.IncludeDrafts {
		return ""
	}
	return string(models.StatusPublished)
}

// tenantID returns the tenant for the current tool call. In HTTP mode the
// tenant is resolved from the API token and stored in the request context by
// mcpTokenAuth; in stdio mode it falls back to the default tenant.
func (t *toolset) tenantID(ctx context.Context) uint {
	return TenantFromContext(ctx)
}

// articleURL builds an absolute article URL from the configured base URL.
func (t *toolset) articleURL(slug string) string {
	return strings.TrimRight(t.deps.BaseURL, "/") + "/articles/" + slug
}

// ─── search_content ──────────────────────────────────────────────────────────

type searchInput struct {
	Query    string `json:"query" jsonschema:"the full-text search query"`
	Type     string `json:"type,omitempty" jsonschema:"optional content type filter: article or page"`
	Locale   string `json:"locale,omitempty" jsonschema:"optional BCP-47 locale filter, e.g. en or zh"`
	Page     int    `json:"page,omitempty" jsonschema:"1-based page number (default 1)"`
	PageSize int    `json:"page_size,omitempty" jsonschema:"results per page, 1-100 (default 20)"`
}

type searchHit struct {
	ID      uint    `json:"id"`
	Type    string  `json:"type"`
	Title   string  `json:"title"`
	Excerpt string  `json:"excerpt"`
	Slug    string  `json:"slug"`
	Score   float64 `json:"score"`
	URL     string  `json:"url"`
}

type searchOutput struct {
	Hits       []searchHit `json:"hits"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

func (t *toolset) searchContent(ctx context.Context, req *mcpsdk.CallToolRequest, in searchInput) (*mcpsdk.CallToolResult, searchOutput, error) {
	if err := t.requirePermission(ctx, requestHeader(req), permissions.ArticlesRead); err != nil {
		return nil, searchOutput{}, err
	}
	if strings.TrimSpace(in.Query) == "" {
		return nil, searchOutput{}, fmt.Errorf("query is required")
	}
	res, err := t.deps.Article.Search(ctx, services.SearchQuery{
		Query:    in.Query,
		TenantID: t.tenantID(ctx),
		Type:     in.Type,
		Status:   t.status(),
		Locale:   in.Locale,
		Page:     in.Page,
		PageSize: in.PageSize,
	})
	if err != nil {
		return nil, searchOutput{}, err
	}
	out := searchOutput{
		Total:      res.Total,
		Page:       res.Page,
		PageSize:   res.PageSize,
		TotalPages: res.TotalPages,
		Hits:       make([]searchHit, 0, len(res.Hits)),
	}
	for _, h := range res.Hits {
		out.Hits = append(out.Hits, searchHit{
			ID:      h.ID,
			Type:    h.Type,
			Title:   h.Title,
			Excerpt: h.Excerpt,
			Slug:    h.Slug,
			Score:   h.Score,
			URL:     t.articleURL(h.Slug),
		})
	}
	return nil, out, nil
}

// ─── list_articles ─────────────────────────────────────────────────────────��

type listArticlesInput struct {
	Page     int    `json:"page,omitempty" jsonschema:"1-based page number (default 1)"`
	PageSize int    `json:"page_size,omitempty" jsonschema:"items per page, 1-100 (default 20)"`
	Category string `json:"category,omitempty" jsonschema:"optional category ID filter"`
	Tag      string `json:"tag,omitempty" jsonschema:"optional tag slug filter"`
	Sort     string `json:"sort,omitempty" jsonschema:"sort order: newest, oldest, title, views or likes (default newest)"`
}

type articleSummary struct {
	ID          uint       `json:"id"`
	Version     int        `json:"version"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Excerpt     string     `json:"excerpt"`
	Status      string     `json:"status"`
	Author      string     `json:"author,omitempty"`
	Category    string     `json:"category,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	URL         string     `json:"url"`
}

type listArticlesOutput struct {
	Items      []articleSummary `json:"items"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
}

func (t *toolset) listArticles(ctx context.Context, req *mcpsdk.CallToolRequest, in listArticlesInput) (*mcpsdk.CallToolResult, listArticlesOutput, error) {
	if err := t.requirePermission(ctx, requestHeader(req), permissions.ArticlesRead); err != nil {
		return nil, listArticlesOutput{}, err
	}
	resp, err := t.deps.Article.List(services.ListArticlesFilter{
		Page:       in.Page,
		PageSize:   in.PageSize,
		Status:     t.status(),
		CategoryID: in.Category,
		TagSlug:    in.Tag,
		Sort:       in.Sort,
	}, t.tenantID(ctx))
	if err != nil {
		return nil, listArticlesOutput{}, err
	}
	out := listArticlesOutput{
		Total:      resp.Total,
		Page:       resp.Page,
		PageSize:   resp.PageSize,
		TotalPages: resp.TotalPages,
		Items:      []articleSummary{},
	}
	if articles, ok := resp.Items.([]models.Article); ok {
		for i := range articles {
			out.Items = append(out.Items, t.summarize(&articles[i]))
		}
	}
	return nil, out, nil
}

// summarize projects an article into the compact AI-facing shape (no body).
func (t *toolset) summarize(a *models.Article) articleSummary {
	s := articleSummary{
		ID:          a.ID,
		Version:     a.Version,
		Title:       a.Title,
		Slug:        a.Slug,
		Excerpt:     a.Excerpt,
		Status:      string(a.Status),
		PublishedAt: a.PublishedAt,
		URL:         t.articleURL(a.Slug),
	}
	s.Author = authorName(a)
	if a.Category != nil {
		s.Category = a.Category.Name
	}
	for _, tag := range a.Tags {
		s.Tags = append(s.Tags, tag.Name)
	}
	return s
}

// ─── get_article ─────────────────────────────────────────────────────────────

type getArticleInput struct {
	ID uint `json:"id" jsonschema:"the numeric article ID"`
}

type articleDetail struct {
	ID          uint       `json:"id"`
	Version     int        `json:"version"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Excerpt     string     `json:"excerpt"`
	Content     string     `json:"content"`
	Status      string     `json:"status"`
	Author      string     `json:"author,omitempty"`
	Category    string     `json:"category,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	Locale      string     `json:"locale,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	URL         string     `json:"url"`
}

func (t *toolset) getArticle(ctx context.Context, req *mcpsdk.CallToolRequest, in getArticleInput) (*mcpsdk.CallToolResult, articleDetail, error) {
	if err := t.requirePermission(ctx, requestHeader(req), permissions.ArticlesRead); err != nil {
		return nil, articleDetail{}, err
	}
	if in.ID == 0 {
		return nil, articleDetail{}, fmt.Errorf("id is required")
	}
	a, err := t.deps.Article.Get(in.ID, t.tenantID(ctx))
	if err != nil {
		return nil, articleDetail{}, fmt.Errorf("article not found")
	}
	if !t.deps.IncludeDrafts && a.Status != models.StatusPublished {
		return nil, articleDetail{}, fmt.Errorf("article not found or not published")
	}
	d := articleDetail{
		ID:          a.ID,
		Version:     a.Version,
		Title:       a.Title,
		Slug:        a.Slug,
		Excerpt:     a.Excerpt,
		Content:     a.Content,
		Status:      string(a.Status),
		Locale:      a.Locale,
		PublishedAt: a.PublishedAt,
		URL:         t.articleURL(a.Slug),
		Author:      authorName(a),
	}
	if a.Category != nil {
		d.Category = a.Category.Name
	}
	for _, tag := range a.Tags {
		d.Tags = append(d.Tags, tag.Name)
	}
	return nil, d, nil
}

// ─── list_content_types ────────────────────────────────────────────────────��

// emptyInput is used for tools that take no arguments. AddTool requires a
// struct (or map) input so the inferred JSON schema is an object.
type emptyInput struct{}

type contentFieldInfo struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	RelationUID string `json:"relation_uid,omitempty"`
}

type contentTypeInfo struct {
	UID         string             `json:"uid"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	IsSingle    bool               `json:"is_single"`
	Fields      []contentFieldInfo `json:"fields"`
}

type listContentTypesOutput struct {
	ContentTypes []contentTypeInfo `json:"content_types"`
}

func (t *toolset) listContentTypes(ctx context.Context, req *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, listContentTypesOutput, error) {
	if err := t.requirePermission(ctx, requestHeader(req), permissions.ContentTypesRead); err != nil {
		return nil, listContentTypesOutput{}, err
	}
	types, err := t.deps.ContentType.ListContentTypes(t.tenantID(ctx))
	if err != nil {
		return nil, listContentTypesOutput{}, err
	}
	out := listContentTypesOutput{ContentTypes: make([]contentTypeInfo, 0, len(types))}
	for i := range types {
		ct := &types[i]
		info := contentTypeInfo{
			UID:         ct.UID,
			Name:        ct.Name,
			Description: ct.Description,
			IsSingle:    ct.IsSingle,
			Fields:      make([]contentFieldInfo, 0, len(ct.Fields)),
		}
		for _, f := range ct.Fields {
			info.Fields = append(info.Fields, contentFieldInfo{
				Name:        f.Name,
				Label:       f.Label,
				Type:        f.FieldType,
				Required:    f.Required,
				RelationUID: f.RelationUID,
			})
		}
		out.ContentTypes = append(out.ContentTypes, info)
	}
	return nil, out, nil
}

// authorName returns the article author's display name, falling back to the
// username when no display name is set.
func authorName(a *models.Article) string {
	if a.Author.DisplayName != "" {
		return a.Author.DisplayName
	}
	return a.Author.Username
}

// ─── rag_search ──────────────────────────────────────────────────────────────

type ragSearchInput struct {
	Query string `json:"query" jsonschema:"the natural-language search query"`
	TopK  int    `json:"top_k,omitempty" jsonschema:"max results to return, 1-50 (default 5)"`
}

type ragSearchHit struct {
	DocID   uint    `json:"doc_id"`
	DocType string  `json:"doc_type"`
	Title   string  `json:"title"`
	Slug    string  `json:"slug"`
	Excerpt string  `json:"excerpt"`
	Score   float64 `json:"score"`
	Locale  string  `json:"locale"`
	URL     string  `json:"url"`
}

type ragSearchOutput struct {
	Query  string         `json:"query"`
	Hits   []ragSearchHit `json:"hits"`
	Total  int            `json:"total"`
	TookMs float64        `json:"took_ms"`
}

func (t *toolset) ragSearch(ctx context.Context, req *mcpsdk.CallToolRequest, in ragSearchInput) (*mcpsdk.CallToolResult, ragSearchOutput, error) {
	if err := t.requirePermission(ctx, requestHeader(req), permissions.AIRead); err != nil {
		return nil, ragSearchOutput{}, err
	}
	if strings.TrimSpace(in.Query) == "" {
		return nil, ragSearchOutput{}, fmt.Errorf("query is required")
	}
	if in.TopK <= 0 || in.TopK > 50 {
		in.TopK = 5
	}
	result, err := t.deps.RAG.Search(ctx, in.Query, t.tenantID(ctx), in.TopK)
	if err != nil {
		return nil, ragSearchOutput{}, err
	}
	t.auditRAG(ctx, "mcp.rag_search", in.Query, in.TopK, result.Total)
	out := ragSearchOutput{
		Query:  result.Query,
		Total:  result.Total,
		TookMs: float64(result.Took) / float64(time.Millisecond),
		Hits:   make([]ragSearchHit, 0, len(result.Hits)),
	}
	for _, h := range result.Hits {
		out.Hits = append(out.Hits, ragSearchHit{
			DocID:   h.DocID,
			DocType: h.DocType,
			Title:   h.Title,
			Slug:    h.Slug,
			Excerpt: h.Excerpt,
			Score:   h.Score,
			Locale:  h.Locale,
			URL:     t.articleURL(h.Slug),
		})
	}
	return nil, out, nil
}

// ─── rag_ask ─────────────────────────────────────────────────────────────────

type ragAskInput struct {
	Query string `json:"query" jsonschema:"the question to ask"`
	TopK  int    `json:"top_k,omitempty" jsonschema:"max context chunks to retrieve, 1-20 (default 5)"`
}

type ragContextItem struct {
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

type ragAskOutput struct {
	Query   string           `json:"query"`
	Answer  string           `json:"answer,omitempty"`
	Context []ragContextItem `json:"context"`
	TookMs  float64          `json:"took_ms"`
}

func (t *toolset) ragAsk(ctx context.Context, req *mcpsdk.CallToolRequest, in ragAskInput) (*mcpsdk.CallToolResult, ragAskOutput, error) {
	if err := t.requirePermission(ctx, requestHeader(req), permissions.AIAsk); err != nil {
		return nil, ragAskOutput{}, err
	}
	if strings.TrimSpace(in.Query) == "" {
		return nil, ragAskOutput{}, fmt.Errorf("query is required")
	}
	if in.TopK <= 0 || in.TopK > 20 {
		in.TopK = 5
	}
	result, err := t.deps.RAG.Ask(ctx, in.Query, t.tenantID(ctx), in.TopK)
	if err != nil {
		return nil, ragAskOutput{}, err
	}
	t.auditRAG(ctx, "mcp.rag_ask", in.Query, in.TopK, len(result.Context))
	out := ragAskOutput{
		Query:   result.Query,
		Answer:  result.Answer,
		TookMs:  float64(result.Took) / float64(time.Millisecond),
		Context: make([]ragContextItem, 0, len(result.Context)),
	}
	for _, c := range result.Context {
		out.Context = append(out.Context, ragContextItem{
			Title:   c.Title,
			Content: c.Content,
			Score:   c.Score,
		})
	}
	return nil, out, nil
}
