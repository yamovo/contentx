# ContentX 项目总览

> 最后更新：2026-07-22（P2 全部完成）
> 本文件是项目的唯一指导文档，集成了路线图、架构决策、开发规范、进度追踪和项目约定。

---

## 目录

1. [项目定位与差异化](#1-项目定位与差异化)
2. [架构概览](#2-架构概览)
3. [技术决策](#3-技术决策)
4. [项目约定](#4-项目约定)
5. [开发指南](#5-开发指南)
6. [进度追踪](#6-进度追踪)
7. [P3 商业化路线图](#7-p3-商业化路线图)
8. [成功指标与风险](#8-成功指标与风险)
9. [不做清单](#9-不做清单)
10. [编译与测试验证](#10-编译与测试验证)

---

## 1. 项目定位与差异化

**核心定位**：给开发者的高性能 Go Headless CMS — API-first 内容平台，单二进制部署。

**差异化卖点**：
1. **Go 单二进制** — ~30MB 内存跑起来，Docker 镜像 < 30MB
2. **代码优先建模** — 用 Go struct / API 定义内容类型，编译期类型安全
3. **超高并发** — 适合内容分发、API 网关场景
4. **零依赖部署** — SQLite 模式无需外部数据库
5. **REST + GraphQL 双 API** — 前端灵活选择

**对标差距（vs Strapi v5）**：

| 能力 | Strapi | ContentX | 说明 |
|------|--------|-----------|------|
| Content Type Builder | ✅ 可视化建模 | ✅ API 建模（不做 UI Builder） | 后端全量实现，不做前端可视化 |
| REST API | ✅ 自动生成 | ✅ 手写 + 自动生成 | 动态内容类型自动生成 CRUD |
| GraphQL | ✅ 内置 | ✅ 只读端点 | 写操作走 REST（安全设计） |
| Webhook | ✅ | ✅ | 8 类事件 + HMAC 签名 |
| 国际化 (i18n) | ✅ | ✅ | Locale + TranslationGroup |
| 版本/发布工作流 | ✅ Draft/Publish | ✅ 6 态状态机 | 含审核 + 定时发布 |
| API Token | ✅ 细粒度 | ✅ 细粒度 | 按模块授权 |
| 插件系统 | ✅ 600+ 市场 | ✅ Hook/Filter（内置注册） | 不做插件市场 |
| 媒体管理 | ✅ S3/OSS | ✅ S3/本地双路径 | |
| 单二进制部署 | ❌ Node.js | ✅ Go | **核心优势** |
| 内存占用 | ~200MB+ | ~30MB | **核心优势** |
| 并发性能 | 中 | 高 | **核心优势** |

---

## 2. 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│  Vue 3 管理后台 (web/)  —  独立前端，非核心                   │
├─────────────────────────────────────────────────────────────┤
│  Gin HTTP Server                                            │
│  ├── Middleware 链 (7 层)                                    │
│  │   Recover → RequestID → Logger → CORS →                  │
│  │   SecurityHeaders → ContentTypeJSON → ActivityLogger     │
│  ├── Handlers 层 (HTTP/Swagger/DTO)                         │
│  ├── Services 层 (业务逻辑 + Webhook + Plugin Hook)         │
│  ├── Repository 层 (接口，12 个 Service 全量重构)            │
│  └── GORM (PostgreSQL / MySQL / SQLite)                     │
├─────────────────────────────────────────────────────────────┤
│  横切能力                                                    │
│  ├── GraphQL (只读，复用 Service 层)                         │
│  ├── Plugin Manager (Hook/Filter + 优先级排序)              │
│  ├── Webhook Dispatcher (8 事件 + HMAC)                     │
│  ├── Storage Driver (Local / S3 接口注入)                   │
│  ├── Cache Driver (Memory / Redis 接口注入)                 │
│  └── PublishScheduler (后台定时发布 worker)                 │
└─────────────────────────────────────────────────────────────┘
```

**分层依赖单向流动**：
```
handlers → services → repository(接口) → models + GORM
```
- Service 层不持有 `*gorm.DB`，只依赖 `repository.XxxRepository` 接口
- 事务、原生 SQL、关联操作全封装在 repository 实现内
- 测试可注入 mock，未来换 ORM 只动 repository 实现

**依赖注入**：所有扩展能力（webhook、plugin、storage、cache）均为接口注入，Service 不知道具体实现。外部依赖不可用时优雅降级（Redis→内存，S3→本地）。

---

## 3. 技术决策

### 为什么用数据库驱动建模而非代码生成？

| 方案 | 优点 | 缺点 |
|------|------|------|
| **数据库驱动（已选）** | 运行时动态、无需重启 | 性能略差、类型不安全 |
| 代码生成 | 编译期类型安全、性能好 | 需要重新编译部署 |

Strapi 用数据库驱动，市场验证过了。

### 为什么不用 gRPC？

Headless CMS 的消费者是前端应用，REST + GraphQL 是标准配置。gRPC 前端不直接用，需要网关，增加复杂度。

### 为什么 GraphQL 是只读？

安全设计。写操作（创建/更新/删除）统一走 REST API，受 RBAC 权限中间件保护。GraphQL 只暴露公开查询，降低攻击面。

### 为什么插件是内置注册而非 .so 动态加载？

Go `plugin` 包仅支持 Linux/macOS，不支持 Windows，且 ABI 兼容性脆弱。当前方案为编译期注册（Plugin 接口），未来可扩展为进程外插件或脚本引擎。

---

## 4. 项目约定

### 命名

- Go module: `github.com/yamovo/contentx`
- 二进制: `contentx`
- SDK 包名: `@contentx/sdk`
- Docker 镜像/容器: `contentx`
- 网络: `contentx-net`
- Redis 前缀: `contentx:`
- 旧名称 `vortexcms`/`go-cms` 已彻底清理

### 安全约定

- Release 模式必须设置: `JWT_SECRET` (≥16 字符)、`ADMIN_PASSWORD` (≥8 字符)、`DB_PASSWORD` (非 SQLite)
- 启动时 `config.Validate()` 自动检查，失败立即退出
- 已知弱 JWT_SECRET 值会被拒绝
- 无硬编码回退密码（release 模式缺失即报错）

### 编码规范

- 遵循标准 Go 格式化（`gofmt`, `goimports`）
- 使用 `golangci-lint` 进行 lint
- Handler 保持精简，业务逻辑放在 Service 层
- 使用 `errs.AppError` 进行结构化错误处理
- Service 错误统一通过 handler 的 `handleServiceError` 映射
- 表驱动测试优先
- Conventional Commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`

### 测试约定

- Handler 集成测试放 `internal/handlers/*_integration_test.go`
- 使用 in-memory SQLite + httptest
- Mock auth middleware 放在 `auth_integration_test.go`，其他文件复用
- 测试共享 helpers: `setupTestRouter`, `createTestUserDB`, `generateTestJWT`
- Service 层 mock 测试：手写 mock 仓库（数据预置 + 错误注入 + 调用跟踪），零外部依赖

### CI 规则

- lint 失败 = CI 失败（不跳过）
- 覆盖率上传失败 = CI 失败（不跳过）

---

## 5. 开发指南

### 前置条件

- Go 1.22+
- PostgreSQL 16+（或 Docker）
- Redis 7+（或 Docker，可选）
- Node.js 18+（前端开发）

### 快速启动

```bash
git clone https://github.com/yamovo/contentx.git
cd contentx
cp .env.example .env

# 启动依赖
docker-compose up -d postgres redis

# 数据库迁移 + 种子
make migrate
make seed

# 启动后端
make dev

# 启动前端（另一个终端）
cd web && npm install && npm run dev
```

### 项目结构

```
contentx/
├── cmd/server/main.go          # 入口
├── internal/
│   ├── auth/                   # JWT、密码、API Key、TOTP
│   ├── cache/                  # 缓存驱动（Memory / Redis）
│   ├── config/                 # 30+ 环境变量
│   ├── database/               # 连接、迁移、种子
│   ├── errs/                   # 统一错误码体系
│   ├── graphql/                # GraphQL schema + resolvers
│   ├── handlers/               # HTTP 处理器（Swagger 95.6%）
│   ├── middleware/             # 认证、限流、CORS、RBAC
│   ├── models/                 # 数据模型（含动态内容类型、i18n）
│   ├── plugin/                 # 插件接口 + Hook/Filter + Manager
│   ├── repository/             # GORM 仓库接口层（12 Service 全量重构）
│   ├── services/               # 业务逻辑层（覆盖率 83.5%）
│   ├── storage/                # 存储驱动（Local / S3）
│   └── logger/                 # 结构化日志
├── sdk/typescript/             # TypeScript SDK
├── docs/api/                   # Swagger JSON/YAML（main.go 导入）
├── deploy/                     # Docker + Nginx 配置
└── web/                        # Vue 3 管理后台
```

### 配置

```env
DB_DRIVER=sqlite                 # postgres | mysql | sqlite
SERVER_PORT=8080
SERVER_MODE=debug                # debug | release
JWT_SECRET=your-secret-key
STORAGE_DRIVER=local             # local | s3
S3_ENDPOINT=minio:9000
S3_BUCKET=contentx
CACHE_DRIVER=memory              # memory | redis
REDIS_ADDR=localhost:6379
I18N_DEFAULT_LOCALE=en
I18N_LOCALES=en,zh,ja
```

### 提交 Pull Request

1. Fork 仓库
2. 创建功能分支: `git checkout -b feature/your-feature`
3. 遵循编码规范
4. 为新功能添加测试
5. 确保全部测试通过: `go test ./...`
6. 提交 PR 并附带清晰描述

---

## 6. 进度追踪

### 阶段一：P0 — 安全加固 + 核心缺陷修复 ✅ (7/7)

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 1.1 | JWT 黑名单实际生效 | ✅ | `AuthMiddleware` 调用 `store.IsRevoked(token)`；Logout 时 `Revoke` |
| 1.2 | Redis 集成（黑名单 + 缓存） | ✅ | `cache/redis.go` + `auth/redis_token_store.go`；Redis 不可用时回退内存版 |
| 1.3 | 登录暴力破解防护 | ✅ | `LoginGuard` 完整实现（计数+锁定+清理） |
| 1.4 | 错误响应脱敏 | ✅ | `sanitizeBindErr` + `sanitizeMessage`；批量替换 42 处 `err.Error()` |
| 1.5 | SVG 上传安全 | ✅ | `svg_sanitize.go` 白名单净化（移除 script/on* 事件/外部 href） |
| 1.6 | 种子文件统一 | ✅ | `database/seeds/` 死代码已删除 |
| 1.7 | 移除硬编码回退密码 | ✅ | release 模式强制 `ADMIN_PASSWORD`，否则 `crypto/rand` 随机生成 |

### 阶段二：P1 — 工程化 + 可观测性 ✅ (8/8 + 延伸)

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 2.1 | 结构化日志 | ✅ | 0 处 `log.Printf` 残留；89 处 `slog.` 调用分布于 12 个文件 |
| 2.2 | 统一错误码体系 | ✅ | `APIResponse.err_code` + `errs.AppError` 15 通用码 + 6 业务码 |
| 2.3 | Repository 接口层 | ✅ | 12/12 Service 全量重构；Service 不持有 `*gorm.DB`；`NewXxxServiceWithRepo` 支持 mock 注入 |
| 2.4 | Handler + Middleware 测试 | ✅ | Handler 75.9%（目标 60%+）；Middleware 70.4% |
| 2.5 | 数据库迁移工具 | ✅ | Go Migrator 激活 + `--migrate`/`--migrate-down=N`/`--migrate-status`/`--seed` CLI flags |
| 2.6 | OpenAPI 文档 | ✅ | swagger 注解覆盖率 31.6% → 95.6%（109/114 方法） |
| 2.7 | CI/CD 流水线 | ✅ | 多平台 Docker（amd64+arm64）+ GHCR + GitHub Release（5 平台二进制） |
| 2.8 | 部署配置补全 | ✅ | `.env.example` + `nginx.conf` + `Makefile` + `.golangci.yml` |
| ext | Repository mock 测试 | ✅ | services 覆盖率 64.1% → 83.5%；10 个手写 mock 仓库；修复 detectOS bug |

### 阶段三：P2 — 功能完善 ✅ (7/7)

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| - | Webhook 投递补全 | ✅ | `WebhookDispatcher` 接口 + 4 Service 注入 + 8 事件 + 14 测试 |
| - | S3/OSS 媒体存储 | ✅ | 双路径（storage.Driver + 本地回退）+ 11 测试 |
| - | 版本/发布工作流 | ✅ | 6 态状态机 + `AllowedTransition` + 6 Service/Handler + `PublishScheduler` + 28 测试 |
| - | GraphQL API | ✅ | 只读端点 + 6 对象类型 + 10 Query + `FieldsThunk` 循环引用 + 18 测试 |
| - | i18n 内容 | ✅ | `Locale` + `TranslationGroupID` + 翻译创建/查询 + 4 端点 + `?locale=` 过滤 + 15 测试 |
| - | 插件动态加载 | ✅ | `Plugin` 接口 + Hook/Filter + `Manager` + `WordCountPlugin` + ArticleService 集成 + 23 测试 |
| - | Content Type Builder | ✅ | 后端全量实现（`ContentType`/`ContentField`/`ContentEntry` 全链路） |

**测试过程中发现并修复的真实缺陷**：

| 缺陷 | 修复 |
|------|------|
| `errs.Is()` 不支持 `errors.Join` 多错误链 | 增加 `interface{ Unwrap() []error }` 分支 |
| `token_service.Delete()` 返回 plain error 导致 500 | 改为 `errs.ErrNotFound.WithMessage(...)` |
| `JSONMap.Scan()` SQLite 兼容（只处理 `[]byte` 不处理 `string`） | 添加 `string` 类型分支 |
| `detectOS` case 顺序 bug（iphone/ipad 在 mac os 后） | 调整 case 顺序 |
| i18n `ListTranslations` 未匹配翻译组根 | 查询兼容 `translation_group_id IS NULL AND id = groupID` |

---

## 7. P3 商业化路线图

> P0/P1/P2 全部完成。P3 按"能否收钱"重排优先级，分三层：
> **P3-A 商业化基建**（不做就不能卖）→ **P3-B 企业级增值**（能卖更贵）→ **P3-C 工程补丁**（基础设施，归回 P1 性质）。
> 核心原则：商业化 = 产生收入。监控指标不产生收入，计费引擎才产生收入。

### P3-A 商业化基建（硬前提，不做就不能卖）

> 完成 P3-A 后即可开始收费。这是 SaaS 化的最低门槛。

#### 7.1 多租户架构
- 行级隔离（`tenant_id` 贯穿所有模型 + 查询自动注入）
- **影响面**（非仅 Repository 层）：数据模型、RBAC 权限、缓存 key 命名空间、webhook 路由、存储隔离（S3 key 前缀）、插件作用域、GraphQL schema 注入
- 租户管理后台（CRUD + 配额配置 + 暂停/恢复）
- 默认租户（single-tenant 兼容模式，平滑迁移）
- **依赖**：2.3 Repository 接口 ✅（基础已就绪，但需扩展 tenant scope）

#### 7.2 计费引擎
- Plan/Tier 定义（Free / Pro / Enterprise：文章数、API 调用量、存储空间、用户数四维度）
- Stripe 集成（订阅创建、升降级、续费、取消 webhook）
- 订阅生命周期（试用期 → 付费 → 过期降级，自动锁定写操作）
- 发票与账单历史
- **这是商业化的核心**——没有计费就没有 SaaS

#### 7.3 API 用量计量
- Middleware 计数（每请求按 tenant_id + endpoint 维度累计到 Redis）
- 实时用量 dashboard（当日/当月 API 调用、存储占用、文章数）
- 配额超额拦截（429 + `X-RateLimit-Remaining` 头）
- 月度用量报表（用于计费依据）
- **Headless CMS 的命脉**：Contentful/Strapi 的核心商业模式

#### 7.4 SSO / SAML
- SAML 2.0（企业客户敲门砖，比 2FA 重要 10 倍）
- OIDC（Google Workspace / Microsoft Entra）
- Just-in-time 用户 provisioning（首次 SSO 登录自动建账）
- 域名白名单（限定企业邮箱域名）
- **企业销售的关键卡点**：没有 SSO 进不了采购短名单

### P3-B 企业级增值（提升客单价）

> P3-A 完成后，P3-B 让 Pro 客户升级到 Enterprise。

#### 7.5 白标与自定义域名
- 租户绑定自有域名（CNAME → ContentX）
- 主题白标（Logo / 配色 / 域名全替换）
- SSL 证书自动签发（Let's Encrypt + ACME）
- 管理后台隐藏 ContentX 品牌

#### 7.6 SLA 与合规
- 数据导出承诺（全量 JSON 导出 + GDPR 删除权 `DELETE /tenant/:id/everything`）
- 审计日志导出（CSV/JSON，含 IP、UA、操作时间）
- 数据驻留选项（选区域存储，合规要求）
- SOC2 / ISO27001 文档支持（流程文档，非技术实现）

#### 7.7 内容协作
- 实时协同编辑（WebSocket + CRDT 或 OT）
- 行内评论与批注（类似 Google Docs）
- 版本 diff 可视化（前端工作，后端已有 Revision 数据）
- 编辑锁（防止并发覆盖）

#### 7.8 图片处理与 CDN
- 动态 resize（`?w=800&h=600` URL 参数）
- WebP/AVIF 自动转换（按 Accept 头）
- CDN 集成（CloudFront / Cloudflare / 阿里云 CDN）
- 水印与图片优化

### P3-C 工程补丁（基础设施，非商业化）

> 这些是运维与安全补丁，归入 P3 是因为 P1 已关闭。它们不产生收入，但不做会影响留存。

#### 7.9 Prometheus 指标
```
http_requests_total{method, path, status, tenant_id}
http_request_duration_seconds{method, path}
active_users_total{tenant_id}
articles_total{tenant_id, status}
api_usage_current{tenant_id}          # 商业化关键指标
```

#### 7.10 OpenTelemetry 分布式追踪
- Request ID → Trace ID 映射
- 跨服务调用追踪（未来微服务化时有用）

#### 7.11 定时备份
- 数据库定时备份（pg_dump/mysqldump）
- 备份文件上传到 S3/MinIO
- 保留策略（最近 7 天 + 每月 1 个）
- **多租户后**：支持按租户导出/恢复

#### 7.12 全文搜索
- 集成 MeiliSearch（轻量、支持中文、单二进制，符合项目调性）
- **独立服务，不依赖 Redis**（修正原路线图错误依赖）
- `SearchConfig` 已存在但未接入
- 多租户后：按 tenant_id 分索引

#### 7.13 邮件系统
- 邮箱验证注册流程
- 密码重置（邮件发送重置链接）
- 评论邮件通知
- 计费通知（试用到期、续费成功、配额预警）—— **与计费引擎联动**
- `MailConfig` 已存在，需实现 SMTP 发送

#### 7.14 安全增强
- 2FA / TOTP（模型已存在，完善流程 + 备份恢复码）
- 会话管理（活跃会话列表 + 远程注销 + 设备指纹）
- 内容审核（敏感词过滤 + Akismet 集成 + 合规检查）

### P3 优先级与依赖关系

```
P3-A 商业化基建（硬前提）
  ├── 7.1 多租户 ──→ 2.3 Repository 接口 ✅
  ├── 7.2 计费引擎 ──→ 7.1 多租户（按租户计费）
  ├── 7.3 API 用量计量 ──→ 7.1 多租户 + 1.2 Redis ✅
  └── 7.4 SSO/SAML (独立，企业销售可并行推进)

P3-B 企业级增值（提升客单价）
  ├── 7.5 白标域名 ──→ 7.1 多租户
  ├── 7.6 SLA/合规 ──→ 7.1 多租户（数据导出按租户）
  ├── 7.7 内容协作 (独立)
  └── 7.8 图片CDN (独立)

P3-C 工程补丁（基础设施）
  ├── 7.9 Prometheus (独立)
  ├── 7.10 OTel (独立)
  ├── 7.11 备份 ──→ 7.1 多租户（按租户恢复）
  ├── 7.12 全文搜索 (独立，MeiliSearch 不依赖 Redis)
  ├── 7.13 邮件 ──→ 7.2 计费引擎（计费通知联动）
  └── 7.14 安全增强 (独立)
```

### P3 商业化判断标准

| 项 | 判断标准 | 归类 |
|----|---------|------|
| 能直接产生收入？ | 是 → P3-A 计费/计量 | 商业化核心 |
| 企业客户采购必问？ | 是 → P3-A SSO / P3-B SLA | 商业化基建 |
| 不做会流失客户？ | 是 → P3-C 补丁 | 留存底线 |
| 不做不影响收钱？ | 是 → 归 P1 补丁，不挂商业化牌 | 工程补丁 |

**原路线图的问题**：把 Prometheus/OTel/备份/2FA 这些"不挂商业化牌"的工程补丁塞进 P3，回避了真正难的商业决策（计费模型、定价、企业销售路径）。重构后，P3-A 完成即可收费，P3-C 是留存底线——顺序清晰、目标可度量。

---

## 8. 成功指标与风险

### 成功指标

**工程指标**

| 指标 | 目标 | 衡量方式 |
|------|------|----------|
| API 响应时间 | < 50ms (P95) | 基准测试 |
| 内存占用 | < 50MB (1000 篇文章) | `docker stats` |
| Docker 镜像 | < 30MB | 构建产物 |
| 并发能力 | > 1000 req/s (SQLite) | wrk 压测 |
| API 覆盖率 | 100% CRUD | Swagger 验证 |
| Service 覆盖率 | > 80% | go test -cover |

**商业化指标（P3-A 完成后启用）**

| 指标 | 目标 | 衡量方式 |
|------|------|----------|
| 付费转化率 | Free → Pro ≥ 5% | Stripe 订阅数据 |
| API 用量可计费准确度 | 100%（与账单一致） | 用量报表 vs Stripe 账单 |
| SSO 接入客户数 | ≥ 3 家企业 | 企业客户签约 |
| 月度经常性收入 (MRR) | P3-A 上线 6 个月内 > 0 | Stripe dashboard |

### 风险与应对

| 风险 | 概率 | 影响 | 应对 |
|------|------|------|------|
| **多租户改造影响面被低估** | 高 | 高 | 7.1 已标注全栈影响面（非仅 repo 层），需先做影响评估 PoC |
| **计费引擎与业务耦合** | 中 | 高 | 计费逻辑独立为 `billing` 包，通过事件订阅业务变更，不侵入 Service |
| **API 用量计量丢数据** | 中 | 高 | Redis 计数 + 定时持久化到 DB；Redis 不可用时降级到内存累计 |
| **SSO 协议复杂度高** | 高 | 中 | 先做 OIDC（简单），SAML 2.0 作为企业版单独迭代 |
| 无水平扩展（定时发布单实例） | 中 | 中 | P3-C 加分布式锁（Redis） |
| 无全文搜索 | 高 | 中 | P3-C 集成 MeiliSearch（不依赖 Redis，独立服务） |
| 插件系统仅内置注册 | 高 | 中 | P3 后扩展进程外插件或脚本引擎 |
| 生态建设慢 | 高 | 中 | 先做 TS SDK，其他社区贡献 |
| 动态内容类型性能差 | 中 | 中 | 加缓存、预编译查询 |
| GraphQL 仅只读 | 中 | 低 | 安全设计，写操作走 REST（非缺陷） |

---

## 9. 不做清单

明确**不做的事**，防止范围蔓延：

- ❌ 可视化内容类型建模（UI Builder）— 打不过 Strapi，后端 API 已够用
- ❌ 插件市场 — 生态需要时间，先做核心
- ❌ WYSIWYG 编辑器 — 用现有的（TinyMCE/MDX）
- ❌ 可视化页面构建 — 不是 CMS 的事
- ❌ 电商功能 — 专注内容管理

---

## 10. 编译与测试验证

```
✅ go build ./...        — 通过
✅ go vet ./...          — 通过
✅ go test ./...         — 全部通过（后端 11 个包有测试）
✅ npx vitest run        — 36/36 通过（前端 3 个文件）
```

### 后端覆盖率汇总

```
internal/auth        52.6%
internal/cache       75.0%
internal/config      70.4%
internal/database    (8 migrator tests)
internal/graphql     (18 tests — P2-4)
internal/handlers    75.9%  ⬆ (was 44.3%)
internal/middleware  70.4%
internal/models      74.6%
internal/plugin      (23 tests — P2-6)
internal/services    83.5%  ⬆ (was 64.1%)
```

### 前端覆盖率

```
stores/app.spec.ts    9 tests  ✅
stores/auth.spec.ts  14 tests  ✅
api/http.spec.ts     13 tests  ✅
```

### 测试命令

```bash
go test ./...                       # 全部测试
go test ./internal/services/ -v     # Service 层（含 i18n、工作流、mock）
go test ./internal/graphql/ -v      # GraphQL schema + resolvers
go test ./internal/plugin/ -v       # 插件 Manager + Hook/Filter
make test-cover                     # 覆盖率报告
go test -bench=. ./internal/...     # 基准测试
cd web && npm run test              # 前端测试
```

---

## 依赖关系总览

```
P0 安全加固 ✅
  ├── 1.1 JWT黑名单 ──→ 1.2 Redis集成 ✅
  ├── 1.3 登录防护 ──→ 1.2 Redis集成 ✅
  ├── 1.4 错误脱敏 ✅ (独立)
  ├── 1.5 SVG安全 ✅ (独立)
  └── 1.6-1.7 种子/密码 ✅ (独立)

P1 工程化 ✅
  ├── 2.1 结构化日志 ✅ (独立)
  ├── 2.2 错误码 ✅ ──→ 1.4 错误脱敏 ✅
  ├── 2.3 Repository接口 ✅ ──→ 2.4 测试 ✅
  ├── 2.5 数据库迁移 ✅ (独立)
  ├── 2.6 OpenAPI文档 ✅ (独立)
  └── 2.7-2.8 CI/CD + 部署 ✅ (独立)

P2 功能完善 ✅
  ├── Webhook ✅ (独立)
  ├── S3/OSS ✅ (独立)
  ├── 发布工作流 ✅ (独立)
  ├── GraphQL ✅ (独立)
  ├── i18n ✅ (独立)
  ├── 插件系统 ✅ (独立)
  └── Content Type Builder ✅ (已存在)

P3 商业化（未开始，按能否收钱排序）
  ├── P3-A 硬前提（不做就不能卖）
  │   ├── 7.1 多租户 ──→ 2.3 Repository ✅（全栈影响，需 PoC）
  │   ├── 7.2 计费引擎 ──→ 7.1 多租户
  │   ├── 7.3 API用量计量 ──→ 7.1 多租户 + 1.2 Redis ✅
  │   └── 7.4 SSO/SAML (独立，企业销售并行)
  ├── P3-B 增值（提升客单价）
  │   ├── 7.5 白标域名 ──→ 7.1 多租户
  │   ├── 7.6 SLA/合规 ──→ 7.1 多租户
  │   ├── 7.7 内容协作 (独立)
  │   └── 7.8 图片CDN (独立)
  └── P3-C 工程补丁（基础设施，非商业化）
      ├── 7.9 Prometheus (独立)
      ├── 7.10 OTel (独立)
      ├── 7.11 备份 ──→ 7.1 多租户
      ├── 7.12 全文搜索 (独立，MeiliSearch 不依赖 Redis)
      ├── 7.13 邮件 ──→ 7.2 计费引擎
      └── 7.14 安全增强 (独立)
```
