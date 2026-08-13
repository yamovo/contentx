package middleware

import (
	"container/list"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
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

// AuthMiddleware validates JWT tokens, checks revocation, and injects user into context.
func AuthMiddleware(jwtMgr *auth.JWTManager, db *gorm.DB, store auth.TokenStore, invalidators ...*AuthUserCacheInvalidator) gin.HandlerFunc {
	cache := newUserCache(authCacheSize, authCacheTTL, invalidators...)
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

		claims, err := jwtMgr.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Try the LRU cache first to avoid the DB round-trip on hot paths.
		// Revocation is still enforced above on every request, and cached
		// entries expire after authCacheTTL so status/role changes propagate.
		cacheGeneration := cache.invalidator.current()
		if user, ok := cache.getAtGeneration(claims.UserID, cacheGeneration); ok {
			if !user.IsActive() {
				c.JSON(http.StatusForbidden, gin.H{"error": "Account is disabled"})
				c.Abort()
				return
			}
			c.Set(ContextKeyUser, user)
			c.Set(ContextKeyClaims, claims)
			setTenantContext(c, user, claims)
			c.Next()
			return
		}

		// Cache miss: load user from database.
		var user models.User
		if err := db.Preload("Role").Preload("Role.Permissions").
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

		cache.putAtGeneration(&user, cacheGeneration)

		c.Set(ContextKeyUser, &user)
		c.Set(ContextKeyClaims, claims)
		setTenantContext(c, &user, claims)
		c.Next()
	}
}

// OptionalAuthMiddleware tries to authenticate but doesn't block.
func OptionalAuthMiddleware(jwtMgr *auth.JWTManager, db *gorm.DB, store auth.TokenStore, invalidators ...*AuthUserCacheInvalidator) gin.HandlerFunc {
	cache := newUserCache(authCacheSize, authCacheTTL, invalidators...)
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

		claims, err := jwtMgr.ValidateToken(token)
		if err != nil {
			c.Next()
			return
		}

		// LRU cache fast path.
		cacheGeneration := cache.invalidator.current()
		if user, ok := cache.getAtGeneration(claims.UserID, cacheGeneration); ok {
			if user.IsActive() {
				c.Set(ContextKeyUser, user)
				c.Set(ContextKeyClaims, claims)
				setTenantContext(c, user, claims)
			}
			c.Next()
			return
		}

		var user models.User
		if err := db.Preload("Role").Preload("Role.Permissions").
			Where("id = ?", claims.UserID).First(&user).Error; err != nil {
			c.Next()
			return
		}

		if user.IsActive() {
			cache.putAtGeneration(&user, cacheGeneration)
			c.Set(ContextKeyUser, &user)
			c.Set(ContextKeyClaims, claims)
			setTenantContext(c, &user, claims)
		}
		c.Next()
	}
}

// RequirePermission checks if the authenticated user has a specific permission.
func RequirePermission(permissionSlug string) gin.HandlerFunc {
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
		if !hasPermission(u, permissionSlug) {
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

// RequireEditor checks for editor or admin role.
func RequireEditor() gin.HandlerFunc {
	return RequireRole("admin", "editor")
}

// hasPermission checks if a user has a specific permission.
func hasPermission(user *models.User, slug string) bool {
	// Admins have all permissions.
	if user.Role.Slug == "admin" {
		return true
	}
	for _, perm := range user.Role.Permissions {
		if perm.Slug == slug {
			return true
		}
	}
	return false
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

// setTenantContext resolves and stores the request tenant in the gin
// context. Resolution order: X-Tenant-ID header (platform admins only) →
// JWT claim TenantID → default tenant.
func setTenantContext(c *gin.Context, user *models.User, claims *auth.Claims) {
	tenantID := models.DefaultTenantID
	if claims != nil && claims.TenantID > 0 {
		tenantID = claims.TenantID
	}
	if user != nil && user.Role.Slug == "admin" {
		if v := c.GetHeader(TenantOverrideHeader); v != "" {
			if id, err := strconv.ParseUint(v, 10, 64); err == nil && id > 0 {
				tenantID = uint(id)
			}
		}
	}
	c.Set(ContextKeyTenant, tenantID)
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
