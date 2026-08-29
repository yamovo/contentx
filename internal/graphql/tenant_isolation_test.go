package graphql

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/middleware"
	"github.com/yamovo/contentx/internal/models"
)

// ─── GraphQL Tenant A/B Attack Matrix ──────────────────────────────────────
//
// GraphQL is the read-only public surface, but authenticated requests resolve
// a tenant through OptionalAuthMiddleware. These tests drive the full HTTP
// handler (middleware → context injection → resolvers) and assert that no
// query combination can read another tenant's content.

const (
	gqlTenantAID uint = 100
	gqlTenantBID uint = 200
)

// setupTenantABGraph seeds two tenants with one editor each plus published
// (and one draft) article per tenant, and returns an engine wired with
// OptionalAuthMiddleware + the GraphQL handler, together with pre-minted
// access tokens for the tenant A editor, tenant B editor, and a platform admin.
func setupTenantABGraph(t *testing.T) (*gin.Engine, string, string, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	schema, db := setupTestSchema(t)

	for _, tid := range []uint{gqlTenantAID, gqlTenantBID} {
		tenant := models.Tenant{
			Name:   fmt.Sprintf("Tenant %d", tid),
			Slug:   fmt.Sprintf("tenant-%d", tid),
			Status: models.TenantStatusActive,
		}
		tenant.ID = tid
		if err := db.Create(&tenant).Error; err != nil {
			t.Fatalf("create tenant %d: %v", tid, err)
		}
	}

	editorA := models.User{
		BaseModel: models.BaseModel{ID: 101},
		Username:  "editor-a",
		Email:     "editor-a@example.com",
		Status:    models.UserStatusActive,
		RoleID:    2,
	}
	editorB := models.User{
		BaseModel: models.BaseModel{ID: 102},
		Username:  "editor-b",
		Email:     "editor-b@example.com",
		Status:    models.UserStatusActive,
		RoleID:    2,
	}
	if err := db.Create(&editorA).Error; err != nil {
		t.Fatalf("create editor A: %v", err)
	}
	if err := db.Create(&editorB).Error; err != nil {
		t.Fatalf("create editor B: %v", err)
	}
	if err := db.Create(&models.TenantMembership{TenantID: gqlTenantAID, UserID: editorA.ID, RoleSlug: models.TenantRoleEditor}).Error; err != nil {
		t.Fatalf("membership A: %v", err)
	}
	if err := db.Create(&models.TenantMembership{TenantID: gqlTenantBID, UserID: editorB.ID, RoleSlug: models.TenantRoleEditor}).Error; err != nil {
		t.Fatalf("membership B: %v", err)
	}

	// Roles: editors must NOT resolve to an admin global role (otherwise the
	// X-Tenant-ID override branch would legitimately apply to them), so the
	// admin role gets its own explicit row and ID.
	authorRole := models.Role{BaseModel: models.BaseModel{ID: 2}, Name: "Author", Slug: "author"}
	if err := db.Create(&authorRole).Error; err != nil {
		t.Fatalf("create author role: %v", err)
	}
	adminRole := models.Role{BaseModel: models.BaseModel{ID: 3}, Name: "Administrator", Slug: "admin"}
	if err := db.Create(&adminRole).Error; err != nil {
		t.Fatalf("create admin role: %v", err)
	}
	admin := models.User{
		BaseModel: models.BaseModel{ID: 103},
		Username:  "platform-admin",
		Email:     "platform-admin@example.com",
		Status:    models.UserStatusActive,
		RoleID:    adminRole.ID,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create platform admin: %v", err)
	}

	published := time.Now()
	articles := []models.Article{
		{BaseModel: models.BaseModel{ID: 201}, Title: "Alpha Public", Slug: "alpha-public",
			Content: "alpha", AuthorID: editorA.ID, Status: models.StatusPublished,
			PostType: models.PostTypePost, Visibility: models.VisibilityPublic,
			PublishedAt: &published, TenantID: gqlTenantAID},
		{BaseModel: models.BaseModel{ID: 202}, Title: "Alpha Draft", Slug: "alpha-draft",
			Content: "alpha draft", AuthorID: editorA.ID, Status: models.StatusDraft,
			PostType: models.PostTypePost, Visibility: models.VisibilityPublic,
			TenantID: gqlTenantAID},
		{BaseModel: models.BaseModel{ID: 301}, Title: "Beta Public", Slug: "beta-public",
			Content: "beta", AuthorID: editorB.ID, Status: models.StatusPublished,
			PostType: models.PostTypePost, Visibility: models.VisibilityPublic,
			PublishedAt: &published, TenantID: gqlTenantBID},
		{BaseModel: models.BaseModel{ID: 401}, Title: "Default Public", Slug: "default-public",
			Content: "default", AuthorID: 1, Status: models.StatusPublished,
			PostType: models.PostTypePost, Visibility: models.VisibilityPublic,
			PublishedAt: &published, TenantID: models.DefaultTenantID},
	}
	for i := range articles {
		if err := db.Create(&articles[i]).Error; err != nil {
			t.Fatalf("create article %s: %v", articles[i].Slug, err)
		}
	}

	jwtMgr := auth.NewJWTManager(config.JWTConfig{
		Secret:          "graphql-tenant-ab-test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		Issuer:          "contentx-test",
	})
	mustToken := func(userID, tenantID uint, username, email, role string) string {
		pair, err := jwtMgr.GenerateTokenPairWithTenant(userID, tenantID, username, email, role, username)
		if err != nil {
			t.Fatalf("mint token for %s: %v", username, err)
		}
		return pair.AccessToken
	}
	tokenA := mustToken(editorA.ID, gqlTenantAID, editorA.Username, editorA.Email, "author")
	tokenB := mustToken(editorB.ID, gqlTenantBID, editorB.Username, editorB.Email, "author")
	tokenAdmin := mustToken(admin.ID, gqlTenantAID, admin.Username, admin.Email, "admin")

	r := gin.New()
	r.POST("/graphql", middleware.OptionalAuthMiddleware(jwtMgr, db, nil), Handler(schema))
	return r, tokenA, tokenB, tokenAdmin
}

// postGraphQL sends a GraphQL query with an optional bearer token and tenant
// override header, returning the status code and parsed response body.
func postGraphQL(r *gin.Engine, token, tenantOverride, query string) (int, map[string]interface{}) {
	body := fmt.Sprintf(`{"query":%q}`, query)
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if tenantOverride != "" {
		req.Header.Set("X-Tenant-ID", tenantOverride)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp
}

// gqlArticleTotal returns the articles connection total from a response body.
func gqlArticleTotal(t *testing.T, resp map[string]interface{}) int {
	t.Helper()
	data, _ := resp["data"].(map[string]interface{})
	if data == nil {
		return -1
	}
	conn, _ := data["articles"].(map[string]interface{})
	if conn == nil {
		t.Fatalf("missing articles connection: %v", resp)
	}
	switch total := conn["total"].(type) {
	case int:
		return total
	case int64:
		return int(total)
	case float64:
		return int(total)
	default:
		t.Fatalf("unexpected total type %T: %v", conn["total"], conn["total"])
	}
	return -1
}

// gqlSlugVisible asserts that articleBySlug resolves (200 + data) or not.
func gqlSlugVisible(resp map[string]interface{}, slug string) bool {
	data, _ := resp["data"].(map[string]interface{})
	if data == nil {
		return false
	}
	article, _ := data["articleBySlug"].(map[string]interface{})
	return article != nil && article["slug"] == slug
}

func TestGraphQLTenantIsolation_AnonymousCannotForgeTenantHeader(t *testing.T) {
	r, _, _, _ := setupTenantABGraph(t)

	// Anonymous request pretending to be tenant B: no valid token means no
	// tenant resolution, so the query must stay on the default tenant.
	code, resp := postGraphQL(r, "", "200", `{ articleBySlug(slug: "beta-public") { id title slug } }`)
	if code == http.StatusOK && gqlSlugVisible(resp, "beta-public") {
		t.Fatal("anonymous request with forged X-Tenant-ID must not read tenant B content")
	}

	code, resp = postGraphQL(r, "", "200", `{ articles { total items { title } } }`)
	if code != http.StatusOK {
		t.Fatalf("default tenant list should succeed, got %d: %v", code, resp)
	}
	if total := gqlArticleTotal(t, resp); total != 2 {
		t.Fatalf("anonymous (default tenant) should see exactly 2 articles, got %d", total)
	}
}

func TestGraphQLTenantIsolation_TenantBScopesQueriesToB(t *testing.T) {
	r, _, tokenB, _ := setupTenantABGraph(t)

	// Tenant B sees its own published article.
	code, resp := postGraphQL(r, tokenB, "", `{ articleBySlug(slug: "beta-public") { id title slug } }`)
	if code != http.StatusOK || !gqlSlugVisible(resp, "beta-public") {
		t.Fatalf("tenant B should read its own article, got %d: %v", code, resp)
	}

	// Tenant B cannot read tenant A's published article by slug or the list.
	code, resp = postGraphQL(r, tokenB, "", `{ articleBySlug(slug: "alpha-public") { id title slug } }`)
	if code == http.StatusOK && gqlSlugVisible(resp, "alpha-public") {
		t.Fatal("tenant B must not read tenant A's article by slug")
	}

	code, resp = postGraphQL(r, tokenB, "", `{ articles { total items { id title } } }`)
	if code != http.StatusOK {
		t.Fatalf("tenant B list should succeed, got %d", code)
	}
	if total := gqlArticleTotal(t, resp); total != 1 {
		t.Fatalf("tenant B list should contain exactly its 1 article, got %d", total)
	}

	// Even tenant B's editor cannot pull the own-tenant draft: GraphQL is
	// published-only regardless of authentication.
	code, resp = postGraphQL(r, tokenB, "", `{ articleBySlug(slug: "alpha-draft") { id title slug } }`)
	if code == http.StatusOK && gqlSlugVisible(resp, "alpha-draft") {
		t.Fatal("GraphQL must never expose drafts, even to the owning tenant")
	}
}

func TestGraphQLTenantIsolation_TenantASeesOnlyOwnContent(t *testing.T) {
	r, tokenA, _, _ := setupTenantABGraph(t)

	code, resp := postGraphQL(r, tokenA, "", `{ articles { total items { id title } } }`)
	if code != http.StatusOK {
		t.Fatalf("tenant A list should succeed, got %d", code)
	}
	if total := gqlArticleTotal(t, resp); total != 1 {
		t.Fatalf("tenant A list should contain exactly its 1 published article, got %d", total)
	}

	code, resp = postGraphQL(r, tokenA, "", `{ articleBySlug(slug: "beta-public") { id title slug } }`)
	if code == http.StatusOK && gqlSlugVisible(resp, "beta-public") {
		t.Fatal("tenant A must not read tenant B's article")
	}
}

func TestGraphQLTenantIsolation_NonAdminTenantOverrideFailsClosed(t *testing.T) {
	r, tokenA, _, _ := setupTenantABGraph(t)

	// A tenant A editor attempting the admin-only tenant switch must not end
	// up with tenant B (or any cross-tenant) data. The middleware rejects the
	// override, the request degrades to the default tenant, and the response
	// stays a clean public read.
	code, resp := postGraphQL(r, tokenA, "200", `{ articles { total items { id title } } }`)
	if code != http.StatusOK {
		t.Fatalf("default fallback should still serve public content, got %d: %v", code, resp)
	}
	if total := gqlArticleTotal(t, resp); total != 2 {
		t.Fatalf("overridden request must see only default tenant content (2 articles), got %d", total)
	}

	code, resp = postGraphQL(r, tokenA, "200", `{ articleBySlug(slug: "beta-public") { id title slug } }`)
	if code == http.StatusOK && gqlSlugVisible(resp, "beta-public") {
		t.Fatal("non-admin tenant override must not grant tenant B reads")
	}
}

func TestGraphQLTenantIsolation_AdminSwitchScopesToSwitchedTenant(t *testing.T) {
	r, _, _, tokenAdmin := setupTenantABGraph(t)

	// The platform admin switches into tenant B and must see exactly B's
	// content — switching is scope selection, not a cross-tenant union.
	code, resp := postGraphQL(r, tokenAdmin, "200", `{ articles { total items { id title } } }`)
	if code != http.StatusOK {
		t.Fatalf("admin switch should succeed, got %d: %v", code, resp)
	}
	if total := gqlArticleTotal(t, resp); total != 1 {
		t.Fatalf("admin switched to tenant B should see exactly 1 article, got %d", total)
	}

	code, resp = postGraphQL(r, tokenAdmin, "200", `{ articleBySlug(slug: "alpha-public") { id title slug } }`)
	if code == http.StatusOK && gqlSlugVisible(resp, "alpha-public") {
		t.Fatal("admin switched to tenant B must not read tenant A content in the same request")
	}
}
