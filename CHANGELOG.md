# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 格式，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Fixed
- 修复前端 auth store 响应映射 bug（`res.data.token` / `res.data.user` 而非 `res.data` / `res.user`）
- 全局替换 VortexCMS → ContentX 品牌名（7 个 Vue 组件文件）

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
