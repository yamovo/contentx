package repository

import (
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// ActivityLogListFilter holds query parameters for listing activity logs.
type ActivityLogListFilter struct {
	Page     int
	PageSize int
	Entity   string
	Action   string
	UserID   string
}

// PluginRepository defines data-access operations for plugins.
type PluginRepository interface {
	List() ([]models.Plugin, error)
	FindByID(id uint) (*models.Plugin, error)
	UpdateEnabled(id uint, enabled bool) error
	Save(plugin *models.Plugin) error
}

// ThemeRepository defines data-access operations for theme configs.
type ThemeRepository interface {
	List() ([]models.ThemeConfig, error)
	FindByID(id uint) (*models.ThemeConfig, error)
	DeactivateAllExcept(id uint) error
	UpdateActive(id uint, active bool) error
	Save(theme *models.ThemeConfig) error
}

// SystemRepository exposes system-level infrastructure queries and the
// activity-log store. The DialectorName and Ping methods abstract the
// underlying *gorm.DB so SystemService no longer needs to depend on GORM.
type SystemRepository interface {
	DialectorName() string
	Ping() error
	ListActivityLogs(filter ActivityLogListFilter) ([]models.ActivityLog, int64, error)
}

// ---------- Plugin ----------

type gormPluginRepository struct {
	db *gorm.DB
}

// NewPluginRepository builds a GORM-backed PluginRepository.
func NewPluginRepository(db *gorm.DB) PluginRepository {
	return &gormPluginRepository{db: db}
}

func (r *gormPluginRepository) List() ([]models.Plugin, error) {
	var plugins []models.Plugin
	if err := r.db.Order("name ASC").Find(&plugins).Error; err != nil {
		return nil, err
	}
	return plugins, nil
}

func (r *gormPluginRepository) FindByID(id uint) (*models.Plugin, error) {
	var plugin models.Plugin
	if err := r.db.First(&plugin, id).Error; err != nil {
		return nil, err
	}
	return &plugin, nil
}

func (r *gormPluginRepository) UpdateEnabled(id uint, enabled bool) error {
	return r.db.Model(&models.Plugin{}).Where("id = ?", id).
		Update("is_enabled", enabled).Error
}

func (r *gormPluginRepository) Save(plugin *models.Plugin) error {
	return r.db.Save(plugin).Error
}

// ---------- Theme ----------

type gormThemeRepository struct {
	db *gorm.DB
}

// NewThemeRepository builds a GORM-backed ThemeRepository.
func NewThemeRepository(db *gorm.DB) ThemeRepository {
	return &gormThemeRepository{db: db}
}

func (r *gormThemeRepository) List() ([]models.ThemeConfig, error) {
	var themes []models.ThemeConfig
	if err := r.db.Order("name ASC").Find(&themes).Error; err != nil {
		return nil, err
	}
	return themes, nil
}

func (r *gormThemeRepository) FindByID(id uint) (*models.ThemeConfig, error) {
	var theme models.ThemeConfig
	if err := r.db.First(&theme, id).Error; err != nil {
		return nil, err
	}
	return &theme, nil
}

func (r *gormThemeRepository) DeactivateAllExcept(id uint) error {
	return r.db.Model(&models.ThemeConfig{}).
		Where("id != ?", id).
		Update("is_active", false).Error
}

func (r *gormThemeRepository) UpdateActive(id uint, active bool) error {
	return r.db.Model(&models.ThemeConfig{}).Where("id = ?", id).
		Update("is_active", active).Error
}

func (r *gormThemeRepository) Save(theme *models.ThemeConfig) error {
	return r.db.Save(theme).Error
}

// ---------- System ----------

type gormSystemRepository struct {
	db *gorm.DB
}

// NewSystemRepository builds a GORM-backed SystemRepository.
func NewSystemRepository(db *gorm.DB) SystemRepository {
	return &gormSystemRepository{db: db}
}

func (r *gormSystemRepository) DialectorName() string {
	return r.db.Name()
}

func (r *gormSystemRepository) Ping() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func (r *gormSystemRepository) ListActivityLogs(filter ActivityLogListFilter) ([]models.ActivityLog, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 50
	}

	query := r.db.Model(&models.ActivityLog{})

	if filter.Entity != "" {
		query = query.Where("entity = ?", filter.Entity)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.UserID != "" {
		query = query.Where("user_id = ?", filter.UserID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []models.ActivityLog
	if err := query.Order("created_at DESC").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
