package plugin

import (
	"strings"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// StateStore is the narrow persistence contract the plugin Manager uses for
// registry state (config + enabled flag). SEC-7: the Manager no longer holds
// a full *gorm.DB — its database access is confined to this interface, whose
// GORM implementation only ever touches the plugins table. Plugins themselves
// receive nothing but their own config map.
type StateStore interface {
	// Load returns the persisted config and enabled flag for a plugin slug.
	// found is false when the plugin has no persisted row yet.
	Load(slug string) (config map[string]interface{}, enabled bool, found bool)
	// SetEnabled persists the enabled flag for a plugin slug.
	SetEnabled(slug string, enabled bool) error
	// Ensure creates the registry row for a plugin if it does not exist.
	Ensure(rec models.Plugin) error
}

// gormStateStore implements StateStore on the plugins table only.
type gormStateStore struct {
	db *gorm.DB
}

// NewGormStateStore wraps a *gorm.DB into the plugins-table-only StateStore.
func NewGormStateStore(db *gorm.DB) StateStore {
	return &gormStateStore{db: db}
}

func (s *gormStateStore) Load(slug string) (map[string]interface{}, bool, bool) {
	var dbPlugin models.Plugin
	if err := s.db.Where("slug = ?", strings.ToLower(slug)).First(&dbPlugin).Error; err != nil {
		return nil, true, false
	}
	return dbPlugin.Config, dbPlugin.IsEnabled, true
}

func (s *gormStateStore) SetEnabled(slug string, enabled bool) error {
	return s.db.Model(&models.Plugin{}).
		Where("slug = ?", strings.ToLower(slug)).
		Update("is_enabled", enabled).Error
}

func (s *gormStateStore) Ensure(rec models.Plugin) error {
	var count int64
	s.db.Model(&models.Plugin{}).Where("slug = ?", rec.Slug).Count(&count)
	if count > 0 {
		return nil
	}
	return s.db.Create(&rec).Error
}
