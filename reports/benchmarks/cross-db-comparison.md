# ContentX 压测基线报告 — 跨数据库对照（PostgreSQL / MySQL / SQLite）

> ⚠️ **失效历史数据 — 不可用于数据库性能结论**
>
> 本文档 §3 中的现有 PostgreSQL 与 MySQL 横向对照数据已于 2026-07-23 标记为
> **无效历史数据**，原因见 [ROADMAP.md](../../docs/ROADMAP.md) 的 S0-3 与 S0-4：
>
> - **不可比（S0-3）**：两组结果不是同一实验。PostgreSQL 实际为 1,000 篇文章、
>   正文精简**前**的列表响应（约 72,740 B）；MySQL 为 10,000 篇文章、正文精简
>   **后**版本。数据集规模、响应字段均不一致。
> - **单位错误（S0-4）**：Vegeta JSON 延迟单位为纳秒，原报告把部分 P50/P95/P99/Max
>   值缩小了 1,000 倍（见 §3 各行脚注）。已修正数值，但**不得据此得出任何横向结论**。
> - **归因待定**：MySQL 列表/GraphQL 超时、并发写入排队的根因当前统一记为
>   “待定位”，需在 Linux/WSL 受控环境重测后再归因。
>
> 本文档的 §1 对照原则、§2 复现流程仍有效，作为后续重跑的方法依据。
> §3 表格保留为历史快照并明确标注错误，禁止引用其横向结论。
>
> 重跑计划见 [ROADMAP.md](../../docs/ROADMAP.md) Round 3。

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

### 2.1 PostgreSQL（历史结果已失效，见 [postgres-baseline.md](./postgres-baseline.md)；需按 10,000 篇重跑）

```powershell
# 主 compose 即 PostgreSQL 栈
docker compose up -d --build

# 播种 10,000 篇。PowerShell 不支持 `<` 重定向，用 Get-Content 管道；
# Bash 用户可改用：docker exec -i contentx-db psql -U contentx contentx < scripts/benchmark/seed_postgres_10000.sql
Get-Content scripts/benchmark/seed_postgres_10000.sql | docker exec -i contentx-db psql -U contentx contentx

# -BaseUrl 现为可选：-Driver postgres 默认 http://127.0.0.1:18080（S1-2）
pwsh scripts/benchmark/run-benchmark.ps1 -Driver postgres
pwsh scripts/benchmark/sample-memory.ps1 -Container contentx
```

> 历史基线实际使用 `seed_postgres.sql`（1,000 篇），已标记为失效历史数据（S0-3）。
> 上述命令使用 `seed_postgres_10000.sql`（10,000 篇）用于行程 C 重跑。

### 2.2 MySQL（待实测，需 Docker daemon）

```powershell
docker compose -f scripts/benchmark/docker-compose.mysql.yml up -d --build
# 等待 contentx-bench-mysql healthy 后播种。PowerShell 用 Get-Content 管道；
# Bash 用户可改用：mysql -h127.0.0.1 -P13306 -ucontentx -pbenchpass contentx < scripts/benchmark/seed_mysql_10000.sql
Get-Content scripts/benchmark/seed_mysql_10000.sql | mysql -h127.0.0.1 -P13306 -ucontentx -pbenchpass contentx

# -BaseUrl 现为可选：-Driver mysql 默认 http://127.0.0.1:18090（S1-2）。
# 应用端口 18090 与 PostgreSQL 的 18080 不同，两栈可并行运行。
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

> ⚠️ **本节为失效历史数据**（S0-3/S0-4）。PostgreSQL 行实际为 1,000 篇文章、
> 正文精简前版本；MySQL 行已修正单位错误（原报告把纳秒当毫秒，缩小 1,000 倍）。
> 两行**不可横向比较**，根因“待定位”。保留仅作历史快照，禁止引用其结论。

### 3.1 文章列表（GET /api/v1/articles?page=1&page_size=20）

| 驱动 | 成功率 | P50 | P95 | P99 | Max |
|---|---:|---:|---:|---:|---:|
| PostgreSQL（实为 1,000 篇，正文精简前） | 100% | 5.74 ms | 351.12 ms | 1,065.34 ms | 2,334.49 ms |
| MySQL（10,000 篇，正文精简后） | 93.2%¹ | 19,224.45 ms¹ | 30,000.20 ms¹ | 30,000.58 ms¹ | 30,003.54 ms¹ |
| SQLite | 待测 | 待测 | 待测 | 待测 | 待测 |

¹ MySQL P50 原报告误写为 19.22 ms（把纳秒当毫秒，缩小 1,000 倍），正确值为
19,224.45 ms（来自 `raw/mysql/article-list.json` 的 `50th=19,224,451,797 ns`）。
P95/P99/Max 原报告数值正确（已触顶 30s 超时上限）。1,018/15,000 请求超时。
根因“待定位”，需在 Linux/WSL 受控环境重测后再归因，**不得据此判定 MySQL 列表性能**。

### 3.2 文章详情（GET /api/v1/articles/:id）

| 驱动 | 成功率 | P50 | P95 | P99 | Max |
|---|---:|---:|---:|---:|---:|
| PostgreSQL（实为 1,000 篇，正文精简前） | 100% | 2.66 ms | 3.79 ms | 4.82 ms | 26.01 ms |
| MySQL（10,000 篇，正文精简后） | 100% | 3.10 ms | 4.13 ms | 5.65 ms | 17.41 ms |
| SQLite | 待测 | 待测 | 待测 | 待测 | 待测 |

说明：本场景 MySQL 数值**未发生单位错误**（详情查询延迟在微秒到毫秒级，
原报告与原始 JSON 一致）。但因 PostgreSQL/MySQL 数据集规模与响应字段不一致
（S0-3），该行同样**不可横向比较**，仅作历史快照保留。

### 3.3 GraphQL 查询（POST /api/v1/graphql）

| 驱动 | 成功率 | P50 | P95 | P99 | Max |
|---|---:|---:|---:|---:|---:|
| PostgreSQL（实为 1,000 篇，正文精简前） | 100% | 3.13 ms | 4.30 ms | 5.22 ms | 24.16 ms |
| MySQL（10,000 篇，正文精简后） | 60.9%² | 26,090.59 ms² | 30,000.53 ms² | 30,000.72 ms² | 30,003.23 ms² |
| SQLite | 待测 | 待测 | 待测 | 待测 | 待测 |

² MySQL P50 原报告误写为 26.09 ms（把纳秒当毫秒，缩小 1,000 倍），正确值为
26,090.59 ms（来自 `raw/mysql/graphql.json` 的 `50th=26,090,588,280 ns`）。
5,865/15,000 请求失败，错误含 Windows 端口耗尽（`Only one usage of each socket address`）
与超时。原报告据此判定“客户端侧资源耗尽，非数据库瓶颈”**证据不足**，根因“待定位”。

### 3.4 并发写入（PUT /api/v1/articles/:id，100 req/s）

| 驱动 | 成功率 | P50 | P95 | P99 | Max | 实际吞吐 |
|---|---:|---:|---:|---:|---:|---:|
| PostgreSQL（实为 1,000 篇，正文精简前） | 100% | 9.04 ms | 12.04 ms | 17.57 ms | 26.44 ms | 100.02 req/s |
| MySQL（10,000 篇，正文精简后） | 100%³ | 7,562.99 ms³ | 11,348.47 ms³ | 13,028.37 ms³ | 15,063.50 ms³ | 60.73 req/s³ |
| SQLite | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 |

³ MySQL 并发写入 P50/P95/P99/Max 原报告全部误写（7.56/11.35/13.03/15.06 ms，
缩小 1,000 倍），正确值来自 `raw/mysql/concurrent-write.json`：
`50th=7,562,995,186 ns`、`95th=11,348,473,459 ns`、`99th=13,028,366,740 ns`、
`max=15,063,502,100 ns`。实际吞吐 60.73 req/s，**未达到目标 100 req/s**。
原报告“MySQL 写入略优”“MySQL 与 PostgreSQL 相当”结论**不成立**。
根因“待定位”——既有客户端端口耗尽，也存在明显排队，需在 Linux/WSL 重测后再归因。

## 4. 空闲内存对照

| 驱动 | 1,000 篇 | 10,000 篇 |
|---|---:|---:|
| PostgreSQL | 约 145.4 MiB（实为 1,000 篇负载后测量，原始版本误归入 10,000 篇列；需按统一空闲口径复测） | 待测（统一空闲口径） |
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

- [x] ~~运行 MySQL 全场景并回填 §3（2026-07-23 完成；列表/GraphQL 受客户端端口耗尽影响，详情/写入与 PostgreSQL 相当）~~
  **已撤销**：该结论基于错误单位与不可比数据集（S0-3/S0-4），2026-07-23 标记为失效历史数据。
- [x] 修正 §3 MySQL 延迟单位错误，标记 PostgreSQL/MySQL 横向对照为失效历史数据（S0-3/S0-4，2026-07-23）
- [ ] 报告由脚本自动从 Vegeta JSON 生成 Markdown，禁止手工抄写单位（S0-4 完成标准）
- [ ] 原始 JSON 与 Markdown 表格增加一致性校验（S0-4 完成标准）
- [ ] 三库在同一 Git SHA、同一 10,000 篇数据、同一响应字段、同一速率、同一主机条件下重跑（S0-3 完成标准，行程 C）
- [ ] 每次运行保存实际 `COUNT(*)`、Git SHA、应用配置和响应体大小（S0-3 完成标准）
- [ ] 在 Linux/WSL 受控环境对 MySQL 列表/GraphQL/并发写入复测，归因超时与排队根因（当前统一记为“待定位”）
- [ ] 运行 SQLite 全场景并回填 §3、§4
- [ ] 按统一空闲口径复测 PostgreSQL 1,000 / 10,000 篇内存
- [ ] 汇总三库对照结论，更新 ROADMAP.md Round 3 状态
