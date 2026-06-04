package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// PluginHandler manages plugins.
type PluginHandler struct{ db *gorm.DB }

func NewPluginHandler(db *gorm.DB) *PluginHandler { return &PluginHandler{db: db} }

// List returns all plugins.
// GET /api/v1/plugins
func (h *PluginHandler) List(c *gin.Context) {
	var plugins []models.Plugin
	h.db.Order("name ASC").Find(&plugins)
	c.JSON(http.StatusOK, gin.H{"data": plugins})
}

// Enable enables a plugin.
// POST /api/v1/plugins/:id/enable
func (h *PluginHandler) Enable(c *gin.Context) {
	var plugin models.Plugin
	if err := h.db.First(&plugin, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}
	h.db.Model(&plugin).Update("is_enabled", true)
	c.JSON(http.StatusOK, gin.H{"message": "Plugin enabled", "data": plugin})
}

// Disable disables a plugin.
// POST /api/v1/plugins/:id/disable
func (h *PluginHandler) Disable(c *gin.Context) {
	var plugin models.Plugin
	if err := h.db.First(&plugin, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}
	h.db.Model(&plugin).Update("is_enabled", false)
	c.JSON(http.StatusOK, gin.H{"message": "Plugin disabled", "data": plugin})
}

// UpdateConfig updates a plugin's configuration.
// PUT /api/v1/plugins/:id/config
func (h *PluginHandler) UpdateConfig(c *gin.Context) {
	var plugin models.Plugin
	if err := h.db.First(&plugin, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}
	var config map[string]interface{}
	c.ShouldBindJSON(&config)
	h.db.Model(&plugin).Update("config", config)
	c.JSON(http.StatusOK, gin.H{"message": "Plugin config updated", "data": plugin})
}

// ---------- Theme Handler ----------

// ThemeHandler manages themes.
type ThemeHandler struct{ db *gorm.DB }

func NewThemeHandler(db *gorm.DB) *ThemeHandler { return &ThemeHandler{db: db} }

// List returns all themes.
// GET /api/v1/themes
func (h *ThemeHandler) List(c *gin.Context) {
	var themes []models.ThemeConfig
	h.db.Order("name ASC").Find(&themes)
	c.JSON(http.StatusOK, gin.H{"data": themes})
}

// Activate activates a theme.
// POST /api/v1/themes/:id/activate
func (h *ThemeHandler) Activate(c *gin.Context) {
	var theme models.ThemeConfig
	if err := h.db.First(&theme, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Theme not found"})
		return
	}

	// Deactivate all others.
	h.db.Model(&models.ThemeConfig{}).Where("id != ?", theme.ID).Update("is_active", false)
	h.db.Model(&theme).Update("is_active", true)

	c.JSON(http.StatusOK, gin.H{"message": "Theme activated", "data": theme})
}

// UpdateConfig updates theme configuration.
// PUT /api/v1/themes/:id/config
func (h *ThemeHandler) UpdateConfig(c *gin.Context) {
	var theme models.ThemeConfig
	if err := h.db.First(&theme, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Theme not found"})
		return
	}
	var config map[string]interface{}
	c.ShouldBindJSON(&config)
	h.db.Model(&theme).Update("config", config)
	c.JSON(http.StatusOK, gin.H{"message": "Theme config updated", "data": theme})
}

// ---------- System Handler ----------

// SystemHandler provides system information and operations.
type SystemHandler struct{ db *gorm.DB }

func NewSystemHandler(db *gorm.DB) *SystemHandler { return &SystemHandler{db: db} }

// Info returns system information.
// GET /api/v1/system/info
func (h *SystemHandler) Info(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"name":        "VortexCMS",
			"version":     "1.0.0",
			"go_version":  "1.22+",
			"database":    h.db.Dialector.Name(),
		},
	})
}

// Health returns the health status.
// GET /api/v1/system/health
func (h *SystemHandler) Health(c *gin.Context) {
	dbOK := h.db.Exec("SELECT 1").Error == nil
	status := "healthy"
	code := http.StatusOK
	if !dbOK {
		status = "unhealthy"
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, gin.H{"status": status, "database": dbOK})
}

// ActivityLog returns the activity log.
// GET /api/v1/system/activity
func (h *SystemHandler) ActivityLog(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	var logs []models.ActivityLog
	var total int64

	query := h.db.Model(&models.ActivityLog{})
	entity := c.Query("entity")
	action := c.Query("action")
	userID := c.Query("user_id")
	if entity != "" { query = query.Where("entity = ?", entity) }
	if action != "" { query = query.Where("action = ?", action) }
	if userID != "" { query = query.Where("user_id = ?", userID) }

	query.Count(&total)
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

	paginate := models.Paginate{Page: page, PageSize: pageSize, Total: total}
	c.JSON(http.StatusOK, models.NewListResponse(logs, paginate))
}

// Need strconv for system handler.
