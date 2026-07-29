# ContentX

ContentX 是一个使用 Go 和 Vue 3 构建的 API-first Headless CMS。它提供 REST API、只读 GraphQL、管理后台，以及面向 AI Agent 的 Model Context Protocol（MCP）接口。

> 当前稳定版本为 `v1.3.0`。工作区包含尚未发布的安全、稳定性和存储改进；准确状态与发布阻断项见[项目状态](./docs/STATUS.md)。

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
- Swagger UI：<http://localhost:8080/swagger/index.html>
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
| [产品需求](./docs/PRD.md) | 产品定位、范围、非目标和验收标准 |
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
