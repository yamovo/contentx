package models

// Tenant represents an isolated content workspace (multi-tenancy foundation).
// Design: RFC-001 (docs/RFC-001-tenant-model.md) — shared-table isolation via
// tenant_id on business tables; tenants themselves are platform-level rows.
type Tenant struct {
	BaseModel
	Name     string `gorm:"size:128;not null" json:"name"`
	Slug     string `gorm:"uniqueIndex;size:64;not null" json:"slug"`
	Status   string `gorm:"size:20;not null;default:active;index" json:"status"` // active | suspended
	MaxUsers int    `gorm:"not null;default:0" json:"max_users"`                 // 0 = unlimited (quota hook, RFC-001 §8)
}

// Tenant status values.
const (
	TenantStatusActive    = "active"
	TenantStatusSuspended = "suspended"
)

// DefaultTenantID is the fixed ID of the seeded "default" tenant that backs
// all pre-multi-tenancy data and anonymous/public requests (RFC-001 §4.2).
const DefaultTenantID uint = 1

// TenantMembership links a platform-level user to a tenant with a tenant-scoped
// role slug. Users stay global (unique username/email); membership expresses
// which tenants a user belongs to and with which role (RFC-001 §4.3).
type TenantMembership struct {
	BaseModel
	TenantID uint   `gorm:"uniqueIndex:idx_tenant_user;not null" json:"tenant_id"`
	UserID   uint   `gorm:"uniqueIndex:idx_tenant_user;not null" json:"user_id"`
	RoleSlug string `gorm:"size:32;not null;default:member" json:"role_slug"` // member | editor | admin
}

// Tenant role slugs used in memberships.
const (
	TenantRoleMember = "member"
	TenantRoleEditor = "editor"
	TenantRoleAdmin  = "admin"
)
