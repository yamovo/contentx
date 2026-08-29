package migrations

import (
	"strings"
	"testing"

	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newMigrationTestDB opens an empty in-memory SQLite database. Schema is
// expected to come exclusively from the registered migrations.
func newMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

// findMigration returns the registered migration with the given version.
func findMigration(t *testing.T, version int) database.Migration {
	t.Helper()
	for _, mig := range All() {
		if mig.Version == version {
			return mig
		}
	}
	t.Fatalf("migration version %d not registered", version)
	return database.Migration{}
}

func TestAll_VersionsSequentialFromOne(t *testing.T) {
	migs := All()
	if len(migs) == 0 {
		t.Fatal("no migrations registered")
	}
	for i, mig := range migs {
		want := i + 1
		if mig.Version != want {
			t.Errorf("migration at position %d has version %d, want %d (versions must be consecutive)", i, mig.Version, want)
		}
		if mig.Description == "" {
			t.Errorf("migration %d has empty description", mig.Version)
		}
		if mig.Up == nil || mig.Down == nil {
			t.Errorf("migration %d must define both Up and Down", mig.Version)
		}
	}
}

func TestRealMigrations_UpCreatesSchemaAndIndexes(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)

	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	// 001: core tables from AllModels.
	for _, table := range []string{"users", "articles", "activity_logs", "content_types", "webhooks"} {
		if !db.Migrator().HasTable(table) {
			t.Errorf("table %q should exist after Up()", table)
		}
	}
	// 002: composite index on activity_logs.
	if !db.Migrator().HasIndex(&models.ActivityLog{}, "idx_activity_logs_entity_created") {
		t.Error("idx_activity_logs_entity_created should exist after Up()")
	}
	// 003: composite list-sort index on articles.
	if !db.Migrator().HasIndex(&models.Article{}, "idx_articles_list_sort") {
		t.Error("idx_articles_list_sort should exist after Up()")
	}
}

func TestRealMigrations_DownRollsBackEverything(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)

	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}
	if err := m.Down(len(All())); err != nil {
		t.Fatalf("Down(%d) error: %v", len(All()), err)
	}

	for _, table := range []string{"users", "articles", "activity_logs"} {
		if db.Migrator().HasTable(table) {
			t.Errorf("table %q should be dropped after full rollback", table)
		}
	}

	// Status must show nothing applied.
	statuses, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	for _, s := range statuses {
		if s.Applied {
			t.Errorf("migration %d should not be applied after full rollback", s.Version)
		}
	}
}

func TestRealMigrations_ReplayAfterFullRollback(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)

	if err := m.Up(); err != nil {
		t.Fatalf("first Up() error: %v", err)
	}
	if err := m.Down(len(All())); err != nil {
		t.Fatalf("Down() error: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("replay Up() error: %v", err)
	}

	if !db.Migrator().HasTable("articles") {
		t.Error("articles table should exist after replay")
	}
	if !db.Migrator().HasIndex(&models.Article{}, "idx_articles_list_sort") {
		t.Error("idx_articles_list_sort should exist after replay")
	}
}

func TestIndexMigrations_UpIdempotent(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)
	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	// Running the index migrations again must be a no-op (HasIndex guard),
	// not a "index already exists" failure.
	for _, version := range []int{2, 3, 12} {
		mig := findMigration(t, version)
		if err := mig.Up(db); err != nil {
			t.Errorf("migration %d Up() should be idempotent, got: %v", version, err)
		}
	}
}

func TestIndexMigrations_DownIdempotent(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)
	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	for _, version := range []int{2, 3, 12} {
		mig := findMigration(t, version)
		if err := mig.Down(db); err != nil {
			t.Fatalf("migration %d Down() error: %v", version, err)
		}
		// Second Down must be a no-op thanks to the HasIndex guard.
		if err := mig.Down(db); err != nil {
			t.Errorf("migration %d Down() should be idempotent, got: %v", version, err)
		}
	}
}

func TestArticleVersionMigration_UpgradesLegacyTable(t *testing.T) {
	db := newMigrationTestDB(t)
	if err := db.Exec("CREATE TABLE articles (id integer primary key, title text)").Error; err != nil {
		t.Fatalf("create legacy articles table: %v", err)
	}
	if err := db.Exec("INSERT INTO articles (id, title) VALUES (1, 'legacy')").Error; err != nil {
		t.Fatalf("insert legacy article: %v", err)
	}

	mig := findMigration(t, 7)
	if err := mig.Up(db); err != nil {
		t.Fatalf("migration 007 Up() error: %v", err)
	}
	if !db.Migrator().HasColumn(&models.Article{}, "Version") {
		t.Fatal("articles.version should exist after migration 007")
	}

	var version int
	if err := db.Table("articles").Select("version").Where("id = ?", 1).Scan(&version).Error; err != nil {
		t.Fatalf("read migrated version: %v", err)
	}
	if version != 1 {
		t.Fatalf("migrated article version = %d, want 1", version)
	}
}

func TestTenantMigration_UpCreatesTablesAndSeedsDefaultTenant(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)
	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	if !db.Migrator().HasTable(&models.Tenant{}) {
		t.Error("tenants table should exist after Up()")
	}
	if !db.Migrator().HasTable(&models.TenantMembership{}) {
		t.Error("tenant_memberships table should exist after Up()")
	}

	var count int64
	if err := db.Model(&models.Tenant{}).Where("slug = ?", "default").Count(&count).Error; err != nil {
		t.Fatalf("count default tenant: %v", err)
	}
	if count != 1 {
		t.Errorf("default tenant count = %d, want 1", count)
	}
}

func TestTenantMigration_BackfillsMembershipsForExistingUsers(t *testing.T) {
	db := newMigrationTestDB(t)

	// Simulate an upgraded database: apply 001-007, then create a role + user.
	for v := 1; v <= 7; v++ {
		if err := findMigration(t, v).Up(db); err != nil {
			t.Fatalf("migration %d Up() error: %v", v, err)
		}
	}
	role := models.Role{Name: "Reviewer", Slug: "reviewer"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	user := models.User{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "x",
		RoleID:   role.ID,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Now apply 008.
	if err := findMigration(t, 8).Up(db); err != nil {
		t.Fatalf("migration 008 Up() error: %v", err)
	}

	var ms []models.TenantMembership
	if err := db.Find(&ms).Error; err != nil {
		t.Fatalf("list memberships: %v", err)
	}
	if len(ms) != 1 {
		t.Fatalf("memberships after backfill = %d, want 1", len(ms))
	}
	if ms[0].TenantID != models.DefaultTenantID {
		t.Errorf("membership tenant_id = %d, want %d", ms[0].TenantID, models.DefaultTenantID)
	}
	if ms[0].UserID != user.ID {
		t.Errorf("membership user_id = %d, want %d", ms[0].UserID, user.ID)
	}
	if ms[0].RoleSlug != "reviewer" {
		t.Errorf("membership role_slug = %q, want %q", ms[0].RoleSlug, "reviewer")
	}
}

func TestTenantMigration011_NormalizesLegacyMembershipRoles(t *testing.T) {
	db := newMigrationTestDB(t)

	// Migration 008 historically copied arbitrary global role slugs. Build the
	// schema through that point, then simulate canonical and legacy memberships.
	for v := 1; v <= 8; v++ {
		if err := findMigration(t, v).Up(db); err != nil {
			t.Fatalf("migration %d Up() error: %v", v, err)
		}
	}
	tenant := models.Tenant{Name: "Other", Slug: "other", Status: models.TenantStatusActive}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	roles := []string{
		models.TenantRoleMember,
		models.TenantRoleEditor,
		models.TenantRoleAdmin,
		"author",
		"subscriber",
		"reviewer",
	}
	for i, role := range roles {
		membership := models.TenantMembership{
			TenantID: tenant.ID,
			UserID:   uint(i + 100),
			RoleSlug: role,
		}
		if err := db.Create(&membership).Error; err != nil {
			t.Fatalf("create membership %q: %v", role, err)
		}
	}
	legacyToken := models.APIToken{
		Name:        "legacy",
		Token:       "legacy-token-hash",
		IsActive:    true,
		CreatedByID: 100,
	}
	if err := db.Create(&legacyToken).Error; err != nil {
		t.Fatalf("create legacy token: %v", err)
	}

	if err := findMigration(t, 11).Up(db); err != nil {
		t.Fatalf("migration 011 Up() error: %v", err)
	}

	var memberships []models.TenantMembership
	if err := db.Where("tenant_id = ?", tenant.ID).Order("user_id ASC").Find(&memberships).Error; err != nil {
		t.Fatalf("list normalized memberships: %v", err)
	}
	want := []string{
		models.TenantRoleMember,
		models.TenantRoleEditor,
		models.TenantRoleAdmin,
		models.TenantRoleMember,
		models.TenantRoleMember,
		models.TenantRoleMember,
	}
	if len(memberships) != len(want) {
		t.Fatalf("memberships = %d, want %d", len(memberships), len(want))
	}
	for i := range want {
		if memberships[i].RoleSlug != want[i] {
			t.Errorf("membership %d role = %q, want %q", i, memberships[i].RoleSlug, want[i])
		}
	}
	var migratedToken models.APIToken
	if err := db.First(&migratedToken, legacyToken.ID).Error; err != nil {
		t.Fatalf("read migrated token: %v", err)
	}
	if migratedToken.TenantID == nil || *migratedToken.TenantID != models.DefaultTenantID {
		t.Fatalf("legacy token tenant = %v, want %d", migratedToken.TenantID, models.DefaultTenantID)
	}

	// Reapplying a data migration must remain harmless.
	if err := findMigration(t, 11).Up(db); err != nil {
		t.Fatalf("second migration 011 Up() error: %v", err)
	}
}

func TestTenantMigration_DownDropsTables(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)
	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}
	// 008 is not the newest migration anymore (013 through 009 sit above it),
	// so roll back six steps to remove the audit envelope, settings scope
	// uniqueness, role normalization, the embedding table, and both
	// multi-tenancy migrations.
	if err := m.Down(6); err != nil {
		t.Fatalf("Down(6) error: %v", err)
	}

	if db.Migrator().HasTable(&models.Tenant{}) {
		t.Error("tenants table should be dropped after Down(2)")
	}
	if db.Migrator().HasTable(&models.TenantMembership{}) {
		t.Error("tenant_memberships table should be dropped after Down(2)")
	}

	statuses, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	for _, s := range statuses {
		if (s.Version == 8 || s.Version == 9) && s.Applied {
			t.Errorf("migration %d should not be applied after Down(2)", s.Version)
		}
	}
}

func TestTenantMigration009_UpAddsColumnsAndIndexes(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)
	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	// NOT NULL tenant_id column + index on business tables.
	if !db.Migrator().HasColumn(&models.Article{}, "TenantID") {
		t.Error("articles.tenant_id should exist after Up()")
	}
	if !db.Migrator().HasIndex("articles", "idx_articles_tenant_id") {
		t.Error("idx_articles_tenant_id should exist after Up()")
	}
	// Tenant-scoped composite unique replaces the global one.
	if !db.Migrator().HasIndex("articles", "idx_articles_tenant_slug") {
		t.Error("idx_articles_tenant_slug should exist after Up()")
	}
	if db.Migrator().HasIndex("articles", "idx_articles_slug") {
		t.Error("idx_articles_slug should be replaced after Up()")
	}
	if !db.Migrator().HasIndex("content_entries", "idx_content_entries_tenant_document_id") {
		t.Error("idx_content_entries_tenant_document_id should exist after Up()")
	}
	if !db.Migrator().HasIndex("seo_settings", "idx_seo_tenant_entity") {
		t.Error("idx_seo_tenant_entity should exist after Up()")
	}
	if !db.Migrator().HasIndex("site_settings", "idx_site_settings_tenant_key") {
		t.Error("idx_site_settings_tenant_key should exist after Up()")
	}
	// Nullable tenant_id on global tables.
	if !db.Migrator().HasColumn(&models.ActivityLog{}, "TenantID") {
		t.Error("activity_logs.tenant_id should exist after Up()")
	}
	if !db.Migrator().HasColumn(&models.APIToken{}, "TenantID") {
		t.Error("api_tokens.tenant_id should exist after Up()")
	}
}

func TestTenantMigration009_BackfillsAndScopesUnique(t *testing.T) {
	db := newMigrationTestDB(t)

	// Upgraded database: 001-008, then a legacy article row.
	for v := 1; v <= 8; v++ {
		if err := findMigration(t, v).Up(db); err != nil {
			t.Fatalf("migration %d Up() error: %v", v, err)
		}
	}
	legacy := models.Article{Title: "Legacy", Slug: "legacy", AuthorID: 1}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("create legacy article: %v", err)
	}

	if err := findMigration(t, 9).Up(db); err != nil {
		t.Fatalf("migration 009 Up() error: %v", err)
	}

	// Existing rows backfilled into the default tenant.
	var backfilled int64
	if err := db.Model(&models.Article{}).Where("tenant_id = ?", 1).Count(&backfilled).Error; err != nil {
		t.Fatalf("count backfilled: %v", err)
	}
	if backfilled != 1 {
		t.Errorf("backfilled articles = %d, want 1", backfilled)
	}

	// Same slug is allowed across tenants...
	other := models.Article{Title: "Legacy other tenant", Slug: "legacy", AuthorID: 1}
	other.TenantID = 2
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("cross-tenant duplicate slug should be allowed: %v", err)
	}
	// ...but rejected within the same tenant.
	dup := models.Article{Title: "Duplicate", Slug: "legacy", AuthorID: 1}
	if err := db.Create(&dup).Error; err == nil {
		t.Fatal("same-tenant duplicate slug should be rejected by composite unique index")
	}
}

func TestSiteSettingMigration012_EnforcesGlobalAndTenantUniqueness(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)
	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}
	if !db.Migrator().HasIndex("site_settings", siteSettingGlobalKeyIndex) {
		t.Fatalf("%s should exist after migration 012", siteSettingGlobalKeyIndex)
	}

	global := models.SiteSetting{Key: "scope_unique", Value: "global", Type: "string", Group: "general"}
	if err := db.Create(&global).Error; err != nil {
		t.Fatalf("create global setting: %v", err)
	}
	duplicateGlobal := models.SiteSetting{Key: global.Key, Value: "duplicate", Type: "string", Group: "general"}
	if err := db.Create(&duplicateGlobal).Error; err == nil {
		t.Fatal("duplicate global setting key should be rejected")
	}

	tenantID := uint(7)
	override := models.SiteSetting{Key: global.Key, Value: "tenant", Type: "string", Group: "general", TenantID: &tenantID}
	if err := db.Create(&override).Error; err != nil {
		t.Fatalf("global and tenant override should coexist: %v", err)
	}
	duplicateOverride := models.SiteSetting{Key: global.Key, Value: "duplicate tenant", Type: "string", Group: "general", TenantID: &tenantID}
	if err := db.Create(&duplicateOverride).Error; err == nil {
		t.Fatal("duplicate key within one tenant should be rejected")
	}
}

func TestSiteSettingMigration012_RejectsExistingDuplicateGlobals(t *testing.T) {
	db := newMigrationTestDB(t)
	for version := 1; version <= 11; version++ {
		if err := findMigration(t, version).Up(db); err != nil {
			t.Fatalf("migration %03d Up() error: %v", version, err)
		}
	}

	for _, value := range []string{"first", "second"} {
		setting := models.SiteSetting{Key: "duplicate_global", Value: value, Type: "string", Group: "general"}
		if err := db.Create(&setting).Error; err != nil {
			t.Fatalf("create pre-012 duplicate global row: %v", err)
		}
	}

	err := findMigration(t, 12).Up(db)
	if err == nil || !strings.Contains(err.Error(), "deduplicate") {
		t.Fatalf("migration 012 error = %v, want actionable duplicate error", err)
	}
	if db.Migrator().HasIndex("site_settings", siteSettingGlobalKeyIndex) {
		t.Fatalf("%s must not be created when duplicates exist", siteSettingGlobalKeyIndex)
	}
}

func TestTenantMigration009_DownRejectsTenantScopedRowsBeforeDDL(t *testing.T) {
	db := newMigrationTestDB(t)
	for version := 1; version <= 9; version++ {
		if err := findMigration(t, version).Up(db); err != nil {
			t.Fatalf("migration %03d Up() error: %v", version, err)
		}
	}

	tenantID := uint(9)
	setting := models.SiteSetting{Key: "tenant_override", Value: "tenant", Type: "string", Group: "general", TenantID: &tenantID}
	if err := db.Create(&setting).Error; err != nil {
		t.Fatalf("create tenant override: %v", err)
	}

	err := findMigration(t, 9).Down(db)
	if err == nil || !strings.Contains(err.Error(), "scope would be lost") {
		t.Fatalf("migration 009 Down error = %v, want tenant-scope refusal", err)
	}
	if !db.Migrator().HasColumn(&models.SiteSetting{}, "TenantID") {
		t.Fatal("preflight refusal must leave site_settings.tenant_id intact")
	}
	if !db.Migrator().HasIndex("site_settings", "idx_site_settings_tenant_key") {
		t.Fatal("preflight refusal must happen before dropping tenant indexes")
	}
}

func TestTenantMigration009_DownRestoresSchema(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)
	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}
	// Roll back 013 (audit envelope no-op down), 012 (settings scope
	// uniqueness), 011 (role normalization), 010 (embeddings), and 009
	// (tenant_id columns) to verify 009's Down restores the pre-multi-tenancy
	// schema.
	if err := m.Down(5); err != nil {
		t.Fatalf("Down(5) error: %v", err)
	}

	if db.Migrator().HasColumn(&models.Article{}, "TenantID") {
		t.Error("articles.tenant_id should be dropped after Down(1)")
	}
	if db.Migrator().HasIndex("articles", "idx_articles_tenant_slug") {
		t.Error("idx_articles_tenant_slug should be dropped after Down(1)")
	}
	if !db.Migrator().HasIndex("articles", "idx_articles_slug") {
		t.Error("idx_articles_slug should be restored after Down(1)")
	}
}
