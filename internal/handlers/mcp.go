package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yamovo/contentx/internal/mcp"
	"github.com/yamovo/contentx/internal/services"
)

// mcpTokenAuth authenticates MCP HTTP requests using a tenant-bound,
// long-lived API token issued via /api/v1/system/tokens. The token is read from
// "Authorization: Bearer <token>" or the "X-API-Token" header. Missing or
// invalid tokens are rejected with 401 before reaching the MCP handler.
// The fully resolved principal is stored in the request context so MCP tool
// handlers can enforce current effective permissions and tenant scope.
func mcpTokenAuth(tokenSvc *services.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractMCPTokenFromHeader(c.Request.Header)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API token required"})
			return
		}
		principal, err := tokenSvc.Resolve(token)
		if err != nil || principal == nil || principal.UserID == 0 || principal.TenantID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired API token"})
			return
		}
		c.Request = c.Request.WithContext(mcp.WithPrincipal(c.Request.Context(), mcp.Principal{
			UserID:      principal.UserID,
			TenantID:    principal.TenantID,
			Permissions: principal.Permissions,
		}))
		c.Next()
	}
}

// extractMCPTokenFromHeader pulls the API token from the Authorization: Bearer
// header or the X-API-Token header.
func extractMCPTokenFromHeader(h http.Header) string {
	if bearer := h.Get("Authorization"); strings.HasPrefix(bearer, "Bearer ") {
		return strings.TrimSpace(bearer[len("Bearer "):])
	}
	return strings.TrimSpace(h.Get("X-API-Token"))
}

// mcpAuthorizer implements mcp.Authorizer by resolving the request's API token
// through the TokenService into a current tenant-scoped effective principal.
type mcpAuthorizer struct {
	tokenSvc *services.TokenService
}

func (a mcpAuthorizer) Resolve(h http.Header) (*mcp.WriterIdentity, error) {
	token := extractMCPTokenFromHeader(h)
	if token == "" {
		return nil, errors.New("API token required")
	}
	principal, err := a.tokenSvc.Resolve(token)
	if err != nil || principal == nil || principal.UserID == 0 || principal.TenantID == 0 {
		if err == nil {
			err = errors.New("invalid API token principal")
		}
		return nil, err
	}
	return &mcp.WriterIdentity{
		UserID:      principal.UserID,
		TenantID:    principal.TenantID,
		Permissions: append([]string(nil), principal.Permissions...),
	}, nil
}

// mountMCPHTTP mounts the MCP server on the given API group over the SDK's
// Streamable HTTP transport at /mcp, guarded by API-token auth. Write tools are
// enabled via the injected Authorizer (per-call permission checks still apply).
func mountMCPHTTP(api *gin.RouterGroup, deps mcp.Deps, tokenSvc *services.TokenService) {
	// MCP_INCLUDE_DRAFTS is a local stdio escape hatch. Never carry it onto
	// the network transport: HTTP tokens may read only published content even
	// when an operator enables draft access for a trusted local agent.
	deps.IncludeDrafts = false
	deps.Authorizer = mcpAuthorizer{tokenSvc: tokenSvc}
	h := mcpsdk.NewStreamableHTTPHandler(func(req *http.Request) *mcpsdk.Server {
		principal, ok := mcp.PrincipalFromContext(req.Context())
		if !ok {
			return nil
		}
		return mcp.NewServerForPrincipal(deps, "", principal)
	}, &mcpsdk.StreamableHTTPOptions{
		// Per-request servers make resource metadata tenant/permission aware and
		// prevent a stateful MCP session from retaining stale principal context.
		Stateless: true,
	})
	api.Any("/mcp", mcpTokenAuth(tokenSvc), gin.WrapH(h))
}
