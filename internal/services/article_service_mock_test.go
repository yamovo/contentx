package services

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────────────────────────────────────
// ArticleService 纯 mock 单元测试
//
// 这些测试不依赖数据库，通过 MockArticleRepository 精确控制 repo 层返回值，
// 覆盖现有集成测试（article_service_test.go）未触及的业务逻辑分支：
//   - GetBySlug（view count 递增）
//   - RestoreRevision（两步查询 + 恢复）
//   - BulkAction 全部分支（publish/draft/trash/delete/move/pin/unpin/unknown/missing_category）
//   - Update 权限拒绝（非作者非编辑）
//   - Delete 权限拒绝
//   - Create 默认值填充与 slug 生成
//   - GenerateFeed（空列表 + 有数据）
//   - List 分页默认值
//   - repo 错误传播
// ──────────────────────────────────────────────────────────────────────────────

// newMockArticleService 创建一个带 mock repo 的 ArticleService。
func newMockArticleService() (*ArticleService, *MockArticleRepository) {
	mock := &MockArticleRepository{
		Articles: make(map[uint]*models.Article),
	}
	svc := NewArticleServiceWithRepo(mock, "http://localhost:8080")
	return svc, mock
}

func TestMockArticle_GetBySlug_Success(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Articles[1] = &models.Article{BaseModel: models.BaseModel{ID: 1}, Slug: "hello-world", Title: "Hello", ViewCount: 5}

	article, err := svc.GetBySlug("hello-world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if article.ID != 1 {
		t.Fatalf("expected article ID 1, got %d", article.ID)
	}
	if article.ViewCount != 6 {
		t.Fatalf("view count should increment to 6, got %d", article.ViewCount)
	}
	if len(mock.ViewCountIncs) != 1 || mock.ViewCountIncs[0] != 1 {
		t.Fatalf("IncrementViewCount should be called once with ID 1, got %v", mock.ViewCountIncs)
	}
}

func TestMockArticle_GetBySlug_NotFound(t *testing.T) {
	svc, _ := newMockArticleService()

	_, err := svc.GetBySlug("nonexistent")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}

func TestMockArticle_GetBySlug_ViewCountErrorBestEffort(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Articles[1] = &models.Article{BaseModel: models.BaseModel{ID: 1}, Slug: "test", ViewCount: 10}
	mock.IncrementViewErr = errors.New("redis down")

	// 即使 IncrementViewCount 失败，GetBySlug 也应成功返回（best-effort）
	article, err := svc.GetBySlug("test")
	if err != nil {
		t.Fatalf("GetBySlug should not fail even if view count increment fails: %v", err)
	}
	// ViewCount 仍在内存中递增
	if article.ViewCount != 11 {
		t.Fatalf("view count should still increment in-memory to 11, got %d", article.ViewCount)
	}
}

func TestMockArticle_Create_Defaults(t *testing.T) {
	svc, mock := newMockArticleService()

	req := CreateArticleRequest{
		Title:   "Test Article",
		Content: "Some content here",
	}

	article, err := svc.Create(req, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证默认值
	if article.PostType != models.PostTypePost {
		t.Errorf("default PostType should be Post, got %s", article.PostType)
	}
	if article.Status != models.StatusDraft {
		t.Errorf("default Status should be Draft, got %s", article.Status)
	}
	if article.Visibility != models.VisibilityPublic {
		t.Errorf("default Visibility should be Public, got %s", article.Visibility)
	}
	if !article.AllowComment {
		t.Error("default AllowComment should be true")
	}
	if !article.RobotsIndex {
		t.Error("default RobotsIndex should be true")
	}
	if !article.RobotsFollow {
		t.Error("default RobotsFollow should be true")
	}
	if article.Slug == "" {
		t.Error("slug should be generated from title")
	}
	if article.ReadingTime == 0 {
		t.Error("reading time should be calculated")
	}
	if article.Excerpt == "" {
		t.Error("excerpt should be generated")
	}
	if len(mock.CreatedArticles) != 1 {
		t.Fatalf("Create should be called once, got %d", len(mock.CreatedArticles))
	}
}

func TestMockArticle_Create_PublishSetsPublishedAt(t *testing.T) {
	svc, _ := newMockArticleService()
	status := "published"
	req := CreateArticleRequest{
		Title:   "Published Article",
		Status:  status,
		Content: "Content",
	}

	article, err := svc.Create(req, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if article.PublishedAt == nil {
		t.Fatal("PublishedAt should be set when status is published")
	}
}

func TestMockArticle_Create_RepoError(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.CreateErr = errors.New("db connection lost")

	_, err := svc.Create(CreateArticleRequest{Title: "Test"}, 1)
	if err == nil {
		t.Fatal("expected error from repo.Create")
	}
}

func TestMockArticle_Update_ForbiddenNonOwner(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Articles[1] = &models.Article{BaseModel: models.BaseModel{ID: 1}, AuthorID: 100}

	title := "Updated"
	_, err := svc.Update(1, UpdateArticleRequest{Title: &title}, 200, false)
	if err == nil {
		t.Fatal("expected forbidden error for non-owner non-editor")
	}
	var appErr *errs.AppError
	if !errs.Is(err, &appErr) {
		t.Fatalf("expected *errs.AppError, got %T: %v", err, err)
	}
	if appErr.Code != "FORBIDDEN" {
		t.Fatalf("expected err_code FORBIDDEN, got %s", appErr.Code)
	}
}

func TestMockArticle_Update_AsEditorAllowed(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Articles[1] = &models.Article{BaseModel: models.BaseModel{ID: 1}, AuthorID: 100}

	title := "Editor Updated"
	_, err := svc.Update(1, UpdateArticleRequest{Title: &title}, 200, true)
	if err != nil {
		t.Fatalf("editor should be allowed to update: %v", err)
	}
	if len(mock.UpdatedArticles) != 1 {
		t.Fatalf("repo.Update should be called once, got %d", len(mock.UpdatedArticles))
	}
}

func TestMockArticle_Update_AsOwnerAllowed(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Articles[1] = &models.Article{BaseModel: models.BaseModel{ID: 1}, AuthorID: 100}

	title := "Owner Updated"
	_, err := svc.Update(1, UpdateArticleRequest{Title: &title}, 100, false)
	if err != nil {
		t.Fatalf("owner should be allowed to update: %v", err)
	}
}

func TestMockArticle_Update_NotFound(t *testing.T) {
	svc, _ := newMockArticleService()
	title := "x"
	_, err := svc.Update(999, UpdateArticleRequest{Title: &title}, 1, true)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestMockArticle_Update_SlugEnsuresUnique(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Articles[1] = &models.Article{BaseModel: models.BaseModel{ID: 1}, AuthorID: 1}
	mock.UniqueSlugSuffix = "-1"

	newSlug := "my-slug"
	_, err := svc.Update(1, UpdateArticleRequest{Slug: &newSlug}, 1, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.EnsureUniqueCalls) != 1 || mock.EnsureUniqueCalls[0] != "my-slug" {
		t.Fatalf("EnsureUniqueSlug should be called with 'my-slug', got %v", mock.EnsureUniqueCalls)
	}
}

func TestMockArticle_Delete_ForbiddenNonOwner(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Articles[1] = &models.Article{BaseModel: models.BaseModel{ID: 1}, AuthorID: 100}

	err := svc.Delete(1, 200, false)
	if err == nil {
		t.Fatal("expected forbidden error")
	}
	var appErr *errs.AppError
	if !errs.Is(err, &appErr) || appErr.Code != "FORBIDDEN" {
		t.Fatalf("expected FORBIDDEN, got %v", err)
	}
	if len(mock.DeletedArticles) != 0 {
		t.Fatal("repo.Delete should not be called when forbidden")
	}
}

func TestMockArticle_Delete_AsOwner(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Articles[1] = &models.Article{BaseModel: models.BaseModel{ID: 1}, AuthorID: 100}

	if err := svc.Delete(1, 100, false); err != nil {
		t.Fatalf("owner should delete: %v", err)
	}
	if len(mock.DeletedArticles) != 1 {
		t.Fatalf("repo.Delete should be called once")
	}
}

func TestMockArticle_BulkAction_AllBranches(t *testing.T) {
	cases := []struct {
		name       string
		action     string
		categoryID *uint
		expectErr  bool
		errCode    string
	}{
		{"publish", "publish", nil, false, ""},
		{"draft", "draft", nil, false, ""},
		{"trash", "trash", nil, false, ""},
		{"delete", "delete", nil, false, ""},
		{"move_with_category", "move", uintPtr(5), false, ""},
		{"move_without_category", "move", nil, true, "BAD_REQUEST"},
		{"pin", "pin", nil, false, ""},
		{"unpin", "unpin", nil, false, ""},
		{"unknown", "unknown", nil, true, "BAD_REQUEST"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newMockArticleService()
			ids := []uint{1, 2, 3}
			affected, err := svc.BulkAction(BulkActionRequest{
				ArticleIDs: ids,
				Action:     tc.action,
				CategoryID: tc.categoryID,
			})

			if tc.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errCode != "" {
					var appErr *errs.AppError
					if !errs.Is(err, &appErr) || appErr.Code != tc.errCode {
						t.Fatalf("expected err_code %s, got %v", tc.errCode, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if affected != int64(len(ids)) {
				t.Fatalf("expected affected=%d, got %d", len(ids), affected)
			}
		})
	}
}

func TestMockArticle_RestoreRevision_Success(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Articles[1] = &models.Article{BaseModel: models.BaseModel{ID: 1}, Title: "Current"}
	mock.Revision = &models.Revision{BaseModel: models.BaseModel{ID: 10}, ArticleID: 1, Version: 2}

	err := svc.RestoreRevision(1, 10, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMockArticle_RestoreRevision_RevisionNotFound(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.FindRevisionErr = gorm.ErrRecordNotFound

	err := svc.RestoreRevision(1, 99, 100)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestMockArticle_RestoreRevision_ArticleNotFound(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Revision = &models.Revision{BaseModel: models.BaseModel{ID: 10}, ArticleID: 1}
	mock.FindByIDErr = gorm.ErrRecordNotFound

	err := svc.RestoreRevision(1, 10, 100)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestMockArticle_LikeArticle(t *testing.T) {
	svc, mock := newMockArticleService()

	if err := svc.LikeArticle(42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.LikeCountIncs) != 1 || mock.LikeCountIncs[0] != 42 {
		t.Fatalf("IncrementLikeCount should be called with 42, got %v", mock.LikeCountIncs)
	}
}

func TestMockArticle_GenerateFeed_Empty(t *testing.T) {
	svc, _ := newMockArticleService()

	xml, err := svc.GenerateFeed()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if xml == "" {
		t.Fatal("feed should not be empty")
	}
	if !contains(xml, "<rss") || !contains(xml, "</rss>") {
		t.Fatalf("feed should be valid RSS XML, got: %s", xml[:min(100, len(xml))])
	}
	if contains(xml, "<item>") {
		t.Fatal("empty feed should not contain <item>")
	}
}

func TestMockArticle_GenerateFeed_WithData(t *testing.T) {
	svc, mock := newMockArticleService()
	now := time.Now()
	mock.PublishedForFeed = []models.Article{
		{
			BaseModel:   models.BaseModel{ID: 1},
			Title:       "First Post",
			Slug:        "first-post",
			Excerpt:     "An excerpt",
			PublishedAt: &now,
			Author:      models.User{Email: "author@example.com", DisplayName: "Author"},
		},
		{
			BaseModel:   models.BaseModel{ID: 2},
			Title:       "Second <Post>",
			Slug:        "second-post",
			Excerpt:     "Another excerpt",
			PublishedAt: &now,
		},
	}

	xml, err := svc.GenerateFeed()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(xml, "<item>") {
		t.Fatal("feed should contain items")
	}
	if !contains(xml, "first-post") {
		t.Fatal("feed should contain first article slug")
	}
	// XML 特殊字符应被转义
	if contains(xml, "<Post>") {
		t.Fatal("title with < > should be XML-escaped")
	}
	if !strings.Contains(xml, "&lt;Post&gt;") {
		t.Fatal("title should be XML-escaped as &lt;Post&gt;")
	}
}

func TestMockArticle_List_PaginationDefaults(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.ArticlesList = []models.Article{
		{BaseModel: models.BaseModel{ID: 1}},
		{BaseModel: models.BaseModel{ID: 2}},
	}
	mock.ListTotal = 2

	// 传入无效分页参数，应被修正为默认值
	resp, err := svc.List(ListArticlesFilter{Page: 0, PageSize: 0, Sort: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Page != 1 {
		t.Errorf("Page should default to 1, got %d", resp.Page)
	}
	if resp.PageSize != 20 {
		t.Errorf("PageSize should default to 20, got %d", resp.PageSize)
	}
	if resp.Total != 2 {
		t.Errorf("Total should be 2, got %d", resp.Total)
	}
}

func TestMockArticle_List_PageSizeClamped(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.ArticlesList = []models.Article{}

	// PageSize > 100 应被限制为 20
	resp, err := svc.List(ListArticlesFilter{Page: 1, PageSize: 200})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.PageSize != 20 {
		t.Fatalf("PageSize should be clamped to 20, got %d", resp.PageSize)
	}
}

func TestMockArticle_List_RepoError(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.ListErr = errors.New("database timeout")

	_, err := svc.List(ListArticlesFilter{Page: 1, PageSize: 10})
	if err == nil {
		t.Fatal("expected error from repo.List")
	}
}

func TestMockArticle_Get_RepoError(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.GetByIDErr = errors.New("connection lost")

	_, err := svc.Get(1)
	if err == nil {
		t.Fatal("expected error from repo.GetByID")
	}
}

func TestMockArticle_Revisions(t *testing.T) {
	svc, mock := newMockArticleService()
	mock.Revisions = []models.Revision{
		{BaseModel: models.BaseModel{ID: 1}, Version: 1},
		{BaseModel: models.BaseModel{ID: 2}, Version: 2},
	}

	revs, err := svc.Revisions(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(revs))
	}
}

// ---------- helpers ----------
// 注：contains / containsStr / uintPtr 已在 article_service_test.go / category_service_test.go 定义，
// 同一 package 内可直接复用。
