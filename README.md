# ContentX

ContentX 是一个使用 Go 构建的 API-first Headless CMS。它提供 REST API、只读 GraphQL、Vue 3 管理后台，并支持文章工作流、自定义内容类型、国际化、媒体管理、搜索、Webhook、插件和可观测性。

当前发布基线为 `v1.0.0`。P3-A“生产就绪”已通过 ROADMAP Round 1-5 全部验收，下一里程碑为 `v1.1.0`。

## 文档导航

| 文档 | 内容 |
|---|---|
| [docs/PRD.md](./docs/PRD.md) | 产品需求、已交付能力、当前边界和完成定义 |
| [docs/SOP.md](./docs/SOP.md) | 本地开发、Docker 部署、可观测性、压测流程和验证命令 |
| [docs/ROADMAP.md](./docs/ROADMAP.md) | 轮次化执行路线、退出门槛和当前进度 |
| [CHANGELOG.md](./CHANGELOG.md) | 版本变更记录 |

## 技术栈

| 层 | 实现 |
|---|---|
| 后端 | Go 1.25、Gin、GORM |
| 数据库 | PostgreSQL、MySQL、SQLite |
| 缓存与协调 | Redis，可回退到进程内实现 |
| 前端 | Vue 3、TypeScript、Vite、Element Plus |
| API | REST、GraphQL、Swagger/OpenAPI |
| 可观测性 | Prometheus、Grafana、OpenTelemetry、Tempo |
| 部署 | 单应用镜像、Docker Compose、Nginx |

## 快速开始

### Docker Compose（推荐）

要求：Docker Desktop 或 Docker Engine 已启动。

在项目根目录创建 `.env`，至少设置以下值：

```env
POSTGRES_PASSWORD=replace-with-a-strong-password
REDIS_PASSWORD=replace-with-a-strong-password
JWT_SECRET=replace-with-at-least-32-random-characters
ADMIN_PASSWORD=replace-with-at-least-8-characters
GRAFANA_PASSWORD=replace-with-a-strong-password
```

启动应用、PostgreSQL、Redis 和 Nginx：

```bash
docker compose up -d --build
```

需要监控与链路追踪时：

```bash
docker compose --profile monitor up -d --build
```

默认入口：

| 服务 | 地址 |
|---|---|
| 管理后台 | http://localhost:8080 |
| REST API | http://localhost:8080/api/v1 |
| Swagger | http://localhost:8080/swagger/index.html |
| GraphQL | http://localhost:8080/api/v1/graphql |
| 健康检查 | http://localhost:8080/api/v1/system/health |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3001 |
| Tempo | http://localhost:3200 |

如端口冲突，可在 `.env` 中修改 `APP_PORT`、`HTTP_PORT` 和 `HTTPS_PORT`。管理账号为 `admin`，密码取自 `ADMIN_PASSWORD`。

### 本地开发

后端默认使用 SQLite，适合快速开发：

```bash
go run ./cmd/server
```

前端开发服务器：

```bash
cd web
npm ci
npm run dev
```

生产模式不会自动接受弱密钥：必须提供有效的 `JWT_SECRET`、`ADMIN_PASSWORD`，使用 PostgreSQL/MySQL 时还必须提供数据库密码。

详细的验证命令、压测流程和可观测性配置见 [SOP.md](./docs/SOP.md)。

## 阶段性性能基线

> ⚠️ 所有性能数字为**阶段性本机结果，非 SLA**。来自开发机 Docker 环境，仅供相对比较。

跨数据库（PostgreSQL / MySQL / SQLite）对照基线已由 ROADMAP Round 3 在统一条件下重跑
（同一 Git SHA、10,000 篇数据集、同一 Linux 容器内压测）。完整数据与根因归因见
[reports/benchmarks/cross-db-comparison.md](./reports/benchmarks/cross-db-comparison.md)。

PostgreSQL（主推驱动）摘要（读 1,000 req/s × 15s，10,000 篇）：

| 场景 | 成功率 | P50 | P95 | P99 |
|---|---:|---:|---:|---:|
| 文章列表（20 条） | 100% | 8.29 ms | 13.35 ms | 21.75 ms |
| 文章详情 | 100% | 1.52 ms | 1.98 ms | 2.59 ms |
| GraphQL 查询 | 100% | 8.24 ms | 11.22 ms | 18.70 ms |
| 并发写入（100 req/s） | 100% | 7.48 ms | 9.70 ms | 12.49 ms |

原始 Vegeta JSON 位于 `reports/benchmarks/raw/`。

## 项目结构

```text
cmd/server/             应用入口与 HTTP 服务
internal/handlers/      路由、HTTP handler 和 DTO
internal/services/      业务逻辑、工作流、搜索和调度器
internal/repository/    数据访问接口与 GORM 实现
internal/models/        数据模型
internal/cache/         内存/Redis 缓存和分布式锁
internal/storage/       本地/S3 兼容存储
internal/observability/ 指标与链路追踪
internal/graphql/       GraphQL schema 与 resolver
web/                    Vue 3 管理后台
sdk/typescript/         TypeScript SDK
deploy/                 Nginx、Prometheus、Grafana、Tempo 配置
docs/                   PRD、SOP、ROADMAP、OpenAPI 和截图
scripts/benchmark/      可复现压测脚本与数据集
reports/benchmarks/     压测原始结果与后续报告
```

## 当前边界

- GraphQL 当前只读，写操作走 REST。
- 内置搜索索引不跨实例共享，外部 MeiliSearch 驱动尚未完成。
- **Release 二进制不支持 SQLite**：CI Release 作业以 `CGO_ENABLED=0` 构建跨平台二进制，
  SQLite 驱动需要 CGO，因此预编译 Release 包仅支持 PostgreSQL / MySQL。需要 SQLite 时
  请使用 Docker 镜像（内置 CGO 构建）或本地 `go build`（需 C 编译器）。
- README 中的性能数字是阶段性本机结果，不是 SLA。

完整的当前边界和完成定义见 [PRD.md](./docs/PRD.md)。

## 许可证

[MIT](./LICENSE)
