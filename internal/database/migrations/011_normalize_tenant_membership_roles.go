package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// 011 normalizes legacy authentication data to the fixed tenant-role model.
//
// Migration 008 copied each user's global role slug into tenant_memberships.
// Custom roles therefore produced values that TenantGuard cannot safely map to
// a tenant permission ceiling. Preserve the three canonical tenant roles and
// downgrade every legacy/custom value to the least-privileged member role.
//
// Migration 009 also introduced a nullable tenant_id on pre-existing API
// tokens. Those tokens came from the former single-tenant system, so bind them
// explicitly to the seeded default tenant. Runtime token resolution can then
// stay fail-closed for any future corrupt/NULL rows without breaking upgrades.
func init() {
	RegisterMigrations(database.Migration{
		Version:     11,
		Description: "Normalize legacy tenant authorization data",
		Up: func(tx *gorm.DB) error {
			if err := tx.Model(&models.TenantMembership{}).
				Where("role_slug NOT IN ?", []string{
					models.TenantRoleMember,
					models.TenantRoleEditor,
					models.TenantRoleAdmin,
				}).
				Update("role_slug", models.TenantRoleMember).Error; err != nil {
				return err
			}
			return tx.Model(&models.APIToken{}).
				Where("tenant_id IS NULL OR tenant_id = ?", 0).
				Update("tenant_id", models.DefaultTenantID).Error
		},
		Down: func(_ *gorm.DB) error {
			// The original custom slug and implicit API-token tenant are
			// intentionally not retained: restoring either would recreate an
			// ambiguous authorization state. This normalization is irreversible.
			return nil
		},
	})
}
