package services

import (
	"regexp"
	"strings"

	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/permissions"
	"github.com/yamovo/contentx/internal/repository"
)

// slugPattern constrains tenant slugs to URL-safe lowercase identifiers.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// TenantService implements RFC-001 PR-5 (front half): platform-level tenant
// administration and membership management. It is deliberately identity-plane
// only — member management is a platform capability guarded by the
// tenants.* platform permissions, never by tenant membership roles, so a
// tenant administrator cannot grant themselves into another tenant.
type TenantService struct {
	repo  repository.TenantRepository
	audit AuditLogger
}

// NewTenantService builds a TenantService.
func NewTenantService(repo repository.TenantRepository) *TenantService {
	return &TenantService{repo: repo, audit: NoopAuditLogger{}}
}

// SetAuditLogger wires the business-level audit logger. Tenant administration
// events are high-risk and are written via LogReliable (fail-closed).
func (s *TenantService) SetAuditLogger(l AuditLogger) {
	if l != nil {
		s.audit = l
	}
}

// List returns every tenant.
func (s *TenantService) List() ([]models.Tenant, error) {
	return s.repo.List()
}

// Get returns one tenant by ID.
func (s *TenantService) Get(id uint) (*models.Tenant, error) {
	tenant, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errs.ErrNotFound.WithMessage("tenant not found")
	}
	return tenant, nil
}

// CreateTenantRequest is the payload for creating a tenant.
type CreateTenantRequest struct {
	Name     string `json:"name" binding:"required"`
	Slug     string `json:"slug" binding:"required"`
	MaxUsers int    `json:"max_users"`
}

// Create adds a new tenant. The audit event is written reliably; a failed
// audit write surfaces as a failed request (fail-closed).
func (s *TenantService) Create(req CreateTenantRequest) (*models.Tenant, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	if req.Name == "" || req.Slug == "" {
		return nil, errs.ErrBadRequest.WithMessage("name and slug are required")
	}
	if !slugPattern.MatchString(req.Slug) || len(req.Slug) > 64 {
		return nil, errs.ErrBadRequest.WithMessage("slug must be lowercase letters, digits, and dashes (max 64)")
	}
	if _, err := s.repo.GetBySlug(req.Slug); err == nil {
		return nil, errs.ErrConflict.WithMessage("tenant slug already exists")
	}

	tenant := &models.Tenant{
		Name:     req.Name,
		Slug:     req.Slug,
		Status:   models.TenantStatusActive,
		MaxUsers: req.MaxUsers,
	}
	if err := s.repo.Create(tenant); err != nil {
		return nil, err
	}
	if err := s.auditTenant("tenant.create", tenant.ID, map[string]any{
		"name": tenant.Name, "slug": tenant.Slug, "max_users": tenant.MaxUsers,
	}); err != nil {
		return nil, err
	}
	return tenant, nil
}

// UpdateTenantRequest is the payload for updating name, status, and quota hook.
type UpdateTenantRequest struct {
	Name     *string `json:"name"`
	Status   *string `json:"status"`
	MaxUsers *int    `json:"max_users"`
}

// Update applies a partial update to a tenant.
func (s *TenantService) Update(id uint, req UpdateTenantRequest) (*models.Tenant, error) {
	tenant, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errs.ErrNotFound.WithMessage("tenant not found")
	}

	changes := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, errs.ErrBadRequest.WithMessage("name cannot be empty")
		}
		tenant.Name = name
		changes["name"] = name
	}
	if req.Status != nil {
		if *req.Status != models.TenantStatusActive && *req.Status != models.TenantStatusSuspended {
			return nil, errs.ErrBadRequest.WithMessage("status must be active or suspended")
		}
		tenant.Status = *req.Status
		changes["status"] = *req.Status
	}
	if req.MaxUsers != nil {
		if *req.MaxUsers < 0 {
			return nil, errs.ErrBadRequest.WithMessage("max_users cannot be negative")
		}
		tenant.MaxUsers = *req.MaxUsers
		changes["max_users"] = *req.MaxUsers
	}
	if len(changes) == 0 {
		return nil, errs.ErrBadRequest.WithMessage("no changes supplied")
	}

	if err := s.repo.Update(tenant); err != nil {
		return nil, err
	}
	if err := s.auditTenant("tenant.update", tenant.ID, changes); err != nil {
		return nil, err
	}
	return tenant, nil
}

// ListMembers returns the membership roster of a tenant.
func (s *TenantService) ListMembers(tenantID uint) ([]repository.TenantMember, error) {
	if _, err := s.repo.GetByID(tenantID); err != nil {
		return nil, errs.ErrNotFound.WithMessage("tenant not found")
	}
	return s.repo.ListMembers(tenantID)
}

// AddMemberRequest is the payload for adding a member.
type AddMemberRequest struct {
	UserID   uint   `json:"user_id" binding:"required"`
	RoleSlug string `json:"role_slug" binding:"required"`
}

// AddMember links an existing user to a tenant with a canonical role.
func (s *TenantService) AddMember(tenantID uint, req AddMemberRequest) (*repository.TenantMember, error) {
	if _, err := s.repo.GetByID(tenantID); err != nil {
		return nil, errs.ErrNotFound.WithMessage("tenant not found")
	}
	role, ok := permissions.NormalizeTenantRole(req.RoleSlug)
	if !ok {
		return nil, errs.ErrBadRequest.WithMessage("role_slug must be admin, editor, or member")
	}
	exists, err := s.repo.UserExists(req.UserID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errs.ErrNotFound.WithMessage("user not found")
	}
	duplicate, err := s.repo.HasMembership(tenantID, req.UserID)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return nil, errs.ErrConflict.WithMessage("user is already a member of this tenant")
	}

	if err := s.repo.AddMember(&models.TenantMembership{
		TenantID: tenantID,
		UserID:   req.UserID,
		RoleSlug: role,
	}); err != nil {
		return nil, err
	}
	if err := s.auditTenant("tenant.member_add", tenantID, map[string]any{
		"user_id": req.UserID, "role_slug": role,
	}); err != nil {
		return nil, err
	}
	members, err := s.repo.ListMembers(tenantID)
	if err != nil {
		return nil, err
	}
	for i := range members {
		if members[i].UserID == req.UserID {
			return &members[i], nil
		}
	}
	return nil, errs.ErrNotFound.WithMessage("member not found after insert")
}

// UpdateMemberRoleRequest is the payload for changing a member's role.
type UpdateMemberRoleRequest struct {
	RoleSlug string `json:"role_slug" binding:"required"`
}

// UpdateMemberRole changes a member's tenant role with fail-closed auditing.
func (s *TenantService) UpdateMemberRole(tenantID, userID uint, req UpdateMemberRoleRequest) error {
	role, ok := permissions.NormalizeTenantRole(req.RoleSlug)
	if !ok {
		return errs.ErrBadRequest.WithMessage("role_slug must be admin, editor, or member")
	}
	member, err := s.repo.HasMembership(tenantID, userID)
	if err != nil {
		return err
	}
	if !member {
		return errs.ErrNotFound.WithMessage("membership not found")
	}
	if err := s.repo.UpdateMemberRole(tenantID, userID, role); err != nil {
		return err
	}
	return s.auditTenant("tenant.member_role", tenantID, map[string]any{
		"user_id": userID, "role_slug": role,
	})
}

// RemoveMember detaches a user from a tenant. The last tenant admin cannot be
// removed, so a tenant never becomes administratively orphaned.
func (s *TenantService) RemoveMember(tenantID, userID uint) error {
	member, err := s.repo.HasMembership(tenantID, userID)
	if err != nil {
		return err
	}
	if !member {
		return errs.ErrNotFound.WithMessage("membership not found")
	}

	var role string
	members, err := s.repo.ListMembers(tenantID)
	if err != nil {
		return err
	}
	for i := range members {
		if members[i].UserID == userID {
			role = members[i].RoleSlug
		}
	}
	if role == models.TenantRoleAdmin {
		admins, err := s.repo.CountAdmins(tenantID)
		if err != nil {
			return err
		}
		if admins <= 1 {
			return errs.ErrBadRequest.WithMessage("cannot remove the last admin of a tenant")
		}
	}

	if err := s.repo.RemoveMember(tenantID, userID); err != nil {
		return err
	}
	return s.auditTenant("tenant.member_remove", tenantID, map[string]any{"user_id": userID})
}

// auditTenant writes a tenant administration event reliably. Member events
// are attributed to the affected tenant so its administrators see them;
// tenant-level rows are the platform record for that tenant.
func (s *TenantService) auditTenant(action string, tenantID uint, details map[string]any) error {
	return s.audit.LogReliable(AuditEvent{
		TenantID:  &tenantID,
		Action:    action,
		Entity:    "tenant",
		EntityID:  tenantID,
		Details:   details,
		Source:    SourceREST,
		ActorType: ActorUser,
		Outcome:   OutcomeSuccess,
	})
}
