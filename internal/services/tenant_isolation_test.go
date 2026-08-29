package services

import (
	"context"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/cache"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

// ─── Tenant A/B Attack Matrix ──────────────────────────────────────────────
//
// These tests verify that no service-layer operation can cross tenant
// boundaries. For each attack vector (REST via service calls, Search, Cache,
// Webhook, Audit), we seed data in tenant A and tenant B, then attempt to
// read/write across the boundary.

const (
	tenantAID uint = 100
	tenantBID uint = 200
)

// setupTenantABDB creates a test DB with two tenants (A=100, B=200) and a
// user in each. Returns the DB plus the two users.
func setupTenantABDB(t *testing.T) (*gorm.DB, *models.User, *models.User) {
	t.Helper()
	db := setupTestDB(t)

	for _, tid := range []uint{tenantAID, tenantBID} {
		tenant := models.Tenant{
			Name:   "Tenant " + string(rune('A'+int(tid-100))),
			Slug:   "tenant-" + string(rune('a'+int(tid-100))),
			Status: models.TenantStatusActive,
		}
		tenant.ID = tid
		db.Create(&tenant)
	}

	userA := createTestUser(t, db, "user-a", "author")
	userB := createTestUser(t, db, "user-b", "author")

	db.Create(&models.TenantMembership{TenantID: tenantAID, UserID: userA.ID, RoleSlug: models.TenantRoleEditor})
	db.Create(&models.TenantMembership{TenantID: tenantBID, UserID: userB.ID, RoleSlug: models.TenantRoleEditor})

	return db, userA, userB
}

// createTenantArticle creates a published article scoped to tenantID.
func createTenantArticle(t *testing.T, db *gorm.DB, tenantID, authorID uint, title string) *models.Article {
	t.Helper()
	article := models.Article{
		Title:       title,
		Slug:        title,
		Content:     "<p>" + title + "</p>",
		AuthorID:    authorID,
		Status:      models.StatusPublished,
		PublishedAt: ptrTimeNow(),
		TenantID:    tenantID,
	}
	if err := db.Create(&article).Error; err != nil {
		t.Fatalf("create article: %v", err)
	}
	return &article
}

func ptrTimeNow() *time.Time {
	now := time.Now()
	return &now
}

// ─── REST: Article cross-tenant read ────────────────────────────────────────

func TestTenantIsolation_ArticleGet(t *testing.T) {
	db, userA, _ := setupTenantABDB(t)
	articleA := createTenantArticle(t, db, tenantAID, userA.ID, "Tenant A Article")

	svc := NewArticleService(db, "http://localhost:8080")

	// Tenant B cannot read tenant A's article by ID.
	_, err := svc.Get(articleA.ID, tenantBID)
	if err == nil {
		t.Fatal("tenant B should not read tenant A's article")
	}
}

func TestTenantIsolation_ArticleGetBySlug(t *testing.T) {
	db, userA, _ := setupTenantABDB(t)
	createTenantArticle(t, db, tenantAID, userA.ID, "unique-slug-a")

	svc := NewArticleService(db, "http://localhost:8080")

	// Tenant B cannot find tenant A's article by slug.
	_, err := svc.GetBySlug("unique-slug-a", tenantBID)
	if err == nil {
		t.Fatal("tenant B should not find tenant A's article by slug")
	}
}

func TestTenantIsolation_ArticleList(t *testing.T) {
	db, userA, userB := setupTenantABDB(t)
	createTenantArticle(t, db, tenantAID, userA.ID, "a-article-1")
	createTenantArticle(t, db, tenantAID, userA.ID, "a-article-2")
	createTenantArticle(t, db, tenantBID, userB.ID, "b-article-1")

	svc := NewArticleService(db, "http://localhost:8080")

	respA, err := svc.List(ListArticlesFilter{Page: 1, PageSize: 50}, tenantAID)
	if err != nil {
		t.Fatalf("list tenant A: %v", err)
	}
	itemsA, _ := respA.Items.([]models.Article)
	if len(itemsA) != 2 {
		t.Fatalf("tenant A should see 2 articles, got %d", len(itemsA))
	}

	respB, err := svc.List(ListArticlesFilter{Page: 1, PageSize: 50}, tenantBID)
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	itemsB, _ := respB.Items.([]models.Article)
	if len(itemsB) != 1 {
		t.Fatalf("tenant B should see 1 article, got %d", len(itemsB))
	}
}

func TestTenantIsolation_ArticleDelete(t *testing.T) {
	db, userA, _ := setupTenantABDB(t)
	articleA := createTenantArticle(t, db, tenantAID, userA.ID, "tenant-a-del")

	svc := NewArticleService(db, "http://localhost:8080")

	// Tenant B cannot delete tenant A's article.
	err := svc.Delete(articleA.ID, tenantBID, 0, false)
	if err == nil {
		t.Fatal("tenant B should not delete tenant A's article")
	}

	// Verify article still exists.
	var count int64
	db.Model(&models.Article{}).Where("id = ?", articleA.ID).Count(&count)
	if count != 1 {
		t.Fatal("article should still exist after cross-tenant delete attempt")
	}
}

func TestTenantIsolation_ArticleRevisions(t *testing.T) {
	db, userA, _ := setupTenantABDB(t)
	articleA := createTenantArticle(t, db, tenantAID, userA.ID, "rev-article")

	svc := NewArticleService(db, "http://localhost:8080")

	// Tenant B cannot list tenant A's article revisions.
	revisions, err := svc.Revisions(articleA.ID, tenantBID)
	if err != nil {
		t.Fatalf("revisions call should not error: %v", err)
	}
	if len(revisions) != 0 {
		t.Fatalf("tenant B should see 0 revisions, got %d", len(revisions))
	}
}

// ─── Search: cross-tenant search ────────────────────────────────────────────

func TestTenantIsolation_Search(t *testing.T) {
	db, userA, _ := setupTenantABDB(t)
	createTenantArticle(t, db, tenantAID, userA.ID, "searchable-a")

	svc := NewArticleService(db, "http://localhost:8080")
	indexer := &MockSearchIndexer{}
	svc.SetSearchIndexer(indexer)

	// Search from tenant B should not find tenant A's content.
	result, err := svc.Search(context.Background(), SearchQuery{
		Query:    "searchable",
		TenantID: tenantBID,
		Status:   "published",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, hit := range result.Hits {
		if hit.ID != 0 {
			t.Fatalf("tenant B search should return no hits from tenant A, got hit ID %d", hit.ID)
		}
	}
}

// ─── Cache: cross-tenant cache key isolation ────────────────────────────────

func TestTenantIsolation_CacheKeys(t *testing.T) {
	db, userA, userB := setupTenantABDB(t)
	createTenantArticle(t, db, tenantAID, userA.ID, "cached-a")
	createTenantArticle(t, db, tenantBID, userB.ID, "cached-b")

	svc := NewArticleService(db, "http://localhost:8080")
	memCache := cache.NewMemoryDriver(100)
	svc.WithCache(memCache, 10*time.Minute)

	// Prime cache for tenant A.
	respA, err := svc.List(ListArticlesFilter{Page: 1, PageSize: 50}, tenantAID)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	itemsA, _ := respA.Items.([]models.Article)
	if len(itemsA) != 1 || itemsA[0].Title != "cached-a" {
		t.Fatalf("tenant A should see only cached-a, got %+v", itemsA)
	}

	// Tenant B should NOT get tenant A's cached result.
	respB, err := svc.List(ListArticlesFilter{Page: 1, PageSize: 50}, tenantBID)
	if err != nil {
		t.Fatalf("list B: %v", err)
	}
	itemsB, _ := respB.Items.([]models.Article)
	if len(itemsB) != 1 || itemsB[0].Title != "cached-b" {
		t.Fatalf("tenant B should see only cached-b, got %+v", itemsB)
	}
}

// ─── Webhook: cross-tenant dispatch and log isolation ───────────────────────

func TestTenantIsolation_WebhookDispatch(t *testing.T) {
	db, _, _ := setupTenantABDB(t)

	// Create a webhook in tenant A.
	webhookSvc := NewWebhookService(db)
	whA, err := webhookSvc.Create(CreateWebhookRequest{
		Name:   "wh-a",
		URL:    "https://example.com/a",
		Events: []string{"entry.create"},
	}, tenantAID)
	if err != nil {
		t.Fatalf("create webhook A: %v", err)
	}

	// Tenant B should see no webhooks.
	webhooksB, err := webhookSvc.List(tenantBID)
	if err != nil {
		t.Fatalf("list webhooks B: %v", err)
	}
	if len(webhooksB) != 0 {
		t.Fatalf("tenant B should see 0 webhooks, got %d", len(webhooksB))
	}

	// Tenant B cannot get tenant A's webhook by ID.
	_, err = webhookSvc.Get(whA.ID, tenantBID)
	if err == nil {
		t.Fatal("tenant B should not get tenant A's webhook")
	}

	// Dispatch to tenant B should not trigger tenant A's webhook.
	webhookSvc.Dispatch("entry.create", nil, tenantBID)
	webhooksA, _ := webhookSvc.List(tenantAID)
	if len(webhooksA) != 1 {
		t.Fatalf("tenant A should still have 1 webhook, got %d", len(webhooksA))
	}
}

func TestTenantIsolation_WebhookDelete(t *testing.T) {
	db, _, _ := setupTenantABDB(t)

	webhookSvc := NewWebhookService(db)
	whA, err := webhookSvc.Create(CreateWebhookRequest{
		Name:   "wh-a-del",
		URL:    "https://example.com/del",
		Events: []string{"entry.create"},
	}, tenantAID)
	if err != nil {
		t.Fatalf("create webhook A: %v", err)
	}

	// Tenant B cannot delete tenant A's webhook.
	err = webhookSvc.Delete(whA.ID, tenantBID)
	if err == nil {
		t.Fatal("tenant B should not delete tenant A's webhook")
	}

	// Verify webhook still exists.
	webhooksA, _ := webhookSvc.List(tenantAID)
	if len(webhooksA) != 1 {
		t.Fatal("webhook should still exist after cross-tenant delete attempt")
	}
}

// ─── Settings: cross-tenant settings isolation ──────────────────────────────

func TestTenantIsolation_Settings(t *testing.T) {
	db, _, _ := setupTenantABDB(t)

	settingsSvc := NewSettingsService(db)

	// Tenant A sets a custom setting.
	err := settingsSvc.Update(map[string]interface{}{
		"site.title": "Tenant A Site",
	}, tenantAID)
	if err != nil {
		t.Fatalf("update settings A: %v", err)
	}

	// Tenant B should not see tenant A's setting.
	setting, err := settingsSvc.Get("site.title", tenantBID)
	if err == nil && setting != nil && setting.Value == "Tenant A Site" {
		t.Fatal("tenant B should not see tenant A's setting value")
	}

	// Tenant B should get its own (empty/default) setting.
	publicB, err := settingsSvc.PublicSettings(tenantBID)
	if err != nil {
		t.Fatalf("public settings B: %v", err)
	}
	if v, ok := publicB["site.title"]; ok && v == "Tenant A Site" {
		t.Fatal("tenant B public settings should not contain tenant A's value")
	}
}

// ─── Audit: cross-tenant audit log isolation ────────────────────────────────

func TestTenantIsolation_AuditLogs(t *testing.T) {
	db, userA, _ := setupTenantABDB(t)

	auditRepo := NewAuditLogger(repository.NewActivityLogRepository(db))
	articleSvc := NewArticleService(db, "http://localhost:8080")
	articleSvc.SetAuditLogger(auditRepo)

	// Create an article in tenant A (generates audit log).
	_, err := articleSvc.Create(CreateArticleRequest{
		Title:   "Audit Article A",
		Slug:    "audit-a",
		Content: "<p>Audit A</p>",
	}, tenantAID, userA.ID)
	if err != nil {
		t.Fatalf("create article A: %v", err)
	}

	// Verify audit log has tenant A's ID.
	var logs []models.ActivityLog
	db.Where("action = ?", "article.create").Find(&logs)
	if len(logs) == 0 {
		t.Fatal("expected at least 1 audit log entry")
	}
	for _, log := range logs {
		if log.TenantID == nil || *log.TenantID != tenantAID {
			t.Fatalf("audit log should have tenant A ID %d, got %v", tenantAID, log.TenantID)
		}
	}
}

// ─── Analytics: cross-tenant dashboard isolation ────────────────────────────

func TestTenantIsolation_Analytics(t *testing.T) {
	db, userA, userB := setupTenantABDB(t)
	createTenantArticle(t, db, tenantAID, userA.ID, "analytics-a")
	createTenantArticle(t, db, tenantBID, userB.ID, "analytics-b")

	analyticsSvc := NewAnalyticsService(db)

	dashA, err := analyticsSvc.Dashboard(tenantAID)
	if err != nil {
		t.Fatalf("dashboard A: %v", err)
	}
	if dashA.Stats.Articles != 1 {
		t.Fatalf("tenant A should have 1 article, got %d", dashA.Stats.Articles)
	}

	dashB, err := analyticsSvc.Dashboard(tenantBID)
	if err != nil {
		t.Fatalf("dashboard B: %v", err)
	}
	if dashB.Stats.Articles != 1 {
		t.Fatalf("tenant B should have 1 article, got %d", dashB.Stats.Articles)
	}
}

// ─── Category: cross-tenant taxonomy isolation ──────────────────────────────

func TestTenantIsolation_Category(t *testing.T) {
	db, _, _ := setupTenantABDB(t)

	catSvc := NewCategoryService(db)

	catA, err := catSvc.Create(CreateCategoryRequest{Name: "Category A"}, tenantAID)
	if err != nil {
		t.Fatalf("create category A: %v", err)
	}

	// Tenant B cannot get tenant A's category.
	_, err = catSvc.Get(catA.ID, tenantBID)
	if err == nil {
		t.Fatal("tenant B should not get tenant A's category")
	}

	// Tenant B list should be empty.
	catsB, err := catSvc.List(false, tenantBID)
	if err != nil {
		t.Fatalf("list categories B: %v", err)
	}
	if len(catsB) != 0 {
		t.Fatalf("tenant B should see 0 categories, got %d", len(catsB))
	}
}

// ─── Tag: cross-tenant tag isolation ────────────────────────────────────────

func TestTenantIsolation_Tag(t *testing.T) {
	db, _, _ := setupTenantABDB(t)

	tagSvc := NewTagService(db)

	tagA, err := tagSvc.Create(CreateTagRequest{Name: "tag-a"}, tenantAID)
	if err != nil {
		t.Fatalf("create tag A: %v", err)
	}

	// Tenant B cannot get tenant A's tag.
	_, err = tagSvc.Get(tagA.ID, tenantBID)
	if err == nil {
		t.Fatal("tenant B should not get tenant A's tag")
	}

	// Tenant B list should be empty.
	tagsB, _, err := tagSvc.List(TagListParams{}, tenantBID)
	if err != nil {
		t.Fatalf("list tags B: %v", err)
	}
	if len(tagsB) != 0 {
		t.Fatalf("tenant B should see 0 tags, got %d", len(tagsB))
	}
}

// ─── Comments: cross-tenant parent and thread rendering ────────────────────

func TestTenantIsolation_CommentParent(t *testing.T) {
	db, userA, userB := setupTenantABDB(t)

	articleA := createTenantArticle(t, db, tenantAID, userA.ID, "Article A")
	articleB := createTenantArticle(t, db, tenantBID, userB.ID, "Article B")

	commentSvc := NewCommentService(db)

	rootA, err := commentSvc.Create(CreateCommentRequest{
		ArticleID: articleA.ID,
		Content:   "root comment in tenant A",
	}, "127.0.0.1", "agent", &userA.ID, true, tenantAID)
	if err != nil {
		t.Fatalf("create root comment in tenant A: %v", err)
	}

	// Tenant B cannot reply to tenant A's comment.
	_, err = commentSvc.Create(CreateCommentRequest{
		ArticleID: articleB.ID,
		ParentID:  &rootA.ID,
		Content:   "cross-tenant reply from B",
	}, "127.0.0.1", "agent", nil, false, tenantBID)
	if err == nil {
		t.Fatal("tenant B should not reply to tenant A's comment")
	}

	// The rejected reply must not be persisted.
	var count int64
	db.Model(&models.Comment{}).Where("tenant_id = ?", tenantBID).Count(&count)
	if count != 0 {
		t.Fatalf("tenant B should have 0 comments, got %d", count)
	}
}

func TestTenantIsolation_CommentThread(t *testing.T) {
	db, userA, userB := setupTenantABDB(t)

	articleA := createTenantArticle(t, db, tenantAID, userA.ID, "Article A")
	articleB := createTenantArticle(t, db, tenantBID, userB.ID, "Article B")

	commentSvc := NewCommentService(db)

	rootA, err := commentSvc.Create(CreateCommentRequest{
		ArticleID: articleA.ID,
		Content:   "root comment in tenant A",
	}, "127.0.0.1", "agent", &userA.ID, true, tenantAID)
	if err != nil {
		t.Fatalf("create root comment in tenant A: %v", err)
	}

	// Simulate a legacy dangling reply written before the service rejected
	// cross-tenant parents: an approved tenant B row pointing at tenant A.
	childB := models.Comment{
		TenantID:  tenantBID,
		ArticleID: articleB.ID,
		ParentID:  &rootA.ID,
		Content:   "legacy cross-tenant child from B",
		Status:    "approved",
	}
	if err := db.Create(&childB).Error; err != nil {
		t.Fatalf("seed legacy child: %v", err)
	}

	// Tenant A's public thread must not render tenant B's child.
	threadA, err := commentSvc.ArticleComments(articleA.ID, tenantAID)
	if err != nil {
		t.Fatalf("article comments A: %v", err)
	}
	if len(threadA) != 1 || len(threadA[0].Children) != 0 {
		t.Fatalf("tenant A thread must show 1 root with 0 children, got %d roots / %d children",
			len(threadA), len(threadA[0].Children))
	}

	// Tenant B's thread stays empty as well (its child is not a root).
	threadB, err := commentSvc.ArticleComments(articleB.ID, tenantBID)
	if err != nil {
		t.Fatalf("article comments B: %v", err)
	}
	if len(threadB) != 0 {
		t.Fatalf("tenant B thread should be empty, got %d roots", len(threadB))
	}

	// Admin GetByID must not preload the cross-tenant child either.
	gotA, err := commentSvc.Get(rootA.ID, tenantAID)
	if err != nil {
		t.Fatalf("get comment A: %v", err)
	}
	if len(gotA.Children) != 0 {
		t.Fatalf("tenant A comment should preload 0 children, got %d", len(gotA.Children))
	}
}

func TestTenantIsolation_AuditLogReadBoundary(t *testing.T) {
	db, userA, _ := setupTenantABDB(t)

	auditRepo := NewAuditLogger(repository.NewActivityLogRepository(db))
	articleSvc := NewArticleService(db, "http://localhost:8080")
	articleSvc.SetAuditLogger(auditRepo)

	// Tenant A generates a business audit event.
	if _, err := articleSvc.Create(CreateArticleRequest{
		Title:   "Audit Boundary A",
		Slug:    "audit-boundary-a",
		Content: "<p>A</p>",
	}, tenantAID, userA.ID); err != nil {
		t.Fatalf("create article A: %v", err)
	}

	systemSvc := NewSystemService(db)

	// A tenant B viewer must not see tenant A's event.
	_, totalB, err := systemSvc.ActivityLog(ActivityLogParams{Page: 1, PageSize: 50, TenantID: tenantBID})
	if err != nil {
		t.Fatalf("tenant B activity log: %v", err)
	}
	if totalB != 0 {
		t.Fatalf("tenant B must see 0 audit logs, got %d", totalB)
	}

	// The platform-wide surface (platform admin only) still sees the event.
	_, totalAll, err := systemSvc.ActivityLog(ActivityLogParams{Page: 1, PageSize: 50, TenantID: 0})
	if err != nil {
		t.Fatalf("platform activity log: %v", err)
	}
	if totalAll < 1 {
		t.Fatalf("platform view should contain tenant A's event, got %d", totalAll)
	}
}
