package mcp

import (
	"context"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPPromptsRoundTrip drives prompts/list and prompts/get over the SDK's
// in-memory transport, covering registration, argument templating, defaults
// and required-argument validation at the protocol level.
func TestMCPPromptsRoundTrip(t *testing.T) {
	deps, _, _ := newTestDeps(t, false)
	srv := NewServer(deps, "test")

	clientT, serverT := mcpsdk.NewInMemoryTransports()

	ctx := context.Background()
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

	// 1) All four writing prompts must be advertised.
	list, err := cs.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}
	advertised := map[string]bool{}
	for _, p := range list.Prompts {
		advertised[p.Name] = true
	}
	for _, want := range []string{"write_article", "improve_article", "summarize_article", "translate_article"} {
		if !advertised[want] {
			t.Errorf("prompt %q not advertised", want)
		}
	}

	// 2) write_article templates the topic and orchestrates the real tools.
	res, err := cs.GetPrompt(ctx, &mcpsdk.GetPromptParams{
		Name:      "write_article",
		Arguments: map[string]string{"topic": "Go generics", "locale": "zh"},
	})
	if err != nil {
		t.Fatalf("get write_article: %v", err)
	}
	text := promptText(t, res)
	for _, want := range []string{"Go generics", "search_content", "create_article", "publish_article", `locale "zh"`} {
		if !strings.Contains(text, want) {
			t.Errorf("write_article text missing %q", want)
		}
	}
	// Defaults fill unset arguments.
	if !strings.Contains(text, "informative blog post") {
		t.Error("write_article should default style to informative blog post")
	}

	// 3) improve_article references get_article/update_article with the ID.
	res, err = cs.GetPrompt(ctx, &mcpsdk.GetPromptParams{
		Name:      "improve_article",
		Arguments: map[string]string{"article_id": "42", "focus": "seo"},
	})
	if err != nil {
		t.Fatalf("get improve_article: %v", err)
	}
	text = promptText(t, res)
	for _, want := range []string{"42", "seo", "get_article", "update_article"} {
		if !strings.Contains(text, want) {
			t.Errorf("improve_article text missing %q", want)
		}
	}

	// 4) Missing a required argument is a protocol-level error.
	if _, err = cs.GetPrompt(ctx, &mcpsdk.GetPromptParams{Name: "write_article"}); err == nil {
		t.Error("expected error for write_article without topic")
	}
	if _, err = cs.GetPrompt(ctx, &mcpsdk.GetPromptParams{
		Name:      "translate_article",
		Arguments: map[string]string{"article_id": "7"},
	}); err == nil {
		t.Error("expected error for translate_article without target_locale")
	}

	// 5) translate_article templates both source ID and target locale.
	res, err = cs.GetPrompt(ctx, &mcpsdk.GetPromptParams{
		Name:      "translate_article",
		Arguments: map[string]string{"article_id": "7", "target_locale": "ja"},
	})
	if err != nil {
		t.Fatalf("get translate_article: %v", err)
	}
	text = promptText(t, res)
	for _, want := range []string{"7", "ja", "get_article", "create_article"} {
		if !strings.Contains(text, want) {
			t.Errorf("translate_article text missing %q", want)
		}
	}

	// 6) summarize_article defaults max_words to 100.
	res, err = cs.GetPrompt(ctx, &mcpsdk.GetPromptParams{
		Name:      "summarize_article",
		Arguments: map[string]string{"article_id": "9"},
	})
	if err != nil {
		t.Fatalf("get summarize_article: %v", err)
	}
	if text = promptText(t, res); !strings.Contains(text, "100 words") {
		t.Errorf("summarize_article should default to 100 words, got: %s", text)
	}
}

// promptText extracts the single user message's text from a prompt result.
func promptText(t *testing.T, res *mcpsdk.GetPromptResult) string {
	t.Helper()
	if len(res.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(res.Messages))
	}
	if res.Messages[0].Role != "user" {
		t.Errorf("expected role user, got %s", res.Messages[0].Role)
	}
	tc, ok := res.Messages[0].Content.(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Messages[0].Content)
	}
	return tc.Text
}
