# ContentX

ContentX 是一个使用 Go 构建的 API-first Headless CMS。它提供 REST API、只读 GraphQL、Vue 3 管理后台，并支持文章工作流、自定义内容类型、国际化、媒体管理、搜索、Webhook、插件和可观测性。

当前发布基线为 `v1.0.0`。仓库正在进行 P3-A“生产就绪”改进；准确进度、未完成项和验收证据见 [PROGRESS.md](./PROGRESS.md)。

## 主要能力

- 内容管理：文章、页面、分类、标签、评论、修订历史和定时发布
- 发布工作流：草稿、待审核、已排期、已发布、已归档和回收站
- 自定义内容类型：运行时定义字段，并通过统一 REST API 管理内容条目
- API：REST、只读 GraphQL、Swagger/OpenAPI 和 TypeScript SDK
- 国际化：文章和自定义内容条目的多语言版本及翻译组
- 权限与认证：JWT、刷新令牌、API Token、角色和细粒度权限
- 媒体存储：本地文件或 S3 兼容存储
- 搜索：内置 BM25 倒排索引、中文 bigram、高亮、筛选和分页
- 集成：Webhook HMAC 签名、编译期插件接口、Hook/Filter
- 运行能力：Redis 缓存、分布式定时任务锁、Prometheus、Grafana 和 OpenTelemetry

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

停止服务：

```bash
docker compose --profile monitor down
```

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

## 常用接口

所有业务接口以 `/api/v1` 为前缀。完整请求结构和响应模型以运行中的 Swagger 文档为准。

| 分组 | 示例 | 说明 |
|---|---|---|
| Auth | `POST /auth/login` | 登录、注册、刷新、注销和个人资料 |
| Articles | `GET /articles` | CRUD、修订、工作流、翻译和批量操作 |
| Search | `GET /search?q=go` | 公开搜索仅返回已发布内容 |
| GraphQL | `POST /graphql` | 只读聚合查询 |
| Content Types | `POST /content-types` | 定义自定义内容结构 |
| Content | `GET /content/:uid` | 自定义内容 CRUD、发布、导入导出和翻译 |
| Media | `POST /media/upload` | 媒体上传与管理 |
| Webhooks | `POST /webhooks` | Webhook 配置与投递日志 |
| System | `GET /system/health` | 健康检查、系统信息、审计日志和 API Token |

GraphQL 示例：

```bash
curl -X POST http://localhost:8080/api/v1/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"{ articles(page: 1, pageSize: 5) { total items { title slug } } }"}'
```

## 搜索说明

`SEARCH_ENGINE=builtin` 是当前完整实现：索引保存在应用进程内，启动时从数据库重建，适合单实例或对短暂索引重建可接受的部署。

- `builtin`：已实现，支持 BM25、中文 bigram、高亮、筛选和分页
- `noop`：关闭搜索
- `meilisearch`：当前仅保留配置入口，会记录警告并回退到 `builtin`，尚未集成外部驱动

多实例部署时，各实例拥有独立内存索引；在外部搜索驱动完成前，不应把它描述为共享搜索集群。

## 可观测性

应用可暴露 `/metrics`，并通过 OTLP/HTTP 导出 Trace。监控 profile 会自动配置 Prometheus、Grafana 和 Tempo。

核心指标包括 HTTP 请求量和耗时、活跃用户、文章状态、数据库连接、缓存命中/未命中及 Webhook 投递结果。详细配置和排障见 [docs/observability.md](./docs/observability.md)。

## 阶段性性能基线

以下数据来自 2026-07-22 的本机 Docker/PostgreSQL 16 测试，只用于记录当前仓库状态，不代表其他硬件、网络或数据库后端。读取目标速率为 1,000 req/s，写入目标速率为 100 req/s。

| 场景 | 成功率 | P50 | P95 | P99 |
|---|---:|---:|---:|---:|
| 文章列表（20 条） | 100% | 5.74 ms | 351.12 ms | 1.07 s |
| 文章详情 | 100% | 2.66 ms | 3.79 ms | 4.82 ms |
| GraphQL 查询 | 100% | 3.13 ms | 4.30 ms | 5.22 ms |
| 并发更新 | 100% | 9.04 ms | 12.04 ms | 17.57 ms |

10,000 篇文章完整建立内存搜索索引后，应用容器观测为约 `145.4 MiB`；当前应用镜像约 `61.3 MiB`。因此旧文档中的“~30MB 内存”和“镜像 <30MB”已删除。SQLite/MySQL 对照、1,000/10,000 篇可比内存采样和正式报告仍在进行，见 [PROGRESS.md](./PROGRESS.md)。

原始 Vegeta JSON 位于 `reports/benchmarks/raw/postgres/`，复现脚本位于 `scripts/benchmark/`。

## 验证与开发命令

```bash
# 后端
go test ./...
go vet ./...
go build ./cmd/server

# 前端
cd web
npm ci
npm run type-check
npm run test -- --run
npm run build
```

Windows 上若 Go 不在 `PATH`，可直接使用本机安装位置，例如：

```powershell
& 'D:\tool\Go\bin\go.exe' test ./...
```

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
docs/                   OpenAPI、运行文档和截图
scripts/benchmark/      可复现压测脚本与数据集
reports/benchmarks/     压测原始结果与后续报告
```

## 当前边界

- GraphQL 当前只读，写操作走 REST。
- 内置搜索索引不跨实例共享，外部 MeiliSearch 驱动尚未完成。
- CI 中前端类型检查目前允许失败，仍属于 P3-A 待清理项。
- 备份与恢复尚未完成端到端演练。
- README 中的性能数字是阶段性本机结果，不是 SLA。

## 许可证

[MIT](./LICENSE)
