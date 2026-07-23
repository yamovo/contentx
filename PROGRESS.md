# ContentX 开发进度

> 最后更新：2026-07-23
>
> 发布基线：`v1.0.0`（提交 `d408b83`）
>
> 当前主线：P3-A 生产就绪
>
> 本文件只记录进度、证据、风险和下一步；项目介绍与使用方法见 [README.md](./README.md)。

## 状态总览

| 阶段 | 状态 | 结果 |
|---|---|---|
| P0 基础可用 | ✅ 完成 | 核心 CMS、认证权限、管理后台和基础部署可用 |
| P1 工程化 | ✅ 完成 | Repository 分层、Redis、迁移、错误体系与安全加固完成 |
| P2 功能完善 | ✅ 完成 | Webhook、S3、工作流、GraphQL、i18n、插件、自定义内容类型完成 |
| P3-A 生产就绪 | 🚧 进行中 | 6 项完成，1 项进行中（7.2 跨数据库对照） |
| P3-B 商业化基础 | ⏳ 未开始 | 等待 P3-A 验收 |
| P3-C 卓越与生态 | ⏳ 未开始 | 等待 P3-B 验收 |

状态定义：✅ 已实现且有验证证据；🚧 已产生可用产物但未满足全部验收条件；⏳ 尚未开始；⛔ 阻塞。

## 已完成基线

### P0：基础可用 ✅

- 文章、页面、分类、标签、评论、媒体、用户与角色
- JWT、刷新令牌、API Token 和 RBAC
- Vue 3 管理后台、REST API 和 Swagger
- PostgreSQL、MySQL、SQLite 与 Docker 基础部署

### P1：工程化 ✅

- Service → Repository 分层与可注入 mock
- Redis 缓存、令牌黑名单和进程内回退
- 数据库迁移、统一错误模型、配置校验和安全中间件
- 后端测试、基础 CI、多平台构建与发布工作流

### P2：功能完善 ✅

- Webhook：HMAC 签名、投递日志和业务事件接线
- 媒体：本地/S3 兼容存储
- 内容：六状态发布工作流、修订恢复、定时发布
- API：只读 GraphQL、自定义内容类型和内容条目
- 国际化：文章和动态内容翻译组
- 扩展：编译期插件接口与 Hook/Filter

## P3-A：生产就绪

完成门槛：可观测、可压测、多实例任务不重复、可恢复、前端质量门禁有效、CI 真实跑通。全部完成前不进入 P3-B。

| 编号 | 工作项 | 状态 | 验收结论 |
|---|---|---|---|
| 7.1 | Prometheus + Grafana + OpenTelemetry | ✅ | 本机 Docker 运行验收通过 |
| 7.2 | 压测基线与公开报告 | 🚧 | PostgreSQL + MySQL 对照完成（SQLite 待测）；列表字段精简已实现；统一空闲内存待测 |
| 7.3 | 分布式定时任务锁 | ✅ | Redis/Memory 锁与调度器测试通过 |
| 7.4 | 全文搜索 | ✅ | 内置索引、REST/GraphQL 与全量重建通过 |
| 7.5 | 备份与恢复闭环 | ✅ | DB+媒体备份/恢复、REST API、Makefile 入口、20 项测试通过 |
| 7.6 | 前端工程债清理 | ✅ | vue-tsc 零错误、CI 移除 `\|\| true`、77 项前端测试通过 |
| 7.7 | CI 实际跑通 | ✅ | main 分支全链路绿勾（frontend+test+build+docker），GHCR 镜像推送成功 |

### 7.1 可观测性 ✅

已实现：

- `/metrics` 与 HTTP、用户、文章、数据库、缓存、Webhook 指标
- W3C Trace Context、Request ID/Trace ID 关联、GORM span、Webhook/S3 外部调用 span
- OTLP/HTTP exporter、Prometheus、Grafana、Tempo 和自动 provisioning
- 运行说明：[docs/observability.md](./docs/observability.md)

2026-07-22 运行验收：

- 应用健康检查通过
- Prometheus target 为 `up`
- Grafana 数据源和 ContentX dashboard 自动加载
- 真实 Trace ID 可从 Tempo 查询

验收中修复了 PostgreSQL `longtext` 不兼容、Redis 空密码、Tempo 3 配置变化和宿主机端口冲突。

### 7.2 压测基线与公开报告 🚧

已产出：

- Vegeta 复现脚本：`scripts/benchmark/run-postgres.ps1`
- 1,000/10,000 篇 PostgreSQL 数据集脚本
- PostgreSQL 原始结果：`reports/benchmarks/raw/postgres/`
- 1,000 篇数据、1,000 次读取/秒、100 次写入/秒的初步基线

| 场景 | 请求数 | 成功率 | P50 | P95 | P99 |
|---|---:|---:|---:|---:|---:|
| 文章列表（20 条） | 15,000 | 100% | 5.74 ms | 351.12 ms | 1.07 s |
| 文章详情 | 15,000 | 100% | 2.66 ms | 3.79 ms | 4.82 ms |
| GraphQL 查询 | 15,000 | 100% | 3.13 ms | 4.30 ms | 5.22 ms |
| 并发更新 | 1,000 | 100% | 9.04 ms | 12.04 ms | 17.57 ms |

当前实测：

- 10,000 篇文章完整建立内存搜索索引后，应用容器约 `145.4 MiB`
- 当前应用镜像约 `61.3 MiB`
- 旧文档中的“~30MB 内存”和“镜像 <30MB”没有当前证据支持，已从产品宣称中移除

待完成：

- ~~运行 MySQL / SQLite 全场景对照并回填结果~~ → ✅ MySQL 四场景已实测（2026-07-23），结果回填至 [cross-db-comparison.md](./reports/benchmarks/cross-db-comparison.md)；SQLite 待测
- 在统一空闲条件下重测 1,000/10,000 篇文章内存（`sample-memory.ps1` 已就绪）
- 在专用硬件或受控云环境上复测，验证 P95/P99 是否为开发环境噪声
- ✅ 列表接口字段精简已实现（默认不返回正文 `content`，`?full=true` 可取全量；搜索索引仍取全文）；go test 已验证正确性与搜索安全。性能复测待能跑时验证改善幅度

MySQL 对照结论（2026-07-23，详见 cross-db-comparison.md §3）：

| 场景 | PG P95 | MySQL P95 | 说明 |
|---|---:|---:|---|
| 文章详情 | 3.79 ms | 4.13 ms | 两库相当，MySQL 略高 |
| 并发写入 | 12.04 ms | 11.35 ms | MySQL 略优 |
| 文章列表 | 351.12 ms | 30,000 ms¹ | MySQL 1,018/15,000 超时，疑似客户端端口耗尽 |
| GraphQL | 4.30 ms | 30,000 ms² | MySQL 5,865/15,000 失败，Windows 端口耗尽 |

¹² 列表与 GraphQL 场景的 MySQL 失败主要为客户端侧 `TIME_WAIT` 端口耗尽（`Only one usage of each socket address`），非纯数据库瓶颈；需在 Linux + 长端口范围环境复测。详情与写入场景两库表现相当。

⚠️ 未提交（本次提交解决）：列表字段精简代码（repo/service/handler + `article_list_trim_test.go`）、跨库对照基础设施与 MySQL 实测结果将在本次 commit 中一并提交。

跨数据库对照基础设施（2026-07-22，已就绪，待实测）：

- 统一方法与复现指南：[reports/benchmarks/cross-db-comparison.md](./reports/benchmarks/cross-db-comparison.md)
- MySQL/SQLite 方言 seed 脚本（1k/10k），与 PostgreSQL 数据集逐行等价（正文 2,200 字符，长度已核对一致）
- 内置 Go seeder `scripts/benchmark/seeder`（用应用同一 GORM SQLite 驱动播种，免 `sqlite3` CLI；已编译通过）
- 通用压测脚本 `run-benchmark.ps1 -Driver <postgres|mysql|sqlite>`，场景与速率跨库一致
- MySQL 自包含压测栈 `scripts/benchmark/docker-compose.mysql.yml`（app + mysql8.4 + redis）
- 空闲内存采样脚本 `sample-memory.ps1`（容器 / 进程两种模式）
- 实测阻塞（环境性，非代码问题）：MySQL 镜像拉取被境内网络限速；SQLite 需在普通终端启动应用（Go+CGO 已就绪）。两腿均可按复现指南直接跑出数据

已完成（2026-07-22）：

- 正式压测报告：[reports/benchmarks/postgres-baseline.md](./reports/benchmarks/postgres-baseline.md)
- P95/P99 升高原因分析：主因为文章列表响应体大（71 KB/请求，1,000 req/s 下 72 MB/s JSON 序列化），导致 GC 频繁触发和长尾延迟；次要因素为分页+关联查询复杂度和连接池竞争
- 优化方向已记录在报告中：列表字段精简、缓存层、JSON 序列化优化、GOGC/GOMEMLIMIT 调优

### 7.3 分布式定时任务锁 ✅

- 抽象 `cache.DistributedLock`
- 实现 Redis `SET NX EX` 锁和 Memory 回退
- `PublishScheduler` 获取锁后执行，避免多实例重复发布
- 锁与调度器相关测试已通过

### 7.4 全文搜索 ✅

- `SearchIndexer` 接口、`BuiltinIndexer` 和 `NoopIndexer`
- BM25、中文 bigram、高亮、状态/类型/语言筛选和分页
- REST：公开搜索、管理员搜索和手动重建
- GraphQL：只读 `search` 查询
- 文章创建、更新、状态变化、修订恢复和删除的增量同步
- 启动时从数据库全量预热

2026-07-22 修复：全量重建原先请求每页 500 条，但公共列表上限为 100，参数被重置为默认 20，导致启动只建立 20 篇索引。现改为每批 100，并新增 250 篇跨页回归测试。Docker 运行日志已确认 `indexed=10000`。

边界：`meilisearch` 目前只是配置入口，选择后会回退到内置索引；内置索引不跨应用实例共享。

### 7.5 备份与恢复闭环 ✅

已实现：

- `backup.Manager` 支持 PostgreSQL（pg_dump/psql）、MySQL（mysqldump/mysql）、SQLite（VACUUM INTO/文件替换）三种驱动的备份与恢复
- 媒体文件备份：tar.gz 打包 uploads 目录，含 zip-slip 路径遍历防护
- `BackupAll` 同时备份数据库和媒体；`BackupMedia`/`RestoreMedia` 独立操作
- 保留策略：按前缀（db-/media-）独立清理，保留 MaxBackups 个最新备份
- `uniquePath` 防止同一秒内备份文件名冲突
- REST API（admin only）：
  - `POST /api/v1/admin/backup?type=db|media|all` — 触发备份
  - `GET /api/v1/admin/backup` — 列出备份（newest first）
  - `POST /api/v1/admin/backup/:file/restore` — 从备份恢复
  - `DELETE /api/v1/admin/backup/:file` — 删除备份
- Makefile 入口：`make backup API=... TOKEN=...` / `make restore-backup API=... TOKEN=... FILE=...`
- 20 项测试通过：SQLite backup/restore 往返、媒体打包/解包、保留策略、路径遍历防护、tarGz round-trip

边界：

- SQLite restore 会关闭数据库连接并覆盖文件，需重启服务
- PostgreSQL/MySQL restore 依赖 pg_dump/mysqldump/psql/mysql CLI 工具在 PATH 中可用
- 媒体备份仅覆盖本地存储驱动；S3 存储需另行通过云平台生命周期策略备份

### 7.6 前端工程债清理 ✅

验收要求（全部满足）：

- `npm run type-check` 零错误
- CI 不再使用 `npx vue-tsc --noEmit || true`
- 核心 store、API 和主要视图具备有效测试
- 类型检查、测试和构建任一失败都阻止合并

已实现：

- 修复 `vue-tsc --noEmit` 全部 10 项错误，覆盖 7 个 Vue 组件文件：
  - `AdminLayout.vue`：animejs v4 与 `CSSStyleDeclaration` 类型交互导致的 `String not callable`，通过提取局部变量 `const s = (el as HTMLElement).style` 修复
  - `ArticleEditor.vue` / `CategoryList.vue`：`TreeOptionProps.value` 不存在，通过抽离 `treeSelectProps as any` 常量修复
  - `ArticleList.vue` / `CategoryList.vue` / `CommentList.vue` / `MediaLibrary.vue`：Element Plus 表格 slot 的 `DefaultRow` 与领域类型冲突，使用 `row as Article/Category/Comment/Media` 断言修复
  - `MediaLibrary.vue`：`el-statistic :value` 期望 `number | Dayjs`，改为 `:value="rawNumber" :formatter="formatSize"`
  - `PluginList.vue` / `RedirectManager.vue`：API 返回 `unknown`，向 `api/index.ts` 新增 `Plugin` 和 `Redirect` 接口并标注返回类型
- `.github/workflows/ci.yml` 的 type-check 步骤移除 `|| true`，类型检查成为硬性门禁
- 新增前端测试：
  - `src/api/index.spec.ts` — 39 项 API 模块测试，覆盖 authApi、articleApi、categoryApi、tagApi、commentApi、mediaApi、userApi、roleApi、settingsApi、seoApi、menuApi、pluginApi、themeApi、systemApi、analyticsApi
  - `src/views/shared/NotFound.spec.ts` — 2 项视图测试，验证 404 页面渲染与跳转
- `vite.config.ts` 增加 `test.server.deps.inline: [/element-plus/]`，解决 Element Plus CSS 在 jsdom 测试中的导入错误
- 当前测试规模：5 个文件，77 项测试全部通过

验证记录：

- `npx vue-tsc --noEmit` → exit code 0 ✅
- `npx vitest run` → 5 files / 77 tests passed ✅

### 7.7 CI 实际跑通 ✅

现状：`.github/workflows/ci.yml` 已包含后端测试、lint、前端测试、构建、Docker 多架构和 tag release。前端类型检查已取消 `|| true` 放行。

已落地（2026-07-22）：

- 固定 golangci-lint 版本为 `v2.12.2`，消除 `version: latest` 漂移
- `.golangci.yml` 迁移到 v2 配置格式（`version: "2"` + `linters.settings` 取代顶层 `linters-settings`）
- 添加 `concurrency` 取消同一分支上的旧运行（PR 自动取消，main/develop 串行）
- 所有 job 添加 `timeout-minutes`：test 20、frontend 15、build 10、docker 30、release 20
- 质量门禁链：go vet → golangci-lint → go test → vue-tsc → vitest → build，任一失败阻止合并

待完成（需远端验证，本地无法执行）：

- ~~在真实 PR/main 上确认绿勾~~ → ✅ commit `99df4a2` main 分支 frontend(53s) + test(3m20s,含 vet/linter/tests/coverage) + build(22s) 全绿
- ~~在 main 分支推送时确认 Docker 镜像推送到 GHCR~~ → ✅ docker job 23m29s 多架构构建(amd64+arm64)并推送 GHCR 成功
- 打 `v*` tag 时确认 Release 产物（5 平台二进制 + 压缩包）→ 后续发版时验证
- 后续可考虑固定 Docker 监控镜像版本（见风险列表）

CI 修复历程（golangci-lint v2 迁移）：

1. `golangci-lint-action@v6` → `@v7`（v6 不支持 golangci-lint v2）
2. `.golangci.yml` v1→v2 配置迁移：`linters-settings` → `linters.settings`、`issues.exclude-rules` → `linters.exclusions.rules`、`gofmt`/`goimports` 移入 `formatters`、移除 `gosimple`（v2 合入 `staticcheck`）
3. 修复 64 个 lint 错误：errcheck（type assertion comma-ok、defer Close、os.Remove）、staticcheck（noop tracer、De Morgan）、unused、gofmt（结构体对齐）
4. 添加 `.gitattributes` 强制 `*.go text eol=lf`，消除 Windows CRLF 与 Linux CI 的 gofmt 差异

## 当前验证记录

| 日期 | 范围 | 结果 |
|---|---|---|
| 2026-07-22 | `go test ./... -count=1` | ✅ 搜索分页修复后完整通过 |
| 2026-07-22 | `go vet ./...` | ✅ 搜索分页修复后完整通过 |
| 2026-07-22 | 后端构建 | ✅ 通过 |
| 2026-07-22 | 搜索重建回归测试 | ✅ 250 篇跨 3 页全部进入索引 |
| 2026-07-22 | Docker 应用健康检查 | ✅ healthy |
| 2026-07-22 | 搜索启动预热 | ✅ PostgreSQL 10,000 篇全部建立索引 |
| 2026-07-22 | PostgreSQL Vegeta 初测 | ✅ 四个场景成功率 100% |
| 2026-07-22 | backup 包测试 | ✅ 20 项测试通过（SQLite backup/restore/媒体/保留策略） |
| 2026-07-22 | `go build ./...` + `go vet ./...` | ✅ 备份恢复闭环完成后全绿 |
| 2026-07-22 | `npx vue-tsc --noEmit`（web） | ✅ 7.6 完成后零错误 |
| 2026-07-22 | `npx vitest run`（web） | ✅ 5 个文件 / 77 项测试全部通过 |
| 2026-07-22 | `go build ./...` + `go vet ./...` | ✅ 7.7 CI 收紧后全绿 |
| 2026-07-22 | PostgreSQL 压测正式报告 | ✅ 7.2 报告与 P95/P99 分析完成 |
| 2026-07-22 | GitHub Actions CI（main `99df4a2`） | ✅ 全链路绿勾：frontend(53s)+test(3m20s)+build(22s)+docker(23m29s,GHCR推送) |
| 2026-07-22 | 7.2 跨库对照基础设施（seed×4 / run-benchmark / compose / mem / 报告） | ✅ 脚本创建；PowerShell 语法校验通过；SQLite/PG 数据集长度对齐（2,200 字符） |
| 2026-07-22 | 7.2 Go seeder + 应用（SQLite/CGO）编译 | ✅ `go build ./cmd/server` 与 `./scripts/benchmark/seeder` 均通过（windows/amd64 PE） |
| 2026-07-22 | 7.2 MySQL/SQLite 实测 | ⛔ 环境阻塞：MySQL 镜像拉取限速（109/256MB～30min）；agent 沙箱不允许 shell 直接执行新构建应用二进制。需在普通终端按指南跑 |
| 2026-07-23 | 7.2 列表字段精简优化（默认不返回 content，`?full=true` 可选） | ✅ repo/service/handler 改动 + 2 新用例（含搜索安全守卫）全绿；services+handlers 全量测试无回归 |
| 2026-07-23 | 7.2 MySQL 四场景实测 | ✅ 详情/写入 100% 成功与 PG 相当；列表/GraphQL 受 Windows 端口耗尽影响失败率较高，待 Linux 复测 |
| 2026-07-23 | 7.2 跨库对照报告回填 | ✅ cross-db-comparison.md §3 MySQL 数据已回填，含失败原因注释 |
| 2026-07-23 | `go build ./...` + `go vet ./...` | ✅ 列表字段精简后全绿 |
| 2026-07-23 | `go test ./internal/services/ -run TestArticleService_ListOmits` | ✅ 列表字段精简测试通过 |

说明：最新搜索分页修复后已再次执行完整后端测试与 vet；7.6 前端工程债清理已落地，前端类型检查与测试均通过。

## 当前风险与已知边界

1. 文章列表在高目标速率下尾延迟明显高于详情和 GraphQL，需要查询与响应体分析。列表字段精简（默认不返回 `content`）已实现，待复测验证改善幅度。
2. 10,000 篇内置索引使应用内存达到约 145.4 MiB；大数据量需评估流式重建或外部索引。
3. MeiliSearch 驱动尚未实现，不能宣称多实例共享搜索。
4. CI 质量门禁已在 main 分支验证全链路绿勾（frontend+test+build+docker），GHCR 镜像推送成功；tag Release 产物待发版时验证。
5. Docker 监控镜像使用 `latest`，后续应固定兼容版本。
6. MySQL 列表与 GraphQL 压测受 Windows 客户端端口耗尽（TIME_WAIT）影响，失败率非数据库瓶颈；需在 Linux + 长端口范围环境复测。
7. ~~7.2 列表字段精简代码与跨库对照基础设施尚未 commit~~ → ✅ 已在本次提交中解决。

## 下一步执行顺序

1. 完成 7.2：MySQL 对照已完成；待跑 SQLite 全场景、统一空闲内存复测、Linux 复测 MySQL 列表/GraphQL，回填 [cross-db-comparison.md](./reports/benchmarks/cross-db-comparison.md)。
2. ~~提交 7.2 未提交工作区（列表字段精简 + 跨库基础设施 + MySQL 结果），触发 CI 验证。~~ → ✅ 本次提交完成
3. ~~完成 7.7：收紧 CI，在真实 PR/main/tag 验证全链路。~~ → ✅ main 分支全链路绿勾
4. P3-A 全部验收后，再评估 P3-B 多租户、计费、用量和 SSO。

## 完成定义

一个工作项只有同时满足以下条件才可标记 ✅：

- 代码或配置已经落地
- 自动化测试或可重复运行验证通过
- 文档说明当前能力与边界
- 不使用未测量的性能、容量或兼容性宣称
- 若涉及生产运维，至少有一次真实运行或演练记录
