# ContentX 标准操作流程

本文档是 ContentX 开发、配置、验证、部署、备份和恢复的官方操作手册。除非另有说明，命令均在项目根目录执行。

## 1. 本地开发

### 1.1 后端

默认使用 SQLite：

```bash
go run ./cmd/server
```

服务启动时会加载 `.env`、执行待处理迁移并初始化必要数据。debug 模式可生成临时 JWT 密钥和一次性管理员密码；release 模式必须显式配置强密钥。

### 1.2 管理后台

```bash
cd web
npm ci
npm run dev
```

Vite 开发服务器默认监听 `http://localhost:3000`，API 请求代理到后端。

## 2. 配置

从模板创建配置：

```bash
cp .env.example .env
```

PowerShell：

```powershell
Copy-Item .env.example .env
```

常用配置组：

| 范围 | 变量 |
|---|---|
| 服务 | `SERVER_HOST`、`SERVER_PORT`、`SERVER_MODE`、`SERVER_BASE_URL` |
| 数据库 | `DB_DRIVER`、`DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD`、`DB_NAME`、`DB_SSL_MODE` |
| 认证 | `JWT_SECRET`、`JWT_ACCESS_TTL`、`JWT_REFRESH_TTL`、`AUTH_ALLOW_REGISTRATION` |
| Redis | `REDIS_HOST`、`REDIS_PORT`、`REDIS_PASSWORD`、`REDIS_DB`、`REDIS_PREFIX` |
| 上传 | `UPLOAD_MAX_SIZE`、`UPLOAD_ALLOWED_TYPES`、`UPLOAD_STORAGE_PATH`、`UPLOAD_URL_PREFIX` |
| S3 | `STORAGE_DRIVER`、`S3_ENDPOINT`、`S3_BUCKET`、`S3_REGION`、`S3_ACCESS_KEY`、`S3_SECRET_KEY`、`S3_USE_SSL`、`S3_PATH_STYLE` |
| Webhook | `QUEUE_MAX_WORKERS`、`QUEUE_MAX_RETRIES`、`QUEUE_RETRY_DELAY` |
| 监控 | `METRICS_ENABLED`、`METRICS_PATH`、`OTEL_ENABLED`、`OTEL_*` |
| 搜索 | `SEARCH_ENGINE`、`MEILISEARCH_URL`、`MEILISEARCH_KEY` |

完整默认值和注释见 [`.env.example`](../.env.example)。

### 2.1 生产密钥要求

- `JWT_SECRET` 至少 32 个随机字符。
- `ADMIN_PASSWORD`、数据库、Redis 和 Grafana 密码必须使用独立强密码。
- `change-me`、`replace-me` 等占位符不能用于 release 模式。
- `.env`、Token、备份和真实客户数据不得提交到 Git。

## 3. 质量验证

### 3.1 完整检查

```bash
make check
```

它会执行 Go 格式化、`go vet`、Swagger 生成、golangci-lint、Go 全量测试，以及前端类型检查、lint、单测和生产构建。

浏览器端到端测试单独运行：

```bash
cd web
npm run test:e2e
```

### 3.2 单项排查

```bash
make fmt
make vet
make swagger
make lint
make test
```

```bash
cd web
npm run type-check
npm run lint
npm run test -- --run
npm run build
```

### 3.3 OpenAPI

修改 REST 路由、请求、响应或 Swagger 注释后运行：

```bash
swag init -g cmd/server/main.go --parseDependency --parseInternal -o docs/api
git diff --exit-code docs/api
```

`docs/api/` 是生成的接口契约，不手工编辑生成文件。

### 3.4 S3 真实服务测试

启动 MinIO、R2 或 AWS S3 测试环境后运行：

```powershell
$env:S3_TEST_ENDPOINT = "127.0.0.1:9000"
$env:S3_TEST_ACCESS_KEY = "minioadmin"
$env:S3_TEST_SECRET_KEY = "minioadmin"
$env:S3_TEST_USE_SSL = "false"
go test ./internal/storage -run TestS3Integration -v -count=1
```

测试覆盖：

- 上传后通过 SDK 读取并比对内容
- 删除后确认对象不存在及重复删除
- V4 预签名 URL 实际下载
- 17 MiB 流式分片上传
- 错误凭据拒绝

测试结束后撤销临时凭据并删除测试 bucket。MinIO 验证通过不代表 R2 或 AWS 已完成供应商级验证。

## 4. Docker Compose 部署

```bash
docker compose config
docker compose up -d --build
docker compose ps
```

启用 Prometheus、Grafana 和 Tempo：

```bash
docker compose --profile monitor up -d --build
```

默认入口：

| 服务 | 地址 |
|---|---|
| 管理后台 | <http://localhost:8080> |
| REST API | <http://localhost:8080/api/v1> |
| Swagger | <http://localhost:8080/swagger/index.html> |
| GraphQL | <http://localhost:8080/api/v1/graphql> |
| 健康检查 | <http://localhost:8080/api/v1/system/health> |
| Prometheus | <http://localhost:9090> |
| Grafana | <http://localhost:3001> |

生产环境不要公开数据库或 Redis 端口。反向代理必须正确设置 TLS、请求体上限和可信代理头。

## 5. 数据库与迁移

```bash
go run ./cmd/server --migrate-status
go run ./cmd/server --migrate
go run ./cmd/server --migrate-down=1
go run ./cmd/server --seed
```

操作规则：

1. 迁移前备份数据库。
2. 在空库和生产数据副本上验证升级路径。
3. 先检查 `--migrate-status`，再决定是否执行回滚。
4. 回滚可能丢失数据，不得在未确认目标迁移和备份可用时执行。
5. 修改 `.env` 中的 `ADMIN_PASSWORD` 不会自动更新已经存在的管理员。

## 6. 备份与恢复

### 6.1 创建备份

管理 API：

```bash
curl -X POST "http://localhost:8080/api/v1/admin/backup?type=all" \
  -H "Authorization: Bearer <ADMIN_TOKEN>"
```

或：

```bash
make backup API=http://localhost:8080 TOKEN=<ADMIN_TOKEN>
```

记录应用版本、Git SHA、数据库类型、schema 版本、文件大小、校验和和创建时间。命令成功不等于备份可恢复。

PostgreSQL plain SQL 备份使用 `--clean --if-exists --no-owner --no-privileges`，以便在目标角色布局不同的环境恢复。数据库 SQL 与媒体归档是两个独立时间点；需要跨两者严格一致时，必须额外安排停写窗口或一致性快照。

### 6.2 灾难恢复

数据库完全丢失时，不要依赖需要登录的 HTTP 恢复接口。使用无需 HTTP 鉴权的 CLI：

```bash
go run ./cmd/server --restore=path/to/backup.sql
```

容器环境应先停止应用写入，再用继承同一数据库和缓存配置的一次性容器执行：

```bash
docker compose stop app
docker compose run --rm app --restore=/app/backups/<backup-file>
```

PostgreSQL CLI 恢复使用 `psql ON_ERROR_STOP=1 --single-transaction`，SQL 任一语句失败会终止并回滚该次恢复。目标 schema 即使完全为空，也必须通过备份 schema 版本和当前模型表集合预检。

恢复成功后，CLI 会连接配置中的缓存后端，只删除 `articles:` 和 `contenttype:` 数据缓存；JWT 黑名单、登录锁和其他安全状态不会被全量清空。使用 Redis 的部署必须向恢复容器提供与应用一致的 `CACHE_DRIVER`、`REDIS_HOST`、`REDIS_PORT`、`REDIS_PASSWORD`、`REDIS_DB` 和 `REDIS_PREFIX`，不要切换到 memory 绕过清理。

如果缓存连接或定向清理失败，CLI 会以非零状态退出，但数据库可能已经恢复成功。此时不要直接重启应用；先恢复缓存连接或人工清理上述数据缓存，再继续验证。

HTTP 恢复会使用脱离客户端取消信号的 10 秒有界上下文执行相同的数据缓存清理，并同步失效当前进程的认证用户、角色和权限缓存；调用方仍须检查响应中的 `cache_warning`。该认证缓存失效只覆盖处理请求的实例，多实例部署必须停止或排空全部应用实例，使用 CLI 恢复后统一重启，不能把在线 HTTP 恢复当作集群级恢复方案。

恢复前：

1. 停止写入和后台任务。
2. 保存当前数据库及媒体目录副本。
3. 确认备份来源、数据库类型和校验和。
4. 在隔离环境预演命令。

恢复后：

1. 检查迁移状态和健康接口。
2. 核对关键表、行数和外键。
3. 验证管理员登录、内容读取和媒体访问。
4. 正常启动应用，确认启动日志中的 `search index warmed up indexed=<数量>` 与数据库内容一致；必要时再执行管理员手动重建。
5. 记录恢复耗时、版本、异常和验证结果。

发布前必须在当前候选提交和目标数据库类型上完成一次隔离恢复演练。历史报告只证明当时的提交通过。

## 7. 可观测性

- 健康检查：`/api/v1/system/health`
- Prometheus：`METRICS_ENABLED=true`，默认 `/metrics`
- OpenTelemetry：`OTEL_ENABLED=true` 并配置 OTLP/HTTP endpoint
- 日志：生产推荐 `LOG_FORMAT=json`

确认指标和 trace 已在真实采集端出现，再宣布可观测性可用。开发机负载结果不能直接作为 SLA。

## 8. MCP

stdio：

```bash
go run ./cmd/server --mcp
```

Streamable HTTP 默认关闭；启用后使用 `/api/v1/mcp`，并携带有明确权限的 API Token。创建、更新和发布工具分别检查对应权限。

发布前至少验证：

1. `tools/list`
2. 公开只读调用
3. 无权限写入拒绝
4. 有权限创建草稿
5. 更新时的乐观锁冲突
6. 显式发布

## 9. 发布检查

- [ ] 当前候选提交通过 `make check`
- [ ] Playwright E2E 通过
- [ ] 数据库迁移和升级路径通过
- [ ] 目标数据库恢复演练通过
- [ ] 外部服务真实集成测试通过
- [ ] OpenAPI 与实现零漂移
- [ ] README、STATUS、ROADMAP、SOP 和 CHANGELOG 同步
- [ ] 版本号、镜像标签和发布说明一致
- [ ] 无密钥、临时文件、测试 bucket 或调试配置

## 10. 故障排查

| 现象 | 首要检查 |
|---|---|
| release 模式启动失败 | JWT、管理员、数据库和 Redis 密钥是否仍为占位符 |
| 登录突然失效 | JWT 密钥、刷新令牌轮换和 Redis 吊销状态 |
| S3 上传失败 | endpoint 是否含协议、SSL/path-style、bucket 权限和凭据 |
| Webhook 不投递 | `webhook_deliveries` 状态、队列配置、SSRF 拒绝和目标返回码 |
| 恢复后无法登录 | 是否走了依赖用户表的 HTTP 恢复路径；改用 CLI |
| 前端构建有警告 | 区分 Sass legacy 或第三方 pure annotation 警告与实际失败 |

仍无法定位时，保存 request ID、结构化日志、版本、迁移状态和最小复现步骤，再创建 Issue；不要附带真实密钥。
