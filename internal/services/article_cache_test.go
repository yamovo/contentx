package services

import (
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/cache"
)

func TestArticleCache_ListHitAndInvalidate(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	svc.WithCache(cache.NewMemoryDriver(1000), 5*time.Minute)
	user := createTestUser(t, db, "cache-author", "author")
	createTestArticle(t, db, user.ID, "Cached Article")

	// First list: DB hit, result cached.
	r1, err := svc.List(ListArticlesFilter{Status: "published", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if r1.Total != 1 {
		t.Fatalf("total = %d, want 1", r1.Total)
	}

	// Create a new published article (invalidates list cache).
	_, err = svc.Create(CreateArticleRequest{Title: "New One", Status: "published"}, user.ID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// List again: should see 2 articles (cache was invalidated by Create).
	r2, err := svc.List(ListArticlesFilter{Status: "published", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List after create: %v", err)
	}
	if r2.Total != 2 {
		t.Fatalf("after create, total = %d, want 2", r2.Total)
	}
}

func TestArticleCache_GetHitAndInvalidate(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	svc.WithCache(cache.NewMemoryDriver(1000), 5*time.Minute)
	user := createTestUser(t, db, "cache-author2", "author")
	article := createTestArticle(t, db, user.ID, "To Cache")

	// First Get: from DB, result cached.
	a1, err := svc.Get(article.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if a1.Title != "To Cache" {
		t.Fatalf("title = %q, want To Cache", a1.Title)
	}

	// Update the title (invalidates detail cache for this ID).
	newTitle := "Updated Title"
	_, err = svc.Update(article.ID, UpdateArticleRequest{Title: &newTitle}, user.ID, true)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Get again: should see updated title (cache invalidated, re-fetches from DB).
	a2, err := svc.Get(article.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if a2.Title != "Updated Title" {
		t.Fatalf("after update, title = %q, want Updated Title", a2.Title)
	}
}
