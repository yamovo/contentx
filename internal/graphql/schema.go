package graphql

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/graphql-go/graphql"
	"github.com/yamovo/contentx/internal/services"
)

// NewSchema builds the read-only GraphQL schema wired to the given services.
// The schema exposes headless-CMS consumers a single endpoint for fetching
// articles, categories, tags, comments, users, and the RSS feed.
//
// 循环引用（Article↔Comment、Category↔Category）通过 FieldsThunk 延迟
// 求值解决：类型对象的 Fields 字段是一个闭包，在所有类型都创建完成后
// 才被 graphql-go 调用，此时闭包内引用的类型变量已就绪。
func NewSchema(svc Services) (graphql.Schema, error) {
	r := &Resolver{
		Article:  svc.Article,
		Category: svc.Category,
		Tag:      svc.Tag,
		Comment:  svc.Comment,
		User:     svc.User,
	}

	// ---------- Object types ----------
	// 注意：User 类型不暴露 password / email 等敏感字段，仅返回公开资料。
	userType := graphql.NewObject(graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"id":          idField(),
			"username":    nonNullStringField(),
			"displayName": stringField(),
			"avatar":      stringField(),
			"bio":         stringField(),
			"website":     stringField(),
			"status":      stringField(),
			"loginCount":  intField(),
			"createdAt":   stringField(),
		},
	})

	tagType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Tag",
		Fields: graphql.Fields{
			"id":        idField(),
			"name":      nonNullStringField(),
			"slug":      nonNullStringField(),
			"count":     intField(),
			"color":     stringField(),
			"createdAt": stringField(),
			"updatedAt": stringField(),
		},
	})

	// Category 自引用（parent / children），用 FieldsThunk。
	var categoryType *graphql.Object
	categoryType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Category",
		Fields: (graphql.FieldsThunk)(func() graphql.Fields {
			return graphql.Fields{
				"id":          idField(),
				"name":        nonNullStringField(),
				"slug":        nonNullStringField(),
				"description": stringField(),
				"image":       stringField(),
				"color":       stringField(),
				"sortOrder":   intField(),
				"postCount":   intField(),
				"isActive":    boolField(),
				"parentId":    idField(),
				"createdAt":   stringField(),
				"updatedAt":   stringField(),
				"parent": &graphql.Field{
					Type:    categoryType,
					Resolve: r.categoryParent,
				},
				"children": &graphql.Field{
					Type:    graphql.NewList(graphql.NewNonNull(categoryType)),
					Resolve: r.categoryChildren,
				},
			}
		}),
	})

	// Article ↔ Comment 循环引用，两者都用 FieldsThunk。
	var articleType, commentType *graphql.Object

	commentType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Comment",
		Fields: (graphql.FieldsThunk)(func() graphql.Fields {
			return graphql.Fields{
				"id":         idField(),
				"articleId":  intField(),
				"authorName": stringField(),
				"authorUrl":  stringField(),
				"content":    nonNullStringField(),
				"status":     stringField(),
				"agent":      stringField(),
				"depth":      intField(),
				"likeCount":  intField(),
				"isSticky":   boolField(),
				"createdAt":  stringField(),
				"updatedAt":  stringField(),
				"user": &graphql.Field{
					Type:    userType,
					Resolve: r.commentUser,
				},
				"children": &graphql.Field{
					Type:    graphql.NewList(graphql.NewNonNull(commentType)),
					Resolve: r.commentChildren,
				},
			}
		}),
	})

	articleType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Article",
		Fields: (graphql.FieldsThunk)(func() graphql.Fields {
			return graphql.Fields{
				"id":            idField(),
				"title":         nonNullStringField(),
				"slug":          nonNullStringField(),
				"content":       stringField(),
				"excerpt":       stringField(),
				"status":        stringField(),
				"postType":      stringField(),
				"format":        stringField(),
				"visibility":    stringField(),
				"isPinned":      boolField(),
				"isFeatured":    boolField(),
				"allowComment":  boolField(),
				"viewCount":     intField(),
				"likeCount":     intField(),
				"wordCount":     intField(),
				"readingTime":   intField(),
				"publishedAt":   stringField(),
				"scheduledAt":   stringField(),
				"featuredImage": stringField(),
				"metaTitle":     stringField(),
				"metaDesc":      stringField(),
				"metaKeywords":  stringField(),
				"canonicalUrl":  stringField(),
				"template":      stringField(),
				"sortOrder":     intField(),
				"version":       intField(),
				"commentCount":  intField(),
				"createdAt":     stringField(),
				"updatedAt":     stringField(),
				"author": &graphql.Field{
					Type:    graphql.NewNonNull(userType),
					Resolve: r.articleAuthor,
				},
				"category": &graphql.Field{
					Type:    categoryType,
					Resolve: r.articleCategory,
				},
				"tags": &graphql.Field{
					Type:    graphql.NewList(graphql.NewNonNull(tagType)),
					Resolve: r.articleTags,
				},
				"comments": &graphql.Field{
					Type:    graphql.NewList(graphql.NewNonNull(commentType)),
					Resolve: r.articleComments,
				},
			}
		}),
	})

	articleConnectionType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ArticleConnection",
		Fields: graphql.Fields{
			"items": &graphql.Field{
				Type: graphql.NewList(graphql.NewNonNull(articleType)),
			},
			"page":       nonNullIntField(),
			"pageSize":   nonNullIntField(),
			"total":      nonNullIntField(),
			"totalPages": nonNullIntField(),
			"hasNext":    nonNullBoolField(),
			"hasPrev":    nonNullBoolField(),
		},
	})

	// ---------- Search types ----------
	// Exposed via the `search` Query field so headless consumers can run
	// full-text queries through GraphQL (mirrors REST /api/v1/search).
	searchHitType := graphql.NewObject(graphql.ObjectConfig{
		Name: "SearchHit",
		Fields: graphql.Fields{
			"id":          nonNullStringField(),
			"type":        stringField(),
			"title":       stringField(),
			"excerpt":     stringField(),
			"slug":        stringField(),
			"score":       floatField(),
			"highlight":   stringField(),
			"locale":      stringField(),
			"authorId":    stringField(),
			"authorName":  stringField(),
			"publishedAt": stringField(),
		},
	})

	searchResultType := graphql.NewObject(graphql.ObjectConfig{
		Name: "SearchResult",
		Fields: graphql.Fields{
			"hits": &graphql.Field{
				Type: graphql.NewList(graphql.NewNonNull(searchHitType)),
			},
			"total":      nonNullIntField(),
			"page":       nonNullIntField(),
			"pageSize":   nonNullIntField(),
			"totalPages": nonNullIntField(),
			"took":       stringField(),
		},
	})

	// ---------- Query root ----------
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"article": &graphql.Field{
				Type: articleType,
				Args: graphql.FieldConfigArgument{
					"id": nonNullIDArg(),
				},
				Resolve: r.article,
			},
			"articleBySlug": &graphql.Field{
				Type: articleType,
				Args: graphql.FieldConfigArgument{
					"slug": nonNullStringArg(),
				},
				Resolve: r.articleBySlug,
			},
			"articles": &graphql.Field{
				Type: graphql.NewNonNull(articleConnectionType),
				Args: graphql.FieldConfigArgument{
					"page":       intArgConfig(),
					"pageSize":   intArgConfig(),
					"status":     stringArgConfig(),
					"postType":   stringArgConfig(),
					"categoryId": stringArgConfig(),
					"tagSlug":    stringArgConfig(),
					"search":     stringArgConfig(),
					"sort":       stringArgConfig(),
					"authorId":   stringArgConfig(),
				},
				Resolve: r.articles,
			},
			"category": &graphql.Field{
				Type: categoryType,
				Args: graphql.FieldConfigArgument{
					"id": nonNullIDArg(),
				},
				Resolve: r.category,
			},
			"categories": &graphql.Field{
				Type:    graphql.NewList(graphql.NewNonNull(categoryType)),
				Resolve: r.categories,
			},
			"tag": &graphql.Field{
				Type: tagType,
				Args: graphql.FieldConfigArgument{
					"id": nonNullIDArg(),
				},
				Resolve: r.tag,
			},
			"tags": &graphql.Field{
				Type:    graphql.NewList(graphql.NewNonNull(tagType)),
				Resolve: r.tags,
			},
			"comments": &graphql.Field{
				Type: graphql.NewList(graphql.NewNonNull(commentType)),
				Args: graphql.FieldConfigArgument{
					"articleId": nonNullIDArg(),
				},
				Resolve: r.comments,
			},
			"user": &graphql.Field{
				Type: userType,
				Args: graphql.FieldConfigArgument{
					"id": nonNullIDArg(),
				},
				Resolve: r.user,
			},
			"feed": &graphql.Field{
				Type:    graphql.NewNonNull(graphql.String),
				Resolve: r.feed,
			},
			"search": &graphql.Field{
				Type: searchResultType,
				Args: graphql.FieldConfigArgument{
					"q":        nonNullStringArg(),
					"type":     stringArgConfig(),
					"locale":   stringArgConfig(),
					"page":     intArgConfig(),
					"pageSize": intArgConfig(),
				},
				Resolve: r.search,
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
}

// Services bundles the service pointers needed to construct the schema.
type Services struct {
	Article  *services.ArticleService
	Category *services.CategoryService
	Tag      *services.TagService
	Comment  *services.CommentService
	User     *services.UserService
}

// ---------- HTTP handler (Gin) ----------

// Handler returns a gin.HandlerFunc that executes GraphQL queries against the
// provided schema. It supports both POST (application/json with {query,variables,
// operationName}) and GET (?query=...) — the latter is convenient for
// unauthenticated read queries during development.
//
// The handler does NOT enforce authentication itself; callers are expected to
// wrap the route with whatever middleware they need (the public route uses no
// auth, a protected route could reuse middleware.AuthMiddleware).
func Handler(schema graphql.Schema) gin.HandlerFunc {
	return func(c *gin.Context) {
		query, variables, operationName, err := extractQuery(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"errors": []map[string]string{
				{"message": err.Error()},
			}})
			return
		}
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"errors": []map[string]string{
				{"message": "missing 'query' parameter"},
			}})
			return
		}

		result := graphql.Do(graphql.Params{
			Schema:         schema,
			RequestString:  query,
			VariableValues: variables,
			OperationName:  operationName,
			RootObject:     nil,
		})

		// GraphQL 规范要求即使有错误也返回 200（错误在 body 里）；
		// 但完全没有数据时返回 4xx 便于客户端区分“彻底失败”。
		status := http.StatusOK
		if result.HasErrors() && result.Data == nil {
			status = http.StatusBadRequest
		}
		c.JSON(status, result)
	}
}

// extractQuery pulls the GraphQL query string + variables from either a POST
// body (JSON) or GET query params.
func extractQuery(c *gin.Context) (query string, variables map[string]interface{}, operationName string, err error) {
	switch c.Request.Method {
	case http.MethodGet:
		query = c.Query("query")
		if op := c.Query("operationName"); op != "" {
			operationName = op
		}
		if vars := c.Query("variables"); vars != "" {
			if err = json.Unmarshal([]byte(vars), &variables); err != nil {
				return "", nil, "", err
			}
		}
		return query, variables, operationName, nil
	default:
		body, readErr := io.ReadAll(c.Request.Body)
		if readErr != nil {
			return "", nil, "", readErr
		}
		// 允许 query 是原始字符串（非 JSON），便于 curl 直接发 schema 文本。
		// 启发式：尝试 JSON 解析。
		//   - 解析成功 → 用 payload 里的 query（即使为空，由上层报 missing query）
		//   - 解析失败 → 把整个 body 当作原始 query 字符串
		trimmed := strings.TrimSpace(string(body))
		if strings.HasPrefix(trimmed, "{") {
			var payload struct {
				Query         string                 `json:"query"`
				Variables     map[string]interface{} `json:"variables"`
				OperationName string                 `json:"operationName"`
			}
			if jsonErr := json.Unmarshal(body, &payload); jsonErr == nil {
				return payload.Query, payload.Variables, payload.OperationName, nil
			}
			// JSON 解析失败 → 退回原始字符串（如 `{ tags { name } }`）
		}
		return trimmed, nil, "", nil
	}
}

// ---------- Schema field/arg helpers ----------
//
// 这些小工厂函数让上面的类型定义保持紧凑，避免每次重复写
// &graphql.Field{Type: graphql.String} 这样的样板。

func idField() *graphql.Field {
	return &graphql.Field{Type: graphql.ID}
}

func stringField() *graphql.Field {
	return &graphql.Field{Type: graphql.String}
}

func nonNullStringField() *graphql.Field {
	return &graphql.Field{Type: graphql.NewNonNull(graphql.String)}
}

func intField() *graphql.Field {
	return &graphql.Field{Type: graphql.Int}
}

func nonNullIntField() *graphql.Field {
	return &graphql.Field{Type: graphql.NewNonNull(graphql.Int)}
}

func floatField() *graphql.Field {
	return &graphql.Field{Type: graphql.Float}
}

func boolField() *graphql.Field {
	return &graphql.Field{Type: graphql.Boolean}
}

func nonNullBoolField() *graphql.Field {
	return &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)}
}

func nonNullIDArg() *graphql.ArgumentConfig {
	return &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}
}

func nonNullStringArg() *graphql.ArgumentConfig {
	return &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}
}

func stringArgConfig() *graphql.ArgumentConfig {
	return &graphql.ArgumentConfig{Type: graphql.String}
}

func intArgConfig() *graphql.ArgumentConfig {
	return &graphql.ArgumentConfig{Type: graphql.Int}
}
