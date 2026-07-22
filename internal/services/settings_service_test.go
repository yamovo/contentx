package services

import (
	"strings"
	"testing"

	"github.com/yamovo/contentx/internal/models"
)

// ─── SettingsService Tests ──────────────────────────────────────────────────

func TestSettingsService_List_All(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSettingsService(db)

	// Seed creates site_settings.
	settings, grouped, err := svc.List("")
	if err != nil {
		t.Fatalf("list settings: %v", err)
	}
	if len(settings) == 0 {
		t.Fatal("expected non-empty settings from seed")
	}
	if len(grouped) == 0 {
		t.Fatal("expected non-empty grouped map")
	}
}

func TestSettingsService_List_ByGroup(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSettingsService(db)

	settings, _, err := svc.List("general")
	if err != nil {
		t.Fatalf("list by group: %v", err)
	}
	for _, s := range settings {
		if s.Group != "general" {
			t.Fatalf("expected all settings to be in 'general' group, got '%s'", s.Group)
		}
	}
}

func TestSettingsService_Get(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSettingsService(db)

	setting, err := svc.Get("site_name")
	if err != nil {
		t.Fatalf("get setting: %v", err)
	}
	if setting.Value != "ContentX" {
		t.Fatalf("expected site_name 'ContentX', got '%s'", setting.Value)
	}
}

func TestSettingsService_Get_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSettingsService(db)

	_, err := svc.Get("nonexistent_key")
	if err == nil {
		t.Fatal("expected error for non-existent key")
	}
}

func TestSettingsService_Update_Existing(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSettingsService(db)

	err := svc.Update(map[string]interface{}{"site_name": "Updated Name"})
	if err != nil {
		t.Fatalf("update setting: %v", err)
	}

	setting, _ := svc.Get("site_name")
	if setting.Value != "Updated Name" {
		t.Fatalf("expected 'Updated Name', got '%s'", setting.Value)
	}
}

func TestSettingsService_Update_CreateNew(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSettingsService(db)

	err := svc.Update(map[string]interface{}{"custom_key": "custom_value"})
	if err != nil {
		t.Fatalf("update create new: %v", err)
	}

	setting, err := svc.Get("custom_key")
	if err != nil {
		t.Fatalf("get new setting: %v", err)
	}
	if setting.Value != "custom_value" {
		t.Fatalf("expected 'custom_value', got '%s'", setting.Value)
	}
}

func TestSettingsService_PublicSettings(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSettingsService(db)

	public, err := svc.PublicSettings()
	if err != nil {
		t.Fatalf("public settings: %v", err)
	}
	if len(public) == 0 {
		t.Fatal("expected non-empty public settings")
	}
	if _, ok := public["site_name"]; !ok {
		t.Fatal("expected 'site_name' in public settings")
	}
}

// ─── SEOService Tests ───────────────────────────────────────────────────────

func TestSEOService_GetSetting_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSEOService(db, "http://localhost:8080")

	_, err := svc.GetSetting("article", 999)
	if err == nil {
		t.Fatal("expected error for non-existent SEO setting")
	}
}

func TestSEOService_UpdateSetting_Create(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSEOService(db, "http://localhost:8080")

	err := svc.UpdateSetting("article", 1, SEOSettingRequest{
		Title:    "Test Title",
		Desc:     "Test Description",
		Keywords: "test,go",
	})
	if err != nil {
		t.Fatalf("update SEO setting: %v", err)
	}

	setting, err := svc.GetSetting("article", 1)
	if err != nil {
		t.Fatalf("get SEO setting: %v", err)
	}
	if setting.Title != "Test Title" {
		t.Fatalf("expected title 'Test Title', got '%s'", setting.Title)
	}
}

func TestSEOService_UpdateSetting_Update(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSEOService(db, "http://localhost:8080")

	svc.UpdateSetting("article", 1, SEOSettingRequest{Title: "Original"})
	err := svc.UpdateSetting("article", 1, SEOSettingRequest{Title: "Updated"})
	if err != nil {
		t.Fatalf("update SEO: %v", err)
	}

	setting, _ := svc.GetSetting("article", 1)
	if setting.Title != "Updated" {
		t.Fatalf("expected 'Updated', got '%s'", setting.Title)
	}
}

func TestSEOService_Sitemap(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSEOService(db, "http://localhost:8080")

	// Create a published article.
	user := createTestUser(t, db, "sitemapuser", "author")
	createTestArticle(t, db, user.ID, "Sitemap Test")

	sitemap, err := svc.Sitemap()
	if err != nil {
		t.Fatalf("sitemap: %v", err)
	}
	if !strings.Contains(sitemap, "<urlset") {
		t.Fatal("expected sitemap to contain <urlset>")
	}
	if !strings.Contains(sitemap, "Sitemap%20Test") && !strings.Contains(sitemap, "Sitemap Test") && !strings.Contains(sitemap, "sitemap-test") {
		t.Fatal("expected sitemap to contain article slug")
	}
}

func TestSEOService_RobotsTxt(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSEOService(db, "http://localhost:8080")

	robots := svc.RobotsTxt()
	if !strings.Contains(robots, "User-agent: *") {
		t.Fatal("expected robots.txt to contain 'User-agent: *'")
	}
	if !strings.Contains(robots, "Sitemap: http://localhost:8080/sitemap.xml") {
		t.Fatal("expected sitemap URL in robots.txt")
	}
}

func TestSEOService_Redirects(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSEOService(db, "http://localhost:8080")

	// Create redirect.
	rule, err := svc.CreateRedirect(CreateRedirectRequest{
		FromPath: "/old-page",
		ToPath:   "/new-page",
	})
	if err != nil {
		t.Fatalf("create redirect: %v", err)
	}
	if rule.StatusCode != 301 {
		t.Fatalf("expected default status 301, got %d", rule.StatusCode)
	}
	if !rule.IsActive {
		t.Fatal("expected redirect to be active")
	}

	// List redirects.
	rules, err := svc.ListRedirects()
	if err != nil {
		t.Fatalf("list redirects: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 redirect, got %d", len(rules))
	}

	// Delete redirect.
	if err := svc.DeleteRedirect(rule.ID); err != nil {
		t.Fatalf("delete redirect: %v", err)
	}

	rules, _ = svc.ListRedirects()
	if len(rules) != 0 {
		t.Fatal("expected 0 redirects after delete")
	}
}

// ─── MenuService Tests ──────────────────────────────────────────────────────

func TestMenuService_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMenuService(db)

	menu, err := svc.Create(CreateMenuRequest{
		Name:      "Main Menu",
		Slug:      "main",
		Locations: "header",
	})
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}
	if menu.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
}

func TestMenuService_List(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMenuService(db)

	svc.Create(CreateMenuRequest{Name: "Menu1", Slug: "m1"})
	svc.Create(CreateMenuRequest{Name: "Menu2", Slug: "m2"})

	menus, err := svc.List()
	if err != nil {
		t.Fatalf("list menus: %v", err)
	}
	if len(menus) != 2 {
		t.Fatalf("expected 2 menus, got %d", len(menus))
	}
}

func TestMenuService_Get_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMenuService(db)

	created, _ := svc.Create(CreateMenuRequest{Name: "GetMenu", Slug: "get"})

	menu, err := svc.Get(created.ID)
	if err != nil {
		t.Fatalf("get menu: %v", err)
	}
	if menu.Name != "GetMenu" {
		t.Fatalf("expected name 'GetMenu', got '%s'", menu.Name)
	}
}

func TestMenuService_Update(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMenuService(db)

	created, _ := svc.Create(CreateMenuRequest{Name: "Original", Slug: "orig"})

	err := svc.Update(created.ID, UpdateMenuRequest{Name: "Updated", Locations: "footer"})
	if err != nil {
		t.Fatalf("update menu: %v", err)
	}

	var refreshed models.Menu
	db.First(&refreshed, created.ID)
	if refreshed.Name != "Updated" {
		t.Fatalf("expected name 'Updated', got '%s'", refreshed.Name)
	}
}

func TestMenuService_Delete(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMenuService(db)

	created, _ := svc.Create(CreateMenuRequest{Name: "ToDelete", Slug: "del"})

	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("delete menu: %v", err)
	}

	var count int64
	db.Model(&models.Menu{}).Where("id = ?", created.ID).Count(&count)
	if count != 0 {
		t.Fatal("menu should be deleted")
	}
}

func TestMenuService_AddItem(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMenuService(db)

	menu, _ := svc.Create(CreateMenuRequest{Name: "Items", Slug: "items"})

	item, err := svc.AddItem(menu.ID, AddMenuItemRequest{
		Title: "Home",
		URL:   "/",
	})
	if err != nil {
		t.Fatalf("add item: %v", err)
	}
	if item.ID == 0 {
		t.Fatal("expected non-zero item ID")
	}
	if item.Target != "_self" {
		t.Fatalf("expected default target '_self', got '%s'", item.Target)
	}
	if item.SortOrder != 1 {
		t.Fatalf("expected sort_order 1, got %d", item.SortOrder)
	}

	// Add second item — should get sort_order 2.
	item2, _ := svc.AddItem(menu.ID, AddMenuItemRequest{Title: "About", URL: "/about"})
	if item2.SortOrder != 2 {
		t.Fatalf("expected sort_order 2, got %d", item2.SortOrder)
	}
}

func TestMenuService_UpdateItem(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMenuService(db)

	menu, _ := svc.Create(CreateMenuRequest{Name: "UpdItems", Slug: "upd_items"})
	item, _ := svc.AddItem(menu.ID, AddMenuItemRequest{Title: "Original", URL: "/"})

	newTitle := "Updated Item"
	err := svc.UpdateItem(item.ID, UpdateMenuItemRequest{Title: &newTitle})
	if err != nil {
		t.Fatalf("update item: %v", err)
	}

	var refreshed models.MenuItem
	db.First(&refreshed, item.ID)
	if refreshed.Title != newTitle {
		t.Fatalf("expected title '%s', got '%s'", newTitle, refreshed.Title)
	}
}

func TestMenuService_DeleteItem(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMenuService(db)

	menu, _ := svc.Create(CreateMenuRequest{Name: "DelItems", Slug: "del_items"})
	item, _ := svc.AddItem(menu.ID, AddMenuItemRequest{Title: "To Delete", URL: "/"})

	if err := svc.DeleteItem(item.ID); err != nil {
		t.Fatalf("delete item: %v", err)
	}

	var count int64
	db.Model(&models.MenuItem{}).Where("id = ?", item.ID).Count(&count)
	if count != 0 {
		t.Fatal("item should be deleted")
	}
}

func TestMenuService_ReorderItems(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMenuService(db)

	menu, _ := svc.Create(CreateMenuRequest{Name: "Reorder", Slug: "reorder"})
	item1, _ := svc.AddItem(menu.ID, AddMenuItemRequest{Title: "A", URL: "/"})
	item2, _ := svc.AddItem(menu.ID, AddMenuItemRequest{Title: "B", URL: "/"})

	err := svc.ReorderItems([]ReorderItem{
		{ID: item2.ID, SortOrder: 1},
		{ID: item1.ID, SortOrder: 2},
	})
	if err != nil {
		t.Fatalf("reorder: %v", err)
	}

	var refreshed1, refreshed2 models.MenuItem
	db.First(&refreshed1, item1.ID)
	db.First(&refreshed2, item2.ID)
	if refreshed1.SortOrder != 2 {
		t.Fatalf("expected item1 sort_order 2, got %d", refreshed1.SortOrder)
	}
	if refreshed2.SortOrder != 1 {
		t.Fatalf("expected item2 sort_order 1, got %d", refreshed2.SortOrder)
	}
}

// ─── AnalyticsService Tests ─────────────────────────────────────────────────

func TestAnalyticsService_Dashboard(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAnalyticsService(db)

	user := createTestUser(t, db, "dashuser", "author")
	createTestArticle(t, db, user.ID, "Dash Article")

	data, err := svc.Dashboard()
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if data.Stats.Articles != 1 {
		t.Fatalf("expected 1 article, got %d", data.Stats.Articles)
	}
	if data.Stats.Published != 1 {
		t.Fatalf("expected 1 published, got %d", data.Stats.Published)
	}
	if data.Stats.Users < 1 {
		t.Fatalf("expected at least 1 user, got %d", data.Stats.Users)
	}
	if len(data.RecentArticles) != 1 {
		t.Fatalf("expected 1 recent article, got %d", len(data.RecentArticles))
	}
}

func TestAnalyticsService_Dashboard_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAnalyticsService(db)

	data, err := svc.Dashboard()
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if data.Stats.Articles != 0 {
		t.Fatalf("expected 0 articles, got %d", data.Stats.Articles)
	}
}

func TestAnalyticsService_RecordView(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAnalyticsService(db)

	err := svc.RecordView(RecordViewRequest{
		Path:     "/test-page",
		Duration: 30,
	}, "127.0.0.1", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/115.0", "https://google.com", "session123")
	if err != nil {
		t.Fatalf("record view: %v", err)
	}

	var count int64
	db.Model(&models.PageView{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 page view, got %d", count)
	}

	var view models.PageView
	db.First(&view)
	if view.Device != "desktop" {
		t.Fatalf("expected device 'desktop', got '%s'", view.Device)
	}
	if view.Browser != "Firefox" {
		t.Fatalf("expected browser 'Firefox', got '%s'", view.Browser)
	}
}
