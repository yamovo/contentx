package middleware

import (
	"container/list"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/permissions"
	"gorm.io/gorm"
)

var (
	errTenantAccessDenied = errors.New("access denied to this tenant")
	errTenantNotFound     = errors.New("tenant not found")
	errTenantSuspended    = errors.New("tenant is suspended")
	errTenantRoleInvalid  = errors.New("tenant membership role is invalid")
	errTenantOverride     = errors.New("invalid tenant override")
)

const (
	// ContextKeyUser is the gin context key for the authenticated user.
	ContextKeyUser = "currentUser"
	// ContextKeyClaims is the gin context key for JWT claims.
	ContextKeyClaims = "claims"
	// ContextKeyTenant is the gin context key for the resolved request tenant
	// (RFC-001 §4.2). Always set by auth middlewares; falls back to the
	// default tenant for anonymous/public requests via GetCurrentTenant.
	ContextKeyTenant = "tenantId"
	// ContextKeyTenantRole is the canonical role of the current tenant
	// membership. It is empty only for a platform administrator accessing a
	// tenant without a membership.
	ContextKeyTenantRole = "tenantRole"
	// ContextKeyTenantOverride records that a platform administrator selected
	// the request tenant explicitly. ActivityLogger turns this into an audit
	// event, including for otherwise read-only requests.
	ContextKeyTenantOverride = "tenantOverride"
	// TenantOverrideHeader lets platform admins switch tenants per request.
	TenantOverrideHeader = "X-Tenant-ID"

	// authCacheTTL bounds how long a cached user record is considered fresh.
	// Changes to user status/role/permissions propagate within this window.
	authCacheTTL = 30 * time.Second
	// authCacheSize caps the number of cached users (LRU eviction).
	authCacheSize = 1024
)

// AuthUserCacheInvalidator coordinates the short-lived user caches owned by
// authentication middleware instances. Incrementing its generation makes all
// entries from the previous database state unusable on the next request.
type AuthUserCacheInvalidator struct {
	generation atomic.Uint64
}

func NewAuthUserCacheInvalidator() *AuthUserCacheInvalidator {
	return &AuthUserCacheInvalidator{}
}

// Invalidate advances the cache generation after a database restore.
func (i *AuthUserCacheInvalidator) Invalidate() {
	if i != nil {
		i.generation.Add(1)
	}
}

func (i *AuthUserCacheInvalidator) current() uint64 {
	if i == nil {
		return 0
	}
	return i.generation.Load()
}

// userCache is a short-TTL LRU cache for authenticated users. It reduces
// database load on the AuthMiddleware hot path by caching the user + role +
// permissions lookup. Entries expire after authCacheTTL so changes to user
// status/role/permissions propagate within that window. The cache is bounded
// to authCacheSize via LRU eviction. Safe for concurrent use.
type userCache struct {
	maxEntries  int
	ttl         time.Duration
	mu          sync.Mutex
	entries     map[uint]*list.Element
	ll          *list.List
	invalidator *AuthUserCacheInvalidator
}

type userCacheEntry struct {
	userID     uint
	user       *models.User
	expiresAt  time.Time
	generation uint64
}

func newUserCache(maxEntries int, ttl time.Duration, invalidators ...*AuthUserCacheInvalidator) *userCache {
	if maxEntries <= 0 {
		maxEntries = authCacheSize
	}
	if ttl <= 0 {
		ttl = authCacheTTL
	}
	invalidator := NewAuthUserCacheInvalidator()
	if len(invalidators) > 0 && invalidators[0] != nil {
		invalidator = invalidators[0]
	}
	return &userCache{
		maxEntries:  maxEntries,
		ttl:         ttl,
		entries:     make(map[uint]*list.Element),
		ll:          list.New(),
		invalidator: invalidator,
	}
}

// get returns the cached user for userID if present and unexpired.
func (c *userCache) get(userID uint) (*models.User, bool) {
	return c.getAtGeneration(userID, c.invalidator.current())
}

func (c *userCache) getAtGeneration(userID uint, generation uint64) (*models.User, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.invalidator.current() != generation {
		return nil, false
	}
	el, ok := c.entries[userID]
	if !ok {
		return nil, false
	}
	entry, _ := el.Value.(*userCacheEntry)
	if entry.generation != generation || time.Now().After(entry.expiresAt) {
		c.ll.Remove(el)
		delete(c.entries, userID)
		return nil, false
	}
	// Move to front (most recently used).
	c.ll.MoveToFront(el)
	return entry.user, true
}

// put stores a user in the cache. The caller must not mutate the stored user
// after handing it over.
func (c *userCache) put(user *models.User) {
	c.putAtGeneration(user, c.invalidator.current())
}

func (c *userCache) putAtGeneration(user *models.User, generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.invalidator.current() != generation {
		return
	}
	if el, ok := c.entries[user.ID]; ok {
		entry, _ := el.Value.(*userCacheEntry)
		entry.user = user
		entry.expiresAt = time.Now().Add(c.ttl)
		entry.generation = generation
		c.ll.MoveToFront(el)
		return
	}
	entry := &userCacheEntry{
		userID:     user.ID,
		user:       user,
		expiresAt:  time.Now().Add(c.ttl),
		generation: generation,
	}
	el := c.ll.PushFront(entry)
	c.entries[user.ID] = el
	if c.ll.Len() > c.maxEntries {
		if oldest := c.ll.Back(); oldest != nil {
			c.ll.Remove(oldest)
			oldestEntry, _ := oldest.Value.(*userCacheEntry)
			delete(c.entries, oldestEntry.userID)
		}
	}
}

// AuthMiddleware validates access JWTs, checks revocation, reloads the current
// user authorization state, and injects the authenticated principal. Tenant
// access is deliberately handled by TenantGuard so self-service and
// platform-scoped routes do not require a tenant membership.
func AuthMiddleware(jwtMgr *auth.JWTManager, db *gorm.DB, store auth.TokenStore, invalidators ...*AuthUserCacheInvalidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
			c.Abort()
			return
		}

		// Check if token has been revoked (always checked, never cached).
		if store != nil && store.IsRevoked(token) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token has been revoked"})
			c.Abort()
			return
		}

		claims, err := jwtMgr.ValidateAccessToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Authorization state is security-sensitive. Reload it for every request
		// so user disablement, role changes, and permission revocation take effect
		// immediately rather than after the former 30-second cache window.
		var user models.User
		if err := db.Preload("Role").Preload("Role.Permissions").Preload("TenantMemberships").
			Where("id = ?", claims.UserID).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		if !user.IsActive() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Account is disabled"})
			c.Abort()
			return
		}

		c.Set(ContextKeyUser, &user)
		c.Set(ContextKeyClaims, claims)
		c.Next()
	}
}

// TenantGuard resolves and validates the current request tenant after
// AuthMiddleware has established the user principal. It performs live tenant
// and membership checks on every tenant-scoped request.
func TenantGuard(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userValue, userOK := c.Get(ContextKeyUser)
		claimsValue, claimsOK := c.Get(ContextKeyClaims)
		user, validUser := userValue.(*models.User)
		claims, validClaims := claimsValue.(*auth.Claims)
		if !userOK || !claimsOK || !validUser || !validClaims || user == nil || claims == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		if err := setTenantContext(c, db, user, claims); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		c.Next()
	}
}

// OptionalAuthMiddleware tries to authenticate but doesn't block.
func OptionalAuthMiddleware(jwtMgr *auth.JWTManager, db *gorm.DB, store auth.TokenStore, invalidators ...*AuthUserCacheInvalidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.Next()
			return
		}

		// Skip revoked tokens silently.
		if store != nil && store.IsRevoked(token) {
			c.Next()
			return
		}

		claims, err := jwtMgr.ValidateAccessToken(token)
		if err != nil {
			c.Next()
			return
		}

		var user models.User
		if err := db.Preload("Role").Preload("Role.Permissions").Preload("TenantMemberships").
			Where("id = ?", claims.UserID).First(&user).Error; err != nil {
			c.Next()
			return
		}

		if user.IsActive() {
			if err := setTenantContext(c, db, &user, claims); err == nil {
				c.Set(ContextKeyUser, &user)
				c.Set(ContextKeyClaims, claims)
			}
		}
		c.Next()
	}
}

// RequirePermission checks a tenant-scoped permission. It remains as a
// compatibility alias while routes migrate to the explicit
// RequireTenantPermission name.
func RequirePermission(permissionSlug string) gin.HandlerFunc {
	return RequireTenantPermission(permissionSlug)
}

// RequireTenantPermission requires both the user's current global role grant
// and the current tenant membership role ceiling. Platform administrators may
// act within any active tenant but never derive platform authority from a
// tenant membership.
func RequireTenantPermission(permissionSlug string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Get(ContextKeyUser)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		u, ok := user.(*models.User)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid user type"})
			c.Abort()
			return
		}
		if !HasTenantPermission(c, u, permissionSlug) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "Insufficient permissions",
				"required": permissionSlug,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequirePlatformPermission protects deployment-wide resources. Tenant roles
// are deliberately ignored and can never grant or amplify these permissions.
func RequirePlatformPermission(permissionSlug string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Get(ContextKeyUser)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		u, ok := user.(*models.User)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid user type"})
			c.Abort()
			return
		}
		if !permissions.IsPlatformPermission(permissionSlug) || !permissions.Has(u, permissionSlug) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "Insufficient platform permissions",
				"required": permissionSlug,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireRole checks if the user has one of the specified roles.
func RequireRole(roleSlugs ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Get(ContextKeyUser)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		u, ok := user.(*models.User)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
			c.Abort()
			return
		}
		for _, slug := range roleSlugs {
			if u.Role.Slug == slug {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient role"})
		c.Abort()
	}
}

// RequireAdmin is a shortcut for admin-only routes.
func RequireAdmin() gin.HandlerFunc {
	return RequireRole("admin")
}

// RequirePlatformAdmin is the explicit name for deployment-wide admin-only
// routes. A tenant membership with role=admin never satisfies it.
func RequirePlatformAdmin() gin.HandlerFunc {
	return RequireRole("admin")
}

// RequireEditor checks for editor or admin role.
func RequireEditor() gin.HandlerFunc {
	return RequireRole("admin", "editor")
}

// hasPermission checks if a user has a specific permission.
func hasPermission(user *models.User, slug string) bool {
	return permissions.Has(user, slug)
}

// HasTenantPermission is the non-middleware form used by handlers that choose
// a permission after parsing the request (for example, article bulk actions).
func HasTenantPermission(c *gin.Context, user *models.User, permissionSlug string) bool {
	if user == nil || !permissions.IsTenantPermission(permissionSlug) || !hasPermission(user, permissionSlug) {
		return false
	}
	if user.IsAdmin() {
		return true
	}
	role, ok := GetCurrentTenantRole(c)
	return ok && permissions.TenantRoleGrants(role, permissionSlug)
}

// extractToken gets the JWT token from the Authorization header.
// Query parameter (?token=) is intentionally NOT supported to prevent
// tokens from leaking into access logs, browser history, and Referer headers.
func extractToken(c *gin.Context) string {
	bearer := c.GetHeader("Authorization")
	if len(bearer) > 7 && strings.HasPrefix(bearer, "Bearer ") {
		return bearer[7:]
	}
	return ""
}

// GetCurrentUser retrieves the authenticated user from context.
func GetCurrentUser(c *gin.Context) *models.User {
	user, exists := c.Get(ContextKeyUser)
	if !exists {
		return nil
	}
	u, ok := user.(*models.User)
	if !ok {
		return nil
	}
	return u
}

// GetCurrentTenant returns the tenant bound to the current request. Auth
// middlewares set it from JWT claims (or the X-Tenant-ID override for
// platform admins); anonymous/public requests fall back to the default
// tenant (RFC-001 §4.2).
func GetCurrentTenant(c *gin.Context) uint {
	if v, ok := c.Get(ContextKeyTenant); ok {
		if id, ok := v.(uint); ok && id > 0 {
			return id
		}
	}
	return models.DefaultTenantID
}

// GetCurrentTenantRole returns the canonical role for the membership selected
// by TenantGuard logic in AuthMiddleware. Platform administrators without a
// membership intentionally have no tenant role.
func GetCurrentTenantRole(c *gin.Context) (string, bool) {
	role, ok := c.Get(ContextKeyTenantRole)
	if !ok {
		return "", false
	}
	slug, ok := role.(string)
	return slug, ok && slug != ""
}

// setTenantContext resolves and stores the request tenant in the gin
// context, validating membership and tenant status. Resolution order:
// X-Tenant-ID header (platform admins only) → JWT claim TenantID → default
// tenant. Returns an error if the user lacks membership or the tenant is
// suspended.
func setTenantContext(c *gin.Context, db *gorm.DB, user *models.User, claims *auth.Claims) error {
	tenantID := models.DefaultTenantID
	if claims != nil && claims.TenantID > 0 {
		tenantID = claims.TenantID
	}
	if override := strings.TrimSpace(c.GetHeader(TenantOverrideHeader)); override != "" {
		if user == nil || !user.IsAdmin() {
			return errTenantAccessDenied
		}
		id, err := strconv.ParseUint(override, 10, 64)
		if err != nil || id == 0 {
			return errTenantOverride
		}
		tenantID = uint(id)
		c.Set(ContextKeyTenantOverride, true)
	}

	// Tenant existence and status are live authorization state, so validate on
	// every request rather than trusting JWT claims or the short-lived user
	// cache.
	if db == nil {
		return errTenantNotFound
	}
	var tenant models.Tenant
	if err := db.Select("id", "status").Where("id = ?", tenantID).First(&tenant).Error; err != nil {
		return errTenantNotFound
	}
	if tenant.Status != models.TenantStatusActive {
		return errTenantSuspended
	}

	if user == nil {
		return errTenantAccessDenied
	}
	if !user.IsAdmin() {
		var membership models.TenantMembership
		if err := db.Select("id", "role_slug").
			Where("tenant_id = ? AND user_id = ?", tenantID, user.ID).
			First(&membership).Error; err != nil {
			return errTenantAccessDenied
		}
		role, ok := permissions.NormalizeTenantRole(membership.RoleSlug)
		if !ok {
			return errTenantRoleInvalid
		}
		c.Set(ContextKeyTenantRole, role)
	} else {
		// A platform admin may switch to any active tenant without a membership.
		// When a membership exists, expose its normalized role for auditing only;
		// tenant permission checks still use the explicit platform-admin branch.
		var membership models.TenantMembership
		if err := db.Select("role_slug").
			Where("tenant_id = ? AND user_id = ?", tenantID, user.ID).
			First(&membership).Error; err == nil {
			if role, ok := permissions.NormalizeTenantRole(membership.RoleSlug); ok {
				c.Set(ContextKeyTenantRole, role)
			}
		}
	}

	c.Set(ContextKeyTenant, tenantID)
	return nil
}

// GetClaims retrieves the JWT claims from context.
func GetClaims(c *gin.Context) *auth.Claims {
	claims, exists := c.Get(ContextKeyClaims)
	if !exists {
		return nil
	}
	cl, ok := claims.(*auth.Claims)
	if !ok {
		return nil
	}
	return cl
}
