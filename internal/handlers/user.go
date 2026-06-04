package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/services"
)

// UserHandler handles user management.
type UserHandler struct {
	svc *services.UserService
}

func NewUserHandler(svc *services.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// List returns users with pagination.
// GET /api/v1/users?role=admin&status=active&search=john&page=1
func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	params := services.UserListParams{
		Page:     page,
		PageSize: pageSize,
		Role:     c.Query("role"),
		Status:   c.Query("status"),
		Search:   c.Query("search"),
	}

	users, total, err := h.svc.List(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	// Sanitize output.
	safe := make([]*services.SafeUser, len(users))
	for i := range users {
		safe[i] = services.SanitizeUser(&users[i])
	}

	paginate := paginateFrom(page, pageSize, total)
	c.JSON(http.StatusOK, listResponse(safe, paginate))
}

// Get returns a single user.
// GET /api/v1/users/:id
func (h *UserHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": services.SanitizeUser(user)})
}

// Create creates a new user (admin operation).
// POST /api/v1/users
func (h *UserHandler) Create(c *gin.Context) {
	var req services.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.svc.Create(req)
	if err != nil {
		if err == services.ErrUsernameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Username or email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": services.SanitizeUser(user)})
}

// Update updates a user (admin operation).
// PUT /api/v1/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req services.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.svc.Update(uint(id), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": services.SanitizeUser(user)})
}

// Delete soft-deletes a user.
// DELETE /api/v1/users/:id
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		if err == services.ErrCannotDeleteAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete admin user"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}

// ResetPassword resets a user's password (admin operation).
// POST /api/v1/users/:id/reset-password
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.ResetPassword(uint(id), req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

// ---------- Role Handlers ----------

// RoleHandler handles role management.
type RoleHandler struct {
	svc *services.RoleService
}

func NewRoleHandler(svc *services.RoleService) *RoleHandler {
	return &RoleHandler{svc: svc}
}

// List returns all roles.
// GET /api/v1/roles
func (h *RoleHandler) List(c *gin.Context) {
	roles, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch roles"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": roles})
}

// Create creates a new role.
// POST /api/v1/roles
func (h *RoleHandler) Create(c *gin.Context) {
	var req services.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role, err := h.svc.Create(req)
	if err != nil {
		if err == services.ErrRoleExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Role already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": role})
}

// Update updates a role.
// PUT /api/v1/roles/:id
func (h *RoleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	var req services.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role, err := h.svc.Update(uint(id), req)
	if err != nil {
		if err == services.ErrSystemRole {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot modify system roles"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": role})
}

// Delete removes a role.
// DELETE /api/v1/roles/:id
func (h *RoleHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		if err == services.ErrSystemRole {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete system roles"})
			return
		}
		if err == services.ErrRoleInUse {
			c.JSON(http.StatusConflict, gin.H{"error": "Role is still assigned to users"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role deleted"})
}

// Permissions returns all permissions.
// GET /api/v1/roles/permissions
func (h *RoleHandler) Permissions(c *gin.Context) {
	perms, grouped, err := h.svc.Permissions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": perms, "grouped": grouped})
}
