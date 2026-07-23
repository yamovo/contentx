# ContentX 压测基线报告 — PostgreSQL

> ⚠️ **数据集规模与正文长度声明有误，已修正（S0-3，2026-07-23）**
>
> 本报告原始版本声称“10,000 篇文章、每篇正文约 2,120 字符”，与实际不符：
>
> - 实际使用 `scripts/benchmark/seed_postgres.sql`，播种 **1,000 篇**文章
>  （`generate_series(1, 1000)`），**非 10,000 篇**。10,000 篇 seed 文件
>   `seed_postgres_10000.sql` 当时未被使用。
> - 正文为 `repeat('ContentX benchmark content for realistic payload size. ', 40)`，
>   单次 55 字符 × 40 = **2,200 字符**，**非 2,120 字符**。
> - 文章列表场景的响应体均值 72,740 B 来自**正文精简前**版本（列表返回完整 content），
>   不能与正文精简后的 MySQL 结果横向比较。
>
> 下文已就地修正上述声明。延迟/吞吐量数值本身对“1,000 篇、正文精简前”这一实验有效，
> 但**不可用于跨数据库对照**（见 [cross-db-comparison.md](./cross-db-comparison.md) 顶部警告）。
>
> 测试日期：2026-07-22
>
> 数据集：1,000 篇已发布文章（原始版本误写为 10,000 篇）
>
> 工具：Vegeta
>
> 原始数据：[raw/postgres/](./raw/postgres/)

## 1. 测试环境

| 项目 | 值 |
|---|---|
| 操作系统 | Windows（Docker Desktop 容器化运行） |
| 数据库 | PostgreSQL（Docker 容器） |
| 应用 | ContentX（Docker 容器，端口 18080） |
| 数据集规模 | 1,000 篇文章（原始版本误写为 10,000 篇），每篇正文 2,200 字符（原始版本误写为 2,120 字符） |
| 搜索索引 | 启动时全量预热，1,000 篇全部索引 |
| 应用镜像大小 | 约 61.3 MiB |
| 应用容器内存 | 约 145.4 MiB（含 1,000 篇索引） |
| Go 版本 | 1.25.0 |

边界说明：本测试在开发机上运行，宿主机负载、磁盘 IO 和其他进程可能影响结果。正式生产部署应在专用硬件上重测。

## 2. 测试方法

### 2.1 数据集准备

使用 `scripts/benchmark/seed_postgres.sql` 向 PostgreSQL 插入 1,000 篇文章（原始版本误写为 `seed_postgres_10000.sql` 与 10,000 篇，实际未使用 10,000 篇 seed）。每篇文章：

- 标题：`Benchmark Article N`
- 正文：`ContentX benchmark content for realistic payload size. `（含尾部空格，55 字符）重复 40 次，共 2,200 字符（原始版本误写为“重复 40 次约 2,120 字符”，实际 55 × 40 = 2,200）
- 摘要：`ContentX benchmark excerpt N`
- 状态：`published`
- 作者：admin（id=1）

### 2.2 压测命令

复现脚本：`scripts/benchmark/run-postgres.ps1`

```powershell
# 读取 .env 中的 ADMIN_PASSWORD，登录获取 token
# 对四个场景分别发起 vegeta attack

# 场景 1：文章列表（GET /api/v1/articles?page=1&page_size=20）
vegeta attack -rate=1000 -duration=15s -header="Authorization: Bearer <token>" -output=article-list.bin

# 场景 2：文章详情（GET /api/v1/articles/<id>）
vegeta attack -rate=1000 -duration=15s -header="Authorization: Bearer <token>" -output=article-detail.bin

# 场景 3：GraphQL 查询（POST /api/v1/graphql）
# body: {"query":"{ articles(page:1,pageSize:20){ total items{ id title slug excerpt } } }"}
vegeta attack -rate=1000 -duration=15s -header="Authorization: Bearer <token>" -header="Content-Type: application/json" -body=graphql-body.json -output=graphql.bin

# 场景 4：并发写入（PUT /api/v1/articles/<id>）
# body: {"title":"Concurrent benchmark update","content":"...","revision_note":"vegeta benchmark"}
vegeta attack -rate=100 -duration=10s -header="Authorization: Bearer <token>" -header="Content-Type: application/json" -body=write-body.json -output=concurrent-write.bin
```

### 2.3 采样规模

| 场景 | 目标速率 | 持续时间 | 总请求数 |
|---|---:|---:|---:|
| 文章列表 | 1,000 req/s | 15s | 15,000 |
| 文章详情 | 1,000 req/s | 15s | 15,000 |
| GraphQL 查询 | 1,000 req/s | 15s | 15,000 |
| 并发写入 | 100 req/s | 10s | 1,000 |

## 3. 测试结果

### 3.1 延迟汇总

| 场景 | 成功率 | P50 | P90 | P95 | P99 | Max | Mean |
|---|---:|---:|---:|---:|---:|---:|---:|
| 文章列表 | 100% | 5.74 ms | 284.29 ms | 351.12 ms | 1,065.34 ms | 2,334.49 ms | 101.80 ms |
| 文章详情 | 100% | 2.66 ms | 3.66 ms | 3.79 ms | 4.82 ms | 26.01 ms | 2.83 ms |
| GraphQL 查询 | 100% | 3.13 ms | 4.15 ms | 4.30 ms | 5.22 ms | 24.16 ms | 3.19 ms |
| 并发写入 | 100% | 9.04 ms | 11.09 ms | 12.04 ms | 17.57 ms | 26.44 ms | 9.37 ms |

### 3.2 吞吐量

| 场景 | 目标速率 | 实际吞吐 | 请求体均值 | 响应体均值 |
|---|---:|---:|---:|---:|
| 文章列表 | 1,000 req/s | 999.64 req/s | 0 B | 72,740 B (≈71 KB) |
| 文章详情 | 1,000 req/s | 999.83 req/s | 0 B | 3,671 B (≈3.6 KB) |
| GraphQL 查询 | 1,000 req/s | 999.90 req/s | 84 B | 2,409 B (≈2.4 KB) |
| 并发写入 | 100 req/s | 100.02 req/s | 130 B | 1,519 B (≈1.5 KB) |

> ⚠️ 文章列表的 72,740 B 响应体来自**正文精简前**版本（列表返回完整 `content` 字段）。
> 自 S0-2 修复后，列表默认省略 `content`（`?full=true` 才返回全量），响应体大幅缩小。
> 因此本行数值**不可与正文精简后的 MySQL 列表结果横向比较**（S0-3）。

### 3.3 状态码

全部场景返回 `200`，无错误。

## 4. 文章列表 P95/P99 升高原因分析

文章列表的 P95（351 ms）和 P99（1,065 ms）显著高于其他场景，原因如下：

### 4.1 响应体大小差异（主因）

| 场景 | 响应体均值 | 与列表比值 |
|---|---:|---:|
| 文章列表 | 72,740 B | 1.0× |
| 文章详情 | 3,671 B | 0.05× |
| GraphQL 查询 | 2,409 B | 0.03× |

文章列表返回 20 条完整文章（含正文、摘要、分类、标签等关联），单次响应约 71 KB。在 1,000 req/s 下，每秒需序列化约 **72 MB** 的 JSON 数据，而详情和 GraphQL 场景分别仅约 3.6 MB/s 和 2.4 MB/s。

### 4.2 GC 压力导致的长尾延迟

- 72 MB/s 的 JSON 序列化产生大量临时对象分配
- Go GC 在高分配率下频繁触发，stop-the-world 暂停导致尾部延迟飙升
- P90（284 ms）到 P99（1,065 ms）跨度达 3.7 倍，符合 GC 周期性暂停的特征
- max（2,334 ms）远超 P99，个别请求可能遭遇多轮 GC 或冷路径

### 4.3 数据库查询复杂度

- 列表查询需要分页 + COUNT + 关联加载（分类、标签、作者）
- 详情查询通过主键直接命中，可能利用缓存
- GraphQL 查询字段精简（仅 id/title/slug/excerpt），响应体更小

### 4.4 连接池竞争

在 1,000 req/s 并发下，数据库连接获取存在排队，叠加 GC 暂停会放大延迟。

### 4.5 优化方向

1. 列表接口返回精简字段（不含正文），提供 `full` 参数按需加载 —— ✅ 已实现：`GET /articles` 默认省略 `content`，`?full=true` 取全量；搜索索引仍取全文。性能复测待补。
2. 对列表查询结果增加缓存层（已有 Redis 基础设施）
3. 评估 JSON 序列化优化（如预分配 buffer、流式编码）
4. 调整 GOGC 参数降低 GC 频率或使用 GOMEMLIMIT

## 5. 内存占用

| 配置 | 应用容器内存 |
|---|---|
| 1,000 篇文章 + 全量搜索索引（原始版本误写为 10,000 篇） | 约 145.4 MiB |
| 应用镜像 | 约 61.3 MiB |

说明：内置搜索索引在启动时全量加载所有文章到内存。大数据量场景需评估流式重建或外部索引（如 MeiliSearch）。原始版本声称“10,000 篇索引”占用 145.4 MiB，实际为 1,000 篇；10,000 篇规模下的内存需按统一空闲口径复测。

## 6. 限制与边界

1. 测试在开发机 Docker 环境运行，非专用硬件，结果仅供基线参考
2. MySQL 已做过对照，但因数据集规模/响应字段不一致 + 单位错误（S0-3/S0-4），现有横向结论已标记为失效历史数据，见 [cross-db-comparison.md](./cross-db-comparison.md)
3. 未在统一空闲条件下分别测量 1,000 篇和 10,000 篇的内存占用
4. 文章列表场景的 P95/P99 受开发环境影响，生产环境需重测
5. 未测试多实例下的搜索索引一致性（内置索引不跨实例共享）
6. 本报告原始版本数据集规模与正文长度声明有误（S0-3），已就地修正为 1,000 篇 / 2,200 字符，但延迟/吞吐量数值未重测

## 7. 待完成项

- [ ] 三库（PostgreSQL/MySQL/SQLite）在同一 Git SHA、同一 10,000 篇数据、同一响应字段、同一速率、同一主机条件下重跑（S0-3 完成标准，行程 C）
- [ ] 报告由脚本自动从 Vegeta JSON 生成 Markdown，禁止手工抄写单位（S0-4 完成标准）
- [ ] 在统一空闲条件下重测 1,000/10,000 篇文章内存
- [ ] 在专用硬件或受控云环境上重测，验证 P95/P99 是否为开发环境噪声
- [x] 实现列表接口字段精简（默认省略 `content`，`?full=true` 取全量）—— 已实现并单测；性能复测待补
- [ ] 实现列表接口字段精简后复测，验证 P95/P99 改善幅度
