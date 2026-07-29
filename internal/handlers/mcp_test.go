package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/mcp"
	"github.com/yamovo/contentx/internal/services"
)

func setupMCPTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.Seed(db)
	return db
}

// TestMCPTokenAuth verifies the API-token gate on the MCP HTTP endpoint.
func TestMCPTokenAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMCPTestDB(t)
	user := createTestUserDB(t, db, "mcp-token-user", "admin")
	tokenSvc := services.NewTokenService(db)
	created, err := tokenSvc.Create(services.CreateTokenRequest{Name: "mcp"}, user.ID)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	r := gin.New()
	r.GET("/guarded", mcpTokenAuth(tokenSvc), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	cases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"no token", nil, http.StatusUnauthorized},
		{"bad bearer", map[string]string{"Authorization": "Bearer nope"}, http.StatusUnauthorized},
		{"valid bearer", map[string]string{"Authorization": "Bearer " + created.Token}, http.StatusOK},
		{"valid x-api-token", map[string]string{"X-API-Token": created.Token}, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/guarded", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}

// authRoundTripper injects a Bearer token on every request, since the SDK's
// StreamableClientTransport has no header field of its own.
type authRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (a authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+a.token)
	return a.base.RoundTrip(r)
}

// TestMCPHTTPRoundTrip exercises the full Streamable HTTP path end-to-end: a
// real SDK client connects over HTTP (through the API-token auth middleware)
// and drives tools/list + tools/call.
func TestMCPHTTPRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMCPTestDB(t)
	user := createTestUserDB(t, db, "mcp-http-user", "admin")
	createTestArticleDB(t, db, user.ID, "Published One")

	articleSvc := services.NewArticleService(db, "http://localhost:8080")
	articleSvc.SetSearchIndexer(services.NewBuiltinIndexer())
	if _, err := articleSvc.ReindexAll(context.Background()); err != nil {
		t.Fatalf("reindex: %v", err)
	}
	tokenSvc := services.NewTokenService(db)
	created, err := tokenSvc.Create(services.CreateTokenRequest{Name: "mcp"}, user.ID)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	r := gin.New()
	api := r.Group("/api/v1")
	mountMCPHTTP(api, mcp.Deps{
		Article:     articleSvc,
		ContentType: services.NewContentTypeService(db),
		BaseURL:     "http://localhost:8080",
	}, tokenSvc)

	ts := httptest.NewServer(r)
	defer ts.Close()
	endpoint := ts.URL + "/api/v1/mcp"

	// Unauthenticated request is rejected with 401 before reaching the handler.
	resp, err := http.Post(endpoint, "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	if err != nil {
		t.Fatalf("unauth post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth status = %d, want 401", resp.StatusCode)
	}

	// Authenticated round-trip via the SDK streamable client transport.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	transport := &mcpsdk.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           &http.Client{Transport: authRoundTripper{token: created.Token, base: http.DefaultTransport}},
		DisableStandaloneSSE: true,
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("authenticated connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(tools.Tools) != 7 {
		t.Errorf("expected 7 tools (4 read + 3 write), got %d", len(tools.Tools))
	}

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "search_content",
		Arguments: map[string]any{"query": "content"},
	})
	if err != nil {
		t.Fatalf("call search_content: %v", err)
	}
	if res.IsError {
		t.Fatalf("search_content returned a tool error: %+v", res.Content)
	}
}

// mkToken creates an API token with the given permissions and returns the raw token string.
func mkToken(t *testing.T, tokenSvc *services.TokenService, userID uint, perms []string) string {
	t.Helper()
	created, err := tokenSvc.Create(services.CreateTokenRequest{Name: "mcp", Permissions: perms}, userID)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	return created.Token
}

// mcpConnect dials the MCP HTTP endpoint with the given bearer token.
func mcpConnect(t *testing.T, ctx context.Context, endpoint, token string) *mcpsdk.ClientSession {
	t.Helper()
	transport := &mcpsdk.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           &http.Client{Transport: authRoundTripper{token: token, base: http.DefaultTransport}},
		DisableStandaloneSSE: true,
	}
	cs, err := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "0.0.1"}, nil).Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return cs
}

// mcpArticle is the subset of the article tool output the write tests assert on.
type mcpArticle struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
	Author string `json:"author"`
}

func decodeArticle(t *testing.T, structured any) mcpArticle {
	t.Helper()
	data, err := json.Marshal(structured)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var a mcpArticle
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return a
}

// TestMCPWriteTools covers the create/update/publish tools and their token-
// permission gating over the real HTTP transport.
func TestMCPWriteTools(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMCPTestDB(t)
	userA := createTestUserDB(t, db, "mcp-writer-a", "admin")
	userB := createTestUserDB(t, db, "mcp-writer-b", "admin")

	articleSvc := services.NewArticleService(db, "http://localhost:8080")
	articleSvc.SetSearchIndexer(services.NewBuiltinIndexer())
	tokenSvc := services.NewTokenService(db)

	r := gin.New()
	api := r.Group("/api/v1")
	mountMCPHTTP(api, mcp.Deps{
		Article:     articleSvc,
		ContentType: services.NewContentTypeService(db),
		BaseURL:     "http://localhost:8080",
	}, tokenSvc)
	ts := httptest.NewServer(r)
	defer ts.Close()
	endpoint := ts.URL + "/api/v1/mcp"

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	tokCreate := mkToken(t, tokenSvc, userA.ID, []string{"articles.create"})
	tokPublish := mkToken(t, tokenSvc, userA.ID, []string{"articles.publish"})
	tokNone := mkToken(t, tokenSvc, userA.ID, nil)
	tokEditB := mkToken(t, tokenSvc, userB.ID, []string{"articles.edit"})
	tokEditAllB := mkToken(t, tokenSvc, userB.ID, []string{"articles.edit", "articles.edit_all"})

	var draftID uint

	// create_article is saved as a draft, authored by the token's user.
	t.Run("create draft", func(t *testing.T) {
		cs := mcpConnect(t, ctx, endpoint, tokCreate)
		defer func() { _ = cs.Close() }()
		res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "create_article",
			Arguments: map[string]any{"title": "AI Draft", "content": "drafted by agent"},
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if res.IsError {
			t.Fatalf("create returned a tool error: %+v", res.Content)
		}
		a := decodeArticle(t, res.StructuredContent)
		if a.Status != "draft" {
			t.Errorf("status = %q, want draft", a.Status)
		}
		if a.ID == 0 {
			t.Fatal("expected a new article ID")
		}
		draftID = a.ID
	})

	// A token without articles.create cannot create.
	t.Run("create denied", func(t *testing.T) {
		cs := mcpConnect(t, ctx, endpoint, tokNone)
		defer func() { _ = cs.Close() }()
		res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "create_article",
			Arguments: map[string]any{"title": "nope"},
		})
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if !res.IsError {
			t.Error("expected IsError for a token without articles.create")
		}
	})

	// publish_article requires articles.publish.
	t.Run("publish permission", func(t *testing.T) {
		cs := mcpConnect(t, ctx, endpoint, tokCreate) // lacks publish
		res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "publish_article",
			Arguments: map[string]any{"id": draftID},
		})
		_ = cs.Close()
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if !res.IsError {
			t.Error("expected IsError publishing without articles.publish")
		}

		cs2 := mcpConnect(t, ctx, endpoint, tokPublish)
		defer func() { _ = cs2.Close() }()
		res2, err := cs2.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "publish_article",
			Arguments: map[string]any{"id": draftID},
		})
		if err != nil {
			t.Fatalf("publish: %v", err)
		}
		if res2.IsError {
			t.Fatalf("publish returned a tool error: %+v", res2.Content)
		}
		if a := decodeArticle(t, res2.StructuredContent); a.Status != "published" {
			t.Errorf("status = %q, want published", a.Status)
		}
	})

	// update ownership: userB editing userA's article needs articles.edit_all.
	t.Run("update ownership", func(t *testing.T) {
		csA := mcpConnect(t, ctx, endpoint, tokCreate)
		resC, err := csA.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "create_article",
			Arguments: map[string]any{"title": "owned by A"},
		})
		_ = csA.Close()
		if err != nil || resC.IsError {
			t.Fatalf("setup create failed: err=%v res=%+v", err, resC)
		}
		otherID := decodeArticle(t, resC.StructuredContent).ID

		// userB with only articles.edit (no edit_all) is denied on another's article.
		csB := mcpConnect(t, ctx, endpoint, tokEditB)
		resB, err := csB.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "update_article",
			Arguments: map[string]any{"id": otherID, "expected_version": 1, "title": "hijacked"},
		})
		_ = csB.Close()
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if !resB.IsError {
			t.Error("expected IsError updating another user's article without articles.edit_all")
		}

		// userB with articles.edit_all succeeds.
		csB2 := mcpConnect(t, ctx, endpoint, tokEditAllB)
		defer func() { _ = csB2.Close() }()
		resB2, err := csB2.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "update_article",
			Arguments: map[string]any{"id": otherID, "expected_version": 1, "title": "edited by editor"},
		})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if resB2.IsError {
			t.Fatalf("update with edit_all returned a tool error: %+v", resB2.Content)
		}
	})
}
