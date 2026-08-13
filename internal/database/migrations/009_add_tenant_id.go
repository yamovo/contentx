package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// 009 adds tenant scoping to business tables (RFC-001 §4.3–§4.5):
//
//   - 17 business tables gain a NOT NULL tenant_id column (default 1, the
//     seeded "default" tenant) plus a tenant_id index; existing rows are
//     backfilled into the default tenant.
//   - article_tags is intentionally excluded: it is the many2many join table
//     for articles.tags and its rows are implicitly scoped through
//     articles.tenant_id (RFC-001 §12 appendix note).
//   - sitemap_entries is intentionally excluded: models.SitemapEntry is a
//     reserved definition that is not registered in database.AllModels and has
//     no persistence today; it gains tenant scoping when it becomes a real
//     table.
//   - 4 global tables (site_settings, page_views, activity_logs, api_tokens)
//     gain a nullable tenant_id (NULL = global/default scope).
//   - Global unique indexes are replaced by tenant-scoped composite unique
//     indexes so slugs/UIDs may repeat across tenants.
//
// Model tags keep the original single-column uniqueIndex so that the legacy
// AutoMigrate test path and migration 001's snapshot remain unchanged; the
// composite unique indexes are created here exclusively, keeping fresh
// (001+009) and upgraded databases identical.
func init() {
	RegisterMigrations(database.Migration{
		Version:     9,
		Description: "Add tenant_id to business tables and tenant-scoped unique indexes",
		Up: func(tx *gorm.DB) error {
			// NOT NULL columns with default 1 (backfill target).
			for _, t := range tenantScopedTables() {
				if err := addTenantColumn(tx, t.model, t.table); err != nil {
					return err
				}
			}
			// Nullable columns for global tables (NULL = global scope).
			for _, t := range tenantNullableTables() {
				if err := addNullableTenantColumn(tx, t.model, t.table); err != nil {
					return err
				}
			}
			// Backfill all existing rows into the default tenant.
			for _, t := range tenantScopedTables() {
				if err := tx.Table(t.table).
					Where("tenant_id IS NULL OR tenant_id = ?", 0).
					UpdateColumn("tenant_id", 1).Error; err != nil {
					return err
				}
			}
			// Replace global unique indexes with tenant-scoped composites.
			for _, u := range tenantUniqueReplaces() {
				if err := replaceUnique(tx, u.table, u.oldName, u.newName, u.columns); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(tx *gorm.DB) error {
			// Data-destructive: drops tenant_id columns and tenant-scoped
			// indexes. Back up and stop writers before rolling back.
			//
			// Order matters: SQLite implements DropColumn by rebuilding the
			// table, which wipes ALL indexes. So: drop tenant-scoped indexes,
			// drop columns (table rebuild clears everything), then recreate
			// the original global unique indexes last.
			for _, u := range tenantUniqueReplaces() {
				if tx.Migrator().HasIndex(u.table, u.newName) {
					if err := tx.Migrator().DropIndex(u.table, u.newName); err != nil {
						return err
					}
				}
			}
			for _, t := range tenantScopedTables() {
				if err := dropTenantColumn(tx, t.model, t.table); err != nil {
					return err
				}
			}
			for _, t := range tenantNullableTables() {
				if err := dropTenantColumn(tx, t.model, t.table); err != nil {
					return err
				}
			}
			for _, u := range tenantUniqueReplaces() {
				if !tx.Migrator().HasIndex(u.table, u.oldName) {
					if err := tx.Exec("CREATE UNIQUE INDEX " + u.oldName + " ON " + u.table + " (" + u.oldColumns + ")").Error; err != nil {
						return err
					}
				}
			}
			return nil
		},
	})
}

// tenantTableModel pairs a GORM model with its table name.
type tenantTableModel struct {
	model interface{}
	table string
}

// tenantScopedTables returns business tables with a NOT NULL tenant_id.
func tenantScopedTables() []tenantTableModel {
	return []tenantTableModel{
		{&models.Article{}, "articles"},
		{&models.Category{}, "categories"},
		{&models.Tag{}, "tags"},
		{&models.Comment{}, "comments"},
		{&models.Media{}, "media"},
		{&models.Revision{}, "revisions"},
		{&models.CustomField{}, "custom_fields"},
		{&models.Menu{}, "menus"},
		{&models.MenuItem{}, "menu_items"},
		{&models.SEOSetting{}, "seo_settings"},
		{&models.RedirectRule{}, "redirect_rules"},
		{&models.Webhook{}, "webhooks"},
		{&models.WebhookLog{}, "webhook_logs"},
		{&models.WebhookDelivery{}, "webhook_deliveries"},
		{&models.ContentType{}, "content_types"},
		{&models.ContentField{}, "content_fields"},
		{&models.ContentEntry{}, "content_entries"},
	}
}

// tenantNullableTables returns global tables with a nullable tenant_id.
func tenantNullableTables() []tenantTableModel {
	return []tenantTableModel{
		{&models.SiteSetting{}, "site_settings"},
		{&models.PageView{}, "page_views"},
		{&models.ActivityLog{}, "activity_logs"},
		{&models.APIToken{}, "api_tokens"},
	}
}

// uniqueReplace describes replacing a global unique index with a tenant-scoped
// composite unique index.
type uniqueReplace struct {
	table      string
	oldName    string
	oldColumns string
	newName    string
	columns    string
}

// tenantUniqueReplaces lists every unique index that must become
// (tenant_id, ...) composite (RFC-001 §4.4).
func tenantUniqueReplaces() []uniqueReplace {
	return []uniqueReplace{
		{"articles", "idx_articles_slug", "slug", "idx_articles_tenant_slug", "tenant_id, slug"},
		{"categories", "idx_categories_name", "name", "idx_categories_tenant_name", "tenant_id, name"},
		{"categories", "idx_categories_slug", "slug", "idx_categories_tenant_slug", "tenant_id, slug"},
		{"tags", "idx_tags_name", "name", "idx_tags_tenant_name", "tenant_id, name"},
		{"tags", "idx_tags_slug", "slug", "idx_tags_tenant_slug", "tenant_id, slug"},
		{"menus", "idx_menus_slug", "slug", "idx_menus_tenant_slug", "tenant_id, slug"},
		{"redirect_rules", "idx_redirect_rules_from_path", "from_path", "idx_redirect_rules_tenant_from_path", "tenant_id, from_path"},
		{"seo_settings", "idx_seo_entity", "entity_type, entity_id", "idx_seo_tenant_entity", "tenant_id, entity_type, entity_id"},
		{"site_settings", "idx_site_settings_key", "key", "idx_site_settings_tenant_key", "tenant_id, key"},
		{"content_types", "idx_content_types_uid", "uid", "idx_content_types_tenant_uid", "tenant_id, uid"},
		{"content_entries", "idx_content_entries_document_id", "document_id", "idx_content_entries_tenant_document_id", "tenant_id, document_id"},
	}
}

// addTenantColumn adds a NOT NULL tenant_id column (default 1) and its index.
func addTenantColumn(tx *gorm.DB, model interface{}, table string) error {
	if !tx.Migrator().HasColumn(model, "TenantID") {
		if err := tx.Migrator().AddColumn(model, "TenantID"); err != nil {
			return err
		}
	}
	idx := "idx_" + table + "_tenant_id"
	if !tx.Migrator().HasIndex(table, idx) {
		if err := tx.Exec("CREATE INDEX " + idx + " ON " + table + " (tenant_id)").Error; err != nil {
			return err
		}
	}
	return nil
}

// addNullableTenantColumn adds a nullable tenant_id column and its index.
func addNullableTenantColumn(tx *gorm.DB, model interface{}, table string) error {
	if !tx.Migrator().HasColumn(model, "TenantID") {
		if err := tx.Migrator().AddColumn(model, "TenantID"); err != nil {
			return err
		}
	}
	idx := "idx_" + table + "_tenant_id"
	if !tx.Migrator().HasIndex(table, idx) {
		if err := tx.Exec("CREATE INDEX " + idx + " ON " + table + " (tenant_id)").Error; err != nil {
			return err
		}
	}
	return nil
}

// dropTenantColumn removes the tenant_id index and column (guarded).
func dropTenantColumn(tx *gorm.DB, model interface{}, table string) error {
	idx := "idx_" + table + "_tenant_id"
	if tx.Migrator().HasIndex(table, idx) {
		if err := tx.Migrator().DropIndex(table, idx); err != nil {
			return err
		}
	}
	if tx.Migrator().HasColumn(model, "TenantID") {
		return tx.Migrator().DropColumn(model, "TenantID")
	}
	return nil
}

// replaceUnique drops the old global unique index and creates the new
// tenant-scoped composite unique index (both guarded).
func replaceUnique(tx *gorm.DB, table, oldName, newName, columns string) error {
	if tx.Migrator().HasIndex(table, oldName) {
		if err := tx.Migrator().DropIndex(table, oldName); err != nil {
			return err
		}
	}
	if !tx.Migrator().HasIndex(table, newName) {
		if err := tx.Exec("CREATE UNIQUE INDEX " + newName + " ON " + table + " (" + columns + ")").Error; err != nil {
			return err
		}
	}
	return nil
}

// addTenantColumn adds a NOT NULL tenant_id column (default 1) and its index.
