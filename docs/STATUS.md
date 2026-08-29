# ContentX 项目状态

本文档是当前版本、完成度、限制和发布阻断项的唯一事实入口。产品承诺见[产品需求](./PRD.md)，未来计划见[路线图](./ROADMAP.md)。

更新日期：2026-08-29

## 1. 发布状态

| 项目 | 当前状态 |
|---|---|
| 最新正式版本 | `v1.4.0` |
| 当前分支 | `main`（`f5f3659`）；多租户 P0、MCP 主体再验证与 RAG 治理收口已随本批次提交 |
| 当前里程碑 | 语言/扩展边界 + Headless CMS 公开交付 + 多租户/RAG 安全收口（进行中） |
| 发布建议 | v1.4.0 已发布；多租户 P0 与 RAG 安全收口仍未全部完成，不应创建下一正式版本 |

## 2. 已交付能力

### 内容与后台

- REST API、只读 GraphQL 和 Vue 3 管理后台
- 文章、页面、分类、标签、评论、媒体、用户和角色
- 自定义内容类型、内容条目、国际化字段、修订和定时发布
- 草稿、审核、发布、取消发布、定时和归档工作流
- 文章版本乐观锁与 `409 CONCURRENT_MODIFICATION`

### 身份与安全

- JWT、刷新令牌轮换、API Token、TOTP 和 RBAC
- 登录、注册、账户和接口维度限流
- Webhook SSRF 防护、HMAC 和安全重试
- 认证失败、账户锁定、权限拒绝和关键业务实体审计
- 审计详情递归脱敏与 Webhook URL 凭据清理

### 数据、存储与运维

- SQLite、PostgreSQL、MySQL、Redis 和 Docker Compose
- 数据库迁移、种子数据、备份与恢复
- 本地文件和基于 `minio-go/v7` 的 S3 兼容存储
- MinIO 上传、SDK 读取、删除、预签名下载、17 MiB 分片和错误凭据验证
- 健康检查、Prometheus、OpenTelemetry 和结构化日志

### 集成与自动化

- stdio 与 Streamable HTTP MCP
- 受权限控制的 MCP 创建、更新和发布工具
- Webhook 持久化队列、并发限制、指数退避、jitter 和崩溃复投
- Go 测试、前端单测、Playwright E2E、静态检查和 OpenAPI 漂移检查

## 3. 在研能力（不计入生产交付）

### 语言、扩展与公开内容契约

- [ADR-001](./ADR-001-language-and-extension-boundary.md) 已进入验证：暂不重写服务端，先采用“Go 核心 + 语言中立公共契约”；这不是永久语言承诺
- 当前 Go `Plugin` interface 只支持编译期内置插件，不作为外部插件 ABI；进程外扩展或 WASM 尚未交付
- 动态 ContentEntry 当前只有受保护的管理接口；[RFC-002](./RFC-002-public-content-delivery.md) 已定义默认关闭、published-only、显式 allowlist 的第一公开交付切片
- 本批次已关闭 create/update/import/translation 绕过独立发布流程的路径：启用草稿发布的类型只能以 draft 创建或导入，更新请求不能携带 status，翻译也不能直接发布；公开接口仍须建立专用 DTO、分页上限与 default tenant 固定策略

### 多租户 P0 安全收口

- 核心内容模型、Repository 查询、WebhookLog 和 Revision 已加入租户范围
- MCP HTTP 已接入统一验证的 TokenPrincipal：认证中间件注入租户绑定 principal，每次工具/资源调用经 Authorizer 按请求头再验证并与会话绑定，HTTP 传输使用 per-request stateless server 且强制关闭 drafts；资源发现与 RAG 工具均已按有效权限门控
- JWT 与 API Token 均按 TenantMembership 角色、租户状态复核并取有效权限交集；刷新令牌每请求重新校验租户存在性、成员资格与角色（显式租户保持 + 遗留令牌确定性解析，专项测试覆盖）
- 评论回复 fail closed：父评论必须同租户同文章，Children 预加载显式租户过滤，历史悬挂引用不再跨租户渲染；审计日志写入/读取/暴露面逐层核验并新增链路测试
- GraphQL tenant A/B 攻击矩阵已补齐：5 条端到端 HTTP 测试覆盖匿名伪造头、双向读取、非管理员切换 fail closed 与管理员切换作用域
- Settings 更新现已写入租户覆盖行并保留全局默认值；Get/List/Public 会优先采用租户覆盖，且私有覆盖不会回退暴露公开全局值
- Settings 批量写入已置于单一事务，迁移 012 封堵 NULL 全局键重复，并已在 PostgreSQL 16 与 MySQL 8.4 完成迁移、CRUD、唯一性和故障回滚实测；API Token 生命周期、评论父子归属和平台审计日志边界仍需继续收口
- 攻击矩阵现状：Service/Repository（16+ 条 `TestTenantIsolation_*`）、MCP（token 绑定/主体再验证/资源隔离/RAG 权限）、GraphQL（5 条端到端 HTTP 测试）、刷新链（5 项专项测试）均已本地闭环；剩余为远程 CI 验证与开放真实 tenant B 前的验收

### AI 与 RAG 原型

- 已实现语义搜索、RAG 问答、按 tenantID 分区的内存向量存储和 REST AI 权限
- 外发边界已统一下沉到 Service 层：`AI_ALLOW_OUTBOUND=false` 经 `External()` 语义拦截所有外部 Embedding/LLM 调用（REST Ask 维持 403，MCP rag_ask 返回工具错误，Search/Retrieve/Index 同样拒绝），本地 dummy provider 不受影响
- Reindex 已改为清理式重建：按当前已发布集合清理向量存储与嵌入表孤儿；文章同步路径增加有界重试（策略性拒绝不重试），定时发布同步经复核已接线
- REST AI 有 IP 限流与操作审计；MCP RAG 具备独立权限（`ai.read`/`ai.ask`）、租户分区、调用审计（`mcp.rag_search`/`mcp.rag_ask`）与独立 IP 限流（`MCP_RATE_LIMIT`，默认 60/分钟）
- `AI_ENABLED=false`、`MCP_HTTP_ENABLED=false` 为默认状态；生产验收通过前不得对公网启用 MCP RAG

## 4. 最近验证

2026-07-29 至 2026-08-29 本地验证结果：

| 检查 | 结果 |
|---|---|
| `go build ./...` | 通过（CGO_ENABLED=1，Go 1.26.5，GCC 8.1.0） |
| `go vet ./...` | 通过，0 warnings |
| `go test ./... -count=1` | 2026-08-29 全量通过：21 个包 0 失败，含新增 MCP 主体再验证、RAG 权限与资源租户隔离回归 |
| `golangci-lint run ./...` | 2026-08-29 通过，0 issues（修复 AI provider/migration 的 7 项告警；v2.12.2 与 CI 同版本） |
| Go 模块完整性 | `go mod verify` 通过 |
| 前端类型检查、lint、单测和构建 | 类型检查、ESLint 与 189 项单测 2026-08-29 通过；22 个文件 |
| OpenAPI 漂移 | 2026-08-29 `swag init` 重新生成，`/ai/search`、`/ai/rag/ask`、`/ai/reindex`、`/ai/status` 已补齐，无 diff |
| 动态内容发布边界 | `internal/services` 全量回归通过；`internal/handlers` 全量回归通过；新增 create/update/import/translation 拒绝与 unpublish 时间清理测试 |
| Playwright E2E | 2026-08-22 本地 Chromium 35/35 通过；CI workflow 已加入 Chromium 安装与 E2E 门禁，仍待远程运行验证 |
| Settings 租户覆盖 | SQLite 回归及 PostgreSQL 16/MySQL 8.4 真机测试通过；覆盖 001–012 迁移、全局/租户唯一性、覆盖合并、原子批量回滚和 009 安全降级 |
| TypeScript SDK | v3 lockfile、8/8 单测、严格类型检查、ESM/CJS 构建、示例检查和 21 文件 pack dry-run 通过；CI 已加入 `npm ci/check/pack`，仍待远程空环境运行 |
| Release 独立包 | Linux amd64 CI 路径已改为 CGO 原生构建并携带 `web/dist`；手动 dispatch 只验证不发布，Windows amd64 本地等价 smoke 通过，目标 Ubuntu 产物仍待远程 workflow |
| MinIO 真机集成 | 5 个场景通过 |
| PostgreSQL 16.14 隔离恢复 | 2026-07-30 通过；见[演练报告](../reports/backup/pg-drill-20260730.md) |
| RAG/AI 测试 | 现有 RAG Service 与 MCP RAG 测试全绿；新增外发策略（外部/本地 provider × 允许/禁止）、MCP 外发拒绝与审计链路、清理式重建孤儿清扫（租户内/全量）、索引同步有界重试回归 |
| 多租户隔离测试 | 2026-08-29 本地全绿：16+ 条 `TestTenantIsolation_*`（Service/Repository + 审计链路）、MCP 主体/资源/RAG 回归、5 条 GraphQL 端到端攻击测试、5 项刷新令牌租户专项测试 |
| RAG/AI 测试 | 现有 RAG Service 与 MCP RAG 测试全绿，MCP `rag_search`/`rag_ask` 权限门控已覆盖；孤儿向量、定时发布、外发关闭和成本边界尚无完整回归测试 |
| 远程 CI `da3b408` | `test`（含 golangci-lint v2.12.2 与全量 Go 测试）、`sdk`（npm ci/check/pack）、`frontend`、`build`、`settings-databases` 全部成功；首轮失败的 lint 告警与 SDK lockfile 缺 Linux 原生依赖问题已修复 |
| `git diff --check` | 通过 |

上述结果证明当前工作区在本机通过，不等同于已提交代码或远程 CI 已通过。

## 5. 已知限制

| 范围 | 限制 |
|---|---|
| GraphQL | 仅支持公开只读查询 |
| 动态内容公开交付 | ContentEntry 目前只有受保护的管理接口；published-only 公共 REST 契约仍在设计与实现中，不得直接复用管理 List/Get |
| 语言与扩展 | 服务端当前使用 Go；语言策略正在验证，外部扩展尚无稳定 ABI，不应把 Go interface 或 `.so` 描述为插件生态 |
| 日志与审计 | access log 已含 request ID/trace ID，业务审计也已落库与脱敏；ActivityLog 尚无 request/trace/span/source 等一等关联字段，高风险审计仍是 best-effort，OTLP logs、保留、导出和完整性策略未交付 |
| S3 | MinIO 已实测；R2、AWS 和其他供应商需使用部署账户做上线前冒烟测试 |
| 搜索 | 内置搜索可用；Meilisearch 尚未达到生产集成状态 |
| 插件 | 仅支持编译期内置插件，不支持任意外部插件沙箱 |
| 前端 | 仍有 Sass legacy API 警告；部分页面尚未统一采用设计系统 |
| 无障碍 | 尚未完成 WCAG 2.2 AA 与 axe 系统审计 |
| 多租户 | P0 安全收口接近完成：数据查询、Settings 租户覆盖、MCP/API Token 主体再验证、刷新令牌租户保持、评论归属与审计边界均已闭环并有本地测试；远程 CI 验证与真实 tenant B 开放前验收待完成；不得开放真实 tenant B |
| AI/RAG | 默认关闭的原型已完成最小安全闭环：外发边界、索引生命周期、MCP 治理与成本边界均有实现和测试；生产验收（真实 provider 验证、pgvector 等持久化向量存储）仍待交付，向量存储仅有内存实现 |
| 商业化 | 计费、SSO、SLA 和合规能力尚未交付 |

## 6. 下一正式版本阻断项

v1.4.0 的 PostgreSQL 演练、审查修复和 CI 证据已经归档。以下是当前未提交里程碑进入下一正式版本前必须关闭的新阻断项：

1. 动态内容 create/update/import/translation 的发布权限绕过已关闭；公共内容交付上线前仍须通过 draft/publish/unpublish、allowlist、输出 DTO 和 tenant A/B 拒绝测试。
2. access/refresh、TenantMembership 角色、租户状态与 API Token/MCP principal 的统一验证已完成本地闭环（含 GraphQL/MCP/刷新链攻击矩阵）；须在远程 CI 复现通过并在开放真实 tenant B 前完成验收。
3. ~~修复评论父子归属、平台审计日志和其他剩余跨租户边界~~ 已收口（评论回复 fail closed + 预加载租户过滤 + 审计链路测试）；后续如发现新的跨租户边界按同标准处理。
4. ~~RAG 孤儿清理、定时发布同步、失败补偿、外发统一拦截、限流、审计和成本边界~~ 已收口（见"RAG 最小安全闭环完成"）；上线前仍须完成真实 provider 验证与持久化向量存储选型。
5. 通过安全的手动 workflow 或正式 tag CI 验证 Ubuntu 22.04 上的 Linux amd64 CGO + SQLite + `web/dist` 独立包；其他平台须有原生 CGO runner 和同等 smoke 后再发布。
6. 在远程 CI 验证 TypeScript SDK 的锁文件空环境安装、check 与 pack，并补发布前消费者冒烟测试。

## 7. 下一步

1. 继续验证 ADR-001 的语言中立公共/扩展边界；发布绕过前置修复已完成，下一步按 RFC-002 实现默认关闭的 published-only 公开 REST 切片。
2. 多租户 P0 安全收口：剩余工作为远程 CI 复现攻击矩阵结果，并在开放真实 tenant B 前完成上线验收；租户管理 API、成员管理、切换器与 tenant-aware 前端缓存随之推进。
3. RAG 生产化：真实 provider（OpenAI 兼容）端到端验证、模型/维度切换与 `AI_MIN_SCORE` 语义、pgvector/Qdrant 等持久化向量存储。
4. 交付链验证：在真实 tag CI 运行 Linux amd64 SQLite、前端打包和 Release 产物冒烟测试。
5. 同步 OpenAPI、SOP、部署文档和状态口径。
6. 实现租户管理 API、成员管理、切换器和 tenant-aware Vue Query 缓存。
7. 远程验证新增的 Playwright/ESLint、SDK 与 PostgreSQL/MySQL workflow，并为 `main` 配置 CI required checks。

## 8. 相关文档

- [文档中心](./README.md)
- [产品定位](./POSITIONING.md)
- [语言与扩展边界](./ADR-001-language-and-extension-boundary.md)
- [公开动态内容交付契约](./RFC-002-public-content-delivery.md)
- [竞品与日志/审计方向](./RESEARCH-001-market-and-observability.md)
- [产品需求](./PRD.md)
- [路线图](./ROADMAP.md)
- [标准操作流程](./SOP.md)
- [OpenAPI](./api/swagger.yaml)
- [变更日志](../CHANGELOG.md)
- [验证报告](../reports/)
- [历史归档](./archive/)
