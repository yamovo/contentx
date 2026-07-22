package services

import (
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

func strPtr(s string) *string { return &s }

// ──────────────────────────────────────────────────────────────────────────────
// Webhook dispatch 验证测试
//
// 验证各 Service 在关键操作后正确触发 webhook 事件。
// 使用 MockWebhookDispatcher 记录 Dispatch 调用，不依赖真实 HTTP。
// ──────────────────────────────────────────────────────────────────────────────

// ---------- ArticleService ----------

func TestWebhookDispatch_ArticleCreate(t *testing.T) {
	svc, _ := newMockArticleService()
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	_, err := svc.Create(CreateArticleRequest{Title: "Hello", Content: "World"}, 1)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if len(wh.Dispatches) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(wh.Dispatches))
	}
	if wh.Dispatches[0].Event != models.WebhookEventEntryCreate {
		t.Errorf("event = %q, want %q", wh.Dispatches[0].Event, models.WebhookEventEntryCreate)
	}
}

func TestWebhookDispatch_ArticleCreate_NoDispatcher(t *testing.T) {
	svc, _ := newMockArticleService()
	// 不设置 dispatcher → 不应 panic
	_, err := svc.Create(CreateArticleRequest{Title: "Hello"}, 1)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
}

func TestWebhookDispatch_ArticleCreate_RepoError(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.CreateErr = gorm.ErrInvalidDB
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	_, err := svc.Create(CreateArticleRequest{Title: "Hello"}, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(wh.Dispatches) != 0 {
		t.Errorf("expected 0 dispatches on error, got %d", len(wh.Dispatches))
	}
}

func TestWebhookDispatch_ArticleUpdate(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Articles[1] = &models.Article{BaseModel: models.BaseModel{ID: 1}, AuthorID: 1, Slug: "test"}
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	_, err := svc.Update(1, UpdateArticleRequest{Title: strPtr("Updated")}, 1, true)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if len(wh.Dispatches) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(wh.Dispatches))
	}
	if wh.Dispatches[0].Event != models.WebhookEventEntryUpdate {
		t.Errorf("event = %q, want %q", wh.Dispatches[0].Event, models.WebhookEventEntryUpdate)
	}
}

func TestWebhookDispatch_ArticleDelete(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Articles[1] = &models.Article{BaseModel: models.BaseModel{ID: 1}, AuthorID: 1}
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	err := svc.Delete(1, 1, true)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if len(wh.Dispatches) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(wh.Dispatches))
	}
	if wh.Dispatches[0].Event != models.WebhookEventEntryDelete {
		t.Errorf("event = %q, want %q", wh.Dispatches[0].Event, models.WebhookEventEntryDelete)
	}
}

func TestWebhookDispatch_ArticleBulkPublish(t *testing.T) {
	svc, _ := newMockArticleService()
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	n, err := svc.BulkAction(BulkActionRequest{Action: "publish", ArticleIDs: []uint{1, 2, 3}})
	if err != nil {
		t.Fatalf("BulkAction failed: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 affected, got %d", n)
	}
	if len(wh.Dispatches) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(wh.Dispatches))
	}
	if wh.Dispatches[0].Event != models.WebhookEventEntryPublish {
		t.Errorf("event = %q, want %q", wh.Dispatches[0].Event, models.WebhookEventEntryPublish)
	}
}

func TestWebhookDispatch_ArticleBulkDelete(t *testing.T) {
	svc, _ := newMockArticleService()
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	_, err := svc.BulkAction(BulkActionRequest{Action: "delete", ArticleIDs: []uint{1}})
	if err != nil {
		t.Fatalf("BulkAction failed: %v", err)
	}
	if len(wh.Dispatches) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(wh.Dispatches))
	}
	if wh.Dispatches[0].Event != models.WebhookEventEntryDelete {
		t.Errorf("event = %q, want %q", wh.Dispatches[0].Event, models.WebhookEventEntryDelete)
	}
}

func TestWebhookDispatch_ArticleBulkDraft_NoEvent(t *testing.T) {
	svc, _ := newMockArticleService()
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	_, err := svc.BulkAction(BulkActionRequest{Action: "draft", ArticleIDs: []uint{1}})
	if err != nil {
		t.Fatalf("BulkAction failed: %v", err)
	}
	// draft 不触发 webhook（没有对应事件常量）
	if len(wh.Dispatches) != 0 {
		t.Errorf("expected 0 dispatches for draft, got %d", len(wh.Dispatches))
	}
}

// ---------- CommentService ----------

func TestWebhookDispatch_CommentCreate(t *testing.T) {
	repo := &MockCommentRepository{
		Article: &models.Article{BaseModel: models.BaseModel{ID: 1}, AllowComment: true},
	}
	svc := NewCommentServiceWithRepo(repo)
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	_, err := svc.Create(CreateCommentRequest{
		ArticleID:   1,
		Content:     "Nice post!",
		AuthorName:  "Visitor",
		AuthorEmail: "v@example.com",
	}, "127.0.0.1", "Mozilla", nil, false)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if len(wh.Dispatches) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(wh.Dispatches))
	}
	if wh.Dispatches[0].Event != models.WebhookEventCommentCreate {
		t.Errorf("event = %q, want %q", wh.Dispatches[0].Event, models.WebhookEventCommentCreate)
	}
}

func TestWebhookDispatch_CommentCreate_ArticleNotFound(t *testing.T) {
	repo := &MockCommentRepository{
		FindArticleByIDErr: gorm.ErrRecordNotFound,
	}
	svc := NewCommentServiceWithRepo(repo)
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	_, err := svc.Create(CreateCommentRequest{ArticleID: 999}, "", "", nil, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(wh.Dispatches) != 0 {
		t.Errorf("expected 0 dispatches on error, got %d", len(wh.Dispatches))
	}
}

// ---------- MediaService ----------

func TestWebhookDispatch_MediaUpload(t *testing.T) {
	repo := &MockMediaRepository{}
	cfg := newTestUploadConfig(t)
	svc := NewMediaServiceWithRepo(repo, cfg)
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	content := append(pngHeader, make([]byte, 100)...)
	fh := newMultipartHeader(t, "photo.png", content)
	f, header := openFileHeader(t, fh)
	defer f.Close()

	_, err := svc.Upload(f, header, "", "", "", "", "", 1)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	if len(wh.Dispatches) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(wh.Dispatches))
	}
	if wh.Dispatches[0].Event != models.WebhookEventMediaCreate {
		t.Errorf("event = %q, want %q", wh.Dispatches[0].Event, models.WebhookEventMediaCreate)
	}
}

func TestWebhookDispatch_MediaDelete(t *testing.T) {
	repo := &MockMediaRepository{
		FindMedia: &models.Media{BaseModel: models.BaseModel{ID: 1}, FilePath: "/nonexistent"},
	}
	cfg := newTestUploadConfig(t)
	svc := NewMediaServiceWithRepo(repo, cfg)
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	err := svc.Delete(1)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if len(wh.Dispatches) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(wh.Dispatches))
	}
	if wh.Dispatches[0].Event != models.WebhookEventMediaDelete {
		t.Errorf("event = %q, want %q", wh.Dispatches[0].Event, models.WebhookEventMediaDelete)
	}
}

// ---------- UserService ----------

func TestWebhookDispatch_UserCreate(t *testing.T) {
	repo := &MockUserRepository{}
	svc := NewUserServiceWithRepo(repo)
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	_, err := svc.Create(CreateUserRequest{
		Username:    "newuser",
		Email:       "new@example.com",
		Password:    "StrongPass1",
		DisplayName: "New User",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if len(wh.Dispatches) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(wh.Dispatches))
	}
	if wh.Dispatches[0].Event != models.WebhookEventUserCreate {
		t.Errorf("event = %q, want %q", wh.Dispatches[0].Event, models.WebhookEventUserCreate)
	}
}

func TestWebhookDispatch_UserCreate_Error(t *testing.T) {
	repo := &MockUserRepository{CreateErr: ErrUsernameExists}
	svc := NewUserServiceWithRepo(repo)
	wh := &MockWebhookDispatcher{}
	svc.SetWebhookDispatcher(wh)

	_, err := svc.Create(CreateUserRequest{
		Username: "dup",
		Email:    "dup@example.com",
		Password: "StrongPass1",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(wh.Dispatches) != 0 {
		t.Errorf("expected 0 dispatches on error, got %d", len(wh.Dispatches))
	}
}

// 编译期检查：*WebhookService 实现 WebhookDispatcher
var _ WebhookDispatcher = (*WebhookService)(nil)

// 确保 time 包被使用（testutil 可能用到）
var _ = time.Now
