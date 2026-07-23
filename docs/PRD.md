# ContentX 产品需求文档（PRD）

> 本文件定义 ContentX 的产品能力、边界和完成定义。执行进度见 [ROADMAP.md](./ROADMAP.md)，操作流程见 [SOP.md](./SOP.md)。

## 1. 产品定位

ContentX 是一个 API-first 的 Headless CMS，使用 Go 构建，提供 REST API、只读 GraphQL 和 Vue 3 管理后台。核心价值：内容管理、发布工作流、多语言、可观测性和多数据库支持。

当前发布基线：`v1.0.0`。正在推进 P3-A“生产就绪”。

## 2. 已交付能力

### P0：基础可用 ✅

- 文章、页面、分类、标签、评论、媒体、用户与角色
- JWT、刷新令牌、API Token 和 RBAC
- Vue 3 管理后台、REST API 和 Swagger 基础
- PostgreSQL、MySQL、SQLite 与 Docker 基础部署

### P1：工程化 ✅

- Service → Repository 分层与可注入 mock
- Redis 缓存、令牌黑名单和进程内回退
- 数据库迁移、统一错误模型、配置校验和安全中间件
- 后端测试、CI、多平台构建与发布工作流基础

### P2：功能完善 ✅

- Webhook：HMAC 签名、投递日志和业务事件接线
- 媒体：本地/S3 兼容存储
- 内容：六状态发布工作流、修订恢复和定时发布
- API：只读 GraphQL、自定义内容类型和内容条目
- 国际化：文章和动态内容翻译组
- 扩展：编译期插件接口与 Hook/Filter

## 3. 主要功能清单

| 领域 | 能力 |
|---|---|
| 内容管理 | 文章、页面、分类、标签、评论、修订历史和定时发布 |
| 发布工作流 | 草稿、待审核、已排期、已发布、已归档和回收站 |
| 自定义内容类型 | 运行时定义字段，通过统一 REST API 管理内容条目 |
| API | REST、只读 GraphQL、Swagger/OpenAPI 和 TypeScript SDK |
| 国际化 | 文章和自定义内容条目的多语言版本及翻译组 |
| 权限与认证 | JWT、刷新令牌、API Token、角色和细粒度权限 |
| 媒体存储 | 本地文件或 S3 兼容存储 |
| 搜索 | 内置 BM25 倒排索引、中文 bigram、高亮、筛选和分页 |
| 集成 | Webhook HMAC 签名、编译期插件接口、Hook/Filter |
| 运行能力 | Redis 缓存、分布式定时任务锁、Prometheus、Grafana 和 OpenTelemetry |

## 4. 当前边界

以下是已知的未完成或受限能力，不构成 SLA 承诺：

- **GraphQL**：当前只读，写操作走 REST。
- **搜索**：内置索引不跨实例共享，外部 MeiliSearch 驱动尚未完成。
- **备份与恢复**：默认 Docker 部署的 PostgreSQL 备份端到端演练尚未完成（见 ROADMAP Round 2）。
- **压测基线**：现有 PostgreSQL/MySQL 横向对照已标记为失效历史数据，需统一条件重跑（见 ROADMAP Round 3）。
- **Release 二进制**：无 CGO 发行版不支持 SQLite（见 ROADMAP Round 4 / S1-3）。
- **性能数字**：README 中引用的性能数字是阶段性本机结果，不是 SLA。

## 5. P3-B：商业化基础路线

P3-B 在 P3-A 完成后开始。

### B1：多租户设计与 PoC

- `tenant_id` 数据模型与默认租户迁移
- Repository 自动 tenant scope
- RBAC、缓存 key、搜索、Webhook、存储路径的租户隔离
- 两租户数据泄漏测试

完成标准：任一租户无法通过 REST、GraphQL、搜索、缓存或媒体路径访问其他租户数据。

### B2：租户管理与配额

- 租户 CRUD、暂停、恢复和配额配置
- 用户数、文章数、存储量和 API 请求量限制
- 管理员租户切换与审计

完成标准：配额在并发请求下仍准确执行，暂停租户只能进行允许的只读/导出操作。

### B3：API 用量计量

- 按 tenant、endpoint、status 计数
- Redis 实时聚合与数据库持久化
- 月度用量报表
- Redis 故障时的降级和补偿

完成标准：压测请求数与持久化用量误差可解释且满足计费要求。

### B4：计费系统

- Free、Pro、Enterprise 套餐
- 订阅、升级、降级、续费和取消
- 支付 Webhook 幂等
- 逾期和超额策略

完成标准：沙箱环境完整跑通注册 → 试用 → 付费 → 续费/取消 → 权限变化。

### B5：SSO/OIDC

- Google Workspace、Microsoft Entra 等 OIDC
- Just-in-time 用户创建
- 企业域名限制和角色映射
- SAML 作为后续企业增强

完成标准：至少一个真实 OIDC 提供方完成登录、退出、禁用和审计。

### B6：商业化运维

- 租户级监控、告警和审计导出
- 数据导出与删除
- 管理员支持工具
- 账单与用量对账

完成标准：单个租户问题可以定位、限流、暂停、导出和恢复，不影响其他租户。

## 6. P3-C：卓越与生态路线

### C1：GraphQL 完整化

- Mutation
- DataLoader 解决 N+1
- 权限与 REST 对齐
- 可选 Subscription

### C2：真正的外部插件

- WASM 或独立进程插件
- 插件 SDK、权限、版本和隔离
- 插件失败不拖垮主进程

### C3：内容协作

- 编辑锁
- 行内评论
- 版本 diff
- 实时协同编辑

### C4：媒体处理与 CDN

- resize、WebP/AVIF、裁剪和水印
- CDN URL、缓存失效和签名 URL
- 大文件异步处理

### C5：白标和自定义域名

- 租户品牌、Logo、配色
- 自定义域名和自动证书
- 域名验证和安全隔离

### C6：SLA、安全与合规

- 审计日志导出
- 数据保留、导出和删除
- 灾难恢复 RPO/RTO
- 安全扫描、依赖更新和事件响应

### C7：SDK 与生态

- TypeScript SDK 覆盖全部稳定 API
- Next.js、Nuxt 示例
- 部署模板
- 贡献者指南和版本兼容策略

## 7. 完成定义

一个任务只有同时满足以下条件才可标记 ✅：

- 代码或配置已经落地
- 自动化测试覆盖成功和失败路径
- 在目标运行环境中完成真实验收
- 文档与当前代码一致
- 结果可以由他人复现
- 不使用未经测量或已失效的数据宣称性能、容量和兼容性
- 涉及数据恢复、发行物或外部服务时，必须验证实际产物，而不是只验证 mock
