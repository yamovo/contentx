package services

import (
	"runtime"
	"runtime/debug"

	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// ActivityLogParams holds query parameters for activity log listing.
type ActivityLogParams struct {
	Page     int
	PageSize int
	Entity   string
	Action   string
	UserID   string
}

// ---------- PluginService ----------

// PluginService handles plugin business logic.
type PluginService struct {
	db *gorm.DB
}

// NewPluginService creates a new PluginService.
func NewPluginService(db *gorm.DB) *PluginService {
	return &PluginService{db: db}
}

// List returns all plugins ordered by name.
func (s *PluginService) List() ([]models.Plugin, error) {
	var plugins []models.Plugin
	if err := s.db.Order("name ASC").Find(&plugins).Error; err != nil {
		return nil, err
	}
	return plugins, nil
}

// Enable sets a plugin as enabled.
func (s *PluginService) Enable(id uint) error {
	var plugin models.Plugin
	if err := s.db.First(&plugin, id).Error; err != nil {
		return err
	}
	return s.db.Model(&plugin).Update("is_enabled", true).Error
}

// Disable sets a plugin as disabled.
func (s *PluginService) Disable(id uint) error {
	var plugin models.Plugin
	if err := s.db.First(&plugin, id).Error; err != nil {
		return err
	}
	return s.db.Model(&plugin).Update("is_enabled", false).Error
}

// UpdateConfig updates a plugin's configuration JSON.
func (s *PluginService) UpdateConfig(id uint, config map[string]interface{}) error {
	var plugin models.Plugin
	if err := s.db.First(&plugin, id).Error; err != nil {
		return err
	}
	return s.db.Model(&plugin).Update("config", config).Error
}

// ---------- ThemeService ----------

// ThemeService handles theme business logic.
type ThemeService struct {
	db *gorm.DB
}

// NewThemeService creates a new ThemeService.
func NewThemeService(db *gorm.DB) *ThemeService {
	return &ThemeService{db: db}
}

// List returns all themes ordered by name.
func (s *ThemeService) List() ([]models.ThemeConfig, error) {
	var themes []models.ThemeConfig
	if err := s.db.Order("name ASC").Find(&themes).Error; err != nil {
		return nil, err
	}
	return themes, nil
}

// Activate activates a theme and deactivates all others.
func (s *ThemeService) Activate(id uint) error {
	var theme models.ThemeConfig
	if err := s.db.First(&theme, id).Error; err != nil {
		return err
	}

	// Deactivate all other themes.
	if err := s.db.Model(&models.ThemeConfig{}).Where("id != ?", id).Update("is_active", false).Error; err != nil {
		return err
	}
	return s.db.Model(&theme).Update("is_active", true).Error
}

// UpdateConfig updates a theme's configuration JSON.
func (s *ThemeService) UpdateConfig(id uint, config map[string]interface{}) error {
	var theme models.ThemeConfig
	if err := s.db.First(&theme, id).Error; err != nil {
		return err
	}
	return s.db.Model(&theme).Update("config", config).Error
}

// ---------- SystemService ----------

// SystemService provides system information and operations.
type SystemService struct {
	db *gorm.DB
}

// NewSystemService creates a new SystemService.
func NewSystemService(db *gorm.DB) *SystemService {
	return &SystemService{db: db}
}

// Info returns system information as a map.
func (s *SystemService) Info() map[string]interface{} {
	goVersion := runtime.Version()

	info := map[string]interface{}{
		"name":       "VortexCMS",
		"version":    "1.0.0",
		"go_version": goVersion,
		"database":   s.db.Dialector.Name(),
	}

	// Include build info if available.
	if bi, ok := debug.ReadBuildInfo(); ok {
		info["go_version"] = bi.GoVersion
		if bi.Main.Version != "" {
			info["module_version"] = bi.Main.Version
		}
	}

	return info
}

// Health checks whether the system is healthy by pinging the database.
func (s *SystemService) Health() (bool, error) {
	sqlDB, err := s.db.DB()
	if err != nil {
		return false, err
	}
	if err := sqlDB.Ping(); err != nil {
		return false, err
	}
	return true, nil
}

// ActivityLog returns activity log entries with pagination and filters.
func (s *SystemService) ActivityLog(params ActivityLogParams) ([]models.ActivityLog, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 50
	}

	query := s.db.Model(&models.ActivityLog{})

	if params.Entity != "" {
		query = query.Where("entity = ?", params.Entity)
	}
	if params.Action != "" {
		query = query.Where("action = ?", params.Action)
	}
	if params.UserID != "" {
		query = query.Where("user_id = ?", params.UserID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []models.ActivityLog
	if err := query.Order("created_at DESC").Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
