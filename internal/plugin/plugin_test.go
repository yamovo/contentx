package plugin

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ─── Test helpers ────────────────────────────────────────────────────────────

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// fakePlugin is a configurable test plugin for verifying hook dispatch.
type fakePlugin struct {
	name      string
	version   string
	desc      string
	author    string
	initErr   error
	hooks     []HookRegistration
	initCalls int
	mu        sync.Mutex
}

func (p *fakePlugin) Name() string        { return p.name }
func (p *fakePlugin) Version() string     { return p.version }
func (p *fakePlugin) Description() string { return p.desc }
func (p *fakePlugin) Author() string      { return p.author }
func (p *fakePlugin) Init(config map[string]interface{}) error {
	p.mu.Lock()
	p.initCalls++
	p.mu.Unlock()
	return p.initErr
}
func (p *fakePlugin) Hooks() []HookRegistration { return p.hooks }

// callTracker records invocations for assertion in tests.
type callTracker struct {
	mu     sync.Mutex
	calls  []string
	values []interface{}
}

func (t *callTracker) record(name string, val interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls = append(t.calls, name)
	t.values = append(t.values, val)
}

func (t *callTracker) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.calls)
}

// ─── Manager.Register ───────────────────────────────────────────────────────

func TestManager_Register_Success(t *testing.T) {
	m := NewManager(nil)
	p := &fakePlugin{name: "test", version: "1.0.0", desc: "A test", author: "tester"}

	if err := m.Register(p); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if p.initCalls != 1 {
		t.Errorf("Init calls = %d, want 1", p.initCalls)
	}

	got, ok := m.Get("test")
	if !ok {
		t.Fatal("Get: plugin not found after Register")
	}
	if got.Name() != "test" {
		t.Errorf("Get returned %q, want test", got.Name())
	}
	// Without a DB, plugins default to enabled.
	if !m.IsEnabled("test") {
		t.Error("plugin should be enabled by default")
	}
}

func TestManager_Register_Duplicate(t *testing.T) {
	m := NewManager(nil)
	p := &fakePlugin{name: "dup"}
	if err := m.Register(p); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	err := m.Register(p)
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Errorf("expected duplicate error, got %v", err)
	}
}

func TestManager_Register_InitError(t *testing.T) {
	m := NewManager(nil)
	p := &fakePlugin{name: "bad", initErr: errors.New("boom")}
	err := m.Register(p)
	if err == nil || !strings.Contains(err.Error(), "init failed") {
		t.Errorf("expected init error, got %v", err)
	}
}

// ─── Manager.Enable / Disable ───────────────────────────────────────────────

func TestManager_EnableDisable(t *testing.T) {
	m := NewManager(nil)
	m.Register(&fakePlugin{name: "p1"})

	if !m.IsEnabled("p1") {
		t.Error("should start enabled")
	}
	if err := m.Disable("p1"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if m.IsEnabled("p1") {
		t.Error("should be disabled")
	}
	if err := m.Enable("p1"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !m.IsEnabled("p1") {
		t.Error("should be re-enabled")
	}
}

func TestManager_Enable_NotFound(t *testing.T) {
	m := NewManager(nil)
	if err := m.Enable("nope"); err == nil {
		t.Error("expected error for unknown plugin")
	}
}

// ─── Manager.List ───────────────────────────────────────────────────────────

func TestManager_List_Sorted(t *testing.T) {
	m := NewManager(nil)
	m.Register(&fakePlugin{name: "charlie"})
	m.Register(&fakePlugin{name: "alpha"})
	m.Register(&fakePlugin{name: "bravo"})

	list := m.List()
	if len(list) != 3 {
		t.Fatalf("List count = %d, want 3", len(list))
	}
	if list[0].Name != "alpha" || list[1].Name != "bravo" || list[2].Name != "charlie" {
		t.Errorf("List not sorted: %s, %s, %s", list[0].Name, list[1].Name, list[2].Name)
	}
}

func TestManager_ListHookNames(t *testing.T) {
	m := NewManager(nil)
	m.Register(&fakePlugin{
		name: "p",
		hooks: []HookRegistration{
			{Name: "z.hook", Fn: func(a map[string]interface{}) (interface{}, error) { return nil, nil }},
			{Name: "a.hook", Fn: func(a map[string]interface{}) (interface{}, error) { return nil, nil }},
		},
	})

	names := m.ListHookNames()
	if len(names) != 2 || names[0] != "a.hook" || names[1] != "z.hook" {
		t.Errorf("ListHookNames = %v, want [a.hook z.hook]", names)
	}
}

// ─── ExecuteAction ──────────────────────────────────────────────────────────

func TestManager_ExecuteAction(t *testing.T) {
	tracker := &callTracker{}
	m := NewManager(nil)
	m.Register(&fakePlugin{
		name: "p",
		hooks: []HookRegistration{
			{Name: "ev.do", Type: HookAction, Fn: func(args map[string]interface{}) (interface{}, error) {
				tracker.record("do", args["x"])
				return nil, nil
			}},
		},
	})

	errs := m.ExecuteAction("ev.do", map[string]interface{}{"x": 42})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d", len(errs))
	}
	if tracker.count() != 1 {
		t.Errorf("handler called %d times, want 1", tracker.count())
	}
}

func TestManager_ExecuteAction_SkipsDisabled(t *testing.T) {
	tracker := &callTracker{}
	m := NewManager(nil)
	m.Register(&fakePlugin{
		name: "p",
		hooks: []HookRegistration{
			{Name: "ev.do", Type: HookAction, Fn: func(args map[string]interface{}) (interface{}, error) {
				tracker.record("do", nil)
				return nil, nil
			}},
		},
	})
	m.Disable("p")

	errs := m.ExecuteAction("ev.do", nil)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d", len(errs))
	}
	if tracker.count() != 0 {
		t.Errorf("handler should not be called on disabled plugin, got %d calls", tracker.count())
	}
}

func TestManager_ExecuteAction_NoHandlers(t *testing.T) {
	m := NewManager(nil)
	errs := m.ExecuteAction("nonexistent", nil)
	if errs != nil {
		t.Errorf("expected nil for no handlers, got %v", errs)
	}
}

func TestManager_ExecuteAction_CollectsErrors(t *testing.T) {
	m := NewManager(nil)
	m.Register(&fakePlugin{
		name: "p1",
		hooks: []HookRegistration{
			{Name: "ev.do", Fn: func(args map[string]interface{}) (interface{}, error) {
				return nil, errors.New("p1 err")
			}},
		},
	})
	m.Register(&fakePlugin{
		name: "p2",
		hooks: []HookRegistration{
			{Name: "ev.do", Fn: func(args map[string]interface{}) (interface{}, error) {
				return nil, nil // succeeds
			}},
		},
	})

	errs := m.ExecuteAction("ev.do", nil)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !strings.Contains(errs[0].Error(), "p1 err") {
		t.Errorf("error should mention p1: %v", errs[0])
	}
}

// ─── ApplyFilter ────────────────────────────────────────────────────────────

func TestManager_ApplyFilter_Chain(t *testing.T) {
	m := NewManager(nil)
	// Two filters: first appends "-A", second appends "-B".
	m.Register(&fakePlugin{
		name: "a",
		hooks: []HookRegistration{
			{Name: "f.transform", Type: HookFilter, Priority: 1, Fn: func(args map[string]interface{}) (interface{}, error) {
				return args["value"].(string) + "-A", nil
			}},
		},
	})
	m.Register(&fakePlugin{
		name: "b",
		hooks: []HookRegistration{
			{Name: "f.transform", Type: HookFilter, Priority: 2, Fn: func(args map[string]interface{}) (interface{}, error) {
				return args["value"].(string) + "-B", nil
			}},
		},
	})

	result, err := m.ApplyFilter("f.transform", "start", nil)
	if err != nil {
		t.Fatalf("ApplyFilter: %v", err)
	}
	if result != "start-A-B" {
		t.Errorf("result = %v, want start-A-B", result)
	}
}

func TestManager_ApplyFilter_NoHandlers(t *testing.T) {
	m := NewManager(nil)
	result, err := m.ApplyFilter("nonexistent", "value", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "value" {
		t.Errorf("result = %v, want value (unchanged)", result)
	}
}

func TestManager_ApplyFilter_SkipsDisabled(t *testing.T) {
	m := NewManager(nil)
	m.Register(&fakePlugin{
		name: "p",
		hooks: []HookRegistration{
			{Name: "f.transform", Type: HookFilter, Fn: func(args map[string]interface{}) (interface{}, error) {
				return "changed", nil
			}},
		},
	})
	m.Disable("p")

	result, err := m.ApplyFilter("f.transform", "original", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "original" {
		t.Errorf("result = %v, want original (disabled plugin should not transform)", result)
	}
}

func TestManager_ApplyFilter_ErrorStopsChain(t *testing.T) {
	m := NewManager(nil)
	m.Register(&fakePlugin{
		name: "p1",
		hooks: []HookRegistration{
			{Name: "f.transform", Type: HookFilter, Priority: 1, Fn: func(args map[string]interface{}) (interface{}, error) {
				return nil, errors.New("filter error")
			}},
		},
	})
	m.Register(&fakePlugin{
		name: "p2",
		hooks: []HookRegistration{
			{Name: "f.transform", Type: HookFilter, Priority: 2, Fn: func(args map[string]interface{}) (interface{}, error) {
				return "should-not-reach", nil
			}},
		},
	})

	result, err := m.ApplyFilter("f.transform", "start", nil)
	if err == nil || !strings.Contains(err.Error(), "filter error") {
		t.Errorf("expected filter error, got %v", err)
	}
	// On error, the last successful value is returned.
	if result != "start" {
		t.Errorf("result = %v, want start (last good value)", result)
	}
}

// ─── Priority ordering ──────────────────────────────────────────────────────

func TestManager_PriorityOrder(t *testing.T) {
	var order []string
	var mu sync.Mutex

	m := NewManager(nil)
	addTrack := func(name string, priority int) {
		m.Register(&fakePlugin{
			name: name,
			hooks: []HookRegistration{
				{Name: "ev", Type: HookAction, Priority: priority, Fn: func(args map[string]interface{}) (interface{}, error) {
					mu.Lock()
					order = append(order, name)
					mu.Unlock()
					return nil, nil
				}},
			},
		})
	}
	addTrack("late", 10)
	addTrack("early", 1)
	addTrack("middle", 5)

	m.ExecuteAction("ev", nil)

	expected := []string{"early", "middle", "late"}
	if len(order) != 3 || order[0] != expected[0] || order[1] != expected[1] || order[2] != expected[2] {
		t.Errorf("order = %v, want %v", order, expected)
	}
}

// ─── Reload ─────────────────────────────────────────────────────────────────

func TestManager_Reload(t *testing.T) {
	m := NewManager(nil)
	p := &fakePlugin{name: "p"}
	m.Register(p)

	if err := m.Reload("p"); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if p.initCalls != 2 {
		t.Errorf("Init calls after reload = %d, want 2", p.initCalls)
	}
}

func TestManager_Reload_NotFound(t *testing.T) {
	m := NewManager(nil)
	if err := m.Reload("nope"); err == nil {
		t.Error("expected error for reloading unknown plugin")
	}
}

// ─── DB integration ─────────────────────────────────────────────────────────

func TestManager_InitDB_CreatesRecords(t *testing.T) {
	db := setupTestDB(t)
	m := NewManager(db)
	m.Register(&fakePlugin{name: "db-test", version: "2.0.0", desc: "test plugin", author: "tester"})

	if err := m.InitDB(); err != nil {
		t.Fatalf("InitDB: %v", err)
	}

	// Verify the DB row was created.
	var count int64
	db.Table("plugins").Where("slug = ?", "db-test").Count(&count)
	if count != 1 {
		t.Errorf("plugins row count = %d, want 1", count)
	}
}

func TestManager_LoadDBState_ExistingRecord(t *testing.T) {
	db := setupTestDB(t)
	// Pre-create a DB record with IsEnabled=false and a config value.
	db.Create(&models.Plugin{
		Name:      "preexist",
		Slug:      "preexist",
		IsEnabled: false,
		Config:    map[string]interface{}{"key": "val"},
	})

	m := NewManager(db)
	m.Register(&fakePlugin{name: "preexist"})

	// The plugin should start disabled because the DB record says so.
	if m.IsEnabled("preexist") {
		t.Error("plugin should be disabled per DB record")
	}

	// Enabling should flip both runtime and DB.
	if err := m.Enable("preexist"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !m.IsEnabled("preexist") {
		t.Error("plugin should be enabled after Enable")
	}

	var dbPlugin models.Plugin
	db.Where("slug = ?", "preexist").First(&dbPlugin)
	if !dbPlugin.IsEnabled {
		t.Error("DB record should reflect enabled=true")
	}
}

// ─── WordCountPlugin (built-in) ─────────────────────────────────────────────

func TestWordCountPlugin_Interface(t *testing.T) {
	p := NewWordCountPlugin()
	if p.Name() != "word-count" {
		t.Errorf("Name = %q, want word-count", p.Name())
	}
	if p.Version() != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", p.Version())
	}
	if p.Description() == "" {
		t.Error("Description should not be empty")
	}
	if err := p.Init(nil); err != nil {
		t.Errorf("Init: %v", err)
	}
	hooks := p.Hooks()
	if len(hooks) != 3 {
		t.Errorf("Hooks count = %d, want 3", len(hooks))
	}
}

func TestWordCountPlugin_FilterContent(t *testing.T) {
	p := NewWordCountPlugin()
	p.Init(nil)

	// Find the filter hook.
	var filterFn HookFunc
	for _, h := range p.Hooks() {
		if h.Name == "article.filterContent" {
			filterFn = h.Fn
		}
	}
	if filterFn == nil {
		t.Fatal("article.filterContent hook not found")
	}

	result, err := filterFn(map[string]interface{}{
		"value": "  hello   world  ",
	})
	if err != nil {
		t.Fatalf("filterContent: %v", err)
	}
	if result != "hello world" {
		t.Errorf("filterContent result = %q, want %q", result, "hello world")
	}
}

func TestWordCountPlugin_AfterCreate(t *testing.T) {
	p := NewWordCountPlugin()
	p.Init(nil)

	var actionFn HookFunc
	for _, h := range p.Hooks() {
		if h.Name == "article.afterCreate" {
			actionFn = h.Fn
		}
	}
	if actionFn == nil {
		t.Fatal("article.afterCreate hook not found")
	}

	// Should not error.
	_, err := actionFn(map[string]interface{}{
		"title":   "Test",
		"content": "one two three",
	})
	if err != nil {
		t.Errorf("afterCreate: %v", err)
	}
}
