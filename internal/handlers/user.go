package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/yamovo/contentx/internal/models" // swag annotation resolution
	"github.com/yamovo/contentx/internal/services"
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
//
//	@Summary      List users
//	@Description  Returns users with pagination and filters (requires users.view permission)
//	@Tags         Users
//	@Produce      json
//	@Param        page       query  int     false  "Page number"          default(1)
//	@Param        page_size  query  int     false  "Items per page"       default(20)
//	@Param        role       query  string  false  "Filter by role"
//	@Param        status     query  string  false  "Filter by status"
//	@Param        search     query  string  false  "Search by name/email"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /users [get]
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
		handleServiceError(c, err)
		return
	}

	// Sanitize output.
	safe := make([]*services.SafeUser, len(users))
	for i := range users {
		safe[i] = services.SanitizeUser(&users[i])
	}

	paginate := paginateFrom(page, pageSize, total)
	Success(c, listResponse(safe, paginate))
}

// Get returns a single user.
// GET /api/v1/users/:id
//
//	@Summary      Get user
//	@Description  Returns a single user by ID (requires users.view permission)
//	@Tags         Users
//	@Produce      json
//	@Param        id   path      int     true  "User ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=services.SafeUser}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /users/{id} [get]
func (h *UserHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	user, err := h.svc.Get(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, services.SanitizeUser(user))
}

// Create creates a new user (admin operation).
// POST /api/v1/users
//
//	@Summary      Create user
//	@Description  Creates a new user (requires users.create permission)
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.CreateUserRequest  true  "User data"
//	@Security     BearerAuth
//	@Success      201   {object}  APIResponse{data=services.SafeUser}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var req services.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	user, err := h.svc.Create(req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, services.SanitizeUser(user))
}

// Update updates a user (admin operation).
// PUT /api/v1/users/:id
//
//	@Summary      Update user
//	@Description  Updates a user (requires users.edit permission)
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                        true  "User ID"
//	@Param        body  body      services.UpdateUserRequest  true  "User data"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse{data=services.SafeUser}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Failure      404   {object}  APIResponse
//	@Router       /users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	var req services.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	user, err := h.svc.Update(uint(id), req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, services.SanitizeUser(user))
}

// Delete soft-deletes a user.
// DELETE /api/v1/users/:id
//
//	@Summary      Delete user
//	@Description  Soft-deletes a user (requires users.delete permission)
//	@Tags         Users
//	@Produce      json
//	@Param        id   path      int     true  "User ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "User deleted"})
}

// ResetPassword resets a user's password (admin operation).
// POST /api/v1/users/:id/reset-password
//
//	@Summary      Reset user password
//	@Description  Resets a user's password (requires users.edit permission)
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int     true  "User ID"
//	@Param        body  body      object  true  "Payload {new_password}"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Failure      404   {object}  APIResponse
//	@Router       /users/{id}/reset-password [post]
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.ResetPassword(uint(id), req.NewPassword); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Password reset successfully"})
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
//
//	@Summary      List roles
//	@Description  Returns all roles (requires roles.view permission)
//	@Tags         Roles
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=[]models.Role}
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /roles [get]
func (h *RoleHandler) List(c *gin.Context) {
	roles, err := h.svc.List()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, roles)
}

// Create creates a new role.
// POST /api/v1/roles
//
//	@Summary      Create role
//	@Description  Creates a new role (requires roles.manage permission)
//	@Tags         Roles
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.CreateRoleRequest  true  "Role data"
//	@Security     BearerAuth
//	@Success      201   {object}  APIResponse{data=models.Role}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /roles [post]
func (h *RoleHandler) Create(c *gin.Context) {
	var req services.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	role, err := h.svc.Create(req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, role)
}

// Update updates a role.
// PUT /api/v1/roles/:id
//
//	@Summary      Update role
//	@Description  Updates a role (requires roles.manage permission)
//	@Tags         Roles
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                        true  "Role ID"
//	@Param        body  body      services.UpdateRoleRequest  true  "Role data"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse{data=models.Role}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Failure      404   {object}  APIResponse
//	@Router       /roles/{id} [put]
func (h *RoleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid role ID")
		return
	}

	var req services.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	role, err := h.svc.Update(uint(id), req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, role)
}

// Delete removes a role.
// DELETE /api/v1/roles/:id
//
//	@Summary      Delete role
//	@Description  Removes a role (requires roles.manage permission)
//	@Tags         Roles
//	@Produce      json
//	@Param        id   path      int     true  "Role ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /roles/{id} [delete]
func (h *RoleHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid role ID")
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Role deleted"})
}

// Permissions returns all permissions.
// GET /api/v1/roles/permissions
//
//	@Summary      List permissions
//	@Description  Returns all available permissions grouped by module
//	@Tags         Roles
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Router       /roles/permissions [get]
func (h *RoleHandler) Permissions(c *gin.Context) {
	perms, grouped, err := h.svc.Permissions()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"data": perms, "grouped": grouped})
}
