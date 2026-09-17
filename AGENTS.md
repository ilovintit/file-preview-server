# file-preview-server

SHW 模式：`managed`（`.shw/project.yaml`）。

独立文件预览服务：HTTPS 上签名保护的 token 签发、Office/图片→PDF 或浏览器预览、内容 hash 转换缓存、可替换对象存储适配与 Valkey 生命周期管理。

## 作用域隔离（绝对约束）

- **项目外只读**：写入范围仅为已确认的当前项目工作目录；进入 Issue worktree 后以插件 worktree MCP 返回的该工作区为唯一编辑边界，主 checkout、其他 Issue 工作区及相邻仓库不因同属本项目而自动可写。用户级配置、shell profile、插件安装/缓存和共享库源码一律只读；符号链接、切换 cwd、脚本、子进程、MCP 或委派都不能绕过。需要外部修改时先停止，报告目标绝对路径、原因和拟修改内容，交用户安排；“继续”“自动修复”不等于外部写授权。有限运行例外仅限 worktree MCP 管理受管工作区、已授权 Git 操作的仓库元数据写入和 Gitea shim 后端缓存。
- 不得创建、修改或依赖任何其他仓库的代码、文档、CI、部署声明或 Gitea 状态。本项目的 Issue、PR 和分支不构成操作外部仓库的授权。
- Issue 工作区只经插件 worktree MCP `acquire/inspect/list/release/remove/reconcile` 管理，按返回的绝对路径在同一会话完成交付；不手写索引、不自动 fork/Handoff、不在同一受管目录跨 Issue 切换分支。旧 `.worktrees/` 目录只盘点归属，不自动采纳、移动或删除。临时产物、测试环境和 demo 仍须位于项目仓库内或当前工作区内。
- 合入 `dev` 后先 fetch 证明 Issue HEAD 已进入 `origin/dev`，再经 MCP remove 预检删除工作区与本地 Issue 分支，并把本地 `dev` 快进到 `origin/dev`；脏、分叉或竞态时不 reset/force，保留现场并报告。
- H5/PC Web demo 与测试 fixture 必须由本仓库维护；不得将 v1.0.0 验收前置到其他项目。

<!-- shw-workflow:v7 -->
## 工作流（Issue 驱动）

- **真相源**：`docs/prd/product.md` 是唯一产品级当前真相；`docs/design/index.html` 索引全部适用交互入口与跨入口旅程；`docs/architecture/index.md` 索引服务、领域、数据、接口、安全、测试与部署文档。Issue 只承载摘要、状态、验收和链接，不复制文档正文。
- **生命周期命令**：只使用 `/init`、`/update`、`/explore`、`/product`、`/prototype`、`/architecture`、`/product-review`、`/version`、`/split`、`/test-plan`、`/roadmap`、`/work`、`/bug`、`/release`、`/deploy`、`/version-close`。`/explore` 是免 Issue 的纯读调查；首次接入与插件升级统一用 `/update`。不保留任何旧入口、别名、change 草稿或兼容 wrapper。
- **Issue 与 worktree 闸门**：除 `/init` 引导期和纯读 `/explore` 外，文件、CI、部署声明或 Gitea 状态的变更必须先关联一个 Issue；随后从最新 `dev` 在一个独立编号分支/worktree 中工作。一个 Issue 对应一个分支和一个 PR，依赖先进入 `dev`，禁止堆叠 PR 或 feature 分支互相合并。
- **分支与 PR**：`dev` 是唯一开发主线；`main` 永远可发布，只接受 `dev→main` 放行 PR，且只由用户在 Web UI 合并。普通 PR 目标为 `dev`，正文含 `Closes #N`；hotfix 从 `main` 切出、合回后再同步 `dev`。`test` 仅在实际被创建为集成部署源时使用。
- **版本推进**：产品基线由 `/product`、`/prototype`、`/architecture` 与 `/product-review` 收敛；`/version` 选择可发布范围，`/split` 和 `/test-plan` 建立独立验收 Issue 与映射，`/roadmap` 严格串行推进，`/work` 交付一个 Issue。Bug 由 `/bug` 建单；`/release` 只创建放行 PR，`/deploy` 只读观测，`/version-close` 按该版本已确认完成口径关闭 Milestone；v1.0.0 适用下方首版裁决，不要求人工生产观察。
- **验证**：本地只做构建/类型级确认。没有新鲜命令输出或对应 CI job 结果，不得宣称通过。CI 严格分三条路径：
  1. **工程检查** `pr-gate.yml`（`ci-fast`）：文档/原型脚本、gofmt/vet、单元测试与构建；唯一 required context 来源。Issue→dev 必须全绿后由 Agent 合入 `dev`；dev→main 候选实际运行全量工程检查，main 仅用户 Web UI 合并，用户可例外覆盖，例外须记录 candidate SHA、失败项、人工验证、理由、风险、回滚与后续 Issue。
  2. **已有基线比较** `regression-report.yml`（`ci-heavy`，push dev/手动）：TLS API/Valkey/OSS 服务集成与 Chromium 双 profile 浏览器导航。非阻断、不挂 required 或发布门禁；失败、无基线、partial、执行错误如实记录，不改写为通过，不写 tests、不重录 expected、不自动提交。
  3. **基线维护**：只经 `/shw-baseline` 独立维护 Issue，绑定不可变 accepted source SHA、制品 revision、最终验收引用与范围；普通 push/PR、回归失败和发布流程无写权限。当前 `tests/vrt`、`e2e` 无已接受基线。
- **CI 依赖**：workflow 显式使用 `ci-fast`/`ci-heavy`，不用 `ubuntu-latest`；执行镜像、服务镜像固定 `reg.shw.top/ci-cache` digest，Action 使用 `git.shw.top/actions` 固定引用，Go 经 `gop.shw.top` + `GO_TOKEN` Secret；业务 Dockerfile `FROM` 不引用 `shw-ci-*` 执行镜像。
- **Gitea 状态**：使用 `backlog`、`status/方案中`、`status/待开发`、`status/开发中`、`status/待评审` 和 closed 表达生命周期，任一 Issue 只保留一个状态 label。每次 Gitea 写入后重新读取验证。
- **部署边界**：项目在 `deploy/` 维护声明；业务分支 main/test/dev 不变，Fleet 分别监听 `fleet/prod`、`fleet/test`、`fleet/dev`，由固定 GitOps publisher 从精确业务 SHA 以普通追加提交推进，不强推、不浮动镜像标签。所有环境运行配置只用 ConfigMap（`configMapKeyRef`/`configMapRef`），不得声明 Kubernetes Secret 或 `secretKeyRef`/`secretRef`/Secret volume；按名称引用基础设施预置的 `imagePullSecrets` 例外。dev 自管中间件经内部缓存 digest 声明；test/生产只部署应用并连接外部中间件。项目 Agent 只经统一多集群 MCP 只读观察 Kubernetes（Fleet 管理面与下游业务 context 分离），绝不保存 kubeconfig、Token 或执行集群写操作。当前 Fleet 目标尚未登记，未知值不编造。
- **工作记忆**：过程性结论记录在 `docs/journal/issue-N.md`；稳定结论回写本文件或对应产品/架构真相文档。

## v1.0.0 首版验收裁决

- 用户明确本版本不做人工真实验收：以连接实际服务链路的 H5 自动化验证作为首版交付证据，交付给业务系统接入使用后再收集真实反馈。
- 2026-09-17（#45）用户裁决：服务集成与浏览器链路自动化迁出 PR required 门禁，改为 `regression-report.yml` 非阻断报告；首版放行引用其对候选 SHA 的真实结果作为证据，失败或缺失如实记录，不再阻断 Issue→dev 合并，也不得记为通过。
- 不再设置人工浏览器、微信开发者工具/iOS/Android 真机、人工部署回滚或固定 24 小时生产观察前置；上述未执行项不得记为通过。首版仅交付 H5 与 PC Web，小程序工程已取消，不再作为交付或验收依赖。
- 支撑 H5 链路的自动 API、安全、双 TTL/hash、两 profile、产品允许格式、镜像/声明与自动恢复检查仍随对应 Issue 交付；当前静态原型的 jsdom 检查不能替代真实 H5 E2E。
- 本版本不再因通用工作流的人工验收/观察要求阻塞首版交付或再次向用户索要同类确认。发布/交付事实、自动化证据仍需记录；Milestone 仅在本版本实际交付完成后经版本关闭流程收口，不能现在提前关闭。
- `main` 的用户 Web UI 合并、项目外禁止写入与 Kubernetes 只读边界不变；真实业务接入反馈进入后续 Bug/需求迭代，不修改其他业务仓库来代替本项目交付。

## 永久格式范围与当前交付顺序（2026-09-09）

- 用户明确本服务只支持常见图片、PDF 和微软新旧版/金山 WPS 办公文件；其他格式永久不支持，不转入后续版本或 backlog。不能把转换器的能力清单、历史代码或 WPS 能打开的格式作为产品要求。
- 精确允许清单共 18 个后缀，以 `docs/architecture/formats.md` 和唯一 PRD 为准。用户已澄清办公是 Word/Excel/PPT 三件套，PDF 单列；图片按常见图片清单实施，禁止自动扩大。
- 已合入的超范围支持需要实际禁用，签发、旧 token 预览和缓存命中都执行允许集合；旧代码/旧 CI 不证明新范围已实现。Microsoft Works 的 `.wps` 样例不能当作金山 WPS 验收。
- 范围修正 #36 完成后，剩余实施按 #37（允许格式与真实 WPS）→ #25（H5/PC Web）→ #26（镜像、部署模板与接入交付）串行执行。#20–#24 和 #27 的旧计划取消；#19 只保留历史交付事实。版本与 AC/TC 当前状态以 Milestone/Issue 为准。
- 首版要交付可部署镜像，不以业务环境/Fleet 目标登记、外部项目接入或人工生产观察阻塞；正式 main 合并仍由用户 Web UI 执行。
