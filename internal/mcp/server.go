// Package mcp exposes ContentX content to AI agents over the Model Context
// Protocol (https://modelcontextprotocol.io). It wraps the existing services
// as MCP tools using the official Go SDK.
//
// The tool layer is transport-agnostic: NewServer returns a configured
// *mcpsdk.Server that the caller binds to a transport (stdio via
// `contentx --mcp`, or Streamable HTTP mounted on the API server).
package mcp

import (
	"context"
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
)

// Principal is the fully verified HTTP API-token identity available to MCP
// handlers. Permissions are already reduced to the token/global/tenant
// intersection by TokenService.Resolve.
type Principal struct {
	UserID      uint
	TenantID    uint
	Permissions []string
}

// WriterIdentity is kept as the Authorizer-facing name for write tools. It is
// exactly the same verified identity used by HTTP read-tool guards.
type WriterIdentity = Principal

// principalCtxKey is private to prevent context-key collisions and ad hoc
// reads. Authentication code remains responsible for calling WithPrincipal
// only after resolving current token/user/tenant/membership state.
type principalCtxKey struct{}

// tenantCtxKey is the context key for the per-request tenant in MCP tool calls.
type tenantCtxKey struct{}

// WithPrincipal embeds a defensive copy of a verified principal in a request
// context. HTTP authentication middleware must call this only after resolving
// the token against current user, tenant, membership, and permissions state.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	principal.Permissions = append([]string(nil), principal.Permissions...)
	return context.WithValue(ctx, principalCtxKey{}, principal)
}

// PrincipalFromContext returns a defensive copy of the verified HTTP
// principal. Missing or structurally invalid principals are rejected so HTTP
// permission guards can fail closed. Stdio callers intentionally have none.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	principal, ok := ctx.Value(principalCtxKey{}).(Principal)
	if !ok || principal.UserID == 0 || principal.TenantID == 0 {
		return Principal{}, false
	}
	principal.Permissions = append([]string(nil), principal.Permissions...)
	return principal, true
}

// WithTenant embeds the tenant ID in a context for downstream tool handlers.
func WithTenant(ctx context.Context, tenantID uint) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, tenantID)
}

// TenantFromContext extracts the tenant ID from a tool-call context, falling
// back to the default tenant when unset (e.g. stdio mode).
func TenantFromContext(ctx context.Context) uint {
	if principal, ok := PrincipalFromContext(ctx); ok {
		return principal.TenantID
	}
	if ctx == nil {
		return models.DefaultTenantID
	}
	if v, ok := ctx.Value(tenantCtxKey{}).(uint); ok && v > 0 {
		return v
	}
	return models.DefaultTenantID
}

// Authorizer resolves the API token in an MCP HTTP request's headers into the
// current effective principal used by every HTTP tool and resource guard. Its
// presence also enables write tools; nil means local read-only stdio mode.
type Authorizer interface {
	Resolve(h http.Header) (*WriterIdentity, error)
}

// Deps holds the services the MCP tools delegate to.
type Deps struct {
	Article     *services.ArticleService
	ContentType *services.ContentTypeService
	// RAG, when non-nil, enables the semantic search and RAG Q&A tools
	// (rag_search, rag_ask). When nil the RAG tools are not registered.
	RAG *services.RAGService
	// BaseURL is used to build absolute article URLs in tool results.
	BaseURL string
	// IncludeDrafts, when true, lets read tools return non-published content.
	// Default false: only published content is exposed (safe default that
	// mirrors the public REST/GraphQL surface). Enable via MCP_INCLUDE_DRAFTS
	// for trusted local (stdio) use only.
	IncludeDrafts bool
	// Authorizer, when non-nil, guards every HTTP tool/resource and enables the
	// write tools (create/update/publish). Each call re-resolves current user,
	// tenant, membership, and effective permissions from the request token.
	Authorizer Authorizer
}

// NewServer builds the local/transport-agnostic MCP surface. Stdio has no HTTP
// Authorizer, so resources use the default tenant for backward compatibility.
func NewServer(deps Deps, version string) *mcpsdk.Server {
	return newServer(deps, version, nil)
}

// NewServerForPrincipal builds one authenticated HTTP MCP surface for the
// given effective principal. HTTP mounting uses stateless transport so this is
// rebuilt per request: resources/list metadata is therefore tenant-correct,
// permission-filtered, and immediately reflects permission revocation.
func NewServerForPrincipal(deps Deps, version string, principal Principal) *mcpsdk.Server {
	principal.Permissions = append([]string(nil), principal.Permissions...)
	return newServer(deps, version, &principal)
}

func newServer(deps Deps, version string, principal *Principal) *mcpsdk.Server {
	if version == "" {
		version = "dev"
	}
	s := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "contentx",
		Title:   "ContentX",
		Version: version,
	}, nil)
	if deps.Authorizer != nil {
		// Listing resources has no SDK-level per-resource callback. Gate both
		// discovery methods here so an HTTP token without the corresponding
		// effective read permission receives an explicit denial rather than an
		// apparently valid empty catalogue.
		s.AddReceivingMiddleware(resourceDiscoveryPermissionMiddleware(deps))
	}
	registerTools(s, deps)
	registerResources(s, deps, principal)
	registerPrompts(s)
	return s
}
