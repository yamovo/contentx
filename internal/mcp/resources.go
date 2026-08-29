package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/permissions"
)

// resourceDiscoveryPermissionMiddleware protects the SDK's built-in resource
// discovery handlers, which otherwise have no application callback. It uses
// Request.GetExtra so resources/list and resources/templates/list re-resolve
// the current POST token exactly like tools and resources/read do.
func resourceDiscoveryPermissionMiddleware(deps Deps) mcpsdk.Middleware {
	return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			var want string
			switch method {
			case "resources/list":
				want = permissions.ContentTypesRead
			case "resources/templates/list":
				want = permissions.ArticlesRead
			}
			if want == "" {
				return next(ctx, method, req)
			}
			var header http.Header
			if req != nil && req.GetExtra() != nil {
				header = req.GetExtra().Header
			}
			principal, err := (&toolset{deps: deps}).verifiedHTTPPrincipal(ctx, header)
			if err != nil || principal == nil {
				return nil, fmt.Errorf("invalid MCP principal for resource discovery")
			}
			if !permissions.Grants(principal.Permissions, want) {
				return nil, fmt.Errorf("token lacks permission: %s", want)
			}
			return next(ctx, method, req)
		}
	}
}

// registerResources exposes ContentX content as MCP resources. In local stdio
// mode resources use the default tenant. In HTTP mode the server is rebuilt
// for each verified principal, so concrete metadata is tenant-scoped and only
// registered when the principal carries the matching effective permission.
func registerResources(s *mcpsdk.Server, deps Deps, principal *Principal) {
	tenantID := models.DefaultTenantID
	allowContentTypes := deps.Authorizer == nil
	allowArticles := deps.Authorizer == nil
	if deps.Authorizer != nil {
		if principal == nil || principal.UserID == 0 || principal.TenantID == 0 {
			return
		}
		tenantID = principal.TenantID
		allowContentTypes = permissions.Grants(principal.Permissions, permissions.ContentTypesRead)
		allowArticles = permissions.Grants(principal.Permissions, permissions.ArticlesRead)
	}

	// Content types: concrete resources. Do not even enumerate schemas for a
	// principal that lacks content_types.read.
	if allowContentTypes && deps.ContentType != nil {
		types, err := deps.ContentType.ListContentTypes(tenantID)
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
	}

	// Articles: resource template (dynamic read by ID).
	if allowArticles {
		s.AddResourceTemplate(&mcpsdk.ResourceTemplate{
			URITemplate: "contentx://articles/{id}",
			Name:        "article",
			Title:       "Article",
			Description: "Read a published article by numeric ID. Use search_content or list_articles tools to discover IDs.",
			MIMEType:    "application/json",
		}, readArticleResource(deps))
	}
}

// readContentTypeResource handles resources/read for content-type URIs.
func readContentTypeResource(deps Deps) mcpsdk.ResourceHandler {
	return func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		var header http.Header
		if req != nil && req.Extra != nil {
			header = req.Extra.Header
		}
		if err := (&toolset{deps: deps}).requirePermission(ctx, header, permissions.ContentTypesRead); err != nil {
			return nil, err
		}
		uid := lastPathSegment(req.Params.URI)
		if uid == "" {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		ct, err := deps.ContentType.GetContentType(uid, TenantFromContext(ctx))
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
	return func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		var header http.Header
		if req != nil && req.Extra != nil {
			header = req.Extra.Header
		}
		if err := (&toolset{deps: deps}).requirePermission(ctx, header, permissions.ArticlesRead); err != nil {
			return nil, err
		}
		idStr := lastPathSegment(req.Params.URI)
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil || id == 0 {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		a, err := deps.Article.Get(uint(id), TenantFromContext(ctx))
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
