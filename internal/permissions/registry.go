// Package permissions is the single source of truth for ContentX RBAC slugs.
//
// New code must use the constants in this package instead of spelling slugs
// inline. Legacy view/edit/manage slugs remain readable for one release through
// Canonicalize and Grants, but they are never emitted by public APIs.
package permissions

import (
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/yamovo/contentx/internal/models"
)

const Wildcard = "*"

const (
	ArticlesRead      = "articles.read"
	ArticlesCreate    = "articles.create"
	ArticlesUpdate    = "articles.update"
	ArticlesDelete    = "articles.delete"
	ArticlesUpdateAll = "articles.update_all"
	ArticlesDeleteAll = "articles.delete_all"
	ArticlesPublish   = "articles.publish"

	CommentsRead      = "comments.read"
	CommentsCreate    = "comments.create"
	CommentsUpdate    = "comments.update"
	CommentsDelete    = "comments.delete"
	CommentsUpdateAll = "comments.update_all"
	CommentsDeleteAll = "comments.delete_all"
	CommentsModerate  = "comments.moderate"

	MediaRead   = "media.read"
	MediaUpload = "media.upload"
	MediaUpdate = "media.update"
	MediaDelete = "media.delete"

	CategoriesRead   = "categories.read"
	CategoriesCreate = "categories.create"
	CategoriesUpdate = "categories.update"
	CategoriesDelete = "categories.delete"

	TagsRead   = "tags.read"
	TagsCreate = "tags.create"
	TagsUpdate = "tags.update"
	TagsDelete = "tags.delete"

	MenusRead   = "menus.read"
	MenusCreate = "menus.create"
	MenusUpdate = "menus.update"
	MenusDelete = "menus.delete"

	UsersRead   = "users.read"
	UsersCreate = "users.create"
	UsersUpdate = "users.update"
	UsersDelete = "users.delete"

	RolesRead   = "roles.read"
	RolesCreate = "roles.create"
	RolesUpdate = "roles.update"
	RolesDelete = "roles.delete"

	SettingsRead   = "settings.read"
	SettingsUpdate = "settings.update"

	AnalyticsRead = "analytics.read"

	SEORead   = "seo.read"
	SEOUpdate = "seo.update"

	PluginsRead   = "plugins.read"
	PluginsUpdate = "plugins.update"

	ThemesRead   = "themes.read"
	ThemesUpdate = "themes.update"

	SystemRead        = "system.read"
	SystemActivityLog = "system.activity_log"

	ContentRead    = "content.read"
	ContentCreate  = "content.create"
	ContentUpdate  = "content.update"
	ContentDelete  = "content.delete"
	ContentPublish = "content.publish"

	ContentTypesRead   = "content_types.read"
	ContentTypesCreate = "content_types.create"
	ContentTypesUpdate = "content_types.update"
	ContentTypesDelete = "content_types.delete"

	APITokensRead   = "api_tokens.read"
	APITokensCreate = "api_tokens.create"
	APITokensDelete = "api_tokens.delete"

	WebhooksRead   = "webhooks.read"
	WebhooksCreate = "webhooks.create"
	WebhooksDelete = "webhooks.delete"

	BackupsRead    = "backups.read"
	BackupsCreate  = "backups.create"
	BackupsRestore = "backups.restore"
	BackupsDelete  = "backups.delete"
)

// Definition describes one canonical permission persisted in the database and
// exposed to API clients.
type Definition struct {
	Slug        string
	Module      string
	Description string
}

var definitions = []Definition{
	{ArticlesRead, "articles", "Read articles and pages"},
	{ArticlesCreate, "articles", "Create article and page drafts"},
	{ArticlesUpdate, "articles", "Update own articles and pages"},
	{ArticlesDelete, "articles", "Delete own articles and pages"},
	{ArticlesUpdateAll, "articles", "Update articles and pages owned by any author"},
	{ArticlesDeleteAll, "articles", "Delete articles and pages owned by any author"},
	{ArticlesPublish, "articles", "Publish, unpublish, schedule, approve, and archive content"},

	{CommentsRead, "comments", "Read comments"},
	{CommentsCreate, "comments", "Create comments"},
	{CommentsUpdate, "comments", "Update own comments"},
	{CommentsDelete, "comments", "Delete own comments"},
	{CommentsUpdateAll, "comments", "Update comments owned by any user"},
	{CommentsDeleteAll, "comments", "Delete comments owned by any user"},
	{CommentsModerate, "comments", "Approve, spam, trash, and bulk moderate comments"},

	{MediaRead, "media", "Read media"},
	{MediaUpload, "media", "Upload media"},
	{MediaUpdate, "media", "Update media metadata"},
	{MediaDelete, "media", "Delete media"},

	{CategoriesRead, "categories", "Read categories"},
	{CategoriesCreate, "categories", "Create categories"},
	{CategoriesUpdate, "categories", "Update categories"},
	{CategoriesDelete, "categories", "Delete categories"},

	{TagsRead, "tags", "Read tags"},
	{TagsCreate, "tags", "Create tags"},
	{TagsUpdate, "tags", "Update tags"},
	{TagsDelete, "tags", "Delete tags"},

	{MenusRead, "menus", "Read navigation menus"},
	{MenusCreate, "menus", "Create navigation menus"},
	{MenusUpdate, "menus", "Update navigation menus"},
	{MenusDelete, "menus", "Delete navigation menus"},

	{UsersRead, "users", "Read users"},
	{UsersCreate, "users", "Create users"},
	{UsersUpdate, "users", "Update users"},
	{UsersDelete, "users", "Delete users"},

	{RolesRead, "roles", "Read roles and permissions"},
	{RolesCreate, "roles", "Create roles"},
	{RolesUpdate, "roles", "Update roles"},
	{RolesDelete, "roles", "Delete roles"},

	{SettingsRead, "settings", "Read settings"},
	{SettingsUpdate, "settings", "Update settings"},
	{AnalyticsRead, "analytics", "Read analytics"},
	{SEORead, "seo", "Read SEO settings"},
	{SEOUpdate, "seo", "Update SEO settings"},
	{PluginsRead, "plugins", "Read plugins"},
	{PluginsUpdate, "plugins", "Enable, disable, and configure plugins"},
	{ThemesRead, "themes", "Read themes"},
	{ThemesUpdate, "themes", "Activate and configure themes"},
	{SystemRead, "system", "Read system information"},
	{SystemActivityLog, "system", "Read the system activity log"},

	{ContentRead, "content", "Read structured content entries"},
	{ContentCreate, "content", "Create structured content entries"},
	{ContentUpdate, "content", "Update structured content entries"},
	{ContentDelete, "content", "Delete structured content entries"},
	{ContentPublish, "content", "Publish and unpublish structured content entries"},

	{ContentTypesRead, "content_types", "Read content type schemas"},
	{ContentTypesCreate, "content_types", "Create content type schemas"},
	{ContentTypesUpdate, "content_types", "Update content type schemas"},
	{ContentTypesDelete, "content_types", "Delete content type schemas"},

	{APITokensRead, "api_tokens", "Read API tokens"},
	{APITokensCreate, "api_tokens", "Create API tokens"},
	{APITokensDelete, "api_tokens", "Delete API tokens"},
	{WebhooksRead, "webhooks", "Read webhooks and delivery logs"},
	{WebhooksCreate, "webhooks", "Create webhooks"},
	{WebhooksDelete, "webhooks", "Delete webhooks"},
	{BackupsRead, "backups", "Read and download backups"},
	{BackupsCreate, "backups", "Create backups"},
	{BackupsRestore, "backups", "Restore backups"},
	{BackupsDelete, "backups", "Delete backups"},
}

var (
	definitionBySlug = buildDefinitionIndex()
	warnedAliases    sync.Map
)

func buildDefinitionIndex() map[string]Definition {
	out := make(map[string]Definition, len(definitions))
	for _, definition := range definitions {
		out[definition.Slug] = definition
	}
	return out
}

// Definitions returns a copy of the canonical registry in stable display order.
func Definitions() []Definition {
	out := make([]Definition, len(definitions))
	copy(out, definitions)
	return out
}

// IsCanonical reports whether slug is a registered canonical permission.
func IsCanonical(slug string) bool {
	_, ok := definitionBySlug[slug]
	return ok
}

// Canonicalize expands one permission into canonical slugs. A legacy manage
// permission expands to the module's CRUD-equivalent permissions. Unknown
// slugs return nil.
func Canonicalize(slug string) []string {
	slug = strings.TrimSpace(slug)
	if slug == Wildcard {
		return []string{Wildcard}
	}
	if IsCanonical(slug) {
		return []string{slug}
	}

	module, action, ok := strings.Cut(slug, ".")
	if !ok {
		return nil
	}
	var canonical []string
	switch action {
	case "view":
		canonical = existing(module + ".read")
	case "edit":
		canonical = existing(module + ".update")
	case "edit_all":
		canonical = existing(module + ".update_all")
	case "manage":
		canonical = moduleManagePermissions(module)
	default:
		return nil
	}
	if len(canonical) > 0 {
		warnLegacy(slug, canonical)
	}
	return canonical
}

func existing(slug string) []string {
	if IsCanonical(slug) {
		return []string{slug}
	}
	return nil
}

func moduleManagePermissions(module string) []string {
	candidates := []string{
		module + ".read",
		module + ".create",
		module + ".update",
		module + ".delete",
	}
	if module == "media" {
		candidates[1] = MediaUpload
	}
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if IsCanonical(candidate) {
			out = append(out, candidate)
		}
	}
	return out
}

func warnLegacy(alias string, canonical []string) {
	if _, loaded := warnedAliases.LoadOrStore(alias, struct{}{}); loaded {
		return
	}
	slog.Warn("deprecated permission alias used",
		"alias", alias,
		"canonical", strings.Join(canonical, ","),
	)
}

// CanonicalizeList converts a permission list to unique canonical slugs. The
// returned boolean is false when at least one unknown slug was supplied.
func CanonicalizeList(slugs []string) ([]string, bool) {
	seen := make(map[string]struct{}, len(slugs))
	valid := true
	for _, slug := range slugs {
		expanded := Canonicalize(slug)
		if len(expanded) == 0 {
			valid = false
			continue
		}
		for _, canonical := range expanded {
			seen[canonical] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for canonical := range seen {
		out = append(out, canonical)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i] == Wildcard {
			return true
		}
		if out[j] == Wildcard {
			return false
		}
		return out[i] < out[j]
	})
	return out, valid
}

// Grants reports whether a list of canonical or legacy permissions grants want.
func Grants(grants []string, want string) bool {
	for _, grant := range grants {
		if grant == Wildcard {
			return true
		}
		for _, canonical := range Canonicalize(grant) {
			if canonical == want {
				return true
			}
		}
	}
	return false
}

// Has reports whether a user has a canonical permission. Admin remains the
// explicit all-access system role; every other role is evaluated by grants.
func Has(user *models.User, want string) bool {
	if user == nil {
		return false
	}
	if user.Role.Slug == "admin" {
		return true
	}
	grants := make([]string, 0, len(user.Role.Permissions))
	for _, permission := range user.Role.Permissions {
		grants = append(grants, permission.Slug)
	}
	return Grants(grants, want)
}

// RoleDefinition defines one built-in role and its canonical permission set.
type RoleDefinition struct {
	Name        string
	Slug        string
	Description string
	IsDefault   bool
	Permissions []string
}

var roleDefinitions = []RoleDefinition{
	{
		Name:        "admin",
		Slug:        "admin",
		Description: "Administrator with full access",
		Permissions: allCanonicalSlugs(),
	},
	{
		Name:        "editor",
		Slug:        "editor",
		Description: "Can manage and publish content across authors",
		Permissions: []string{
			ArticlesRead, ArticlesCreate, ArticlesUpdate, ArticlesDelete,
			ArticlesUpdateAll, ArticlesDeleteAll, ArticlesPublish,
			CommentsRead, CommentsCreate, CommentsUpdate, CommentsDelete,
			CommentsUpdateAll, CommentsDeleteAll, CommentsModerate,
			MediaRead, MediaUpload, MediaUpdate, MediaDelete,
			CategoriesRead, CategoriesCreate, CategoriesUpdate, CategoriesDelete,
			TagsRead, TagsCreate, TagsUpdate, TagsDelete,
		},
	},
	{
		Name:        "author",
		Slug:        "author",
		Description: "Can create and manage own content",
		Permissions: []string{
			ArticlesRead, ArticlesCreate, ArticlesUpdate, ArticlesDelete,
			CommentsRead, CommentsCreate,
			MediaRead, MediaUpload,
			CategoriesRead, TagsRead,
		},
	},
	{
		Name:        "subscriber",
		Slug:        "subscriber",
		Description: "API-side personal access only",
		IsDefault:   true,
		Permissions: []string{},
	},
}

func allCanonicalSlugs() []string {
	out := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, definition.Slug)
	}
	return out
}

// DefaultRoles returns copies of the built-in role definitions.
func DefaultRoles() []RoleDefinition {
	out := make([]RoleDefinition, len(roleDefinitions))
	for i, role := range roleDefinitions {
		out[i] = role
		out[i].Permissions = append([]string(nil), role.Permissions...)
	}
	return out
}
