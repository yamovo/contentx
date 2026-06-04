package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/auth"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// UserHandler handles user management.
type UserHandler struct{ db *gorm.DB }

func NewUserHandler(db *gorm.DB) *UserHandler { return &UserHandler{db: db} }

// List returns users with pagination.
// GET /api/v1/users?role=admin&status=active&search=john&page=1
func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	role := c.Query("role")
	status := c.Query("status")
	search := c.Query("search")

	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }

	query := h.db.Model(&models.User{}).Preload("Role")
	if role != "" { query = query.Joins("JOIN roles ON roles.id = users.role_id").Where("roles.slug = ?", role) }
	if status != "" { query = query.Where("status = ?", status) }
	if search != "" {
		query = query.Where("username LIKE ? OR email LIKE ? OR display_name LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var users []models.User
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users)

	// Sanitize output.
	type SafeUser struct {
		ID          uint   `json:"id"`
		Username    string `json:"username"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Avatar      string `json:"avatar"`
		Role        models.Role `json:"role"`
		Status      string `json:"status"`
		LoginCount  int    `json:"login_count"`
		CreatedAt   string `json:"created_at"`
	}
	safe := make([]SafeUser, len(users))
	for i, u := range users {
		safe[i] = SafeUser{
			ID: u.ID, Username: u.Username, Email: u.Email,
			DisplayName: u.DisplayName, Avatar: u.AvatarURL(),
			Role: u.Role, Status: string(u.Status),
			LoginCount: u.LoginCount, CreatedAt: u.CreatedAt.Format("2006-01-02 15:04"),
		}
	}

	paginate := models.Paginate{Page: page, PageSize: pageSize, Total: total}
	c.JSON(http.StatusOK, models.NewListResponse(safe, paginate))
}

// Get returns a single user.
// GET /api/v1/users/:id
func (h *UserHandler) Get(c *gin.Context) {
	var user models.User
	if err := h.db.Preload("Role").First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": sanitizeUser(&user)})
}

// Create creates a new user (admin operation).
// POST /api/v1/users
func (h *UserHandler) Create(c *gin.Context) {
	var req struct {
		Username    string `json:"username" binding:"required,min=3,max=64"`
		Email       string `json:"email" binding:"required,email"`
		Password    string `json:"password" binding:"required,min=8"`
		DisplayName string `json:"display_name"`
		RoleID      uint   `json:"role_id" binding:"required"`
		Status      string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPw, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := models.User{
		Username:    req.Username,
		Email:       req.Email,
		Password:    hashedPw,
		DisplayName: req.DisplayName,
		RoleID:      req.RoleID,
		Status:      models.UserStatus(req.Status),
	}
	if user.Status == "" { user.Status = models.UserStatusActive }

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username or email already exists"})
		return
	}

	h.db.Preload("Role").First(&user, user.ID)
	c.JSON(http.StatusCreated, gin.H{"data": sanitizeUser(&user)})
}

// Update updates a user (admin operation).
// PUT /api/v1/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	var user models.User
	if err := h.db.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		Email       *string `json:"email"`
		DisplayName *string `json:"display_name"`
		RoleID      *uint   `json:"role_id"`
		Status      *string `json:"status"`
		Bio         *string `json:"bio"`
		Website     *string `json:"website"`
		Avatar      *string `json:"avatar"`
	}
	c.ShouldBindJSON(&req)

	updates := map[string]interface{}{}
	if req.Email != nil { updates["email"] = *req.Email }
	if req.DisplayName != nil { updates["display_name"] = *req.DisplayName }
	if req.RoleID != nil { updates["role_id"] = *req.RoleID }
	if req.Status != nil { updates["status"] = *req.Status }
	if req.Bio != nil { updates["bio"] = *req.Bio }
	if req.Website != nil { updates["website"] = *req.Website }
	if req.Avatar != nil { updates["avatar"] = *req.Avatar }

	h.db.Model(&user).Updates(updates)
	h.db.Preload("Role").First(&user, user.ID)
	c.JSON(http.StatusOK, gin.H{"data": sanitizeUser(&user)})
}

// Delete soft-deletes a user.
// DELETE /api/v1/users/:id
func (h *UserHandler) Delete(c *gin.Context) {
	var user models.User
	if err := h.db.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.IsAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete admin user"})
		return
	}
	h.db.Delete(&user)
	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}

// ResetPassword resets a user's password (admin operation).
// POST /api/v1/users/:id/reset-password
func (h *UserHandler) ResetPassword(c *gin.Context) {
	var user models.User
	if err := h.db.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPw, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Model(&user).Update("password", hashedPw)
	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

// ---------- Role Handlers ----------

// RoleHandler handles role management.
type RoleHandler struct{ db *gorm.DB }

func NewRoleHandler(db *gorm.DB) *RoleHandler { return &RoleHandler{db: db} }

// List returns all roles.
// GET /api/v1/roles
func (h *RoleHandler) List(c *gin.Context) {
	var roles []models.Role
	h.db.Preload("Permissions").Order("id ASC").Find(&roles)

	// Add user counts.
	for i := range roles {
		var count int64
		h.db.Model(&models.User{}).Where("role_id = ?", roles[i].ID).Count(&count)
		roles[i].UserCount = int(count)
	}

	c.JSON(http.StatusOK, gin.H{"data": roles})
}

// Create creates a new role.
// POST /api/v1/roles
func (h *RoleHandler) Create(c *gin.Context) {
	var req struct {
		Name          string   `json:"name" binding:"required"`
		Slug          string   `json:"slug" binding:"required"`
		Description   string   `json:"description"`
		PermissionIDs []uint   `json:"permission_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role := models.Role{
		Name: req.Name, Slug: req.Slug, Description: req.Description,
	}

	if err := h.db.Create(&role).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Role already exists"})
		return
	}

	if len(req.PermissionIDs) > 0 {
		var perms []models.Permission
		h.db.Where("id IN ?", req.PermissionIDs).Find(&perms)
		h.db.Model(&role).Association("Permissions").Replace(perms)
	}

	h.db.Preload("Permissions").First(&role, role.ID)
	c.JSON(http.StatusCreated, gin.H{"data": role})
}

// Update updates a role.
// PUT /api/v1/roles/:id
func (h *RoleHandler) Update(c *gin.Context) {
	var role models.Role
	if err := h.db.First(&role, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
		return
	}
	if role.IsSystem {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot modify system roles"})
		return
	}

	var req struct {
		Name          *string `json:"name"`
		Description   *string `json:"description"`
		PermissionIDs []uint  `json:"permission_ids"`
	}
	c.ShouldBindJSON(&req)

	if req.Name != nil { h.db.Model(&role).Update("name", *req.Name) }
	if req.Description != nil { h.db.Model(&role).Update("description", *req.Description) }
	if req.PermissionIDs != nil {
		var perms []models.Permission
		h.db.Where("id IN ?", req.PermissionIDs).Find(&perms)
		h.db.Model(&role).Association("Permissions").Replace(perms)
	}

	h.db.Preload("Permissions").First(&role, role.ID)
	c.JSON(http.StatusOK, gin.H{"data": role})
}

// Delete removes a role.
// DELETE /api/v1/roles/:id
func (h *RoleHandler) Delete(c *gin.Context) {
	var role models.Role
	if err := h.db.First(&role, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
		return
	}
	if role.IsSystem {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete system roles"})
		return
	}
	// Check if role is in use.
	var count int64
	h.db.Model(&models.User{}).Where("role_id = ?", role.ID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Role is still assigned to users"})
		return
	}
	h.db.Model(&role).Association("Permissions").Clear()
	h.db.Delete(&role)
	c.JSON(http.StatusOK, gin.H{"message": "Role deleted"})
}

// Permissions returns all permissions.
// GET /api/v1/permissions
func (h *RoleHandler) Permissions(c *gin.Context) {
	var perms []models.Permission
	h.db.Order("module ASC, name ASC").Find(&perms)

	// Group by module.
	grouped := make(map[string][]models.Permission)
	for _, p := range perms {
		grouped[p.Module] = append(grouped[p.Module], p)
	}
	c.JSON(http.StatusOK, gin.H{"data": perms, "grouped": grouped})
}
