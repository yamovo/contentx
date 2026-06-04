package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/services"
)

// SettingsHandler handles site settings.
type SettingsHandler struct {
	svc *services.SettingsService
}

func NewSettingsHandler(svc *services.SettingsService) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

// List returns all settings grouped by category.
// GET /api/v1/settings?group=general
func (h *SettingsHandler) List(c *gin.Context) {
	group := c.Query("group")

	settings, grouped, err := h.svc.List(group)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": settings, "grouped": grouped})
}

// Get returns a single setting by key.
// GET /api/v1/settings/:key
func (h *SettingsHandler) Get(c *gin.Context) {
	setting, err := h.svc.Get(c.Param("key"))
	if err != nil {
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

	if err := h.svc.Update(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated"})
}

// PublicSettings returns settings safe for public consumption.
// GET /api/v1/settings/public
func (h *SettingsHandler) PublicSettings(c *gin.Context) {
	settings, err := h.svc.PublicSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": settings})
}

// ---------- SEO Handler ----------

// SEOHandler handles SEO settings and tools.
type SEOHandler struct {
	svc *services.SEOService
}

func NewSEOHandler(svc *services.SEOService) *SEOHandler {
	return &SEOHandler{svc: svc}
}

// GetSEOSetting returns SEO settings for a specific entity.
// GET /api/v1/seo/:type/:id
func (h *SEOHandler) GetSEOSetting(c *gin.Context) {
	entityType := c.Param("type")
	entityID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	setting, err := h.svc.GetSetting(entityType, uint(entityID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SEO setting not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": setting})
}

// UpdateSEOSetting updates SEO settings for a specific entity.
// PUT /api/v1/seo/:type/:id
func (h *SEOHandler) UpdateSEOSetting(c *gin.Context) {
	entityType := c.Param("type")
	entityID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var req services.SEOSettingRequest
	c.ShouldBindJSON(&req)

	if err := h.svc.UpdateSetting(entityType, uint(entityID), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update SEO setting"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "SEO setting updated"})
}

// Sitemap generates the sitemap entries.
// GET /api/v1/seo/sitemap
func (h *SEOHandler) Sitemap(c *gin.Context) {
	xml, err := h.svc.Sitemap()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate sitemap"})
		return
	}

	c.Data(http.StatusOK, "application/xml", []byte(xml))
}

// RobotsTxt generates robots.txt.
// GET /api/v1/seo/robots.txt
func (h *SEOHandler) RobotsTxt(c *gin.Context) {
	txt := h.svc.RobotsTxt()
	c.Data(http.StatusOK, "text/plain", []byte(txt))
}

// ListRedirects handles URL redirects.
// GET /api/v1/seo/redirects
func (h *SEOHandler) ListRedirects(c *gin.Context) {
	rules, err := h.svc.ListRedirects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch redirects"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": rules})
}

// CreateRedirect creates a redirect rule.
// POST /api/v1/seo/redirects
func (h *SEOHandler) CreateRedirect(c *gin.Context) {
	var req services.CreateRedirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule, err := h.svc.CreateRedirect(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create redirect"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": rule})
}

// DeleteRedirect deletes a redirect rule.
// DELETE /api/v1/seo/redirects/:id
func (h *SEOHandler) DeleteRedirect(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid redirect ID"})
		return
	}

	if err := h.svc.DeleteRedirect(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete redirect"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Redirect deleted"})
}

// ---------- Menu Handler ----------

// MenuHandler handles navigation menus.
type MenuHandler struct {
	svc *services.MenuService
}

func NewMenuHandler(svc *services.MenuService) *MenuHandler {
	return &MenuHandler{svc: svc}
}

// List returns all menus.
// GET /api/v1/menus
func (h *MenuHandler) List(c *gin.Context) {
	menus, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch menus"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": menus})
}

// Get returns a single menu with items.
// GET /api/v1/menus/:id
func (h *MenuHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid menu ID"})
		return
	}

	menu, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Menu not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": menu})
}

// Create creates a new menu.
// POST /api/v1/menus
func (h *MenuHandler) Create(c *gin.Context) {
	var req services.CreateMenuRequest
	c.ShouldBindJSON(&req)

	menu, err := h.svc.Create(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create menu"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": menu})
}

// Update updates a menu.
// PUT /api/v1/menus/:id
func (h *MenuHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid menu ID"})
		return
	}

	var req services.UpdateMenuRequest
	c.ShouldBindJSON(&req)

	if err := h.svc.Update(uint(id), req); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Menu not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Menu updated"})
}

// Delete deletes a menu and its items.
// DELETE /api/v1/menus/:id
func (h *MenuHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid menu ID"})
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Menu not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Menu deleted"})
}

// AddItem adds an item to a menu.
// POST /api/v1/menus/:id/items
func (h *MenuHandler) AddItem(c *gin.Context) {
	menuID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid menu ID"})
		return
	}

	var req services.AddMenuItemRequest
	c.ShouldBindJSON(&req)

	item, err := h.svc.AddItem(uint(menuID), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add item"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": item})
}

// UpdateItem updates a menu item.
// PUT /api/v1/menus/:id/items/:item_id
func (h *MenuHandler) UpdateItem(c *gin.Context) {
	itemID, err := strconv.ParseUint(c.Param("item_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	var req services.UpdateMenuItemRequest
	c.ShouldBindJSON(&req)

	if err := h.svc.UpdateItem(uint(itemID), req); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Menu item not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Item updated"})
}

// DeleteItem removes a menu item.
// DELETE /api/v1/menus/:id/items/:item_id
func (h *MenuHandler) DeleteItem(c *gin.Context) {
	itemID, err := strconv.ParseUint(c.Param("item_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	h.svc.DeleteItem(uint(itemID))
	c.JSON(http.StatusOK, gin.H{"message": "Menu item deleted"})
}

// ReorderItems updates sort order for menu items.
// PUT /api/v1/menus/:id/items/reorder
func (h *MenuHandler) ReorderItems(c *gin.Context) {
	var req struct {
		Items []services.ReorderItem `json:"items" binding:"required"`
	}
	c.ShouldBindJSON(&req)

	if err := h.svc.ReorderItems(req.Items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reorder items"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Items reordered"})
}

// ---------- Analytics Handler ----------

// AnalyticsHandler handles analytics data.
type AnalyticsHandler struct {
	svc *services.AnalyticsService
}

func NewAnalyticsHandler(svc *services.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{svc: svc}
}

// Dashboard returns dashboard statistics.
// GET /api/v1/analytics/dashboard
func (h *AnalyticsHandler) Dashboard(c *gin.Context) {
	data, err := h.svc.Dashboard()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch dashboard"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats":            data.Stats,
		"recent_articles":  data.RecentArticles,
		"recent_comments":  data.RecentComments,
		"popular_articles": data.PopularArticles,
	})
}

// ViewsOverTime returns view data over time for charts.
// GET /api/v1/analytics/views?days=30
func (h *AnalyticsHandler) ViewsOverTime(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 {
		days = 30
	}

	data, err := h.svc.ViewsOverTime(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch views"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// TopReferrers returns top referrers.
// GET /api/v1/analytics/referrers
func (h *AnalyticsHandler) TopReferrers(c *gin.Context) {
	data, err := h.svc.TopReferrers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch referrers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// DeviceBreakdown returns device/browser/OS breakdown.
// GET /api/v1/analytics/devices
func (h *AnalyticsHandler) DeviceBreakdown(c *gin.Context) {
	data, err := h.svc.DeviceBreakdown()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch devices"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"devices":  data.Devices,
		"browsers": data.Browsers,
		"os":       data.OS,
	})
}

// RecordView records a page view.
// POST /api/v1/analytics/record
func (h *AnalyticsHandler) RecordView(c *gin.Context) {
	var req services.RecordViewRequest
	c.ShouldBindJSON(&req)

	if err := h.svc.RecordView(req, c.ClientIP(), c.Request.UserAgent(),
		c.GetHeader("Referer"), c.GetHeader("X-Session-ID")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record view"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "View recorded"})
}
