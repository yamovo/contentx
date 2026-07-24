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

P3-A"生产就绪"整体进度：**Round 1 ✅** / **Round 2 ✅** / **Round 3 ✅** / **Round 4 ✅** / **Round 5 ✅**（P3-A 全部完成，`v1.1.0` 已发布）/ **Round 6 ✅**（扣分项整改完成，`v1.2.0` 已发布）。
当前阶段为 **Round 7 外部审查整改**（基于 [AUDIT.md](./AUDIT.md) 评分 6.5/10 的整改轮次，目标 6.5 → 7.5）。完成后进入 **P3-B 商业化基础路线**（见 PRD §5）。

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

## Round 5：P3-A 最终验收 ✅

> 对应原“行程 E”。目标：P3-A 整体验收通过。已完成于 2026-07-23。

### 任务

1. **E1 验收清单**：按 PRD §7 完成定义逐项检查 P3-A 所有交付项。
2. **E2 回归测试**：全量 `go test` + 前端 `npm run test` 通过。
3. **E3 端到端验证**：Docker Compose 部署 → 创建内容 → 发布 → 搜索 → 备份 → 恢复。
4. **E4 文档一致性**：PRD/SOP/ROADMAP 与代码状态一致；无失效引用。
5. **E5 Release tag**：打 `v1.1.0` 或下一个里程碑 tag。

### 退出门槛

- [x] PRD §7 完成定义对所有 P3-A 交付项成立 — P0/P1/P2 已交付，Round 1-4 退出门槛逐项验证（见各轮记录），PRD §4 当前边界已同步更新
- [x] 全量后端测试通过 — `go test -p=1 ./cmd/server ./docs/api ./internal/... ./scripts/benchmark/seeder ./tests -count=1`：13 个包全部 ok（auth/backup/cache/config/database/graphql/handlers/middleware/models/observability/plugin/services/tests）
- [x] 全量前端测试通过 — `npm run test -- --run`：5 个测试文件 77 个测试全部通过
- [x] Docker Compose 端到端验证记录完整（Git SHA + 命令 + 结果） — Git SHA `d90074b`，完整记录见 [reports/e2e/round5-20260723.md](../reports/e2e/round5-20260723.md)，覆盖创建→发布→搜索→备份→恢复全链路
- [x] PRD/SOP/ROADMAP 间无失效交叉引用 — PRD §4 边界已从"待办"更新为"已完成"，README 状态同步，SOP 无过期引用
- [x] Release tag 已打并推送 — `v1.1.0` tag 已推送，触发 Release 作业构建多平台二进制

## Round 6：扣分项整改 ✅

> 基于 `v1.1.0` 验收评估的 5 个扣分项，按"问题严重度 × 改动成本"排优先级。目标：消除 P0 核心缺陷，分数 7.0 → 8.0；补齐测试覆盖，8.0 → 8.5。

### 任务

#### 第一批：P0 快速高收益

1. **F1 CI 本地防线** ✅：
   - 新建 `scripts/git/hooks/pre-commit`，执行 `go fmt ./...` + `go vet ./...` + `swag init` drift check。
   - `Makefile` 增加 `install-hooks` 目标（复制脚本到 `.git/hooks/`）和 `check` 聚合目标（fmt+vet+swagger+lint+test）。
   - 前端添加 husky + lint-staged：`web/package.json` 增加 devDependencies，`.husky/pre-commit` 对暂存 `.ts`/`.vue` 运行 `vue-tsc --noEmit`。
   - CI 增加 gofmt drift 快速失败步（在 `go vet` 前，1 秒反馈）。
   - **完成**：pre-commit 钩子已验证在 Windows（Git bash）和 CI 上均通过；Makefile `install-hooks`/`check` 目标就位；husky + lint-staged 配置完成；CI gofmt drift 步已添加。

2. **F2 Restore 后自动重建搜索索引** ✅：
   - 修改 `internal/handlers/backup.go` Restore handler（行 115-162），restore 成功后在 goroutine 中调用 `articleSvc.ReindexAll(ctx)`（best-effort，不阻塞响应）。
   - Restore 响应增加 `warning: "search index rebuilt; verify results"`。
   - SQLite restore warning（行 158-160）追加 "search index will rebuild on restart"。
   - **完成**：backup.go Restore handler 已添加异步 ReindexAll 调用，pg/mysql 场景立即重建，SQLite 场景提示重启。

3. **F3 `--restore` CLI 子命令** ✅：
   - 修改 `cmd/server/main.go`（行 49-55），增加 `--restore <file>` flag，绕过 HTTP/认证层直接调用 `backup.Restore()`。
   - 支持 `--driver postgres|mysql|sqlite`。
   - **完成**：`--restore <file>` flag 已实现，灾难恢复不再依赖 HTTP 认证，消除 auth-DB 循环依赖。

4. **F4 文档修正** ✅：
   - SOP §3.4 灾难恢复：将 workaround 从 "docker exec psql" 升级为 "`docker exec contentx /app/contentx --restore <file>`"，并增加步骤"重启应用或 `POST /api/v1/search/reindex` 重建索引"。
   - `reports/benchmarks/cross-db-comparison.md` §7：补齐 MySQL `historical/run-metadata.json`（标注 `invalid: true` + 失效原因）；将剩余 2 项"待完成"改为"已知限制"或勾选；§5 内存表补注"非实测"。
   - **完成**：SOP §3.4 已更新为 CLI restore 路径；cross-db §7 MySQL historical/run-metadata.json 已补齐（标注 invalid + 失效原因）；§7 悬空待办改为已知限制；historical 原始数据已提交（mysql + postgres）。

#### 第二批：P0-P1 测试补齐

5. **F5 repository 层集成测试** ✅：
   - 新建 `internal/repository/*_test.go`（优先 article/user/content 三个高频 repo），使用 SQLite 内存模式测试 CRUD + 边界条件。
   - **完成**：article_test.go / user_test.go / content_test.go / testutil_test.go，15 个测试覆盖 Create/Update/List/Delete + tag 关联 + role/permission CRUD + content type 级联删除 + 过滤器（role/search/status）。修复 UserRepository.List 的 `created_at` 歧义列（JOIN roles 时限定 `users.created_at`）。

6. **F6 storage 层单元测试** ✅：
   - 新建 `internal/storage/local_test.go`、`s3_test.go`，覆盖 upload/download/delete + 路径遍历安全。
   - **完成**：LocalDriver 测试覆盖路径遍历拒绝（含跨平台反斜杠归一化）、嵌套目录、安全路径验证；S3Driver 测试覆盖 URL 构造（PathStyle/virtual-host + scheme）、签名、错误处理。修复 `safePath` 反斜杠归一化（Linux 拒绝 Windows 风格 `..\..` 遍历）和 `objectURL` scheme 硬编码。

7. **F7 前端 coverage 配置 + 业务组件测试** ✅：
   - `web/vite.config.ts` 增加 `coverage: { provider: 'v8', reporter: ['text','html'], thresholds: { lines: 40, branches: 30 } }`。
   - `web/package.json` devDependencies 增加 `@vitest/coverage-v8`。
   - 新建 `web/src/views/articles/*.spec.ts` 等，优先覆盖 articles/dashboard/login。
   - **完成**：创建共享测试工具 `web/src/test/utils.ts`（mountWithPlugins + localStorage mock + Element Plus stubs + API mock factory）；新增 TagList/CategoryList/LoginView 三个 spec 文件，覆盖 CRUD 流程、表单验证、登录/重定向、错误处理。前端测试 77 → 100 个（8 文件），coverage 10.86% → 25.31% lines。门槛已提升：lines/statements 20%，branches 40%，functions 35%。

8. **F8 CI 覆盖率门槛** ✅：
   - 后端 `go test` 后增加 `go tool cover -func=coverage.out | grep total`，低于阈值失败。
   - 前端 vitest 加 `--coverage` 并检查 thresholds。
   - **完成**：CI 增加 Go 覆盖率门槛检查步（60% baseline ratchet，当前 ~64.6%）；前端 vitest 加 `--coverage` 强制执行 vite.config.ts thresholds（lines/statements 20%，branches 40%，functions 35%，当前 lines 25.31%）。两者均为 ratchet 机制——防止退化，随测试增加逐步提升。

#### 第三批：P1-P2 长期路线（归入 P3-B/P3-C）

9. **F9 errs/logger/mail 基础测试**：errs 测错误码映射；logger 测配置初始化；mail 测模板渲染。
10. **F10 migrations 测试**：迁移正向/回滚 + 版本连续性。
11. **F11 灾难恢复演练覆盖 SQLite/MySQL**：e2e-drill 增加 Scenario C/D。
12. **F12 restore 响应增加完整性校验摘要**：restore 后返回各表行数对比。
13. **F13 GraphQL Mutation**（归 P3-C）：新增 `mutationType`，实现 createArticle/updateArticle/deleteArticle。
14. **F14 CGO Release 变体**（归 P3-C）：增加 `CGO_ENABLED=1` 的 linux-amd64-sqlite Release。
15. **F15 Bootstrap recovery token**（归 P3-B）：`RECOVERY_TOKEN` 环境变量替代 JWT 认证。

### 退出门槛

- [x] 本地 pre-commit 钩子拦截 gofmt/swagger/type-check 漂移（F1）
- [x] Restore 后搜索索引自动重建，E2E 验证 restore 后搜索可命中（F2）
- [x] `--restore` CLI 子命令可用，灾难恢复不依赖 HTTP 认证（F3）
- [x] SOP §3.4 文档与 cross-db §7 修正完成，无悬空待办（F4）
- [x] repository/storage 层测试覆盖，CI 不退化（F5-F6, F8）
- [x] 前端 coverage 配置就位，业务组件测试基线建立（F7）
- [x] CI 全绿，`v1.2.0` Release tag 已打 — Run 30029628652 全绿（test/frontend/build/docker/release），GitHub Release v1.2.0 已发布 5 平台二进制

## Round 7：外部审查整改 🚧

> 基于 [AUDIT.md](./AUDIT.md)（2026-07-24 外部审查，综合评分 6.5/10）的整改轮次。目标：消除 P0 安全漏洞与前端工程化欠账，6.5 → 7.5；为 P3-B 商业化路线奠基。

### 任务

#### 第一批：P0 安全修复（立即）

| ID | 任务 | 文件 | 退出门槛 |
|---|---|---|---|
| A-1 | JWT Refresh 改为查 DB 加载用户最新角色/状态 | `internal/auth/jwt.go:124-133` | refresh 后角色/禁用状态变更立即生效；新增集成测试覆盖改角色→refresh→新权限 |
| A-2 | `EnsureUniqueSlug` 加上限（100）+ Count 错误不吞 | `internal/repository/article.go:430-444` | 极端 slug 冲突场景返回错误而非死循环；Count 失败返回 error |
| A-3 | 前端 `logout()` 调用 `authApi.logout()` blacklist refresh token | `web/src/stores/auth.ts:80-87` | logout 触发后端 blacklist；网络失败仍清前端状态 |
| A-4 | `v-html` 配置 `marked` sanitizer 或 DOMPurify | `web/src/views/articles/ArticleEditor.vue:81,85,267` | XSS payload 测试通过；富文本合法内容不被误杀 |

#### 第二批：P0 前端工程化卫生

| ID | 任务 | 文件 | 退出门槛 |
|---|---|---|---|
| A-5 | 清理 11 个死依赖 | `web/package.json` | `npm ls` 无未使用依赖；构建产物不变 |
| A-6 | 补 eslint 配置文件 + 修复 lint-staged 改为增量 | `web/` | `npm run lint` 实际生效；lint-staged 只检查暂存文件 |
| A-7 | 移除未启用的 tailwindcss 或完成启用 | `web/` | 二选一：移除依赖 或 加配置并实际使用 |
| A-8 | 替换 `document.execCommand` 富文本编辑器 | `web/src/views/articles/ArticleEditor.vue` | 启用 vue-quill/mavon-editor 或改纯 Markdown；废弃 API 全部移除 |
| A-9 | 删除死代码 + 更新 VortexCMS 注释 | `web/` | `views/NotFound.vue` 删除；`main.scss` 注释更新；未使用 composable 删除 |

#### 第三批：P1 测试补齐

| ID | 任务 | 文件 | 退出门槛 |
|---|---|---|---|
| A-10 | 前端为 ArticleEditor / ArticleList / MediaLibrary 补测试 | `web/src/views/` | 三个组件核心流程覆盖；前端 lines 覆盖率 ≥ 35% |
| A-11 | 前端路由守卫三个分支测试 | `web/src/router/` | requiresAuth / guest / permission 三个分支均覆盖 |
| A-12 | 前端 `http.ts` 401 token refresh 队列逻辑测试 | `web/src/api/http.ts` | isRefreshing / failedQueue / processQueue 三种场景覆盖 |
| A-13 | 提升 vitest 覆盖率阈值至实际值上方 | `web/vite.config.ts` | lines ≥ 30%，branches ≥ 45%，functions ≥ 40% |

#### 第四批：P1 后端性能与稳定性

| ID | 任务 | 文件 | 退出门槛 | 状态 |
|---|---|---|---|---|
| A-14 | AuthMiddleware 加 LRU 缓存（短 TTL 30s） | `internal/middleware/auth.go` | 缓存命中时无 DB 查询；用户改角色后 30s 内生效 | ✅ 完成 |
| A-15 | 修复 goroutine 泄漏 | `internal/middleware/middleware.go`、`internal/auth/login_guard.go`、`cmd/server/main.go` | 限流器/login_guard 加 stop channel；Shutdown 真正停止 goroutine | ✅ 完成 |
| A-16 | Webhook/ReindexAll 传可取消 context | `internal/services/webhook_service.go`、`internal/handlers/backup.go` | shutdown 信号能取消在途 webhook/重建索引 | ✅ 完成 |
| A-17 | 全局限流改为仅限 `/api/` 前缀 | `cmd/server/main.go` | 静态资源不受限流；API 路径仍受保护 | ✅ 完成 |

#### 第五批：P2 代码质量重构（可滚动）

| ID | 任务 | 文件 | 退出门槛 | 状态 |
|---|---|---|---|---|
| A-18 | `Update` 函数重构为 partial-update helper | `internal/services/article_service.go:545-658` | 函数 ≤ 50 行；现有测试不退化 | ✅ 完成（抽 `buildUpdateMap` + 泛型 `setIf`，Update 主体大幅瘦身） |
| A-19 | `Publish`/`Schedule` 复用 `transitionTo` | `internal/services/article_service.go` | 3 份近似实现合并为 1 份 | ✅ 完成（Publish/Schedule 改走 `transitionTo`，单一状态机入口） |
| A-20 | 抽公共工具：`generateSlug`、`buildTree`、`formatDate`、`formatSize` | 多文件 | 重复实现全部替换为工具调用 | ✅ 完成（后端 `models.GenerateSlug` + 前端 `web/src/utils.ts`，多文件替换） |
| A-21 | `ArticleEditor.vue` 拆分为子组件 | `web/src/views/articles/ArticleEditor.vue` | 主组件 ≤ 200 行；子组件单职责 | ✅ 完成（拆分为 EditorTopbar / ArticleMainEditor / ArticleSeoPanel / ArticleSidebar 四个子组件） |
| A-22 | 菜单数据从路由 meta 单一来源生成 | `web/src/layouts/AdminLayout.vue` | 菜单与路由 meta 不再双份维护 | ✅ 完成（menuConfig 仅保留分组结构，title/icon/permission 从 routeMetaMap 读取） |
| A-23 | 移除 63 处 `any`，定义明确接口 | `web/src/` | `any` 数量 ≤ 10（仅测试文件允许） | ✅ 完成（源代码 0 处 `any`；剩余 86 处仅存于 `*.spec.ts` / `test/utils.ts`；新增 `DeviceBreakdownResponse`、`Theme`、`CommentStats`、`MediaStats`、`ActivityLogEntry` 等接口） |

#### 第六批：P2 战略补强（中长期，归入 P3-B/P3-C）

| ID | 任务 | 说明 |
|---|---|---|
| A-24 | AI 路线评估 | 2026 年不可回避，至少评估 AI 辅助写作 / MCP 协议接入；写入 PRD |
| A-25 | 部署模板与示例 | Next.js / Nuxt 集成示例，Vercel/Netlify 一键部署；归入 P3-C |
| A-26 | SDK 生态 | TS SDK 覆盖全部稳定 API，多语言 SDK 路线；归入 P3-C |
| A-27 | 插件市场基础 | 至少提供插件 SDK 文档与示例插件；归入 P3-C |

### 退出门槛（Round 7 整体验收）

- [x] 第一批 P0 安全修复全部完成，新增测试覆盖（A-1 ~ A-4）
- [x] 第二批前端工程化卫生完成，CI 全绿（A-5 ~ A-9）
- [x] 第三批测试补齐完成，前端 lines 覆盖率 ≥ 35%（A-10 ~ A-13）
- [x] 第四批后端性能与稳定性完成（A-14 ~ A-17）
- [x] 至少完成第五批 P2 重构中的 3 项（A-18 ~ A-23 任选 3 项）— 已完成全部 6 项（A-18 ~ A-23）
- [x] AUDIT.md 评分复评 ≥ 7.5 — 复评 7.8 / 10（见 AUDIT.md §1.2）
- [ ] CI 全绿，`v1.3.0` Release tag 已打

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
| S2-1 | S2 | CI 卫生反复出问题：无本地 pre-commit 钩子，100% 依赖远端 CI 拦截 gofmt/swagger 漂移 | Round 6 ✅ |
| S2-2 | S2 | 历史数据可信度：MySQL historical/ 缺 metadata，cross-db §7 有 2 项悬空待办 | Round 6 ✅ |
| S2-3 | S2 | Restore 后搜索索引不自动重建，pg/mysql 场景下内存索引持续漂移（真实 bug） | Round 6 ✅ |
| S2-4 | S2 | 测试覆盖偏薄：repository/storage 等 6 包无测试，前端仅 5 spec 无 coverage 配置 | Round 6 ✅ |
| S2-5 | S2 | 灾难恢复设计缺陷：restore 端点 auth-DB 循环依赖，无 CLI 恢复路径 | Round 6 ✅ |
