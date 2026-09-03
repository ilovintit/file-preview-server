# file-preview-server

独立文件预览服务：Office→PDF 转换（Gotenberg sidecar）、PDF 像素化脱敏、图片处理、token 鉴权预览链接管理（Redis）。

<!-- shw-workflow:v6 -->
## 工作流（Issue 驱动）

- **真相源**：`docs/prd/`（业务规则/验收标准）、`docs/design/`（交互原型）、`docs/architecture/`（技术方案与决策）。Issue 只做指针与状态，不承载文档内容。
- **流程**：需求入口 `/shw-issue` 建 Issue（backlog label），agent 侧由 `shw-issue-gate` skill 兜底（无单不开工），单入口三分支——
  - **微活**：`/shw-do <N>`（直通执行）→ `/shw-wrap <N>`（回写 + 提 PR + 盯 CI + 全绿自动合并）——单文件单点、不碰逻辑结构的改动，不进 change 族
  - **小活**：`/shw-explore` 差距调查 → `/shw-propose` 同会话固化（`.changes/` 本地草稿，不入库）→ `/shw-test-draft` → `/shw-test-review`（测试起稿+对齐审查）→ `/shw-apply`（提 PR）→ `/shw-archive`（合并 + 轨迹回写 Issue）
  - **大活**：docs 三件套（`/shw-prd-draft`→review → `/shw-proto-draft`→review（有界面时）→ `/shw-arch-draft`→review，各自 docs PR `Refs #N`）→ `/shw-milestone` 排期拆分（验收 checklist）→ test 族起稿审查（`/shw-test-draft` → `/shw-test-review`）→ `/shw-roadmap` 驱动逐 Issue 走 change 族（explore → propose → apply → archive，编排层等 CI 红绿）
  - 三层数点相同：PR（`Closes #N`，目标 dev）→ CI 门禁 → **用户人工验收 → 归档环节合并**（交互模式；roadmap/goal 编排模式 CI 全绿自动合并）；发布前 `/shw-review` 双模型对抗终审 → dev→main 放行（用户确认）
  - 微活/小活/大活定义见 `shw-gitea-flow` §6；边界模糊保守缺省走小活
- **分支（dev 主线制）**：`dev` 开发主线（一切 PR 目标，roadmap 接力地）+ `main` 发布线（永久可发布，仅用户合并）+ `test` 可选（集成部署源）；Issue:分支:PR = 1:1:1 编号进分支名（无例外，紧急修复也先建 Issue）；分支从 dev 切（hotfix/N-slug 从 main 切、合 main 后 cherry-pick 回 dev）；禁堆叠、禁分支互 merge、依赖等前序合进 dev；落后 dev 时 rebase。
- **验证分层**：本地只做编译级确认（build/typecheck）；lint 与一切测试（单元/集成/API/E2E/VRT）一律在 PR CI 执行——agent 本地不跑测试、不卡开发流程，推送后 CI 出结果，失败由用户发起排查（roadmap/goal 编排模式由 agent 等 CI 循环到绿）。
- **状态流转（label）**：需求池(`backlog`) → 方案中 → 待开发 → 开发中 → 待评审 → 完成(closed)。`待开发` = Definition of Ready。状态全用 label 过滤，不用 Project 看板。
- **工作记忆**：过程性记录（试错/被否方案/踩坑）按 `docs/journal/issue-N.md` 留痕，定期提炼进本文件与 `docs/architecture/`。
