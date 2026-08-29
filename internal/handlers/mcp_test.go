package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/permissions"
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
	prepareHandlerTestDB(t, db)
	if err := database.Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	defaultTenant := &models.Tenant{
		BaseModel: models.BaseModel{ID: models.DefaultTenantID},
		Name:      "Default",
		Slug:      "default",
		Status:    models.TenantStatusActive,
	}
	if err := db.Where("id = ?", models.DefaultTenantID).FirstOrCreate(defaultTenant).Error; err != nil {
		t.Fatalf("create default tenant: %v", err)
	}
	return db
}

func grantMCPMembership(t *testing.T, db *gorm.DB, userID, tenantID uint, role string) {
	t.Helper()
	membership := &models.TenantMembership{
		TenantID: tenantID,
		UserID:   userID,
		RoleSlug: role,
	}
	if err := db.Create(membership).Error; err != nil {
		t.Fatalf("create MCP tenant membership: %v", err)
	}
}

// TestMCPTokenAuth verifies the API-token gate on the MCP HTTP endpoint.
func TestMCPTokenAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMCPTestDB(t)
	user := createTestUserDB(t, db, "mcp-token-user", "admin")
	grantMCPMembership(t, db, user.ID, models.DefaultTenantID, models.TenantRoleAdmin)
	tokenSvc := services.NewTokenService(db)
	created, err := tokenSvc.Create(services.CreateTokenRequest{
		Name:        "mcp",
		Permissions: []string{permissions.ArticlesRead},
	}, user.ID, models.DefaultTenantID)
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

func TestMCPTokenAuth_UsesVerifiedTokenTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMCPTestDB(t)
	user := createTestUserDB(t, db, "mcp-tenant-principal", "admin")
	tenantB := &models.Tenant{Name: "MCP Tenant B", Slug: "mcp-tenant-b", Status: models.TenantStatusActive}
	if err := db.Create(tenantB).Error; err != nil {
		t.Fatalf("create tenant B: %v", err)
	}
	grantMCPMembership(t, db, user.ID, tenantB.ID, models.TenantRoleAdmin)
	tokenSvc := services.NewTokenService(db)
	created, err := tokenSvc.Create(services.CreateTokenRequest{
		Name:        "tenant-b-mcp",
		Permissions: []string{permissions.ArticlesRead},
	}, user.ID, tenantB.ID)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	r := gin.New()
	r.GET("/guarded", mcpTokenAuth(tokenSvc), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"tenant_id": mcp.TenantFromContext(c.Request.Context())})
	})
	req := httptest.NewRequest(http.MethodGet, "/guarded", nil)
	req.Header.Set("Authorization", "Bearer "+created.Token)
	req.Header.Set("X-Tenant-ID", "1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body struct {
		TenantID uint `json:"tenant_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.TenantID != tenantB.ID {
		t.Fatalf("context tenant = %d, want token tenant %d", body.TenantID, tenantB.ID)
	}
}

func TestMCPAuthorizer_UsesEffectiveVerifiedPrincipal(t *testing.T) {
	db := setupMCPTestDB(t)
	user := createTestUserDB(t, db, "mcp-effective-principal", "admin")
	grantMCPMembership(t, db, user.ID, models.DefaultTenantID, models.TenantRoleMember)
	tokenSvc := services.NewTokenService(db)
	created, err := tokenSvc.Create(services.CreateTokenRequest{
		Name:        "effective-mcp",
		Permissions: []string{permissions.Wildcard},
	}, user.ID, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	header := make(http.Header)
	header.Set("Authorization", "Bearer "+created.Token)
	identity, err := (mcpAuthorizer{tokenSvc: tokenSvc}).Resolve(header)
	if err != nil {
		t.Fatalf("resolve MCP writer: %v", err)
	}
	if identity.UserID != user.ID || identity.TenantID != models.DefaultTenantID {
		t.Fatalf("writer identity = user %d tenant %d", identity.UserID, identity.TenantID)
	}
	if !permissions.Grants(identity.Permissions, permissions.ArticlesRead) {
		t.Fatal("expected member-readable article permission")
	}
	if permissions.Grants(identity.Permissions, permissions.ArticlesPublish) {
		t.Fatal("MCP writer must use the tenant-member permission ceiling")
	}
	if permissions.Grants(identity.Permissions, permissions.UsersDelete) {
		t.Fatal("MCP writer must not receive platform permissions")
	}

	if err := db.Where("tenant_id = ? AND user_id = ?", models.DefaultTenantID, user.ID).
		Delete(&models.TenantMembership{}).Error; err != nil {
		t.Fatalf("remove membership: %v", err)
	}
	if _, err := (mcpAuthorizer{tokenSvc: tokenSvc}).Resolve(header); err == nil {
		t.Fatal("MCP authorizer must reject a principal after membership removal")
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
	grantMCPMembership(t, db, user.ID, models.DefaultTenantID, models.TenantRoleAdmin)
	createTestArticleDB(t, db, user.ID, "Published One")

	articleSvc := services.NewArticleService(db, "http://localhost:8080")
	articleSvc.SetSearchIndexer(services.NewBuiltinIndexer())
	if _, err := articleSvc.ReindexAll(context.Background()); err != nil {
		t.Fatalf("reindex: %v", err)
	}
	tokenSvc := services.NewTokenService(db)
	created, err := tokenSvc.Create(services.CreateTokenRequest{
		Name:        "mcp",
		Permissions: []string{permissions.ArticlesRead},
	}, user.ID, models.DefaultTenantID)
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
	return mkTenantToken(t, tokenSvc, userID, models.DefaultTenantID, perms)
}

func mkTenantToken(t *testing.T, tokenSvc *services.TokenService, userID, tenantID uint, perms []string) string {
	t.Helper()
	created, err := tokenSvc.Create(services.CreateTokenRequest{Name: "mcp", Permissions: perms}, userID, tenantID)
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

func requireMCPToolDenied(t *testing.T, ctx context.Context, cs *mcpsdk.ClientSession, name string, args map[string]any) {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err == nil && res != nil && !res.IsError {
		t.Fatalf("%s unexpectedly succeeded: %+v", name, res.StructuredContent)
	}
}

// TestMCPReadPermissionsAndDrafts proves that HTTP read tools fail closed on
// empty or unrelated effective permissions. MCP_INCLUDE_DRAFTS is restricted
// to local stdio: even an articles.read HTTP token cannot expose drafts.
func TestMCPReadPermissionsAndDrafts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMCPTestDB(t)
	user := createTestUserDB(t, db, "mcp-read-guard", "admin")
	grantMCPMembership(t, db, user.ID, models.DefaultTenantID, models.TenantRoleAdmin)

	draft := &models.Article{
		TenantID: models.DefaultTenantID,
		Title:    "MCP Secret Draft",
		Slug:     "mcp-secret-draft",
		Content:  "draft body must require articles.read",
		AuthorID: user.ID,
		Status:   models.StatusDraft,
	}
	if err := db.Create(draft).Error; err != nil {
		t.Fatalf("create draft: %v", err)
	}
	published := createTestArticleDB(t, db, user.ID, "MCP Published Article")

	articleSvc := services.NewArticleService(db, "http://localhost:8080")
	articleSvc.SetSearchIndexer(services.NewBuiltinIndexer())
	tokenSvc := services.NewTokenService(db)
	r := gin.New()
	mountMCPHTTP(r.Group("/api/v1"), mcp.Deps{
		Article:       articleSvc,
		ContentType:   services.NewContentTypeService(db),
		BaseURL:       "http://localhost:8080",
		IncludeDrafts: true,
	}, tokenSvc)
	ts := httptest.NewServer(r)
	defer ts.Close()
	endpoint := ts.URL + "/api/v1/mcp"

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	empty := mcpConnect(t, ctx, endpoint, mkToken(t, tokenSvc, user.ID, nil))
	requireMCPToolDenied(t, ctx, empty, "search_content", map[string]any{"query": "draft"})
	_ = empty.Close()

	wrong := mcpConnect(t, ctx, endpoint, mkToken(t, tokenSvc, user.ID, []string{permissions.ContentTypesRead}))
	requireMCPToolDenied(t, ctx, wrong, "get_article", map[string]any{"id": draft.ID})
	if _, err := wrong.ReadResource(ctx, &mcpsdk.ReadResourceParams{URI: fmt.Sprintf("contentx://articles/%d", draft.ID)}); err == nil {
		t.Fatal("draft resource unexpectedly readable without articles.read")
	}
	_ = wrong.Close()

	reader := mcpConnect(t, ctx, endpoint, mkToken(t, tokenSvc, user.ID, []string{permissions.ArticlesRead}))
	defer func() { _ = reader.Close() }()
	requireMCPToolDenied(t, ctx, reader, "get_article", map[string]any{"id": draft.ID})
	if _, err := reader.ReadResource(ctx, &mcpsdk.ReadResourceParams{URI: fmt.Sprintf("contentx://articles/%d", draft.ID)}); err == nil {
		t.Fatal("articles.read HTTP token unexpectedly read a draft resource")
	}
	res, err := reader.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_article",
		Arguments: map[string]any{"id": published.ID},
	})
	if err != nil || res.IsError {
		t.Fatalf("authorized published read failed: err=%v result=%+v", err, res)
	}
	if got := decodeArticle(t, res.StructuredContent); got.Status != string(models.StatusPublished) {
		t.Fatalf("published status = %q, want %q", got.Status, models.StatusPublished)
	}
}

// TestMCPRAGPermissions keeps semantic search and potentially billable answer
// synthesis separate: ai.read can search but cannot ask, while ai.ask is
// required for rag_ask. Unrelated article permissions grant neither.
func TestMCPRAGPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMCPTestDB(t)
	user := createTestUserDB(t, db, "mcp-rag-guard", "admin")
	grantMCPMembership(t, db, user.ID, models.DefaultTenantID, models.TenantRoleAdmin)
	article := createTestArticleDB(t, db, user.ID, "Go Agent Security")
	article.TenantID = models.DefaultTenantID

	articleSvc := services.NewArticleService(db, "http://localhost:8080")
	articleSvc.SetSearchIndexer(services.NewBuiltinIndexer())
	ragSvc := services.NewRAGService(
		db,
		services.NewDummyEmbeddingProvider(128),
		services.NewMemoryVectorStore(),
		services.NewDummyLLM(),
		500, 100, 5, 0.01,
	)
	if err := ragSvc.IndexArticle(context.Background(), article); err != nil {
		t.Fatalf("index RAG article: %v", err)
	}

	tokenSvc := services.NewTokenService(db)
	r := gin.New()
	mountMCPHTTP(r.Group("/api/v1"), mcp.Deps{
		Article:     articleSvc,
		ContentType: services.NewContentTypeService(db),
		RAG:         ragSvc,
		BaseURL:     "http://localhost:8080",
	}, tokenSvc)
	ts := httptest.NewServer(r)
	defer ts.Close()
	endpoint := ts.URL + "/api/v1/mcp"

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	articleOnly := mcpConnect(t, ctx, endpoint, mkToken(t, tokenSvc, user.ID, []string{permissions.ArticlesRead}))
	requireMCPToolDenied(t, ctx, articleOnly, "rag_search", map[string]any{"query": "Go"})
	_ = articleOnly.Close()

	searcher := mcpConnect(t, ctx, endpoint, mkToken(t, tokenSvc, user.ID, []string{permissions.AIRead}))
	searchRes, err := searcher.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "rag_search",
		Arguments: map[string]any{"query": "Go programming"},
	})
	if err != nil || searchRes.IsError {
		t.Fatalf("ai.read rag_search failed: err=%v result=%+v", err, searchRes)
	}
	requireMCPToolDenied(t, ctx, searcher, "rag_ask", map[string]any{"query": "What is Go?"})
	_ = searcher.Close()

	asker := mcpConnect(t, ctx, endpoint, mkToken(t, tokenSvc, user.ID, []string{permissions.AIAsk}))
	defer func() { _ = asker.Close() }()
	askRes, err := asker.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "rag_ask",
		Arguments: map[string]any{"query": "What is Go?"},
	})
	if err != nil || askRes.IsError {
		t.Fatalf("ai.ask rag_ask failed: err=%v result=%+v", err, askRes)
	}
}

// TestMCPResourcesAreTenantScopedAndDynamic covers the SDK resources/list
// blind spot end-to-end. Each HTTP request discovers schemas from the token's
// verified tenant, refreshes the catalogue, and rejects callers that lack the
// resource-specific read permission.
func TestMCPResourcesAreTenantScopedAndDynamic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMCPTestDB(t)
	user := createTestUserDB(t, db, "mcp-resource-tenant", "admin")
	tenantA := &models.Tenant{Name: "MCP Tenant A", Slug: "mcp-resource-a", Status: models.TenantStatusActive}
	tenantB := &models.Tenant{Name: "MCP Tenant B", Slug: "mcp-resource-b", Status: models.TenantStatusActive}
	if err := db.Create(tenantA).Error; err != nil {
		t.Fatalf("create tenant A: %v", err)
	}
	if err := db.Create(tenantB).Error; err != nil {
		t.Fatalf("create tenant B: %v", err)
	}
	grantMCPMembership(t, db, user.ID, tenantA.ID, models.TenantRoleAdmin)
	grantMCPMembership(t, db, user.ID, tenantB.ID, models.TenantRoleAdmin)

	ctSvc := services.NewContentTypeService(db)
	createType := func(tenantID uint, uid, name, description string) {
		t.Helper()
		_, err := ctSvc.CreateContentType(services.CreateContentTypeRequest{
			UID: uid, Name: name, Description: description,
			Fields: []services.CreateFieldRequest{{Name: "title", Label: "Title", FieldType: models.FieldTypeText}},
		}, tenantID)
		if err != nil {
			t.Fatalf("create content type %s: %v", uid, err)
		}
	}
	createType(tenantA.ID, "tenant_a_schema", "Tenant A Secret", "A-only metadata")
	createType(tenantB.ID, "tenant_b_schema", "Tenant B Secret", "B-only metadata")

	tokenSvc := services.NewTokenService(db)
	r := gin.New()
	mountMCPHTTP(r.Group("/api/v1"), mcp.Deps{
		Article:     services.NewArticleService(db, "http://localhost:8080"),
		ContentType: ctSvc,
		BaseURL:     "http://localhost:8080",
	}, tokenSvc)
	ts := httptest.NewServer(r)
	defer ts.Close()
	endpoint := ts.URL + "/api/v1/mcp"
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	listText := func(cs *mcpsdk.ClientSession) string {
		t.Helper()
		listed, err := cs.ListResources(ctx, nil)
		if err != nil {
			t.Fatalf("list resources: %v", err)
		}
		var values []string
		for _, resource := range listed.Resources {
			values = append(values, resource.URI, resource.Name, resource.Title, resource.Description)
		}
		return strings.Join(values, "\n")
	}

	tokenA := mkTenantToken(t, tokenSvc, user.ID, tenantA.ID, []string{permissions.ContentTypesRead})
	tokenB := mkTenantToken(t, tokenSvc, user.ID, tenantB.ID, []string{permissions.ContentTypesRead})
	clientA := mcpConnect(t, ctx, endpoint, tokenA)
	defer func() { _ = clientA.Close() }()
	clientB := mcpConnect(t, ctx, endpoint, tokenB)
	defer func() { _ = clientB.Close() }()

	aList := listText(clientA)
	if !strings.Contains(aList, "tenant_a_schema") || !strings.Contains(aList, "Tenant A Secret") {
		t.Fatalf("tenant A metadata missing: %s", aList)
	}
	if strings.Contains(aList, "tenant_b_schema") || strings.Contains(aList, "Tenant B Secret") {
		t.Fatalf("tenant B metadata leaked into tenant A list: %s", aList)
	}
	bList := listText(clientB)
	if !strings.Contains(bList, "tenant_b_schema") || !strings.Contains(bList, "Tenant B Secret") {
		t.Fatalf("tenant B metadata missing: %s", bList)
	}
	if strings.Contains(bList, "tenant_a_schema") || strings.Contains(bList, "Tenant A Secret") {
		t.Fatalf("tenant A metadata leaked into tenant B list: %s", bList)
	}

	createType(tenantA.ID, "tenant_a_fresh", "Tenant A Fresh", "registered after connect")
	if refreshed := listText(clientA); !strings.Contains(refreshed, "tenant_a_fresh") {
		t.Fatalf("resources/list did not refresh after schema creation: %s", refreshed)
	}
	if _, err := clientB.ReadResource(ctx, &mcpsdk.ReadResourceParams{URI: "contentx://content-types/tenant_a_schema"}); err == nil {
		t.Fatal("tenant B unexpectedly read tenant A content-type resource")
	}

	noResourcePerm := mcpConnect(t, ctx, endpoint, mkTenantToken(t, tokenSvc, user.ID, tenantA.ID, nil))
	defer func() { _ = noResourcePerm.Close() }()
	if _, err := noResourcePerm.ListResources(ctx, nil); err == nil {
		t.Fatal("resources/list unexpectedly succeeded without content_types.read")
	}
	if _, err := noResourcePerm.ListResourceTemplates(ctx, nil); err == nil {
		t.Fatal("resources/templates/list unexpectedly succeeded without articles.read")
	}
}

// TestMCPWriteTools covers the create/update/publish tools and their token-
// permission gating over the real HTTP transport.
func TestMCPWriteTools(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupMCPTestDB(t)
	userA := createTestUserDB(t, db, "mcp-writer-a", "admin")
	userB := createTestUserDB(t, db, "mcp-writer-b", "admin")
	grantMCPMembership(t, db, userA.ID, models.DefaultTenantID, models.TenantRoleAdmin)
	grantMCPMembership(t, db, userB.ID, models.DefaultTenantID, models.TenantRoleAdmin)

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
