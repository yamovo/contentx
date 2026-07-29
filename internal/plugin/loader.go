// Package plugin provides the plugin interface, hook/filter registry, and
// lifecycle manager for ContentX's dynamic extension system.
//
// Built-in plugins are registered as Go types implementing the Plugin
// interface. The Manager dispatches action hooks (fire-and-forget side
// effects) and filter hooks (chained data transformation) to enabled
// plugins. Enable/disable state is persisted to the plugins table and
// mirrored at runtime.
package plugin

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// HookFunc is the universal hook handler. For action hooks the return value
// is ignored (only the error is inspected). For filter hooks the return
// value replaces the current value, which is passed in via args["value"].
type HookFunc func(args map[string]interface{}) (interface{}, error)

// HookType distinguishes action hooks (side-effect) from filter hooks
// (data transformation).
type HookType int

const (
	HookAction HookType = iota
	HookFilter
)

// HookRegistration binds a hook name to a handler with a priority.
type HookRegistration struct {
	Name     string   // e.g. "article.afterCreate"
	Type     HookType // Action or Filter
	Priority int      // lower runs first (default 0)
	Fn       HookFunc
}

// Plugin is the interface every plugin must implement. Built-in plugins
// implement this directly in Go; future dynamically-loaded plugins (.so
// or script-based) will adapt to the same interface.
type Plugin interface {
	Name() string
	Version() string
	Description() string
	Author() string
	// Init is called once at registration time. Config is the plugin's
	// persisted configuration (may be nil for a fresh install).
	Init(config map[string]interface{}) error
	// Hooks returns the hooks this plugin wants to register.
	Hooks() []HookRegistration
}

// PluginInfo is the read-only metadata returned by Manager.List.
type PluginInfo struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Author      string                 `json:"author"`
	Enabled     bool                   `json:"enabled"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

type hookEntry struct {
	pluginName string
	priority   int
	fn         HookFunc
}

// Manager manages plugin lifecycle and hook dispatch.
// SEC-7: the Manager holds a narrow StateStore (plugins table only) instead
// of a full *gorm.DB, so the plugin subsystem has no general database access.
type Manager struct {
	mu      sync.RWMutex
	store   StateStore // nil → in-memory only (no persistence)
	plugins map[string]Plugin
	configs map[string]map[string]interface{}
	enabled map[string]bool
	hooks   map[string][]hookEntry
}

// NewManager creates a new plugin manager. A nil db yields an in-memory
// manager (used by tests); otherwise persistence is confined to the
// plugins table via the GORM StateStore.
func NewManager(db *gorm.DB) *Manager {
	var store StateStore
	if db != nil {
		store = NewGormStateStore(db)
	}
	return NewManagerWithStore(store)
}

// NewManagerWithStore creates a plugin manager with an explicit StateStore,
// enabling tests to inject fakes.
func NewManagerWithStore(store StateStore) *Manager {
	return &Manager{
		store:   store,
		plugins: make(map[string]Plugin),
		configs: make(map[string]map[string]interface{}),
		enabled: make(map[string]bool),
		hooks:   make(map[string][]hookEntry),
	}
}

// ─── Registration ────────────────────────────────────────────────────────────

// Register registers a plugin. It calls Init with the plugin's persisted
// config (if any) and registers all declared hooks.
func (m *Manager) Register(p Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := p.Name()
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin %q already registered", name)
	}

	config, enabled := m.loadDBState(name)

	if err := p.Init(config); err != nil {
		return fmt.Errorf("plugin %q init failed: %w", name, err)
	}

	m.plugins[name] = p
	m.configs[name] = config
	m.enabled[name] = enabled

	for _, h := range p.Hooks() {
		m.hooks[h.Name] = append(m.hooks[h.Name], hookEntry{
			pluginName: name,
			priority:   h.Priority,
			fn:         h.Fn,
		})
	}
	m.sortHooks()

	slog.Info("plugin registered", "name", name, "version", p.Version(), "enabled", enabled)
	return nil
}

func (m *Manager) sortHooks() {
	for _, entries := range m.hooks {
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i].priority < entries[j].priority
		})
	}
}

// loadDBState fetches the plugin's config and enabled state from the store.
// Defaults to enabled=true, config=nil when the plugin has no row yet.
func (m *Manager) loadDBState(name string) (map[string]interface{}, bool) {
	if m.store == nil {
		return nil, true
	}
	config, enabled, found := m.store.Load(name)
	if !found {
		return nil, true
	}
	return config, enabled
}

// ─── Lifecycle ───────────────────────────────────────────────────────────────

// Enable activates a plugin at runtime and syncs the DB flag.
func (m *Manager) Enable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.plugins[name]; !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	m.enabled[name] = true
	if m.store != nil {
		_ = m.store.SetEnabled(name, true)
	}
	slog.Info("plugin enabled", "name", name)
	return nil
}

// Disable deactivates a plugin at runtime and syncs the DB flag.
func (m *Manager) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.plugins[name]; !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	m.enabled[name] = false
	if m.store != nil {
		_ = m.store.SetEnabled(name, false)
	}
	slog.Info("plugin disabled", "name", name)
	return nil
}

// IsEnabled reports whether a plugin is currently enabled.
func (m *Manager) IsEnabled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled[name]
}

// Reload re-initializes a plugin with fresh config from the DB.
func (m *Manager) Reload(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}

	config, enabled := m.loadDBState(name)
	if err := p.Init(config); err != nil {
		return fmt.Errorf("plugin %q reload failed: %w", name, err)
	}
	m.configs[name] = config
	m.enabled[name] = enabled
	slog.Info("plugin reloaded", "name", name)
	return nil
}

// ─── Queries ─────────────────────────────────────────────────────────────────

// Get returns a registered plugin by name.
func (m *Manager) Get(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[name]
	return p, ok
}

// List returns metadata for all registered plugins, sorted by name.
func (m *Manager) List() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]PluginInfo, 0, len(m.plugins))
	for name, p := range m.plugins {
		result = append(result, PluginInfo{
			Name:        p.Name(),
			Version:     p.Version(),
			Description: p.Description(),
			Author:      p.Author(),
			Enabled:     m.enabled[name],
			Config:      m.configs[name],
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// ListHookNames returns all registered hook names, sorted.
func (m *Manager) ListHookNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.hooks))
	for name := range m.hooks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ─── Hook dispatch ───────────────────────────────────────────────────────────

// ExecuteAction runs all action-hook handlers for the given hook name across
// enabled plugins. Errors are collected but do not stop execution. Returns
// all errors encountered (nil/empty if all succeeded).
func (m *Manager) ExecuteAction(hookName string, args map[string]interface{}) []error {
	entries := m.snapshotEntries(hookName)
	if entries == nil {
		return nil
	}
	var errs []error
	for _, entry := range entries {
		if !m.IsEnabled(entry.pluginName) {
			continue
		}
		if _, err := runHook(entry.pluginName, hookName, entry.fn, args); err != nil {
			slog.Error("action hook failed",
				"plugin", entry.pluginName, "hook", hookName, "error", err)
			errs = append(errs, fmt.Errorf("plugin %s: %w", entry.pluginName, err))
		}
	}
	return errs
}

// hookTimeout bounds each hook handler run (SEC-7): a slow or hung plugin
// must not stall the calling write path indefinitely. Variable (not const)
// so tests can shrink it.
var hookTimeout = 5 * time.Second

// runHook executes a single hook handler with a timeout and panic recovery.
// On timeout the handler goroutine keeps running (goroutines cannot be
// killed) but the caller proceeds; the leak is logged for diagnosis.
func runHook(pluginName, hookName string, fn HookFunc, args map[string]interface{}) (interface{}, error) {
	type hookResult struct {
		v   interface{}
		err error
	}
	ch := make(chan hookResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- hookResult{nil, fmt.Errorf("panic: %v", r)}
			}
		}()
		v, err := fn(args)
		ch <- hookResult{v, err}
	}()

	timer := time.NewTimer(hookTimeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-timer.C:
		slog.Warn("plugin hook timed out; handler goroutine abandoned",
			"plugin", pluginName, "hook", hookName, "timeout", hookTimeout)
		return nil, fmt.Errorf("hook %s timed out after %s", hookName, hookTimeout)
	}
}

// ApplyFilter chains all filter-hook handlers for the given hook name. The
// initial value is passed to the first handler via args["value"]; each
// handler's non-nil return value replaces the current value for the next.
func (m *Manager) ApplyFilter(hookName string, value interface{}, args map[string]interface{}) (interface{}, error) {
	entries := m.snapshotEntries(hookName)
	if entries == nil {
		return value, nil
	}
	if args == nil {
		args = make(map[string]interface{})
	}
	current := value
	for _, entry := range entries {
		if !m.IsEnabled(entry.pluginName) {
			continue
		}
		args["value"] = current
		result, err := runHook(entry.pluginName, hookName, entry.fn, args)
		if err != nil {
			return current, fmt.Errorf("plugin %s: %w", entry.pluginName, err)
		}
		if result != nil {
			current = result
		}
	}
	return current, nil
}

// snapshotEntries returns a copy of the hook entries list so callers can
// iterate without holding the read lock during handler execution.
func (m *Manager) snapshotEntries(hookName string) []hookEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entries, ok := m.hooks[hookName]
	if !ok {
		return nil
	}
	cp := make([]hookEntry, len(entries))
	copy(cp, entries)
	return cp
}

// ─── DB sync ─────────────────────────────────────────────────────────────────

// InitDB ensures all registered plugins have DB records. Plugins discovered
// in the filesystem but not yet registered should be pre-registered before
// calling this.
func (m *Manager) InitDB() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.store == nil {
		return nil
	}
	for name, p := range m.plugins {
		_ = m.store.Ensure(models.Plugin{
			Name:        name,
			Slug:        strings.ToLower(name),
			Description: p.Description(),
			Version:     p.Version(),
			Author:      p.Author(),
			IsEnabled:   m.enabled[name],
		})
	}
	return nil
}
