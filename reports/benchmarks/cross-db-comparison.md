# ContentX 压测基线报告 — 跨数据库对照（PostgreSQL / MySQL / SQLite）

> 状态：🚧 进行中。PostgreSQL 与 MySQL 数据已就绪；SQLite 待实测。
>
> 本文档定义 7.2 跨数据库对照的**统一方法**与**可复现流程**，保证三种驱动使用
> 同一数据集、同一场景、同一采样规模。结果表在实测前保持“待测”，不填入未测量数据。

## 1. 对照原则

- **同一数据集**：三种驱动都用等价的 1,000 / 10,000 篇文章 seed（正文均为
  `ContentX benchmark content for realistic payload size.` 重复 40 次）。
  - PostgreSQL：`scripts/benchmark/seed_postgres.sql` / `seed_postgres_10000.sql`
  - MySQL：`scripts/benchmark/seed_mysql.sql` / `seed_mysql_10000.sql`
  - SQLite：`scripts/benchmark/seed_sqlite.sql` / `seed_sqlite_10000.sql`
- **同一场景**：文章列表、文章详情、GraphQL 查询、并发写入四个场景。
- **同一采样规模**：读 1,000 req/s × 15s；写 100 req/s × 10s。
- **同一压测脚本**：`scripts/benchmark/run-benchmark.ps1 -Driver <驱动>`，只有原始结果
  输出目录随驱动变化（`reports/benchmarks/raw/<驱动>/`），场景与速率完全一致。
- **同一空闲内存口径**：`scripts/benchmark/sample-memory.ps1`，无负载下采样 12 次取
  min/mean/max。

## 2. 复现流程

### 2.1 PostgreSQL（已完成，见 [postgres-baseline.md](./postgres-baseline.md)）

```powershell
# 主 compose 即 PostgreSQL 栈
docker compose up -d --build
docker exec -i contentx-db psql -U contentx contentx < scripts/benchmark/seed_postgres_10000.sql
pwsh scripts/benchmark/run-benchmark.ps1 -Driver postgres
pwsh scripts/benchmark/sample-memory.ps1 -Container contentx
```

### 2.2 MySQL（待实测，需 Docker daemon）

```powershell
docker compose -f scripts/benchmark/docker-compose.mysql.yml up -d --build
# 等待 contentx-bench-mysql healthy 后播种
mysql -h127.0.0.1 -P13306 -ucontentx -pbenchpass contentx < scripts/benchmark/seed_mysql_10000.sql
pwsh scripts/benchmark/run-benchmark.ps1 -Driver mysql -AdminPassword 'BenchAdmin123!'
pwsh scripts/benchmark/sample-memory.ps1 -Container contentx-bench-app
docker compose -f scripts/benchmark/docker-compose.mysql.yml down -v
```

### 2.3 SQLite（待实测，无需数据库服务；在普通终端执行）

> 前提：Go 1.21+ 与一个 C 编译器（SQLite 驱动需 CGO，如 MinGW gcc）。内置
> `scripts/benchmark/seeder` 用与应用相同的 GORM SQLite 驱动播种，免 `sqlite3` CLI 依赖。

```powershell
$env:DB_DRIVER="sqlite"; $env:DB_NAME="bench_sqlite.db"; $env:SERVER_MODE="release"
$env:SERVER_PORT="18081"; $env:CACHE_DRIVER="memory"; $env:QUEUE_DRIVER="memory"
$env:JWT_SECRET="bench-jwt-secret-please-use-32-chars-x"; $env:ADMIN_PASSWORD="BenchAdmin123"

# 1) 构建二进制（CGO）
go build -o cxbench.exe ./cmd/server
go build -o cxseed.exe ./scripts/benchmark/seeder

# 2) 建表 + 建 admin，再播 10k
.\cxbench.exe -migrate; .\cxbench.exe -seed
.\cxseed.exe -db bench_sqlite.db -sql scripts/benchmark/seed_sqlite_10000.sql

# 3) 启动应用（启动时从 DB 全量建搜索索引）
.\cxbench.exe

# —— 另开一个终端 ——
pwsh scripts/benchmark/run-benchmark.ps1 -Driver sqlite -BaseUrl http://127.0.0.1:18081 -AdminPassword 'BenchAdmin123'
pwsh scripts/benchmark/sample-memory.ps1 -ProcessName cxbench
```

## 3. 延迟对照（10,000 篇数据集）

### 3.1 文章列表（GET /api/v1/articles?page=1&page_size=20）

| 驱动 | 成功率 | P50 | P95 | P99 | Max |
|---|---:|---:|---:|---:|---:|
| PostgreSQL | 100% | 5.74 ms | 351.12 ms | 1,065.34 ms | 2,334.49 ms |
| MySQL | 93.2%¹ | 19.22 ms | 30,000.20 ms¹ | 30,000.58 ms¹ | 30,003.54 ms¹ |
| SQLite | 待测 | 待测 | 待测 | 待测 | 待测 |

¹ MySQL 列表场景出现 1,018/15,000 请求超时（30s deadline），P95/P99 触顶超时上限。疑似连接池竞争或客户端端口耗尽（TIME_WAIT 堆积），非纯数据库瓶颈。

### 3.2 文章详情（GET /api/v1/articles/:id）

| 驱动 | 成功率 | P50 | P95 | P99 | Max |
|---|---:|---:|---:|---:|---:|
| PostgreSQL | 100% | 2.66 ms | 3.79 ms | 4.82 ms | 26.01 ms |
| MySQL | 100% | 3.10 ms | 4.13 ms | 5.65 ms | 17.41 ms |
| SQLite | 待测 | 待测 | 待测 | 待测 | 待测 |

### 3.3 GraphQL 查询（POST /api/v1/graphql）

| 驱动 | 成功率 | P50 | P95 | P99 | Max |
|---|---:|---:|---:|---:|---:|
| PostgreSQL | 100% | 3.13 ms | 4.30 ms | 5.22 ms | 24.16 ms |
| MySQL | 60.9%² | 26.09 ms | 30,000.53 ms² | 30,000.72 ms² | 30,003.23 ms² |
| SQLite | 待测 | 待测 | 待测 | 待测 | 待测 |

² GraphQL 场景 5,865/15,000 请求失败，错误含 Windows 端口耗尽（`Only one usage of each socket address`）与超时，属客户端侧资源耗尽，非数据库查询性能问题。

### 3.4 并发写入（PUT /api/v1/articles/:id，100 req/s）

| 驱动 | 成功率 | P50 | P95 | P99 | Max |
|---|---:|---:|---:|---:|---:|
| PostgreSQL | 100% | 9.04 ms | 12.04 ms | 17.57 ms | 26.44 ms |
| MySQL | 100% | 7.56 ms | 11.35 ms | 13.03 ms | 15.06 ms |
| SQLite | 待测 | 待测 | 待测 | 待测 | 待测 |

## 4. 空闲内存对照

| 驱动 | 1,000 篇 | 10,000 篇 |
|---|---:|---:|
| PostgreSQL | 待测（统一空闲口径） | 约 145.4 MiB（负载后测量，需按统一口径复测） |
| MySQL | 待测 | 待测 |
| SQLite | 待测 | 待测 |

说明：内置搜索索引在启动时全量加载所有文章到应用内存，因此内存主要由文章数量决定，
与数据库驱动关系较小；本表用于验证该假设并给出可比数字。

## 5. 边界

1. 开发机 Docker 环境非专用硬件，跨库绝对值仅供相对比较，不作为生产容量承诺。
2. SQLite 为单文件嵌入式库，高并发写入存在文件锁串行化，写场景对照需重点关注。
3. MySQL 使用 8.4 镜像、`caching_sha2_password`，与生产托管版本可能有差异。
4. 应用内置搜索索引不跨实例共享，本对照为单实例。

## 6. 待完成项

- [x] 运行 MySQL 全场景并回填 §3（2026-07-23 完成；列表/GraphQL 受客户端端口耗尽影响，详情/写入与 PostgreSQL 相当）
- [ ] 运行 SQLite 全场景并回填 §3、§4
- [ ] 按统一空闲口径复测 PostgreSQL 1,000 / 10,000 篇内存
- [ ] MySQL 列表与 GraphQL 场景在受控环境（Linux + 长端口范围）复测，排除客户端端口耗尽干扰
- [ ] 汇总三库对照结论，更新 PROGRESS.md 的 7.2 状态
