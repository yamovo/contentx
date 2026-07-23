# ContentX 压测基线报告 — 跨数据库对照（PostgreSQL / MySQL / SQLite）

> **Round 3 重跑结果（2026-07-23）**
>
> 本文档 §3 的数据已于 2026-07-23 由 Round 3 行程 C 重新生成，替换了此前标记为
> "失效历史数据"的版本。重跑条件：
> - 三库同一 Git SHA（`0f5d624`）、同一 10,000 篇数据集、同一响应字段（列表省略
>   `content`）、同一速率（读 1,000 req/s、写 100 req/s）、同一 Linux 容器内发起压测。
> - 三库均从 Docker 网络内的 Linux 容器运行 vegeta（`runner=linux-container`），
>   消除了 Windows 客户端端口耗尽对历史 MySQL 数据的污染（S0-3 修正）。
> - 三库均使用 Redis 作为缓存与队列驱动，唯一变量为数据库驱动。
> - 报告由 `scripts/benchmark/generate-report.ps1` 从 Vegeta JSON 自动生成，含一致性校验。
>
> 历史失效数据保留在 `raw/<driver>/historical/` 子目录，禁止引用其结论。

## 1. 对照原则

- **同一数据集**：三种驱动都用等价的 10,000 篇文章 seed（正文均为
  `ContentX benchmark content for realistic payload size.` 重复 40 次）。
  - PostgreSQL：`scripts/benchmark/seed_postgres_10000.sql`
  - MySQL：`scripts/benchmark/seed_mysql_10000.sql`
  - SQLite：`scripts/benchmark/seed_sqlite_10000.sql`
- **同一场景**：文章列表、文章详情、GraphQL 查询、并发写入四个场景。
- **同一采样规模**：读 1,000 req/s × 15s；写 100 req/s × 10s。
- **同一压测路径**：三库均从 Docker 网络内的 Linux 容器运行
  `scripts/benchmark/run-benchmark-linux.sh`（C3/C4），消除 Windows 客户端端口耗尽。
  SQLite 虽为嵌入式数据库，但为保持条件一致同样从 Linux 容器内发起压测。
- **同一空闲内存口径**：`scripts/benchmark/sample-memory.ps1`，无负载下采样 12 次取
  min/mean/max。
- **同一元数据**：每次运行保存 `run-metadata.json`（Git SHA、文章数、响应体大小、
  应用配置、采样 goroutine 数），由 `generate-report.ps1` 读入报告头部。

## 2. 复现流程

### 2.1 PostgreSQL（主 compose 栈）

```powershell
# 主 compose 即 PostgreSQL 栈
docker compose up -d --build

# 播种 10,000 篇
Get-Content scripts/benchmark/seed_postgres_10000.sql | docker exec -i contentx-db psql -U contentx contentx

# 从 Linux 容器内运行压测（消除 Windows 端口耗尽）：
# 1) 先构建 bench-runner 镜像
docker build -t contentx-bench-runner -f scripts/benchmark/Dockerfile.bench --build-arg GOPROXY=https://goproxy.cn,direct .
# 2) 在主栈网络上运行 bench-runner
$sha = (git rev-parse --short HEAD).Trim()
docker run --rm --network contentx-main_contentx-net `
    -e BASE_URL=http://contentx:8080 -e ADMIN_PASSWORD=<你的 admin 密码> `
    -e OUTPUT_DIR=/out -e DRIVER=postgres -e GIT_SHA=$sha -e GIT_BRANCH=main `
    -v "$PWD\reports\benchmarks\raw\postgres:/out" contentx-bench-runner

# 生成报告
pwsh scripts/benchmark/generate-report.ps1 -Driver postgres
```

### 2.2 MySQL（独立 benchmark 栈 + Linux 容器压测）

```powershell
# 构建并启动 MySQL benchmark 栈
docker compose -f scripts/benchmark/docker-compose.mysql.yml up -d --build

# 等待 contentx-bench-mysql healthy 后播种
Get-Content scripts/benchmark/seed_mysql_10000.sql | mysql -h127.0.0.1 -P13306 -ucontentx -pbenchpass contentx

# 构建 bench-runner 镜像（如尚未构建）
docker build -t contentx-bench-runner -f scripts/benchmark/Dockerfile.bench --build-arg GOPROXY=https://goproxy.cn,direct .

# 从 Linux 容器内运行压测
$sha = (git rev-parse --short HEAD).Trim()
docker run --rm --network benchmark_bench-net `
    -e BASE_URL=http://app:8080 -e ADMIN_PASSWORD=BenchAdmin123! `
    -e OUTPUT_DIR=/out -e DRIVER=mysql -e GIT_SHA=$sha -e GIT_BRANCH=main `
    -v "$PWD\reports\benchmarks\raw\mysql:/out" contentx-bench-runner

# 生成报告
pwsh scripts/benchmark/generate-report.ps1 -Driver mysql

# 清理
docker compose -f scripts/benchmark/docker-compose.mysql.yml down -v
```

### 2.3 SQLite（独立 benchmark 栈 + Linux 容器压测）

> SQLite 为嵌入式数据库，无需独立 DB 容器；DB 文件存于 named volume。
> 缓存与队列使用 Redis（与 PostgreSQL/MySQL 一致），唯一变量为数据库驱动。
> CGO 构建在 Docker 多阶段构建中完成，无需宿主机安装 C 编译器。

```powershell
# 构建并启动 SQLite benchmark 栈（app + redis + seeder）
docker compose -f scripts/benchmark/docker-compose.sqlite.yml up -d --build

# seeder 会在 app healthy 后自动播种 10,000 篇并退出。
# 确认播种完成（应输出 articles rows = 10000）：
docker logs contentx-bench-sqlite-seeder

# 构建 bench-runner 镜像（如尚未构建，见 §2.1/2.2）
# 从 Linux 容器内运行压测
$sha = (git rev-parse --short HEAD).Trim()
docker run --rm --network benchmark_sqlite-net `
    -e BASE_URL=http://app:8080 -e ADMIN_PASSWORD=BenchAdmin123! `
    -e OUTPUT_DIR=/out -e DRIVER=sqlite -e GIT_SHA=$sha -e GIT_BRANCH=main `
    -v "$PWD\reports\benchmarks\raw\sqlite:/out" contentx-bench-runner

# 生成报告
powershell -ExecutionPolicy Bypass -File scripts\benchmark\generate-report.ps1 -Driver sqlite

# 清理
docker compose -f scripts/benchmark/docker-compose.sqlite.yml down -v
```

## 3. 延迟对照（10,000 篇数据集，Round 3 重跑）

> 以下数据由 `generate-report.ps1` 从 Vegeta JSON 自动生成，一致性校验 **PASS**。
> 三库均从 Linux 容器内运行 vegeta（runner=`linux-container`），消除了 Windows
> 客户端端口耗尽（S0-3 修正）。三库均使用 Redis 缓存/队列，唯一变量为数据库驱动。

### 3.1 文章列表（GET /api/v1/articles?page=1&page_size=20）

| 驱动 | 成功率 | P50 | P95 | P99 | Max | 吞吐 (req/s) | 响应 (KB) |
|---|---:|---:|---:|---:|---:|---:|---:|
| PostgreSQL | 100% | 8.293 ms | 13.354 ms | 21.754 ms | 51.933 ms | 999.5 | 28.0 |
| MySQL | 94.70% | 18,664.101 ms | 29,981.309 ms | 30,000.951 ms | 30,014.797 ms | 351.0 | 25.9 |
| SQLite | 100% | 3,557.849 ms | 7,692.494 ms | 9,310.831 ms | 14,084.089 ms | 733.9 | 27.5 |

### 3.2 文章详情（GET /api/v1/articles/:id）

| 驱动 | 成功率 | P50 | P95 | P99 | Max | 吞吐 (req/s) | 响应 (KB) |
|---|---:|---:|---:|---:|---:|---:|---:|
| PostgreSQL | 100% | 1.524 ms | 1.984 ms | 2.587 ms | 4.057 ms | 999.96 | 3.6 |
| MySQL | 100% | 2.440 ms | 3.112 ms | 4.391 ms | 8.350 ms | 999.88 | 1.4 |
| SQLite | 100% | 0.851 ms | 1.311 ms | 1.767 ms | 4.246 ms | 1000.0 | 3.6 |

### 3.3 GraphQL 查询（POST /api/v1/graphql）

| 驱动 | 成功率 | P50 | P95 | P99 | Max | 吞吐 (req/s) | 响应 (KB) |
|---|---:|---:|---:|---:|---:|---:|---:|
| PostgreSQL | 100% | 8.239 ms | 11.219 ms | 18.698 ms | 44.937 ms | 999.55 | 2.4 |
| MySQL | 62.77% | 28,045.165 ms | 30,001.013 ms | 30,002.497 ms | 30,009.900 ms | 217.4 | 1.5 |
| SQLite | 72.00% | 26,560.965 ms | 30,001.488 ms | 30,003.888 ms | 30,013.507 ms | 267.8 | 1.6 |

### 3.4 并发写入（PUT /api/v1/articles/:id，100 req/s）

| 驱动 | 成功率 | P50 | P95 | P99 | Max | 吞吐 (req/s) | 响应 (KB) |
|---|---:|---:|---:|---:|---:|---:|---:|
| PostgreSQL | 100% | 7.480 ms | 9.699 ms | 12.489 ms | 32.637 ms | 100.04 | 1.5 |
| MySQL | 100% | 149.203 ms | 286.882 ms | 339.656 ms | 394.756 ms | 98.0 | 1.5 |
| SQLite | 100% | 1.986 ms | 3.121 ms | 13.381 ms | 19.947 ms | 100.1 | 1.5 |

## 4. 列表查询根因归因（C6）

> 此前标记为"待定位"。Round 3 从 Linux 容器内重跑后，MySQL 与 SQLite 的列表/GraphQL
> 超时根因均已由 EXPLAIN 实测归因。

### 4.1 排除：Windows 客户端端口耗尽

历史 MySQL 运行从 Windows 宿主机发起，错误含 `Only one usage of each socket
address`（客户端端口耗尽）。Round 3 改为从 Docker 网络内的 Linux 容器运行 vegeta
（`runner=linux-container`），彻底消除 Windows 网络栈。重跑后错误变为
`context deadline exceeded`（30s 服务端超时），**不再出现端口耗尽错误**。

**结论：历史"客户端侧资源耗尽"假说不成立。MySQL/SQLite 列表/GraphQL 超时为服务端问题。**

### 4.2 实测归因：三库查询计划对照

文章列表与 GraphQL 查询都执行相同的 SQL 模式：
`SELECT ... FROM articles WHERE status='published' AND post_type='post'
 ORDER BY is_pinned DESC, published_at DESC, created_at DESC LIMIT 20 OFFSET 0`

应用代码（`internal/repository/article.go`）对三库使用完全相同的 GORM 查询构建器，
索引集也相同（GORM AutoMigrate 从 struct tag 生成单列索引）。差异完全在数据库查询计划：

**PostgreSQL**（P50=8.3 ms，1000 req/s 下 100% 成功）：
```
Limit  (cost=484.33..485.55 rows=20 width=4218)
  ->  Incremental Sort  (cost=484.33..1093.42 rows=10000 width=4218)
        Sort Key: is_pinned DESC, published_at DESC, created_at DESC
        Presorted Key: is_pinned
        ->  Index Scan Backward using idx_articles_is_pinned on articles
              Filter: (status = 'published' AND post_type = 'post')
```
PostgreSQL 利用 `idx_articles_is_pinned` 索引反向扫描提供 `is_pinned` 预排序，
再对 `published_at`、`created_at` 做增量排序（Incremental Sort），仅需对相邻的
`is_pinned` 分组内排序，无需全量 filesort。

**MySQL**（P50=18,664 ms，1000 req/s 下 94.7% 成功，795/15000 超时）：
```
type: ref
key:  idx_articles_status
rows: 4170
Extra: Using where; Using filesort
```
MySQL 先用 `idx_articles_status` 过滤出 4170 行，然后对全部 4170 行做 **filesort**
（内存/磁盘排序），因为没有任何索引能辅助三列 `ORDER BY`。MySQL 8.4 不支持
PostgreSQL 的 Incremental Sort 优化。每个列表请求都要执行一次 filesort，在
1000 req/s 并发 + MaxOpenConns=25 的连接池下，请求堆积直至 30s 超时。

**SQLite**（P50=3,558 ms，1000 req/s 下 100% 成功；GraphQL P50=26,561 ms，72% 成功）：
```
QUERY PLAN
|--SCAN articles
`--USE TEMP B-TREE FOR ORDER BY
```
SQLite 对 `articles` 做全表扫描（`SCAN`），再用临时 B-tree 对全部 10,000 行排序
（`USE TEMP B-TREE FOR ORDER BY`，等价于 MySQL 的 filesort）。SQLite 同样没有
复合索引辅助三列 `ORDER BY`，也不支持 PostgreSQL 的 Incremental Sort。列表场景
虽未超时（SQLite 单连接串行执行，无连接池堆积），但 P50 已达 3.6 秒；GraphQL
场景因额外查询开销叠加，28% 请求超过 30s 客户端超时。

### 4.3 验证：单行操作不受影响

| 场景 | SQL 模式 | PostgreSQL P50 | MySQL P50 | SQLite P50 | 说明 |
|---|---|---:|---:|---:|---|
| 文章列表 | COUNT + ORDER BY + LIMIT + filesort | 8.3 ms | 18,664 ms | 3,558 ms | MySQL/SQLite filesort |
| 文章详情 | PK 查询 | 1.5 ms | 2.4 ms | 0.85 ms | 均走主键索引，无排序 |
| GraphQL | 同列表查询 | 8.2 ms | 28,045 ms | 26,561 ms | 同列表瓶颈 |
| 并发写入 | PK 更新 | 7.5 ms | 149.2 ms | 1.99 ms | 无排序，SQLite 最快（嵌入式无网络） |

详情与写入场景在三库上均 100% 成功，证明三库本身功能正常，瓶颈仅在涉及
多列排序的列表/查询路径。SQLite 在单行操作（详情、写入）上最快，因为嵌入式
数据库无网络往返开销；但在多列排序的列表查询上同样受限于无复合索引。

### 4.4 改进方向（非本轮阻断项）

1. **添加复合索引** `(status, post_type, is_pinned, published_at, created_at)` —
   可让 MySQL 与 SQLite 也走索引扫描避免 filesort/TEMP B-TREE。需通过 GORM struct
   tag 或 migration 添加。PostgreSQL 虽已有 Incremental Sort 优化，复合索引可进一步
   降低延迟。
2. **调大连接池** MaxOpenConns（当前默认 25）以应对高并发列表请求（主要影响 MySQL）。
3. **列表场景使用搜索索引** — 当前列表走 DB，不走内置搜索索引。如果列表只需
   title/slug/excerpt 等少量字段，可考虑从内存索引返回，避免 DB 排序。

## 5. 空闲内存对照

| 驱动 | 10,000 篇 |
|---|---:|
| PostgreSQL | 待测（统一空闲口径） |
| MySQL | 待测（统一空闲口径） |
| SQLite | 待测 |

说明：内置搜索索引在启动时全量加载所有文章到应用内存，因此内存主要由文章数量决定，
与数据库驱动关系较小；本表用于验证该假设并给出可比数字。

## 6. 边界

1. 开发机 Docker 环境非专用硬件，跨库绝对值仅供相对比较，不作为生产容量承诺。
2. SQLite 为单文件嵌入式库，高并发写入存在文件锁串行化，写场景对照需重点关注。
3. MySQL 使用 8.4 镜像、`caching_sha2_password`，与生产托管版本可能有差异。
4. 应用内置搜索索引不跨实例共享，本对照为单实例。
5. 所有性能数字为阶段性本机结果，非 SLA。

## 7. 待完成项

- [x] ~~运行 MySQL 全场景并回填 §3~~（Round 3 重跑完成，2026-07-23）
- [x] ~~修正 §3 MySQL 延迟单位错误，标记 PostgreSQL/MySQL 横向对照为失效历史数据~~（Round 1 完成，Round 3 重跑替换）
- [x] 报告由脚本自动从 Vegeta JSON 生成 Markdown，禁止手工抄写单位（C5 完成）
- [x] 原始 JSON 与 Markdown 表格增加一致性校验（C5 完成）
- [x] PostgreSQL 与 MySQL 在同一 Git SHA、同一 10,000 篇数据、同一响应字段、同一速率、
  同一 Linux 容器条件下重跑（C1/C2/C3 完成，2026-07-23）
- [x] 每次运行保存实际 `COUNT(*)`、Git SHA、应用配置和响应体大小（C1 完成）
- [x] 在 Linux/WSL 受控环境对 MySQL 列表/GraphQL/并发写入复测，归因超时与排队根因
  （C3/C6 完成，2026-07-23 — 根因：MySQL filesort vs PostgreSQL Incremental Sort）
- [x] 运行 SQLite 全场景并回填 §3（C4 完成，2026-07-23 — Docker CGO 构建 + 10,000 篇，
  根因：SQLite SCAN + TEMP B-TREE，同 MySQL 无复合索引瓶颈）
- [ ] 按统一空闲口径复测 PostgreSQL / MySQL / SQLite 10,000 篇内存
- [ ] 汇总三库对照结论，更新 ROADMAP.md Round 3 状态
