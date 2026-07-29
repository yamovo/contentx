# ContentX

ContentX 是一个使用 Go 构建的 API-first Headless CMS，提供 REST API、只读 GraphQL、Vue 3 管理后台，以及面向 AI Agent 的 MCP 接口。

## 功能

- 文章、页面、分类、标签、评论和媒体管理
- 草稿、审核、定时发布、发布和归档工作流
- JWT、API Token、TOTP 和基于角色的访问控制
- 自定义内容类型、国际化和版本修订
- PostgreSQL、MySQL 和 SQLite
- Redis 缓存、Webhook、Prometheus 和 OpenTelemetry
- 本地与 S3 兼容存储
- stdio 与 Streamable HTTP MCP

## 要求

- Go 1.25 或更高版本
- Node.js 及 npm（开发管理后台时需要）
- Docker Desktop 或 Docker Engine（使用容器部署时需要）

## 快速开始

### Docker Compose

复制环境变量模板并设置至少以下密钥：

```bash
cp .env.example .env
```

```env
POSTGRES_PASSWORD=replace-with-a-strong-password
REDIS_PASSWORD=replace-with-a-strong-password
JWT_SECRET=replace-with-at-least-32-random-characters
ADMIN_PASSWORD=replace-with-at-least-8-characters
GRAFANA_PASSWORD=replace-with-a-strong-password
```

启动服务：

```bash
docker compose up -d --build
```

管理后台位于 <http://localhost:8080>，Swagger 文档位于
<http://localhost:8080/swagger/index.html>。

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

```bash
make check
```

该命令依次执行格式化、静态检查、Swagger 同步检查、lint 和测试。更完整的开发、部署与恢复步骤见 [标准操作流程](./docs/SOP.md)。

## 文档

| 文档 | 用途 |
|---|---|
| [产品需求](./docs/PRD.md) | 产品定位、范围和验收标准 |
| [路线图](./docs/ROADMAP.md) | 里程碑、优先级和后续计划 |
| [标准操作流程](./docs/SOP.md) | 开发、部署、验证、备份与恢复 |
| [项目状态](./docs/STATUS.md) | 当前版本、已知限制和下一步 |
| [API 文档](./docs/api/swagger.yaml) | OpenAPI 规范 |
| [变更日志](./CHANGELOG.md) | 版本变更记录 |

## 获取帮助

请通过 [GitHub Issues](https://github.com/yamovo/contentx/issues) 报告缺陷或提出功能建议。提交问题前请避免公开令牌、密码、个人数据或未修复漏洞的利用细节。

## 贡献

提交变更前请运行 `make check`，并确保文档、测试和生成的 Swagger 文件与代码保持同步。较大的功能变更建议先创建 Issue 说明需求和设计。

## 安全

安全问题请使用 GitHub Security Advisory 私下报告，不要创建公开 Issue。

## 许可证

本项目采用 [MIT License](./LICENSE)。
