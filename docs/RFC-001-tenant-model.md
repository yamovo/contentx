# RFC-001：多租户数据模型设计

| 项 | 值 |
|---|---|
| 状态 | **已实施（多租户安全收口完成）** |
| 作者 | ContentX 维护者 |
| 日期 | 2026-08-07 |
| 关联 | [ROADMAP §3 当前里程碑](./ROADMAP.md)、[STATUS §1 发布状态](./STATUS.md) |
| 目标版本 | v1.5.0（未排期） |
| 实施状态 | PR-1（迁移 008/009）、PR-2（租户上下文）、PR-3（Article 全链路租户化）、PR-4（Content/Webhook/Analytics/Token/评论/审计全量推广 + 横切隔离）已完成（2026-08-29，含 tenant A/B 攻击矩阵远程验证）；PR-5 拆分执行：后端部分（`/admin/tenants` CRUD + 成员管理 + 最后管理员保护 + `tenants.read`/`tenants.manage` 平台权限 + 可靠审计）已于 2026-08-29 交付，前端部分（租户管理页、平台管理员切换器、切换即清空查询缓存、X-Tenant-ID 注入）已于同日交付；配额计量经审查暂缓（触发条件见 [ROADMAP §3.3](./ROADMAP.md)）；PR-6 的多租户备份演练已在 v1.4.0 完成，OpenAPI/文档同步持续进行 |

## 1. 背景与动机

ContentX v1.4.0 为单租户系统。当前里程碑（多租户基础）要求引入租户隔离，使同一部署可以为多个组织提供隔离的内容空间。本文档定义：

1. 租户模型与数据归属规则；
2. 租户上下文在认证、服务与仓储层的传递方式；
3. 数据库迁移方案（含现有数据回填）；
4. 缓存、搜索、Webhook 等横切能力的隔离边界；
5. 分阶段实施步骤与退出条件对照。

代码事实（截至 2026-08-07）：

- 全项目**不存在任何 tenant 概念**：无 `TenantID` 字段、无租户上下文 key、缓存与搜索索引均无租户维度。
- 归属仅通过 `AuthorID`/`UploaderID`/`EditorID`/`CreatedByID`/`UserID` 表达（见 models 清单 §6）。
- 身份链路：JWT Claims（`UserID/Username/Email/RoleSlug/DisplayName/TokenUse`，`internal/auth/jwt.go:27-35`）→ `AuthMiddleware` 解析后写入 `c.Set(ContextKeyUser/ContextKeyClaims)`（`internal/middleware/auth.go:17-21,194-195`）→ handler 经 `GetCurrentUser(c)` 取出 → **显式传 `userID uint` 给 service**（如 `ArticleService.Create(req, userID)`，`internal/services/article_service.go:404`）。
- Repository 模式：`New<X>Repository(db *gorm.DB) <X>Repository`，未导出实现持 `*gorm.DB`（`internal/repository/repository.go:8-14`）。
- 迁移风格：`Migration{Version, Description, Up, Down}`，`init()` 注册（`registry.go:8-14`），`gorm.Migrator` API 为主（`007_add_article_version.go:16-31`）。

## 2. 目标

- 所有业务数据表具有明确租户归属或全局归属。
- REST、GraphQL、Webhook、MCP、缓存、搜索和定时任务不能绕过租户边界。
- 跨租户读取、更新、删除和标识符猜测均有拒绝测试。
- 数据迁移、备份和恢复支持租户字段并有升级验证。
- 现有单租户数据无损迁入默认租户，公开行为保持向后兼容。

## 3. 非目标（本 RFC 范围外）

- 计费、SSO/OIDC、SLA、合规认证（仍属 PRD 非目标）。
- 跨租户数据迁移工具（租户间搬移内容）。
- 租户级自定义域名绑定与多站点路由（作为候选方向，另行评估）。
- 配额与用量计量的完整实现（本文档仅给出 schema 草案，见 §8，实施排期见 §9 PR-5）。

## 4. 设计决策

### 4.1 隔离模型选型

| 方案 | 优点 | 缺点 | 结论 |
|---|---|---|---|
| 独立数据库 | 隔离最强 | 迁移/备份/连接管理×N；SQLite 不适用 | ✗ |
| 独立 schema（PostgreSQL） | 隔离较强、可迁移 | MySQL/SQLite 不支持或成本高；跨 schema 查询复杂 | ✗ |
| **共享表 + `tenant_id` 列** | 迁移成本低；三种数据库一致；备份恢复沿用现有 pg_dump 全库流程 | 需在 Repository/Service 层强制注入租户条件；唯一约束需复合化 | **✓ 采用** |

采用共享表方案。隔离正确性由**仓储层强制注入 + 服务层复核 + 跨租户拒绝测试**三层保证（§5、§7）。

### 4.2 租户身份与上下文注入

- **数据模型**：新增 `tenants` 表；业务表增加 `tenant_id`（`NOT NULL`，默认回填 `1` = default 租户）。
- **JWT Claims**：`AccessToken` claims 增加 `TenantID uint`（签发时绑定当前租户）。RefreshToken 轮换时重发。
- **中间件**：
  - `AuthMiddleware`：从 claims 解析 `TenantID` → `c.Set(ContextKeyTenant, tenantID)`。
  - `APIKeyMiddleware`：`api_tokens` 增加 `tenant_id`（可空 = 使用创建者默认租户），鉴权时写入同一 context key。
  - 平台管理员（`admin` 角色）可通过 `X-Tenant-ID` 请求头覆盖租户上下文（用于管理端切换），普通用户忽略该头。
  - 新增 `GetCurrentTenant(c) (uint, error)` 与现有 `GetCurrentUser(c)`（`internal/middleware/auth.go:364-374`）并列。
- **Service 层**：与 `userID` 相同模式——查询/写入方法**显式接收 `tenantID uint` 参数**（如 `ArticleService.List(filter, tenantID)`），不从 context 自行解析。保持现有"service 不依赖 gin context"的约束。
- **公开接口（无需认证）**：固定使用 default 租户（`tenant_id=1`），现有公开行为不变。
- **MCP**：stdio 模式 = default 租户；HTTP 模式从 API Token 身份解析租户，写工具校验租户与权限。

### 4.3 数据归属规则

**租户级表（17 张加列）**——加 `tenant_id` 列：

`articles`、`categories`、`tags`、`comments`、`media`、`revisions`、`custom_fields`、`menus`、`menu_items`、`seo_settings`、`redirect_rules`、`webhooks`、`webhook_logs`、`webhook_deliveries`、`content_types`、`content_fields`、`content_entries`

> 实现注记：`article_tags`（many2many 连接表）经 `articles.tenant_id` 隐式归属，不单独设列；`sitemap_entries` 为预留模型（未注册 `AllModels`、无持久化），成为真实表时再补列。

**全局表（12 张）**——不隔离，但按需记录租户上下文：

| 表 | 处理方式 |
|---|---|
| `users` / `roles` / `permissions` / `role_permissions` | 保持全局。用户为平台级实体，租户关系经 `tenant_memberships` 表达 |
| `api_tokens` | 加 `tenant_id`（可空，NULL = 创建者默认租户） |
| `user_totp` | 全局（跟随用户） |
| `site_settings` | 加 `tenant_id`（可空，NULL = 全局默认；非 NULL = 租户覆盖） |
| `plugins` / `theme_configs` | 全局；租户级主题/插件开关由 `tenant_settings` 表达（PR-5） |
| `activity_logs` | 加 `tenant_id`（可空）；平台管理员可见全部，租户管理员仅见本租户 |
| `page_views` | 加 `tenant_id`（可空，公开流量归 default 租户） |
| `schema_migrations` | 全局（迁移表本身） |

**新增表**：

```go
// tenants
type Tenant struct {
    BaseModel
    Name        string `gorm:"not null"`
    Slug        string `gorm:"uniqueIndex;not null"`
    Status      string `gorm:"index;default:active"` // active | suspended
    MaxUsers    int    `gorm:"default:0"`             // 0 = 不限（配额预留，PR-5 启用）
}

// tenant_memberships —— 用户↔租户归属（含租户内角色）
type TenantMembership struct {
    BaseModel
    TenantID uint   `gorm:"uniqueIndex:idx_tenant_user;not null"`
    UserID   uint   `gorm:"uniqueIndex:idx_tenant_user;not null"`
    RoleSlug string `gorm:"not null;default:member"` // member | editor | admin
}
```

### 4.4 唯一性约束复合化

现有全局唯一索引在共享表下会跨租户冲突，需改为 `(tenant_id, ...)` 复合唯一。受影响清单（来自 models 定义）：

| 表 | 现唯一索引 | 改为 |
|---|---|---|
| `articles` | `slug`（models.go:112） | `(tenant_id, slug)` |
| `categories` | `name`、`slug`（models.go:189-190） | `(tenant_id, name)`、`(tenant_id, slug)` |
| `tags` | `name`、`slug`（models.go:208-209） | `(tenant_id, name)`、`(tenant_id, slug)` |
| `menus` | `slug`（models.go:287） | `(tenant_id, slug)` |
| `redirect_rules` | `from_path`（models.go:344） | `(tenant_id, from_path)` |
| `seo_settings` | `idx_seo_entity (entity_type, entity_id)`（models.go:331） | `(tenant_id, entity_type, entity_id)` |
| `site_settings` | `key`（models.go:315，NULL 租户行不参与唯一约束，全局+每租户各一份） | `(tenant_id, key)` |
| `content_types` | `uid`（content_type.go:10） | `(tenant_id, uid)` |
| `content_entries` | `document_id`（content_entry.go:14） | `(tenant_id, document_id)` |

`users.username/email` **保持全局唯一**（用户是平台级实体，不随租户重复）。

### 4.5 迁移方案（008）

采用现有迁移风格（`gorm.Migrator` + `init()` 注册 + `HasColumn`/`HasIndex` 幂等守卫，参照 `007_add_article_version.go:16-31`）：

**Up**（顺序执行）：

1. `AutoMigrate` 新建 `tenants`、`tenant_memberships`（含索引）；插入 default 租户行（`id=1, slug='default'`）。
2. 对 17 张租户级表：`AddColumn("tenant_id", BIGINT)`（先 `HasColumn` 守卫），`UpdateColumn("tenant_id", 1)` 回填，再 `CreateIndex("idx_<table>_tenant")`。
3. 复合唯一化：`DropIndex` 旧唯一索引 → `CreateIndex` 新 `(tenant_id, ...)` 复合唯一（§4.4 清单）。
4. `activity_logs`/`page_views`/`site_settings`/`api_tokens` 加可空 `tenant_id` + 索引。
5. 将现有非系统用户按默认租户写入 `tenant_memberships`（`role_slug` 取用户当前角色）。

**Down**（数据破坏性，遵循 006/007 先例需备份与停写）：

1. `DropIndex` 复合唯一 → 恢复旧唯一索引。
2. `DropColumn("tenant_id")`（全部表）。
3. `DropTable` `tenant_memberships`、`tenants`（连同其数据）。

**验证**：空库 v1→v8、含数据升级、v8→v7→v8 回滚、回填断言（每张业务表 `tenant_id=1` 行数 = 升级前行数）。

## 5. 隔离边界与安全

- **仓储层强制注入**：所有租户级表的 Repository 查询方法增加 `tenantID` 条件（`WHERE tenant_id = ?`），**不允许查询方法在无租户条件下执行**（List/Get/Count 等全部覆盖）。写操作（Create）由 service 统一写入当前租户。
- **服务层复核**：Service 在 `Update/Delete/RestoreRevision/Publish` 等按 ID 操作时，先以 `(id, tenantID)` 定位记录——跨租户 ID 命中返回 `404 NOT_FOUND`（不泄露存在性），与现有乐观锁/归属错误语义（`errs.ErrForbidden`）一致。
- **标识符猜测防护**：`GetByID` 类跨租户访问一律 404；跨租户写操作（如用他人租户的 `document_id` 更新条目）在 service 层拒绝。
- **定时任务**：`PublishScheduler`、Webhook worker、备份调度器按租户遍历执行，不得使用全局无租户查询。
- **审计**：`activity_logs` 记录 `tenant_id`，租户管理员查询强制过滤。
- **限流/配额**（PR-5）：`LIMITS_API_RATE` 扩展为按租户 + 接口维度计数（Redis key 含 `tenant:{id}`）。

## 6. 横切能力隔离

| 能力 | 现状 | 改造 |
|---|---|---|
| 缓存 | key `articles:list:v{gen}:{hash}`、`articles:id:{id}`（article_cache.go:32-37,102-104） | 前缀加 `t{tenant}:`，如 `articles:t1:list:...`；世代号可保持全局（key 已含租户，天然隔离） |
| 搜索 | `SearchDocument` Type∈article/page/content_entry，docKey=`type:id`（search_builtin.go:66） | `SearchDocument` 加 `TenantID`；docKey=`t{tenant}:{type}:{id}`；内置倒排索引按租户分区；公开搜索=default 租户 |
| Webhook | 持久化投递 `webhook_deliveries`（webhook.go:48-61） | 表加 `tenant_id`；worker 认领与重试逻辑不变（投递行自带租户），管理接口按租户过滤 |
| 备份/恢复 | pg_dump 全库 + CLI `--restore`（SOP §6） | 全库 SQL 天然包含 tenants 表与数据；恢复演练增加"租户数据完整性核对"；恢复后定向缓存清理 key 同步租户化 |
| GraphQL | 公开只读（depth/complexity 限制已有） | 查询固定 default 租户；不暴露租户切换 |

## 7. 测试策略

- **迁移测试**：空库 v1→v8（表/列/复合唯一齐全）；v8→v7→v8 回滚（遵循 006/007 数据破坏性先例，回滚前备份）；现有数据回填 default 租户断言。
- **Repository 层拒绝测试**：tenant A 的数据在 tenant B 的 List/Get/Update/Delete/Count 下全部不可见或 404；标识符猜测返回 404 而非 403/500。
- **Service 层**：跨租户更新/发布/恢复修订/批量操作拒绝；租户切换（X-Tenant-ID 覆盖）仅平台管理员生效。
- **缓存隔离**：同 filter 不同租户不互相命中；失效不越界。
- **搜索隔离**：重建索引后租户 A 搜不到租户 B 内容。
- **Webhook**：租户 A 的投递不触发租户 B 的 webhook；worker 崩溃复投跨租户正确。
- **备份恢复演练**：含多租户数据 + 配额计数，恢复后逐租户核对行数。

## 8. 配额与用量计量（schema 草案，PR-5 实施）

```go
type UsageCounter struct {          // 按租户+维度计数（Redis 加速，DB 持久化）
    TenantID   uint   `gorm:"uniqueIndex:idx_usage;not null"`
    Dimension  string `gorm:"uniqueIndex:idx_usage;not null"` // articles|media|api_requests|webhooks|...
    PeriodKey  string `gorm:"uniqueIndex:idx_usage;not null"` // 2026-08（月）
    Used       uint64
}

type UsageQuota struct {            // 租户配额定义
    TenantID  uint   `gorm:"uniqueIndex:idx_quota;not null"`
    Dimension string `gorm:"uniqueIndex:idx_quota;not null"`
    Limit     uint64 // 0 = 不限制
}
```

## 9. 实施步骤（映射 ROADMAP §3）

| PR | 内容 | ROADMAP 步骤 | 完成标志 |
|---|---|---|---|
| PR-1 | 迁移 008 + 009：`tenants`、`tenant_memberships`；17 张业务表 `tenant_id` 列 + 默认租户回填 + 复合唯一索引；4 张全局表可空列；models 更新 | 1（租户模型与迁移策略） | 空库/升级库迁移验证；回填断言 |
| PR-2 | 租户上下文：Claims + 中间件 + `GetCurrentTenant` + 公开接口默认租户 + `X-Tenant-ID` | 1（租户上下文） | 认证测试全绿 |
| PR-3 | Repository/Service 注入 `tenantID` + 跨租户拒绝测试 | 2、3（仓储注入与隔离测试） | 拒绝测试全绿；`go test ./...` 通过 |
| PR-4 | 缓存 key 租户化、搜索分区、Webhook 租户化 | 3（横切隔离） | 隔离测试全绿 |
| PR-5 | 租户管理 REST API（`/api/v1/admin/tenants`、members）+ 前端租户页与切换器 + 配额计量（§8） | 4、5（成员/角色管理、配额计量） | E2E + 配额测试通过 |
| PR-6 | 备份恢复演练（多租户）、OpenAPI 重新生成、STATUS/ROADMAP/SOP/CHANGELOG 同步 | 收尾 | 演练报告归档 |

## 10. 退出条件对照（ROADMAP §3）

| 退出条件 | 对应章节 |
|---|---|
| 所有业务数据表具有明确租户归属或全局归属 | §4.3 |
| REST、GraphQL、Webhook、MCP、缓存、搜索和定时任务不能绕过租户边界 | §4.2、§5、§6 |
| 跨租户读取、更新、删除和标识符猜测均有拒绝测试 | §7 |
| 数据迁移、备份和恢复支持租户字段并有升级验证 | §4.5、§7、§9 PR-6 |

## 11. 开放问题（评审时确认）

1. **用户模型**：用户全局唯一 + `tenant_memberships`（本 RFC 推荐）vs 每租户用户副本？前者保持 `users.username` 全局唯一，跨租户协作简单；后者隔离更强但用户管理复杂。
2. **API Token 租户绑定**：`api_tokens.tenant_id` 可空（跟随创建者默认租户）是否足够？是否需要"一 token 多租户"权限？
3. **审计日志归属**：`activity_logs` 建议租户级 + 平台管理员全量可见，是否接受平台可见性？
4. **公开站点多租户**：公开读取固定 default 租户是否满足近期需求？域名→租户映射是否应提前设计（候选方向，不阻塞）。
5. **迁移 008 大表 DDL**：MySQL 对 `articles` 等大表加列与重建唯一索引的锁表窗口，是否需要在 SOP 补充在线 DDL 说明？

## 12. 附录：表归属总表

**租户级（17 张加列）**：articles、categories、tags、comments、media、revisions、custom_fields、menus、menu_items、seo_settings、redirect_rules、webhooks、webhook_logs、webhook_deliveries、content_types、content_fields、content_entries（`article_tags` 经 `articles.tenant_id` 隐式归属；`sitemap_entries` 为预留模型，暂不设列）

**全局（12 张）**：users、roles、permissions、role_permissions、user_totp、api_tokens、site_settings、plugins、theme_configs、activity_logs、page_views、schema_migrations

**新增（2 张）**：tenants、tenant_memberships
