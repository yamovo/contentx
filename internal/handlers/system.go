package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/yamovo/contentx/internal/models" // swag annotation resolution
	"github.com/yamovo/contentx/internal/services"
)

// PluginHandler manages plugins.
type PluginHandler struct {
	svc *services.PluginService
}

func NewPluginHandler(svc *services.PluginService) *PluginHandler {
	return &PluginHandler{svc: svc}
}

// List returns all plugins.
// GET /api/v1/plugins
//
//	@Summary      List plugins
//	@Description  Returns all installed plugins
//	@Tags         Plugins
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=[]models.Plugin}
//	@Failure      401  {object}  APIResponse
//	@Router       /plugins [get]
func (h *PluginHandler) List(c *gin.Context) {
	plugins, err := h.svc.List()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, plugins)
}

// Enable enables a plugin.
// POST /api/v1/plugins/:id/enable
//
//	@Summary      Enable plugin
//	@Description  Enables a plugin (requires plugins.manage permission)
//	@Tags         Plugins
//	@Produce      json
//	@Param        id   path      int     true  "Plugin ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /plugins/{id}/enable [post]
func (h *PluginHandler) Enable(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid plugin ID")
		return
	}

	if err := h.svc.Enable(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Plugin enabled"})
}

// Disable disables a plugin.
// POST /api/v1/plugins/:id/disable
//
//	@Summary      Disable plugin
//	@Description  Disables a plugin (requires plugins.manage permission)
//	@Tags         Plugins
//	@Produce      json
//	@Param        id   path      int     true  "Plugin ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /plugins/{id}/disable [post]
func (h *PluginHandler) Disable(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid plugin ID")
		return
	}

	if err := h.svc.Disable(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Plugin disabled"})
}

// UpdateConfig updates a plugin's configuration.
// PUT /api/v1/plugins/:id/config
//
//	@Summary      Update plugin config
//	@Description  Updates a plugin's configuration (requires plugins.manage permission)
//	@Tags         Plugins
//	@Accept       json
//	@Produce      json
//	@Param        id     path      int                          true  "Plugin ID"
//	@Param        body   body      map[string]interface{}       true  "Plugin configuration"
//	@Security     BearerAuth
//	@Success      200    {object}  APIResponse
//	@Failure      400    {object}  APIResponse
//	@Failure      401    {object}  APIResponse
//	@Failure      403    {object}  APIResponse
//	@Router       /plugins/{id}/config [put]
func (h *PluginHandler) UpdateConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid plugin ID")
		return
	}

	var config map[string]interface{}
	if err := c.ShouldBindJSON(&config); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.UpdateConfig(uint(id), config); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Plugin config updated"})
}

// ---------- Theme Handler ----------

// ThemeHandler manages themes.
type ThemeHandler struct {
	svc *services.ThemeService
}

func NewThemeHandler(svc *services.ThemeService) *ThemeHandler {
	return &ThemeHandler{svc: svc}
}

// List returns all themes.
// GET /api/v1/themes
//
//	@Summary      List themes
//	@Description  Returns all installed themes
//	@Tags         Themes
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=[]models.ThemeConfig}
//	@Failure      401  {object}  APIResponse
//	@Router       /themes [get]
func (h *ThemeHandler) List(c *gin.Context) {
	themes, err := h.svc.List()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, themes)
}

// Activate activates a theme.
// POST /api/v1/themes/:id/activate
//
//	@Summary      Activate theme
//	@Description  Activates a theme (requires themes.manage permission)
//	@Tags         Themes
//	@Produce      json
//	@Param        id   path      int     true  "Theme ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /themes/{id}/activate [post]
func (h *ThemeHandler) Activate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid theme ID")
		return
	}

	if err := h.svc.Activate(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Theme activated"})
}

// UpdateConfig updates theme configuration.
// PUT /api/v1/themes/:id/config
//
//	@Summary      Update theme config
//	@Description  Updates theme configuration (requires themes.manage permission)
//	@Tags         Themes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                          true  "Theme ID"
//	@Param        body  body      map[string]interface{}       true  "Theme configuration"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /themes/{id}/config [put]
func (h *ThemeHandler) UpdateConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid theme ID")
		return
	}

	var config map[string]interface{}
	if err := c.ShouldBindJSON(&config); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.UpdateConfig(uint(id), config); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Theme config updated"})
}

// ---------- System Handler ----------

// SystemHandler provides system information and operations.
type SystemHandler struct {
	svc *services.SystemService
}

func NewSystemHandler(svc *services.SystemService) *SystemHandler {
	return &SystemHandler{svc: svc}
}

// Info returns system information.
// GET /api/v1/system/info
//
//	@Summary      System info
//	@Description  Returns system information (version, uptime, Go version, etc.)
//	@Tags         System
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Router       /system/info [get]
func (h *SystemHandler) Info(c *gin.Context) {
	Success(c, h.svc.Info())
}

// Health returns the health status.
// GET /api/v1/system/health
//
//	@Summary      Health check
//	@Description  Returns service health status (public, no auth required)
//	@Tags         System
//	@Produce      json
//	@Success      200  {object}  object{status=string,database=bool}
//	@Failure      503  {object}  object{status=string,database=bool}
//	@Router       /system/health [get]
func (h *SystemHandler) Health(c *gin.Context) {
	ok, err := h.svc.Health()
	status := "healthy"
	code := http.StatusOK
	if !ok || err != nil {
		status = "unhealthy"
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{"status": status, "database": ok})
}

// ActivityLog returns the activity log.
// GET /api/v1/system/activity
//
//	@Summary      Activity log
//	@Description  Returns paginated activity audit trail
//	@Tags         System
//	@Produce      json
//	@Param        page      query  int     false  "Page number"  default(1)
//	@Param        page_size query  int     false  "Items per page"  default(50)
//	@Param        entity    query  string  false  "Filter by entity type"
//	@Param        action    query  string  false  "Filter by action"
//	@Param        user_id   query  string  false  "Filter by user ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=[]models.ActivityLog}
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /system/activity [get]
func (h *SystemHandler) ActivityLog(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	params := services.ActivityLogParams{
		Page:     page,
		PageSize: pageSize,
		Entity:   c.Query("entity"),
		Action:   c.Query("action"),
		UserID:   c.Query("user_id"),
	}

	logs, total, err := h.svc.ActivityLog(params)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	paginate := paginateFrom(page, pageSize, total)
	Success(c, listResponse(logs, paginate))
}
