package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPRoundTrip exercises the full MCP protocol layer over the SDK's
// in-memory transport: a real Client drives tools/list and tools/call against
// the Server, so tool registration, JSON-schema validation, argument
// unmarshaling and structured-result serialization are all covered (unlike the
// direct handler tests that call the Go methods).
func TestMCPRoundTrip(t *testing.T) {
	deps, pubID, _ := newTestDeps(t, false)
	srv := NewServer(deps, "test")

	clientT, serverT := mcpsdk.NewInMemoryTransports()

	ctx := context.Background()
	// Servers must be connected before clients (the client initializes the
	// session on connect).
	ss, err := srv.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer func() { _ = ss.Close() }()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	// 1) The four read-only tools must be advertised.
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	advertised := map[string]bool{}
	for _, tl := range tools.Tools {
		advertised[tl.Name] = true
	}
	for _, want := range []string{"search_content", "list_articles", "get_article", "list_content_types"} {
		if !advertised[want] {
			t.Errorf("tool %q not advertised", want)
		}
	}

	// 2) search_content returns the published article via structured output.
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "search_content",
		Arguments: map[string]any{"query": "alphaunique"},
	})
	if err != nil {
		t.Fatalf("call search_content: %v", err)
	}
	if res.IsError {
		t.Fatalf("search_content returned a tool error: %+v", res.Content)
	}
	var out searchOutput
	decodeStructured(t, res.StructuredContent, &out)
	if !hasSlug(out.Hits, "published-one") {
		t.Errorf("expected published-one in hits, got %+v", out.Hits)
	}

	// 3) get_article by ID returns the full body over the wire.
	res, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_article",
		Arguments: map[string]any{"id": pubID},
	})
	if err != nil {
		t.Fatalf("call get_article: %v", err)
	}
	if res.IsError {
		t.Fatalf("get_article returned a tool error: %+v", res.Content)
	}
	var detail articleDetail
	decodeStructured(t, res.StructuredContent, &detail)
	if detail.Content != "hello world alphaunique" {
		t.Errorf("content = %q, want the published body", detail.Content)
	}

	// 4) A handler error surfaces as a tool error (IsError), not a protocol error.
	res, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "search_content",
		Arguments: map[string]any{"query": ""},
	})
	if err != nil {
		t.Fatalf("call search_content(empty): unexpected protocol error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError for an empty query")
	}
}

func hasSlug(hits []searchHit, slug string) bool {
	for _, h := range hits {
		if h.Slug == slug {
			return true
		}
	}
	return false
}

// decodeStructured re-marshals a tool's structured output (an arbitrary JSON
// object) into the given typed value.
func decodeStructured(t *testing.T, structured any, v any) {
	t.Helper()
	data, err := json.Marshal(structured)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
}
