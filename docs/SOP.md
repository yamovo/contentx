# 标准操作流程

本文档提供 ContentX 的开发、验证、部署、备份和故障恢复操作。所有命令默认在项目根目录执行。

## 本地开发

### 后端

默认配置使用 SQLite：

```bash
go run ./cmd/server
```

服务启动时会执行待处理的数据库迁移并初始化必要数据。

### 管理后台

```bash
cd web
npm ci
npm run dev
```

## 配置

从模板创建本地配置：

```bash
cp .env.example .env
```

生产环境至少必须设置强随机的 `JWT_SECRET`、`ADMIN_PASSWORD`、数据库密码、Redis 密码和 Grafana 密码。不要提交 `.env` 或真实凭据。

常用配置组：

| 范围 | 变量 |
|---|---|
| 服务 | `SERVER_HOST`、`SERVER_PORT`、`SERVER_MODE`、`SERVER_BASE_URL` |
| 数据库 | `DB_DRIVER`、`DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD`、`DB_NAME` |
| Redis | `REDIS_HOST`、`REDIS_PORT`、`REDIS_PASSWORD`、`REDIS_DB` |
| 存储 | `STORAGE_DRIVER`、`UPLOAD_STORAGE_PATH`、`S3_*` |
| 监控 | `METRICS_ENABLED`、`OTEL_ENABLED`、`OTEL_*` |
| 搜索 | `SEARCH_ENGINE`、`MEILISEARCH_URL`、`MEILISEARCH_KEY` |

完整列表及默认值见 [`.env.example`](../.env.example)。

## 验证

提交前运行：

```bash
make check
```

需要单独排查时可运行：

```bash
make fmt
make vet
make swagger
make lint
make test
```

前端单独验证：

```bash
cd web
npm ci
npm run lint
npm run test
npm run build
```

如果修改 REST 路由或注释，执行 `make swagger` 并提交更新后的 `docs/api/` 文件。

## Docker Compose 部署

创建并检查 `.env` 后启动：

```bash
docker compose up -d --build
```

启用监控组件：

```bash
docker compose --profile monitor up -d --build
```

检查服务：

```bash
docker compose ps
curl http://localhost:8080/api/v1/system/health
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

## 数据库管理

```bash
go run ./cmd/server --migrate-status
go run ./cmd/server --migrate
go run ./cmd/server --migrate-down 1
go run ./cmd/server --seed
```

回滚迁移可能造成数据丢失，只能在确认目标版本和备份可用后执行。

## 备份与恢复

### 创建备份

使用管理接口创建备份，或运行项目提供的备份命令：

```bash
make backup
```

记录备份文件、数据库类型、应用版本、schema 版本和创建时间。不要只以命令退出码判断备份可用。

### 恢复

先停止写入并保存当前数据库副本，然后执行：

```bash
go run ./cmd/server --restore path/to/backup
```

容器环境可在应用容器中执行同一子命令。恢复后必须：

1. 检查健康接口。
2. 核对关键表和行数。
3. 验证管理员登录和内容读取。
4. 重建或验证搜索索引。
5. 记录恢复耗时与异常。

生产发布前应在隔离环境完成一次与生产数据库类型相同的恢复演练。

## 可观测性

- 健康状态：`/api/v1/system/health`
- Prometheus 指标：`/metrics`
- OpenTelemetry：设置 `OTEL_ENABLED=true` 并配置 `OTEL_EXPORTER_OTLP_ENDPOINT`

告警和 SLA 必须基于目标部署环境重新设定，开发机压测数据不能直接作为生产承诺。

## MCP

stdio 模式要求数据库已迁移并初始化：

```bash
go run ./cmd/server --mcp
```

远程 MCP 默认关闭。启用后使用 `/api/v1/mcp`，客户端必须携带 API Token。写工具还会检查对应的内容创建、编辑或发布权限。

建议用 MCP Inspector 完成 `tools/list`、只读调用、无权限拒绝和受权写入四类冒烟测试。

## 发布检查

发布前逐项确认：

- `make check` 通过
- 前端 lint、测试和构建通过
- 数据库迁移和回滚路径已检查
- 备份恢复演练有效
- OpenAPI 与实现同步
- README、状态、路线图和变更日志同步
- 密钥、调试配置和本地产物未进入版本库
