# RESEARCH-001：竞品与日志/审计方向

| 项目 | 内容 |
|---|---|
| 状态 | 2026-08-22 快照；用于路线决策，不代表永久市场事实 |
| 范围 | Headless CMS 的 MCP/Agent 能力、审计体验与日志可观测性 |
| 证据边界 | 竞品只采用官方文档；ContentX 结论来自当前工作区代码 |

## 1. 结论

Go、MCP 或 RAG 都不能单独构成 ContentX 的市场壁垒。当前更可信的方向是：以语言中立契约交付自托管内容，并把 Agent 变更做成可授权、可审批、可关联、可追责、可回滚的治理链。

因此不应先重写服务端，也不应继续堆 Agent 工具数量。下一顺序应是：

1. 交付默认关闭、published-only 的动态内容公开契约，并用非 Go 客户端验证。
2. 补 schema version、类型/字段可见性和向后兼容的模型演进。
3. 在生产信任阶段建立跨 REST、MCP 与后台任务的统一审计关联和高风险可靠写入。
4. 再交付 `提议 → diff → 审批 → 独立发布 → 审计 → 回滚` 的 Agent 黄金路径。
5. 最后扩展 OTLP 日志、Collector、保留、导出、告警与外部扩展生态。

## 2. 竞品快照

| 产品 | 当前官方能力 | 对 ContentX 的含义 |
|---|---|---|
| Strapi 5 | 内置 MCP server，默认关闭；按 Admin token 暴露 CRUD、publish/unpublish，并在工具、字段、locale 和运行时对象层检查权限 | “有 MCP”已经是同类能力；ContentX 必须用独立发布授权、默认安全契约和可验证治理体现差异 |
| Payload | 官方 MCP 插件支持按 collection/global 开关能力、运行时调整、API key 与既有 access rules | TypeScript/插件生态具有明显采用优势；ContentX 的公共边界不能要求消费者或扩展作者理解 Go 类型 |
| Directus | 已有 MCP/AI Chat 路线；Activity Log/Revision 记录 actor、action、时间、IP、User-Agent、collection/item，并强调 AI 与人工操作进入同一审计轨迹 | Agent 接入统一审计已成为成熟产品方向；ContentX 必须统一 REST/MCP/后台事件，不能只在 HTTP 中间件补日志 |
| Strapi 审计与历史 | Audit Logs 提供可搜索/筛选的管理操作历史但属于 Enterprise；Content History 属于 Growth/Enterprise | 开源自托管的完整审计与回滚可能形成价值，但只有在可靠性、租户隔离和敏感数据边界完成后才能对外承诺 |

## 3. ContentX 当前日志与审计基础

已存在：

- `slog` 文本/JSON 输出与可选文件输出；HTTP access log 包含 method、path、status、latency、request ID 和 trace ID。
- W3C trace context、HTTP span、GORM span 与可选 OTLP/HTTP trace exporter。
- `ActivityLog`、HTTP mutation/denial 审计和业务级 `AuditLogger`；详情具有递归敏感字段脱敏。
- 审计列表支持 tenant、entity、action、user 与分页筛选。

当前缺口：

1. `ActivityLog` 没有一等字段保存 request ID、trace ID、span ID、来源通道、actor/token 类型和 outcome，业务审计无法稳定跳转到运行日志与 trace。
2. 业务审计是 best-effort，写失败只报警且不影响高风险操作；发布、权限、租户和密钥变更需要事务 outbox 或等价可靠写入策略。
3. REST、MCP、Webhook、定时任务和后台 RAG 尚未使用同一个审计事件 envelope，可能重复记录，也可能缺少 actor/IP/trace。
4. access log 当前记录原始 query；还缺统一敏感字段规则、保留/删除周期、导出、完整性保护和租户级访问测试。
5. 当前 OTLP 只覆盖 trace；尚未验证日志经 Collector 与 trace/span 关联，也没有真实后端查询、告警和失败降级证据。

OpenTelemetry 的官方方向是让日志携带 TraceId、SpanId 与统一 Resource 属性，再由 Collector 处理和导出。ContentX 已有 trace ID 基础，下一步应先统一事件 envelope，而不是立即替换 `slog` 或绑定某个日志厂商。

## 4. 最小日志/审计交付切片

在 Agent 黄金路径前完成：

- 定义版本化 `AuditEvent` envelope：event ID、request ID、trace ID、span ID、tenant、actor 类型/ID、source、action、entity、entity ID、outcome、safe diff、timestamp。
- REST、MCP 和一个后台任务采用同一 envelope，并证明同一操作只产生一条可关联的业务事件。
- publish/unpublish、角色权限、租户切换、Token/密钥操作使用事务 outbox 或明确的 fail-closed 策略。
- Activity Log UI/API 支持按 event/request/trace、actor、source、outcome、时间筛选；详情只显示经过字段策略处理的 diff。
- tenant A 不能枚举 tenant B 事件；平台级事件只有平台权限可读。
- JSON stdout 保持默认；先用 Collector file/stdout receiver 验证 trace/log 关联，再决定是否直接接入 OTLP logs SDK。

## 5. 退出指标

- 审计完整率：纳入清单的高风险成功/拒绝操作均有且只有一条业务事件。
- 关联成功率：可从 event 或 request ID 定位对应 trace 与运行日志。
- 敏感数据测试：密码、Token、Cookie、Authorization、Webhook 查询凭据和 AI provider key 均不会出现。
- 可靠性测试：数据库/Collector 故障时，高风险事件不静默丢失，恢复后可重放且不重复。
- 租户攻击测试：跨租户列表、详情、筛选和标识符猜测全部拒绝。

## 6. 官方资料

- [Strapi MCP server](https://docs.strapi.io/cms/features/strapi-mcp-server)
- [Strapi Audit Logs](https://docs.strapi.io/cms/features/audit-logs)
- [Strapi Content History](https://docs.strapi.io/cms/features/content-history)
- [Payload MCP Plugin](https://payloadcms.com/docs/plugins/mcp)
- [Directus Activity Log](https://directus.com/features/activity-log)
- [Directus v11.14：AI Chat 与 accountability](https://directus.com/resources/directus-v11-14-release)
- [OpenTelemetry Logging](https://opentelemetry.io/docs/specs/otel/logs/)

## 7. ContentX 代码证据

- [`internal/logger/logger.go`](../internal/logger/logger.go)
- [`internal/middleware/middleware.go`](../internal/middleware/middleware.go)
- [`internal/middleware/tracing.go`](../internal/middleware/tracing.go)
- [`internal/services/audit_logger.go`](../internal/services/audit_logger.go)
- [`internal/models/models.go`](../internal/models/models.go)
- [`internal/observability/tracing.go`](../internal/observability/tracing.go)
