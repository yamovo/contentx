package services

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/cache"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// ============================================================
// 构造器测试 — 覆盖所有 NewXxxServiceWithRepo 入口
// 构造器仅做字段赋值，传 nil repo 即可（不会调用任何方法）。
// ============================================================

func TestConstructors_WithRepo(t *testing.T) {
	t.Run("Category", func(t *testing.T) {
		s := NewCategoryServiceWithRepo(nil)
		if s == nil {
			t.Fatal("expected non-nil CategoryService")
		}
	})
	t.Run("ContentType", func(t *testing.T) {
		s := NewContentTypeServiceWithRepo(nil, nil)
		if s == nil {
			t.Fatal("expected non-nil ContentTypeService")
		}
	})
	t.Run("Settings", func(t *testing.T) {
		s := NewSettingsServiceWithRepo(nil)
		if s == nil {
			t.Fatal("expected non-nil SettingsService")
		}
	})
	t.Run("SEO", func(t *testing.T) {
		s := NewSEOServiceWithRepo(nil, "https://example.com")
		if s == nil {
			t.Fatal("expected non-nil SEOService")
		}
	})
	t.Run("Menu", func(t *testing.T) {
		s := NewMenuServiceWithRepo(nil)
		if s == nil {
			t.Fatal("expected non-nil MenuService")
		}
	})
	t.Run("Plugin", func(t *testing.T) {
		s := NewPluginServiceWithRepo(nil)
		if s == nil {
			t.Fatal("expected non-nil PluginService")
		}
	})
	t.Run("Theme", func(t *testing.T) {
		s := NewThemeServiceWithRepo(nil)
		if s == nil {
			t.Fatal("expected non-nil ThemeService")
		}
	})
	t.Run("System", func(t *testing.T) {
		s := NewSystemServiceWithRepo(nil)
		if s == nil {
			t.Fatal("expected non-nil SystemService")
		}
	})
	t.Run("Tag", func(t *testing.T) {
		s := NewTagServiceWithRepo(nil)
		if s == nil {
			t.Fatal("expected non-nil TagService")
		}
	})
	t.Run("Token", func(t *testing.T) {
		s := NewTokenServiceWithRepo(nil)
		if s == nil {
			t.Fatal("expected non-nil TokenService")
		}
	})
	t.Run("User", func(t *testing.T) {
		s := NewUserServiceWithRepo(nil)
		if s == nil {
			t.Fatal("expected non-nil UserService")
		}
	})
	t.Run("Role", func(t *testing.T) {
		s := NewRoleServiceWithRepo(nil)
		if s == nil {
			t.Fatal("expected non-nil RoleService")
		}
	})
}

// ============================================================
// ContentTypeService.WithCache
// ============================================================

func TestContentTypeService_WithCache_DefaultTTL(t *testing.T) {
	typeRepo := &MockContentTypeRepository{}
	s := NewContentTypeServiceWithRepo(typeRepo, nil)

	c := &MockCacheDriver{}
	returned := s.WithCache(c, 0) // ttl <= 0 → 默认 10 分钟
	if returned != s {
		t.Error("WithCache should return the same service for chaining")
	}
	if s.cache != c {
		t.Error("cache not set")
	}
	if s.cacheTTL != 10*time.Minute {
		t.Errorf("cacheTTL = %v, want 10m", s.cacheTTL)
	}
}

func TestContentTypeService_WithCache_CustomTTL(t *testing.T) {
	typeRepo := &MockContentTypeRepository{}
	s := NewContentTypeServiceWithRepo(typeRepo, nil)

	c := &MockCacheDriver{}
	s.WithCache(c, 5*time.Minute)
	if s.cacheTTL != 5*time.Minute {
		t.Errorf("cacheTTL = %v, want 5m", s.cacheTTL)
	}
}

// ============================================================
// ContentTypeService.invalidateType (private, but same package)
// ============================================================

func TestContentTypeService_InvalidateType_WithCache(t *testing.T) {
	typeRepo := &MockContentTypeRepository{}
	c := &MockCacheDriver{}
	s := NewContentTypeServiceWithRepo(typeRepo, nil)
	s.WithCache(c, 5*time.Minute)

	s.invalidateType("product")
	if len(c.DeleteCalls) != 1 {
		t.Fatalf("expected 1 Delete call, got %d", len(c.DeleteCalls))
	}
	if c.DeleteCalls[0] != "contenttype:product" {
		t.Errorf("Delete key = %q, want contenttype:product", c.DeleteCalls[0])
	}
}

func TestContentTypeService_InvalidateType_NoCache(t *testing.T) {
	typeRepo := &MockContentTypeRepository{}
	s := NewContentTypeServiceWithRepo(typeRepo, nil)
	// 无 cache → 不应 panic
	s.invalidateType("product")
}

// ============================================================
// ContentTypeService.GetEntriesByUID
// ============================================================

func TestContentTypeService_GetEntriesByUID_Success(t *testing.T) {
	ct := &models.ContentType{ID: 7, UID: "product", Name: "Product"}
	typeRepo := &MockContentTypeRepository{ContentType: ct}
	entryRepo := &MockContentEntryRepository{
		Entries: []models.ContentEntry{
			{ID: 1, ContentTypeID: 7, DocumentID: "doc-1"},
			{ID: 2, ContentTypeID: 7, DocumentID: "doc-2"},
		},
	}
	s := NewContentTypeServiceWithRepo(typeRepo, entryRepo)

	entries, err := s.GetEntriesByUID("product", []uint{1, 2})
	if err != nil {
		t.Fatalf("GetEntriesByUID failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}
}

func TestContentTypeService_GetEntriesByUID_NotFound(t *testing.T) {
	typeRepo := &MockContentTypeRepository{
		FindByUIDErr: gorm.ErrRecordNotFound,
	}
	entryRepo := &MockContentEntryRepository{}
	s := NewContentTypeServiceWithRepo(typeRepo, entryRepo)

	_, err := s.GetEntriesByUID("nonexistent", []uint{1})
	if err == nil {
		t.Fatal("expected error for not found content type")
	}
}

func TestContentTypeService_GetEntriesByUID_RepoError(t *testing.T) {
	typeRepo := &MockContentTypeRepository{ContentType: &models.ContentType{ID: 1}}
	entryRepo := &MockContentEntryRepository{FindByIDsErr: gorm.ErrInvalidDB}
	s := NewContentTypeServiceWithRepo(typeRepo, entryRepo)

	_, err := s.GetEntriesByUID("product", []uint{1})
	if err == nil {
		t.Fatal("expected error from FindByIDs")
	}
}

// ============================================================
// ContentTypeService.GetContentType with cache
// ============================================================

func TestContentTypeService_GetContentType_CacheHit(t *testing.T) {
	ct := &models.ContentType{ID: 1, UID: "product", Name: "Product"}
	ctJSON, _ := json.Marshal(ct)
	typeRepo := &MockContentTypeRepository{ContentType: ct}
	c := &MockCacheDriver{
		Data: map[string][]byte{
			"contenttype:product": ctJSON,
		},
	}
	s := NewContentTypeServiceWithRepo(typeRepo, nil)
	s.WithCache(c, 5*time.Minute)

	got, err := s.GetContentType("product")
	if err != nil {
		t.Fatalf("GetContentType failed: %v", err)
	}
	if got.UID != "product" {
		t.Errorf("UID = %q, want product", got.UID)
	}
	// 缓存命中时不应调用 repo
	if typeRepo.CreatedCT != nil {
		t.Error("repo should not be called on cache hit")
	}
}

func TestContentTypeService_GetContentType_CacheMiss(t *testing.T) {
	ct := &models.ContentType{ID: 1, UID: "product", Name: "Product"}
	typeRepo := &MockContentTypeRepository{ContentType: ct}
	c := &MockCacheDriver{} // 空 Data → miss
	s := NewContentTypeServiceWithRepo(typeRepo, nil)
	s.WithCache(c, 5*time.Minute)

	got, err := s.GetContentType("product")
	if err != nil {
		t.Fatalf("GetContentType failed: %v", err)
	}
	if got.UID != "product" {
		t.Errorf("UID = %q, want product", got.UID)
	}
	// 缓存未命中 → 应写入缓存
	if len(c.SetCalls) != 1 {
		t.Fatalf("expected 1 Set call, got %d", len(c.SetCalls))
	}
	if c.SetCalls[0].Key != "contenttype:product" {
		t.Errorf("Set key = %q, want contenttype:product", c.SetCalls[0].Key)
	}
}

func TestContentTypeService_GetContentType_NotFound(t *testing.T) {
	typeRepo := &MockContentTypeRepository{FindByUIDErr: gorm.ErrRecordNotFound}
	s := NewContentTypeServiceWithRepo(typeRepo, nil)

	_, err := s.GetContentType("missing")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

// ============================================================
// ThemeService.UpdateConfig
// ============================================================

func TestThemeService_UpdateConfig_Success(t *testing.T) {
	original := &models.ThemeConfig{
		BaseModel: models.BaseModel{ID: 3},
		Name:      "Default",
		Config:    map[string]interface{}{"old": "value"},
	}
	repo := &MockThemeRepository{Theme: original}
	s := NewThemeServiceWithRepo(repo)

	newConfig := map[string]interface{}{"color": "blue", "layout": "grid"}
	err := s.UpdateConfig(3, newConfig)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	// 应调用 Save
	if repo.SavedTheme == nil {
		t.Fatal("expected Save to be called")
	}
	if repo.SavedTheme.Config["color"] != "blue" {
		t.Errorf("Config[color] = %v, want blue", repo.SavedTheme.Config["color"])
	}
	if repo.SavedTheme.Config["layout"] != "grid" {
		t.Errorf("Config[layout] = %v, want grid", repo.SavedTheme.Config["layout"])
	}
}

func TestThemeService_UpdateConfig_NotFound(t *testing.T) {
	repo := &MockThemeRepository{FindByIDErr: gorm.ErrRecordNotFound}
	s := NewThemeServiceWithRepo(repo)

	err := s.UpdateConfig(999, map[string]interface{}{"x": 1})
	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestThemeService_UpdateConfig_SaveError(t *testing.T) {
	repo := &MockThemeRepository{
		Theme:   &models.ThemeConfig{BaseModel: models.BaseModel{ID: 1}},
		SaveErr: gorm.ErrInvalidDB,
	}
	s := NewThemeServiceWithRepo(repo)

	err := s.UpdateConfig(1, map[string]interface{}{"x": 1})
	if err == nil {
		t.Fatal("expected error from Save")
	}
}

// ============================================================
// ThemeService.Activate (boost from 60%)
// ============================================================

func TestThemeService_Activate_Success(t *testing.T) {
	repo := &MockThemeRepository{
		Theme: &models.ThemeConfig{BaseModel: models.BaseModel{ID: 5}, Name: "MyTheme"},
	}
	s := NewThemeServiceWithRepo(repo)

	err := s.Activate(5)
	if err != nil {
		t.Fatalf("Activate failed: %v", err)
	}
	if repo.DeactivatedID != 5 {
		t.Errorf("DeactivateAllExcept ID = %d, want 5", repo.DeactivatedID)
	}
	if repo.UpdatedActiveID != 5 || !repo.UpdatedActiveVal {
		t.Errorf("UpdateActive = (id=%d, val=%v), want (5, true)", repo.UpdatedActiveID, repo.UpdatedActiveVal)
	}
}

func TestThemeService_Activate_NotFound(t *testing.T) {
	repo := &MockThemeRepository{FindByIDErr: gorm.ErrRecordNotFound}
	s := NewThemeServiceWithRepo(repo)

	err := s.Activate(404)
	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestThemeService_Activate_DeactivateError(t *testing.T) {
	repo := &MockThemeRepository{
		Theme:            &models.ThemeConfig{BaseModel: models.BaseModel{ID: 5}},
		DeactivateAllErr: gorm.ErrInvalidDB,
	}
	s := NewThemeServiceWithRepo(repo)

	err := s.Activate(5)
	if err == nil {
		t.Fatal("expected error from DeactivateAllExcept")
	}
}

// ============================================================
// cache.Driver compile-time check
// ============================================================

var _ cache.Driver = (*MockCacheDriver)(nil)
