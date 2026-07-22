package services

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

func TestMockComment_NewWithRepo(t *testing.T) {
	svc := NewCommentServiceWithRepo(&MockCommentRepository{})
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// ---------- List ----------

func TestMockComment_List_Success(t *testing.T) {
	comments := []models.Comment{
		{BaseModel: models.BaseModel{ID: 1}, Content: "first"},
		{BaseModel: models.BaseModel{ID: 2}, Content: "second"},
	}
	repo := &MockCommentRepository{
		ListComments: comments,
		ListTotal:    2,
	}
	svc := NewCommentServiceWithRepo(repo)

	result, total, err := svc.List(CommentListParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 comments, got %d", len(result))
	}
}

func TestMockComment_List_Error(t *testing.T) {
	repo := &MockCommentRepository{ListErr: gorm.ErrInvalidDB}
	svc := NewCommentServiceWithRepo(repo)

	_, _, err := svc.List(CommentListParams{Page: 1, PageSize: 10})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------- Update ----------

func TestMockComment_Update_Success(t *testing.T) {
	repo := &MockCommentRepository{}
	svc := NewCommentServiceWithRepo(repo)

	if err := svc.Update(1, "updated content"); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if len(repo.UpdatedContent) != 1 {
		t.Fatalf("expected 1 UpdateContent call, got %d", len(repo.UpdatedContent))
	}
	if repo.UpdatedContent[0].ID != 1 {
		t.Errorf("expected ID 1, got %d", repo.UpdatedContent[0].ID)
	}
	if repo.UpdatedContent[0].Content != "updated content" {
		t.Errorf("expected content 'updated content', got %s", repo.UpdatedContent[0].Content)
	}
}

func TestMockComment_Update_NotFound(t *testing.T) {
	// zeroRowsCommentRepo returns 0 rows affected → service returns "comment not found".
	svc := NewCommentServiceWithRepo(&zeroRowsCommentRepo{})

	err := svc.Update(99, "x")
	if err == nil {
		t.Fatal("expected error for not found")
	}
	if err.Error() != "comment not found" {
		t.Errorf("expected 'comment not found', got %v", err)
	}
}

func TestMockComment_Update_Error(t *testing.T) {
	repo := &MockCommentRepository{UpdateContentErr: gorm.ErrInvalidDB}
	svc := NewCommentServiceWithRepo(repo)

	if err := svc.Update(1, "x"); err == nil {
		t.Fatal("expected error")
	}
}

// ---------- Create ----------

func TestMockComment_Create_ArticleNotFound(t *testing.T) {
	repo := &MockCommentRepository{FindArticleByIDErr: gorm.ErrRecordNotFound}
	svc := NewCommentServiceWithRepo(repo)

	_, err := svc.Create(CreateCommentRequest{ArticleID: 99, Content: "hi"}, "1.2.3.4", "ua", nil, false)
	if err == nil || err.Error() != "article not found" {
		t.Errorf("expected 'article not found', got %v", err)
	}
}

func TestMockComment_Create_CommentsDisabled(t *testing.T) {
	repo := &MockCommentRepository{
		Article: &models.Article{BaseModel: models.BaseModel{ID: 1}, AllowComment: false},
	}
	svc := NewCommentServiceWithRepo(repo)

	_, err := svc.Create(CreateCommentRequest{ArticleID: 1, Content: "hi"}, "1.2.3.4", "ua", nil, false)
	if err == nil || err.Error() != "comments are disabled for this article" {
		t.Errorf("expected comments disabled error, got %v", err)
	}
}

func TestMockComment_Create_AnonymousSuccess(t *testing.T) {
	repo := &MockCommentRepository{
		Article: &models.Article{BaseModel: models.BaseModel{ID: 1}, AllowComment: true},
	}
	svc := NewCommentServiceWithRepo(repo)

	comment, err := svc.Create(CreateCommentRequest{
		ArticleID: 1, Content: "nice post", AuthorName: "Guest",
	}, "1.2.3.4", "ua", nil, false)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if comment.Status != "pending" {
		t.Errorf("expected status pending for anonymous, got %s", comment.Status)
	}
	if comment.AuthorIP != "1.2.3.4" {
		t.Errorf("expected AuthorIP 1.2.3.4, got %s", comment.AuthorIP)
	}
	if len(repo.ArticleCommentCountIncs) != 1 || repo.ArticleCommentCountIncs[0] != 1 {
		t.Errorf("expected 1 comment count increment for article 1, got %v", repo.ArticleCommentCountIncs)
	}
}

func TestMockComment_Create_AuthenticatedEditor(t *testing.T) {
	repo := &MockCommentRepository{
		Article: &models.Article{BaseModel: models.BaseModel{ID: 1}, AllowComment: true},
	}
	svc := NewCommentServiceWithRepo(repo)

	uid := uint(5)
	comment, err := svc.Create(CreateCommentRequest{
		ArticleID: 1, Content: "editor comment",
	}, "1.2.3.4", "ua", &uid, true)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if comment.Status != "approved" {
		t.Errorf("expected status approved for editor, got %s", comment.Status)
	}
	if comment.UserID == nil || *comment.UserID != 5 {
		t.Errorf("expected user ID 5, got %v", comment.UserID)
	}
}

func TestMockComment_Create_AuthenticatedNonEditor(t *testing.T) {
	repo := &MockCommentRepository{
		Article: &models.Article{BaseModel: models.BaseModel{ID: 1}, AllowComment: true},
	}
	svc := NewCommentServiceWithRepo(repo)

	uid := uint(5)
	comment, err := svc.Create(CreateCommentRequest{
		ArticleID: 1, Content: "user comment",
	}, "1.2.3.4", "ua", &uid, false)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if comment.Status != "pending" {
		t.Errorf("expected status pending for non-editor, got %s", comment.Status)
	}
}

func TestMockComment_Create_WithParent(t *testing.T) {
	parent := &models.Comment{BaseModel: models.BaseModel{ID: 10}, Depth: 2}
	repo := &MockCommentRepository{
		Article:       &models.Article{BaseModel: models.BaseModel{ID: 1}, AllowComment: true},
		ParentComment: parent,
	}
	svc := NewCommentServiceWithRepo(repo)

	pid := uint(10)
	comment, err := svc.Create(CreateCommentRequest{
		ArticleID: 1, Content: "reply", ParentID: &pid,
	}, "1.2.3.4", "ua", nil, false)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if comment.Depth != 3 {
		t.Errorf("expected depth 3 (parent 2 + 1), got %d", comment.Depth)
	}
}

func TestMockComment_Create_ParentNotFound(t *testing.T) {
	// Parent lookup fails → depth stays 0 (best-effort).
	repo := &MockCommentRepository{
		Article:            &models.Article{BaseModel: models.BaseModel{ID: 1}, AllowComment: true},
		FindCommentByIDErr: gorm.ErrRecordNotFound,
	}
	svc := NewCommentServiceWithRepo(repo)

	pid := uint(99)
	comment, err := svc.Create(CreateCommentRequest{
		ArticleID: 1, Content: "reply", ParentID: &pid,
	}, "1.2.3.4", "ua", nil, false)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if comment.Depth != 0 {
		t.Errorf("expected depth 0 when parent not found, got %d", comment.Depth)
	}
}

func TestMockComment_Create_RepoError(t *testing.T) {
	repo := &MockCommentRepository{
		Article:   &models.Article{BaseModel: models.BaseModel{ID: 1}, AllowComment: true},
		CreateErr: gorm.ErrInvalidDB,
	}
	svc := NewCommentServiceWithRepo(repo)

	_, err := svc.Create(CreateCommentRequest{ArticleID: 1, Content: "hi"}, "1.2.3.4", "ua", nil, false)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------- UpdateStatus ----------

func TestMockComment_UpdateStatus_Success(t *testing.T) {
	repo := &MockCommentRepository{}
	svc := NewCommentServiceWithRepo(repo)

	if err := svc.UpdateStatus(1, "approved"); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	if len(repo.UpdatedStatus) != 1 {
		t.Fatalf("expected 1 UpdateStatus call, got %d", len(repo.UpdatedStatus))
	}
}

func TestMockComment_UpdateStatus_NotFound(t *testing.T) {
	svc := NewCommentServiceWithRepo(&zeroRowsCommentRepo{})

	err := svc.UpdateStatus(99, "approved")
	if err == nil || err.Error() != "comment not found" {
		t.Errorf("expected 'comment not found', got %v", err)
	}
}

// ---------- BulkAction ----------

func TestMockComment_BulkAction_AllBranches(t *testing.T) {
	cases := []struct {
		name       string
		action     string
		wantStatus string // for approve/spam/trash
		wantDelete bool
		wantErr    bool
	}{
		{"approve", "approve", "approved", false, false},
		{"spam", "spam", "spam", false, false},
		{"trash", "trash", "trash", false, false},
		{"delete", "delete", "", true, false},
		{"unknown", "unknown", "", false, true},
	}
	ids := []uint{1, 2, 3}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &MockCommentRepository{}
			svc := NewCommentServiceWithRepo(repo)

			affected, err := svc.BulkAction(ids, tc.action)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("BulkAction failed: %v", err)
			}
			if affected != 3 {
				t.Errorf("expected 3 affected, got %d", affected)
			}
			if tc.wantDelete {
				if len(repo.BulkDeletedIDs) != 3 {
					t.Errorf("expected 3 deleted IDs, got %d", len(repo.BulkDeletedIDs))
				}
			} else if tc.wantStatus != "" {
				if len(repo.BulkUpdatedStatus) != 1 {
					t.Fatalf("expected 1 BulkUpdateStatus call, got %d", len(repo.BulkUpdatedStatus))
				}
				if repo.BulkUpdatedStatus[0].Status != tc.wantStatus {
					t.Errorf("expected status %s, got %s", tc.wantStatus, repo.BulkUpdatedStatus[0].Status)
				}
			}
		})
	}
}

// ---------- Stats ----------

func TestMockComment_Stats_Success(t *testing.T) {
	repo := &MockCommentRepository{
		StatsData: repository.CommentStatsData{Total: 100, Pending: 5, Approved: 90, Spam: 5, Today: 3},
	}
	svc := NewCommentServiceWithRepo(repo)

	stats, err := svc.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.Total != 100 || stats.Pending != 5 || stats.Approved != 90 || stats.Spam != 5 || stats.Today != 3 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestMockComment_Stats_Error(t *testing.T) {
	// On error, Stats returns zero CommentStats (mirrors prior best-effort behaviour).
	repo := &MockCommentRepository{StatsErr: gorm.ErrInvalidDB}
	svc := NewCommentServiceWithRepo(repo)

	stats, err := svc.Stats()
	if err != nil {
		t.Errorf("expected nil error on stats error, got %v", err)
	}
	if stats.Total != 0 {
		t.Errorf("expected zero stats on error, got %+v", stats)
	}
}

// ---------- ArticleComments ----------

func TestMockComment_ArticleComments_Success(t *testing.T) {
	comments := []models.Comment{{BaseModel: models.BaseModel{ID: 1}, Content: "top"}}
	repo := &MockCommentRepository{ArticleComments: comments}
	svc := NewCommentServiceWithRepo(repo)

	result, err := svc.ArticleComments(1)
	if err != nil {
		t.Fatalf("ArticleComments failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 comment, got %d", len(result))
	}
}

func TestMockComment_ArticleComments_Error(t *testing.T) {
	repo := &MockCommentRepository{FindArticleCommentsErr: gorm.ErrInvalidDB}
	svc := NewCommentServiceWithRepo(repo)

	_, err := svc.ArticleComments(1)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------- Get ----------

func TestMockComment_Get_Success(t *testing.T) {
	repo := &MockCommentRepository{
		Comment: &models.Comment{BaseModel: models.BaseModel{ID: 1}, Content: "hi"},
	}
	svc := NewCommentServiceWithRepo(repo)

	comment, err := svc.Get(1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if comment.ID != 1 {
		t.Errorf("expected ID 1, got %d", comment.ID)
	}
}

// zeroRowsCommentRepo is a minimal CommentRepository whose Update methods return 0 rows affected,
// simulating a "not found" scenario.
type zeroRowsCommentRepo struct{}

func (z *zeroRowsCommentRepo) List(_ repository.CommentListFilter) ([]models.Comment, int64, error) {
	return nil, 0, nil
}
func (z *zeroRowsCommentRepo) GetByID(_ uint) (*models.Comment, error) {
	return nil, gorm.ErrRecordNotFound
}
func (z *zeroRowsCommentRepo) FindArticleByID(_ uint) (*models.Article, error) {
	return nil, gorm.ErrRecordNotFound
}
func (z *zeroRowsCommentRepo) FindCommentByID(_ uint) (*models.Comment, error) {
	return nil, gorm.ErrRecordNotFound
}
func (z *zeroRowsCommentRepo) Create(_ *models.Comment) error { return nil }
func (z *zeroRowsCommentRepo) UpdateContent(_ uint, _ string) (int64, error) {
	return 0, nil
}
func (z *zeroRowsCommentRepo) UpdateStatus(_ uint, _ string) (int64, error) {
	return 0, nil
}
func (z *zeroRowsCommentRepo) BulkUpdateStatus(_ []uint, _ string) (int64, error) {
	return 0, nil
}
func (z *zeroRowsCommentRepo) BulkDelete(_ []uint) (int64, error) {
	return 0, nil
}
func (z *zeroRowsCommentRepo) FindArticleComments(_ uint) ([]models.Comment, error) {
	return nil, nil
}
func (z *zeroRowsCommentRepo) IncrementArticleCommentCount(_ uint) error { return nil }
func (z *zeroRowsCommentRepo) Stats() (repository.CommentStatsData, error) {
	return repository.CommentStatsData{}, nil
}
func (z *zeroRowsCommentRepo) CountToday() (int64, error) { return 0, nil }
