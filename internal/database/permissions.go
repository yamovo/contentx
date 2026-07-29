package database

import (
	"fmt"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/permissions"
	"gorm.io/gorm"
)

// SyncCanonicalPermissionsAndRoles upserts the permission registry, carries
// legacy grants forward to custom roles, and repairs the built-in role matrix.
//
// Legacy permission rows are intentionally retained for one release so older
// clients remain readable. Public APIs and new API tokens only emit canonical
// slugs.
func SyncCanonicalPermissionsAndRoles(db *gorm.DB) error {
	canonicalBySlug := make(map[string]models.Permission, len(permissions.Definitions()))
	for _, definition := range permissions.Definitions() {
		permission := models.Permission{}
		if err := db.Where("slug = ?", definition.Slug).
			Assign(models.Permission{
				Name:        definition.Slug,
				Module:      definition.Module,
				Description: definition.Description,
			}).
			FirstOrCreate(&permission, models.Permission{
				Name: definition.Slug,
				Slug: definition.Slug,
			}).Error; err != nil {
			return fmt.Errorf("upsert permission %s: %w", definition.Slug, err)
		}
		canonicalBySlug[definition.Slug] = permission
	}

	if err := migrateLegacyRolePermissions(db, canonicalBySlug); err != nil {
		return err
	}
	if err := syncDefaultRoleMatrix(db, canonicalBySlug); err != nil {
		return err
	}
	if err := canonicalizeStoredAPITokens(db); err != nil {
		return err
	}
	return nil
}

func migrateLegacyRolePermissions(db *gorm.DB, canonicalBySlug map[string]models.Permission) error {
	var stored []models.Permission
	if err := db.Find(&stored).Error; err != nil {
		return fmt.Errorf("list stored permissions: %w", err)
	}

	for _, legacy := range stored {
		if permissions.IsCanonical(legacy.Slug) {
			continue
		}
		targetSlugs := permissions.Canonicalize(legacy.Slug)
		if len(targetSlugs) == 0 {
			continue
		}

		var roleIDs []uint
		if err := db.Table("role_permissions").
			Where("permission_id = ?", legacy.ID).
			Pluck("role_id", &roleIDs).Error; err != nil {
			return fmt.Errorf("list roles for legacy permission %s: %w", legacy.Slug, err)
		}
		if len(roleIDs) == 0 {
			continue
		}

		targets := make([]models.Permission, 0, len(targetSlugs))
		for _, slug := range targetSlugs {
			if permission, ok := canonicalBySlug[slug]; ok {
				targets = append(targets, permission)
			}
		}
		for _, roleID := range roleIDs {
			role := models.Role{BaseModel: models.BaseModel{ID: roleID}}
			if err := db.Model(&role).Association("Permissions").Append(&targets); err != nil {
				return fmt.Errorf("migrate legacy permission %s for role %d: %w", legacy.Slug, roleID, err)
			}
		}
	}
	return nil
}

func syncDefaultRoleMatrix(db *gorm.DB, canonicalBySlug map[string]models.Permission) error {
	// Public registration, when explicitly enabled, must never default to an
	// author-capable account.
	if err := db.Model(&models.Role{}).Where("is_default = ?", true).
		Update("is_default", false).Error; err != nil {
		return fmt.Errorf("clear default roles: %w", err)
	}

	for _, definition := range permissions.DefaultRoles() {
		role := models.Role{}
		if err := db.Where("slug = ?", definition.Slug).
			FirstOrCreate(&role, models.Role{
				Name: definition.Name,
				Slug: definition.Slug,
			}).Error; err != nil {
			return fmt.Errorf("upsert role %s: %w", definition.Slug, err)
		}
		if err := db.Model(&role).Updates(map[string]interface{}{
			"name":        definition.Name,
			"description": definition.Description,
			"is_default": definition.IsDefault,
			"is_system":  true,
		}).Error; err != nil {
			return fmt.Errorf("update role %s: %w", definition.Slug, err)
		}

		rolePermissions := make([]models.Permission, 0, len(definition.Permissions))
		for _, slug := range definition.Permissions {
			permission, ok := canonicalBySlug[slug]
			if !ok {
				return fmt.Errorf("role %s references unknown permission %s", definition.Slug, slug)
			}
			rolePermissions = append(rolePermissions, permission)
		}
		if err := db.Model(&role).Association("Permissions").Replace(&rolePermissions); err != nil {
			return fmt.Errorf("replace permissions for role %s: %w", definition.Slug, err)
		}
	}
	return nil
}

func canonicalizeStoredAPITokens(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.APIToken{}) {
		return nil
	}
	var tokens []models.APIToken
	if err := db.Find(&tokens).Error; err != nil {
		return fmt.Errorf("list API tokens: %w", err)
	}
	for i := range tokens {
		canonical, _ := permissions.CanonicalizeList([]string(tokens[i].Permissions))
		if err := db.Model(&tokens[i]).Update("permissions", models.StringSlice(canonical)).Error; err != nil {
			return fmt.Errorf("canonicalize API token %d: %w", tokens[i].ID, err)
		}
	}
	return nil
}
