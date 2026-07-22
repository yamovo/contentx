package repository

import (
	"time"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// ============================================================
// SettingsRepository
// ============================================================

// SettingsRepository defines data-access operations for site settings.
type SettingsRepository interface {
	List(group string) ([]models.SiteSetting, error)
	Get(key string) (*models.SiteSetting, error)
	UpdateValue(key, value string) (rowsAffected int64, err error)
	Create(setting *models.SiteSetting) error
	ListPublic() ([]models.SiteSetting, error)
}

// gormSettingsRepository implements SettingsRepository with GORM.
type gormSettingsRepository struct {
	db *gorm.DB
}

// NewSettingsRepository builds a GORM-backed SettingsRepository.
func NewSettingsRepository(db *gorm.DB) SettingsRepository {
	return &gormSettingsRepository{db: db}
}

func (r *gormSettingsRepository) List(group string) ([]models.SiteSetting, error) {
	query := r.db.Model(&models.SiteSetting{})
	if group != "" {
		query = query.Where("`group` = ?", group)
	}
	var settings []models.SiteSetting
	if err := query.Order("sort_order ASC").Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

func (r *gormSettingsRepository) Get(key string) (*models.SiteSetting, error) {
	var setting models.SiteSetting
	if err := r.db.Where("key = ?", key).First(&setting).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}

func (r *gormSettingsRepository) UpdateValue(key, value string) (int64, error) {
	result := r.db.Model(&models.SiteSetting{}).
		Where("key = ?", key).
		Update("value", value)
	return result.RowsAffected, result.Error
}

func (r *gormSettingsRepository) Create(setting *models.SiteSetting) error {
	return r.db.Create(setting).Error
}

func (r *gormSettingsRepository) ListPublic() ([]models.SiteSetting, error) {
	var settings []models.SiteSetting
	if err := r.db.Where("is_public = ?", true).Order("sort_order ASC").Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

// ============================================================
// SEORepository
// ============================================================

// SEORepository defines data-access operations for SEO settings, redirect rules,
// and the sitemap article query.
type SEORepository interface {
	GetSetting(entityType string, entityID uint) (*models.SEOSetting, error)
	CreateSetting(setting *models.SEOSetting) error
	SaveSetting(setting *models.SEOSetting) error
	ListPublishedArticlesForSitemap() ([]models.Article, error)
	ListRedirects() ([]models.RedirectRule, error)
	CreateRedirect(rule *models.RedirectRule) error
	DeleteRedirect(id uint) error
}

// gormSEORepository implements SEORepository with GORM.
type gormSEORepository struct {
	db *gorm.DB
}

// NewSEORepository builds a GORM-backed SEORepository.
func NewSEORepository(db *gorm.DB) SEORepository {
	return &gormSEORepository{db: db}
}

func (r *gormSEORepository) GetSetting(entityType string, entityID uint) (*models.SEOSetting, error) {
	var setting models.SEOSetting
	if err := r.db.Where("entity_type = ? AND entity_id = ?", entityType, entityID).
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

func (r *gormSEORepository) ListPublishedArticlesForSitemap() ([]models.Article, error) {
	var articles []models.Article
	if err := r.db.Where("status = ? AND post_type = ?", models.StatusPublished, models.PostTypePost).
		Order("updated_at DESC").Find(&articles).Error; err != nil {
		return nil, err
	}
	return articles, nil
}

func (r *gormSEORepository) ListRedirects() ([]models.RedirectRule, error) {
	var rules []models.RedirectRule
	if err := r.db.Order("from_path ASC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *gormSEORepository) CreateRedirect(rule *models.RedirectRule) error {
	return r.db.Create(rule).Error
}

func (r *gormSEORepository) DeleteRedirect(id uint) error {
	return r.db.Delete(&models.RedirectRule{}, id).Error
}

// ============================================================
// MenuRepository
// ============================================================

// MenuRepository defines data-access operations for menus and menu items.
type MenuRepository interface {
	ListMenus() ([]models.Menu, error)
	GetMenuByID(id uint) (*models.Menu, error)
	FindMenu(id uint) (*models.Menu, error)
	CreateMenu(menu *models.Menu) error
	UpdateMenuFields(id uint, fields map[string]interface{}) error
	DeleteMenu(id uint) error
	FindItem(id uint) (*models.MenuItem, error)
	CreateItem(item *models.MenuItem) error
	UpdateItemFields(id uint, fields map[string]interface{}) error
	DeleteItem(id uint) error
	MaxItemSortOrder(menuID uint) (int, error)
}

// gormMenuRepository implements MenuRepository with GORM.
type gormMenuRepository struct {
	db *gorm.DB
}

// NewMenuRepository builds a GORM-backed MenuRepository.
func NewMenuRepository(db *gorm.DB) MenuRepository {
	return &gormMenuRepository{db: db}
}

func (r *gormMenuRepository) ListMenus() ([]models.Menu, error) {
	var menus []models.Menu
	if err := r.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Find(&menus).Error; err != nil {
		return nil, err
	}
	return menus, nil
}

func (r *gormMenuRepository) GetMenuByID(id uint) (*models.Menu, error) {
	var menu models.Menu
	if err := r.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).First(&menu, id).Error; err != nil {
		return nil, err
	}
	return &menu, nil
}

func (r *gormMenuRepository) FindMenu(id uint) (*models.Menu, error) {
	var menu models.Menu
	if err := r.db.First(&menu, id).Error; err != nil {
		return nil, err
	}
	return &menu, nil
}

func (r *gormMenuRepository) CreateMenu(menu *models.Menu) error {
	return r.db.Create(menu).Error
}

func (r *gormMenuRepository) UpdateMenuFields(id uint, fields map[string]interface{}) error {
	return r.db.Model(&models.Menu{}).Where("id = ?", id).Updates(fields).Error
}

func (r *gormMenuRepository) DeleteMenu(id uint) error {
	// Best-effort: delete items first, then the menu (mirrors prior service behaviour).
	if err := r.db.Where("menu_id = ?", id).Delete(&models.MenuItem{}).Error; err != nil {
		return err
	}
	return r.db.Delete(&models.Menu{}, id).Error
}

func (r *gormMenuRepository) FindItem(id uint) (*models.MenuItem, error) {
	var item models.MenuItem
	if err := r.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormMenuRepository) CreateItem(item *models.MenuItem) error {
	return r.db.Create(item).Error
}

func (r *gormMenuRepository) UpdateItemFields(id uint, fields map[string]interface{}) error {
	return r.db.Model(&models.MenuItem{}).Where("id = ?", id).Updates(fields).Error
}

func (r *gormMenuRepository) DeleteItem(id uint) error {
	return r.db.Delete(&models.MenuItem{}, id).Error
}

func (r *gormMenuRepository) MaxItemSortOrder(menuID uint) (int, error) {
	var maxSort int
	if err := r.db.Model(&models.MenuItem{}).Where("menu_id = ?", menuID).
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
	DashboardStats() (DashboardStatsData, error)
	RecentArticles(limit int) ([]models.Article, error)
	RecentComments(limit int) ([]models.Comment, error)
	PopularArticles(limit int) ([]models.Article, error)
	ViewsOverTime(days int) ([]DayStatsData, error)
	TopReferrers(limit int) ([]ReferrerData, error)
	DeviceBreakdown() (DeviceBreakdownData, error)
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

func (r *gormAnalyticsRepository) DashboardStats() (DashboardStatsData, error) {
	var stats DashboardStatsData
	// Mirrors original service behaviour: individual count errors are ignored.
	r.db.Model(&models.Article{}).Count(&stats.Articles)
	r.db.Model(&models.Article{}).Where("status = ?", models.StatusPublished).Count(&stats.Published)
	r.db.Model(&models.Comment{}).Count(&stats.Comments)
	r.db.Model(&models.Comment{}).Where("status = ?", "pending").Count(&stats.PendingComments)
	r.db.Model(&models.User{}).Count(&stats.Users)
	r.db.Model(&models.Media{}).Count(&stats.Media)
	r.db.Model(&models.PageView{}).Where("DATE(created_at) = DATE(?)", time.Now()).Count(&stats.ViewsToday)
	r.db.Model(&models.PageView{}).Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).Count(&stats.ViewsThisWeek)
	r.db.Model(&models.PageView{}).Where("created_at >= ?", time.Now().AddDate(0, -1, 0)).Count(&stats.ViewsThisMonth)
	r.db.Model(&models.PageView{}).Count(&stats.TotalViews)
	return stats, nil
}

func (r *gormAnalyticsRepository) RecentArticles(limit int) ([]models.Article, error) {
	var articles []models.Article
	if err := r.db.Preload("Author").Order("created_at DESC").Limit(limit).Find(&articles).Error; err != nil {
		return nil, err
	}
	return articles, nil
}

func (r *gormAnalyticsRepository) RecentComments(limit int) ([]models.Comment, error) {
	var comments []models.Comment
	if err := r.db.Preload("User").Preload("Article").Order("created_at DESC").Limit(limit).Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *gormAnalyticsRepository) PopularArticles(limit int) ([]models.Article, error) {
	var articles []models.Article
	if err := r.db.Where("status = ?", models.StatusPublished).
		Order("view_count DESC").Limit(limit).Find(&articles).Error; err != nil {
		return nil, err
	}
	return articles, nil
}

func (r *gormAnalyticsRepository) ViewsOverTime(days int) ([]DayStatsData, error) {
	var results []DayStatsData
	if err := r.db.Model(&models.PageView{}).
		Select("DATE(created_at) as date, COUNT(*) as views").
		Where("created_at >= ?", time.Now().AddDate(0, 0, -days)).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (r *gormAnalyticsRepository) TopReferrers(limit int) ([]ReferrerData, error) {
	var results []ReferrerData
	if err := r.db.Model(&models.PageView{}).
		Select("referrer, COUNT(*) as count").
		Where("referrer != ''").
		Group("referrer").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (r *gormAnalyticsRepository) DeviceBreakdown() (DeviceBreakdownData, error) {
	var data DeviceBreakdownData
	// Mirrors original service behaviour: scan errors are ignored.
	r.db.Model(&models.PageView{}).Select("device as name, COUNT(*) as count").
		Group("device").Order("count DESC").Scan(&data.Devices)
	r.db.Model(&models.PageView{}).Select("browser as name, COUNT(*) as count").
		Group("browser").Order("count DESC").Limit(10).Scan(&data.Browsers)
	r.db.Model(&models.PageView{}).Select("os as name, COUNT(*) as count").
		Group("os").Order("count DESC").Limit(10).Scan(&data.OS)
	return data, nil
}

func (r *gormAnalyticsRepository) CreatePageView(view *models.PageView) error {
	return r.db.Create(view).Error
}
