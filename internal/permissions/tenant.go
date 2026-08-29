package permissions

import (
	"sort"
	"strings"

	"github.com/yamovo/contentx/internal/models"
)

// Platform permissions operate on deployment-wide resources. They are never
// granted by a tenant membership role; only the user's global role can grant
// them.
var platformPermissionSlugs = map[string]struct{}{
	UsersRead: {}, UsersCreate: {}, UsersUpdate: {}, UsersDelete: {},
	RolesRead: {}, RolesCreate: {}, RolesUpdate: {}, RolesDelete: {},
	PluginsRead: {}, PluginsUpdate: {},
	ThemesRead: {}, ThemesUpdate: {},
	SystemRead:  {},
	BackupsRead: {}, BackupsCreate: {}, BackupsRestore: {}, BackupsDelete: {},
}

// tenantPermissionModules contains resources whose data and actions are bound
// to the request tenant. system.activity_log is handled separately because it
// is tenant-scoped for tenant administrators but system.read is platform-wide.
var tenantPermissionModules = map[string]struct{}{
	"articles":      {},
	"comments":      {},
	"media":         {},
	"categories":    {},
	"tags":          {},
	"menus":         {},
	"settings":      {},
	"analytics":     {},
	"seo":           {},
	"content":       {},
	"content_types": {},
	"api_tokens":    {},
	"webhooks":      {},
	"ai":            {},
}

// IsPlatformPermission reports whether slug protects a deployment-wide
// resource. Legacy aliases that expand to more than one permission are not
// accepted by authorization middleware; routes should use canonical slugs.
func IsPlatformPermission(slug string) bool {
	if !IsCanonical(slug) {
		return false
	}
	_, ok := platformPermissionSlugs[slug]
	return ok
}

// IsTenantPermission reports whether slug protects data or an action scoped to
// the current tenant.
func IsTenantPermission(slug string) bool {
	if !IsCanonical(slug) {
		return false
	}
	if slug == SystemActivityLog {
		return true
	}
	definition, ok := definitionBySlug[slug]
	if !ok {
		return false
	}
	_, ok = tenantPermissionModules[definition.Module]
	return ok
}

// NormalizeTenantRole maps the three supported membership roles and the
// legacy built-in roles copied by migration 008 to the canonical tenant role
// hierarchy. Unknown or empty roles are rejected so authorization fails closed.
func NormalizeTenantRole(roleSlug string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(roleSlug)) {
	case models.TenantRoleAdmin:
		return models.TenantRoleAdmin, true
	case models.TenantRoleEditor:
		return models.TenantRoleEditor, true
	case models.TenantRoleMember, "author", "subscriber":
		return models.TenantRoleMember, true
	default:
		return "", false
	}
}

// TenantRoleGrants reports whether a membership role permits a canonical,
// tenant-scoped permission. It is a ceiling only: callers must also check the
// user's global role permissions (and token permissions, when applicable).
func TenantRoleGrants(roleSlug, want string) bool {
	role, ok := NormalizeTenantRole(roleSlug)
	if !ok || !IsTenantPermission(want) {
		return false
	}

	switch role {
	case models.TenantRoleAdmin:
		return true
	case models.TenantRoleEditor:
		return builtInRoleGrants("editor", want)
	case models.TenantRoleMember:
		return builtInRoleGrants("author", want)
	default:
		return false
	}
}

func builtInRoleGrants(roleSlug, want string) bool {
	for _, role := range roleDefinitions {
		if role.Slug == roleSlug {
			return Grants(role.Permissions, want)
		}
	}
	return false
}

// EffectiveForTenant computes the permissions carried by a long-lived API
// token after intersecting them with the token owner's current global role and
// current tenant membership role. A wildcard expands only to canonical tenant
// permissions that survive both ceilings. The boolean is false for an unknown
// role or corrupt/unknown stored token permission, allowing resolvers to reject
// the principal instead of silently widening it.
func EffectiveForTenant(user *models.User, tokenGrants []string, tenantRole string) ([]string, bool) {
	if user == nil {
		return nil, false
	}
	if _, ok := NormalizeTenantRole(tenantRole); !ok {
		return nil, false
	}

	canonical, valid := CanonicalizeList(tokenGrants)
	if !valid {
		return nil, false
	}

	candidates := canonical
	if Grants(canonical, Wildcard) {
		candidates = make([]string, 0, len(definitions))
		for _, definition := range definitions {
			candidates = append(candidates, definition.Slug)
		}
	}

	effective := make([]string, 0, len(candidates))
	for _, permission := range candidates {
		if IsTenantPermission(permission) && Has(user, permission) && TenantRoleGrants(tenantRole, permission) {
			effective = append(effective, permission)
		}
	}
	sort.Strings(effective)
	return effective, true
}
