// Package graphql exposes a read-only GraphQL endpoint on top of the
// existing service layer. It is intended for headless-CMS consumers that
// prefer GraphQL over REST for content delivery (queries only; all writes
// continue to go through the REST API).
//
// The package deliberately avoids code generation: types and the schema are
// defined programmatically with github.com/graphql-go/graphql, and resolvers
// are plain methods on a Resolver struct that delegates to the existing
// services. This matches the rest of the codebase's hand-written style
// (no testify/gomock, no gqlgen codegen step).
package graphql

import (
	"errors"
	"strconv"

	"github.com/graphql-go/graphql"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
	"gorm.io/gorm"
)

// Resolver holds the services the GraphQL layer delegates to. All fields are
// required; nil services will panic on first use.
type Resolver struct {
	Article  *services.ArticleService
	Category *services.CategoryService
	Tag      *services.TagService
	Comment  *services.CommentService
	User     *services.UserService
}

// ---------- Top-level Query resolvers ----------

// article returns a single article by ID (any status).
func (r *Resolver) article(p graphql.ResolveParams) (interface{}, error) {
	id, err := parseID(p, "id")
	if err != nil {
		return nil, err
	}
	return r.Article.Get(id)
}

// articleBySlug returns a single published article by slug and increments
// its view count (same semantics as the REST GET /articles/slug/:slug).
// Returns null (not an error) when the article doesn't exist.
func (r *Resolver) articleBySlug(p graphql.ResolveParams) (interface{}, error) {
	slug, _ := p.Args["slug"].(string)
	article, err := r.Article.GetBySlug(slug)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return article, nil
}

// articles returns a paginated, filtered list of articles. Only published
// articles are exposed when status is empty (public headless-CMS surface);
// authenticated callers can pass status explicitly if needed.
func (r *Resolver) articles(p graphql.ResolveParams) (interface{}, error) {
	filter := services.ListArticlesFilter{
		Page:       intArg(p, "page"),
		PageSize:   intArg(p, "pageSize"),
		Status:     strArg(p, "status"),
		PostType:   strArg(p, "postType"),
		CategoryID: strArg(p, "categoryId"),
		TagSlug:    strArg(p, "tagSlug"),
		Search:     strArg(p, "search"),
		Sort:       strArg(p, "sort"),
		AuthorID:   strArg(p, "authorId"),
	}
	// 默认只返回已发布文章（公共查询面）。调用方显式传 status 时尊重其选择。
	if filter.Status == "" {
		filter.Status = string(models.StatusPublished)
	}
	resp, err := r.Article.List(filter)
	if err != nil {
		return nil, err
	}
	// ListResponse.Items 是 interface{}，转成 []models.Article 以便 GraphQL 解析。
	items, _ := resp.Items.([]models.Article)
	return map[string]interface{}{
		"items":      items,
		"page":       resp.Page,
		"pageSize":   resp.PageSize,
		"total":      resp.Total,
		"totalPages": resp.TotalPages,
		"hasNext":    resp.HasNext,
		"hasPrev":    resp.HasPrev,
	}, nil
}

// category returns a single category by ID.
func (r *Resolver) category(p graphql.ResolveParams) (interface{}, error) {
	id, err := parseID(p, "id")
	if err != nil {
		return nil, err
	}
	return r.Category.Get(id)
}

// categories returns all categories (tree form).
func (r *Resolver) categories(_ graphql.ResolveParams) (interface{}, error) {
	return r.Category.List(false)
}

// tag returns a single tag by ID.
func (r *Resolver) tag(p graphql.ResolveParams) (interface{}, error) {
	id, err := parseID(p, "id")
	if err != nil {
		return nil, err
	}
	return r.Tag.Get(id)
}

// tags returns all tags.
func (r *Resolver) tags(_ graphql.ResolveParams) (interface{}, error) {
	tags, _, err := r.Tag.List(services.TagListParams{})
	if err != nil {
		return nil, err
	}
	return tags, nil
}

// comments returns approved top-level comments for an article (with nested
// children preloaded by the repository).
func (r *Resolver) comments(p graphql.ResolveParams) (interface{}, error) {
	id, err := parseID(p, "articleId")
	if err != nil {
		return nil, err
	}
	return r.Comment.ArticleComments(id)
}

// user returns a single user's public profile by ID. Sensitive fields
// (password/email) are excluded from the GraphQL type definition.
func (r *Resolver) user(p graphql.ResolveParams) (interface{}, error) {
	id, err := parseID(p, "id")
	if err != nil {
		return nil, err
	}
	return r.User.Get(id)
}

// feed returns the site RSS feed as a raw XML string.
func (r *Resolver) feed(_ graphql.ResolveParams) (interface{}, error) {
	return r.Article.GenerateFeed()
}

// ---------- Field resolvers for nested relations ----------
//
// 这些 resolver 处理两种 source 形式：
//   - 值类型 (models.Article)：来自 List 查询，items 是 []models.Article
//   - 指针类型 (*models.Article)：来自 Get/GetBySlug，返回的是 *models.Article
// 统一用 sourceArticle / sourceComment / sourceCategory 提取。

// sourceArticle 从 ResolveParams.Source 提取 *models.Article，兼容值/指针两种形式。
func sourceArticle(p graphql.ResolveParams) (models.Article, bool) {
	switch v := p.Source.(type) {
	case models.Article:
		return v, true
	case *models.Article:
		return *v, true
	}
	return models.Article{}, false
}

// sourceComment 从 ResolveParams.Source 提取 *models.Comment。
func sourceComment(p graphql.ResolveParams) (models.Comment, bool) {
	switch v := p.Source.(type) {
	case models.Comment:
		return v, true
	case *models.Comment:
		return *v, true
	}
	return models.Comment{}, false
}

// sourceCategory 从 ResolveParams.Source 提取 *models.Category。
func sourceCategory(p graphql.ResolveParams) (models.Category, bool) {
	switch v := p.Source.(type) {
	case models.Category:
		return v, true
	case *models.Category:
		return *v, true
	}
	return models.Category{}, false
}

// articleAuthor returns the article's author. The repository preloads Author
// on GetByID/GetPublishedBySlug/List, so this is a plain field access.
func (r *Resolver) articleAuthor(p graphql.ResolveParams) (interface{}, error) {
	article, ok := sourceArticle(p)
	if !ok {
		return nil, nil
	}
	return article.Author, nil
}

// articleCategory returns the article's category (nullable).
func (r *Resolver) articleCategory(p graphql.ResolveParams) (interface{}, error) {
	article, ok := sourceArticle(p)
	if !ok || article.Category == nil {
		return nil, nil
	}
	return *article.Category, nil
}

// articleTags returns the article's tags.
func (r *Resolver) articleTags(p graphql.ResolveParams) (interface{}, error) {
	article, ok := sourceArticle(p)
	if !ok {
		return nil, nil
	}
	return article.Tags, nil
}

// articleComments returns approved comments for the article via the comment
// service (comments are NOT preloaded on articles by default).
func (r *Resolver) articleComments(p graphql.ResolveParams) (interface{}, error) {
	article, ok := sourceArticle(p)
	if !ok {
		return nil, nil
	}
	return r.Comment.ArticleComments(article.ID)
}

// commentUser returns the comment's author user (nullable for guest comments).
func (r *Resolver) commentUser(p graphql.ResolveParams) (interface{}, error) {
	comment, ok := sourceComment(p)
	if !ok || comment.User == nil {
		return nil, nil
	}
	return *comment.User, nil
}

// commentChildren returns the comment's replies (preloaded by FindArticleComments).
func (r *Resolver) commentChildren(p graphql.ResolveParams) (interface{}, error) {
	comment, ok := sourceComment(p)
	if !ok {
		return nil, nil
	}
	return comment.Children, nil
}

// categoryParent returns the category's parent (nullable for root categories).
func (r *Resolver) categoryParent(p graphql.ResolveParams) (interface{}, error) {
	cat, ok := sourceCategory(p)
	if !ok || cat.Parent == nil {
		return nil, nil
	}
	return *cat.Parent, nil
}

// categoryChildren returns the category's child categories.
func (r *Resolver) categoryChildren(p graphql.ResolveParams) (interface{}, error) {
	cat, ok := sourceCategory(p)
	if !ok {
		return nil, nil
	}
	return cat.Children, nil
}

// ---------- helpers ----------

// parseID extracts a uint from the named argument in the resolve params.
func parseID(p graphql.ResolveParams, key string) (uint, error) {
	v, ok := p.Args[key]
	if !ok {
		return 0, nil
	}
	switch n := v.(type) {
	case int:
		return uint(n), nil
	case int64:
		return uint(n), nil
	case string:
		id, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			return 0, err
		}
		return uint(id), nil
	}
	return 0, nil
}

// strArg returns the named string argument or "".
func strArg(p graphql.ResolveParams, key string) string {
	if v, ok := p.Args[key].(string); ok {
		return v
	}
	return ""
}

// intArg returns the named int argument or 0.
func intArg(p graphql.ResolveParams, key string) int {
	if v, ok := p.Args[key].(int); ok {
		return v
	}
	return 0
}
