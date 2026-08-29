package repository

import (
	"errors"
	"strconv"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ============================================================
// SettingsRepository
// ============================================================

// SettingsRepository defines data-access operations for site settings.
// Site settings carry a nullable tenant_id (NULL = global default, non-NULL =
// tenant override, RFC-001 §4.3): queries always include the global rows and
// the request tenant's overrides.
type SettingsRepository interface {
	List(group string, tenantID uint) ([]models.SiteSetting, error)
	Get(key string, tenantID uint) (*models.SiteSetting, error)
	UpdateValue(key, value string, tenantID uint) (rowsAffected int64, err error)
	UpsertTenantOverride(setting *models.SiteSetting) error
	Create(setting *models.SiteSetting) error // TenantID nil = global default
	ListPublic(tenantID uint) ([]models.SiteSetting, error)
	WithTransaction(fn func(SettingsRepository) error) error
}

// gormSettingsRepository implements SettingsRepository with GORM.
type gormSettingsRepository struct {
	db *gorm.DB
}

// NewSettingsRepository builds a GORM-backed SettingsRepository.
func NewSettingsRepository(db *gorm.DB) SettingsRepository {
	return &gormSettingsRepository{db: db}
}

// tenantScope returns a WHERE fragment matching the request tenant's rows
// plus global (NULL tenant_id) rows.
func tenantScope(tenantID uint) string {
	return "(tenant_id = " + strconv.FormatUint(uint64(tenantID), 10) + " OR tenant_id IS NULL)"
}

func (r *gormSettingsRepository) List(group string, tenantID uint) ([]models.SiteSetting, error) {
	query := r.db.Model(&models.SiteSetting{}).Where(tenantScope(tenantID))
	if group != "" {
		query = query.Where(map[string]interface{}{"group": group})
	}
	var settings []models.SiteSetting
	if err := query.Order("sort_order ASC, id ASC").Find(&settings).Error; err != nil {
		return nil, err
	}
	return preferTenantSettingOverrides(settings, tenantID), nil
}

func (r *gormSettingsRepository) Get(key string, tenantID uint) (*models.SiteSetting, error) {
	var setting models.SiteSetting
	if err := r.db.Where(map[string]interface{}{"key": key, "tenant_id": tenantID}).First(&setting).Error; err == nil {
		return &setting, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if err := r.db.Where(map[string]interface{}{"key": key, "tenant_id": nil}).First(&setting).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}

func (r *gormSettingsRepository) UpdateValue(key, value string, tenantID uint) (int64, error) {
	result := r.db.Model(&models.SiteSetting{}).
		Where(map[string]interface{}{"key": key, "tenant_id": tenantID}).
		Update("value", value)
	return result.RowsAffected, result.Error
}

// UpsertTenantOverride atomically creates a tenant override or updates the
// existing override's value. The composite conflict target is portable across
// SQLite, PostgreSQL, and MySQL; tenant_id must never be NULL on this path.
func (r *gormSettingsRepository) UpsertTenantOverride(setting *models.SiteSetting) error {
	if setting.TenantID == nil {
		return errors.New("tenant override requires tenant_id")
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(setting).Error
}

func (r *gormSettingsRepository) Create(setting *models.SiteSetting) error {
	return r.db.Create(setting).Error
}

// WithTransaction runs a settings operation on a repository bound to one
// database transaction. Callers must not retain the transaction repository
// after fn returns.
func (r *gormSettingsRepository) WithTransaction(fn func(SettingsRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(&gormSettingsRepository{db: tx})
	})
}

func (r *gormSettingsRepository) ListPublic(tenantID uint) ([]models.SiteSetting, error) {
	var settings []models.SiteSetting
	if err := r.db.Where(tenantScope(tenantID)).Order("sort_order ASC, id ASC").Find(&settings).Error; err != nil {
		return nil, err
	}

	effective := preferTenantSettingOverrides(settings, tenantID)
	public := make([]models.SiteSetting, 0, len(effective))
	for _, setting := range effective {
		if setting.IsPublic {
			public = append(public, setting)
		}
	}
	return public, nil
}

// preferTenantSettingOverrides collapses global defaults and tenant overrides
// into the effective settings visible to one tenant. Query order remains stable,
// while a matching tenant row replaces the global row with the same key.
func preferTenantSettingOverrides(settings []models.SiteSetting, tenantID uint) []models.SiteSetting {
	effective := make([]models.SiteSetting, 0, len(settings))
	keyIndex := make(map[string]int, len(settings))

	for _, setting := range settings {
		index, exists := keyIndex[setting.Key]
		if !exists {
			keyIndex[setting.Key] = len(effective)
			effective = append(effective, setting)
			continue
		}
		if setting.TenantID != nil && *setting.TenantID == tenantID {
			effective[index] = setting
		}
	}

	return effective
}

// ============================================================
// SEORepository
// ============================================================

// SEORepository defines data-access operations for SEO settings, redirect rules,
// and the sitemap article query, scoped to a tenant (RFC-001 §5).
type SEORepository interface {
	GetSetting(entityType string, entityID, tenantID uint) (*models.SEOSetting, error)
	CreateSetting(setting *models.SEOSetting) error // setting.TenantID must be set by the caller
	SaveSetting(setting *models.SEOSetting) error
	ListPublishedArticlesForSitemap(tenantID uint) ([]models.Article, error)
	ListRedirects(tenantID uint) ([]models.RedirectRule, error)
	CreateRedirect(rule *models.RedirectRule) error // rule.TenantID must be set by the caller
	DeleteRedirect(id, tenantID uint) error
}

// gormSEORepository implements SEORepository with GORM.
type gormSEORepository struct {
	db *gorm.DB
}

// NewSEORepository builds a GORM-backed SEORepository.
func NewSEORepository(db *gorm.DB) SEORepository {
	return &gormSEORepository{db: db}
}

func (r *gormSEORepository) GetSetting(entityType string, entityID, tenantID uint) (*models.SEOSetting, error) {
	var setting models.SEOSetting
	if err := r.db.Where("entity_type = ? AND entity_id = ? AND tenant_id = ?", entityType, entityID, tenantID).
		First(&setting).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}

func (r *gormSEORepository) CreateSetting(setting *models.SEOSetting) error {
	return r.db.Create(setting).Error
}

func (r *gormSEORepository) SaveSetting(setting *models.SEOSetting) error {
	return r.db.Save(setting).Error
}

func (r *gormSEORepository) ListPublishedArticlesForSitemap(tenantID uint) ([]models.Article, error) {
	var articles []models.Article
	if err := r.db.Where("status = ? AND post_type = ? AND tenant_id = ?", models.StatusPublished, models.PostTypePost, tenantID).
		Order("updated_at DESC").Find(&articles).Error; err != nil {
		return nil, err
	}
	return articles, nil
}

func (r *gormSEORepository) ListRedirects(tenantID uint) ([]models.RedirectRule, error) {
	var rules []models.RedirectRule
	if err := r.db.Where("tenant_id = ?", tenantID).Order("from_path ASC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *gormSEORepository) CreateRedirect(rule *models.RedirectRule) error {
	return r.db.Create(rule).Error
}

func (r *gormSEORepository) DeleteRedirect(id, tenantID uint) error {
	return r.db.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&models.RedirectRule{}).Error
}

// ============================================================
// MenuRepository
// ============================================================

// MenuRepository defines data-access operations for menus and menu items,
// scoped to a tenant (RFC-001 §5).
type MenuRepository interface {
	ListMenus(tenantID uint) ([]models.Menu, error)
	GetMenuByID(id, tenantID uint) (*models.Menu, error)
	FindMenu(id, tenantID uint) (*models.Menu, error)
	CreateMenu(menu *models.Menu) error // menu.TenantID must be set by the caller
	UpdateMenuFields(id uint, fields map[string]interface{}, tenantID uint) error
	DeleteMenu(id, tenantID uint) error
	FindItem(id, tenantID uint) (*models.MenuItem, error)
	CreateItem(item *models.MenuItem) error // item.TenantID must be set by the caller
	UpdateItemFields(id uint, fields map[string]interface{}, tenantID uint) error
	DeleteItem(id, tenantID uint) error
	MaxItemSortOrder(menuID, tenantID uint) (int, error)
}

// gormMenuRepository implements MenuRepository with GORM.
type gormMenuRepository struct {
	db *gorm.DB
}

// NewMenuRepository builds a GORM-backed MenuRepository.
func NewMenuRepository(db *gorm.DB) MenuRepository {
	return &gormMenuRepository{db: db}
}

func (r *gormMenuRepository) ListMenus(tenantID uint) ([]models.Menu, error) {
	var menus []models.Menu
	if err := r.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Where("tenant_id = ?", tenantID).Order("sort_order ASC")
	}).Where("tenant_id = ?", tenantID).Find(&menus).Error; err != nil {
		return nil, err
	}
	return menus, nil
}

func (r *gormMenuRepository) GetMenuByID(id, tenantID uint) (*models.Menu, error) {
	var menu models.Menu
	if err := r.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Where("tenant_id = ?", tenantID).Order("sort_order ASC")
	}).Where("id = ? AND tenant_id = ?", id, tenantID).First(&menu).Error; err != nil {
		return nil, err
	}
	return &menu, nil
}

func (r *gormMenuRepository) FindMenu(id, tenantID uint) (*models.Menu, error) {
	var menu models.Menu
	if err := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&menu).Error; err != nil {
		return nil, err
	}
	return &menu, nil
}

func (r *gormMenuRepository) CreateMenu(menu *models.Menu) error {
	return r.db.Create(menu).Error
}

func (r *gormMenuRepository) UpdateMenuFields(id uint, fields map[string]interface{}, tenantID uint) error {
	return r.db.Model(&models.Menu{}).Where("id = ? AND tenant_id = ?", id, tenantID).Updates(fields).Error
}

func (r *gormMenuRepository) DeleteMenu(id, tenantID uint) error {
	// Best-effort: delete items first, then the menu (mirrors prior service behaviour).
	if err := r.db.Where("menu_id = ? AND tenant_id = ?", id, tenantID).Delete(&models.MenuItem{}).Error; err != nil {
		return err
	}
	return r.db.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&models.Menu{}).Error
}

func (r *gormMenuRepository) FindItem(id, tenantID uint) (*models.MenuItem, error) {
	var item models.MenuItem
	if err := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormMenuRepository) CreateItem(item *models.MenuItem) error {
	return r.db.Create(item).Error
}

func (r *gormMenuRepository) UpdateItemFields(id uint, fields map[string]interface{}, tenantID uint) error {
	return r.db.Model(&models.MenuItem{}).Where("id = ? AND tenant_id = ?", id, tenantID).Updates(fields).Error
}

func (r *gormMenuRepository) DeleteItem(id, tenantID uint) error {
	return r.db.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&models.MenuItem{}).Error
}

func (r *gormMenuRepository) MaxItemSortOrder(menuID, tenantID uint) (int, error) {
	var maxSort int
	if err := r.db.Model(&models.MenuItem{}).Where("menu_id = ? AND tenant_id = ?", menuID, tenantID).
		Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSort).Error; err != nil {
		return 0, err
	}
	return maxSort, nil
}

// ============================================================
// AnalyticsRepository
// ============================================================

// DashboardStatsData holds raw aggregate counts for the dashboard.
type DashboardStatsData struct {
	Articles        int64
	Published       int64
	Comments        int64
	PendingComments int64
	Users           int64
	Media           int64
	ViewsToday      int64
	ViewsThisWeek   int64
	ViewsThisMonth  int64
	TotalViews      int64
}

// DayStatsData holds a single day's view count.
type DayStatsData struct {
	Date  string
	Views int64
}

// ReferrerData holds a referrer URL and its hit count.
type ReferrerData struct {
	Referrer string
	Count    int64
}

// BreakdownData holds a named count.
type BreakdownData struct {
	Name  string
	Count int64
}

// DeviceBreakdownData groups device, browser, and OS breakdowns.
type DeviceBreakdownData struct {
	Devices  []BreakdownData
	Browsers []BreakdownData
	OS       []BreakdownData
}

// AnalyticsRepository defines data-access operations for page views and
// dashboard aggregations.
type AnalyticsRepository interface {
	DashboardStats(tenantID uint) (DashboardStatsData, error)
	RecentArticles(limit int, tenantID uint) ([]models.Article, error)
	RecentComments(limit int, tenantID uint) ([]models.Comment, error)
	PopularArticles(limit int, tenantID uint) ([]models.Article, error)
	ViewsOverTime(days int, tenantID uint) ([]DayStatsData, error)
	TopReferrers(limit int, tenantID uint) ([]ReferrerData, error)
	DeviceBreakdown(tenantID uint) (DeviceBreakdownData, error)
	CreatePageView(view *models.PageView) error
}

// gormAnalyticsRepository implements AnalyticsRepository with GORM.
type gormAnalyticsRepository struct {
	db *gorm.DB
}

// NewAnalyticsRepository builds a GORM-backed AnalyticsRepository.
func NewAnalyticsRepository(db *gorm.DB) AnalyticsRepository {
	return &gormAnalyticsRepository{db: db}
}

func (r *gormAnalyticsRepository) DashboardStats(tenantID uint) (DashboardStatsData, error) {
	var stats DashboardStatsData
	// Mirrors original service behaviour: individual count errors are ignored.
	r.db.Model(&models.Article{}).Where("tenant_id = ?", tenantID).Count(&stats.Articles)
	r.db.Model(&models.Article{}).Where("status = ? AND tenant_id = ?", models.StatusPublished, tenantID).Count(&stats.Published)
	r.db.Model(&models.Comment{}).Where("tenant_id = ?", tenantID).Count(&stats.Comments)
	r.db.Model(&models.Comment{}).Where("status = ? AND tenant_id = ?", "pending", tenantID).Count(&stats.PendingComments)
	r.db.Model(&models.User{}).Count(&stats.Users)
	r.db.Model(&models.Media{}).Where("tenant_id = ?", tenantID).Count(&stats.Media)
	r.db.Model(&models.PageView{}).Where("tenant_id = ? AND DATE(created_at) = DATE(?)", tenantID, time.Now()).Count(&stats.ViewsToday)
	r.db.Model(&models.PageView{}).Where("tenant_id = ? AND created_at >= ?", tenantID, time.Now().AddDate(0, 0, -7)).Count(&stats.ViewsThisWeek)
	r.db.Model(&models.PageView{}).Where("tenant_id = ? AND created_at >= ?", tenantID, time.Now().AddDate(0, -1, 0)).Count(&stats.ViewsThisMonth)
	r.db.Model(&models.PageView{}).Where("tenant_id = ?", tenantID).Count(&stats.TotalViews)
	return stats, nil
}

func (r *gormAnalyticsRepository) RecentArticles(limit int, tenantID uint) ([]models.Article, error) {
	var articles []models.Article
	if err := r.db.Where("tenant_id = ?", tenantID).Preload("Author").Order("created_at DESC").Limit(limit).Find(&articles).Error; err != nil {
		return nil, err
	}
	return articles, nil
}

func (r *gormAnalyticsRepository) RecentComments(limit int, tenantID uint) ([]models.Comment, error) {
	var comments []models.Comment
	if err := r.db.Where("tenant_id = ?", tenantID).Preload("User").Preload("Article").Order("created_at DESC").Limit(limit).Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *gormAnalyticsRepository) PopularArticles(limit int, tenantID uint) ([]models.Article, error) {
	var articles []models.Article
	if err := r.db.Where("status = ? AND tenant_id = ?", models.StatusPublished, tenantID).
		Order("view_count DESC").Limit(limit).Find(&articles).Error; err != nil {
		return nil, err
	}
	return articles, nil
}

func (r *gormAnalyticsRepository) ViewsOverTime(days int, tenantID uint) ([]DayStatsData, error) {
	var results []DayStatsData
	if err := r.db.Model(&models.PageView{}).
		Select("DATE(created_at) as date, COUNT(*) as views").
		Where("tenant_id = ? AND created_at >= ?", tenantID, time.Now().AddDate(0, 0, -days)).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (r *gormAnalyticsRepository) TopReferrers(limit int, tenantID uint) ([]ReferrerData, error) {
	var results []ReferrerData
	if err := r.db.Model(&models.PageView{}).
		Select("referrer, COUNT(*) as count").
		Where("tenant_id = ? AND referrer != ''", tenantID).
		Group("referrer").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (r *gormAnalyticsRepository) DeviceBreakdown(tenantID uint) (DeviceBreakdownData, error) {
	var data DeviceBreakdownData
	// Mirrors original service behaviour: scan errors are ignored.
	r.db.Model(&models.PageView{}).Where("tenant_id = ?", tenantID).Select("device as name, COUNT(*) as count").
		Group("device").Order("count DESC").Scan(&data.Devices)
	r.db.Model(&models.PageView{}).Where("tenant_id = ?", tenantID).Select("browser as name, COUNT(*) as count").
		Group("browser").Order("count DESC").Limit(10).Scan(&data.Browsers)
	r.db.Model(&models.PageView{}).Where("tenant_id = ?", tenantID).Select("os as name, COUNT(*) as count").
		Group("os").Order("count DESC").Limit(10).Scan(&data.OS)
	return data, nil
}

func (r *gormAnalyticsRepository) CreatePageView(view *models.PageView) error {
	return r.db.Create(view).Error
}
