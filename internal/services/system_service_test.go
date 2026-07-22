package services

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
)

// ─── SystemService Tests ────────────────────────────────────────────────────

func TestSystemService_Health(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSystemService(db)

	healthy, err := svc.Health()
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	if !healthy {
		t.Fatal("expected healthy=true for in-memory DB")
	}
}

func TestSystemService_Info(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSystemService(db)

	info := svc.Info()
	if info["name"] != "ContentX" {
		t.Fatalf("expected name 'ContentX', got '%v'", info["name"])
	}
	if info["version"] == nil {
		t.Fatal("expected non-nil version")
	}
	if info["go_version"] == nil {
		t.Fatal("expected non-nil go_version")
	}
	if info["database"] == nil {
		t.Fatal("expected non-nil database")
	}
}

func TestSystemService_ActivityLog_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSystemService(db)

	logs, total, err := svc.ActivityLog(ActivityLogParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("activity log: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected 0 logs, got %d", total)
	}
	if len(logs) != 0 {
		t.Fatalf("expected empty logs, got %d", len(logs))
	}
}

func TestSystemService_ActivityLog_WithFilters(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "loguser", "admin")
	svc := NewSystemService(db)

	// Create some activity logs.
	db.Create(&models.ActivityLog{UserID: &user.ID, Action: "login", Entity: "user", EntityID: user.ID})
	db.Create(&models.ActivityLog{UserID: &user.ID, Action: "create", Entity: "article", EntityID: 1})
	db.Create(&models.ActivityLog{UserID: &user.ID, Action: "create", Entity: "article", EntityID: 2})

	// Filter by action.
	logs, total, err := svc.ActivityLog(ActivityLogParams{Page: 1, PageSize: 10, Action: "create"})
	if err != nil {
		t.Fatalf("activity log: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 logs with action=create, got %d", total)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}

	// Filter by entity.
	logs, total, _ = svc.ActivityLog(ActivityLogParams{Page: 1, PageSize: 10, Entity: "article"})
	if total != 2 {
		t.Fatalf("expected 2 logs with entity=article, got %d", total)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
}

// ─── PluginService Tests ────────────────────────────────────────────────────

func TestPluginService_List_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPluginService(db)

	plugins, err := svc.List()
	if err != nil {
		t.Fatalf("list plugins: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestPluginService_EnableDisable(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPluginService(db)

	plugin := models.Plugin{Name: "test-plugin", Slug: "test_plugin", IsEnabled: false}
	db.Create(&plugin)

	if err := svc.Enable(plugin.ID); err != nil {
		t.Fatalf("enable plugin: %v", err)
	}

	var refreshed models.Plugin
	db.First(&refreshed, plugin.ID)
	if !refreshed.IsEnabled {
		t.Fatal("expected plugin to be enabled")
	}

	if err := svc.Disable(plugin.ID); err != nil {
		t.Fatalf("disable plugin: %v", err)
	}

	db.First(&refreshed, plugin.ID)
	if refreshed.IsEnabled {
		t.Fatal("expected plugin to be disabled")
	}
}

func TestPluginService_UpdateConfig(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPluginService(db)

	plugin := models.Plugin{Name: "config-plugin", Slug: "config_plugin"}
	db.Create(&plugin)

	config := map[string]interface{}{"key": "value", "nested": map[string]interface{}{"a": 1}}
	if err := svc.UpdateConfig(plugin.ID, config); err != nil {
		t.Fatalf("update config: %v", err)
	}
}

// ─── ThemeService Tests ─────────────────────────────────────────────────────

func TestThemeService_List_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewThemeService(db)

	themes, err := svc.List()
	if err != nil {
		t.Fatalf("list themes: %v", err)
	}
	if len(themes) != 0 {
		t.Fatalf("expected 0 themes, got %d", len(themes))
	}
}

func TestThemeService_Activate(t *testing.T) {
	db := setupTestDB(t)
	svc := NewThemeService(db)

	theme1 := models.ThemeConfig{Name: "theme1", Slug: "theme1", IsActive: false}
	theme2 := models.ThemeConfig{Name: "theme2", Slug: "theme2", IsActive: true}
	db.Create(&theme1)
	db.Create(&theme2)

	if err := svc.Activate(theme1.ID); err != nil {
		t.Fatalf("activate theme: %v", err)
	}

	var active, inactive models.ThemeConfig
	db.First(&active, theme1.ID)
	db.First(&inactive, theme2.ID)

	if !active.IsActive {
		t.Fatal("theme1 should be active")
	}
	if inactive.IsActive {
		t.Fatal("theme2 should be deactivated")
	}
}
