package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/yamovo/contentx/internal/models" // swag annotation resolution
	"github.com/yamovo/contentx/internal/services"
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
//
//	@Summary      List settings
//	@Description  Returns all settings, optionally filtered by group (requires settings.view permission)
//	@Tags         Settings
//	@Produce      json
//	@Param        group  query  string  false  "Filter by group"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /settings [get]
func (h *SettingsHandler) List(c *gin.Context) {
	group := c.Query("group")

	settings, grouped, err := h.svc.List(group)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"data": settings, "grouped": grouped})
}

// Get returns a single setting by key.
// GET /api/v1/settings/:key
//
//	@Summary      Get setting
//	@Description  Returns a single setting by key
//	@Tags         Settings
//	@Produce      json
//	@Param        key  path      string  true  "Setting key"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.SiteSetting}
//	@Failure      401  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /settings/{key} [get]
func (h *SettingsHandler) Get(c *gin.Context) {
	setting, err := h.svc.Get(c.Param("key"))
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, setting)
}

// Update updates multiple settings at once.
// PUT /api/v1/settings
//
//	@Summary      Update settings
//	@Description  Updates multiple settings at once (requires settings.manage permission)
//	@Tags         Settings
//	@Accept       json
//	@Produce      json
//	@Param        body  body      map[string]interface{}  true  "Settings key-value map"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /settings [put]
func (h *SettingsHandler) Update(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.Update(req); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Settings updated"})
}

// PublicSettings returns settings safe for public consumption.
// GET /api/v1/settings/public
//
//	@Summary      Public settings
//	@Description  Returns settings safe for public consumption (no auth required)
//	@Tags         Settings
//	@Produce      json
//	@Success      200  {object}  APIResponse{data=object}
//	@Router       /settings/public [get]
func (h *SettingsHandler) PublicSettings(c *gin.Context) {
	settings, err := h.svc.PublicSettings()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, settings)
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
//
//	@Summary      Get SEO setting
//	@Description  Returns SEO settings for a specific entity
//	@Tags         SEO
//	@Produce      json
//	@Param        type  path      string  true  "Entity type (article|category|page)"
//	@Param        id    path      int     true  "Entity ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.SEOSetting}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /seo/{type}/{id} [get]
func (h *SEOHandler) GetSEOSetting(c *gin.Context) {
	entityType := c.Param("type")
	entityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid entity ID")
		return
	}

	setting, err := h.svc.GetSetting(entityType, uint(entityID))
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, setting)
}

// UpdateSEOSetting updates SEO settings for a specific entity.
// PUT /api/v1/seo/:type/:id
//
//	@Summary      Update SEO setting
//	@Description  Updates SEO settings for a specific entity (requires seo.manage permission)
//	@Tags         SEO
//	@Accept       json
//	@Produce      json
//	@Param        type  path      string                       true  "Entity type (article|category|page)"
//	@Param        id    path      int                          true  "Entity ID"
//	@Param        body  body      services.SEOSettingRequest   true  "SEO setting data"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /seo/{type}/{id} [put]
func (h *SEOHandler) UpdateSEOSetting(c *gin.Context) {
	entityType := c.Param("type")
	entityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid entity ID")
		return
	}

	var req services.SEOSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.UpdateSetting(entityType, uint(entityID), req); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "SEO setting updated"})
}

// Sitemap generates the sitemap entries.
// GET /api/v1/seo/sitemap
//
//	@Summary      Sitemap
//	@Description  Generates the XML sitemap (public, no auth required)
//	@Tags         SEO
//	@Produce      xml
//	@Success      200  {string}  string
//	@Router       /seo/sitemap [get]
func (h *SEOHandler) Sitemap(c *gin.Context) {
	xml, err := h.svc.Sitemap()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	c.Data(http.StatusOK, "application/xml", []byte(xml))
}

// RobotsTxt generates robots.txt.
// GET /api/v1/seo/robots.txt
//
//	@Summary      Robots.txt
//	@Description  Generates the robots.txt file (public, no auth required)
//	@Tags         SEO
//	@Produce      plain
//	@Success      200  {string}  string
//	@Router       /seo/robots.txt [get]
func (h *SEOHandler) RobotsTxt(c *gin.Context) {
	txt := h.svc.RobotsTxt()
	c.Data(http.StatusOK, "text/plain", []byte(txt))
}

// ListRedirects handles URL redirects.
// GET /api/v1/seo/redirects
//
//	@Summary      List redirects
//	@Description  Returns all redirect rules
//	@Tags         SEO
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=[]models.RedirectRule}
//	@Failure      401  {object}  APIResponse
//	@Router       /seo/redirects [get]
func (h *SEOHandler) ListRedirects(c *gin.Context) {
	rules, err := h.svc.ListRedirects()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, rules)
}

// CreateRedirect creates a redirect rule.
// POST /api/v1/seo/redirects
//
//	@Summary      Create redirect
//	@Description  Creates a redirect rule (requires seo.manage permission)
//	@Tags         SEO
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.CreateRedirectRequest  true  "Redirect data"
//	@Security     BearerAuth
//	@Success      201   {object}  APIResponse{data=models.RedirectRule}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /seo/redirects [post]
func (h *SEOHandler) CreateRedirect(c *gin.Context) {
	var req services.CreateRedirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	rule, err := h.svc.CreateRedirect(req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, rule)
}

// DeleteRedirect deletes a redirect rule.
// DELETE /api/v1/seo/redirects/:id
//
//	@Summary      Delete redirect
//	@Description  Deletes a redirect rule (requires seo.manage permission)
//	@Tags         SEO
//	@Produce      json
//	@Param        id   path      int     true  "Redirect ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /seo/redirects/{id} [delete]
func (h *SEOHandler) DeleteRedirect(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid redirect ID")
		return
	}

	if err := h.svc.DeleteRedirect(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Redirect deleted"})
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
//
//	@Summary      List menus
//	@Description  Returns all navigation menus
//	@Tags         Menus
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=[]models.Menu}
//	@Failure      401  {object}  APIResponse
//	@Router       /menus [get]
func (h *MenuHandler) List(c *gin.Context) {
	menus, err := h.svc.List()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, menus)
}

// Get returns a single menu with items.
// GET /api/v1/menus/:id
//
//	@Summary      Get menu
//	@Description  Returns a single menu with its items
//	@Tags         Menus
//	@Produce      json
//	@Param        id   path      int     true  "Menu ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.Menu}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /menus/{id} [get]
func (h *MenuHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid menu ID")
		return
	}

	menu, err := h.svc.Get(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, menu)
}

// Create creates a new menu.
// POST /api/v1/menus
//
//	@Summary      Create menu
//	@Description  Creates a new menu (requires menus.manage permission)
//	@Tags         Menus
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.CreateMenuRequest  true  "Menu data"
//	@Security     BearerAuth
//	@Success      201   {object}  APIResponse{data=models.Menu}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /menus [post]
func (h *MenuHandler) Create(c *gin.Context) {
	var req services.CreateMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	menu, err := h.svc.Create(req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, menu)
}

// Update updates a menu.
// PUT /api/v1/menus/:id
//
//	@Summary      Update menu
//	@Description  Updates a menu (requires menus.manage permission)
//	@Tags         Menus
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                        true  "Menu ID"
//	@Param        body  body      services.UpdateMenuRequest  true  "Menu data"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Failure      404   {object}  APIResponse
//	@Router       /menus/{id} [put]
func (h *MenuHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid menu ID")
		return
	}

	var req services.UpdateMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.Update(uint(id), req); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Menu updated"})
}

// Delete deletes a menu and its items.
// DELETE /api/v1/menus/:id
//
//	@Summary      Delete menu
//	@Description  Deletes a menu and its items (requires menus.manage permission)
//	@Tags         Menus
//	@Produce      json
//	@Param        id   path      int     true  "Menu ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /menus/{id} [delete]
func (h *MenuHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid menu ID")
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Menu deleted"})
}

// AddItem adds an item to a menu.
// POST /api/v1/menus/:id/items
//
//	@Summary      Add menu item
//	@Description  Adds an item to a menu (requires menus.manage permission)
//	@Tags         Menus
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                        true  "Menu ID"
//	@Param        body  body      services.AddMenuItemRequest  true  "Menu item data"
//	@Security     BearerAuth
//	@Success      201   {object}  APIResponse{data=models.MenuItem}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /menus/{id}/items [post]
func (h *MenuHandler) AddItem(c *gin.Context) {
	menuID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid menu ID")
		return
	}

	var req services.AddMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	item, err := h.svc.AddItem(uint(menuID), req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, item)
}

// UpdateItem updates a menu item.
// PUT /api/v1/menus/:id/items/:item_id
//
//	@Summary      Update menu item
//	@Description  Updates a menu item (requires menus.manage permission)
//	@Tags         Menus
//	@Accept       json
//	@Produce      json
//	@Param        id        path      int                           true  "Menu ID"
//	@Param        item_id   path      int                           true  "Item ID"
//	@Param        body      body      services.UpdateMenuItemRequest  true  "Item data"
//	@Security     BearerAuth
//	@Success      200       {object}  APIResponse
//	@Failure      400       {object}  APIResponse
//	@Failure      401       {object}  APIResponse
//	@Failure      403       {object}  APIResponse
//	@Router       /menus/{id}/items/{item_id} [put]
func (h *MenuHandler) UpdateItem(c *gin.Context) {
	itemID, err := strconv.ParseUint(c.Param("item_id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid item ID")
		return
	}

	var req services.UpdateMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.UpdateItem(uint(itemID), req); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Item updated"})
}

// DeleteItem removes a menu item.
// DELETE /api/v1/menus/:id/items/:item_id
//
//	@Summary      Delete menu item
//	@Description  Removes a menu item (requires menus.manage permission)
//	@Tags         Menus
//	@Produce      json
//	@Param        id        path      int     true  "Menu ID"
//	@Param        item_id   path      int     true  "Item ID"
//	@Security     BearerAuth
//	@Success      200       {object}  APIResponse
//	@Failure      400       {object}  APIResponse
//	@Failure      401       {object}  APIResponse
//	@Failure      403       {object}  APIResponse
//	@Router       /menus/{id}/items/{item_id} [delete]
func (h *MenuHandler) DeleteItem(c *gin.Context) {
	itemID, err := strconv.ParseUint(c.Param("item_id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid item ID")
		return
	}

	if err := h.svc.DeleteItem(uint(itemID)); err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, gin.H{"message": "Menu item deleted"})
}

// ReorderItems updates sort order for menu items.
// PUT /api/v1/menus/:id/items/reorder
//
//	@Summary      Reorder menu items
//	@Description  Updates sort order for menu items (requires menus.manage permission)
//	@Tags         Menus
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int     true  "Menu ID"
//	@Param        body  body      object  true  "Reorder payload {items}"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /menus/{id}/items/reorder [put]
func (h *MenuHandler) ReorderItems(c *gin.Context) {
	var req struct {
		Items []services.ReorderItem `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.ReorderItems(req.Items); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Items reordered"})
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
//
//	@Summary      Analytics dashboard
//	@Description  Returns dashboard statistics (requires analytics.view permission)
//	@Tags         Analytics
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /analytics/dashboard [get]
func (h *AnalyticsHandler) Dashboard(c *gin.Context) {
	data, err := h.svc.Dashboard()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{
		"stats":            data.Stats,
		"recent_articles":  data.RecentArticles,
		"recent_comments":  data.RecentComments,
		"popular_articles": data.PopularArticles,
	})
}

// ViewsOverTime returns view data over time for charts.
// GET /api/v1/analytics/views?days=30
//
//	@Summary      Views over time
//	@Description  Returns view data over time for charts (requires analytics.view permission)
//	@Tags         Analytics
//	@Produce      json
//	@Param        days  query  int  false  "Number of days"  default(30)
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /analytics/views [get]
func (h *AnalyticsHandler) ViewsOverTime(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 {
		days = 30
	}

	data, err := h.svc.ViewsOverTime(days)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, data)
}

// TopReferrers returns top referrers.
// GET /api/v1/analytics/referrers
//
//	@Summary      Top referrers
//	@Description  Returns top referrers (requires analytics.view permission)
//	@Tags         Analytics
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /analytics/referrers [get]
func (h *AnalyticsHandler) TopReferrers(c *gin.Context) {
	data, err := h.svc.TopReferrers()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, data)
}

// DeviceBreakdown returns device/browser/OS breakdown.
// GET /api/v1/analytics/devices
//
//	@Summary      Device breakdown
//	@Description  Returns device/browser/OS breakdown (requires analytics.view permission)
//	@Tags         Analytics
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /analytics/devices [get]
func (h *AnalyticsHandler) DeviceBreakdown(c *gin.Context) {
	data, err := h.svc.DeviceBreakdown()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{
		"devices":  data.Devices,
		"browsers": data.Browsers,
		"os":       data.OS,
	})
}

// RecordView records a page view.
// POST /api/v1/analytics/record
//
//	@Summary      Record page view
//	@Description  Records a page view (public, no auth required)
//	@Tags         Analytics
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.RecordViewRequest  true  "View data"
//	@Success      201   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Router       /analytics/record [post]
func (h *AnalyticsHandler) RecordView(c *gin.Context) {
	var req services.RecordViewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.RecordView(req, c.ClientIP(), c.Request.UserAgent(),
		c.GetHeader("Referer"), c.GetHeader("X-Session-ID")); err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, gin.H{"message": "View recorded"})
}
