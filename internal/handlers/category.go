package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/yamovo/contentx/internal/models" // swag annotation resolution
	"github.com/yamovo/contentx/internal/services"
)

// CategoryHandler handles category CRUD operations.
type CategoryHandler struct {
	svc *services.CategoryService
}

func NewCategoryHandler(svc *services.CategoryService) *CategoryHandler {
	return &CategoryHandler{svc: svc}
}

// List returns all categories in a tree structure.
// GET /api/v1/categories
//
//	@Summary      List categories
//	@Description  Returns all categories as a tree structure
//	@Tags         Categories
//	@Produce      json
//	@Param        all   query   string  false  "Show all categories including empty"  default(false)
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse{data=[]services.CategoryTree}
//	@Failure      401   {object}  APIResponse
//	@Router       /categories [get]
func (h *CategoryHandler) List(c *gin.Context) {
	showAll := c.Query("all") == "true"

	categories, err := h.svc.List(showAll)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	tree := services.BuildCategoryTree(categories, nil)
	Success(c, tree)
}

// Get returns a single category.
// GET /api/v1/categories/:id
//
//	@Summary      Get category
//	@Description  Returns a single category by ID
//	@Tags         Categories
//	@Produce      json
//	@Param        id   path      int     true  "Category ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.Category}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /categories/{id} [get]
func (h *CategoryHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid category ID")
		return
	}

	category, err := h.svc.Get(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, category)
}

// Create creates a new category.
// POST /api/v1/categories
//
//	@Summary      Create category
//	@Description  Creates a new category (requires categories.manage permission)
//	@Tags         Categories
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.CreateCategoryRequest  true  "Category data"
//	@Security     BearerAuth
//	@Success      201   {object}  APIResponse{data=models.Category}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /categories [post]
func (h *CategoryHandler) Create(c *gin.Context) {
	var req services.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	category, err := h.svc.Create(req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, category)
}

// Update updates an existing category.
// PUT /api/v1/categories/:id
//
//	@Summary      Update category
//	@Description  Updates an existing category (requires categories.manage permission)
//	@Tags         Categories
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                             true  "Category ID"
//	@Param        body  body      services.CreateCategoryRequest  true  "Category data"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Failure      404   {object}  APIResponse
//	@Router       /categories/{id} [put]
func (h *CategoryHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid category ID")
		return
	}

	var req services.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.Update(uint(id), req); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Category updated"})
}

// Delete removes a category.
// DELETE /api/v1/categories/:id
//
//	@Summary      Delete category
//	@Description  Removes a category (requires categories.manage permission)
//	@Tags         Categories
//	@Produce      json
//	@Param        id   path      int     true  "Category ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /categories/{id} [delete]
func (h *CategoryHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid category ID")
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Category deleted"})
}

// Reorder updates sort order for multiple categories.
// PUT /api/v1/categories/reorder
//
//	@Summary      Reorder categories
//	@Description  Updates sort order for multiple categories (requires categories.manage permission)
//	@Tags         Categories
//	@Accept       json
//	@Produce      json
//	@Param        body  body      object  true  "Reorder payload"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /categories/reorder [put]
func (h *CategoryHandler) Reorder(c *gin.Context) {
	var req struct {
		Items []services.ReorderItem `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.Reorder(req.Items); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Categories reordered"})
}
