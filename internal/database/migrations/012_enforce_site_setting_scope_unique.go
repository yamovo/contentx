package migrations

import (
	"fmt"

	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

const (
	siteSettingGlobalKeyIndex = "idx_site_settings_global_key"
	siteSettingScopeKeyIndex  = "idx_site_settings_scope_key"
	siteSettingScopeColumn    = "tenant_scope_id"
)

// 012 closes the NULL uniqueness hole left by UNIQUE(tenant_id, key).
// SQLite and PostgreSQL use a partial unique index for global rows. MySQL,
// which has no partial indexes, uses a generated COALESCE(tenant_id, 0) column
// and a unique index over that scope plus key. Tenant IDs are positive, so zero
// is reserved for the global scope.
func init() {
	RegisterMigrations(database.Migration{
		Version:     12,
		Description: "Enforce unique global and tenant site setting keys",
		Up:          ensureSiteSettingScopeUniqueness,
		Down:        dropSiteSettingScopeUniqueness,
	})
}

func ensureSiteSettingScopeUniqueness(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&models.SiteSetting{}) {
		return nil
	}
	if err := rejectDuplicateSiteSettingScopes(tx); err != nil {
		return err
	}

	switch tx.Name() {
	case "sqlite", "postgres":
		if tx.Migrator().HasIndex("site_settings", siteSettingGlobalKeyIndex) {
			return nil
		}
		return tx.Exec(
			"CREATE UNIQUE INDEX " + siteSettingGlobalKeyIndex +
				" ON site_settings (\"key\") WHERE tenant_id IS NULL",
		).Error
	case "mysql":
		if !tx.Migrator().HasColumn("site_settings", siteSettingScopeColumn) {
			if err := tx.Exec(
				"ALTER TABLE site_settings ADD COLUMN " + siteSettingScopeColumn +
					" BIGINT UNSIGNED GENERATED ALWAYS AS (IFNULL(tenant_id, 0)) STORED",
			).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasIndex("site_settings", siteSettingScopeKeyIndex) {
			return nil
		}
		return tx.Exec(
			"CREATE UNIQUE INDEX " + siteSettingScopeKeyIndex +
				" ON site_settings (" + siteSettingScopeColumn + ", `key`)",
		).Error
	default:
		return fmt.Errorf("unsupported database dialect %q for site setting scope uniqueness", tx.Name())
	}
}

func dropSiteSettingScopeUniqueness(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&models.SiteSetting{}) {
		return nil
	}

	switch tx.Name() {
	case "sqlite", "postgres":
		if tx.Migrator().HasIndex("site_settings", siteSettingGlobalKeyIndex) {
			return tx.Migrator().DropIndex("site_settings", siteSettingGlobalKeyIndex)
		}
		return nil
	case "mysql":
		if tx.Migrator().HasIndex("site_settings", siteSettingScopeKeyIndex) {
			if err := tx.Migrator().DropIndex("site_settings", siteSettingScopeKeyIndex); err != nil {
				return err
			}
		}
		if tx.Migrator().HasColumn("site_settings", siteSettingScopeColumn) {
			return tx.Exec("ALTER TABLE site_settings DROP COLUMN " + siteSettingScopeColumn).Error
		}
		return nil
	default:
		return fmt.Errorf("unsupported database dialect %q for site setting scope uniqueness", tx.Name())
	}
}

func rejectDuplicateSiteSettingScopes(tx *gorm.DB) error {
	var settings []models.SiteSetting
	if err := tx.Select("id", "tenant_id", "key").Order("id ASC").Find(&settings).Error; err != nil {
		return err
	}

	type scopeKey struct {
		TenantID uint
		Key      string
	}
	seen := make(map[scopeKey]uint, len(settings))
	for _, setting := range settings {
		tenantID := uint(0)
		if setting.TenantID != nil {
			tenantID = *setting.TenantID
		}
		scope := scopeKey{TenantID: tenantID, Key: setting.Key}
		if existingID, exists := seen[scope]; exists {
			return fmt.Errorf(
				"cannot enforce site setting scope uniqueness: rows %d and %d share tenant scope %d and key %q; deduplicate them before retrying migration 012",
				existingID, setting.ID, tenantID, setting.Key,
			)
		}
		seen[scope] = setting.ID
	}
	return nil
}
