# 🌪️ ContentX

> **高性能 Go Headless CMS** — API-first 内容平台，单二进制部署，自定义内容类型，自动生成 REST API。

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![Swagger](https://img.shields.io/badge/API--Docs-Swagger-85EA2D?logo=swagger)](/swagger/index.html)
[![License](https://img.shields.io/badge/license-MIT-green)](./LICENSE)

---

## 为什么选 ContentX

| | ContentX | Strapi | Ghost |
|---|-----------|--------|-------|
| 语言 | **Go** | Node.js | Node.js |
| 内存占用 | **~30MB** | ~200MB | ~150MB |
| Docker 镜像 | **< 30MB** | ~500MB | ~400MB |
| 部署方式 | **单二进制** | npm + Node | npm + Node |
| 自定义内容类型 | ✅ | ✅ | ❌ |
| Webhook | ✅ | ✅ | ✅ |
| API Token | ✅ 细粒度 | ✅ | ❌ |
| SDK | TypeScript | JS/TS | ❌ |
| SQLite 零依赖 | ✅ | ❌ | ❌ |

---

## 核心能力

### 🧩 自定义内容类型

像 Strapi 一样定义内容结构，自动生成 CRUD API：

```bash
# 创建 "产品" 内容类型
curl -X POST /api/v1/content-types \
  -H "Authorization: Bearer {token}" \
  -d '{
    "uid": "product",
    "name": "产品",
    "fields": [
      {"name": "title", "label": "标题", "field_type": "text", "required": true},
      {"name": "price", "label": "价格", "field_type": "float", "min_value": 0},
      {"name": "status", "label": "状态", "field_type": "enum", "options": ["在售", "下架"]}
    ]
  }'

# 自动生成的 API：
# GET    /api/v1/content/product           # 列表
# GET    /api/v1/content/product/:id       # 详情
# POST   /api/v1/content/product           # 创建
# PUT    /api/v1/content/product/:id       # 更新
# DELETE /api/v1/content/product/:id       # 删除
# POST   /api/v1/content/product/:id/publish   # 发布
```

### 🔐 API Token 系统

```bash
# 创建细粒度 Token
curl -X POST /api/v1/system/tokens \
  -d '{"name":"Next.js","permissions":["articles.read","categories.read"]}'
# → vc_live_aa3f2a989d57960db02e3328ffe5b079
```

### 🪝 Webhook

内容变更时自动通知外部系统，支持 HMAC 签名：

```bash
curl -X POST /api/v1/webhooks \
  -d '{"name":"Discord","url":"https://hooks.example.com","events":["entry.create","entry.publish"]}'
```

### 📖 Swagger 文档

启动后访问 `http://localhost:8080/swagger/index.html`，36 个接口全部有文档。

---

## 快速开始

### 仅需 Go

```bash
git clone https://github.com/yamovo/go-cms.git
cd go-cms
go run cmd/server/main.go

# API:     http://localhost:8080/api/v1
# Swagger: http://localhost:8080/swagger/index.html
# 账号:    admin / admin123
```

### TypeScript SDK

```bash
npm install @vortexcms/sdk
```

```typescript
import { ContentX } from '@vortexcms/sdk'

const cms = new ContentX({
  baseURL: 'http://localhost:8080/api/v1',
  token: 'vc_live_...',
})

// 内置内容
const articles = await cms.articles.list({ status: 'published' })

// 动态内容类型
const products = await cms.content('product').list()
await cms.content('product').create({
  data: { title: 'Go 语言圣经', price: 99.9, status: '在售' }
})
```

---

## API 概览

| 分组 | 接口数 | 说明 |
|------|--------|------|
| Auth | 7 | 登录、注册、Token 刷新、个人信息 |
| Articles | 10 | CRUD、批量操作、版本历史、RSS |
| Content Types | 4 | 自定义内容类型管理 |
| Content Entries | 8 | 动态内容 CRUD + 发布/取消发布 |
| Categories | 6 | 树形分类、拖拽排序 |
| Tags | 6 | 增删改查、标签合并 |
| Comments | 9 | 审核、垃圾标记、批量操作 |
| Media | 8 | 上传（单/批量）、文件夹管理 |
| Users & Roles | 8 | 用户管理、角色权限分配 |
| Webhooks | 4 | 配置、日志查看 |
| API Tokens | 3 | 创建、列表、删除 |
| SEO | 5 | Meta、Sitemap、重定向 |
| System | 3 | 系统信息、健康检查、活动日志 |

> 完整文档：启动后访问 `/swagger/index.html`

---

## 项目结构

```
go-cms/
├── cmd/server/main.go          # 入口
├── internal/
│   ├── auth/                   # JWT、密码、API Key
│   ├── config/                 # 30+ 环境变量
│   ├── database/               # 连接、迁移、种子
│   ├── models/                 # 数据模型（含动态内容类型）
│   ├── handlers/               # HTTP 处理器（Swagger 注解）
│   ├── services/               # 业务逻辑层
│   ├── middleware/              # 认证、限流、CORS
│   ├── storage/                # 存储驱动（Local / S3）
│   └── cache/                  # 缓存驱动（Memory / Redis）
├── sdk/typescript/             # TypeScript SDK
├── docs/
│   └── api/                    # Swagger JSON/YAML
└── web/                        # Vue 3 管理后台
```

---

## 配置

```env
# 数据库
DB_DRIVER=sqlite               # postgres | mysql | sqlite

# 服务器
SERVER_PORT=8080
SERVER_MODE=debug              # debug | release

# 认证
JWT_SECRET=your-secret-key

# 存储
STORAGE_DRIVER=local           # local | s3
S3_ENDPOINT=minio:9000
S3_BUCKET=vortexcms

# 缓存
CACHE_DRIVER=memory            # memory | redis
REDIS_ADDR=localhost:6379
```

---

## 测试

```bash
go test ./...                       # 全部测试
go test ./internal/services/ -v     # Service 层（39 用例）
```

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.22+ / Gin / GORM |
| 数据库 | PostgreSQL / MySQL / SQLite |
| 认证 | JWT + API Token + RBAC |
| 文档 | Swagger / OpenAPI 2.0 |
| 存储 | Local / S3 兼容 |
| 缓存 | Memory / Redis |
| 前端 | Vue 3 + TypeScript + Element Plus |
| SDK | TypeScript (@vortexcms/sdk) |

---

## License

MIT © 2024 ContentX
