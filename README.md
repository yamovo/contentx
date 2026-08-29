# ContentX

**ContentX：面向开发者和 AI Agent 的自托管 Headless CMS。**

ContentX 通过 REST API、只读 GraphQL 和 OpenAPI 向应用交付内容，通过 Vue 3 管理后台服务内容团队，并通过内置 MCP 接口让 AI Agent 在明确的身份、权限和发布工作流内操作内容。当前服务端以 Go 构建，但公共契约、SDK 和外部扩展不要求消费者采用 Go。

> 当前稳定版本为 `v1.4.0`。准确状态、已知限制与下一步计划见[项目状态](./docs/STATUS.md)。

## 为什么选择 ContentX

| 优势 | 带来的价值 |
|---|---|
| 自托管优先，当前核心以 Go 构建 | 在自己的基础设施和数据边界内运行；Go 是当前实现选择，语言收益与生态约束持续验证，不作为客户端的技术前提 |
| 开发者优先的接口 | REST 负责主要读写，GraphQL 负责公开只读查询，OpenAPI 提供可验证的接口契约，便于接入不同前端和自动化流程 |
| AI Agent 是一等客户端 | 同时提供 stdio 与 Streamable HTTP MCP；HTTP MCP 的创建、更新和发布操作复用 API Token、权限检查和显式发布边界 |
| 不止是内容 CRUD | 内置草稿、审核、定时发布、归档、修订历史和乐观锁，适合需要治理的内容生产流程 |
| 可运维、可验证 | 支持多种数据库、备份恢复、Prometheus、OpenTelemetry，以及后端、前端和浏览器端质量门禁 |

## 与常见 Headless CMS 的差异

ContentX 不以插件数量或功能堆叠为目标，而是集中在 **自托管的数据边界、语言中立的开发者契约、受控的 Agent 操作和清晰的安全边界**。Go、MCP 或 RAG 本身都不单独作为市场壁垒。下表比较的是产品重心，不代表其他产品不具备表中能力。

| 方案 | 典型产品重心 | ContentX 的差异化取向 |
|---|---|---|
| [Strapi](https://docs.strapi.io/) | Node.js 生态、插件扩展与原生 MCP | 自托管内容控制、语言中立契约，以及受治理的 Agent 变更路径 |
| [Directus](https://docs.directus.io/getting-started/architecture) | 从现有数据库出发的数据平台体验 | 从内容模型、发布工作流和开发者/Agent 接口出发 |
| [Payload](https://payloadcms.com/docs/configuration/overview) | TypeScript、Next.js、代码优先配置与 MCP 插件 | 独立内容服务，通过标准契约对接不同前端技术栈和 Agent |
| [Sanity Content Lake](https://www.sanity.io/docs/content-lake) / [Studio](https://www.sanity.io/docs/studio/deployment) | 托管 Content Lake 与可定制 Studio | 团队自主管理服务、数据和运维边界，并为 Agent 提供受控的内容工具 |

完整的定位边界、选型说明和文案规则见[产品定位](./docs/POSITIONING.md)。

## 核心能力

| 范围 | 能力 |
|---|---|
| 内容 | 文章、页面、分类、标签、评论、自定义内容类型、国际化、修订历史 |
| 工作流 | 草稿、提交审核、发布、取消发布、定时发布、归档、乐观锁冲突保护 |
| 媒体 | 本地文件与 S3 兼容存储；MinIO 已完成真实上传、读取、删除和分片验证 |
| 安全 | JWT、刷新令牌轮换、API Token、TOTP、RBAC、限流、审计日志 |
| 接口 | REST、只读 GraphQL、OpenAPI、stdio MCP、Streamable HTTP MCP |
| 运维 | SQLite、PostgreSQL、MySQL、Redis、备份恢复、Prometheus、OpenTelemetry |
| 扩展 | Webhook 持久化投递、HMAC、SSRF 防护、重试与并发限制 |

## 系统要求

- Go `1.25` 或更高版本
- Node.js `20` 和 npm（开发或构建管理后台）
- Docker Desktop 或 Docker Engine（容器部署）
- PostgreSQL 与 Redis（推荐生产配置；本地开发默认使用 SQLite）

## 快速开始

### Docker Compose

1. 复制配置模板：

   ```bash
   cp .env.example .env
   ```

2. 至少设置以下生产密钥：

   ```env
   POSTGRES_PASSWORD=replace-with-a-strong-password
   REDIS_PASSWORD=replace-with-a-strong-password
   JWT_SECRET=replace-with-at-least-32-random-characters
   ADMIN_PASSWORD=replace-with-a-strong-password
   GRAFANA_PASSWORD=replace-with-a-strong-password
   ```

3. 启动服务：

   ```bash
   docker compose up -d --build
   ```

默认入口：

- 管理后台：<http://localhost:8080>
- REST API：<http://localhost:8080/api/v1>
- Swagger UI：<http://localhost:8080/swagger/index.html>（仅在非 release 模式挂载；容器部署为 release 模式时不可用）
- 健康检查：<http://localhost:8080/api/v1/system/health>

### 本地开发

后端默认使用 SQLite：

```bash
go run ./cmd/server
```

在另一个终端启动管理后台：

```bash
cd web
npm ci
npm run dev
```

## 验证

提交前运行完整质量门禁：

```bash
make check
```

浏览器端到端测试：

```bash
cd web
npm run test:e2e
```

涉及 S3、Redis、数据库恢复或 OpenTelemetry 的变更还必须运行对应真实服务测试，不能仅依赖 mock。具体命令见[标准操作流程](./docs/SOP.md)。

## 官方文档

完整导航见[文档中心](./docs/README.md)。

| 文档 | 用途 |
|---|---|
| [产品定位](./docs/POSITIONING.md) | 统一定位、差异化价值、选型比较和文案规则 |
| [语言与扩展边界](./docs/ADR-001-language-and-extension-boundary.md) | 当前 Go 实现、语言中立契约和扩展策略的暂定决策 |
| [公开动态内容交付](./docs/RFC-002-public-content-delivery.md) | published-only 公共内容 API 的安全契约与实施顺序 |
| [产品需求](./docs/PRD.md) | 产品承诺范围、非目标和验收标准 |
| [项目状态](./docs/STATUS.md) | 当前版本、已交付能力、限制和发布阻断项 |
| [路线图](./docs/ROADMAP.md) | 当前、下一里程碑及退出条件 |
| [标准操作流程](./docs/SOP.md) | 开发、配置、验证、部署、备份和恢复 |
| [OpenAPI](./docs/api/swagger.yaml) | REST API 的机器可读接口契约 |
| [变更日志](./CHANGELOG.md) | 已发布与未发布变更 |
| [贡献指南](./CONTRIBUTING.md) | 分支、测试和提交要求 |
| [安全策略](./SECURITY.md) | 漏洞报告和支持范围 |

`docs/archive/` 和 `reports/` 仅保存历史方案与验证证据，不是当前事实来源。

## 支持与贡献

- 缺陷和功能建议：使用 [GitHub Issues](https://github.com/yamovo/contentx/issues)
- 安全漏洞：按[安全策略](./SECURITY.md)私下报告，不要创建公开 Issue
- 代码贡献：先阅读[贡献指南](./CONTRIBUTING.md)

## 许可证

本项目采用 [MIT License](./LICENSE)。
