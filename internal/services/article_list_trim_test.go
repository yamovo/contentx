package services

import (
	"context"
	"testing"

	"github.com/yamovo/contentx/internal/models"
)

// TestArticleService_ListOmitsContentByDefault verifies the list payload
// trimming optimization: List drops the heavy Content field unless Full is set.
// See reports/benchmarks/postgres-baseline.md §4 (list body dominates P95/P99).
func TestArticleService_ListOmitsContentByDefault(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "trim-author", "author")
	createTestArticle(t, db, user.ID, "trim-me")

	svc := NewArticleService(db, "http://localhost:8080")

	// Default (Full=false): Content omitted, other fields intact.
	resp, err := svc.List(ListArticlesFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	items, ok := resp.Items.([]models.Article)
	if !ok || len(items) == 0 {
		t.Fatalf("expected at least one article, got %#v", resp.Items)
	}
	for _, a := range items {
		if a.Content != "" {
			t.Errorf("default List should omit Content, got %q for %q", a.Content, a.Title)
		}
		if a.Title == "" {
			t.Errorf("List should still return Title")
		}
	}

	// Full=true: Content present.
	full, err := svc.List(ListArticlesFilter{Page: 1, PageSize: 10, Full: true})
	if err != nil {
		t.Fatalf("List full: %v", err)
	}
	fullItems, _ := full.Items.([]models.Article)
	if len(fullItems) == 0 || fullItems[0].Content == "" {
		t.Fatalf("Full=true List should include Content, got %#v", fullItems)
	}
}

// TestArticleService_ReindexStillGetsContent guards against the trimming
// optimization starving the search index of article bodies: ReindexAll pulls
// through List and must request Full content.
func TestArticleService_ReindexStillGetsContent(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "reindex-content-author", "author")
	createTestArticle(t, db, user.ID, "indexed-body")

	svc := NewArticleService(db, "http://localhost:8080")
	indexer := &MockSearchIndexer{}
	svc.SetSearchIndexer(indexer)

	if _, err := svc.ReindexAll(context.Background()); err != nil {
		t.Fatalf("ReindexAll: %v", err)
	}
	if len(indexer.Reindexed) == 0 {
		t.Fatalf("expected reindexed articles")
	}
	for _, a := range indexer.Reindexed {
		if a.Content == "" {
			t.Fatalf("reindexed article %q missing Content — search would break", a.Title)
		}
	}
}
