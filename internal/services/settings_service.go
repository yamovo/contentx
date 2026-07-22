package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
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
	Stats           DashboardStats   `json:"stats"`
	RecentArticles  []models.Article `json:"recent_articles"`
	RecentComments  []models.Comment `json:"recent_comments"`
	PopularArticles []models.Article `json:"popular_articles"`
}

// DashboardStats holds aggregate counts for the dashboard.
type DashboardStats struct {
	Articles        int64 `json:"total_articles"`
	Published       int64 `json:"published_articles"`
	Comments        int64 `json:"total_comments"`
	PendingComments int64 `json:"pending_comments"`
	Users           int64 `json:"total_users"`
	Media           int64 `json:"total_media"`
	ViewsToday      int64 `json:"views_today"`
	ViewsThisWeek   int64 `json:"views_this_week"`
	ViewsThisMonth  int64 `json:"views_this_month"`
	TotalViews      int64 `json:"total_views"`
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
	repo repository.SettingsRepository
}

// NewSettingsService creates a new SettingsService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewSettingsService(db *gorm.DB) *SettingsService {
	return &SettingsService{repo: repository.NewSettingsRepository(db)}
}

// NewSettingsServiceWithRepo builds a SettingsService with an explicit repository,
// enabling unit tests to inject mocks.
func NewSettingsServiceWithRepo(repo repository.SettingsRepository) *SettingsService {
	return &SettingsService{repo: repo}
}

// List returns all settings, optionally filtered by group, plus a grouped map.
func (s *SettingsService) List(group string) ([]models.SiteSetting, map[string][]models.SiteSetting, error) {
	settings, err := s.repo.List(group)
	if err != nil {
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
	return s.repo.Get(key)
}

// Update upserts multiple settings at once.
func (s *SettingsService) Update(settings map[string]interface{}) error {
	for key, value := range settings {
		strValue := stringifyValue(value)
		rowsAffected, err := s.repo.UpdateValue(key, strValue)
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			if err := s.repo.Create(&models.SiteSetting{
				Key:   key,
				Value: strValue,
				Type:  detectType(value),
				Group: "custom",
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// PublicSettings returns public settings as a flat key-value map.
func (s *SettingsService) PublicSettings() (map[string]string, error) {
	settings, err := s.repo.ListPublic()
	if err != nil {
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
	repo    repository.SEORepository
	baseURL string
}

// NewSEOService creates a new SEOService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewSEOService(db *gorm.DB, baseURL string) *SEOService {
	return &SEOService{repo: repository.NewSEORepository(db), baseURL: baseURL}
}

// NewSEOServiceWithRepo builds an SEOService with an explicit repository,
// enabling unit tests to inject mocks.
func NewSEOServiceWithRepo(repo repository.SEORepository, baseURL string) *SEOService {
	return &SEOService{repo: repo, baseURL: baseURL}
}

// GetSetting returns the SEO setting for a specific entity.
func (s *SEOService) GetSetting(entityType string, entityID uint) (*models.SEOSetting, error) {
	return s.repo.GetSetting(entityType, entityID)
}

// UpdateSetting upserts SEO settings for a specific entity.
func (s *SEOService) UpdateSetting(entityType string, entityID uint, req SEOSettingRequest) error {
	setting, err := s.repo.GetSetting(entityType, entityID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			setting = &models.SEOSetting{
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
			return s.repo.CreateSetting(setting)
		}
		return err
	}

	setting.Title = req.Title
	setting.Desc = req.Desc
	setting.Keywords = req.Keywords
	setting.Canonical = req.Canonical
	setting.OGImage = req.OGImage
	setting.OGType = req.OGType
	setting.Robots = req.Robots
	setting.Extra = req.Extra
	return s.repo.SaveSetting(setting)
}

// Sitemap generates a sitemap XML string using the configured base URL.
func (s *SEOService) Sitemap() (string, error) {
	articles, err := s.repo.ListPublishedArticlesForSitemap()
	if err != nil {
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
	return s.repo.ListRedirects()
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
	if err := s.repo.CreateRedirect(&rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// DeleteRedirect deletes a redirect rule by ID.
func (s *SEOService) DeleteRedirect(id uint) error {
	return s.repo.DeleteRedirect(id)
}

// ============================================================
// MenuService
// ============================================================

// MenuService provides navigation menu operations.
type MenuService struct {
	repo repository.MenuRepository
}

// NewMenuService creates a new MenuService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewMenuService(db *gorm.DB) *MenuService {
	return &MenuService{repo: repository.NewMenuRepository(db)}
}

// NewMenuServiceWithRepo builds a MenuService with an explicit repository,
// enabling unit tests to inject mocks.
func NewMenuServiceWithRepo(repo repository.MenuRepository) *MenuService {
	return &MenuService{repo: repo}
}

// List returns all menus with their items sorted by sort_order.
func (s *MenuService) List() ([]models.Menu, error) {
	return s.repo.ListMenus()
}

// Get returns a single menu by ID with its items sorted by sort_order.
func (s *MenuService) Get(id uint) (*models.Menu, error) {
	return s.repo.GetMenuByID(id)
}

// Create creates a new menu.
func (s *MenuService) Create(req CreateMenuRequest) (*models.Menu, error) {
	menu := models.Menu{
		Name:      req.Name,
		Slug:      req.Slug,
		Locations: req.Locations,
	}
	if err := s.repo.CreateMenu(&menu); err != nil {
		return nil, err
	}
	return &menu, nil
}

// Update updates a menu's name and locations.
func (s *MenuService) Update(id uint, req UpdateMenuRequest) error {
	if _, err := s.repo.FindMenu(id); err != nil {
		return err
	}
	return s.repo.UpdateMenuFields(id, map[string]interface{}{
		"name":      req.Name,
		"locations": req.Locations,
	})
}

// Delete deletes a menu and its associated items.
func (s *MenuService) Delete(id uint) error {
	if _, err := s.repo.FindMenu(id); err != nil {
		return err
	}
	return s.repo.DeleteMenu(id)
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

	maxSort, err := s.repo.MaxItemSortOrder(menuID)
	if err != nil {
		return nil, err
	}
	item.SortOrder = maxSort + 1

	if err := s.repo.CreateItem(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateItem updates a menu item, only modifying fields that are non-nil.
func (s *MenuService) UpdateItem(itemID uint, req UpdateMenuItemRequest) error {
	if _, err := s.repo.FindItem(itemID); err != nil {
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

	return s.repo.UpdateItemFields(itemID, updates)
}

// DeleteItem removes a menu item by ID.
func (s *MenuService) DeleteItem(itemID uint) error {
	return s.repo.DeleteItem(itemID)
}

// ReorderItems updates sort_order (and optionally parent_id) for a batch of items.
func (s *MenuService) ReorderItems(items []ReorderItem) error {
	for _, item := range items {
		updates := map[string]interface{}{"sort_order": item.SortOrder}
		if item.ParentID != nil {
			updates["parent_id"] = item.ParentID
		}
		if err := s.repo.UpdateItemFields(item.ID, updates); err != nil {
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
	repo repository.AnalyticsRepository
}

// NewAnalyticsService creates a new AnalyticsService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewAnalyticsService(db *gorm.DB) *AnalyticsService {
	return &AnalyticsService{repo: repository.NewAnalyticsRepository(db)}
}

// NewAnalyticsServiceWithRepo builds an AnalyticsService with an explicit repository,
// enabling unit tests to inject mocks.
func NewAnalyticsServiceWithRepo(repo repository.AnalyticsRepository) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

// Dashboard returns aggregate stats, recent articles/comments, and popular articles.
func (s *AnalyticsService) Dashboard() (DashboardData, error) {
	var data DashboardData

	stats, _ := s.repo.DashboardStats()
	data.Stats = DashboardStats{
		Articles:        stats.Articles,
		Published:       stats.Published,
		Comments:        stats.Comments,
		PendingComments: stats.PendingComments,
		Users:           stats.Users,
		Media:           stats.Media,
		ViewsToday:      stats.ViewsToday,
		ViewsThisWeek:   stats.ViewsThisWeek,
		ViewsThisMonth:  stats.ViewsThisMonth,
		TotalViews:      stats.TotalViews,
	}

	recentArticles, _ := s.repo.RecentArticles(5)
	data.RecentArticles = recentArticles
	recentComments, _ := s.repo.RecentComments(5)
	data.RecentComments = recentComments
	popularArticles, _ := s.repo.PopularArticles(5)
	data.PopularArticles = popularArticles

	return data, nil
}

// ViewsOverTime returns per-day view counts for the last N days, with gaps filled.
func (s *AnalyticsService) ViewsOverTime(days int) ([]DayStats, error) {
	if days < 1 {
		days = 30
	}

	results, err := s.repo.ViewsOverTime(days)
	if err != nil {
		return nil, err
	}

	dayStats := make([]DayStats, len(results))
	for i, r := range results {
		dayStats[i] = DayStats{Date: r.Date, Views: r.Views}
	}

	return fillDateGaps(dayStats, days), nil
}

// TopReferrers returns the top 10 referrers by hit count.
func (s *AnalyticsService) TopReferrers() ([]Referrer, error) {
	results, err := s.repo.TopReferrers(10)
	if err != nil {
		return nil, err
	}
	referrers := make([]Referrer, len(results))
	for i, r := range results {
		referrers[i] = Referrer{Referrer: r.Referrer, Count: r.Count}
	}
	return referrers, nil
}

// DeviceBreakdown returns device, browser, and OS breakdowns.
func (s *AnalyticsService) DeviceBreakdown() (DeviceBreakdownData, error) {
	data, err := s.repo.DeviceBreakdown()
	if err != nil {
		return DeviceBreakdownData{}, err
	}

	mapBreakdown := func(items []repository.BreakdownData) []Breakdown {
		out := make([]Breakdown, len(items))
		for i, b := range items {
			out[i] = Breakdown{Name: b.Name, Count: b.Count}
		}
		return out
	}

	return DeviceBreakdownData{
		Devices:  mapBreakdown(data.Devices),
		Browsers: mapBreakdown(data.Browsers),
		OS:       mapBreakdown(data.OS),
	}, nil
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
	return s.repo.CreatePageView(&view)
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
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		return "iOS"
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "mac os"):
		return "macOS"
	case strings.Contains(ua, "linux"):
		return "Linux"
	default:
		return "Other"
	}
}
