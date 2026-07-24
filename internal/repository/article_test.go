package repository

import (
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

func TestArticleRepository_Create_WithTagsAndRevision(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author1", "author")
	tag1 := createTestTag(t, db, "Go", "go")
	tag2 := createTestTag(t, db, "Testing", "testing")
	cat := createTestCategory(t, db, "Tech", "tech")

	article := &models.Article{
		Title:      "Test Article",
		Slug:       "test-article",
		Content:    "<p>Hello</p>",
		AuthorID:   author.ID,
		Status:     models.StatusDraft,
		CategoryID: &cat.ID,
	}
	if err := repo.Create(article, []uint{tag1.ID, tag2.ID}, "initial", author.ID); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if article.ID == 0 {
		t.Fatal("article ID should be set after Create")
	}
	if len(article.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(article.Tags))
	}
	if article.Category == nil || article.Category.ID != cat.ID {
		t.Fatalf("category not preloaded")
	}

	// Verify initial revision was created.
	revisions, err := repo.ListRevisions(article.ID)
	if err != nil {
		t.Fatalf("ListRevisions: %v", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("expected 1 revision, got %d", len(revisions))
	}
	if revisions[0].Version != 1 {
		t.Fatalf("expected version 1, got %d", revisions[0].Version)
	}
	if revisions[0].Note != "initial" {
		t.Fatalf("expected note 'initial', got %q", revisions[0].Note)
	}

	// Verify tag counts were bumped.
	for _, tag := range article.Tags {
		var got models.Tag
		db.First(&got, tag.ID)
		if got.Count != 1 {
			t.Fatalf("tag %q count should be 1, got %d", tag.Name, got.Count)
		}
	}

	// Verify category post_count was bumped.
	var gotCat models.Category
	db.First(&gotCat, cat.ID)
	if gotCat.PostCount != 1 {
		t.Fatalf("category post_count should be 1, got %d", gotCat.PostCount)
	}
}

func TestArticleRepository_Create_DefaultRevisionNote(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author2", "author")

	article := &models.Article{
		Title:    "No Note",
		Slug:     "no-note",
		Content:  "x",
		AuthorID: author.ID,
		Status:   models.StatusDraft,
	}
	if err := repo.Create(article, nil, "", author.ID); err != nil {
		t.Fatalf("Create: %v", err)
	}
	revisions, _ := repo.ListRevisions(article.ID)
	if len(revisions) != 1 || revisions[0].Note != "Initial version" {
		t.Fatalf("expected default note 'Initial version', got %+v", revisions)
	}
}

func TestArticleRepository_Update_PartialFieldsAndRevision(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author3", "author")

	article := &models.Article{
		Title:    "Original",
		Slug:     "original",
		Content:  "original content",
		AuthorID: author.ID,
		Status:   models.StatusDraft,
	}
	if err := repo.Create(article, nil, "", author.ID); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updates := map[string]interface{}{
		"title":   "Updated Title",
		"content": "updated content",
	}
	if err := repo.Update(article, updates, nil, "edit", author.ID); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if article.Title != "Updated Title" {
		t.Fatalf("title should be updated, got %q", article.Title)
	}

	// Verify a new revision was created (version 2).
	revisions, _ := repo.ListRevisions(article.ID)
	if len(revisions) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(revisions))
	}
	if revisions[0].Version != 2 {
		t.Fatalf("expected latest version 2, got %d", revisions[0].Version)
	}
}

func TestArticleRepository_Update_ReplaceTags(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author4", "author")
	tag1 := createTestTag(t, db, "A", "a")
	tag2 := createTestTag(t, db, "B", "b")
	tag3 := createTestTag(t, db, "C", "c")

	article := &models.Article{
		Title:    "Tagged",
		Slug:     "tagged",
		Content:  "x",
		AuthorID: author.ID,
		Status:   models.StatusDraft,
	}
	if err := repo.Create(article, []uint{tag1.ID, tag2.ID}, "", author.ID); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Replace tags: remove tag1, keep tag2, add tag3.
	if err := repo.Update(article, nil, []uint{tag2.ID, tag3.ID}, "retag", author.ID); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(article.Tags) != 2 {
		t.Fatalf("expected 2 tags after replace, got %d", len(article.Tags))
	}

	// Verify tag counts were recomputed.
	var t1, t2, t3 models.Tag
	db.First(&t1, tag1.ID)
	db.First(&t2, tag2.ID)
	db.First(&t3, tag3.ID)
	if t1.Count != 0 {
		t.Fatalf("tag1 count should be 0, got %d", t1.Count)
	}
	if t2.Count != 1 {
		t.Fatalf("tag2 count should be 1, got %d", t2.Count)
	}
	if t3.Count != 1 {
		t.Fatalf("tag3 count should be 1, got %d", t3.Count)
	}
}

func TestArticleRepository_List_Filters(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author5", "author")

	// Create articles with different statuses.
	createTestArticleDirect(t, db, author.ID, "Published One", "pub-one")
	createTestArticleDirect(t, db, author.ID, "Draft One", "draft-one")

	// Manually set the second article to draft.
	db.Model(&models.Article{}).Where("slug = ?", "draft-one").Update("status", models.StatusDraft)

	// Filter by status=published.
	articles, total, err := repo.List(ArticleListFilter{
		Page: 1, PageSize: 10, Status: string(models.StatusPublished),
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1 for published, got %d", total)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
	if articles[0].Slug != "pub-one" {
		t.Fatalf("expected 'pub-one', got %q", articles[0].Slug)
	}
}

func TestArticleRepository_List_OmitsContentByDefault(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author6", "author")

	a := createTestArticleDirect(t, db, author.ID, "Has Content", "has-content")
	_ = a

	articles, _, err := repo.List(ArticleListFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(articles) == 0 {
		t.Fatal("expected at least 1 article")
	}
	// Content should be omitted (zero value) in default list mode.
	if articles[0].Content != "" {
		t.Fatalf("Content should be omitted in list mode, got %q", articles[0].Content)
	}

	// With Full=true, content should be present.
	articles, _, err = repo.List(ArticleListFilter{Page: 1, PageSize: 10, Full: true})
	if err != nil {
		t.Fatalf("List Full: %v", err)
	}
	if articles[0].Content == "" {
		t.Fatal("Content should be present with Full=true")
	}
}

func TestArticleRepository_GetByID_Preloads(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author7", "author")
	tag := createTestTag(t, db, "T", "t")

	article := &models.Article{
		Title:    "Preload Test",
		Slug:     "preload-test",
		Content:  "x",
		AuthorID: author.ID,
		Status:   models.StatusDraft,
	}
	if err := repo.Create(article, []uint{tag.ID}, "", author.ID); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(article.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Author.ID != author.ID {
		t.Fatalf("Author should be preloaded: got ID %d, want %d", got.Author.ID, author.ID)
	}
	if len(got.Tags) != 1 {
		t.Fatal("Tags should be preloaded")
	}
}

func TestArticleRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	_, err := repo.GetByID(99999)
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestArticleRepository_GetPublishedBySlug(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author8", "author")
	createTestArticleDirect(t, db, author.ID, "Published", "published-slug")

	// Should find published article.
	got, err := repo.GetPublishedBySlug("published-slug")
	if err != nil {
		t.Fatalf("GetPublishedBySlug: %v", err)
	}
	if got.Slug != "published-slug" {
		t.Fatalf("unexpected slug: %q", got.Slug)
	}

	// Non-existent slug should return ErrRecordNotFound.
	_, err = repo.GetPublishedBySlug("no-such-slug")
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestArticleRepository_IncrementViewAndLikeCount(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author9", "author")
	article := createTestArticleDirect(t, db, author.ID, "Counts", "counts")

	if err := repo.IncrementViewCount(article.ID); err != nil {
		t.Fatalf("IncrementViewCount: %v", err)
	}
	if err := repo.IncrementLikeCount(article.ID); err != nil {
		t.Fatalf("IncrementLikeCount: %v", err)
	}

	var got models.Article
	db.First(&got, article.ID)
	if got.ViewCount != 1 {
		t.Fatalf("view_count should be 1, got %d", got.ViewCount)
	}
	if got.LikeCount != 1 {
		t.Fatalf("like_count should be 1, got %d", got.LikeCount)
	}
}

func TestArticleRepository_EnsureUniqueSlug(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author10", "author")
	createTestArticleDirect(t, db, author.ID, "First", "my-slug")
	createTestArticleDirect(t, db, author.ID, "Second", "my-slug-1")

	// "my-slug" and "my-slug-1" are taken; EnsureUniqueSlug should return "my-slug-2".
	got, err := repo.EnsureUniqueSlug("my-slug", 0)
	if err != nil {
		t.Fatalf("EnsureUniqueSlug error: %v", err)
	}
	if got != "my-slug-2" {
		t.Fatalf("expected 'my-slug-2', got %q", got)
	}

	// With excludeID, the article itself is excluded.
	first := models.Article{}
	db.Where("slug = ?", "my-slug").First(&first)
	got, err = repo.EnsureUniqueSlug("my-slug", first.ID)
	if err != nil {
		t.Fatalf("EnsureUniqueSlug error (excludeID): %v", err)
	}
	if got != "my-slug" {
		t.Fatalf("expected 'my-slug' (excluding self), got %q", got)
	}
}

func TestArticleRepository_RestoreRevision(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author11", "author")

	article := &models.Article{
		Title:    "v1",
		Slug:     "restore-test",
		Content:  "v1 content",
		AuthorID: author.ID,
		Status:   models.StatusDraft,
	}
	if err := repo.Create(article, nil, "", author.ID); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update to v2.
	if err := repo.Update(article, map[string]interface{}{
		"title":   "v2",
		"content": "v2 content",
	}, nil, "v2 edit", author.ID); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Get the v1 revision.
	revisions, _ := repo.ListRevisions(article.ID)
	var v1 *models.Revision
	for i := range revisions {
		if revisions[i].Version == 1 {
			v1 = &revisions[i]
			break
		}
	}
	if v1 == nil {
		t.Fatal("v1 revision not found")
	}

	// Restore to v1.
	if err := repo.RestoreRevision(article, v1, author.ID); err != nil {
		t.Fatalf("RestoreRevision: %v", err)
	}

	// Verify article content was restored.
	var got models.Article
	db.First(&got, article.ID)
	if got.Title != "v1" {
		t.Fatalf("title should be 'v1', got %q", got.Title)
	}
	if got.Content != "v1 content" {
		t.Fatalf("content should be 'v1 content', got %q", got.Content)
	}

	// Verify a new revision (v3) was created with "Restored from version 1" note.
	revisions, _ = repo.ListRevisions(article.ID)
	if len(revisions) != 3 {
		t.Fatalf("expected 3 revisions, got %d", len(revisions))
	}
	// Latest revision should be version 3.
	if revisions[0].Version != 3 {
		t.Fatalf("expected version 3, got %d", revisions[0].Version)
	}
	if revisions[0].Note != "Restored from version 1" {
		t.Fatalf("expected restore note, got %q", revisions[0].Note)
	}
}

func TestArticleRepository_BulkOperations(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author12", "author")
	cat := createTestCategory(t, db, "Cat", "cat")

	a1 := createTestArticleDirect(t, db, author.ID, "A1", "bulk-1")
	a2 := createTestArticleDirect(t, db, author.ID, "A2", "bulk-2")
	ids := []uint{a1.ID, a2.ID}

	// BulkUpdateStatus
	n, err := repo.BulkUpdateStatus(ids, string(models.StatusArchived))
	if err != nil {
		t.Fatalf("BulkUpdateStatus: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 rows affected, got %d", n)
	}

	// BulkSetPinned
	n, err = repo.BulkSetPinned(ids, true)
	if err != nil {
		t.Fatalf("BulkSetPinned: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 rows affected, got %d", n)
	}

	// BulkMoveCategory
	n, err = repo.BulkMoveCategory(ids, cat.ID)
	if err != nil {
		t.Fatalf("BulkMoveCategory: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 rows affected, got %d", n)
	}

	// BulkPublish
	now := time.Now()
	n, err = repo.BulkPublish(ids, now)
	if err != nil {
		t.Fatalf("BulkPublish: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 rows affected, got %d", n)
	}

	// BulkDelete
	n, err = repo.BulkDelete(ids)
	if err != nil {
		t.Fatalf("BulkDelete: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 rows affected, got %d", n)
	}
}

func TestArticleRepository_ListScheduledDue(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author13", "author")

	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	// Scheduled article due in the past.
	a1 := createTestArticleDirect(t, db, author.ID, "Due", "scheduled-due")
	db.Model(&models.Article{}).Where("id = ?", a1.ID).Updates(map[string]interface{}{
		"status":       models.StatusScheduled,
		"scheduled_at": past,
	})

	// Scheduled article due in the future (should NOT be returned).
	a2 := createTestArticleDirect(t, db, author.ID, "Not Due", "scheduled-future")
	db.Model(&models.Article{}).Where("id = ?", a2.ID).Updates(map[string]interface{}{
		"status":       models.StatusScheduled,
		"scheduled_at": future,
	})

	due, err := repo.ListScheduledDue(time.Now())
	if err != nil {
		t.Fatalf("ListScheduledDue: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("expected 1 due article, got %d", len(due))
	}
	if due[0].Slug != "scheduled-due" {
		t.Fatalf("expected 'scheduled-due', got %q", due[0].Slug)
	}
}

func TestArticleRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArticleRepository(db)
	author := createTestUser(t, db, "author14", "author")
	article := createTestArticleDirect(t, db, author.ID, "ToDelete", "to-delete")

	if err := repo.Delete(article); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify the article is gone.
	_, err := repo.FindByID(article.ID)
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("expected ErrRecordNotFound after delete, got %v", err)
	}
}
