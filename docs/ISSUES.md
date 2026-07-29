# ContentX 已知问题与改进清单

> 本文档是全部未完成项与已知不足的唯一汇总清单（单源权威）。  
> 来源：前端重构审查、质量门禁审计、[已归档的重构方案](./archive/FRONTEND_REFACTOR_PLAN.md)遗留项、日常使用反馈  
> 更新日期：2026-07-28

## 1. 测试覆盖不足

### 1.1 整体覆盖率偏低
- 总体行覆盖率 37.3%，仅略高于 35% 阈值
- 总体函数覆盖率 38.8%，仅略高于 35% 阈值
- 大量 Vue 页面组件覆盖率为 0%（`views/` 下的文件几乎无测试）

### 1.2 未覆盖的关键模块
以下 `features/` 子模块完全无测试（0% 覆盖率）：
- `features/backups/`
- `features/menus/`
- `features/plugins/`
- `features/roles/`
- `features/seo/`
- `features/settings/`（仅 profile 有少量测试）
- `features/system/`
- `features/themes/`
- `features/tokens/`
- `features/users/`
- `features/webhooks/`

### 1.3 E2E 测试覆盖不足
当前 Playwright E2E 仅覆盖：
- 认证流程（登录/TOTP/刷新）
- 文章 CRUD（新增）
- 权限矩阵（新增）

计划中但尚未实现的 7 个关键 E2E 场景：
1. 媒体上传、选择、删除
2. 动态内容条目的主要成功与失败路径
3. 备份和 Webhook 的主要成功与失败路径
4. 角色权限影响菜单、路由和操作按钮的完整验证
5. Editor 审核、发布、定时、归档及跨作者编辑
6. Author 创建草稿、编辑本人内容并提交审核，但不能发布
7. 文章和页面 CRUD，且页面类型不会被改写

## 2. 设计系统采用率低

### 2.1 shared/ui 组件未被广泛采用
`web/src/shared/ui/` 有 10 个设计系统组件：
- AppShell, AsyncState, ConfirmAction, DataTablePage, EmptyState, FilterBar, FormSection, PageHeader, PermissionGate, StatusBadge

但仅在 3 个视图中实际采用（CategoryList, TagList, ArticleList）。其余 17+ 个视图仍直接拼装 Element Plus 组件。

### 2.2 异步状态展示不统一
计划要求所有异步页面必须明确区分：首次加载、局部刷新、空数据、权限不足、请求失败及重试、保存成功、未保存离开。但大部分视图未使用 `AsyncState` 组件。

## 3. 工程卫生

### 3.1 工作目录存量变更（已解决 2026-07-29）
存量重构变更已按主题分三笔提交（docs / backend / web），此前因未提交修改导致失败的 `http.spec.ts`、`index.spec.ts`、`auth.spec.ts` 已验证全部通过（72/72）。遗留：`tmp_diff_api.txt`、`tmp_old_api.txt` 临时文件待清理或加入 .gitignore。

### 3.2 Git 提交粒度粗
所有重构变更集中在大量文件中，缺乏按模块/阶段的原子提交。

### 3.3 种子密码与 .env 不同步（新增 2026-07-28）
`seedAdminUser` 仅在用户表为空时写入 `ADMIN_PASSWORD`，后续修改 `.env` 不会更新存量账号，导致 `.env` 中的密码与数据库实际密码不一致，排障和自动化测试时易误导。建议提供密码重置子命令或在文档中明确说明。

## 4. 依赖与构建

### 4.1 Sass Legacy API 弃用警告
构建时出现 28 个 Dart Sass 弃用警告：legacy JS API 将在 Dart Sass 2.0.0 移除。需要升级 Sass 配置。

### 4.2 DashboardCharts chunk 偏大
`DashboardCharts` chunk 为 515KB minified（175KB gzip），虽然低于 200KB gzip 门禁，但 minified 体积超过 500KB。ECharts 按需引入还有优化空间。

### 4.3 循环依赖风险
重构前存在 `stores/auth.ts` → `@/api` → `domains/auth.ts` 的循环依赖。已修复为直接导入 `@/api/domains/auth`，但需要持续监控防止新增。

## 5. 前端架构债务

### 5.1 ProfileView 迁移不完整
ProfileView 已迁移到 Vue Query（`useProfileQuery` + `useProfileMutation`），但 TOTP 相关操作（`totpApi.status/setup/enable/disable`）仍使用手动 API 调用，未完全统一。

### 5.2 部分路由未指向 pages/ 薄壳
以下路由仍直接指向 `@/views/`：
- `ContentTypeList` → `views/content/ContentTypeList.vue`
- `ContentEntryList` → `views/content/ContentEntryList.vue`
- `NotFound` → `views/shared/NotFound.vue`

（Login 保留直接引用是合理的，NotFound 和 Content 模块应补充 pages/ 薄壳）

### 5.3 barrel 重导出与 tree-shaking
`web/src/api/index.ts` 作为 barrel 重导出所有 14 个领域模块的全部内容。如果消费方未使用具名导入，可能导致 tree-shaking 不完全。

### 5.4 存量 vue-tsc 类型错误（新增 2026-07-28）
全量类型检查存在以下既有错误，未阻断构建但应清零：
- `layouts/AdminLayout.vue`：字符串未收窄为 `PermissionSlug` 类型
- `views/articles/ArticleList.vue`：置顶/精选局部更新入参不满足 mutation 类型、tag type 字符串未收窄为枚举

### 5.5 评论列表不支持 URL 参数预筛选（新增 2026-07-28）
仪表盘“待审评论”卡片跳转到 `/admin/comments` 后无法直达待审状态，需手动切换筛选。应支持 `?status=pending` 类参数。

### 5.6 国际化能力缺失（新增 2026-07-28）
项目无 i18n 框架，全站文案硬编码中文；设置页已通过前端映射统一为中文。完整中英切换需引入 vue-i18n 并抽取 40+ 文件文案，已评估为低 ROI，暂不排期，有英文用户需求时再启动。

## 6. 覆盖率阈值下调

### 6.1 阈值从 42%/85% 下调到 35%/70%
由于重构新增了大量未测试代码（32 个 composables、22 个 pages），覆盖率阈值被迫下调：

| 指标 | 原阈值 | 当前阈值 | 当前实际 |
|------|--------|----------|----------|
| Lines | 42% | 35% | 37.3% |
| Branches | 85% | 70% | 76% |
| Functions | 38% | 35% | 38.8% |
| Statements | 42% | 35% | 37.3% |

核心模块覆盖率优秀（API 86%、auth 100%、articles 98%），但整体趋势向下。

## 7. 后端待改进

### 7.1 测试执行时间长
Handler 和 Service 测试需要 5 分钟+ 才能完成，说明测试粒度或 mock 策略有优化空间。

### 7.2 Swagger 注释维护
计划要求所有已挂载路由补齐 Swagger 注释并重新生成 `docs/api`，生成结果必须和仓库零差异。尚未验证。

### 7.3 可观测性验证
Prometheus metrics、tracing 等中间件已集成但缺乏端到端验证。

## 8. 无障碍与体验

### 8.1 WCAG 2.2 AA 未验证
计划要求满足 WCAG 2.2 AA，关键流程可完全使用键盘操作，但尚未进行审计。

### 8.2 axe 扫描未执行
计划要求 axe 扫描无 Critical/Serious 问题，但尚未执行。

### 8.3 prefers-reduced-motion 未验证
计划要求动画支持 `prefers-reduced-motion`，但尚未验证 `composables/useAnime.ts` 和 `DashboardView` 中的动画。

### 8.4 浅色/暗色截图回归未执行
计划要求浅色和暗色主题均通过截图回归，但尚未执行。

## 9. 非目标记录（已闭环）

以下内容按归档的重构方案明确列为非目标，**均已记录在 `STATUS.md` 已知限制与 `ROADMAP.md` 后续候选中**：
- React 或 Next.js 重写
- 公共首页和博客的视觉重构
- 匿名评论、点赞及公共内容 REST API
- Markdown 到富文本的数据迁移
- 手机端复杂编辑器和系统管理的完整适配
- Webhook 持久化队列、S3 正式 SDK、计费、SSO 和多租户

## 10. 下一步优先级建议

结合 [STATUS.md](./STATUS.md) 的里程碑目标，建议按以下顺序推进：

| 优先级 | 事项 | 对应章节 | 理由 |
|---|---|---|---|
| ~~P0~~ | ~~提交工作区存量变更，恢复三个失败单测~~ | §3.1 | ✅ 已完成（2026-07-29，三笔主题提交） |
| P0 | 当前里程碑收口：Webhook 队列、S3 验证、统一回归 | STATUS 进行中 | 发布阻断项 |
| P1 | 存量 vue-tsc 类型错误清零 | §5.4 | 体量小、阻止类型门禁形同虚设 |
| P1 | features/ 零覆盖模块补测试，恢复 42% 门槛 | §1、§6 | 覆盖率趋势向下需尽早止跌 |
| P2 | 补齐 7 个关键 E2E 场景 | §1.3 | 核心工作流回归保障 |
| P2 | pages/ 薄壳补齐、ProfileView TOTP 统一、评论 URL 预筛选 | §5.1–5.5 | 架构一致性与体验小改 |
| P3 | Sass 升级、ECharts 包体积、无障碍审计 | §4、§8 | 不阻断发布，排期消化 |

完成任一项后在本文档删除对应条目并同步 `STATUS.md`。
