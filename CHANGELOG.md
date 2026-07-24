# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 格式，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [1.2.0] - 2026-07-24

Round 6 扣分项整改：基于 v1.1.0 验收评估的 5 个扣分项（CI 卫生、历史数据可信度、功能边界缺口、测试覆盖偏薄、灾难恢复设计缺陷）全部整改完成。

### Added — F1 CI 本地防线
- pre-commit 钩子（`scripts/git/hooks/pre-commit`）：gofmt + go vet + swagger drift 检查
- Makefile `install-hooks` 和 `check` 聚合目标
- 前端 husky + lint-staged：暂存 `.ts`/`.vue` 运行 `vue-tsc --noEmit`
- CI 增加 gofmt drift 快速失败步

### Added — F2 Restore 后自动重建搜索索引
- `internal/handlers/backup.go` Restore handler 恢复成功后异步调用 `ReindexAll`
- pg/mysql 场景立即重建，SQLite 场景提示重启后重建
- 响应返回 `search_index: "rebuilding"`

### Added — F3 `--restore` CLI 子命令
- `cmd/server/main.go` 增加 `--restore <file>` flag，绕过 HTTP/认证层直接调用 `backup.Restore()`
- 支持 `--driver postgres|mysql|sqlite`
- 消除灾难恢复 auth-DB 循环依赖

### Added — F5 repository 层集成测试
- `internal/repository/article_test.go`、`user_test.go`、`content_test.go`、`testutil_test.go`
- 15 个测试覆盖 Create/Update/List/Delete + tag 关联 + role/permission CRUD + content type 级联删除 + 过滤器

### Added — F6 storage 层单元测试 + 安全修复
- `internal/storage/local_test.go`、`s3_test.go`
- 覆盖 upload/download/delete + 路径遍历拒绝 + URL 构造 + 签名 + 错误处理

### Added — F7 前端业务组件测试
- 共享测试工具 `web/src/test/utils.ts`（mountWithPlugins + localStorage mock + Element Plus stubs）
- `TagList.spec.ts`、`CategoryList.spec.ts`、`LoginView.spec.ts`
- 前端测试 77 → 100 个，coverage 10.86% → 25.31% lines

### Added — F8 CI 覆盖率门槛
- 后端 Go 覆盖率门槛 60%（当前 ~64.6%）
- 前端 vitest `--coverage` 强制执行 thresholds（lines/statements 20%，branches 40%，functions 35%）

### Fixed — 安全
- `internal/storage/local.go`：`safePath` 方法修复路径遍历漏洞，跨平台反斜杠归一化（Linux 拒绝 Windows 风格 `..\..` 遍历）
- `internal/storage/s3.go`：`objectURL` scheme 硬编码修复（PathStyle 从 `UseSSL` 派生 scheme 而非硬编码 `http://`）
- `internal/repository/user.go`：`UserRepository.List` 限定 `users.created_at` 解决 JOIN roles 歧义列

### Fixed — F4 文档修正
- SOP §3.4 灾难恢复：workaround 从 psql 升级为 `--restore` CLI
- `cross-db-comparison.md` §7：MySQL historical/run-metadata.json 补齐（标注 `invalid: true`）；悬空待办改为已知限制
- 提交 historical 原始数据（mysql + postgres）

## [1.1.0] - 2026-07-23

P3-A"生产就绪"Round 1-5 全部完成。

### Added — Round 1-3 正确性与构建卫生
- `.dockerignore` 减少 build context ~2018 MB
- GraphQL resolvers 按需加载 content（`omitempty`）
- 6 个集成测试（3 GraphQL + 3 REST）
- 跨数据库基准测试（SQLite/PostgreSQL/MySQL，10,000 篇文章，Git SHA 0f5d624）
- `run-metadata.json` 保存 COUNT、Git SHA、配置、响应大小

### Added — Round 4-5 生产就绪收尾
- golangci-lint v2 格式迁移
- CI concurrency control + timeouts
- Swagger 文档 11 端点注解 + 9 空白导入 + regenerated swagger.json + CI drift check
- Docker Compose E2E 验证

### Fixed
- golangci-lint v2.12.2 版本固定
- 前端 10 个 vue-tsc 类型错误修复
- 基准测试脚本可复现性修复（run-benchmark.ps1、run-postgres.ps1、docker-compose.mysql.yml）

## [1.0.0] - 2026-07-22

### Added — 安全加固 (P0)
- JWT 黑名单 + Redis 集成（不可用时回退内存版）
- 登录暴力破解防护（LoginGuard 计数 + 锁定）
- 错误响应脱敏（`sanitizeBindErr` / `sanitizeMessage`）
- SVG 上传净化（白名单移除 script/on* 事件/外部 href）
- Release 模式强制 `ADMIN_PASSWORD` + `JWT_SECRET`，启动审计 `config.Validate()`

### Added — 工程化 (P1)
- 结构化日志（slog，89 处调用，0 处 `log.Printf` 残留）
- 统一错误码体系（`errs.AppError` + `APIResponse.err_code`）
- Repository 接口层（12/12 Service 全量重构，Service 不持有 `*gorm.DB`）
- Handler + Middleware 测试（覆盖率 75.9% / 70.4%）
- Go Migrator CLI（`--migrate` / `--migrate-down=N` / `--migrate-status` / `--seed`）
- Swagger 注解 95.6%（109/114 方法）
- CI/CD：多平台 Docker（amd64+arm64）+ GHCR + GitHub Release（5 平台二进制）
- 部署配置：`.env.example` + `nginx.conf` + `.golangci.yml` + `Makefile`
- Repository mock 测试（services 覆盖率 83.5%，10 个手写 mock 仓库）

### Added — 功能完善 (P2)
- Webhook 投递（8 类事件 + HMAC 签名 + 4 Service 注入 + 14 测试）
- S3/OSS 媒体存储（双路径 + `storage.Driver` 接口注入 + 11 测试）
- 6 态发布工作流（draft → pending → published → scheduled → archived → trash + `PublishScheduler` + 28 测试）
- GraphQL 只读 API（6 对象类型 + 10 Query + 18 测试）
- i18n 多语言（`Locale` + `TranslationGroupID` + 翻译创建/查询 + `?locale=` 过滤 + 15 测试）
- 插件系统（`Plugin` 接口 + Hook/Filter + `Manager` + `WordCountPlugin` + 23 测试）
- Content Type Builder 后端（动态建模 + 字段验证 + 导入/导出）

### Fixed
- `errs.Is()` 支持 `errors.Join` 多错误链
- `token_service.Delete()` 返回 `errs.ErrNotFound` 而非 plain error
- `JSONMap.Scan()` SQLite 兼容（添加 `string` 类型分支）
- `detectOS` case 顺序 bug（iphone/ipad 在 mac os 前，android 在 linux 前）
- i18n `ListTranslations` 查询兼容翻译组根（`translation_group_id IS NULL`）

### Changed
- Go module 从 `vortexcms` / `go-cms` 统一为 `github.com/yamovo/contentx`
- 前端品牌名从 VortexCMS 统一为 ContentX
- `.gitignore` 更新：`go-cms` → `/contentx`
