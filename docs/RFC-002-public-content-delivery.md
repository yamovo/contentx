# RFC-002：公开动态内容交付契约

| 项目 | 内容 |
|---|---|
| 状态 | 部分实施：发布边界前置修复与第一切片（published-only 公开读取、allowlist、DTO）已交付；schema version、字段级公开策略尚未实施 |
| 日期 | 2026-08-22 |
| 目标 | 为非 Go 客户端提供默认关闭、只读且只返回已发布动态内容的稳定 REST 契约 |

## 1. 问题

动态 ContentType/ContentEntry 当前只有需要 JWT、TenantGuard 和权限的管理接口。现有 List/Get 能读取草稿和内部字段，不能直接挂到公共路由。

公开交付还受到以下边界约束：

- 创建、更新、导入和翻译不能绕过独立发布权限。
- ContentType 暂无公开开关，ContentField 暂无字段级可见性。
- 匿名流量当前固定到 default tenant；tenant B 尚无域名或站点键解析。
- `unique`、slug 唯一性和 schema 演进尚未形成稳定契约。

## 2. 第一安全切片

计划提供：

```http
GET /api/v1/public/content/:uid
GET /api/v1/public/content/:uid/:documentId
```

规则：

- `CONTENT_DELIVERY_ENABLED=false` 为默认值。
- 第一版通过显式 UID allowlist 开放类型；未开放、类型不存在、条目不存在或条目未发布统一返回 404。
- 匿名请求固定读取 `models.DefaultTenantID`，忽略 JWT、查询参数和匿名 `X-Tenant-ID`。
- Repository 查询必须同时限定 tenant、content type、`status=published`、非空且不晚于当前时间的 `published_at`。
- 列表只接受有上限的 page、page_size 和 locale；首版不开放任意 JSON 模糊搜索。
- 首版使用 document ID，不提供 slug 查询。
- 取消发布后必须立即不可读取；首版使用 `Cache-Control: no-store`，不把缓存失效留给调用方猜测。

## 3. 公开响应

不得直接序列化内部 `models.ContentEntry`。公开 DTO 只包含：

```json
{
  "document_id": "...",
  "data": {},
  "locale": "zh-CN",
  "published_at": "...",
  "updated_at": "..."
}
```

不得返回 tenant ID、content type 内部 ID、创建者/更新者 ID 或管理状态字段。

在字段级公开策略交付前，allowlist 必须明确表示“该类型全部 Data 字段均可公开”。生产里程碑退出前增加 ContentType 公开开关、ContentField 可见性、schema version 和按 schema 白名单过滤，未知或遗留 JSON key 不得自动公开。

## 4. 实施顺序

1. ~~关闭 create/update/import/translation 的发布权限绕过。~~ — 已完成并补充 Service/Handler 回归测试。
2. ~~增加专用 published-only Repository 与 Service，不复用管理查询的 status 参数。~~ — 已交付 `PublicContentRepository`/`PublicContentService`。
3. ~~增加默认关闭的公开交付配置、专用 Handler 和公开 DTO。~~ — `CONTENT_DELIVERY_ENABLED` + `CONTENT_DELIVERY_UIDS` allowlist、`PublicContentHandler` 与 `publicContentEntry` DTO 已交付。
4. ~~生成 OpenAPI，并加入 TypeScript 消费者冒烟测试。~~ — OpenAPI 已同步；SDK `publicContent` 契约与消费者冒烟测试已交付。
5. 增加 schema version、类型/字段公开策略和仅向后兼容的 schema update。
6. 在域名或站点键租户解析完成前，不开放 tenant B 匿名交付。

## 5. 最小测试矩阵

- draft 读取 404；专用 publish 后 200；unpublish 后立即恢复 404。
- 只有 create/update 权限的主体不能通过请求 status、导入或翻译发布内容。
- tenant A/B 使用相同 UID 和 document ID 时不能串读；匿名头部或 JWT 不能切换公开租户。
- 非 allowlist 类型与不存在类型采用相同 404 行为。
- page_size 有硬上限，非法分页返回 400。
- 响应不含 tenant、actor 或内部 type ID。
- SQLite、PostgreSQL、MySQL 均验证 published-only 查询。
- OpenAPI 无漂移，非 Go 消费者测试不引用内部 Go 类型。

## 6. 暂不包含

- 动态 GraphQL schema、GraphQL 写入。
- slug 查询与跨数据库 JSON 唯一约束。
- tenant B 的匿名域名映射。
- 破坏性 schema 迁移、字段重命名或类型转换。
- 公共 CDN 缓存策略。
