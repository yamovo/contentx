package repository

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

func TestCommentRepository_TenantIsolation(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommentRepository(db)

	comment := &models.Comment{TenantID: 1, ArticleID: 1, Content: "T1 comment", Status: "pending"}
	if err := repo.Create(comment); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Cross-tenant reads.
	if _, err := repo.GetByID(comment.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("GetByID cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	if _, err := repo.FindCommentByID(comment.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("FindCommentByID cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	_, total, err := repo.List(CommentListFilter{}, 2)
	if err != nil || total != 0 {
		t.Fatalf("List cross-tenant = %d/%v, want 0/nil", total, err)
	}
	if _, err := repo.FindArticleByID(1, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("FindArticleByID cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	stats, err := repo.Stats(2)
	if err != nil || stats.Total != 0 {
		t.Fatalf("Stats cross-tenant = %+v/%v, want zero/nil", stats, err)
	}

	// Cross-tenant writes are no-ops.
	if n, err := repo.UpdateContent(comment.ID, "hacked", 2); err != nil || n != 0 {
		t.Fatalf("UpdateContent cross-tenant = %d/%v, want 0/nil", n, err)
	}
	if n, err := repo.UpdateStatus(comment.ID, "approved", 2); err != nil || n != 0 {
		t.Fatalf("UpdateStatus cross-tenant = %d/%v, want 0/nil", n, err)
	}
	if n, err := repo.BulkUpdateStatus([]uint{comment.ID}, "approved", 2); err != nil || n != 0 {
		t.Fatalf("BulkUpdateStatus cross-tenant = %d/%v, want 0/nil", n, err)
	}
	if n, err := repo.BulkDelete([]uint{comment.ID}, 2); err != nil || n != 0 {
		t.Fatalf("BulkDelete cross-tenant = %d/%v, want 0/nil", n, err)
	}

	// Tenant 1 row intact and still pending.
	got, err := repo.FindCommentByID(comment.ID, 1)
	if err != nil || got.Status != "pending" || got.Content != "T1 comment" {
		t.Fatalf("tenant 1 comment changed after cross-tenant writes: %v/%+v", err, got)
	}
}
