# ContentX 商业化改进路线图

> 基于全面代码审查，按优先级分为 4 个阶段，每个阶段完成后均可独立发布。

---

## 📊 当前状态评估

| 维度 | 评分 | 说明 |
|------|------|------|
| 架构设计 | ⭐⭐⭐☆☆ | 三层架构清晰，但缺 Repository 接口层 |
| 安全性 | ⭐⭐☆☆☆ | JWT/黑名单未实际生效，无账号锁定，SVG 漏洞 |
| 测试覆盖 | ⭐⭐☆☆☆ | 仅 Service 层有测试，Handler/Middleware 零测试 |
| 部署就绪 | ⭐⭐⭐☆☆ | Docker 配置好，但缺 CI/CD、缺 .env.example |
| API 规范 | ⭐⭐⭐☆☆ | 有版本前缀，但无 OpenAPI 文档，响应格式不统一 |
| 日志/可观测 | ⭐☆☆☆☆ | 纯 log.Printf，无结构化日志，无指标采集 |
| 数据库迁移 | ⭐☆☆☆☆ | 仅 AutoMigrate，无法回滚，无版本管理 |
| 文档 | ⭐☆☆☆☆ | 无 API 文档、无开发者指南、无部署文档 |

---

## 🔴 第一阶段：P0 — 安全加固 + 核心缺陷修复

> 目标：消除生产环境阻塞性问题，预计 3-5 天

### 1.1 JWT 黑名单实际生效
**问题**：`AuthMiddleware` 从不检查黑名单 → logout 无效

- `internal/auth/jwt.go:142-151` — 内存黑名单，重启丢失
- `internal/middleware/auth.go:21-56` — 未调用 `blacklist.IsRevoked()`

**改动**：
```go
// middleware/auth.go — 在 token 验证后添加
if authManager.Blacklist.IsRevoked(tokenStr) {
    c.AbortWithStatusJSON(401, gin.H{"error": "token revoked"})
    return
}
```
同时将黑名单迁移到 Redis（见 1.2）

### 1.2 Redis 集成（黑名单 + 缓存）
**问题**：`RedisConfig` 存在但从未使用

- 新建 `internal/cache/redis.go` — 连接池、健康检查
- 将 `Blacklist` 改为 Redis SET 实现（TTL = token 剩余有效期）
- 为后续缓存、队列打基础

### 1.3 登录暴力破解防护
**问题**：仅有 IP 级限流，无账号锁定

**改动**：
- 新增 `LoginAttempt` 模型（user_id, ip, attempts, locked_until）
- 连续 5 次失败 → 锁定 15 分钟
- Redis 存储（key: `login:attempts:{user_id}`，TTL 15min）

### 1.4 错误响应脱敏
**问题**：`err.Error()` 直接返回给客户端

```go
// 改前
c.JSON(500, gin.H{"error": err.Error()})

// 改后
log.Error("internal error", "request_id", c.GetString("request_id"), "err", err)
c.JSON(500, gin.H{"error": "Internal server error", "code": "INTERNAL_ERROR"})
```

### 1.5 SVG 上传安全
**问题**：SVG 可包含 `<script>`，XSS 风险

- 从 `config.go:221` 默认允许列表中移除 `.svg`
- 或添加 SVG 清理（strip `<script>` 等标签）

### 1.6 种子文件统一
**问题**：`database/seed.go` 和 `database/seeds/seeds.go` 重复定义 `SeedAll()`

- 删除 `database/seeds/seeds.go`（死代码）
- 统一使用 `database/seed.go`

### 1.7 移除硬编码回退密码
**问题**：`seed.go:252` — 随机密码生成失败时使用 `"ChangeMeNow123!"`

```go
// 改为直接报错，不允许弱密码
if err != nil {
    return fmt.Errorf("failed to generate admin password: %w", err)
}
```

---

## 🟡 第二阶段：P1 — 工程化 + 可观测性

> 目标：具备团队协作和生产运维能力，预计 5-7 天

### 2.1 结构化日志
**现状**：全部 `log.Printf`，无级别、无字段

**改动**：
- 使用 Go 1.21+ 标准库 `log/slog`
- JSON 输出格式，支持日志级别（debug/info/warn/error）
- 将 `LogConfig` 实际接入（`config.go:170-178`）
- 替换所有 `log.Printf` → `slog.Info/Error/Warn`

```go
// 统一初始化
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: cfg.Log.Level, // debug, info, warn, error
}))
slog.SetDefault(logger)
```

### 2.2 统一错误码体系
**现状**：错误信息为自由文本，客户端无法程序化处理

```go
// internal/errs/errs.go
type AppError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Status  int    `json:"-"`
}

var (
    ErrNotFound     = &AppError{Code: "NOT_FOUND", Message: "Resource not found", Status: 404}
    ErrUnauthorized = &AppError{Code: "UNAUTHORIZED", Message: "Authentication required", Status: 401}
    ErrForbidden    = &AppError{Code: "FORBIDDEN", Message: "Insufficient permissions", Status: 403}
    ErrValidation   = &AppError{Code: "VALIDATION_ERROR", Message: "Validation failed", Status: 422}
    // ...
)
```

### 2.3 Repository 接口层
**现状**：Service 直接依赖 `*gorm.DB`，无法 mock 测试

```go
// internal/repository/article_repository.go
type ArticleRepository interface {
    Create(ctx context.Context, article *models.Article) error
    GetByID(ctx context.Context, id uint) (*models.Article, error)
    List(ctx context.Context, filter ArticleFilter) ([]models.Article, int64, error)
    Update(ctx context.Context, article *models.Article) error
    Delete(ctx context.Context, id uint) error
}

// 实现
type gormArticleRepository struct { db *gorm.DB }
```

### 2.4 Handler + Middleware 测试
**现状**：零 HTTP 层测试

- 为每个 Handler 编写 `httptest` 测试
- 为 AuthMiddleware、RateLimitMiddleware 编写测试
- 目标：Handler 层覆盖率达 60%+

### 2.5 数据库迁移工具
**现状**：仅 `AutoMigrate`，无法回滚

**方案**：使用 `golang-migrate/migrate`
```
migrations/
  000001_init_schema.up.sql
  000001_init_schema.down.sql
  000002_add_seo_fields.up.sql
  000002_add_seo_fields.down.sql
```

- 新增 CLI 命令：`vortexcms migrate up/down/status`
- 从现有模型逆向生成初始迁移文件

### 2.6 OpenAPI 文档
**现状**：无任何 API 文档

- 使用 `swaggo/swag` 从 Handler 注释自动生成
- 访问 `/swagger/index.html`
- 覆盖全部 80+ 端点

### 2.7 CI/CD 流水线
```yaml
# .github/workflows/ci.yml
- lint (golangci-lint)
- test (go test -cover ./...)
- build (docker build)
- deploy (staging/production)
```

### 2.8 部署配置补全
- 创建 `deploy/docker/.env.example`
- 创建 `deploy/nginx/nginx.conf`
- 创建 `Makefile`（build, test, lint, migrate, seed, dev）
- 创建 `.golangci.yml`

---

## 🟢 第二阶段：P2 — 功能完善

> 目标：达到竞品同等功能水平，预计 7-10 天

### 3.1 邮件系统完善
- 邮箱验证注册流程
- 密码重置（邮件发送重置链接）
- 评论邮件通知
- `MailConfig` 已存在，需实现 SMTP 发送

### 3.2 全文搜索
**现状**：文章搜索用 SQL LIKE

- 集成 MeiliSearch 或 Elasticsearch
- `SearchConfig` 已存在但未接入
- 支持中文分词

### 3.3 限流响应国际化
**现状**：`ratelimit.go:54` 返回中文 `"请求过于频繁"`

- 改为英文错误码 + 可选消息翻译

### 3.4 安全头更新
- 移除已废弃的 `X-XSS-Protection`
- 收紧 CSP（移除 `unsafe-inline`，使用 nonce）
- 添加 `Permissions-Policy`

### 3.5 API 响应格式统一
```json
// 统一格式
{
  "code": 0,
  "message": "success",
  "data": { ... },
  "meta": {
    "page": 1,
    "page_size": 20,
    "total": 100
  }
}
```

### 3.6 插件系统实现
**现状**：仅数据库记录，无实际加载机制

- 定义 Plugin 接口
- 实现 Go plugin 或进程外插件加载
- 钩子系统（Hook/Filter）

### 3.7 主题系统完善
- 主题文件结构规范
- 主题切换机制
- 子主题支持

---

## 🔵 第四阶段：P3 — 商业化特性

> 目标：具备 SaaS 商业化能力，预计 10-15 天

### 4.1 多租户架构
- 数据库级隔离（schema per tenant）或行级隔离（tenant_id）
- 租户管理后台
- 配额管理（存储、API 调用）

### 4.2 API Key 认证
- 为第三方集成提供 API Key
- 权限范围控制（scope）
- 调用配额和速率限制

### 4.3 双因素认证 (2FA)
- TOTP（Google Authenticator）
- 备份恢复码

### 4.4 会话管理
- 活跃会话列表
- 远程注销
- 设备指纹

### 4.5 Prometheus 指标
```
http_requests_total{method, path, status}
http_request_duration_seconds{method, path}
active_users_total
articles_total
```

### 4.6 OpenTelemetry 分布式追踪
- Request ID → Trace ID 映射
- 跨服务调用追踪

### 4.7 定时备份实现
**现状**：`BackupConfig` 存在但无实现

- 数据库定时备份（pg_dump/mysqldump）
- 备份文件上传到 S3/MinIO
- 保留策略（最近 7 天 + 每月 1 个）

### 4.8 内容审核
- 敏感词过滤
- 评论自动审核（Akismet 类似服务）
- 内容合规检查

---

## 📋 依赖关系图

```
P0 安全加固
  ├── 1.1 JWT黑名单 ──→ 1.2 Redis集成
  ├── 1.3 登录防护 ──→ 1.2 Redis集成
  ├── 1.4 错误脱敏 (独立)
  ├── 1.5 SVG安全 (独立)
  └── 1.6-1.7 种子/密码 (独立)

P1 工程化
  ├── 2.1 结构化日志 (独立)
  ├── 2.2 错误码 ──→ 1.4 错误脱敏
  ├── 2.3 Repository接口 ──→ 2.4 测试
  ├── 2.5 数据库迁移 (独立)
  ├── 2.6 OpenAPI文档 (独立)
  └── 2.7-2.8 CI/CD + 部署 (独立)

P2 功能完善
  ├── 3.1 邮件系统 (独立)
  ├── 3.2 全文搜索 ──→ 1.2 Redis集成
  └── 3.3-3.7 (均相对独立)

P3 商业化
  ├── 4.1 多租户 ──→ 2.3 Repository接口
  ├── 4.2 API Key ──→ 1.2 Redis集成
  └── 4.3-4.8 (均相对独立)
```

---

## ⏱️ 预估时间

| 阶段 | 工作量 | 建议周期 |
|------|--------|----------|
| P0 安全加固 | 3-5 天 | 第 1 周 |
| P1 工程化 | 5-7 天 | 第 2-3 周 |
| P2 功能完善 | 7-10 天 | 第 4-5 周 |
| P3 商业化 | 10-15 天 | 第 6-8 周 |
| **合计** | **25-37 天** | **约 2 个月** |

---

## 🎯 建议启动顺序

1. **先做 1.1 + 1.2**（JWT + Redis）— 解决最核心的安全缺陷
2. **再做 1.4 + 2.2**（错误处理）— 统一错误体系，为后续开发铺路
3. **然后 2.1 + 2.5**（日志 + 迁移）— 工程化基础
4. **最后 2.3 + 2.4**（Repository + 测试）— 提升代码可维护性

每个阶段完成后打 tag 发布，确保随时可交付。
