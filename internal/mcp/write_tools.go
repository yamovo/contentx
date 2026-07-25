package mcp

import (
	"context"
	"fmt"
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yamovo/contentx/internal/services"
)

// MCP write-tool permission strings. These are matched against the API token's
// own Permissions list (models.APIToken.Permissions); "*" grants everything.
const (
	permCreate  = "articles.create"
	permEdit    = "articles.edit"
	permEditAll = "articles.edit_all"
	permPublish = "articles.publish"
)

// registerWriteTools adds the mutating tools. It is only called when an
// Authorizer is configured (HTTP mode); stdio stays read-only.
func registerWriteTools(s *mcpsdk.Server, ts *toolset) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "create_article",
		Description: "Create a new article as the API token's user. Always saved as a draft; use publish_article to publish. Requires the token permission 'articles.create'.",
	}, ts.createArticle)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "update_article",
		Description: "Update an existing article's fields (never publishes). Requires 'articles.edit'; editing another user's article additionally requires 'articles.edit_all'.",
	}, ts.updateArticle)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "publish_article",
		Description: "Publish an existing article by ID. Requires the token permission 'articles.publish'.",
	}, ts.publishArticle)
}

// permitted reports whether the token permission set grants want ("*" wildcard
// or exact match).
func permitted(perms []string, want string) bool {
	for _, p := range perms {
		if p == "*" || p == want {
			return true
		}
	}
	return false
}

// identity resolves the acting writer from the request's HTTP headers via the
// configured Authorizer. Returns an error when write access is unavailable.
func (t *toolset) identity(req *mcpsdk.CallToolRequest) (*WriterIdentity, error) {
	if t.deps.Authorizer == nil {
		return nil, fmt.Errorf("write operations are not enabled")
	}
	var h http.Header
	if req != nil && req.Extra != nil {
		h = req.Extra.Header
	}
	if h == nil {
		return nil, fmt.Errorf("missing request headers")
	}
	return t.deps.Authorizer.Resolve(h)
}

// ─── create_article ────────────────────────────────────────────────────────

type createArticleInput struct {
	Title      string `json:"title" jsonschema:"the article title (required)"`
	Content    string `json:"content,omitempty" jsonschema:"the article body (Markdown or HTML)"`
	Excerpt    string `json:"excerpt,omitempty" jsonschema:"optional short summary"`
	CategoryID *uint  `json:"category_id,omitempty" jsonschema:"optional category ID"`
	TagIDs     []uint `json:"tag_ids,omitempty" jsonschema:"optional list of tag IDs"`
	Locale     string `json:"locale,omitempty" jsonschema:"optional BCP-47 locale, e.g. en or zh (default en)"`
}

func (t *toolset) createArticle(_ context.Context, req *mcpsdk.CallToolRequest, in createArticleInput) (*mcpsdk.CallToolResult, articleSummary, error) {
	id, err := t.identity(req)
	if err != nil {
		return nil, articleSummary{}, err
	}
	if !permitted(id.Permissions, permCreate) {
		return nil, articleSummary{}, fmt.Errorf("token lacks permission: %s", permCreate)
	}
	if in.Title == "" {
		return nil, articleSummary{}, fmt.Errorf("title is required")
	}
	// Status is deliberately left unset so the service defaults to draft; MCP
	// never auto-publishes on create.
	article, err := t.deps.Article.Create(services.CreateArticleRequest{
		Title:      in.Title,
		Content:    in.Content,
		Excerpt:    in.Excerpt,
		CategoryID: in.CategoryID,
		TagIDs:     in.TagIDs,
		Locale:     in.Locale,
	}, id.UserID)
	if err != nil {
		return nil, articleSummary{}, err
	}
	return nil, t.summarize(article), nil
}

// ─── update_article ────────────────────────────────────────────────────────

type updateArticleInput struct {
	ID         uint    `json:"id" jsonschema:"the article ID to update (required)"`
	Title      *string `json:"title,omitempty" jsonschema:"new title"`
	Content    *string `json:"content,omitempty" jsonschema:"new body"`
	Excerpt    *string `json:"excerpt,omitempty" jsonschema:"new excerpt"`
	CategoryID *uint   `json:"category_id,omitempty" jsonschema:"new category ID"`
	TagIDs     []uint  `json:"tag_ids,omitempty" jsonschema:"replacement tag IDs"`
}

func (t *toolset) updateArticle(_ context.Context, req *mcpsdk.CallToolRequest, in updateArticleInput) (*mcpsdk.CallToolResult, articleSummary, error) {
	id, err := t.identity(req)
	if err != nil {
		return nil, articleSummary{}, err
	}
	if !permitted(id.Permissions, permEdit) {
		return nil, articleSummary{}, fmt.Errorf("token lacks permission: %s", permEdit)
	}
	if in.ID == 0 {
		return nil, articleSummary{}, fmt.Errorf("id is required")
	}
	// articles.edit_all lets the token edit articles it does not own; otherwise
	// the service enforces ownership. Status is not touched here (no publish).
	isEditor := permitted(id.Permissions, permEditAll)
	article, err := t.deps.Article.Update(in.ID, services.UpdateArticleRequest{
		Title:      in.Title,
		Content:    in.Content,
		Excerpt:    in.Excerpt,
		CategoryID: in.CategoryID,
		TagIDs:     in.TagIDs,
	}, id.UserID, isEditor)
	if err != nil {
		return nil, articleSummary{}, err
	}
	return nil, t.summarize(article), nil
}

// ─── publish_article ───────────────────────────────────────────────────────

type publishArticleInput struct {
	ID uint `json:"id" jsonschema:"the article ID to publish (required)"`
}

func (t *toolset) publishArticle(_ context.Context, req *mcpsdk.CallToolRequest, in publishArticleInput) (*mcpsdk.CallToolResult, articleSummary, error) {
	id, err := t.identity(req)
	if err != nil {
		return nil, articleSummary{}, err
	}
	if !permitted(id.Permissions, permPublish) {
		return nil, articleSummary{}, fmt.Errorf("token lacks permission: %s", permPublish)
	}
	if in.ID == 0 {
		return nil, articleSummary{}, fmt.Errorf("id is required")
	}
	article, err := t.deps.Article.Publish(in.ID)
	if err != nil {
		return nil, articleSummary{}, err
	}
	return nil, t.summarize(article), nil
}
