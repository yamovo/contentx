# Show HN 发布材料（ContentX）

> 状态：草稿，待发布人审定。发布动作由项目所有者在 Hacker News 执行。
> 本文档不随版本发布分发，仅作发布准备材料。

## 1. 标题（选一，≤80 字符）

1. `Show HN: ContentX – An AI-native headless CMS in a single Go binary`（推荐）
2. `Show HN: I built a headless CMS that AI agents can use directly over MCP`
3. `Show HN: ContentX – Headless CMS with a built-in MCP server for AI agents`

推荐理由：#1 同时打中 "AI-native"、"headless CMS"、"single Go binary" 三个 HN 高共鸣关键词，且不夸张。

## 2. 正文（发帖 text，英文）

```text
Hi HN! I've been building ContentX, an API-first headless CMS written in Go,
and recently made it speak the Model Context Protocol natively — meaning AI
agents (Claude Desktop, MCP-enabled IDEs, etc.) can search, read, draft and
translate content in it directly, without any glue code.

What it does:

- Standard headless CMS: REST + read-only GraphQL, article workflow
  (draft → review → scheduled → published → archived), custom content types,
  i18n, media (local/S3), built-in BM25 search with CJK support, webhooks
  with HMAC + retries, RBAC.
- MCP server built in: run `contentx --mcp` for local stdio, or enable a
  Streamable HTTP endpoint (/api/v1/mcp) authenticated by API tokens.
  4 read tools, 3 permission-gated write tools, resources
  (contentx://articles/{id}), and 4 prompt templates (write / improve /
  summarize / translate) that orchestrate those tools.
- Safety model I settled on: AI output is ALWAYS saved as a draft. Publishing
  is a separate tool behind a separate token permission. Read tools only
  expose published content by default. stdio sessions are read-only.
- Single binary, embeds the Vue 3 admin UI. PostgreSQL / MySQL / SQLite.
  Prometheus + OpenTelemetry built in.

Some honest numbers from my dev machine (not a benchmark claim): article
list P50 ~8ms at 1,000 req/s on PostgreSQL with 10k articles; reproducible
scripts and raw Vegeta output are in the repo.

Why: most CMSes are getting AI bolted on as a chat sidebar. I wanted the
opposite — the CMS as a well-behaved tool surface that any agent can drive,
with the permission model doing the guardrailing instead of prompt hopes.

Stack: Go 1.25 / Gin / GORM, official modelcontextprotocol/go-sdk,
Vue 3 + Element Plus admin. MIT licensed.

Repo: https://github.com/yamovo/contentx

Would love feedback, especially on the MCP write-permission model and
what other tools/prompts would be useful for agent-driven content work.
```

## 3. 首评预置（发帖后立刻自评，补技术细节）

```text
A few implementation notes for the curious:

- MCP transport: official Go SDK's Streamable HTTP handler mounted inside
  the existing Gin router, so the MCP endpoint shares the API's rate
  limiting and middleware. Identity flows from the HTTP Authorization
  header into tool handlers via the request's Extra headers — no context
  smuggling.
- Write authorization is the API token's own permission list
  (articles.create / edit / edit_all / publish, "*" wildcard), checked
  per tool call, and every write executes as the token's owner, so the
  audit trail is the same as human edits.
- Prompts are pure server-side templates (prompts/get) that instruct the
  agent to search for duplicates first, draft in Markdown, save via
  create_article, and never publish. If the session has no write tools
  (stdio), the same prompt degrades to presenting the draft in chat.
- Protocol behavior is tested over the SDK's in-memory transport
  (tools/list, tools/call, resources/read, prompts/get round trips), not
  just unit-tested handlers.

Known limitations I'm aware of: GraphQL is read-only (mutations via REST),
the S3 driver is a simplified signer (swap-in for aws-sdk planned), search
index is per-instance, and the plugin system is compile-time only. Roadmap
and an honest external audit live in /docs.
```

## 4. 高频问题预案（FAQ）

| 可能的问题 | 回答要点 |
|---|---|
| "Why not just give the agent your REST API + OpenAPI?" | 可以，但 MCP 给了统一的发现/调用/资源/提示词语义，客户端零胶水；且 prompts 把安全工作流（查重→草稿→人审→发布）固化在服务端而不是靠客户端提示词自觉。 |
| "AI writing slop will flood the CMS" | 产出一律草稿 + 发布独立权限 + 以 token 所属用户身份写入（同人类编辑同一审计轨迹），治理权在人。 |
| "Yet another headless CMS?" | 同意赛道拥挤；差异点是 Go 单二进制（低资源）+ MCP 原生 + 诚实的可复现基准和外部审计文档。不装作取代 Strapi。 |
| "Benchmarks look cherry-picked" | 直接承认是开发机阶段性数字非 SLA，repo 内含可复现脚本、原始 Vegeta JSON、以及 MySQL/SQLite 表现更差的完整对照（含根因 EXPLAIN 归因）。诚实是卖点。 |
| "Is the MCP endpoint safe to expose?" | 默认关闭（opt-in），强制 API token 鉴权，读默认 published-only，写按细粒度权限，共享全局限流。仍建议内网/网关后部署。 |
| "License / sustainability?" | MIT，目前个人项目，商业化路线（多租户/配额/计费）在 PRD 中列为 P3-B，未开始。 |

## 5. 发布前检查单（发布人执行）

- [ ] GitHub 仓库设为 public，确认 LICENSE（MIT）在根目录
- [ ] 仓库加 topics：`headless-cms`、`mcp`、`model-context-protocol`、`golang`、`ai-native`、`cms`
- [ ] README 顶部补一段英文 TL;DR（HN 读者第一屏是英文环境；中文正文可保留）
- [ ] 确认 `git push` 已完成、CI 全绿徽章正常
- [ ] 确认 README 中 repo URL、Swagger、快速开始命令在干净机器上可跑通
- [ ] 向 MCP 官方 servers 列表（modelcontextprotocol/servers）提交收录 PR（可发帖后做）
- [ ] 发布时间：美东工作日早上 8-10 点（北京时间 20:00-22:00），避开周五/周末
- [ ] 发帖后 2-3 小时内守评论区，用首评补技术细节，逐条回复不辩护、只补事实

## 6. 发布后动作

- 若进首页：将讨论中的高频改进点整理进 ROADMAP；24h 内出一条 "thanks + what I learned" 评论
- 若未进首页：不重发同标题；隔 2-4 周换角度（如 "the MCP write-permission model" 技术文）再试
- 同步渠道（错峰 1-2 天）：r/selfhosted、r/golang、V2EX、Lobsters
