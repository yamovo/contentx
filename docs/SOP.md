# ContentX 标准操作流程（SOP）

> 本文件定义 ContentX 的验证、部署、压测和可观测性操作流程。产品能力见 [PRD.md](./PRD.md)，执行进度见 [ROADMAP.md](./ROADMAP.md)。

## 1. 本地开发

### 后端

默认使用 SQLite，适合快速开发：

```bash
go run ./cmd/server
```

### 前端

```bash
cd web
npm ci
npm run dev
```

### 验证命令

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

为避免扫描 `web/node_modules` 中碰巧存在的 Go 包，使用明确包范围：

```powershell
$env:GOCACHE = Join-Path $env:TEMP 'contentx-verify-cache'
& 'D:\tool\Go\bin\go.exe' test -p=1 ./cmd/server ./docs/api ./internal/... ./scripts/benchmark/seeder ./tests -count=1
& 'D:\tool\Go\bin\go.exe' vet ./cmd/server ./docs/api ./internal/... ./scripts/benchmark/seeder ./tests
& 'D:\tool\Go\bin\go.exe' build -o (Join-Path $env:TEMP 'contentx-verify.exe') ./cmd/server
```

## 2. Docker Compose 部署

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

### 配置解析与健康检查

```powershell
docker compose --profile monitor config --quiet
docker compose --profile monitor ps
Invoke-RestMethod http://127.0.0.1:18080/api/v1/system/health
```

端口以 `.env` 为准。验收还必须检查 Prometheus target、Grafana 数据源和 Tempo Trace。

生产模式不会自动接受弱密钥：必须提供有效的 `JWT_SECRET`、`ADMIN_PASSWORD`，使用 PostgreSQL/MySQL 时还必须提供数据库密码。

## 3. 可观测性

ContentX 提供 Prometheus 指标、Grafana 仪表盘和 OpenTelemetry 分布式追踪。追踪通过 OTLP/HTTP 发送至 Tempo，默认关闭；Prometheus 指标默认在 `/metrics` 开启。

### 启动监控栈

先在 `.env` 中设置生产启动所需的密码，并开启追踪：

```env
POSTGRES_PASSWORD=change-me
JWT_SECRET=change-me-to-at-least-16-characters
ADMIN_PASSWORD=change-me
OTEL_ENABLED=true
```

然后启动应用与监控 profile：

```bash
docker compose --profile monitor up -d
```

Grafana 会自动加载 Prometheus、Tempo 数据源和 ContentX 仪表盘。

如默认端口已被占用，可在 `.env` 中设置 `APP_PORT`、`HTTP_PORT`、`HTTPS_PORT`。应用容器内部仍使用 8080，不影响 Prometheus 抓取。

### 指标

- `http_requests_total{method,path,status}`
- `http_request_duration_seconds{method,path}`
- `active_users_total`
- `articles_total{status}`
- `db_connections_in_use`
- `cache_hits_total` / `cache_misses_total`
- `webhook_dispatch_total{event,status}`

HTTP 路由参数会统一为 `:param`，避免 Prometheus 标签基数失控。

### 追踪

每个 HTTP 请求创建 server span，并自动提取和注入 W3C `traceparent` / `baggage`。数据库操作创建 GORM client span；Webhook 和 S3 请求创建外部 HTTP client span。

响应头 `X-Trace-ID` 和结构化日志字段 `trace_id` 使用同一 TraceID；请求的 `X-Request-ID` 会写入根 span 的 `request.id` 属性，便于日志和链路互查。

常用配置：

```env
OTEL_ENABLED=false
OTEL_SERVICE_NAME=contentx
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
OTEL_EXPORTER_OTLP_INSECURE=true
OTEL_TRACE_SAMPLE_RATIO=1.0
```

生产环境可降低 `OTEL_TRACE_SAMPLE_RATIO`，并为 OTLP 端点启用 TLS。

### 验收记录

2026-07-22 已使用 PostgreSQL、Redis、Prometheus、Grafana、Tempo 完成真实容器验收：应用健康检查返回 200，Prometheus target 为 `up`，Grafana 自动加载两个数据源和 ContentX 仪表盘，真实 HTTP 请求的 `X-Trace-ID` 可从 Tempo API 查询。

## 4. 压测流程

### 4.1 对照原则

- **同一数据集**：三种驱动都用等价的 1,000 / 10,000 篇文章 seed（正文均为 `ContentX benchmark content for realistic payload size. ` 重复 40 次，2,200 字符）。
- **同一场景**：文章列表、文章详情、GraphQL 查询、并发写入四个场景。
- **同一采样规模**：读 1,000 req/s × 15s；写 100 req/s × 10s。
- **同一压测脚本**：`scripts/benchmark/run-benchmark.ps1 -Driver <驱动>`，只有原始结果输出目录随驱动变化。
- **同一空闲内存口径**：`scripts/benchmark/sample-memory.ps1`，无负载下采样 12 次取 min/mean/max。

### 4.2 PostgreSQL

```powershell
# 主 compose 即 PostgreSQL 栈
docker compose up -d --build

# 播种 10,000 篇。PowerShell 不支持 `<` 重定向，用 Get-Content 管道；
# Bash 用户可改用：docker exec -i contentx-db psql -U contentx contentx < scripts/benchmark/seed_postgres_10000.sql
Get-Content scripts/benchmark/seed_postgres_10000.sql | docker exec -i contentx-db psql -U contentx contentx

# -BaseUrl 现为可选：-Driver postgres 默认 http://127.0.0.1:18080
pwsh scripts/benchmark/run-benchmark.ps1 -Driver postgres
pwsh scripts/benchmark/sample-memory.ps1 -Container contentx
```

### 4.3 MySQL

```powershell
docker compose -f scripts/benchmark/docker-compose.mysql.yml up -d --build
# 等待 contentx-bench-mysql healthy 后播种。PowerShell 用 Get-Content 管道；
# Bash 用户可改用：mysql -h127.0.0.1 -P13306 -ucontentx -pbenchpass contentx < scripts/benchmark/seed_mysql_10000.sql
Get-Content scripts/benchmark/seed_mysql_10000.sql | mysql -h127.0.0.1 -P13306 -ucontentx -pbenchpass contentx

# -BaseUrl 现为可选：-Driver mysql 默认 http://127.0.0.1:18090。
# 应用端口 18090 与 PostgreSQL 的 18080 不同，两栈可并行运行。
pwsh scripts/benchmark/run-benchmark.ps1 -Driver mysql -AdminPassword 'BenchAdmin123!'
pwsh scripts/benchmark/sample-memory.ps1 -Container contentx-bench-app
docker compose -f scripts/benchmark/docker-compose.mysql.yml down -v
```

### 4.4 SQLite

前提：Go 1.21+ 与一个 C 编译器（SQLite 驱动需 CGO，如 MinGW gcc）。内置 `scripts/benchmark/seeder` 用与应用相同的 GORM SQLite 驱动播种，免 `sqlite3` CLI 依赖。

```powershell
$env:DB_DRIVER="sqlite"; $env:DB_NAME="bench_sqlite.db"; $env:SERVER_MODE="release"
$env:SERVER_PORT="18081"; $env:CACHE_DRIVER="memory"; $env:QUEUE_DRIVER="memory"
$env:JWT_SECRET="bench-jwt-secret-please-use-32-chars-x"; $env:ADMIN_PASSWORD="BenchAdmin123"

# 1) 构建二进制（CGO）
go build -o cxbench.exe ./cmd/server
go build -o cxseed.exe ./scripts/benchmark/seeder

# 2) 建表 + 建 admin，再播 10k
.\cxbench.exe -migrate; .\cxbench.exe -seed
.\cxseed.exe -db bench_sqlite.db -sql scripts/benchmark/seed_sqlite_10000.sql

# 3) 启动应用（启动时从 DB 全量建搜索索引）
.\cxbench.exe

# —— 另开一个终端 ——
pwsh scripts/benchmark/run-benchmark.ps1 -Driver sqlite -BaseUrl http://127.0.0.1:18081 -AdminPassword 'BenchAdmin123'
pwsh scripts/benchmark/sample-memory.ps1 -ProcessName cxbench
```

### 4.5 搜索引擎配置

`SEARCH_ENGINE=builtin` 是当前完整实现：索引保存在应用进程内，启动时从数据库重建，适合单实例或对短暂索引重建可接受的部署。

- `builtin`：已实现，支持 BM25、中文 bigram、高亮、筛选和分页
- `noop`：关闭搜索
- `meilisearch`：当前仅保留配置入口，会记录警告并回退到 `builtin`，尚未集成外部驱动

多实例部署时，各实例拥有独立内存索引；在外部搜索驱动完成前，不应把它描述为共享搜索集群。

## 5. 证据要求

每项验收证据最少包含：

- Git SHA
- 日期与环境
- 完整命令
- exit code
- 测试/请求数量
- 原始结果路径
- 已知限制

## 6. API 文档生成

Swagger 文档由源码注解自动生成：

```bash
make swagger
# 或直接调用：
swag init -g cmd/server/main.go --parseDependency --parseInternal -o docs/api
```

CI 会检查生成结果与提交的 `docs/api/` 是否一致（漂移检查）。运行中的 `/swagger/index.html` 是最新版本，仓库中的 `docs/api/` 可能滞后于代码。

所有业务接口以 `/api/v1` 为前缀。

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
