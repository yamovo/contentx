# ContentX 可观测性

ContentX 提供 Prometheus 指标、Grafana 仪表盘和 OpenTelemetry 分布式追踪。追踪通过 OTLP/HTTP 发送至 Tempo，默认关闭；Prometheus 指标默认在 `/metrics` 开启。

## 启动监控栈

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

服务入口：

- ContentX 指标：<http://localhost:8080/metrics>
- Prometheus：<http://localhost:9090>
- Grafana：<http://localhost:3001>（默认账号由 `GRAFANA_USER` / `GRAFANA_PASSWORD` 控制）
- Tempo API：<http://localhost:3200>

Grafana 会自动加载 Prometheus、Tempo 数据源和 ContentX 仪表盘。

如默认端口已被占用，可在 `.env` 中设置 `APP_PORT`、`HTTP_PORT`、`HTTPS_PORT`。应用容器内部仍使用 8080，不影响 Prometheus 抓取。

## 验收记录

2026-07-22 已使用 PostgreSQL、Redis、Prometheus、Grafana、Tempo 完成真实容器验收：应用健康检查返回 200，Prometheus target 为 `up`，Grafana 自动加载两个数据源和 ContentX 仪表盘，真实 HTTP 请求的 `X-Trace-ID` 可从 Tempo API 查询。

## 指标

- `http_requests_total{method,path,status}`
- `http_request_duration_seconds{method,path}`
- `active_users_total`
- `articles_total{status}`
- `db_connections_in_use`
- `cache_hits_total` / `cache_misses_total`
- `webhook_dispatch_total{event,status}`

HTTP 路由参数会统一为 `:param`，避免 Prometheus 标签基数失控。

## 追踪

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
