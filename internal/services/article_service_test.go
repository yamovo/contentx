package services

import (
	"context"
	"fmt"
	"testing"

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

func TestArticleService_Create(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	req := CreateArticleRequest{
		Title:   "Hello World",
		Content: "<p>This is a test article with enough content to calculate reading time.</p>",
		Status:  "published",
	}

	article, err := svc.Create(req, user.ID)
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
	if article.Status != "published" {
		t.Errorf("Status = %q, want %q", article.Status, "published")
	}
	if article.PublishedAt == nil {
		t.Error("PublishedAt should be set for published articles")
	}
	if article.ReadingTime < 1 {
		t.Error("ReadingTime should be >= 1")
	}
}

func TestArticleService_Create_SlugGeneration(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	// Create two articles with the same title.
	req := CreateArticleRequest{Title: "Same Title", Status: "draft"}
	a1, err := svc.Create(req, user.ID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	a2, err := svc.Create(req, user.ID)
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
	tag1, _ := tagSvc.Create(CreateTagRequest{Name: "Go"})
	tag2, _ := tagSvc.Create(CreateTagRequest{Name: "Testing"})

	req := CreateArticleRequest{
		Title:  "Tagged Article",
		TagIDs: []uint{tag1.ID, tag2.ID},
		Status: "draft",
	}

	article, err := svc.Create(req, user.ID)
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

	got, err := svc.Get(article.ID)
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

	_, err := svc.Get(9999)
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
	svc.Create(draft, user.ID)

	result, err := svc.List(ListArticlesFilter{Status: "published", Page: 1, PageSize: 20, Sort: "newest"})
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
	updated, err := svc.Update(article.ID, UpdateArticleRequest{Title: &newTitle}, user.ID, false)
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", updated.Title, "Updated Title")
	}

	// Verify revision was created.
	revisions, _ := svc.Revisions(article.ID)
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
	_, err := svc.Update(article.ID, UpdateArticleRequest{Title: &newTitle}, other.ID, false)
	if err == nil {
		t.Error("Update() should return forbidden for non-owner, non-editor")
	}
}

func TestArticleService_Delete(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")
	article := createTestArticle(t, db, user.ID, "To Delete")

	if err := svc.Delete(article.ID, user.ID, false); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Verify soft-deleted.
	_, err := svc.Get(article.ID)
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
	})
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

	feed, err := svc.GenerateFeed()
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

	if err := svc.LikeArticle(article.ID); err != nil {
		t.Fatalf("LikeArticle() error: %v", err)
	}

	got, _ := svc.Get(article.ID)
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
