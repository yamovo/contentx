package repository

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

func TestSettingsRepository_TenantIsolation(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSettingsRepository(db)
	seo := NewSEORepository(db)
	menu := NewMenuRepository(db)

	// Tenant-1-only setting row. (Global+override coexistence requires the
	// composite unique index from migration 009 and is covered there; the
	// legacy AutoMigrate test schema keeps a single-column unique key.)
	tid := uint(1)
	if err := repo.Create(&models.SiteSetting{Key: "tenant_test_key", Value: "Tenant1", Group: "general", Type: "string", TenantID: &tid}); err != nil {
		t.Fatalf("create tenant setting: %v", err)
	}

	// Tenant 2 scope: global settings visible, tenant 1 overrides invisible.
	settings, err := repo.List("general", 2)
	if err != nil {
		t.Fatalf("List tenant 2: %v", err)
	}
	for _, s := range settings {
		if s.Key == "tenant_test_key" {
			t.Fatal("tenant 2 must not see tenant 1's override row")
		}
	}
	if _, err := repo.Get("tenant_test_key", 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("Get tenant 2 = %v, want gorm.ErrRecordNotFound", err)
	}
	// Tenant 1 scope sees its own row.
	settings1, err := repo.List("general", 1)
	if err != nil {
		t.Fatalf("List tenant 1: %v", err)
	}
	found := false
	for _, s := range settings1 {
		if s.Key == "tenant_test_key" {
			found = true
		}
	}
	if !found {
		t.Fatal("tenant 1 should see its own override row")
	}
	got, err := repo.Get("tenant_test_key", 1)
	if err != nil || got.Value != "Tenant1" {
		t.Fatalf("Get tenant 1 = %v/%v, want Tenant1/nil", got, err)
	}

	// SEO setting isolation.
	if err := seo.CreateSetting(&models.SEOSetting{TenantID: 1, EntityType: "article", EntityID: 1, Title: "T1"}); err != nil {
		t.Fatalf("create seo: %v", err)
	}
	if _, err := seo.GetSetting("article", 1, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("GetSetting cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}

	// Redirect isolation.
	if err := seo.CreateRedirect(&models.RedirectRule{TenantID: 1, FromPath: "/a", ToPath: "/b"}); err != nil {
		t.Fatalf("create redirect: %v", err)
	}
	rules, err := seo.ListRedirects(2)
	if err != nil || len(rules) != 0 {
		t.Fatalf("ListRedirects cross-tenant = %d/%v, want 0/nil", len(rules), err)
	}

	// Menu + item isolation.
	if err := menu.CreateMenu(&models.Menu{TenantID: 1, Name: "M1", Slug: "m1"}); err != nil {
		t.Fatalf("create menu: %v", err)
	}
	if _, err := menu.FindMenu(1, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("FindMenu cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestAuthRepository_FindSettingPrefersTenantOverride(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuthRepository(db)
	tenantID := uint(77)

	global := models.SiteSetting{
		Key: "auth_setting_precedence", Value: "global", Type: "string", Group: "general",
	}
	if err := db.Create(&global).Error; err != nil {
		t.Fatalf("create global setting: %v", err)
	}
	override := global
	override.ID = 0
	override.TenantID = &tenantID
	override.Value = "tenant"
	if err := db.Create(&override).Error; err != nil {
		t.Fatalf("create tenant override: %v", err)
	}

	got, err := repo.FindSetting(global.Key, tenantID)
	if err != nil {
		t.Fatalf("FindSetting tenant override: %v", err)
	}
	if got.TenantID == nil || *got.TenantID != tenantID || got.Value != "tenant" {
		t.Fatalf("FindSetting returned %+v, want tenant %d override", got, tenantID)
	}

	fallback, err := repo.FindSetting(global.Key, tenantID+1)
	if err != nil {
		t.Fatalf("FindSetting global fallback: %v", err)
	}
	if fallback.TenantID != nil || fallback.Value != global.Value {
		t.Fatalf("FindSetting fallback returned %+v, want global value %q", fallback, global.Value)
	}
}

func TestSettingsRepository_EnforcesGlobalAndTenantKeyUniqueness(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSettingsRepository(db)

	global := models.SiteSetting{Key: "repository_scope_unique", Value: "global", Type: "string", Group: "general"}
	if err := repo.Create(&global); err != nil {
		t.Fatalf("create global setting: %v", err)
	}
	duplicateGlobal := models.SiteSetting{Key: global.Key, Value: "duplicate", Type: "string", Group: "general"}
	if err := repo.Create(&duplicateGlobal); err == nil {
		t.Fatal("duplicate global setting key should be rejected")
	}

	tenantID := uint(55)
	override := models.SiteSetting{Key: global.Key, Value: "tenant", Type: "string", Group: "general", TenantID: &tenantID}
	if err := repo.UpsertTenantOverride(&override); err != nil {
		t.Fatalf("create tenant override: %v", err)
	}
	second := models.SiteSetting{Key: global.Key, Value: "tenant-updated", Type: "string", Group: "general", TenantID: &tenantID}
	if err := repo.UpsertTenantOverride(&second); err != nil {
		t.Fatalf("upsert existing tenant override: %v", err)
	}

	got, err := repo.Get(global.Key, tenantID)
	if err != nil {
		t.Fatalf("get tenant override: %v", err)
	}
	if got.Value != "tenant-updated" {
		t.Fatalf("tenant override value = %q, want tenant-updated", got.Value)
	}
	var overrideCount int64
	if err := db.Model(&models.SiteSetting{}).
		Where("tenant_id = ? AND key = ?", tenantID, global.Key).
		Count(&overrideCount).Error; err != nil {
		t.Fatalf("count tenant overrides: %v", err)
	}
	if overrideCount != 1 {
		t.Fatalf("tenant override count = %d, want 1", overrideCount)
	}
}
