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

// mcpTokenAuth authenticates MCP HTTP requests using a long-lived API token
// (models.APIToken, issued via /api/v1/system/tokens). The token is read from
// "Authorization: Bearer <token>" or the "X-API-Token" header. Missing or
// invalid tokens are rejected with 401 before reaching the MCP handler.
func mcpTokenAuth(tokenSvc *services.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractMCPTokenFromHeader(c.Request.Header)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API token required"})
			return
		}
		ok, _, err := tokenSvc.Validate(token, "")
		if err != nil || !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired API token"})
			return
		}
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
// through the TokenService into the acting user plus the token's permissions.
type mcpAuthorizer struct {
	tokenSvc *services.TokenService
}

func (a mcpAuthorizer) Resolve(h http.Header) (*mcp.WriterIdentity, error) {
	token := extractMCPTokenFromHeader(h)
	if token == "" {
		return nil, errors.New("API token required")
	}
	tok, err := a.tokenSvc.Resolve(token)
	if err != nil {
		return nil, err
	}
	return &mcp.WriterIdentity{UserID: tok.CreatedByID, Permissions: []string(tok.Permissions)}, nil
}

// mountMCPHTTP mounts the MCP server on the given API group over the SDK's
// Streamable HTTP transport at /mcp, guarded by API-token auth. Write tools are
// enabled via the injected Authorizer (per-call permission checks still apply).
func mountMCPHTTP(api *gin.RouterGroup, deps mcp.Deps, tokenSvc *services.TokenService) {
	deps.Authorizer = mcpAuthorizer{tokenSvc: tokenSvc}
	srv := mcp.NewServer(deps, "")
	h := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return srv }, nil)
	api.Any("/mcp", mcpTokenAuth(tokenSvc), gin.WrapH(h))
}
