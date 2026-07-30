# ContentX 项目状态

本文档是当前版本、完成度、限制和发布阻断项的唯一事实入口。产品承诺见[产品需求](./PRD.md)，未来计划见[路线图](./ROADMAP.md)。

更新日期：2026-07-30

## 1. 发布状态

| 项目 | 当前状态 |
|---|---|
| 最新正式版本 | `v1.4.0` |
| 当前分支 | `main` 已合并 PR #1（`504d510`）并完成发布整理 |
| 当前里程碑 | v1.4.0 已发布 |
| 下一里程碑 | 多租户基础 |
| 发布建议 | v1.4.0 已发布；下一步启动多租户数据模型设计 |

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

## 3. 最近验证

2026-07-29 至 2026-07-30 本地验证结果：

| 检查 | 结果 |
|---|---|
| `go test ./... -count=1` | 通过；运行时包含真实 MinIO 集成用例 |
| `go vet ./...` | 通过 |
| `golangci-lint run ./...` | 通过，0 issues |
| Go 模块完整性 | `go mod verify` 通过 |
| 前端类型检查、lint、单测和构建 | 通过；22 个文件、189 项单测 |
| Playwright E2E | 2026-07-30 在跟进分支本地运行 Chromium，35/35 通过；尚未接入 GitHub Actions |
| MinIO 真机集成 | 5 个场景通过 |
| PostgreSQL 16.14 隔离恢复 | 2026-07-30 通过：空库 v1–v7、含数据 v7→v5→v7、备份、清空、CLI 恢复、定向缓存清理和搜索重建；见[演练报告](../reports/backup/pg-drill-20260730.md) |
| 远程 CI `05de8f3` | `test`、`frontend`、`build`、`docker` 全部成功；`Swagger drift check` 成功；[运行记录](https://github.com/yamovo/contentx/actions/runs/30514387569) |
| 审查修复 `504d510` | 全量 Go 测试、vet、golangci-lint、定向回归、前端 ESLint 和 Chromium E2E 通过；[PR CI](https://github.com/yamovo/contentx/actions/runs/30515979618) 的 `test`、`frontend`、`build` 成功 |
| `git diff --check` | 通过 |

上述结果证明当前工作区在本机通过，不等同于已提交代码或远程 CI 已通过。

## 4. 已知限制

| 范围 | 限制 |
|---|---|
| GraphQL | 仅支持公开只读查询 |
| S3 | MinIO 已实测；R2、AWS 和其他供应商需使用部署账户做上线前冒烟测试 |
| 搜索 | 内置搜索可用；Meilisearch 尚未达到生产集成状态 |
| 插件 | 仅支持编译期内置插件，不支持任意外部插件沙箱 |
| 前端 | 仍有 Sass legacy API 警告；部分页面尚未统一采用设计系统 |
| 无障碍 | 尚未完成 WCAG 2.2 AA 与 axe 系统审计 |
| 多租户 | 尚未实现租户隔离、配额和用量计量 |
| 商业化 | 计费、SSO、SLA 和合规能力尚未交付 |

## 5. 发布阻断项

以下事项完成前不应创建下一正式版本：

1. ~~评审并合并草稿 PR #1~~ — 已完成。
2. ~~浏览器 E2E 当前只有本地证据~~ — 35/35 已通过并归档至 `reports/e2e/`；接入 CI 作为后续门禁改进。
3. ~~确定版本号并同步发布说明与镜像标签~~ — 已确定为 `v1.4.0`，CHANGELOG 已同步。

v1.4.0 的 PostgreSQL 演练、审查修复和 CI 证据均已归档。下一里程碑为多租户基础。

## 6. 下一步

1. 启动多租户数据模型设计（RFC）。
2. 将 Playwright E2E 接入 GitHub Actions。
3. 为 `main` 配置 CI required checks。

## 7. 相关文档

- [文档中心](./README.md)
- [产品需求](./PRD.md)
- [路线图](./ROADMAP.md)
- [标准操作流程](./SOP.md)
- [OpenAPI](./api/swagger.yaml)
- [变更日志](../CHANGELOG.md)
- [验证报告](../reports/)
- [历史归档](./archive/)
