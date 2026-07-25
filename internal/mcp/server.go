// Package mcp exposes ContentX content to AI agents over the Model Context
// Protocol (https://modelcontextprotocol.io). It wraps the existing services
// as MCP tools using the official Go SDK.
//
// The tool layer is transport-agnostic: NewServer returns a configured
// *mcpsdk.Server that the caller binds to a transport (stdio via
// `contentx --mcp`, or Streamable HTTP mounted on the API server).
package mcp

import (
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yamovo/contentx/internal/services"
)

// WriterIdentity is the authenticated identity behind a write-tool call,
// resolved from the request's API token.
type WriterIdentity struct {
	UserID      uint
	Permissions []string
}

// Authorizer resolves the API token in an MCP HTTP request's headers into a
// WriterIdentity. It is set only when write tools should be exposed (HTTP
// mode); a nil Authorizer means read-only (e.g. stdio).
type Authorizer interface {
	Resolve(h http.Header) (*WriterIdentity, error)
}

// Deps holds the services the MCP tools delegate to.
type Deps struct {
	Article     *services.ArticleService
	ContentType *services.ContentTypeService
	// BaseURL is used to build absolute article URLs in tool results.
	BaseURL string
	// IncludeDrafts, when true, lets read tools return non-published content.
	// Default false: only published content is exposed (safe default that
	// mirrors the public REST/GraphQL surface). Enable via MCP_INCLUDE_DRAFTS
	// for trusted local (stdio) use only.
	IncludeDrafts bool
	// Authorizer, when non-nil, enables the write tools (create/update/publish)
	// and resolves the acting user + permissions from each request's API token.
	Authorizer Authorizer
}

// NewServer builds an MCP server exposing ContentX's tools. The returned server
// is transport-agnostic; the caller runs it over stdio or mounts it on HTTP.
func NewServer(deps Deps, version string) *mcpsdk.Server {
	if version == "" {
		version = "dev"
	}
	s := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "contentx",
		Title:   "ContentX",
		Version: version,
	}, nil)
	registerTools(s, deps)
	registerResources(s, deps)
	return s
}
