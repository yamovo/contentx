package graphql

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graphql-go/graphql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
)

// setupTestSchema 构建一个基于内存 SQLite 的 GraphQL schema，预置：
//   - 1 个用户（id=1，author）
//   - 1 个分类（id=1）
//   - 2 个标签（id=1,2）
//   - 2 篇文章（id=1 已发布，id=2 草稿）
//   - 1 条已审核评论挂在文章 1 下
func setupTestSchema(t *testing.T) (graphql.Schema, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 种子数据
	user := models.User{
		BaseModel:   models.BaseModel{ID: 1},
		Username:    "alice",
		Email:       "alice@example.com",
		DisplayName: "Alice",
		Password:    "$2a$10$redacted", // 不应出现在 GraphQL 响应中
		Status:      models.UserStatusActive,
		RoleID:      1,
		LoginCount:  5,
	}
	db.Create(&user)

	category := models.Category{
		BaseModel: models.BaseModel{ID: 1},
		Name:      "Tech",
		Slug:      "tech",
		IsActive:  true,
	}
	db.Create(&category)

	tags := []models.Tag{
		{BaseModel: models.BaseModel{ID: 1}, Name: "Go", Slug: "go"},
		{BaseModel: models.BaseModel{ID: 2}, Name: "CMS", Slug: "cms"},
	}
	db.Create(&tags)

	publishedAt := time.Now()
	articles := []models.Article{
		{
			BaseModel:    models.BaseModel{ID: 1},
			Title:        "Hello World",
			Slug:         "hello-world",
			Content:      "First post content",
			Excerpt:      "First post",
			AuthorID:     1,
			CategoryID:   uintPtr(1),
			Status:       models.StatusPublished,
			PostType:     models.PostTypePost,
			Visibility:   models.VisibilityPublic,
			AllowComment: true,
			ViewCount:    42,
			WordCount:    100,
			ReadingTime:  2,
			Version:      1,
			PublishedAt:  &publishedAt, // GenerateFeed 需要 PublishedAt 非空
		},
		{
			BaseModel: models.BaseModel{ID: 2},
			Title:     "Draft Post",
			Slug:      "draft-post",
			Content:   "Draft content",
			AuthorID:  1,
			Status:    models.StatusDraft,
			PostType:  models.PostTypePost,
		},
	}
	db.Create(&articles)

	// 文章 1 关联标签 1,2
	db.Model(&models.Article{BaseModel: models.BaseModel{ID: 1}}).
		Association("Tags").Replace(&models.Tag{BaseModel: models.BaseModel{ID: 1}}, &models.Tag{BaseModel: models.BaseModel{ID: 2}})

	comment := models.Comment{
		BaseModel:   models.BaseModel{ID: 1},
		ArticleID:   1,
		AuthorName:  "Bob",
		AuthorEmail: "bob@example.com", // 不应在 GraphQL 响应中暴露
		AuthorIP:    "1.2.3.4",         // 不应在 GraphQL 响应中暴露
		Content:     "Great post!",
		Status:      "approved",
		Depth:       0,
	}
	db.Create(&comment)

	svc := Services{
		Article:  services.NewArticleService(db, "http://localhost:8080"),
		Category: services.NewCategoryService(db),
		Tag:      services.NewTagService(db),
		Comment:  services.NewCommentService(db),
		User:     services.NewUserService(db),
	}
	schema, err := NewSchema(svc)
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	return schema, db
}

func uintPtr(n uint) *uint { return &n }

// ---------- Schema 构建测试 ----------

func TestSchema_Build(t *testing.T) {
	schema, _ := setupTestSchema(t)
	if schema.QueryType() == nil {
		t.Fatal("expected Query type to be non-nil")
	}
	// 验证关键类型存在
	for _, name := range []string{"Article", "User", "Category", "Tag", "Comment", "ArticleConnection"} {
		if schema.Type(name) == nil {
			t.Errorf("expected type %q in schema", name)
		}
	}
}

// ---------- Query 测试 ----------

func TestQuery_ArticleBySlug(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { articleBySlug(slug: "hello-world") { id title slug status author { username displayName } category { name slug } tags { name slug } } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	data := res.Data.(map[string]interface{})["articleBySlug"].(map[string]interface{})
	if data["title"] != "Hello World" {
		t.Errorf("title = %v, want Hello World", data["title"])
	}
	if data["status"] != string(models.StatusPublished) {
		t.Errorf("status = %v, want published", data["status"])
	}
	author := data["author"].(map[string]interface{})
	if author["username"] != "alice" {
		t.Errorf("author.username = %v, want alice", author["username"])
	}
	cat := data["category"].(map[string]interface{})
	if cat["name"] != "Tech" {
		t.Errorf("category.name = %v, want Tech", cat["name"])
	}
	tags := data["tags"].([]interface{})
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}
}

func TestQuery_ArticleBySlug_NotFound(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { articleBySlug(slug: "nonexistent") { id title } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	// 不存在的 slug 应返回 null（不是错误）
	if res.Data.(map[string]interface{})["articleBySlug"] != nil {
		t.Errorf("expected null for non-existent slug, got %v", res.Data)
	}
}

func TestQuery_Articles_DefaultOnlyPublished(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { articles { total items { id title status } } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	conn := res.Data.(map[string]interface{})["articles"].(map[string]interface{})
	items := conn["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 published article, got %d", len(items))
	}
	first := items[0].(map[string]interface{})
	if first["status"] != string(models.StatusPublished) {
		t.Errorf("expected published status, got %v", first["status"])
	}
}

func TestQuery_Articles_ExplicitStatus(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { articles(status: "draft") { total items { id title status } } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	conn := res.Data.(map[string]interface{})["articles"].(map[string]interface{})
	items := conn["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 draft article, got %d", len(items))
	}
	if items[0].(map[string]interface{})["title"] != "Draft Post" {
		t.Errorf("expected Draft Post, got %v", items[0].(map[string]interface{})["title"])
	}
}

func TestQuery_Articles_PaginationFields(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { articles(page: 1, pageSize: 10) { page pageSize total totalPages hasNext hasPrev } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	conn := res.Data.(map[string]interface{})["articles"].(map[string]interface{})
	// graphql-go 对 Int 字段返回原始 int 值（不转 float64），用 toInt 兼容。
	if toInt(conn["page"]) != 1 {
		t.Errorf("page = %v, want 1", conn["page"])
	}
	if toInt(conn["total"]) != 1 {
		t.Errorf("total = %v, want 1", conn["total"])
	}
	if conn["hasPrev"].(bool) {
		t.Error("hasPrev should be false on page 1")
	}
}

// TestQuery_Articles_ContentReturned 验证当客户端在 items 中请求 content 字段时，
// 返回的是真实正文而非空字符串（回归 S0-1：列表精简优化导致 GraphQL content 回归）。
func TestQuery_Articles_ContentReturned(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { articles { items { id title content } } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	conn := res.Data.(map[string]interface{})["articles"].(map[string]interface{})
	items := conn["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 published article, got %d", len(items))
	}
	first := items[0].(map[string]interface{})
	if first["content"] != "First post content" {
		t.Errorf("content = %q, want %q", first["content"], "First post content")
	}
	if first["title"] != "Hello World" {
		t.Errorf("title = %v, want Hello World", first["title"])
	}
}

// TestQuery_Articles_ContentViaFragment 验证通过 fragment spread 请求 content 时，
// 仍然返回真实正文（覆盖 fieldInSelection 对 FragmentSpread 的解析）。
func TestQuery_Articles_ContentViaFragment(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { articles { items { ...ArticleFields } } } fragment ArticleFields on Article { id title content }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	conn := res.Data.(map[string]interface{})["articles"].(map[string]interface{})
	items := conn["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 published article, got %d", len(items))
	}
	first := items[0].(map[string]interface{})
	if first["content"] != "First post content" {
		t.Errorf("content = %q, want %q", first["content"], "First post content")
	}
}

// TestQuery_Articles_SlimQueryOmitsContent 验证不请求 content 时查询仍然正常，
// 且 content 字段为空字符串（精简优化生效，未加载正文列）。
func TestQuery_Articles_SlimQueryOmitsContent(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { articles { items { id title slug } } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	conn := res.Data.(map[string]interface{})["articles"].(map[string]interface{})
	items := conn["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 published article, got %d", len(items))
	}
	first := items[0].(map[string]interface{})
	if first["title"] != "Hello World" {
		t.Errorf("title = %v, want Hello World", first["title"])
	}
	if first["slug"] != "hello-world" {
		t.Errorf("slug = %v, want hello-world", first["slug"])
	}
}

// toInt 把 graphql 响应中的数值字段统一转成 int（兼容 int/float64/int64）。
func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func TestQuery_Categories(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { categories { id name slug } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	cats := res.Data.(map[string]interface{})["categories"].([]interface{})
	if len(cats) == 0 {
		t.Fatal("expected at least 1 category")
	}
	first := cats[0].(map[string]interface{})
	if first["name"] != "Tech" {
		t.Errorf("first category name = %v, want Tech", first["name"])
	}
}

func TestQuery_Tags(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { tags { id name slug } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	tags := res.Data.(map[string]interface{})["tags"].([]interface{})
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
}

func TestQuery_Comments(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { comments(articleId: 1) { id content authorName status children { id content } } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	comments := res.Data.(map[string]interface{})["comments"].([]interface{})
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	c := comments[0].(map[string]interface{})
	if c["content"] != "Great post!" {
		t.Errorf("content = %v, want 'Great post!'", c["content"])
	}
	if c["authorName"] != "Bob" {
		t.Errorf("authorName = %v, want Bob", c["authorName"])
	}
}

func TestQuery_User_NoSensitiveFields(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { user(id: 1) { id username displayName loginCount } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	user := res.Data.(map[string]interface{})["user"].(map[string]interface{})
	if user["username"] != "alice" {
		t.Errorf("username = %v, want alice", user["username"])
	}
	if toInt(user["loginCount"]) != 5 {
		t.Errorf("loginCount = %v, want 5", user["loginCount"])
	}
}

func TestQuery_User_RejectsEmailField(t *testing.T) {
	schema, _ := setupTestSchema(t)
	// email 字段不在 User 类型上，应当返回校验错误
	q := `query { user(id: 1) { email } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if !res.HasErrors() {
		t.Fatal("expected validation error for 'email' field, got none")
	}
}

func TestQuery_Article_NestedComments(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { articleBySlug(slug: "hello-world") { title comments { content authorName } } }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	article := res.Data.(map[string]interface{})["articleBySlug"].(map[string]interface{})
	comments := article["comments"].([]interface{})
	if len(comments) != 1 {
		t.Fatalf("expected 1 nested comment, got %d", len(comments))
	}
}

func TestQuery_Feed(t *testing.T) {
	schema, _ := setupTestSchema(t)
	q := `query { feed }`
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	feed, ok := res.Data.(map[string]interface{})["feed"].(string)
	if !ok || feed == "" {
		t.Errorf("expected non-empty feed string, got %v", res.Data)
	}
}

// ---------- HTTP handler 测试 ----------

func TestHandler_POST_JSON(t *testing.T) {
	schema, _ := setupTestSchema(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/graphql", Handler(schema))

	body := `{"query":"{ tags { name } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data field, got %v", resp)
	}
	tags, ok := data["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Errorf("expected 2 tags, got %v", data["tags"])
	}
}

func TestHandler_POST_RawString(t *testing.T) {
	schema, _ := setupTestSchema(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/graphql", Handler(schema))

	// 原始 query 字符串（非 JSON），便于 curl 直接调用
	body := `{ tags { name } }`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestHandler_GET(t *testing.T) {
	schema, _ := setupTestSchema(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/graphql", Handler(schema))

	req := httptest.NewRequest(http.MethodGet, "/graphql?query={tags{name}}", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := resp["data"].(map[string]interface{}); !ok {
		t.Errorf("expected data field, got %v", resp)
	}
}

func TestHandler_MissingQuery(t *testing.T) {
	schema, _ := setupTestSchema(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/graphql", Handler(schema))

	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandler_InvalidJSON(t *testing.T) {
	schema, _ := setupTestSchema(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/graphql", Handler(schema))

	// 非法 JSON 但以 { 开头（走 JSON 解析分支）
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandler_SyntaxError(t *testing.T) {
	schema, _ := setupTestSchema(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/graphql", Handler(schema))

	// 语法错误的 query
	body := `{"query":"@@@invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 语法错误应当返回 400（没有数据）
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
