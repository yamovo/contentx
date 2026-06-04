# 🌪️ VortexCMS

一个基于 **Go + Vue 3** 构建的现代化、高性能内容管理系统。

## ✨ 特性

### 后端 (Go)
- 🚀 **高性能** — Go 编译型语言，极致的并发处理能力
- 🔐 **JWT 认证** — 无状态认证，支持 Access/Refresh Token
- 🛡️ **RBAC 权限** — 细粒度的角色权限控制系统
- 📝 **内容管理** — 文章、页面、分类、标签、评论全功能
- 🗃️ **媒体库** — 文件上传、图片管理、文件夹组织
- 🔍 **SEO 优化** — Meta 管理、Sitemap、Robots.txt、URL 重定向
- 📊 **数据分析** — 访问统计、来源分析、设备分布
- 🧩 **插件系统** — 可扩展的插件架构
- 🎨 **主题引擎** — 支持多主题切换
- 📋 **版本历史** — 文章修订版本追踪与恢复
- 💬 **评论系统** — 多级评论、审核机制、垃圾过滤
- 📡 **RSS Feed** — 自动生成 RSS 2.0 订阅源
- ⚡ **限流保护** — IP 级别的 API 速率限制
- 📦 **数据库支持** — PostgreSQL / MySQL / SQLite

### 前端 (Vue 3)
- 🖥️ **管理后台** — 完整的后台管理界面
- ✍️ **双编辑器** — Markdown + 富文本编辑器
- 📱 **响应式设计** — 适配桌面和移动端
- 🎭 **暗色主题** — 支持亮色/暗色主题切换
- 📈 **数据图表** — ECharts 驱动的可视化仪表盘
- 🖼️ **媒体管理** — 网格/列表视图、拖拽上传
- 🌳 **分类树** — 可拖拽的树形分类管理
- 🔎 **全局搜索** — 文章、评论、用户全文搜索
- 🔄 **实时预览** — Markdown 即时预览

## 🛠️ 技术栈

| 层级 | 技术 |
|------|------|
| 后端语言 | Go 1.22+ |
| Web 框架 | Gin |
| ORM | GORM |
| 数据库 | PostgreSQL 16 / MySQL 8 / SQLite |
| 缓存 | Redis 7 / 内存缓存 |
| 认证 | JWT (HS256) |
| 前端框架 | Vue 3.4 + TypeScript |
| UI 组件 | Element Plus |
| 状态管理 | Pinia |
| 构建工具 | Vite 5 |
| 图表 | ECharts 5 |
| CSS | SCSS + Tailwind |

## 🚀 快速开始

### 方式一：Docker Compose（推荐）

```bash
# 克隆项目
git clone https://github.com/vortexcms/go-cms.git
cd go-cms

# 一键启动
docker-compose up -d

# 访问
# 前台: http://localhost
# 后台: http://localhost/login
# 账号: admin / admin123
```

### 方式二：本地开发

#### 前置条件
- Go 1.22+
- Node.js 20+
- PostgreSQL 或 SQLite

#### 后端
```bash
cd go-cms

# 安装依赖
go mod tidy

# 设置环境变量
export JWT_SECRET=your-secret-key
export DB_DRIVER=sqlite
export DB_NAME=vortexcms

# 运行
go run cmd/server/main.go
```

#### 前端
```bash
cd go-cms/web

# 安装依赖
npm install

# 开发模式
npm run dev     # http://localhost:3000

# 生产构建
npm run build
```

## 📁 项目结构

```
go-cms/
├── cmd/server/              # 应用入口
│   └── main.go
├── internal/                # 内部包（不对外暴露）
│   ├── config/              # 配置加载
│   ├── database/            # 数据库连接、迁移、种子数据
│   ├── models/              # 数据模型
│   ├── auth/                # JWT + 密码处理
│   ├── middleware/           # 中间件（认证、限流、CORS）
│   └── handlers/            # HTTP 处理器 + 路由注册
├── web/                     # Vue 3 前端
│   ├── src/
│   │   ├── api/             # API 接口层
│   │   ├── assets/          # 静态资源
│   │   ├── components/      # 通用组件
│   │   ├── layouts/         # 布局组件
│   │   ├── router/          # 路由配置
│   │   ├── stores/          # Pinia 状态管理
│   │   ├── types/           # TypeScript 类型
│   │   ├── utils/           # 工具函数
│   │   └── views/           # 页面组件
│   └── package.json
├── deploy/                  # 部署配置
│   ├── docker/
│   └── nginx/
├── tests/                   # 测试文件
├── docs/                    # 文档
├── Dockerfile
├── docker-compose.yml
└── README.md
```

## 📡 API 接口

### 认证
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/login` | 登录 |
| POST | `/api/v1/auth/register` | 注册 |
| POST | `/api/v1/auth/refresh` | 刷新 Token |
| POST | `/api/v1/auth/logout` | 登出 |
| GET | `/api/v1/auth/me` | 获取当前用户 |

### 文章
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/articles` | 文章列表 |
| GET | `/api/v1/articles/:id` | 文章详情 |
| POST | `/api/v1/articles` | 创建文章 |
| PUT | `/api/v1/articles/:id` | 更新文章 |
| DELETE | `/api/v1/articles/:id` | 删除文章 |
| POST | `/api/v1/articles/bulk` | 批量操作 |
| GET | `/api/v1/articles/:id/revisions` | 版本历史 |

### 分类 / 标签 / 评论 / 媒体 / 用户 / 角色 / 设置 / SEO / 菜单 / 分析
*(完整 API 文档请参阅 `docs/api.md`)*

## 🔧 配置

### 环境变量

所有配置通过环境变量设置，详见 `deploy/docker/.env.example`。

### 数据库

默认使用 SQLite（零配置），生产环境建议 PostgreSQL：

```bash
DB_DRIVER=postgres
DB_HOST=localhost
DB_PORT=5432
DB_USER=vortexcms
DB_PASSWORD=your-password
DB_NAME=vortexcms
```

## 🧪 测试

```bash
# 后端测试
go test ./...

# 前端测试
cd web && npm test

# E2E 测试
cd web && npm run test:e2e
```

## 📄 License

MIT License

---

> **VortexCMS** — Built with ❤️ using Go + Vue 3
