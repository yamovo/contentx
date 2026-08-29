package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/yamovo/contentx/internal/services"
)

// TenantHandler serves the RFC-001 PR-5 platform tenant administration API:
// tenant CRUD and membership management, guarded by the platform
// tenants.* permissions. Tenant membership roles deliberately cannot reach
// these routes (identity plane belongs to the deployment operator).
type TenantHandler struct {
	svc *services.TenantService
}

// NewTenantHandler builds the handler.
func NewTenantHandler(svc *services.TenantService) *TenantHandler {
	return &TenantHandler{svc: svc}
}

// List godoc
//
//	@Summary		List tenants (platform)
//	@Description	Returns every tenant with status and member quota hook.
//	@Tags			Tenants
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Failure		403	{object}	map[string]interface{}	"Requires tenants.read platform permission"
//	@Router			/admin/tenants [get]
func (h *TenantHandler) List(c *gin.Context) {
	tenants, err := h.svc.List()
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, tenants)
}

// Get godoc
//
//	@Summary		Get a tenant (platform)
//	@Tags			Tenants
//	@Produce		json
//	@Param			id	path		int	true	"Tenant ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Router			/admin/tenants/{id} [get]
func (h *TenantHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid tenant id")
		return
	}
	tenant, err := h.svc.Get(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, tenant)
}

// Create godoc
//
//	@Summary		Create a tenant (platform)
//	@Tags			Tenants
//	@Accept			json
//	@Produce		json
//	@Param			body	body		services.CreateTenantRequest	true	"Tenant payload"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		409		{object}	map[string]interface{}	"Slug already exists"
//	@Router			/admin/tenants [post]
func (h *TenantHandler) Create(c *gin.Context) {
	var req services.CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	tenant, err := h.svc.Create(req)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, tenant)
}

// Update godoc
//
//	@Summary		Update a tenant (platform)
//	@Description	Partial update of name, status (active/suspended), and max_users.
//	@Tags			Tenants
//	@Accept			json
//	@Produce		json
//	@Param			id		path	int								true	"Tenant ID"
//	@Param			body	body	services.UpdateTenantRequest	true	"Fields to update"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		404		{object}	map[string]interface{}
//	@Router			/admin/tenants/{id} [put]
func (h *TenantHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid tenant id")
		return
	}
	var req services.UpdateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	tenant, err := h.svc.Update(uint(id), req)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, tenant)
}

// ListMembers godoc
//
//	@Summary		List tenant members (platform)
//	@Tags			Tenants
//	@Produce		json
//	@Param			id	path		int	true	"Tenant ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Router			/admin/tenants/{id}/members [get]
func (h *TenantHandler) ListMembers(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid tenant id")
		return
	}
	members, err := h.svc.ListMembers(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, members)
}

// AddMember godoc
//
//	@Summary		Add a tenant member (platform)
//	@Tags			Tenants
//	@Accept			json
//	@Produce		json
//	@Param			id		path	int								true	"Tenant ID"
//	@Param			body	body	services.AddMemberRequest		true	"User and role"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}	"Invalid role"
//	@Failure		404		{object}	map[string]interface{}	"Tenant or user not found"
//	@Failure		409		{object}	map[string]interface{}	"Already a member"
//	@Router			/admin/tenants/{id}/members [post]
func (h *TenantHandler) AddMember(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid tenant id")
		return
	}
	var req services.AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	member, err := h.svc.AddMember(uint(id), req)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, member)
}

// UpdateMemberRole godoc
//
//	@Summary		Update a member's tenant role (platform)
//	@Tags			Tenants
//	@Accept			json
//	@Produce		json
//	@Param			id		path	int									true	"Tenant ID"
//	@Param			userId	path	int									true	"User ID"
//	@Param			body	body	services.UpdateMemberRoleRequest	true	"New role"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		404		{object}	map[string]interface{}
//	@Router			/admin/tenants/{id}/members/{userId} [put]
func (h *TenantHandler) UpdateMemberRole(c *gin.Context) {
	ids, ok := parseTenantMemberIDs(c)
	if !ok {
		return
	}
	var req services.UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if err := h.svc.UpdateMemberRole(ids.tenantID, ids.userID, req); err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, gin.H{"message": "role updated"})
}

// RemoveMember godoc
//
//	@Summary		Remove a tenant member (platform)
//	@Description	The last admin of a tenant cannot be removed.
//	@Tags			Tenants
//	@Produce		json
//	@Param			id		path	int	true	"Tenant ID"
//	@Param			userId	path	int	true	"User ID"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}	"Last admin protection"
//	@Failure		404		{object}	map[string]interface{}
//	@Router			/admin/tenants/{id}/members/{userId} [delete]
func (h *TenantHandler) RemoveMember(c *gin.Context) {
	ids, ok := parseTenantMemberIDs(c)
	if !ok {
		return
	}
	if err := h.svc.RemoveMember(ids.tenantID, ids.userID); err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, gin.H{"message": "member removed"})
}

type tenantMemberIDs struct {
	tenantID uint
	userID   uint
}

func parseTenantMemberIDs(c *gin.Context) (tenantMemberIDs, bool) {
	tenantID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || tenantID == 0 {
		Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid tenant id")
		return tenantMemberIDs{}, false
	}
	userID, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil || userID == 0 {
		Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid user id")
		return tenantMemberIDs{}, false
	}
	return tenantMemberIDs{tenantID: uint(tenantID), userID: uint(userID)}, true
}
