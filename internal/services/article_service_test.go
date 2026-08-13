package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/cache"
	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
)

func TestArticleService_ReindexAllPaginatesPastPublicLimit(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "reindex-author", "author")

	articles := make([]models.Article, 250)
	for i := range articles {
		articles[i] = models.Article{
			Title:    fmt.Sprintf("Reindex article %03d", i+1),
			Slug:     fmt.Sprintf("reindex-article-%03d", i+1),
			Content:  "benchmark content",
			AuthorID: user.ID,
			Status:   models.StatusPublished,
		}
	}
	if err := db.CreateInBatches(&articles, 100).Error; err != nil {
		t.Fatalf("seed articles: %v", err)
	}

	svc := NewArticleService(db, "http://localhost:8080")
	indexer := &MockSearchIndexer{}
	svc.SetSearchIndexer(indexer)

	indexed, err := svc.ReindexAll(context.Background())
	if err != nil {
		t.Fatalf("ReindexAll: %v", err)
	}
	if indexed != 250 {
		t.Fatalf("indexed = %d, want 250", indexed)
	}
	if len(indexer.Reindexed) != 250 {
		t.Fatalf("reindexed documents = %d, want 250", len(indexer.Reindexed))
	}
}

func TestArticleService_ReindexAllBypassesStaleListCache(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "reindex-cache-author", "author")

	svc := NewArticleService(db, "http://localhost:8080")
	svc.WithCache(cache.NewMemoryDriver(100), 10*time.Minute)
	indexer := &MockSearchIndexer{}
	svc.SetSearchIndexer(indexer)

	// Cache the empty repository result under the same filter the old reindex
	// path used, then insert an article directly to simulate a database restore.
	if _, err := svc.List(ListArticlesFilter{
		Page: 1, PageSize: 100, Sort: "oldest", Full: true,
	}, models.DefaultTenantID); err != nil {
		t.Fatalf("prime stale list cache: %v", err)
	}
	createTestArticle(t, db, user.ID, "Restored article")

	indexed, err := svc.ReindexAll(context.Background())
	if err != nil {
		t.Fatalf("ReindexAll: %v", err)
	}
	if indexed != 1 || len(indexer.Reindexed) != 1 {
		t.Fatalf("reindex used stale cache: indexed=%d docs=%d", indexed, len(indexer.Reindexed))
	}
}

func TestArticleService_ReindexAllClearsIndexWhenRepositoryIsEmpty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	indexer := &MockSearchIndexer{
		Reindexed: []models.Article{{Title: "stale indexed article"}},
	}
	svc.SetSearchIndexer(indexer)

	indexed, err := svc.ReindexAll(context.Background())
	if err != nil {
		t.Fatalf("ReindexAll: %v", err)
	}
	if indexed != 0 {
		t.Fatalf("indexed = %d, want 0", indexed)
	}
	if len(indexer.Reindexed) != 0 {
		t.Fatalf("empty repository did not clear stale index: %+v", indexer.Reindexed)
	}
}

func TestArticleService_Create(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	req := CreateArticleRequest{
		Title:   "Hello World",
		Content: "<p>This is a test article with enough content to calculate reading time.</p>",
		Status:  "published",
	}

	article, err := svc.Create(req, models.DefaultTenantID, user.ID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if article.Title != "Hello World" {
		t.Errorf("Title = %q, want %q", article.Title, "Hello World")
	}
	if article.Slug == "" {
		t.Error("Slug should not be empty")
	}
	if article.AuthorID != user.ID {
		t.Errorf("AuthorID = %d, want %d", article.AuthorID, user.ID)
	}
	if article.Status != models.StatusDraft {
		t.Errorf("Status = %q, want %q", article.Status, models.StatusDraft)
	}
	if article.PublishedAt != nil {
		t.Error("PublishedAt must remain nil until the dedicated publish operation succeeds")
	}
	if article.ReadingTime < 1 {
		t.Error("ReadingTime should be >= 1")
	}
}

func TestArticleService_Update_IgnoresLifecycleAndPostTypeFields(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "immutable-fields-author", "author")
	article := createTestArticle(t, db, user.ID, "Immutable Fields")

	status := string(models.StatusArchived)
	postType := string(models.PostTypePage)
	now := time.Now()
	updated, err := svc.Update(article.ID, UpdateArticleRequest{
		Status:       &status,
		PostType:     &postType,
		PublishedAt:  &now,
		ScheduledAt:  &now,
		RevisionNote: "attempt lifecycle bypass",
	}, models.DefaultTenantID, user.ID, false)
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if updated.Status != article.Status {
		t.Errorf("status changed through ordinary update: %q", updated.Status)
	}
	if updated.PostType != article.PostType {
		t.Errorf("post_type changed through ordinary update: %q", updated.PostType)
	}
	if !sameTime(updated.PublishedAt, article.PublishedAt) || !sameTime(updated.ScheduledAt, article.ScheduledAt) {
		t.Error("lifecycle timestamps changed through ordinary update")
	}
}

func sameTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Equal(*right)
}

func TestArticleService_Create_SlugGeneration(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	// Create two articles with the same title.
	req := CreateArticleRequest{Title: "Same Title", Status: "draft"}
	a1, err := svc.Create(req, models.DefaultTenantID, user.ID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	a2, err := svc.Create(req, models.DefaultTenantID, user.ID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if a1.Slug == a2.Slug {
		t.Errorf("Slugs should be unique: both got %q", a1.Slug)
	}
}

func TestArticleService_Create_WithTags(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	// Create tags first.
	tagSvc := NewTagService(db)
	tag1, _ := tagSvc.Create(CreateTagRequest{Name: "Go"}, models.DefaultTenantID)
	tag2, _ := tagSvc.Create(CreateTagRequest{Name: "Testing"}, models.DefaultTenantID)

	req := CreateArticleRequest{
		Title:  "Tagged Article",
		TagIDs: []uint{tag1.ID, tag2.ID},
		Status: "draft",
	}

	article, err := svc.Create(req, models.DefaultTenantID, user.ID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if len(article.Tags) != 2 {
		t.Errorf("Tags count = %d, want 2", len(article.Tags))
	}
}

func TestArticleService_Get(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")
	article := createTestArticle(t, db, user.ID, "Test Article")

	got, err := svc.Get(article.ID, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Title != "Test Article" {
		t.Errorf("Title = %q, want %q", got.Title, "Test Article")
	}
	if got.Author.ID != user.ID {
		t.Error("Author should be preloaded")
	}
}

func TestArticleService_Get_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")

	_, err := svc.Get(9999, models.DefaultTenantID)
	if err == nil {
		t.Error("Get() should return error for non-existent article")
	}
}

func TestArticleService_List_FilterByStatus(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	createTestArticle(t, db, user.ID, "Published 1")
	createTestArticle(t, db, user.ID, "Published 2")

	// Create a draft.
	draft := CreateArticleRequest{Title: "Draft 1", Status: "draft"}
	svc.Create(draft, models.DefaultTenantID, user.ID)

	result, err := svc.List(ListArticlesFilter{Status: "published", Page: 1, PageSize: 20, Sort: "newest"}, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
}

func TestArticleService_Update(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")
	article := createTestArticle(t, db, user.ID, "Original Title")

	newTitle := "Updated Title"
	updated, err := svc.Update(article.ID, UpdateArticleRequest{Title: &newTitle}, models.DefaultTenantID, user.ID, false)
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", updated.Title, "Updated Title")
	}

	// Verify revision was created.
	revisions, _ := svc.Revisions(article.ID, models.DefaultTenantID)
	if len(revisions) < 2 {
		t.Errorf("Expected >= 2 revisions, got %d", len(revisions))
	}
}

func TestArticleService_Update_Forbidden(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	author := createTestUser(t, db, "author1", "author")
	other := createTestUser(t, db, "author2", "author")
	article := createTestArticle(t, db, author.ID, "My Article")

	newTitle := "Hacked"
	_, err := svc.Update(article.ID, UpdateArticleRequest{Title: &newTitle}, models.DefaultTenantID, other.ID, false)
	if err == nil {
		t.Error("Update() should return forbidden for non-owner, non-editor")
	}
}

func TestArticleService_Update_OptimisticLock_Conflict(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	author := createTestUser(t, db, "lock-author", "author")
	article := createTestArticle(t, db, author.ID, "Lock Test")

	// 第一次更新：传入正确的 version=1 → 成功。
	v1 := 1
	firstTitle := "Saved by Editor A"
	_, err := svc.Update(article.ID, UpdateArticleRequest{
		Title:           &firstTitle,
		ExpectedVersion: &v1,
	}, models.DefaultTenantID, author.ID, true)
	if err != nil {
		t.Fatalf("first update should succeed: %v", err)
	}

	// 第二次更新：用过期的 version=1 → 返回 ErrConcurrentModification。
	staleTitle := "Stale edit by Editor B"
	_, err = svc.Update(article.ID, UpdateArticleRequest{
		Title:           &staleTitle,
		ExpectedVersion: &v1, // 过期的 version
	}, models.DefaultTenantID, author.ID, true)
	if err == nil {
		t.Fatal("expected ErrConcurrentModification, got nil")
	}
	if !errors.Is(err, errs.ErrConcurrentModification) {
		t.Fatalf("expected ErrConcurrentModification, got %v", err)
	}
}

func TestArticleService_Update_OptimisticLock_CoversTagOnlyChanges(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	author := createTestUser(t, db, "tag-lock-author", "author")
	article := createTestArticle(t, db, author.ID, "Tag Lock Test")

	v1 := article.Version
	if _, err := svc.Update(article.ID, UpdateArticleRequest{
		TagIDs:          []uint{},
		ExpectedVersion: &v1,
	}, models.DefaultTenantID, author.ID, true); err != nil {
		t.Fatalf("first tag-only update should succeed: %v", err)
	}
	if _, err := svc.Update(article.ID, UpdateArticleRequest{
		TagIDs:          []uint{},
		ExpectedVersion: &v1,
	}, models.DefaultTenantID, author.ID, true); !errors.Is(err, errs.ErrConcurrentModification) {
		t.Fatalf("stale tag-only update = %v, want concurrent modification", err)
	}
}

func TestArticleService_Delete(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")
	article := createTestArticle(t, db, user.ID, "To Delete")

	if err := svc.Delete(article.ID, models.DefaultTenantID, user.ID, false); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Verify soft-deleted.
	_, err := svc.Get(article.ID, models.DefaultTenantID)
	if err == nil {
		t.Error("Get() should fail for deleted article")
	}
}

func TestArticleService_BulkAction(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	a1 := createTestArticle(t, db, user.ID, "Bulk 1")
	a2 := createTestArticle(t, db, user.ID, "Bulk 2")

	// Publish both.
	affected, err := svc.BulkAction(BulkActionRequest{
		ArticleIDs: []uint{a1.ID, a2.ID},
		Action:     "publish",
	}, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("BulkAction(publish) error: %v", err)
	}
	if affected != 2 {
		t.Errorf("Affected = %d, want 2", affected)
	}
}

func TestArticleService_GenerateFeed(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://example.com")
	user := createTestUser(t, db, "author1", "author")

	createTestArticle(t, db, user.ID, "Feed Article 1")
	createTestArticle(t, db, user.ID, "Feed Article 2")

	feed, err := svc.GenerateFeed(models.DefaultTenantID)
	if err != nil {
		t.Fatalf("GenerateFeed() error: %v", err)
	}

	if feed == "" {
		t.Error("Feed should not be empty")
	}
	if !contains(feed, "<rss") {
		t.Error("Feed should contain <rss tag")
	}
	if !contains(feed, "http://example.com") {
		t.Error("Feed should use baseURL, not localhost")
	}
}

func TestArticleService_LikeArticle(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")
	article := createTestArticle(t, db, user.ID, "Likeable")

	if err := svc.LikeArticle(article.ID, models.DefaultTenantID); err != nil {
		t.Fatalf("LikeArticle() error: %v", err)
	}

	got, _ := svc.Get(article.ID, models.DefaultTenantID)
	if got.LikeCount != 1 {
		t.Errorf("LikeCount = %d, want 1", got.LikeCount)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
