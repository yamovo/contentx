# ContentX 执行路线（ROADMAP）

> 本文件按轮次组织执行计划，每轮完成 → review → 通过后 push → 进入下一轮。产品能力见 [PRD.md](./PRD.md)，操作流程见 [SOP.md](./SOP.md)。

## 工作流

```
每轮流程：
1. 执行该轮任务
2. 按退出门槛逐项检查 → 报告结果
3. 用户确认通过 → git commit + push
4. 进入下一轮
```

轮次状态标记：`✅ 已完成` / `🚧 进行中` / `⏳ 待开始`

## 当前状态

P3-A“生产就绪”整体进度：**Round 1 ✅** / **Round 2 ✅** / **Round 3 ✅** / **Round 4 ✅**（行程 D 完成）。下一轮为 Round 5（P3-A 最终验收）。

## 问题优先级

- **S0**：正确性、安全或数据可信度问题。优先修复，不依赖外部环境。
- **S1**：阻塞生产验收的问题。本轮内尽量解决，依赖外部环境的部分需给出明确阻断条件。
- **S2**：改进项，非阻断。

## Round 1：正确性与构建卫生 ✅

> 对应原“行程 A”。已完成于 2026-07-23。

| 任务 | 问题 | 退出门槛 | 状态 |
|---|---|---|---|
| 1 | S0-5 缺 `.dockerignore` | `.dockerignore` 排除 .git、.env、node_modules、.bin 等；Dockerfile 所有 COPY 不受影响 | ✅ |
| 2 | S0-1 GraphQL 正文回归 | 列表精简优化保留；客户端请求 `content` 时正确返回正文；3 个 GraphQL 回归测试通过 | ✅ |
| 3 | S0-2 REST `content` 字段语义 | `Article.Content` 加 `omitempty`；TS SDK 与前端类型同步；3 个 REST 集成测试通过 | ✅ |
| 4 | 回归测试 | GraphQL 3 + REST 3 测试通过 | ✅ |
| 5 | S0-3/S0-4 跨库结论失效 + MySQL 单位 | 两份报告标记失效历史数据；MySQL 单位修正（×1000）；PostgreSQL 数据集改回 1,000 篇/2,200 字符；根因记为“待定位” | ✅ |
| 6 | S1-2 压测脚本不可复现 | Driver 感知默认端口；Vegeta 从 PATH/工具目录解析；场景间冷却；每场景指标快照 | ✅ |
| 7 | S1-4 Swagger 过期 | 11 个端点加注解；`swag init` 重新生成；CI 漂移检查；README 更新 | ✅ |

## Round 2：备份与恢复生产闭环 ✅

> 对应原“行程 B”。目标：默认 Docker 部署能备份、能恢复、能验证。

### 任务

1. **B1 备份端点**：`POST /api/v1/admin/backup` 触发 `pg_dump` 流式下载；权限限制为 superadmin；幂等防并发。
2. **B2 恢复端点**：`POST /api/v1/admin/restore` 接收备份文件；验证 schema 版本；恢复前自动停写。
3. **B3 备份完整性校验**：备份文件包含 expected tables；restore 后行数与原库一致。
4. **B4 定时备份**：cron 配置；保留策略（数量/天数）；备份失败告警。
5. **B5 端到端演练**：真实 PostgreSQL 容器 → backup → drop DB → restore → 数据一致。

### 退出门槛（Review 检查单）

- [x] 备份端点返回有效 SQL 文件，包含所有业务表 — `TestBackupHandler_Admin_CreateListDownload` + 演练 Scenario A 验证 25MB SQL 含 CREATE TABLE + schema_migrations + articles 数据
- [x] 恢复端点能从备份文件还原，行数与原库一致 — 演练 Scenario A：28 表行数 0 回归，articles 10000→0→10000
- [x] 定时备份按配置执行，保留策略生效 — `BackupScheduler` 7 个测试（cron 解析、分布式锁、保留策略 count/days、失败告警）
- [x] 端到端演练在真实容器中完成，记录 Git SHA + 命令 + 结果 — `reports/backup/e2e-drill-20260723-195645.md`，Git SHA 1d9b923，两场景 PASS
- [x] 非 superadmin 调用备份/恢复端点返回 403 — `TestBackupHandler_NonAdmin_Forbidden` 覆盖 GET/POST/DELETE
- [x] 并发备份请求被正确拒绝 — `TestBackupHandler_ConcurrentBackup_Returns409` 返回 409 BACKUP_IN_PROGRESS
- [x] 文档（SOP.md）更新备份恢复操作步骤 — 新增第 3 节：端点、手动备份恢复、定时备份、灾难恢复、端到端演练

## Round 3：重建可信压测基线 ✅

> 对应原“行程 C”。目标：三库在同一条件下重跑，产出可信对照。已完成于 2026-07-23。

### 任务

1. **C1 统一条件**：同一 Git SHA、同一 10,000 篇数据、同一响应字段、同一速率、同一主机。
2. **C2 PostgreSQL 重跑**：10,000 篇 + 正文精简后版本。
3. **C3 MySQL 重跑**：Linux/WSL 受控环境，排除客户端端口耗尽。
4. **C4 SQLite 重跑**：CGO 构建 + 10,000 篇。
5. **C5 报告自动化**：脚本从 Vegeta JSON 生成 Markdown，禁止手工抄写单位；增加一致性校验。
6. **C6 结论归因**：MySQL 超时/排队根因从“待定位”改为实测归因。

### 退出门槛

- [x] 三库结果来自同一 Git SHA、同一数据集规模、同一响应字段 — 三库均 Git SHA `0f5d624`、10,000 篇、列表省略 `content`、读 1,000 req/s × 15s / 写 100 req/s × 10s
- [x] 每次运行保存实际 `COUNT(*)`、Git SHA、应用配置和响应体大小 — 三库 `run-metadata.json` 均含 timestamp/git_sha/article_count/response_bytes/app_config
- [x] 报告由脚本自动生成，无手工抄写单位 — `generate-report.ps1` 从 Vegeta JSON 生成，ns→ms / bytes→KB 自动换算
- [x] 原始 JSON 与 Markdown 表格一致性校验通过 — 三库报告均输出 "Consistency check: PASS (0 error(s))"
- [x] MySQL 超时/排队根因有实测归因，不再是“待定位” — EXPLAIN 显示 MySQL filesort / SQLite TEMP B-TREE vs PostgreSQL Incremental Sort
- [x] 现有失效历史数据保留为历史快照，新结果独立呈现 — 历史数据在 `raw/<driver>/historical/`，§3 由 Round 3 数据替换
- [x] PROGRESS/ROADMAP 中 7.2 状态更新 — 本节标记 ✅，cross-db-comparison.md §7 同步

## Round 4：CI、发行物与文档收口 ✅

> 对应原“行程 D”。目标：CI 全绿、发行物可用、文档与代码一致。已完成于 2026-07-23。

### 任务

1. **D1 CI 修复**：Swagger 漂移检查（已在 Round 1 加）；golangci-lint v2 配置；前端 type-check 强制。
2. **D2 Release 二进制**：CGO 发行版支持 SQLite；或明确文档说明无 CGO 版本不支持 SQLite（S1-3）。
3. **D3 文档收口**：README 精简为入口+索引；所有性能数字标注为阶段性本机结果，非 SLA；Swagger 描述更新。
4. **D4 贡献者文档**：CONTRIBUTING.md（如需要）；开发环境搭建；测试运行指南。

### 退出门槛

- [x] CI 所有 job 在 main 分支最后一次 push 上为绿色 — Run 30011784054（commit `0a9facc`）：test ✓ 3m35s / frontend ✓ 53s / build ✓ 32s / docker ✓ 21m31s
- [x] Release 二进制在 Linux/Windows/macOS 至少一个平台验证可运行 — build 作业在 ubuntu-latest 编译 `contentx-linux-amd64` 成功并上传 artifact；docker 作业构建多平台镜像（amd64+arm64）并推送 GHCR
- [x] 无 CGO 发行版的 SQLite 限制在 README 和 Release notes 中明确 — README §当前边界 明确标注 `CGO_ENABLED=0` 限制，Release notes 由 `generate_release_notes` 自动包含 commit 描述
- [x] README 中无过期 Swagger 描述 — README 引用 SOP §7 描述 Swagger 生成与漂移检查，无过期端点列表
- [x] 所有性能数字有“阶段性本机结果，非 SLA”标注 — README §阶段性性能基线 首行标注，cross-db-comparison.md 同步

## Round 5：P3-A 最终验收 ⏳

> 对应原“行程 E”。目标：P3-A 整体验收通过。

### 任务

1. **E1 验收清单**：按 PRD §7 完成定义逐项检查 P3-A 所有交付项。
2. **E2 回归测试**：全量 `go test` + 前端 `npm run test` 通过。
3. **E3 端到端验证**：Docker Compose 部署 → 创建内容 → 发布 → 搜索 → 备份 → 恢复。
4. **E4 文档一致性**：PRD/SOP/ROADMAP 与代码状态一致；无失效引用。
5. **E5 Release tag**：打 `v1.1.0` 或下一个里程碑 tag。

### 退出门槛

- [ ] PRD §7 完成定义对所有 P3-A 交付项成立
- [ ] 全量后端测试通过
- [ ] 全量前端测试通过
- [ ] Docker Compose 端到端验证记录完整（Git SHA + 命令 + 结果）
- [ ] PRD/SOP/ROADMAP 间无失效交叉引用
- [ ] Release tag 已打并推送

## 历史问题总表

以下问题在 Round 1 中已修复，保留为历史记录：

| ID | 优先级 | 描述 | 修复轮次 |
|---|---|---|---|
| S0-1 | S0 | GraphQL `articles` 列表不返回 `content` 正文 | Round 1 ✅ |
| S0-2 | S0 | REST 列表 `content` 字段返回空字符串而非省略 | Round 1 ✅ |
| S0-3 | S0 | 跨库压测 PostgreSQL/MySQL 不可比（数据集/响应字段不一致） | Round 1 标记失效，Round 3 ✅ 重跑 |
| S0-4 | S0 | MySQL 压测报告单位错误（纳秒当毫秒，缩小 1000 倍） | Round 1 ✅ |
| S0-5 | S0 | 缺 `.dockerignore`，~2 GiB 无关文件进入构建上下文 | Round 1 ✅ |
| S1-2 | S1 | 压测脚本硬编码 Vegeta 路径、不区分驱动端口 | Round 1 ✅ |
| S1-3 | S1 | 无 CGO 发行版不支持 SQLite | Round 4 ✅ |
| S1-4 | S1 | Swagger 文档过期，缺 search/backup/workflow/translation 端点 | Round 1 ✅ |
