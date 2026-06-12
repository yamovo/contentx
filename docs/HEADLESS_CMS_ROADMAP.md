# VortexCMS → Headless CMS 转型路线图

> 目标：从"带前端的博客系统"转型为"API-first 内容平台"，对标 Strapi / Directus，但用 Go 实现，主打**高性能 + 单二进制部署**。

---

## 一、现状分析

### 当前架构

```
┌─────────────────────────────────────────────────┐
│  Vue 3 管理后台 (web/)                          │
│  Vue 3 博客前台 (web/)                          │
├─────────────────────────────────────────────────┤
│  Gin HTTP Server                                │
│  ├── Handler 层 (11 个 Handler)                 │
│  ├── Service 层 (业务逻辑)                      │
│  └── GORM (PostgreSQL / MySQL / SQLite)         │
└─────────────────────────────────────────────────┘
```

### 已有能力

| 能力 | 状态 | 说明 |
|------|------|------|
| 文章 CRUD | ✅ 完整 | 含版本历史、批量操作、定时发布 |
| 分类标签 | ✅ 完整 | 无限层级分类、标签合并 |
| 用户 RBAC | ✅ 完整 | 角色权限、登录锁定 |
| 媒体管理 | ✅ 完整 | 上传、缩略图、文件夹 |
| 评论系统 | ✅ 完整 | 多级嵌套、审核、垃圾过滤 |
| SEO | ✅ 完整 | Sitemap、Robots、301 重定向 |
| 菜单管理 | ✅ 完整 | 多级菜单、拖拽排序 |
| 插件/主题 | ⚠️ 基础 | 有模型，缺动态加载 |
| Webhook | ❌ 缺失 | 无 |
| GraphQL | ❌ 缺失 | 无 |
| 多语言内容 | ❌ 缺失 | 无 i18n |
| 内容类型自定义 | ❌ 缺失 | 只有 Article 一种 |

### 对标差距（vs Strapi v5）

| 能力 | Strapi | VortexCMS | 差距 |
|------|--------|-----------|------|
| Content Type Builder | ✅ 可视化建模 | ❌ 硬编码 | **核心差距** |
| REST API | ✅ 自动生成 | ⚠️ 手写 | 中 |
| GraphQL | ✅ 内置 | ❌ 无 | 中 |
| Webhook | ✅ | ❌ 无 | 中 |
| 国际化 (i18n) | ✅ | ❌ 无 | 中 |
| 版本/发布工作流 | ✅ Draft/Publish | ⚠️ 有状态但无工作流 | 小 |
| API Token | ✅ 细粒度 | ⚠️ 有 API Key 模型 | 小 |
| 插件市场 | ✅ 600+ | ❌ 无 | 大（生态） |
| 媒体管理 | ✅ S3/OSS | ⚠️ 仅本地 | 中 |
| 单二进制部署 | ❌ Node.js | ✅ Go | **我们赢** |
| 内存占用 | ~200MB+ | ~30MB | **我们赢** |
| 并发性能 | 中 | 高 | **我们赢** |

---

## 二、转型策略

### 核心定位

```
"给开发者的高性能 Headless CMS"
→ 不做可视化建模（打不过 Strapi）
→ 做：代码定义内容类型 + 自动生成 API + 极致性能
```

### 差异化卖点

1. **Go 单二进制** — 50MB 内存跑起来，Docker 镜像 < 30MB
2. **代码优先建模** — 用 Go struct 定义内容类型，编译期类型安全
3. **超高并发** — 适合内容分发、API 网关场景
4. **零依赖部署** — SQLite 模式无需数据库

---

## 三、阶段计划

### Phase 1：API 清理 & 文档（1-2 周）

> 目标：让现有 API 可被前端框架直接消费

#### 1.1 OpenAPI 文档自动生成

```
现状：无 API 文档
目标：Swagger UI 自动生成，前端可直接导入
```

**做法：接入 swaggo**
```bash
go get -u github.com/swaggo/swag/cmd/swag
go get -u github.com/swaggo/gin-swagger
```

在每个 Handler 加注释：
```go
// @Summary      获取文章列表
// @Description  分页获取文章，支持状态、分类、标签筛选
// @Tags         Articles
// @Accept       json
// @Produce      json
// @Param        page      query  int  false  "页码"  default(1)
// @Param        page_size query  int  false  "每页数量"  default(20)
// @Param        status    query  string  false  "状态"  Enums(draft,published)
// @Success      200  {object}  APIResponse{data=[]models.Article}
// @Security     BearerAuth
// @Router       /articles [get]
func (h *ArticleHandler) List(c *gin.Context) { ... }
```

**产出：**
- `GET /swagger/index.html` 在线文档
- `openapi.json` 可导入 Postman / Apifox

#### 1.2 统一 API 分页格式

```
现状：各接口返回格式不一致
目标：全部统一为
```
```json
{
  "code": 0,
  "message": "success",
  "data": [...],
  "meta": {
    "page": 1,
    "page_size": 20,
    "total": 100,
    "total_pages": 5,
    "has_next": true,
    "has_prev": false
  }
}
```

#### 1.3 API Token 细粒度权限

```
现状：有 API Key 模型但未完整接入
目标：支持创建多个 API Token，按模块授权
```

```go
// 示例：创建一个只读 Token
POST /api/v1/system/tokens
{
  "name": "Next.js 前端",
  "permissions": ["articles.read", "categories.read", "tags.read"],
  "expires_at": "2027-01-01"
}
```

**改动范围：**
- [apikey.go](internal/auth/apikey.go) — 扩展权限字段
- [middleware/apikey.go](internal/middleware/apikey.go) — 检查 Token 权限
- 新增 `handlers/token.go` — Token CRUD

---

### Phase 2：Headless 核心能力（3-4 周）

> 目标：实现 Headless CMS 的核心差异化功能

#### 2.1 自定义内容类型（Collection Type）

**这是 Headless CMS 的核心。**

当前只有 `Article` 一种类型。需要支持用户自定义内容结构。

**方案：数据库驱动的动态类型（不用代码生成）**

```sql
-- 内容类型定义
CREATE TABLE content_types (
  id          SERIAL PRIMARY KEY,
  uid         VARCHAR(64) UNIQUE NOT NULL,  -- 如 "product", "event"
  name        VARCHAR(128) NOT NULL,
  description TEXT,
  is_single   BOOLEAN DEFAULT FALSE,       -- 单例(如"关于我们") vs 集合
  draft_publish BOOLEAN DEFAULT TRUE,      -- 是否支持草稿/发布
  created_at  TIMESTAMP,
  updated_at  TIMESTAMP
);

-- 字段定义
CREATE TABLE content_fields (
  id            SERIAL PRIMARY KEY,
  content_type_id INTEGER REFERENCES content_types(id),
  name          VARCHAR(64) NOT NULL,       -- 字段名
  field_type    VARCHAR(32) NOT NULL,       -- text, rich_text, integer, float, boolean, date, media, relation, json, enum
  required      BOOLEAN DEFAULT FALSE,
  unique_field  BOOLEAN DEFAULT FALSE,
  default_value TEXT,
  options       JSONB,                      -- 枚举选项、关联配置等
  sort_order    INTEGER DEFAULT 0
);

-- 实际内容数据（EAV 模式）
CREATE TABLE content_entries (
  id              SERIAL PRIMARY KEY,
  content_type_id INTEGER REFERENCES content_types(id),
  document_id     VARCHAR(36) UNIQUE NOT NULL,  -- UUID
  status          VARCHAR(20) DEFAULT 'draft',   -- draft, published
  created_by_id   INTEGER REFERENCES users(id),
  updated_by_id   INTEGER REFERENCES users(id),
  published_at    TIMESTAMP,
  created_at      TIMESTAMP,
  updated_at      TIMESTAMP,
  locale          VARCHAR(10) DEFAULT 'en'
);

-- 字段值存储
CREATE TABLE content_entry_values (
  id          SERIAL PRIMARY KEY,
  entry_id    INTEGER REFERENCES content_entries(id) ON DELETE CASCADE,
  field_id    INTEGER REFERENCES content_fields(id),
  value_text  TEXT,
  value_int   BIGINT,
  value_float DOUBLE PRECISION,
  value_bool  BOOLEAN,
  value_json  JSONB,
  value_date  TIMESTAMP,
  UNIQUE(entry_id, field_id)
);
```

**API 自动生成：**

当用户创建了 `product` 类型后，自动暴露：
```
GET    /api/v1/content/products          # 列表
GET    /api/v1/content/products/:id      # 详情
POST   /api/v1/content/products          # 创建
PUT    /api/v1/content/products/:id      # 更新
DELETE /api/v1/content/products/:id      # 删除
POST   /api/v1/content/products/:id/publish   # 发布
POST   /api/v1/content/products/:id/unpublish # 取消发布
```

**改动范围：**
- 新增 `models/content_type.go` — 内容类型模型
- 新增 `models/content_entry.go` — 内容条目模型
- 新增 `services/content_service.go` — 动态 CRUD
- 新增 `handlers/content.go` — 动态路由注册
- 修改 `handlers/routes.go` — 启动时注册已有类型路由

#### 2.2 Webhook 系统

```
现状：无
目标：内容变更时通知外部系统
```

```sql
CREATE TABLE webhooks (
  id         SERIAL PRIMARY KEY,
  name       VARCHAR(128) NOT NULL,
  url        VARCHAR(512) NOT NULL,
  events     TEXT[] NOT NULL,           -- ["entry.create", "entry.update", "entry.delete", "entry.publish"]
  headers    JSONB,                     -- 自定义请求头
  is_active  BOOLEAN DEFAULT TRUE,
  secret     VARCHAR(128),             -- HMAC 签名密钥
  created_at TIMESTAMP
);

CREATE TABLE webhook_logs (
  id          SERIAL PRIMARY KEY,
  webhook_id  INTEGER REFERENCES webhooks(id),
  event       VARCHAR(64),
  payload     JSONB,
  response    INTEGER,                 -- HTTP 状态码
  duration    INTEGER,                 -- 耗时 ms
  success     BOOLEAN,
  created_at  TIMESTAMP
);
```

**事件列表：**
| 事件 | 触发时机 |
|------|----------|
| `entry.create` | 内容创建 |
| `entry.update` | 内容更新 |
| `entry.delete` | 内容删除 |
| `entry.publish` | 内容发布 |
| `entry.unpublish` | 内容取消发布 |
| `media.create` | 媒体上传 |
| `media.delete` | 媒体删除 |
| `comment.create` | 评论创建 |
| `user.create` | 用户注册 |

**改动范围：**
- 新增 `models/webhook.go`
- 新增 `services/webhook_service.go` — 异步触发
- 新增 `handlers/webhook.go` — Webhook CRUD
- 修改各 Service — 在关键操作后触发事件

#### 2.3 GraphQL 接口

```
现状：仅 REST
目标：REST + GraphQL 双模
```

**选型：gqlgen（Go 最成熟的 GraphQL 库）**

```bash
go get github.com/99designs/gqlgen
```

**Schema 示例：**
```graphql
type Query {
  articles(page: Int, pageSize: Int, filters: ArticleFilters): ArticleConnection!
  article(id: ID, slug: String): Article
  categories: [Category!]!
  contentEntry(type: String!, id: ID!): ContentEntry
  contentEntries(type: String!, page: Int, pageSize: Int): ContentEntryConnection!
}

type Mutation {
  createArticle(input: CreateArticleInput!): Article!
  updateArticle(id: ID!, input: UpdateArticleInput!): Article!
  deleteArticle(id: ID!): Boolean!
  publishArticle(id: ID!): Article!
}

type Article {
  id: ID!
  title: String!
  slug: String!
  content: String
  status: ArticleStatus!
  author: User!
  category: Category
  tags: [Tag!]!
  seo: SEO
  createdAt: DateTime!
  updatedAt: DateTime!
}
```

**改动范围：**
- 新增 `graphql/` 目录
- 新增 `handlers/graphql.go` — GraphQL 入口
- 复用现有 Service 层，不重复写业务逻辑

---

### Phase 3：生产化（2-3 周）

> 目标：可以上生产

#### 3.1 媒体存储抽象（S3/OSS）

```
现状：仅本地文件存储
目标：支持本地 / S3 / 阿里云 OSS / MinIO
```

**接口设计：**
```go
// StorageDriver 定义存储驱动接口
type StorageDriver interface {
    Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error)
    Delete(ctx context.Context, key string) error
    GetURL(key string) string
    GetSignedURL(key string, expiry time.Duration) string
}

// 本地存储
type LocalStorage struct { ... }

// S3 兼容存储（AWS S3 / MinIO / 阿里云 OSS）
type S3Storage struct {
    bucket   string
    endpoint string
    client   *s3.Client
}
```

**配置：**
```env
STORAGE_DRIVER=local          # local | s3
S3_ENDPOINT=                  # AWS 或 MinIO 地址
S3_BUCKET=vortexcms
S3_ACCESS_KEY=
S3_SECRET_KEY=
S3_REGION=us-east-1
S3_PUBLIC_URL=                # CDN 域名
```

**改动范围：**
- 新增 `internal/storage/` — 驱动接口 + 实现
- 修改 `services/media_service.go` — 使用 StorageDriver
- 修改 `config/config.go` — 新增存储配置

#### 3.2 缓存层

```
现状：无缓存
目标：热点内容内存缓存，可选 Redis
```

```go
type CacheDriver interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Flush(ctx context.Context) error
}

// 内存缓存（默认）
type MemoryCache struct {
    store *lru.Cache[string, cacheItem]
}

// Redis 缓存
type RedisCache struct {
    client *redis.Client
    prefix string
}
```

**缓存策略：**
| 内容 | TTL | 失效时机 |
|------|-----|----------|
| 公开文章 | 10 分钟 | 更新/发布时清除 |
| 分类列表 | 30 分钟 | 分类变更时清除 |
| 站点设置 | 1 小时 | 设置更新时清除 |
| API 文档 | 1 天 | 重启时清除 |

#### 3.3 数据库迁移系统

```
现状：AutoMigrate（仅适合开发）
目标：版本化迁移，支持回滚
```

已有的 [migrator.go](internal/database/migrator.go) 框架，需要补充：

```go
// 迁移文件示例
// migrations/002_add_content_types.up.sql
CREATE TABLE content_types (...);
CREATE TABLE content_fields (...);

// migrations/002_add_content_types.down.sql
DROP TABLE content_fields;
DROP TABLE content_types;
```

**命令：**
```bash
vortexcms migrate up          # 执行迁移
vortexcms migrate down        # 回滚
vortexcms migrate status      # 查看状态
vortexcms migrate create xxx  # 创建新迁移
```

#### 3.4 CLI 工具

```bash
# 安装
go install github.com/vortexcms/go-cms/cmd/vortexcms@latest

# 命令
vortexcms serve               # 启动服务
vortexcms migrate up          # 数据库迁移
vortexcms seed                # 种子数据
vortexcms createsuperuser     # 创建管理员
vortexcms export              # 导出内容
vortexcms import              # 导入内容
vortexcms health              # 健康检查
```

---

### Phase 4：生态建设（持续）

#### 4.1 SDK 发布

| 语言 | 包名 | 状态 |
|------|------|------|
| JavaScript/TypeScript | `@vortexcms/sdk` | 优先 |
| Go | `github.com/vortexcms/go-sdk` | 优先 |
| Python | `vortexcms-python` | 后续 |

**TypeScript SDK 示例：**
```typescript
import { VortexCMS } from '@vortexcms/sdk'

const cms = new VortexCMS({
  baseURL: 'https://cms.example.com/api/v1',
  token: 'your-api-token',
})

// 获取文章列表
const articles = await cms.articles.list({
  filters: { status: 'published' },
  populate: ['author', 'category', 'tags'],
  sort: 'createdAt:desc',
  pagination: { page: 1, pageSize: 10 },
})

// 获取自定义内容类型
const products = await cms.content('products').list({
  filters: { price: { $gte: 100 } },
})
```

#### 4.2 前端框架集成指南

| 框架 | 集成方式 | 优先级 |
|------|----------|--------|
| Next.js | `getServerSideProps` + SDK | 高 |
| Nuxt.js | `useAsyncData` + SDK | 高 |
| Astro | Content Collection + SDK | 中 |
| Vue 3 | `useVortexCMS()` composable | 中 |

#### 4.3 Docker 官方镜像

```dockerfile
FROM scratch
COPY vortexcms /usr/local/bin/
COPY .env /app/.env
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["vortexcms", "serve"]
```

镜像大小：< 30MB（vs Strapi ~500MB）

---

## 四、技术决策

### 为什么不用代码生成建模？

| 方案 | 优点 | 缺点 |
|------|------|------|
| **数据库驱动（我们选这个）** | 运行时动态、无需重启、可视化友好 | 性能略差、类型不安全 |
| 代码生成 | 编译期类型安全、性能好 | 需要重新编译部署、DX 差 |

Strapi 用的是数据库驱动，市场验证过了。

### 为什么不用 gRPC？

| 方案 | 优点 | 缺点 |
|------|------|------|
| **REST + GraphQL（我们选这个）** | 前端友好、生态成熟 | 无 |
| REST + gRPC | 性能好 | 前端不直接用、需要网关 |

Headless CMS 的消费者是前端应用，REST + GraphQL 是标准配置。

### 为什么选 gqlgen 而不是 graphql-go？

- gqlgen：代码生成、类型安全、Star 9.7k
- graphql-go：运行时反射、性能差

---

## 五、目录结构（目标）

```
go-cms/
├── cmd/
│   ├── server/main.go          # HTTP 服务入口
│   └── cli/main.go             # CLI 工具入口
├── internal/
│   ├── auth/                   # 认证授权（已有）
│   ├── config/                 # 配置（已有）
│   ├── database/               # 数据库（已有）
│   │   ├── migrations/         # 迁移文件
│   │   └── migrator.go
│   ├── models/                 # 数据模型（已有）
│   ├── handlers/               # HTTP 处理器（已有）
│   ├── middleware/              # 中间件（已有）
│   ├── services/               # 业务逻辑（已有）
│   ├── storage/                # 🆕 存储驱动
│   │   ├── driver.go           # 接口定义
│   │   ├── local.go            # 本地存储
│   │   └── s3.go               # S3 兼容存储
│   ├── cache/                  # 🆕 缓存层
│   │   ├── driver.go
│   │   ├── memory.go
│   │   └── redis.go
│   ├── webhook/                # 🆕 Webhook 引擎
│   │   ├── dispatcher.go
│   │   └── signer.go
│   └── content/                # 🆕 动态内容类型
│       ├── registry.go         # 类型注册
│       ├── schema.go           # Schema 管理
│       └── resolver.go         # 查询解析
├── graphql/                    # 🆕 GraphQL Schema
│   ├── schema.graphqls
│   └── generated.go
├── web/                        # 管理后台（保留但非核心）
├── sdk/
│   ├── typescript/             # 🆕 TS SDK
│   └── go/                     # 🆕 Go SDK
├── deploy/
│   ├── Dockerfile
│   └── docker-compose.yml
└── docs/
    ├── api/                    # 🆕 OpenAPI 文档
    └── guides/                 # 🆕 集成指南
```

---

## 六、里程碑 & 时间线

| 阶段 | 时间 | 交付物 | 优先级 |
|------|------|--------|--------|
| **Phase 1** | 第 1-2 周 | Swagger 文档 + 统一分页 + API Token | 🔴 必做 |
| **Phase 2.1** | 第 3-4 周 | 自定义内容类型 + 自动生成 API | 🔴 核心 |
| **Phase 2.2** | 第 5 周 | Webhook 系统 | 🟡 重要 |
| **Phase 2.3** | 第 6 周 | GraphQL 接口 | 🟡 重要 |
| **Phase 3.1** | 第 7 周 | S3/OSS 存储 | 🟡 重要 |
| **Phase 3.2** | 第 8 周 | 缓存层 | 🟢 优化 |
| **Phase 3.3** | 第 9 周 | 迁移系统 + CLI | 🟢 优化 |
| **Phase 4.1** | 第 10 周 | TypeScript SDK | 🟡 重要 |
| **Phase 4.2** | 第 11-12 周 | 集成指南 + Docker 镜像 | 🟢 优化 |

**总周期：约 3 个月（1 人全职）**

---

## 七、成功指标

| 指标 | 目标 | 衡量方式 |
|------|------|----------|
| API 响应时间 | < 50ms (P95) | 基准测试 |
| 内存占用 | < 50MB (1000 篇文章) | `docker stats` |
| Docker 镜像 | < 30MB | 构建产物 |
| 并发能力 | > 1000 req/s (SQLite) | wrk 压测 |
| API 覆盖率 | 100% CRUD | Swagger 验证 |
| TypeScript SDK | npm 周下载 > 100 | npm stats |

---

## 八、风险 & 应对

| 风险 | 概率 | 影响 | 应对 |
|------|------|------|------|
| 动态内容类型性能差 | 中 | 高 | 加缓存、预编译查询 |
| GraphQL 复杂度过高 | 中 | 中 | 先做 REST，GraphQL 按需加 |
| 生态建设慢 | 高 | 中 | 先做 TS SDK，其他社区贡献 |
| Strapi 出 Go 版 | 低 | 高 | 强化差异化（性能、部署） |

---

## 九、不做清单

明确**不做的事**，防止范围蔓延：

- ❌ 可视化内容类型建模（UI Builder）— 打不过 Strapi
- ❌ 插件市场 — 生态需要时间，先做核心
- ❌ 多租户 — 复杂度高，市场小
- ❌ 电商功能 — 专注内容管理
- ❌ WYSIWYG 编辑器 — 用现有的（TinyMCE/MDX）
- ❌ 可视化页面构建 — 不是 CMS 的事

---

## 十、下一步行动

**立即可做（本周）：**

1. 接入 swaggo，生成第一版 API 文档
2. 统一所有 API 响应格式
3. 创建 `docs/api/` 目录

**第一个 PR：**
```bash
go get -u github.com/swaggo/swag/cmd/swag
go get -u github.com/swaggo/gin-swagger
```

然后在 [main.go](cmd/server/main.go) 加：
```go
import _ "github.com/vortexcms/go-cms/docs/api" // swagger docs
import ginSwagger "github.com/swaggo/gin-swagger"
import swaggerFiles "github.com/swaggo/swagger/files"

r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
```
