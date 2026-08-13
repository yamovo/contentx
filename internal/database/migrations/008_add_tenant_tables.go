package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// 008 adds the multi-tenancy foundation tables (RFC-001): tenants and
// tenant_memberships. Fresh installations already receive both tables through
// migration 001's AllModels snapshot, so Up must be idempotent when the tables
// already exist (same guard pattern as 007 for the article version column).
//
// Up also seeds the fixed "default" tenant (id=1, slug=default) and backfills
// a membership row for every existing user into it, using the user's current
// role slug. This keeps all pre-multi-tenancy data inside the default tenant.
//
// Business-table tenant_id columns are added by a follow-up migration (009)
// so that this foundation can land and be verified independently.
func init() {
	RegisterMigrations(database.Migration{
		Version:     8,
		Description: "Add tenants and tenant memberships (multi-tenancy foundation)",
		Up: func(tx *gorm.DB) error {
			// Create tables. AutoMigrate is a no-op when they already exist.
			if err := tx.AutoMigrate(&models.Tenant{}, &models.TenantMembership{}); err != nil {
				return err
			}

			// Seed the fixed default tenant (id = 1, RFC-001 §4.2).
			var count int64
			if err := tx.Model(&models.Tenant{}).Where("slug = ?", "default").Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				if err := tx.Create(&models.Tenant{
					Name:   "Default",
					Slug:   "default",
					Status: models.TenantStatusActive,
				}).Error; err != nil {
					return err
				}
			}

			// Backfill memberships for existing users into the default tenant.
			type userRole struct {
				UserID   uint
				RoleSlug string
			}
			var rows []userRole
			if err := tx.Table("users").
				Select("users.id AS user_id, COALESCE(roles.slug, ?) AS role_slug", models.TenantRoleMember).
				Joins("LEFT JOIN roles ON roles.id = users.role_id").
				Scan(&rows).Error; err != nil {
				return err
			}
			for _, r := range rows {
				var existing int64
				if err := tx.Model(&models.TenantMembership{}).
					Where("tenant_id = ? AND user_id = ?", models.DefaultTenantID, r.UserID).
					Count(&existing).Error; err != nil {
					return err
				}
				if existing > 0 {
					continue
				}
				if err := tx.Create(&models.TenantMembership{
					TenantID: models.DefaultTenantID,
					UserID:   r.UserID,
					RoleSlug: r.RoleSlug,
				}).Error; err != nil {
					return err
				}
			}

			return nil
		},
		Down: func(tx *gorm.DB) error {
			// Data-destructive: drops all tenant and membership data. Back up and
			// stop writers before rolling back (RFC-001 §4.5).
			if err := tx.Migrator().DropTable(&models.TenantMembership{}); err != nil {
				return err
			}
			return tx.Migrator().DropTable(&models.Tenant{})
		},
	})
}
