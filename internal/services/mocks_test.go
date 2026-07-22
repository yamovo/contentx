package services

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────────────────────────────────────
// Hand-written mock repositories for unit testing.
//
// 这些 mock 实现了 repository 接口，用于在不需要数据库的情况下测试
// Service 层的业务逻辑。每个 mock 通过字段存储预置数据或错误，
// 方法返回这些预置结果，使测试可以精确控制 repo 层的返回值。
// ──────────────────────────────────────────────────────────────────────────────

// ---------- MockArticleRepository ----------

// MockArticleRepository 实现 repository.ArticleRepository，用于 ArticleService 单测。
type MockArticleRepository struct {
	// 预置数据
	Articles           map[uint]*models.Article
	ArticlesList       []models.Article
	ListTotal          int64
	Revisions          []models.Revision
	Revision           *models.Revision
	PublishedForFeed   []models.Article
	ScheduledDue       []models.Article // ListScheduledDue 返回值
	// i18n
	Translations       []models.Article            // ListTranslations 返回值
	TranslationByLocale map[string]*models.Article // FindTranslationInLocale 按 locale 返回

	// 错误控制
	ListErr            error
	GetByIDErr         error
	FindByIDErr        error
	GetBySlugErr       error
	CreateErr          error
	UpdateErr          error
	DeleteErr          error
	BulkPublishErr     error
	BulkUpdateStatusErr error
	BulkDeleteErr      error
	BulkMoveCategoryErr error
	BulkSetPinnedErr   error
	ListRevisionsErr   error
	FindRevisionErr    error
	RestoreRevisionErr error
	IncrementViewErr   error
	IncrementLikeErr   error
	UpdateStatusErr    error
	ListScheduledDueErr error

	// 调用追踪
	ViewCountIncs      []uint
	LikeCountIncs      []uint
	CreatedArticles    []*models.Article
	UpdatedArticles    []*models.Article
	DeletedArticles    []*models.Article
	EnsureUniqueCalls  []string
	UniqueSlugSuffix   string // 返回 original + suffix
	UpdateStatusCalls  []mockUpdateStatusCall
	BulkPublishCalls   []mockBulkPublishCall
}

type mockUpdateStatusCall struct {
	ID          uint
	Status      string
	PublishedAt *time.Time
	ScheduledAt *time.Time
}

type mockBulkPublishCall struct {
	IDs         []uint
	PublishedAt time.Time
}

func (m *MockArticleRepository) List(filter repository.ArticleListFilter) ([]models.Article, int64, error) {
	if m.ListErr != nil {
		return nil, 0, m.ListErr
	}
	return m.ArticlesList, m.ListTotal, nil
}

func (m *MockArticleRepository) GetByID(id uint) (*models.Article, error) {
	if m.GetByIDErr != nil {
		return nil, m.GetByIDErr
	}
	if a, ok := m.Articles[id]; ok {
		return a, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockArticleRepository) FindByID(id uint) (*models.Article, error) {
	if m.FindByIDErr != nil {
		return nil, m.FindByIDErr
	}
	if a, ok := m.Articles[id]; ok {
		return a, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockArticleRepository) GetPublishedBySlug(slug string) (*models.Article, error) {
	if m.GetBySlugErr != nil {
		return nil, m.GetBySlugErr
	}
	for _, a := range m.Articles {
		if a.Slug == slug {
			return a, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockArticleRepository) IncrementViewCount(id uint) error {
	m.ViewCountIncs = append(m.ViewCountIncs, id)
	return m.IncrementViewErr
}

func (m *MockArticleRepository) IncrementLikeCount(id uint) error {
	m.LikeCountIncs = append(m.LikeCountIncs, id)
	return m.IncrementLikeErr
}

func (m *MockArticleRepository) Create(article *models.Article, tagIDs []uint, revisionNote string, userID uint) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	if m.Articles == nil {
		m.Articles = make(map[uint]*models.Article)
	}
	article.ID = uint(len(m.Articles) + 1)
	m.Articles[article.ID] = article
	m.CreatedArticles = append(m.CreatedArticles, article)
	return nil
}

func (m *MockArticleRepository) Update(article *models.Article, updates map[string]interface{}, tagIDs []uint, revisionNote string, userID uint) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	m.UpdatedArticles = append(m.UpdatedArticles, article)
	return nil
}

func (m *MockArticleRepository) Delete(article *models.Article) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	m.DeletedArticles = append(m.DeletedArticles, article)
	return nil
}

func (m *MockArticleRepository) BulkPublish(articleIDs []uint, publishedAt time.Time) (int64, error) {
	m.BulkPublishCalls = append(m.BulkPublishCalls, mockBulkPublishCall{IDs: articleIDs, PublishedAt: publishedAt})
	return int64(len(articleIDs)), m.BulkPublishErr
}

func (m *MockArticleRepository) UpdateStatus(id uint, status string, publishedAt, scheduledAt *time.Time) error {
	m.UpdateStatusCalls = append(m.UpdateStatusCalls, mockUpdateStatusCall{
		ID: id, Status: status, PublishedAt: publishedAt, ScheduledAt: scheduledAt,
	})
	if m.UpdateStatusErr != nil {
		return m.UpdateStatusErr
	}
	// Mirror the GORM repo: mutate the in-memory article so FindByID reflects
	// the new state (enables workflow tests to assert post-transition status).
	if a, ok := m.Articles[id]; ok {
		a.Status = models.ArticleStatus(status)
		if publishedAt != nil {
			a.PublishedAt = publishedAt
		}
		if scheduledAt != nil {
			a.ScheduledAt = scheduledAt
		}
	}
	return nil
}

func (m *MockArticleRepository) BulkUpdateStatus(articleIDs []uint, status string) (int64, error) {
	return int64(len(articleIDs)), m.BulkUpdateStatusErr
}

func (m *MockArticleRepository) BulkDelete(articleIDs []uint) (int64, error) {
	return int64(len(articleIDs)), m.BulkDeleteErr
}

func (m *MockArticleRepository) BulkMoveCategory(articleIDs []uint, categoryID uint) (int64, error) {
	return int64(len(articleIDs)), m.BulkMoveCategoryErr
}

func (m *MockArticleRepository) BulkSetPinned(articleIDs []uint, pinned bool) (int64, error) {
	return int64(len(articleIDs)), m.BulkSetPinnedErr
}

func (m *MockArticleRepository) ListRevisions(articleID uint) ([]models.Revision, error) {
	if m.ListRevisionsErr != nil {
		return nil, m.ListRevisionsErr
	}
	return m.Revisions, nil
}

func (m *MockArticleRepository) FindRevision(revisionID, articleID uint) (*models.Revision, error) {
	if m.FindRevisionErr != nil {
		return nil, m.FindRevisionErr
	}
	return m.Revision, nil
}

func (m *MockArticleRepository) RestoreRevision(article *models.Article, revision *models.Revision, userID uint) error {
	return m.RestoreRevisionErr
}

func (m *MockArticleRepository) ListPublishedForFeed(limit int) ([]models.Article, error) {
	return m.PublishedForFeed, nil
}

func (m *MockArticleRepository) ListScheduledDue(now time.Time) ([]models.Article, error) {
	if m.ListScheduledDueErr != nil {
		return nil, m.ListScheduledDueErr
	}
	return m.ScheduledDue, nil
}

func (m *MockArticleRepository) EnsureUniqueSlug(original string, excludeID uint) string {
	m.EnsureUniqueCalls = append(m.EnsureUniqueCalls, original)
	if m.UniqueSlugSuffix != "" {
		return original + m.UniqueSlugSuffix
	}
	return original
}

// i18n translation stubs (return zero values by default; tests can swap them).
func (m *MockArticleRepository) ListTranslations(groupID, excludeID uint) ([]models.Article, error) {
	if m.Translations != nil {
		return m.Translations, nil
	}
	return nil, nil
}
func (m *MockArticleRepository) FindTranslationInLocale(groupID uint, locale string) (*models.Article, error) {
	if m.TranslationByLocale != nil {
		if a, ok := m.TranslationByLocale[locale]; ok {
			return a, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// ---------- MockAuthRepository ----------

// MockAuthRepository 实现 repository.AuthRepository，用于 AuthService 单测。
type MockAuthRepository struct {
	// 预置数据
	UserByUsernameOrEmail *models.User
	UserByIDWithRole      *models.User
	UserByIDWithPerms     *models.User
	UserByID              *models.User
	CountByUsernameOrEmail int64
	DefaultRole           *models.Role
	RoleBySlug            *models.Role
	Setting               *models.SiteSetting

	// 错误控制
	FindUserByUsernameOrEmailErr error
	FindUserByIDWithRoleErr      error
	FindUserByIDWithPermsErr     error
	FindUserByIDErr              error
	CountUsersErr                error
	CreateUserErr                error
	UpdateUserFieldsErr          error
	UpdateUserPasswordErr        error
	FindDefaultRoleErr           error
	FindRoleBySlugErr            error
	CreateActivityLogErr         error
	FindSettingErr               error

	// 调用追踪
	CreatedUsers        []*models.User
	UpdatedUserFields   []map[string]interface{}
	UpdatedPasswords    []struct {
		ID       uint
		Password string
	}
	CreatedActivityLogs []*models.ActivityLog
	FindSettingCalls   []string
}

func (m *MockAuthRepository) FindUserByUsernameOrEmail(identifier string) (*models.User, error) {
	if m.FindUserByUsernameOrEmailErr != nil {
		return nil, m.FindUserByUsernameOrEmailErr
	}
	return m.UserByUsernameOrEmail, nil
}

func (m *MockAuthRepository) FindUserByIDWithRole(id uint) (*models.User, error) {
	if m.FindUserByIDWithRoleErr != nil {
		return nil, m.FindUserByIDWithRoleErr
	}
	return m.UserByIDWithRole, nil
}

func (m *MockAuthRepository) FindUserByIDWithPermissions(id uint) (*models.User, error) {
	if m.FindUserByIDWithPermsErr != nil {
		return nil, m.FindUserByIDWithPermsErr
	}
	return m.UserByIDWithPerms, nil
}

func (m *MockAuthRepository) FindUserByID(id uint) (*models.User, error) {
	if m.FindUserByIDErr != nil {
		return nil, m.FindUserByIDErr
	}
	return m.UserByID, nil
}

func (m *MockAuthRepository) CountUsersByUsernameOrEmail(username, email string) (int64, error) {
	if m.CountUsersErr != nil {
		return 0, m.CountUsersErr
	}
	return m.CountByUsernameOrEmail, nil
}

func (m *MockAuthRepository) CreateUser(user *models.User) error {
	if m.CreateUserErr != nil {
		return m.CreateUserErr
	}
	m.CreatedUsers = append(m.CreatedUsers, user)
	return nil
}

func (m *MockAuthRepository) UpdateUserFields(id uint, updates map[string]interface{}) error {
	if m.UpdateUserFieldsErr != nil {
		return m.UpdateUserFieldsErr
	}
	m.UpdatedUserFields = append(m.UpdatedUserFields, updates)
	return nil
}

func (m *MockAuthRepository) UpdateUserPassword(id uint, hashedPassword string) error {
	if m.UpdateUserPasswordErr != nil {
		return m.UpdateUserPasswordErr
	}
	m.UpdatedPasswords = append(m.UpdatedPasswords, struct {
		ID       uint
		Password string
	}{ID: id, Password: hashedPassword})
	return nil
}

func (m *MockAuthRepository) FindDefaultRole() (*models.Role, error) {
	if m.FindDefaultRoleErr != nil {
		return nil, m.FindDefaultRoleErr
	}
	return m.DefaultRole, nil
}

func (m *MockAuthRepository) FindRoleBySlug(slug string) (*models.Role, error) {
	if m.FindRoleBySlugErr != nil {
		return nil, m.FindRoleBySlugErr
	}
	return m.RoleBySlug, nil
}

func (m *MockAuthRepository) CreateActivityLog(log *models.ActivityLog) error {
	if m.CreateActivityLogErr != nil {
		return m.CreateActivityLogErr
	}
	m.CreatedActivityLogs = append(m.CreatedActivityLogs, log)
	return nil
}

func (m *MockAuthRepository) FindSetting(key string) (*models.SiteSetting, error) {
	m.FindSettingCalls = append(m.FindSettingCalls, key)
	if m.FindSettingErr != nil {
		return nil, m.FindSettingErr
	}
	return m.Setting, nil
}

// ---------- MockCommentRepository ----------

// MockCommentRepository 实现 repository.CommentRepository，用于 CommentService 单测。
type MockCommentRepository struct {
	// 预置数据
	ListComments    []models.Comment
	ListTotal       int64
	Comment         *models.Comment
	Article         *models.Article
	ParentComment   *models.Comment
	ArticleComments []models.Comment
	StatsData       repository.CommentStatsData

	// 错误控制
	ListErr                  error
	GetByIDErr               error
	FindArticleByIDErr       error
	FindCommentByIDErr       error
	CreateErr                error
	UpdateContentErr         error
	UpdateStatusErr          error
	BulkUpdateStatusErr      error
	BulkDeleteErr            error
	FindArticleCommentsErr   error
	IncrementArticleCountErr error
	StatsErr                 error
	CountTodayErr            error

	// 行为控制
	UpdateContentRows int64 // 默认 1
	UpdateStatusRows  int64
	BulkUpdateRows    int64
	BulkDeleteRows    int64

	// 调用追踪
	CreatedComments             []*models.Comment
	UpdatedContent              []struct {
		ID      uint
		Content string
	}
	UpdatedStatus               []struct {
		ID     uint
		Status string
	}
	BulkUpdatedStatus           []struct {
		IDs    []uint
		Status string
	}
	BulkDeletedIDs              []uint
	ArticleCommentCountIncs     []uint
	StatsCalls                  int
	CountTodayCalls             int
}

func (m *MockCommentRepository) List(filter repository.CommentListFilter) ([]models.Comment, int64, error) {
	if m.ListErr != nil {
		return nil, 0, m.ListErr
	}
	return m.ListComments, m.ListTotal, nil
}

func (m *MockCommentRepository) GetByID(id uint) (*models.Comment, error) {
	if m.GetByIDErr != nil {
		return nil, m.GetByIDErr
	}
	return m.Comment, nil
}

func (m *MockCommentRepository) FindArticleByID(articleID uint) (*models.Article, error) {
	if m.FindArticleByIDErr != nil {
		return nil, m.FindArticleByIDErr
	}
	return m.Article, nil
}

func (m *MockCommentRepository) FindCommentByID(id uint) (*models.Comment, error) {
	if m.FindCommentByIDErr != nil {
		return nil, m.FindCommentByIDErr
	}
	return m.ParentComment, nil
}

func (m *MockCommentRepository) Create(comment *models.Comment) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.CreatedComments = append(m.CreatedComments, comment)
	return nil
}

func (m *MockCommentRepository) UpdateContent(id uint, content string) (int64, error) {
	m.UpdatedContent = append(m.UpdatedContent, struct {
		ID      uint
		Content string
	}{ID: id, Content: content})
	if m.UpdateContentErr != nil {
		return 0, m.UpdateContentErr
	}
	if m.UpdateContentRows == 0 {
		return 1, nil
	}
	return m.UpdateContentRows, nil
}

func (m *MockCommentRepository) UpdateStatus(id uint, status string) (int64, error) {
	m.UpdatedStatus = append(m.UpdatedStatus, struct {
		ID     uint
		Status string
	}{ID: id, Status: status})
	if m.UpdateStatusErr != nil {
		return 0, m.UpdateStatusErr
	}
	if m.UpdateStatusRows == 0 {
		return 1, nil
	}
	return m.UpdateStatusRows, nil
}

func (m *MockCommentRepository) BulkUpdateStatus(ids []uint, status string) (int64, error) {
	m.BulkUpdatedStatus = append(m.BulkUpdatedStatus, struct {
		IDs    []uint
		Status string
	}{IDs: ids, Status: status})
	if m.BulkUpdateStatusErr != nil {
		return 0, m.BulkUpdateStatusErr
	}
	if m.BulkUpdateRows == 0 {
		return int64(len(ids)), nil
	}
	return m.BulkUpdateRows, nil
}

func (m *MockCommentRepository) BulkDelete(ids []uint) (int64, error) {
	m.BulkDeletedIDs = append(m.BulkDeletedIDs, ids...)
	if m.BulkDeleteErr != nil {
		return 0, m.BulkDeleteErr
	}
	if m.BulkDeleteRows == 0 {
		return int64(len(ids)), nil
	}
	return m.BulkDeleteRows, nil
}

func (m *MockCommentRepository) FindArticleComments(articleID uint) ([]models.Comment, error) {
	if m.FindArticleCommentsErr != nil {
		return nil, m.FindArticleCommentsErr
	}
	return m.ArticleComments, nil
}

func (m *MockCommentRepository) IncrementArticleCommentCount(articleID uint) error {
	m.ArticleCommentCountIncs = append(m.ArticleCommentCountIncs, articleID)
	return m.IncrementArticleCountErr
}

func (m *MockCommentRepository) Stats() (repository.CommentStatsData, error) {
	m.StatsCalls++
	if m.StatsErr != nil {
		return repository.CommentStatsData{}, m.StatsErr
	}
	return m.StatsData, nil
}

func (m *MockCommentRepository) CountToday() (int64, error) {
	m.CountTodayCalls++
	if m.CountTodayErr != nil {
		return 0, m.CountTodayErr
	}
	return m.StatsData.Today, nil
}

// ---------- MockWebhookRepository ----------

// MockWebhookRepository 实现 repository.WebhookRepository，用于 WebhookService 单测。
type MockWebhookRepository struct {
	// 预置数据
	Webhook        *models.Webhook
	WebhooksList   []models.Webhook
	ActiveWebhooks []models.Webhook
	Logs           []models.WebhookLog

	// 错误控制
	CreateErr      error
	ListErr        error
	GetByIDErr     error
	DeleteErr      error
	ListLogsErr    error
	CreateLogErr   error
	ListActiveErr  error

	// 行为控制
	DeleteRows int64

	// 调用追踪
	CreatedWebhooks []*models.Webhook
	CreatedLogs     []*models.WebhookLog
	ListActiveCalls int
}

func (m *MockWebhookRepository) Create(wh *models.Webhook) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.CreatedWebhooks = append(m.CreatedWebhooks, wh)
	return nil
}

func (m *MockWebhookRepository) List() ([]models.Webhook, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	return m.WebhooksList, nil
}

func (m *MockWebhookRepository) GetByID(id uint) (*models.Webhook, error) {
	if m.GetByIDErr != nil {
		return nil, m.GetByIDErr
	}
	return m.Webhook, nil
}

func (m *MockWebhookRepository) Delete(id uint) (int64, error) {
	if m.DeleteErr != nil {
		return 0, m.DeleteErr
	}
	if m.DeleteRows == 0 {
		return 1, nil
	}
	return m.DeleteRows, nil
}

func (m *MockWebhookRepository) ListLogs(webhookID uint, limit int) ([]models.WebhookLog, error) {
	if m.ListLogsErr != nil {
		return nil, m.ListLogsErr
	}
	return m.Logs, nil
}

func (m *MockWebhookRepository) CreateLog(log *models.WebhookLog) error {
	if m.CreateLogErr != nil {
		return m.CreateLogErr
	}
	m.CreatedLogs = append(m.CreatedLogs, log)
	return nil
}

func (m *MockWebhookRepository) ListActive() ([]models.Webhook, error) {
	m.ListActiveCalls++
	if m.ListActiveErr != nil {
		return nil, m.ListActiveErr
	}
	return m.ActiveWebhooks, nil
}

// ---------- MockMediaRepository ----------

// MockMediaRepository 实现 repository.MediaRepository，用于 MediaService 单测。
type MockMediaRepository struct {
	// 预置数据
	MediaList    []models.Media
	ListTotal    int64
	Media        *models.Media
	MediaByIDMap map[uint]*models.Media // GetByID 查找
	FindMedia    *models.Media          // FindByID 返回
	FindByIDsRes []models.Media
	Folders      []string
	StatsData    repository.MediaStatsData

	// 错误控制
	ListErr        error
	GetByIDErr     error
	FindByIDErr    error
	FindByIDsErr   error
	CreateErr      error
	UpdateFieldsErr error
	DeleteErr      error
	DeleteByIDsErr error
	ListFoldersErr error
	StatsErr       error

	// 行为控制
	DeleteByIDsRows int64 // DeleteByIDs 返回的行数（默认 0 → 返回 len(ids)）

	// 调用跟踪
	CreatedMedia      []*models.Media
	UpdatedFields     []updatedFieldsCall
	DeletedMedia      []*models.Media
	DeletedByIDs      []uint
	DeleteByIDsCalls  int
}

type updatedFieldsCall struct {
	ID      uint
	Updates map[string]interface{}
}

func (m *MockMediaRepository) List(filter repository.MediaListFilter) ([]models.Media, int64, error) {
	if m.ListErr != nil {
		return nil, 0, m.ListErr
	}
	return m.MediaList, m.ListTotal, nil
}

func (m *MockMediaRepository) GetByID(id uint) (*models.Media, error) {
	if m.GetByIDErr != nil {
		return nil, m.GetByIDErr
	}
	if m.MediaByIDMap != nil {
		if med, ok := m.MediaByIDMap[id]; ok {
			return med, nil
		}
	}
	return m.Media, nil
}

func (m *MockMediaRepository) FindByID(id uint) (*models.Media, error) {
	if m.FindByIDErr != nil {
		return nil, m.FindByIDErr
	}
	return m.FindMedia, nil
}

func (m *MockMediaRepository) FindByIDs(ids []uint) ([]models.Media, error) {
	if m.FindByIDsErr != nil {
		return nil, m.FindByIDsErr
	}
	return m.FindByIDsRes, nil
}

func (m *MockMediaRepository) Create(media *models.Media) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.CreatedMedia = append(m.CreatedMedia, media)
	return nil
}

func (m *MockMediaRepository) UpdateFields(id uint, updates map[string]interface{}) error {
	if m.UpdateFieldsErr != nil {
		return m.UpdateFieldsErr
	}
	m.UpdatedFields = append(m.UpdatedFields, updatedFieldsCall{ID: id, Updates: updates})
	return nil
}

func (m *MockMediaRepository) Delete(media *models.Media) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	m.DeletedMedia = append(m.DeletedMedia, media)
	return nil
}

func (m *MockMediaRepository) DeleteByIDs(ids []uint) (int64, error) {
	m.DeleteByIDsCalls++
	if m.DeleteByIDsErr != nil {
		return 0, m.DeleteByIDsErr
	}
	m.DeletedByIDs = append(m.DeletedByIDs, ids...)
	if m.DeleteByIDsRows != 0 {
		return m.DeleteByIDsRows, nil
	}
	return int64(len(ids)), nil
}

func (m *MockMediaRepository) ListFolders() ([]string, error) {
	if m.ListFoldersErr != nil {
		return nil, m.ListFoldersErr
	}
	return m.Folders, nil
}

func (m *MockMediaRepository) Stats() (repository.MediaStatsData, error) {
	if m.StatsErr != nil {
		return repository.MediaStatsData{}, m.StatsErr
	}
	return m.StatsData, nil
}

// ---------- MockUserRepository ----------

// MockUserRepository 实现 repository.UserRepository，用于 UserService 单测。
type MockUserRepository struct {
	Users    []models.User
	UserByID *models.User

	ListErr           error
	GetByIDErr        error
	FindByIDErr       error
	CreateErr         error
	UpdateFieldsErr   error
	UpdatePasswordErr error
	SoftDeleteErr     error

	CreatedUsers []*models.User
}

func (m *MockUserRepository) List(filter repository.UserListFilter) ([]models.User, int64, error) {
	if m.ListErr != nil {
		return nil, 0, m.ListErr
	}
	return m.Users, int64(len(m.Users)), nil
}

func (m *MockUserRepository) GetByID(id uint) (*models.User, error) {
	if m.GetByIDErr != nil {
		return nil, m.GetByIDErr
	}
	return m.UserByID, nil
}

func (m *MockUserRepository) FindByID(id uint) (*models.User, error) {
	if m.FindByIDErr != nil {
		return nil, m.FindByIDErr
	}
	return m.UserByID, nil
}

func (m *MockUserRepository) Create(user *models.User) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	user.ID = uint(len(m.CreatedUsers) + 1)
	m.CreatedUsers = append(m.CreatedUsers, user)
	return nil
}

func (m *MockUserRepository) UpdateFields(id uint, updates map[string]interface{}) error {
	if m.UpdateFieldsErr != nil {
		return m.UpdateFieldsErr
	}
	return nil
}

func (m *MockUserRepository) UpdatePassword(id uint, hashedPassword string) error {
	if m.UpdatePasswordErr != nil {
		return m.UpdatePasswordErr
	}
	return nil
}

func (m *MockUserRepository) SoftDelete(user *models.User) error {
	if m.SoftDeleteErr != nil {
		return m.SoftDeleteErr
	}
	return nil
}

// ---------- MockContentTypeRepository ----------

// MockContentTypeRepository 实现 repository.ContentTypeRepository，用于 ContentTypeService 单测。
type MockContentTypeRepository struct {
	ContentType  *models.ContentType
	ContentTypes []models.ContentType
	CountVal     int64

	FindByUIDErr  error
	FindByIDErr   error
	CreateErr     error
	ListErr       error
	DeleteErr     error
	CountErr      error

	CreatedCT  *models.ContentType
	DeletedIDs []uint
}

func (m *MockContentTypeRepository) CountByUID(uid string) (int64, error) {
	if m.CountErr != nil {
		return 0, m.CountErr
	}
	return m.CountVal, nil
}

func (m *MockContentTypeRepository) Create(ct *models.ContentType) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.CreatedCT = ct
	return nil
}

func (m *MockContentTypeRepository) List() ([]models.ContentType, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	return m.ContentTypes, nil
}

func (m *MockContentTypeRepository) FindByUID(uid string) (*models.ContentType, error) {
	if m.FindByUIDErr != nil {
		return nil, m.FindByUIDErr
	}
	return m.ContentType, nil
}

func (m *MockContentTypeRepository) FindByID(id uint) (*models.ContentType, error) {
	if m.FindByIDErr != nil {
		return nil, m.FindByIDErr
	}
	return m.ContentType, nil
}

func (m *MockContentTypeRepository) Delete(id uint) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	m.DeletedIDs = append(m.DeletedIDs, id)
	return nil
}

func (m *MockContentTypeRepository) CountEntriesByTypeID(typeID uint) (int64, error) {
	if m.CountErr != nil {
		return 0, m.CountErr
	}
	return m.CountVal, nil
}

// ---------- MockContentEntryRepository ----------

// MockContentEntryRepository 实现 repository.ContentEntryRepository，用于 ContentTypeService 单测。
type MockContentEntryRepository struct {
	Entries     []models.ContentEntry
	Entry       *models.ContentEntry
	CountVal    int64
	CreateN     int
	// i18n
	EntryTranslations       []models.ContentEntry
	EntryTranslationByLocale map[string]*models.ContentEntry

	FindByIDsErr       error
	FindByDocumentErr  error
	CreateErr          error
	SaveErr            error
	DeleteByDocIDErr   error
	ListErr            error
	SearchErr          error
	ExportErr          error
	CreateManyErr      error

	CreatedEntries []*models.ContentEntry
	SavedEntries   []*models.ContentEntry
}

func (m *MockContentEntryRepository) FindByDocumentID(typeID uint, docID string) (*models.ContentEntry, error) {
	if m.FindByDocumentErr != nil {
		return nil, m.FindByDocumentErr
	}
	return m.Entry, nil
}

func (m *MockContentEntryRepository) Create(entry *models.ContentEntry) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.CreatedEntries = append(m.CreatedEntries, entry)
	return nil
}

func (m *MockContentEntryRepository) Save(entry *models.ContentEntry) error {
	if m.SaveErr != nil {
		return m.SaveErr
	}
	m.SavedEntries = append(m.SavedEntries, entry)
	return nil
}

func (m *MockContentEntryRepository) DeleteByDocumentID(typeID uint, docID string) (int64, error) {
	if m.DeleteByDocIDErr != nil {
		return 0, m.DeleteByDocIDErr
	}
	return 1, nil
}

func (m *MockContentEntryRepository) List(filter repository.ContentEntryListFilter) ([]models.ContentEntry, int64, error) {
	if m.ListErr != nil {
		return nil, 0, m.ListErr
	}
	return m.Entries, m.CountVal, nil
}

func (m *MockContentEntryRepository) FindByIDs(typeID uint, ids []uint) ([]models.ContentEntry, error) {
	if m.FindByIDsErr != nil {
		return nil, m.FindByIDsErr
	}
	return m.Entries, nil
}

func (m *MockContentEntryRepository) Search(typeID uint, query string, limit int) ([]models.ContentEntry, error) {
	if m.SearchErr != nil {
		return nil, m.SearchErr
	}
	return m.Entries, nil
}

func (m *MockContentEntryRepository) ExportAll(typeID uint) ([]models.ContentEntry, error) {
	if m.ExportErr != nil {
		return nil, m.ExportErr
	}
	return m.Entries, nil
}

func (m *MockContentEntryRepository) CreateMany(entries []models.ContentEntry) (int, error) {
	if m.CreateManyErr != nil {
		return 0, m.CreateManyErr
	}
	return len(entries), nil
}

// i18n translation stubs.
func (m *MockContentEntryRepository) ListTranslations(typeID, groupID, excludeID uint) ([]models.ContentEntry, error) {
	if m.EntryTranslations != nil {
		return m.EntryTranslations, nil
	}
	return nil, nil
}
func (m *MockContentEntryRepository) FindTranslationInLocale(typeID, groupID uint, locale string) (*models.ContentEntry, error) {
	if m.EntryTranslationByLocale != nil {
		if e, ok := m.EntryTranslationByLocale[locale]; ok {
			return e, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// ---------- MockThemeRepository ----------

// MockThemeRepository 实现 repository.ThemeRepository，用于 ThemeService 单测。
type MockThemeRepository struct {
	Theme  *models.ThemeConfig
	Themes []models.ThemeConfig

	FindByIDErr        error
	ListErr            error
	DeactivateAllErr   error
	UpdateActiveErr    error
	SaveErr            error

	SavedTheme     *models.ThemeConfig
	DeactivatedID  uint
	UpdatedActiveID uint
	UpdatedActiveVal bool
}

func (m *MockThemeRepository) List() ([]models.ThemeConfig, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	return m.Themes, nil
}

func (m *MockThemeRepository) FindByID(id uint) (*models.ThemeConfig, error) {
	if m.FindByIDErr != nil {
		return nil, m.FindByIDErr
	}
	return m.Theme, nil
}

func (m *MockThemeRepository) DeactivateAllExcept(id uint) error {
	if m.DeactivateAllErr != nil {
		return m.DeactivateAllErr
	}
	m.DeactivatedID = id
	return nil
}

func (m *MockThemeRepository) UpdateActive(id uint, active bool) error {
	if m.UpdateActiveErr != nil {
		return m.UpdateActiveErr
	}
	m.UpdatedActiveID = id
	m.UpdatedActiveVal = active
	return nil
}

func (m *MockThemeRepository) Save(theme *models.ThemeConfig) error {
	if m.SaveErr != nil {
		return m.SaveErr
	}
	m.SavedTheme = theme
	return nil
}

// ---------- MockCacheDriver ----------

// MockCacheDriver 实现 cache.Driver，用于 ContentTypeService 缓存测试。
type MockCacheDriver struct {
	Data    map[string][]byte
	GetErr  error
	SetErr  error
	DelErr  error
	FlushErr error

	SetCalls    []setCall
	DeleteCalls []string
}

type setCall struct {
	Key   string
	Value []byte
	TTL   time.Duration
}

func (m *MockCacheDriver) Get(ctx context.Context, key string) ([]byte, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	if m.Data != nil {
		if data, ok := m.Data[key]; ok {
			return data, nil
		}
	}
	return nil, fmt.Errorf("key not found")
}

func (m *MockCacheDriver) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if m.SetErr != nil {
		return m.SetErr
	}
	m.SetCalls = append(m.SetCalls, setCall{Key: key, Value: value, TTL: ttl})
	return nil
}

func (m *MockCacheDriver) Delete(ctx context.Context, key string) error {
	if m.DelErr != nil {
		return m.DelErr
	}
	m.DeleteCalls = append(m.DeleteCalls, key)
	return nil
}

func (m *MockCacheDriver) Flush(ctx context.Context) error {
	if m.FlushErr != nil {
		return m.FlushErr
	}
	return nil
}

// ---------- MockWebhookDispatcher ----------

// MockWebhookDispatcher 实现 WebhookDispatcher，记录所有 Dispatch 调用。
type MockWebhookDispatcher struct {
	Dispatches []DispatchRecord
}

// DispatchRecord 记录一次 Dispatch 调用的事件名和数据。
type DispatchRecord struct {
	Event string
	Data  interface{}
}

func (m *MockWebhookDispatcher) Dispatch(event string, data interface{}) {
	m.Dispatches = append(m.Dispatches, DispatchRecord{Event: event, Data: data})
}

// Last 返回最后一次 Dispatch 记录，没有则返回零值。
func (m *MockWebhookDispatcher) Last() (DispatchRecord, bool) {
	if len(m.Dispatches) == 0 {
		return DispatchRecord{}, false
	}
	return m.Dispatches[len(m.Dispatches)-1], true
}

// ---------- MockAnalyticsRepository ----------

// MockAnalyticsRepository 实现 repository.AnalyticsRepository，用于 AnalyticsService 单测。
type MockAnalyticsRepository struct {
	// 预置数据
	DashboardStatsData repository.DashboardStatsData
	RecentArticlesData []models.Article
	RecentCommentsData []models.Comment
	PopularData        []models.Article
	ViewsOverTimeData  []repository.DayStatsData
	TopReferrersData    []repository.ReferrerData
	DeviceBreakdownData repository.DeviceBreakdownData

	// 错误控制
	DashboardStatsErr    error
	RecentArticlesErr    error
	RecentCommentsErr    error
	PopularArticlesErr   error
	ViewsOverTimeErr     error
	TopReferrersErr      error
	DeviceBreakdownErr   error
	CreatePageViewErr    error

	// 调用追踪
	CreatedPageViews []*models.PageView
}

func (m *MockAnalyticsRepository) DashboardStats() (repository.DashboardStatsData, error) {
	if m.DashboardStatsErr != nil {
		return repository.DashboardStatsData{}, m.DashboardStatsErr
	}
	return m.DashboardStatsData, nil
}

func (m *MockAnalyticsRepository) RecentArticles(limit int) ([]models.Article, error) {
	if m.RecentArticlesErr != nil {
		return nil, m.RecentArticlesErr
	}
	return m.RecentArticlesData, nil
}

func (m *MockAnalyticsRepository) RecentComments(limit int) ([]models.Comment, error) {
	if m.RecentCommentsErr != nil {
		return nil, m.RecentCommentsErr
	}
	return m.RecentCommentsData, nil
}

func (m *MockAnalyticsRepository) PopularArticles(limit int) ([]models.Article, error) {
	if m.PopularArticlesErr != nil {
		return nil, m.PopularArticlesErr
	}
	return m.PopularData, nil
}

func (m *MockAnalyticsRepository) ViewsOverTime(days int) ([]repository.DayStatsData, error) {
	if m.ViewsOverTimeErr != nil {
		return nil, m.ViewsOverTimeErr
	}
	return m.ViewsOverTimeData, nil
}

func (m *MockAnalyticsRepository) TopReferrers(limit int) ([]repository.ReferrerData, error) {
	if m.TopReferrersErr != nil {
		return nil, m.TopReferrersErr
	}
	return m.TopReferrersData, nil
}

func (m *MockAnalyticsRepository) DeviceBreakdown() (repository.DeviceBreakdownData, error) {
	if m.DeviceBreakdownErr != nil {
		return repository.DeviceBreakdownData{}, m.DeviceBreakdownErr
	}
	return m.DeviceBreakdownData, nil
}

func (m *MockAnalyticsRepository) CreatePageView(view *models.PageView) error {
	if m.CreatePageViewErr != nil {
		return m.CreatePageViewErr
	}
	m.CreatedPageViews = append(m.CreatedPageViews, view)
	return nil
}

// ---------- MockStorageDriver ----------

// MockStorageDriver 实现 storage.Driver，用于 MediaService 存储后端单测。
type MockStorageDriver struct {
	// 预置数据
	BaseURL        string // GetURL/GetSignedURL 返回值的前缀
	UploadURLFunc  func(key string) string
	SignedURLFunc  func(key string, ttl time.Duration) string

	// 错误控制
	UploadErr error
	DeleteErr error

	// 调用跟踪
	UploadCalls []mockStorageUploadCall
	DeleteCalls []string
}

type mockStorageUploadCall struct {
	Key         string
	Content     []byte
	ContentType string
}

func (m *MockStorageDriver) Upload(_ context.Context, key string, reader io.Reader, contentType string) (string, error) {
	if m.UploadErr != nil {
		return "", m.UploadErr
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	m.UploadCalls = append(m.UploadCalls, mockStorageUploadCall{Key: key, Content: data, ContentType: contentType})
	if m.UploadURLFunc != nil {
		return m.UploadURLFunc(key), nil
	}
	return m.GetURL(key), nil
}

func (m *MockStorageDriver) Delete(_ context.Context, key string) error {
	// Record the call first so tests can assert the driver was invoked even
	// when DeleteErr is set (removeStoredFile ignores driver errors).
	m.DeleteCalls = append(m.DeleteCalls, key)
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	return nil
}

func (m *MockStorageDriver) GetURL(key string) string {
	return m.BaseURL + "/" + key
}

func (m *MockStorageDriver) GetSignedURL(key string, ttl time.Duration) string {
	if m.SignedURLFunc != nil {
		return m.SignedURLFunc(key, ttl)
	}
	return m.GetURL(key) + "?signed=" + key
}
