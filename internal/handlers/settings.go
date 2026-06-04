package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// DayStats represents statistics for a single day.
type DayStats struct {
	Date  string `json:"date"`
	Views int64  `json:"views"`
}

// SettingsHandler handles site settings.
type SettingsHandler struct{ db *gorm.DB }

func NewSettingsHandler(db *gorm.DB) *SettingsHandler { return &SettingsHandler{db: db} }

// List returns all settings grouped by category.
// GET /api/v1/settings?group=general
func (h *SettingsHandler) List(c *gin.Context) {
	group := c.Query("group")
	query := h.db.Model(&models.SiteSetting{})
	if group != "" {
		query = query.Where("group = ?", group)
	}
	query.Order("sort_order ASC")

	var settings []models.SiteSetting
	query.Find(&settings)

	// Group by category.
	grouped := make(map[string][]models.SiteSetting)
	for _, s := range settings {
		grouped[s.Group] = append(grouped[s.Group], s)
	}

	c.JSON(http.StatusOK, gin.H{"data": settings, "grouped": grouped})
}

// Get returns a single setting by key.
// GET /api/v1/settings/:key
func (h *SettingsHandler) Get(c *gin.Context) {
	var setting models.SiteSetting
	if err := h.db.Where("key = ?", c.Param("key")).First(&setting).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Setting not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": setting})
}

// Update updates multiple settings at once.
// PUT /api/v1/settings
func (h *SettingsHandler) Update(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for key, value := range req {
		strValue := stringifyValue(value)
		result := h.db.Model(&models.SiteSetting{}).
			Where("key = ?", key).
			Update("value", strValue)
		if result.RowsAffected == 0 {
			// Create if not exists.
			h.db.Create(&models.SiteSetting{
				Key:   key,
				Value: strValue,
				Type:  detectType(value),
				Group: "custom",
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated"})
}

// PublicSettings returns settings safe for public consumption.
// GET /api/v1/settings/public
func (h *SettingsHandler) PublicSettings(c *gin.Context) {
	var settings []models.SiteSetting
	h.db.Where("is_public = ?", true).Order("sort_order ASC").Find(&settings)

	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ---------- SEO Handler ----------

// SEOHandler handles SEO settings and tools.
type SEOHandler struct{ db *gorm.DB }

func NewSEOHandler(db *gorm.DB) *SEOHandler { return &SEOHandler{db: db} }

// GetSEOSetting returns SEO settings for a specific entity.
// GET /api/v1/seo/:type/:id
func (h *SEOHandler) GetSEOSetting(c *gin.Context) {
	entityType := c.Param("type")
	entityID := c.Param("id")

	var setting models.SEOSetting
	if err := h.db.Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		First(&setting).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SEO setting not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": setting})
}

// UpdateSEOSetting updates SEO settings for a specific entity.
// PUT /api/v1/seo/:type/:id
func (h *SEOHandler) UpdateSEOSetting(c *gin.Context) {
	entityType := c.Param("type")
	entityID, _ := strconv.Atoi(c.Param("id"))

	var req struct {
		Title     string            `json:"title"`
		Desc      string            `json:"desc"`
		Keywords  string            `json:"keywords"`
		Canonical string            `json:"canonical"`
		OGImage   string            `json:"og_image"`
		OGType    string            `json:"og_type"`
		Robots    string            `json:"robots"`
		Extra     map[string]string `json:"extra"`
	}
	c.ShouldBindJSON(&req)

	var setting models.SEOSetting
	err := h.db.Where("entity_type = ? AND entity_id = ?", entityType, entityID).First(&setting).Error

	if err == gorm.ErrRecordNotFound {
		setting = models.SEOSetting{
			EntityType: entityType,
			EntityID:   uint(entityID),
			Title:      req.Title,
			Desc:       req.Desc,
			Keywords:   req.Keywords,
			Canonical:  req.Canonical,
			OGImage:    req.OGImage,
			OGType:     req.OGType,
			Robots:     req.Robots,
			Extra:      req.Extra,
		}
		h.db.Create(&setting)
	} else if err == nil {
		h.db.Model(&setting).Updates(map[string]interface{}{
			"title":     req.Title,
			"desc":      req.Desc,
			"keywords":  req.Keywords,
			"canonical": req.Canonical,
			"og_image":  req.OGImage,
			"og_type":   req.OGType,
			"robots":    req.Robots,
			"extra":     req.Extra,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": setting})
}

// Sitemap generates the sitemap entries.
// GET /api/v1/seo/sitemap
func (h *SEOHandler) Sitemap(c *gin.Context) {
	var articles []models.Article
	h.db.Where("status = ? AND post_type = ?", models.StatusPublished, models.PostTypePost).
		Order("updated_at DESC").Find(&articles)

	var sb string
	sb = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`

	for _, a := range articles {
		sb += "\n  <url>"
		sb += "\n    <loc>http://localhost:8080/articles/" + a.Slug + "</loc>"
		sb += "\n    <lastmod>" + a.UpdatedAt.Format("2006-01-02") + "</lastmod>"
		sb += "\n    <changefreq>weekly</changefreq>"
		sb += "\n    <priority>0.8</priority>"
		sb += "\n  </url>"
	}
	sb += "\n</urlset>"

	c.Data(http.StatusOK, "application/xml", []byte(sb))
}

// RobotsTxt generates robots.txt.
// GET /api/v1/seo/robots.txt
func (h *SEOHandler) RobotsTxt(c *gin.Context) {
	txt := `User-agent: *
Allow: /
Disallow: /api/
Disallow: /admin/
Disallow: /wp-admin/

Sitemap: http://localhost:8080/sitemap.xml
`
	c.Data(http.StatusOK, "text/plain", []byte(txt))
}

// Redirects handles URL redirects.
// GET /api/v1/seo/redirects
func (h *SEOHandler) ListRedirects(c *gin.Context) {
	var rules []models.RedirectRule
	h.db.Order("from_path ASC").Find(&rules)
	c.JSON(http.StatusOK, gin.H{"data": rules})
}

// CreateRedirect creates a redirect rule.
// POST /api/v1/seo/redirects
func (h *SEOHandler) CreateRedirect(c *gin.Context) {
	var req struct {
		FromPath   string `json:"from_path" binding:"required"`
		ToPath     string `json:"to_path" binding:"required"`
		StatusCode int    `json:"status_code"`
		Note       string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.StatusCode == 0 { req.StatusCode = 301 }

	rule := models.RedirectRule{
		FromPath: req.FromPath, ToPath: req.ToPath,
		StatusCode: req.StatusCode, Note: req.Note, IsActive: true,
	}
	h.db.Create(&rule)
	c.JSON(http.StatusCreated, gin.H{"data": rule})
}

// DeleteRedirect deletes a redirect rule.
// DELETE /api/v1/seo/redirects/:id
func (h *SEOHandler) DeleteRedirect(c *gin.Context) {
	h.db.Delete(&models.RedirectRule{}, c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"message": "Redirect deleted"})
}

// ---------- Menu Handler ----------

// MenuHandler handles navigation menus.
type MenuHandler struct{ db *gorm.DB }

func NewMenuHandler(db *gorm.DB) *MenuHandler { return &MenuHandler{db: db} }

// List returns all menus.
// GET /api/v1/menus
func (h *MenuHandler) List(c *gin.Context) {
	var menus []models.Menu
	h.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Find(&menus)
	c.JSON(http.StatusOK, gin.H{"data": menus})
}

// Get returns a single menu with items.
// GET /api/v1/menus/:id
func (h *MenuHandler) Get(c *gin.Context) {
	var menu models.Menu
	if err := h.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).First(&menu, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Menu not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": menu})
}

// Create creates a new menu.
// POST /api/v1/menus
func (h *MenuHandler) Create(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		Slug      string `json:"slug" binding:"required"`
		Locations string `json:"locations"`
	}
	c.ShouldBindJSON(&req)
	menu := models.Menu{Name: req.Name, Slug: req.Slug, Locations: req.Locations}
	h.db.Create(&menu)
	c.JSON(http.StatusCreated, gin.H{"data": menu})
}

// Update updates a menu.
// PUT /api/v1/menus/:id
func (h *MenuHandler) Update(c *gin.Context) {
	var menu models.Menu
	if err := h.db.First(&menu, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Menu not found"})
		return
	}
	var req struct {
		Name      string `json:"name"`
		Locations string `json:"locations"`
	}
	c.ShouldBindJSON(&req)
	h.db.Model(&menu).Updates(map[string]interface{}{
		"name": req.Name, "locations": req.Locations,
	})
	c.JSON(http.StatusOK, gin.H{"data": menu})
}

// Delete deletes a menu and its items.
// DELETE /api/v1/menus/:id
func (h *MenuHandler) Delete(c *gin.Context) {
	var menu models.Menu
	if err := h.db.First(&menu, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Menu not found"})
		return
	}
	h.db.Where("menu_id = ?", menu.ID).Delete(&models.MenuItem{})
	h.db.Delete(&menu)
	c.JSON(http.StatusOK, gin.H{"message": "Menu deleted"})
}

// AddItem adds an item to a menu.
// POST /api/v1/menus/:id/items
func (h *MenuHandler) AddItem(c *gin.Context) {
	menuID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Title      string `json:"title" binding:"required"`
		URL        string `json:"url"`
		Target     string `json:"target"`
		CSSClass   string `json:"css_class"`
		Icon       string `json:"icon"`
		ParentID   *uint  `json:"parent_id"`
		ArticleID  *uint  `json:"article_id"`
		CategoryID *uint  `json:"category_id"`
	}
	c.ShouldBindJSON(&req)

	item := models.MenuItem{
		MenuID: uint(menuID), Title: req.Title, URL: req.URL,
		Target: req.Target, CSSClass: req.CSSClass, Icon: req.Icon,
		ParentID: req.ParentID, ArticleID: req.ArticleID, CategoryID: req.CategoryID,
	}
	if item.Target == "" { item.Target = "_self" }

	// Set sort order.
	var maxSort int
	h.db.Model(&models.MenuItem{}).Where("menu_id = ?", menuID).
		Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSort)
	item.SortOrder = maxSort + 1

	h.db.Create(&item)
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

// UpdateItem updates a menu item.
// PUT /api/v1/menus/:id/items/:item_id
func (h *MenuHandler) UpdateItem(c *gin.Context) {
	var item models.MenuItem
	if err := h.db.First(&item, c.Param("item_id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Menu item not found"})
		return
	}
	var req struct {
		Title      *string `json:"title"`
		URL        *string `json:"url"`
		Target     *string `json:"target"`
		CSSClass   *string `json:"css_class"`
		Icon       *string `json:"icon"`
		SortOrder  *int    `json:"sort_order"`
		IsActive   *bool   `json:"is_active"`
		ParentID   *uint   `json:"parent_id"`
	}
	c.ShouldBindJSON(&req)

	updates := map[string]interface{}{}
	if req.Title != nil { updates["title"] = *req.Title }
	if req.URL != nil { updates["url"] = *req.URL }
	if req.Target != nil { updates["target"] = *req.Target }
	if req.CSSClass != nil { updates["css_class"] = *req.CSSClass }
	if req.Icon != nil { updates["icon"] = *req.Icon }
	if req.SortOrder != nil { updates["sort_order"] = *req.SortOrder }
	if req.IsActive != nil { updates["is_active"] = *req.IsActive }
	if req.ParentID != nil { updates["parent_id"] = *req.ParentID }

	h.db.Model(&item).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"data": item})
}

// DeleteItem removes a menu item.
// DELETE /api/v1/menus/:id/items/:item_id
func (h *MenuHandler) DeleteItem(c *gin.Context) {
	h.db.Delete(&models.MenuItem{}, c.Param("item_id"))
	c.JSON(http.StatusOK, gin.H{"message": "Menu item deleted"})
}

// ReorderItems updates sort order for menu items.
// PUT /api/v1/menus/:id/items/reorder
func (h *MenuHandler) ReorderItems(c *gin.Context) {
	var req struct {
		Items []struct {
			ID        uint `json:"id"`
			SortOrder int  `json:"sort_order"`
			ParentID  *uint `json:"parent_id"`
		} `json:"items" binding:"required"`
	}
	c.ShouldBindJSON(&req)
	for _, item := range req.Items {
		updates := map[string]interface{}{"sort_order": item.SortOrder}
		if item.ParentID != nil { updates["parent_id"] = item.ParentID }
		h.db.Model(&models.MenuItem{}).Where("id = ?", item.ID).Updates(updates)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Items reordered"})
}

// ---------- Analytics Handler ----------

// AnalyticsHandler handles analytics data.
type AnalyticsHandler struct{ db *gorm.DB }

func NewAnalyticsHandler(db *gorm.DB) *AnalyticsHandler { return &AnalyticsHandler{db: db} }

// Dashboard returns dashboard statistics.
// GET /api/v1/analytics/dashboard
func (h *AnalyticsHandler) Dashboard(c *gin.Context) {
	var stats struct {
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
	h.db.Model(&models.Article{}).Count(&stats.Articles)
	h.db.Model(&models.Article{}).Where("status = ?", models.StatusPublished).Count(&stats.Published)
	h.db.Model(&models.Comment{}).Count(&stats.Comments)
	h.db.Model(&models.Comment{}).Where("status = ?", "pending").Count(&stats.PendingComments)
	h.db.Model(&models.User{}).Count(&stats.Users)
	h.db.Model(&models.Media{}).Count(&stats.Media)
	h.db.Model(&models.PageView{}).Where("DATE(created_at) = DATE(?)", time.Now()).Count(&stats.ViewsToday)
	h.db.Model(&models.PageView{}).Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).Count(&stats.ViewsThisWeek)
	h.db.Model(&models.PageView{}).Where("created_at >= ?", time.Now().AddDate(0, -1, 0)).Count(&stats.ViewsThisMonth)
	h.db.Model(&models.PageView{}).Count(&stats.TotalViews)

	// Recent articles.
	var recentArticles []models.Article
	h.db.Preload("Author").Order("created_at DESC").Limit(5).Find(&recentArticles)

	// Recent comments.
	var recentComments []models.Comment
	h.db.Preload("User").Preload("Article").Order("created_at DESC").Limit(5).Find(&recentComments)

	// Popular articles.
	var popularArticles []models.Article
	h.db.Where("status = ?", models.StatusPublished).
		Order("view_count DESC").Limit(5).Find(&popularArticles)

	c.JSON(http.StatusOK, gin.H{
		"stats":            stats,
		"recent_articles":  recentArticles,
		"recent_comments":  recentComments,
		"popular_articles": popularArticles,
	})
}

// ViewsOverTime returns view data over time for charts.
// GET /api/v1/analytics/views?days=30
func (h *AnalyticsHandler) ViewsOverTime(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 { days = 30 }

	var results []DayStats

	h.db.Model(&models.PageView{}).
		Select("DATE(created_at) as date, COUNT(*) as views").
		Where("created_at >= ?", time.Now().AddDate(0, 0, -days)).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&results)

	// Fill gaps.
	filled := fillDateGaps(results, days)
	c.JSON(http.StatusOK, gin.H{"data": filled})
}

// TopReferrers returns top referrers.
// GET /api/v1/analytics/referrers
func (h *AnalyticsHandler) TopReferrers(c *gin.Context) {
	type Referrer struct {
		Referrer string `json:"referrer"`
		Count    int64  `json:"count"`
	}
	var results []Referrer
	h.db.Model(&models.PageView{}).
		Select("referrer, COUNT(*) as count").
		Where("referrer != ''").
		Group("referrer").
		Order("count DESC").
		Limit(10).
		Scan(&results)

	c.JSON(http.StatusOK, gin.H{"data": results})
}

// DeviceBreakdown returns device/browser/OS breakdown.
// GET /api/v1/analytics/devices
func (h *AnalyticsHandler) DeviceBreakdown(c *gin.Context) {
	type Breakdown struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}

	var devices, browsers, oses []Breakdown
	h.db.Model(&models.PageView{}).Select("device as name, COUNT(*) as count").
		Group("device").Order("count DESC").Scan(&devices)
	h.db.Model(&models.PageView{}).Select("browser as name, COUNT(*) as count").
		Group("browser").Order("count DESC").Limit(10).Scan(&browsers)
	h.db.Model(&models.PageView{}).Select("os as name, COUNT(*) as count").
		Group("os").Order("count DESC").Limit(10).Scan(&oses)

	c.JSON(http.StatusOK, gin.H{
		"devices":  devices,
		"browsers": browsers,
		"os":       oses,
	})
}

// RecordView records a page view.
// POST /api/v1/analytics/record
func (h *AnalyticsHandler) RecordView(c *gin.Context) {
	var req struct {
		ArticleID *uint  `json:"article_id"`
		Path      string `json:"path" binding:"required"`
		Duration  int    `json:"duration"`
	}
	c.ShouldBindJSON(&req)

	ua := c.Request.UserAgent()
	view := models.PageView{
		ArticleID: req.ArticleID,
		Path:      req.Path,
		Referrer:  c.GetHeader("Referer"),
		UserAgent: ua,
		IP:        c.ClientIP(),
		SessionID: c.GetHeader("X-Session-ID"),
		Duration:  req.Duration,
		Device:    detectDevice(ua),
		Browser:   detectBrowser(ua),
		OS:        detectOS(ua),
	}

	// Non-blocking insert.
	go func() { h.db.Create(&view) }()

	c.JSON(http.StatusCreated, gin.H{"message": "View recorded"})
}

// Helpers

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
		if val { return "true" }
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
	// Build a map of existing dates.
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

// Need time import for Dashboard.
var _ = time.Now()
