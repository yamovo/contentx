package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// ============================================================
// Request / Response types
// ============================================================

// DayStats represents statistics for a single day.
type DayStats struct {
	Date  string `json:"date"`
	Views int64  `json:"views"`
}

// SEOSettingRequest is the payload for UpdateSetting.
type SEOSettingRequest struct {
	Title     string            `json:"title"`
	Desc      string            `json:"desc"`
	Keywords  string            `json:"keywords"`
	Canonical string            `json:"canonical"`
	OGImage   string            `json:"og_image"`
	OGType    string            `json:"og_type"`
	Robots    string            `json:"robots"`
	Extra     map[string]string `json:"extra"`
}

// CreateRedirectRequest is the payload for CreateRedirect.
type CreateRedirectRequest struct {
	FromPath   string `json:"from_path" binding:"required"`
	ToPath     string `json:"to_path" binding:"required"`
	StatusCode int    `json:"status_code"`
	Note       string `json:"note"`
}

// CreateMenuRequest is the payload for MenuService.Create.
type CreateMenuRequest struct {
	Name      string `json:"name" binding:"required"`
	Slug      string `json:"slug" binding:"required"`
	Locations string `json:"locations"`
}

// UpdateMenuRequest is the payload for MenuService.Update.
type UpdateMenuRequest struct {
	Name      string `json:"name"`
	Locations string `json:"locations"`
}

// AddMenuItemRequest is the payload for MenuService.AddItem.
type AddMenuItemRequest struct {
	Title      string `json:"title" binding:"required"`
	URL        string `json:"url"`
	Target     string `json:"target"`
	CSSClass   string `json:"css_class"`
	Icon       string `json:"icon"`
	ParentID   *uint  `json:"parent_id"`
	ArticleID  *uint  `json:"article_id"`
	CategoryID *uint  `json:"category_id"`
}

// UpdateMenuItemRequest is the payload for MenuService.UpdateItem.
type UpdateMenuItemRequest struct {
	Title     *string `json:"title"`
	URL       *string `json:"url"`
	Target    *string `json:"target"`
	CSSClass  *string `json:"css_class"`
	Icon      *string `json:"icon"`
	SortOrder *int    `json:"sort_order"`
	IsActive  *bool   `json:"is_active"`
	ParentID  *uint   `json:"parent_id"`
}

// RecordViewRequest is the payload for AnalyticsService.RecordView.
type RecordViewRequest struct {
	ArticleID *uint  `json:"article_id"`
	Path      string `json:"path" binding:"required"`
	Duration  int    `json:"duration"`
}

// DashboardData is the response for AnalyticsService.Dashboard.
type DashboardData struct {
	Stats           DashboardStats     `json:"stats"`
	RecentArticles  []models.Article   `json:"recent_articles"`
	RecentComments  []models.Comment   `json:"recent_comments"`
	PopularArticles []models.Article   `json:"popular_articles"`
}

// DashboardStats holds aggregate counts for the dashboard.
type DashboardStats struct {
	Articles       int64 `json:"total_articles"`
	Published      int64 `json:"published_articles"`
	Comments       int64 `json:"total_comments"`
	PendingComments int64 `json:"pending_comments"`
	Users          int64 `json:"total_users"`
	Media          int64 `json:"total_media"`
	ViewsToday     int64 `json:"views_today"`
	ViewsThisWeek  int64 `json:"views_this_week"`
	ViewsThisMonth int64 `json:"views_this_month"`
	TotalViews     int64 `json:"total_views"`
}

// Referrer holds a referrer URL and its hit count.
type Referrer struct {
	Referrer string `json:"referrer"`
	Count    int64  `json:"count"`
}

// Breakdown holds a named count (used internally by DeviceBreakdownData).
type Breakdown struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// DeviceBreakdownData groups device, browser, and OS breakdowns.
type DeviceBreakdownData struct {
	Devices  []Breakdown `json:"devices"`
	Browsers []Breakdown `json:"browsers"`
	OS       []Breakdown `json:"os"`
}

// ============================================================
// SettingsService
// ============================================================

// SettingsService provides site settings operations.
type SettingsService struct {
	db *gorm.DB
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(db *gorm.DB) *SettingsService {
	return &SettingsService{db: db}
}

// List returns all settings, optionally filtered by group, plus a grouped map.
func (s *SettingsService) List(group string) ([]models.SiteSetting, map[string][]models.SiteSetting, error) {
	query := s.db.Model(&models.SiteSetting{})
	if group != "" {
		query = query.Where("group = ?", group)
	}
	query.Order("sort_order ASC")

	var settings []models.SiteSetting
	if err := query.Find(&settings).Error; err != nil {
		return nil, nil, err
	}

	grouped := make(map[string][]models.SiteSetting)
	for _, setting := range settings {
		grouped[setting.Group] = append(grouped[setting.Group], setting)
	}

	return settings, grouped, nil
}

// Get returns a single setting by key.
func (s *SettingsService) Get(key string) (*models.SiteSetting, error) {
	var setting models.SiteSetting
	if err := s.db.Where("key = ?", key).First(&setting).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}

// Update upserts multiple settings at once.
func (s *SettingsService) Update(settings map[string]interface{}) error {
	for key, value := range settings {
		strValue := stringifyValue(value)
		result := s.db.Model(&models.SiteSetting{}).
			Where("key = ?", key).
			Update("value", strValue)
		if result.RowsAffected == 0 {
			s.db.Create(&models.SiteSetting{
				Key:   key,
				Value: strValue,
				Type:  detectType(value),
				Group: "custom",
			})
		}
	}
	return nil
}

// PublicSettings returns public settings as a flat key-value map.
func (s *SettingsService) PublicSettings() (map[string]string, error) {
	var settings []models.SiteSetting
	if err := s.db.Where("is_public = ?", true).Order("sort_order ASC").Find(&settings).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

// ============================================================
// SEOService
// ============================================================

// SEOService provides SEO settings and tools.
type SEOService struct {
	db      *gorm.DB
	baseURL string
}

// NewSEOService creates a new SEOService.
func NewSEOService(db *gorm.DB, baseURL string) *SEOService {
	return &SEOService{db: db, baseURL: baseURL}
}

// GetSetting returns the SEO setting for a specific entity.
func (s *SEOService) GetSetting(entityType string, entityID uint) (*models.SEOSetting, error) {
	var setting models.SEOSetting
	if err := s.db.Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		First(&setting).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}

// UpdateSetting upserts SEO settings for a specific entity.
func (s *SEOService) UpdateSetting(entityType string, entityID uint, req SEOSettingRequest) error {
	var setting models.SEOSetting
	err := s.db.Where("entity_type = ? AND entity_id = ?", entityType, entityID).First(&setting).Error

	if err == gorm.ErrRecordNotFound {
		setting = models.SEOSetting{
			EntityType: entityType,
			EntityID:   entityID,
			Title:      req.Title,
			Desc:       req.Desc,
			Keywords:   req.Keywords,
			Canonical:  req.Canonical,
			OGImage:    req.OGImage,
			OGType:     req.OGType,
			Robots:     req.Robots,
			Extra:      req.Extra,
		}
		return s.db.Create(&setting).Error
	}
	if err != nil {
		return err
	}

	return s.db.Model(&setting).Updates(map[string]interface{}{
		"title":     req.Title,
		"desc":      req.Desc,
		"keywords":  req.Keywords,
		"canonical": req.Canonical,
		"og_image":  req.OGImage,
		"og_type":   req.OGType,
		"robots":    req.Robots,
		"extra":     req.Extra,
	}).Error
}

// Sitemap generates a sitemap XML string using the configured base URL.
func (s *SEOService) Sitemap() (string, error) {
	var articles []models.Article
	if err := s.db.Where("status = ? AND post_type = ?", models.StatusPublished, models.PostTypePost).
		Order("updated_at DESC").Find(&articles).Error; err != nil {
		return "", err
	}

	sb := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`

	for _, a := range articles {
		sb += "\n  <url>"
		sb += "\n    <loc>" + s.baseURL + "/articles/" + a.Slug + "</loc>"
		sb += "\n    <lastmod>" + a.UpdatedAt.Format("2006-01-02") + "</lastmod>"
		sb += "\n    <changefreq>weekly</changefreq>"
		sb += "\n    <priority>0.8</priority>"
		sb += "\n  </url>"
	}
	sb += "\n</urlset>"

	return sb, nil
}

// RobotsTxt generates a robots.txt string using the configured base URL.
func (s *SEOService) RobotsTxt() string {
	return `User-agent: *
Allow: /
Disallow: /api/
Disallow: /admin/
Disallow: /wp-admin/

Sitemap: ` + s.baseURL + `/sitemap.xml
`
}

// ListRedirects returns all redirect rules ordered by from_path.
func (s *SEOService) ListRedirects() ([]models.RedirectRule, error) {
	var rules []models.RedirectRule
	if err := s.db.Order("from_path ASC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// CreateRedirect creates a new redirect rule.
func (s *SEOService) CreateRedirect(req CreateRedirectRequest) (*models.RedirectRule, error) {
	if req.StatusCode == 0 {
		req.StatusCode = 301
	}
	rule := models.RedirectRule{
		FromPath:   req.FromPath,
		ToPath:     req.ToPath,
		StatusCode: req.StatusCode,
		Note:       req.Note,
		IsActive:   true,
	}
	if err := s.db.Create(&rule).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

// DeleteRedirect deletes a redirect rule by ID.
func (s *SEOService) DeleteRedirect(id uint) error {
	return s.db.Delete(&models.RedirectRule{}, id).Error
}

// ============================================================
// MenuService
// ============================================================

// MenuService provides navigation menu operations.
type MenuService struct {
	db *gorm.DB
}

// NewMenuService creates a new MenuService.
func NewMenuService(db *gorm.DB) *MenuService {
	return &MenuService{db: db}
}

// List returns all menus with their items sorted by sort_order.
func (s *MenuService) List() ([]models.Menu, error) {
	var menus []models.Menu
	if err := s.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Find(&menus).Error; err != nil {
		return nil, err
	}
	return menus, nil
}

// Get returns a single menu by ID with its items sorted by sort_order.
func (s *MenuService) Get(id uint) (*models.Menu, error) {
	var menu models.Menu
	if err := s.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).First(&menu, id).Error; err != nil {
		return nil, err
	}
	return &menu, nil
}

// Create creates a new menu.
func (s *MenuService) Create(req CreateMenuRequest) (*models.Menu, error) {
	menu := models.Menu{
		Name:      req.Name,
		Slug:      req.Slug,
		Locations: req.Locations,
	}
	if err := s.db.Create(&menu).Error; err != nil {
		return nil, err
	}
	return &menu, nil
}

// Update updates a menu's name and locations.
func (s *MenuService) Update(id uint, req UpdateMenuRequest) error {
	var menu models.Menu
	if err := s.db.First(&menu, id).Error; err != nil {
		return err
	}
	return s.db.Model(&menu).Updates(map[string]interface{}{
		"name":      req.Name,
		"locations": req.Locations,
	}).Error
}

// Delete deletes a menu and its associated items.
func (s *MenuService) Delete(id uint) error {
	var menu models.Menu
	if err := s.db.First(&menu, id).Error; err != nil {
		return err
	}
	if err := s.db.Where("menu_id = ?", menu.ID).Delete(&models.MenuItem{}).Error; err != nil {
		return err
	}
	return s.db.Delete(&menu).Error
}

// AddItem adds a new item to a menu, auto-setting sort_order and default target.
func (s *MenuService) AddItem(menuID uint, req AddMenuItemRequest) (*models.MenuItem, error) {
	item := models.MenuItem{
		MenuID:     menuID,
		Title:      req.Title,
		URL:        req.URL,
		Target:     req.Target,
		CSSClass:   req.CSSClass,
		Icon:       req.Icon,
		ParentID:   req.ParentID,
		ArticleID:  req.ArticleID,
		CategoryID: req.CategoryID,
	}
	if item.Target == "" {
		item.Target = "_self"
	}

	var maxSort int
	s.db.Model(&models.MenuItem{}).Where("menu_id = ?", menuID).
		Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSort)
	item.SortOrder = maxSort + 1

	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateItem updates a menu item, only modifying fields that are non-nil.
func (s *MenuService) UpdateItem(itemID uint, req UpdateMenuItemRequest) error {
	var item models.MenuItem
	if err := s.db.First(&item, itemID).Error; err != nil {
		return err
	}

	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.URL != nil {
		updates["url"] = *req.URL
	}
	if req.Target != nil {
		updates["target"] = *req.Target
	}
	if req.CSSClass != nil {
		updates["css_class"] = *req.CSSClass
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.ParentID != nil {
		updates["parent_id"] = *req.ParentID
	}

	return s.db.Model(&item).Updates(updates).Error
}

// DeleteItem removes a menu item by ID.
func (s *MenuService) DeleteItem(itemID uint) error {
	return s.db.Delete(&models.MenuItem{}, itemID).Error
}

// ReorderItems updates sort_order (and optionally parent_id) for a batch of items.
func (s *MenuService) ReorderItems(items []ReorderItem) error {
	for _, item := range items {
		updates := map[string]interface{}{"sort_order": item.SortOrder}
		if item.ParentID != nil {
			updates["parent_id"] = item.ParentID
		}
		if err := s.db.Model(&models.MenuItem{}).Where("id = ?", item.ID).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

// ============================================================
// AnalyticsService
// ============================================================

// AnalyticsService provides analytics and page-view operations.
type AnalyticsService struct {
	db *gorm.DB
}

// NewAnalyticsService creates a new AnalyticsService.
func NewAnalyticsService(db *gorm.DB) *AnalyticsService {
	return &AnalyticsService{db: db}
}

// Dashboard returns aggregate stats, recent articles/comments, and popular articles.
func (s *AnalyticsService) Dashboard() (DashboardData, error) {
	var data DashboardData

	s.db.Model(&models.Article{}).Count(&data.Stats.Articles)
	s.db.Model(&models.Article{}).Where("status = ?", models.StatusPublished).Count(&data.Stats.Published)
	s.db.Model(&models.Comment{}).Count(&data.Stats.Comments)
	s.db.Model(&models.Comment{}).Where("status = ?", "pending").Count(&data.Stats.PendingComments)
	s.db.Model(&models.User{}).Count(&data.Stats.Users)
	s.db.Model(&models.Media{}).Count(&data.Stats.Media)
	s.db.Model(&models.PageView{}).Where("DATE(created_at) = DATE(?)", time.Now()).Count(&data.Stats.ViewsToday)
	s.db.Model(&models.PageView{}).Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).Count(&data.Stats.ViewsThisWeek)
	s.db.Model(&models.PageView{}).Where("created_at >= ?", time.Now().AddDate(0, -1, 0)).Count(&data.Stats.ViewsThisMonth)
	s.db.Model(&models.PageView{}).Count(&data.Stats.TotalViews)

	s.db.Preload("Author").Order("created_at DESC").Limit(5).Find(&data.RecentArticles)
	s.db.Preload("User").Preload("Article").Order("created_at DESC").Limit(5).Find(&data.RecentComments)
	s.db.Where("status = ?", models.StatusPublished).
		Order("view_count DESC").Limit(5).Find(&data.PopularArticles)

	return data, nil
}

// ViewsOverTime returns per-day view counts for the last N days, with gaps filled.
func (s *AnalyticsService) ViewsOverTime(days int) ([]DayStats, error) {
	if days < 1 {
		days = 30
	}

	var results []DayStats
	if err := s.db.Model(&models.PageView{}).
		Select("DATE(created_at) as date, COUNT(*) as views").
		Where("created_at >= ?", time.Now().AddDate(0, 0, -days)).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&results).Error; err != nil {
		return nil, err
	}

	return fillDateGaps(results, days), nil
}

// TopReferrers returns the top 10 referrers by hit count.
func (s *AnalyticsService) TopReferrers() ([]Referrer, error) {
	var results []Referrer
	if err := s.db.Model(&models.PageView{}).
		Select("referrer, COUNT(*) as count").
		Where("referrer != ''").
		Group("referrer").
		Order("count DESC").
		Limit(10).
		Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// DeviceBreakdown returns device, browser, and OS breakdowns.
func (s *AnalyticsService) DeviceBreakdown() (DeviceBreakdownData, error) {
	var data DeviceBreakdownData

	s.db.Model(&models.PageView{}).Select("device as name, COUNT(*) as count").
		Group("device").Order("count DESC").Scan(&data.Devices)
	s.db.Model(&models.PageView{}).Select("browser as name, COUNT(*) as count").
		Group("browser").Order("count DESC").Limit(10).Scan(&data.Browsers)
	s.db.Model(&models.PageView{}).Select("os as name, COUNT(*) as count").
		Group("os").Order("count DESC").Limit(10).Scan(&data.OS)

	return data, nil
}

// RecordView inserts a new page view record.
func (s *AnalyticsService) RecordView(req RecordViewRequest, clientIP, userAgent, referer, sessionID string) error {
	view := models.PageView{
		ArticleID: req.ArticleID,
		Path:      req.Path,
		Referrer:  referer,
		UserAgent: userAgent,
		IP:        clientIP,
		SessionID: sessionID,
		Duration:  req.Duration,
		Device:    detectDevice(userAgent),
		Browser:   detectBrowser(userAgent),
		OS:        detectOS(userAgent),
	}
	return s.db.Create(&view).Error
}

// ============================================================
// Shared helpers
// ============================================================

func stringifyValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int(val)) {
			return strconv.Itoa(int(val))
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func detectType(v interface{}) string {
	switch v.(type) {
	case bool:
		return "bool"
	case float64:
		return "int"
	default:
		return "string"
	}
}

func fillDateGaps(data []DayStats, days int) []DayStats {
	if len(data) == 0 {
		return data
	}
	existing := make(map[string]int64)
	for _, d := range data {
		existing[d.Date] = d.Views
	}
	var filled []DayStats
	for i := days - 1; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		filled = append(filled, DayStats{
			Date:  date,
			Views: existing[date],
		})
	}
	return filled
}

func detectDevice(ua string) string {
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone") {
		return "mobile"
	}
	if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		return "tablet"
	}
	return "desktop"
}

func detectBrowser(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "edg/"):
		return "Edge"
	case strings.Contains(ua, "chrome"):
		return "Chrome"
	case strings.Contains(ua, "firefox"):
		return "Firefox"
	case strings.Contains(ua, "safari"):
		return "Safari"
	default:
		return "Other"
	}
}

func detectOS(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "windows"):
		return "Windows"
	case strings.Contains(ua, "mac os"):
		return "macOS"
	case strings.Contains(ua, "linux"):
		return "Linux"
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		return "iOS"
	default:
		return "Other"
	}
}
