package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yamovo/contentx/internal/models"
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

	// Write tools are exposed only when an Authorizer is configured (HTTP mode).
	if deps.Authorizer != nil {
		registerWriteTools(s, ts)
	}
}

// toolset carries the shared dependencies for the tool handlers.
type toolset struct {
	deps Deps
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

// tenantID returns the tenant all MCP tools operate in. stdio and HTTP modes
// currently resolve to the default tenant; per-token tenant binding lands
// with RFC-001 §11 open question 2 (PR-5).
func (t *toolset) tenantID() uint {
	return models.DefaultTenantID
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

func (t *toolset) searchContent(ctx context.Context, _ *mcpsdk.CallToolRequest, in searchInput) (*mcpsdk.CallToolResult, searchOutput, error) {
	if strings.TrimSpace(in.Query) == "" {
		return nil, searchOutput{}, fmt.Errorf("query is required")
	}
	res, err := t.deps.Article.Search(ctx, services.SearchQuery{
		Query:    in.Query,
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

func (t *toolset) listArticles(_ context.Context, _ *mcpsdk.CallToolRequest, in listArticlesInput) (*mcpsdk.CallToolResult, listArticlesOutput, error) {
	resp, err := t.deps.Article.List(services.ListArticlesFilter{
		Page:       in.Page,
		PageSize:   in.PageSize,
		Status:     t.status(),
		CategoryID: in.Category,
		TagSlug:    in.Tag,
		Sort:       in.Sort,
	}, t.tenantID())
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

func (t *toolset) getArticle(_ context.Context, _ *mcpsdk.CallToolRequest, in getArticleInput) (*mcpsdk.CallToolResult, articleDetail, error) {
	if in.ID == 0 {
		return nil, articleDetail{}, fmt.Errorf("id is required")
	}
	a, err := t.deps.Article.Get(in.ID, t.tenantID())
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

func (t *toolset) listContentTypes(_ context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, listContentTypesOutput, error) {
	types, err := t.deps.ContentType.ListContentTypes()
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
