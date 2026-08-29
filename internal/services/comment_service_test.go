package services

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
)

func TestCommentService_Create_Authenticated(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCommentService(db)
	articleSvc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "commenter", "subscriber")
	author := createTestUser(t, db, "author1", "author")
	article := createTestArticle(t, db, author.ID, "Commented Article")

	req := CreateCommentRequest{
		ArticleID: article.ID,
		Content:   "Great article!",
	}

	comment, err := svc.Create(req, "127.0.0.1", "test-agent", &user.ID, false, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if comment.Content != "Great article!" {
		t.Errorf("Content = %q, want %q", comment.Content, "Great article!")
	}
	if comment.UserID == nil || *comment.UserID != user.ID {
		t.Error("UserID should be set for authenticated comment")
	}
	if comment.Status != "pending" {
		t.Errorf("Status = %q, want %q", comment.Status, "pending")
	}
	_ = articleSvc
}

func TestCommentService_Create_EditorAutoApprove(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCommentService(db)
	editor := createTestUser(t, db, "editor1", "editor")
	author := createTestUser(t, db, "author1", "author")
	article := createTestArticle(t, db, author.ID, "Article")

	req := CreateCommentRequest{
		ArticleID: article.ID,
		Content:   "Editor comment",
	}

	comment, err := svc.Create(req, "127.0.0.1", "test-agent", &editor.ID, true, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if comment.Status != "approved" {
		t.Errorf("Status = %q, want %q (editor should auto-approve)", comment.Status, "approved")
	}
}

func TestCommentService_Create_Anonymous(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCommentService(db)
	author := createTestUser(t, db, "author1", "author")
	article := createTestArticle(t, db, author.ID, "Article")

	req := CreateCommentRequest{
		ArticleID:   article.ID,
		Content:     "Anonymous comment",
		AuthorName:  "Guest",
		AuthorEmail: "guest@test.com",
	}

	comment, err := svc.Create(req, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if comment.UserID != nil {
		t.Error("UserID should be nil for anonymous comment")
	}
	if comment.AuthorName != "Guest" {
		t.Errorf("AuthorName = %q, want %q", comment.AuthorName, "Guest")
	}
}

func TestCommentService_Create_DisabledComments(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCommentService(db)
	author := createTestUser(t, db, "author1", "author")

	// Create article with comments disabled.
	article := createTestArticle(t, db, author.ID, "No Comments")
	db.Model(article).Update("allow_comment", false)

	req := CreateCommentRequest{
		ArticleID: article.ID,
		Content:   "Should fail",
	}

	_, err := svc.Create(req, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)
	if err == nil {
		t.Error("Create() should fail when comments are disabled")
	}
}

func TestCommentService_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCommentService(db)
	author := createTestUser(t, db, "author1", "author")
	article := createTestArticle(t, db, author.ID, "Article")

	comment, _ := svc.Create(CreateCommentRequest{
		ArticleID: article.ID,
		Content:   "To moderate",
	}, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)

	if err := svc.UpdateStatus(comment.ID, "approved", models.DefaultTenantID); err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	got, _ := svc.Get(comment.ID, models.DefaultTenantID)
	if got.Status != "approved" {
		t.Errorf("Status = %q, want %q", got.Status, "approved")
	}
}

func TestCommentService_BulkAction(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCommentService(db)
	author := createTestUser(t, db, "author1", "author")
	article := createTestArticle(t, db, author.ID, "Article")

	c1, _ := svc.Create(CreateCommentRequest{ArticleID: article.ID, Content: "C1"}, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)
	c2, _ := svc.Create(CreateCommentRequest{ArticleID: article.ID, Content: "C2"}, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)

	affected, err := svc.BulkAction([]uint{c1.ID, c2.ID}, "spam", models.DefaultTenantID)
	if err != nil {
		t.Fatalf("BulkAction() error: %v", err)
	}
	if affected != 2 {
		t.Errorf("Affected = %d, want 2", affected)
	}
}

func TestCommentService_ArticleComments(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCommentService(db)
	author := createTestUser(t, db, "author1", "author")
	article := createTestArticle(t, db, author.ID, "Article")

	// Create and approve a comment.
	comment, _ := svc.Create(CreateCommentRequest{ArticleID: article.ID, Content: "Approved!"}, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)
	svc.UpdateStatus(comment.ID, "approved", models.DefaultTenantID)

	// Create a pending comment (should not appear).
	svc.Create(CreateCommentRequest{ArticleID: article.ID, Content: "Pending"}, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)

	comments, err := svc.ArticleComments(article.ID, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("ArticleComments() error: %v", err)
	}

	if len(comments) != 1 {
		t.Errorf("Comments count = %d, want 1 (only approved)", len(comments))
	}
}

func TestCommentService_Stats(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCommentService(db)
	author := createTestUser(t, db, "author1", "author")
	article := createTestArticle(t, db, author.ID, "Article")

	svc.Create(CreateCommentRequest{ArticleID: article.ID, Content: "C1"}, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)
	svc.Create(CreateCommentRequest{ArticleID: article.ID, Content: "C2"}, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)

	stats, err := svc.Stats(models.DefaultTenantID)
	if err != nil {
		t.Fatalf("Stats() error: %v", err)
	}

	if stats.Total != 2 {
		t.Errorf("Total = %d, want 2", stats.Total)
	}
	if stats.Pending != 2 {
		t.Errorf("Pending = %d, want 2", stats.Pending)
	}
}

func TestCommentService_Create_ReplyComputesDepth(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCommentService(db)
	author := createTestUser(t, db, "author2", "author")
	article := createTestArticle(t, db, author.ID, "Reply Article")

	root, err := svc.Create(CreateCommentRequest{
		ArticleID: article.ID,
		Content:   "root",
	}, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("create root comment: %v", err)
	}

	reply, err := svc.Create(CreateCommentRequest{
		ArticleID: article.ID,
		ParentID:  &root.ID,
		Content:   "reply",
	}, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("create reply: %v", err)
	}

	if reply.Depth != root.Depth+1 {
		t.Errorf("Depth = %d, want %d", reply.Depth, root.Depth+1)
	}
}

func TestCommentService_Create_RejectsMissingParent(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCommentService(db)
	author := createTestUser(t, db, "author3", "author")
	article := createTestArticle(t, db, author.ID, "Orphan Reply Article")

	missing := uint(99999)
	_, err := svc.Create(CreateCommentRequest{
		ArticleID: article.ID,
		ParentID:  &missing,
		Content:   "orphan reply",
	}, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)
	if err == nil {
		t.Fatal("expected error when parent comment does not exist")
	}
}

func TestCommentService_Create_RejectsCrossArticleParent(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCommentService(db)
	author := createTestUser(t, db, "author4", "author")
	articleA := createTestArticle(t, db, author.ID, "Article A Comments")
	articleB := createTestArticle(t, db, author.ID, "Article B Comments")

	parent, err := svc.Create(CreateCommentRequest{
		ArticleID: articleA.ID,
		Content:   "parent on article A",
	}, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("create parent comment: %v", err)
	}

	_, err = svc.Create(CreateCommentRequest{
		ArticleID: articleB.ID,
		ParentID:  &parent.ID,
		Content:   "reply targeting another article",
	}, "127.0.0.1", "test-agent", nil, false, models.DefaultTenantID)
	if err == nil {
		t.Fatal("expected error when parent comment belongs to a different article")
	}
}
