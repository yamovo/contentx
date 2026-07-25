package mcp

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yamovo/contentx/internal/models"
)

// registerResources exposes ContentX content as MCP resources. Content types
// are registered as concrete resources (appear in resources/list); articles
// are exposed via a URI template (appear in resources/templates/list).
func registerResources(s *mcpsdk.Server, deps Deps) {
	// Content types: concrete resources.
	types, err := deps.ContentType.ListContentTypes()
	if err == nil {
		ctHandler := readContentTypeResource(deps)
		for i := range types {
			ct := &types[i]
			s.AddResource(&mcpsdk.Resource{
				URI:         "contentx://content-types/" + ct.UID,
				Name:        ct.UID,
				Title:       ct.Name,
				Description: "Content type schema: " + ct.Name,
				MIMEType:    "application/json",
			}, ctHandler)
		}
	}

	// Articles: resource template (dynamic read by ID).
	s.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "contentx://articles/{id}",
		Name:        "article",
		Title:       "Article",
		Description: "Read a published article by numeric ID. Use search_content or list_articles tools to discover IDs.",
		MIMEType:    "application/json",
	}, readArticleResource(deps))
}

// readContentTypeResource handles resources/read for content-type URIs.
func readContentTypeResource(deps Deps) mcpsdk.ResourceHandler {
	return func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		uid := lastPathSegment(req.Params.URI)
		if uid == "" {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		ct, err := deps.ContentType.GetContentType(uid)
		if err != nil {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
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
		data, _ := json.Marshal(info)
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			}},
		}, nil
	}
}

// readArticleResource handles resources/read for article URIs.
func readArticleResource(deps Deps) mcpsdk.ResourceHandler {
	return func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		idStr := lastPathSegment(req.Params.URI)
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil || id == 0 {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		a, err := deps.Article.Get(uint(id))
		if err != nil {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		if !deps.IncludeDrafts && a.Status != models.StatusPublished {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		detail := articleDetail{
			ID:          a.ID,
			Title:       a.Title,
			Slug:        a.Slug,
			Excerpt:     a.Excerpt,
			Content:     a.Content,
			Status:      string(a.Status),
			Locale:      a.Locale,
			PublishedAt: a.PublishedAt,
			URL:         strings.TrimRight(deps.BaseURL, "/") + "/articles/" + a.Slug,
			Author:      authorName(a),
		}
		if a.Category != nil {
			detail.Category = a.Category.Name
		}
		for _, tag := range a.Tags {
			detail.Tags = append(detail.Tags, tag.Name)
		}
		data, _ := json.Marshal(detail)
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			}},
		}, nil
	}
}

// lastPathSegment extracts the substring after the last "/" in a URI.
func lastPathSegment(uri string) string {
	if i := strings.LastIndex(uri, "/"); i >= 0 && i < len(uri)-1 {
		return uri[i+1:]
	}
	return ""
}
