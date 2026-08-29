# ContentX 路线图

本文档只记录尚未完成的里程碑、优先级和退出条件。路线决策必须服务“面向开发者和 AI Agent 的自托管 Headless CMS”这一[产品定位](./POSITIONING.md)。当前服务端以 Go 构建，但公共契约和扩展边界必须保持语言中立。当前事实以[项目状态](./STATUS.md)为准，历史变化见[变更日志](../CHANGELOG.md)和[归档](./archive/)。

## 1. 规划原则

- 安全、数据正确性和可恢复性优先
- 每个里程碑必须有可验证的退出条件
- 依赖外部服务的能力必须运行真实集成测试
- 未完成能力不得写入已交付清单
- 完成里程碑时同步 STATUS、SOP 和 CHANGELOG
- 新能力优先强化自托管体验、语言中立的开发者契约、Agent 安全边界或多租户隔离

跨里程碑按以下产品顺序推进：

0. **语言与扩展边界决策**：比较保留 Go、Go 核心加语言中立扩展、混合架构和迁移的成本；在证据前不直接重写。
1. **补齐 Headless CMS 本体**：公开动态内容交付、内容模型安全演进、真实后端 E2E 和发布制品一致性。
2. **闭环生产信任边界**：TenantGuard、tenant A/B 攻击矩阵、Token/MCP principal、统一审计关联与高风险可靠写入、RAG 生命周期、外发与成本控制、真实 CI 与恢复验证。
3. **交付受治理的 Agent 黄金路径**：`Agent 提议 → 确定性校验与 diff → 人工或策略审批 → 独立发布授权 → 审计与回滚`。
4. **升级日志、审计和可观测性**：在生产信任阶段的关联/可靠写入基础上，完成 Collector、OTLP logs、保留、导出、完整性保护和告警。
5. **改善开发者采用与平台能力**：持久向量后端、多语言 SDK、框架示例和部署模板；外部插件与复杂商业化在真实需求验证后推进。
6. **市场验证**：由设计合作伙伴、首次发布时间、首次受治理 Agent 修改时间、审计完整率和持续使用率决定后续投入。

阶段顺序不等于暂停既有阻断项。当前仍须并行关闭多租户、RAG、Release 和 SDK 的生产风险；新能力先通过 [ADR-001](./ADR-001-language-and-extension-boundary.md) 和 [RFC-002](./RFC-002-public-content-delivery.md) 固定边界，再转为版本承诺。竞品与日志证据见 [RESEARCH-001](./RESEARCH-001-market-and-observability.md)；其结论是审计关联与高风险可靠写入必须在 Agent 黄金路径前完成。

## 2. 已完成里程碑

| 里程碑 | 主要结果 |
|---|---|
| 基础 CMS | 内容、用户、角色、管理后台和三种数据库支持 |
| 工程化 | 分层架构、迁移、缓存、测试、CI 和 OpenAPI |
| 生产基础 | Docker Compose、监控、备份恢复和发布流程 |
| 安全与稳定性整改 | 权限边界、GraphQL 限制、Webhook 队列、审计和乐观锁 |
| MCP 基础 | stdio、HTTP、资源、提示词和受权限控制的写工具 |
| S3 存储生产化 | `minio-go/v7`、V4 签名、预签名下载和 MinIO 真机分片验证 |
| 发布候选收口（v1.4.0） | PostgreSQL 恢复加固与演练、审查修复 `504d510` 随 PR #1 合并、版本整理与发布 |

## 3. 当前执行阶段：架构边界、CMS 交付与生产信任收口

### 3.1 下一执行批次

1. ~~建立 [ADR-001](./ADR-001-language-and-extension-boundary.md) 的初始决策。~~ — 已完成：暂定 Go 核心，但公共 API、MCP、事件、SDK 和外部扩展保持语言中立；重写与外部扩展形态继续由验证证据决定。
2. ~~关闭动态内容 create/update/import/translation 绕过独立发布流程的路径，并补权限拒绝回归。~~ — 已完成：草稿发布类型只能以 draft 创建/导入，更新 status 被拒绝，翻译不能直接发布。
3. 按 [RFC-002](./RFC-002-public-content-delivery.md) 实现默认关闭、显式 allowlist、published-only 的公开动态内容 REST 列表和单项查询。
4. 使用专用公开 DTO、分页上限和 default tenant 固定策略；同步 OpenAPI 与 TypeScript 消费者冒烟。
5. 为内容模型增加 schema version、公开类型/字段策略和统一完整数据校验，再开放仅向后兼容的 schema update。

本批次退出条件：

- ~~只有 create/update 权限的主体不能通过 status、导入或翻译公开内容。~~ — 已通过 Service 与 Handler 回归验证。
- 公开动态内容接口默认关闭，永不返回草稿、非 allowlist 类型、私有/遗留字段或内部 actor/type 标识。
- 一个非 Go 客户端无需理解内部 Go 类型即可完成公开内容消费流程。
- OpenAPI 与实现零漂移，替换内部实现不会改变契约测试。

### 3.2 已在进行的 P0 阻断项

多租户和 RAG 已形成可运行原型，但生产安全退出条件尚未全部通过，当前状态与验证盲区见[项目状态](./STATUS.md)。

按依赖顺序推进：

1. ~~定义租户模型、租户上下文和迁移策略。~~ — 已完成：RFC-001 已产出；迁移 008/009 已实现。
2. 完成 Repository 与横切能力的租户范围：核心内容查询已推进，Settings 的事务、范围唯一性及 SQLite/PostgreSQL/MySQL 验证已关闭；Token、评论关系和平台审计仍待关闭。
3. 完成运行时 TenantGuard：统一 access/refresh、TenantMembership 角色、租户 active 状态和 API Token/MCP principal 再验证。
4. 补齐 REST、GraphQL、MCP、Webhook、搜索、缓存和后台任务的真实 tenant A/B 攻击测试。
5. 建立版本化审计事件 envelope，将 request/trace/span、tenant、actor、source 和 outcome 关联起来，并为发布、权限、租户与密钥操作增加可靠写入策略。
6. 完成 RAG 最小安全闭环：清理孤儿向量、修复定时发布和失败补偿，将权限、外发、限流、审计和成本边界统一覆盖 REST/MCP。
7. 交付链验证：Linux amd64 已改为 CGO 原生构建、携带 `web/dist` 并加入归档 smoke；手动 dispatch 只验证不发布，仍需远程 Ubuntu workflow，其他平台需原生 CGO runner 与同等 smoke 后再加入。
8. 完成 TypeScript SDK 工程闭环：锁文件、typecheck/test/build/pack、示例和 CI 已落地；仍需远程空环境 `npm ci` 与消费者冒烟。
9. 同步 OpenAPI、SOP 和部署文档。
10. 租户管理 API、成员管理和切换器；tenant-aware Vue Query 缓存。
11. 增加按租户与接口维度的配额及用量计量。

退出条件：

- 所有业务数据表及全局/租户覆盖模型具有明确且经过验证的归属规则。
- REST、GraphQL、Webhook、MCP、缓存、搜索和定时任务不能绕过租户边界。
- 跨租户读取、更新、删除、标识符猜测和身份刷新均有拒绝测试。
- RAG 的发布、下架、删除、缩短更新、WarmUp、Reindex 和失败恢复不产生可检索孤儿数据。
- REST 与 MCP 的 AI 权限、外发、限流、审计和成本边界采用同一套可验证策略。
- REST、MCP 和后台任务的高风险事件可按 event/request/trace 关联，审计写入故障不会静默丢失。
- 数据迁移、备份和恢复支持租户字段并有升级验证。
- 所有声明支持的独立包目标均在 tag CI 中完成迁移、启动、健康检查和管理后台冒烟测试。
- TypeScript SDK 可从锁文件干净安装，并通过 typecheck、test、build 和 pack 验证。

## 4. 持续改进队列

以下工作不阻断当前里程碑，按风险和投入排期：

| 优先级 | 主题 | 验收方向 |
|---|---|---|
| 高 | 测试覆盖率与执行时间 | 覆盖率门槛持续上调；慢包有基准与拆分方案 |
| 高 | 真实后端 E2E | 覆盖角色权限、发布工作流、媒体、备份和 Webhook |
| 高 | 语言中立契约验证 | OpenAPI/SDK 消费者不依赖 Go 类型；进程外扩展或 WASM spike 具备授权、超时和审计 |
| 高 | 动态内容公开交付 | 默认关闭、published-only、显式类型/字段策略、租户攻击测试和取消发布即时失效 |
| 高 | CI 必需检查 | Playwright、前端 ESLint、TypeScript SDK 及 PostgreSQL/MySQL Settings 门禁已写入 workflow；仍需远程验证、为 `main` 配置 required checks，并固定 Swagger 生成器版本 |
| 高 | 可观测性真机验证 | Prometheus 指标与 OTLP trace 在真实采集端可查询 |
| 高 | 审计关联与可靠写入 | 统一 event/request/trace、actor/source/outcome；高风险事件可重放且不重复，tenant A/B 隔离通过 |
| 中 | 前端架构收口 | TOTP 迁入统一查询层；剩余路由使用 `pages/` 薄壳 |
| 中 | 设计系统采用 | 主要列表、表单、空态、错误态使用共享 UI 组件 |
| 中 | OpenAPI 维护 | 已挂载 REST 路由注释覆盖完整，生成结果零漂移 |
| 中 | 无障碍 | WCAG 2.2 AA、键盘流程、axe 和 reduced-motion 验证 |
| 低 | 前端构建告警和体积 | 消除 Sass legacy API；控制 DashboardCharts 分块 |
| 低 | 管理体验 | 评论 URL 预筛选、管理员密码重置命令 |

完整管理后台国际化、动态 GraphQL、Meilisearch、外部插件沙箱、媒体处理、计费和 SSO 仍属于候选方向，未进入承诺里程碑。当前 Go `Plugin` interface 只用于内置插件，不视为外部插件 ABI。

## 5. 状态定义

| 状态 | 含义 |
|---|---|
| 计划中 | 已定义目标，尚未开始 |
| 进行中 | 已有实施，但退出条件未全部通过 |
| 已完成 | 退出条件有证据，状态和变更日志已同步 |
| 延后 | 当前里程碑不再承诺，需要重新排期 |
