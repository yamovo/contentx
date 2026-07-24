package services

import (
	"runtime"
	"runtime/debug"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/plugin"
	"github.com/yamovo/contentx/internal/repository"
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
	repo    repository.PluginRepository
	manager *plugin.Manager
}

// NewPluginService creates a PluginService backed by a GORM repository.
func NewPluginService(db *gorm.DB) *PluginService {
	return &PluginService{repo: repository.NewPluginRepository(db)}
}

// NewPluginServiceWithRepo builds a PluginService with an explicit repository.
func NewPluginServiceWithRepo(repo repository.PluginRepository) *PluginService {
	return &PluginService{repo: repo}
}

// SetPluginManager attaches the runtime plugin manager so that enable/disable
// calls are mirrored at runtime (not just in the DB).
func (s *PluginService) SetPluginManager(m *plugin.Manager) { s.manager = m }

// List returns all plugins ordered by name.
func (s *PluginService) List() ([]models.Plugin, error) {
	return s.repo.List()
}

// Enable sets a plugin as enabled in the DB and at runtime.
func (s *PluginService) Enable(id uint) error {
	p, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateEnabled(id, true); err != nil {
		return err
	}
	if s.manager != nil {
		// Best-effort runtime sync; DB is the source of truth.
		_ = s.manager.Enable(p.Name)
	}
	return nil
}

// Disable sets a plugin as disabled in the DB and at runtime.
func (s *PluginService) Disable(id uint) error {
	p, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateEnabled(id, false); err != nil {
		return err
	}
	if s.manager != nil {
		_ = s.manager.Disable(p.Name)
	}
	return nil
}

// UpdateConfig updates a plugin's configuration JSON and reloads it at runtime.
func (s *PluginService) UpdateConfig(id uint, config map[string]interface{}) error {
	p, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	p.Config = config
	if err := s.repo.Save(p); err != nil {
		return err
	}
	if s.manager != nil {
		_ = s.manager.Reload(p.Name)
	}
	return nil
}

// ---------- ThemeService ----------

// ThemeService handles theme business logic.
type ThemeService struct {
	repo repository.ThemeRepository
}

// NewThemeService creates a ThemeService backed by a GORM repository.
func NewThemeService(db *gorm.DB) *ThemeService {
	return &ThemeService{repo: repository.NewThemeRepository(db)}
}

// NewThemeServiceWithRepo builds a ThemeService with an explicit repository.
func NewThemeServiceWithRepo(repo repository.ThemeRepository) *ThemeService {
	return &ThemeService{repo: repo}
}

// List returns all themes ordered by name.
func (s *ThemeService) List() ([]models.ThemeConfig, error) {
	return s.repo.List()
}

// Activate activates a theme and deactivates all others.
func (s *ThemeService) Activate(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		return err
	}
	if err := s.repo.DeactivateAllExcept(id); err != nil {
		return err
	}
	return s.repo.UpdateActive(id, true)
}

// UpdateConfig updates a theme's configuration JSON.
func (s *ThemeService) UpdateConfig(id uint, config map[string]interface{}) error {
	theme, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	theme.Config = config
	return s.repo.Save(theme)
}

// ---------- SystemService ----------

// MetricsGaugeSetter is the subset of the Prometheus collector used by
// SystemService to publish business gauges. Defined here to avoid importing
// the middleware package into services.
type MetricsGaugeSetter interface {
	SetGauge(name string, help string, value float64)
	SetGaugeWithLabels(name, help string, labels map[string]string, value float64)
}

// SystemService provides system information and operations.
type SystemService struct {
	repo   repository.SystemRepository
	gauges MetricsGaugeSetter
}

// NewSystemService creates a SystemService backed by a GORM repository.
func NewSystemService(db *gorm.DB) *SystemService {
	return &SystemService{repo: repository.NewSystemRepository(db)}
}

// NewSystemServiceWithRepo builds a SystemService with an explicit repository.
func NewSystemServiceWithRepo(repo repository.SystemRepository) *SystemService {
	return &SystemService{repo: repo}
}

// SetMetricsCollector wires a gauge setter (typically *middleware.PrometheusCollector)
// so that SnapshotMetrics can publish business gauges.
func (s *SystemService) SetMetricsCollector(gauges MetricsGaugeSetter) {
	s.gauges = gauges
}

// Info returns system information as a map.
func (s *SystemService) Info() map[string]interface{} {
	goVersion := runtime.Version()

	info := map[string]interface{}{
		"name":       "ContentX",
		"version":    "1.0.0",
		"go_version": goVersion,
		"database":   s.repo.DialectorName(),
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
	if err := s.repo.Ping(); err != nil {
		return false, err
	}
	return true, nil
}

// ActivityLog returns activity log entries with pagination and filters.
func (s *SystemService) ActivityLog(params ActivityLogParams) ([]models.ActivityLog, int64, error) {
	return s.repo.ListActivityLogs(repository.ActivityLogListFilter{
		Page:     params.Page,
		PageSize: params.PageSize,
		Entity:   params.Entity,
		Action:   params.Action,
		UserID:   params.UserID,
	})
}

// SnapshotMetrics collects business gauges (active users, articles by status,
// DB connections in use) and publishes them via the registered gauge setter.
// Intended to be called as a Prometheus snapshotter callback right before
// /metrics is scraped. Errors are logged but never propagated — a transient
// DB failure should not break the metrics endpoint.
func (s *SystemService) SnapshotMetrics() {
	if s.gauges == nil {
		return
	}

	if count, err := s.repo.CountActiveUsers(); err == nil {
		s.gauges.SetGauge("active_users_total", "Current number of active users", float64(count))
	}

	for _, status := range []models.ArticleStatus{
		models.StatusDraft, models.StatusPending, models.StatusScheduled,
		models.StatusPublished, models.StatusArchived, models.StatusTrash,
	} {
		if count, err := s.repo.CountArticlesByStatus(string(status)); err == nil {
			s.gauges.SetGaugeWithLabels("articles_total", "Current number of articles",
				map[string]string{"status": string(status)}, float64(count))
		}
	}

	if inUse, err := s.repo.DBConnectionsInUse(); err == nil {
		s.gauges.SetGauge("db_connections_in_use", "Database connections currently in use", float64(inUse))
	}
}
