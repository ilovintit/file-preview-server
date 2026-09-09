# file-preview-server

独立文件预览服务：HTTPS 上签名保护的 token 签发、Office/图片→PDF 或浏览器预览、内容 hash 转换缓存、可替换对象存储适配与 Valkey 生命周期管理。

## 作用域隔离（绝对约束）

- **项目根目录**：`/Users/ilovintit/Documents/Workspace/Projects/file-preview-server`。禁止在此目录外创建、修改、删除、重命名或格式化任何文件；项目外路径仅可在确有必要时只读。
- 不得创建、修改或依赖任何其他仓库的代码、文档、CI、部署声明或 Gitea 状态。本项目的 Issue、PR 和分支不构成操作外部仓库的授权。
- 所有 Issue worktree、临时产物、测试环境和 demo 必须位于项目根目录内；Git worktree 固定放在 `.worktrees/<issue-number>/`，不得使用同级或外部目录。
- H5/PC Web demo 与测试 fixture 必须由本仓库维护；不得将 v1.0.0 验收前置到其他项目。

<!-- shw-workflow:v7 -->
## 工作流（Issue 驱动）

- **真相源**：`docs/prd/product.md` 是唯一产品级当前真相；`docs/design/index.html` 索引全部适用交互入口与跨入口旅程；`docs/architecture/index.md` 索引服务、领域、数据、接口、安全、测试与部署文档。Issue 只承载摘要、状态、验收和链接，不复制文档正文。
- **生命周期命令**：只使用 `/init`、`/update`、`/explore`、`/product`、`/prototype`、`/architecture`、`/product-review`、`/version`、`/split`、`/test-plan`、`/roadmap`、`/work`、`/bug`、`/release`、`/deploy`、`/version-close`。`/explore` 是免 Issue 的纯读调查；首次接入与插件升级统一用 `/update`。不保留任何旧入口、别名、change 草稿或兼容 wrapper。
- **Issue 与 worktree 闸门**：除 `/init` 引导期和纯读 `/explore` 外，文件、CI、部署声明或 Gitea 状态的变更必须先关联一个 Issue；随后从最新 `dev` 在一个独立编号分支/worktree 中工作。一个 Issue 对应一个分支和一个 PR，依赖先进入 `dev`，禁止堆叠 PR 或 feature 分支互相合并。
- **分支与 PR**：`dev` 是唯一开发主线；`main` 永远可发布，只接受 `dev→main` 放行 PR，且只由用户在 Web UI 合并。普通 PR 目标为 `dev`，正文含 `Closes #N`；hotfix 从 `main` 切出、合回后再同步 `dev`。`test` 仅在实际被创建为集成部署源时使用。
- **版本推进**：产品基线由 `/product`、`/prototype`、`/architecture` 与 `/product-review` 收敛；`/version` 选择可发布范围，`/split` 和 `/test-plan` 建立独立验收 Issue 与映射，`/roadmap` 严格串行推进，`/work` 交付一个 Issue。Bug 由 `/bug` 建单；`/release` 只创建放行 PR，`/deploy` 只读观测，`/version-close` 按该版本已确认完成口径关闭 Milestone；v1.0.0 适用下方首版裁决，不要求人工生产观察。
- **验证**：本地只做构建/类型级确认；lint、单元、集成/API、E2E、VRT 和安全检查都由 PR CI 对当前 head 取证。没有新鲜命令输出或对应 PR job 成功，不得宣称通过。CI 失败按证据排查；`/roadmap` 可等到 CI 绿后合入 `dev`，其余情形遵循当前交付命令和用户已确认的版本验收边界；v1.0.0 不追加人工真实验收。
- **Gitea 状态**：使用 `backlog`、`status/方案中`、`status/待开发`、`status/开发中`、`status/待评审` 和 closed 表达生命周期，任一 Issue 只保留一个状态 label。每次 Gitea 写入后重新读取验证。
- **部署边界**：项目在 `deploy/` 维护声明；Fleet 负责拉取和写入集群。项目 Agent 只经统一多集群 MCP 只读观察 Kubernetes，绝不保存 kubeconfig、Token 或执行集群写操作。
- **工作记忆**：过程性结论记录在 `docs/journal/issue-N.md`；稳定结论回写本文件或对应产品/架构真相文档。

## v1.0.0 首版验收裁决

- 用户明确本版本不做人工真实验收：以连接实际服务链路的 H5 自动化验证作为首版放行依据，交付给业务系统接入使用后再收集真实反馈。
- 不再设置人工浏览器、微信开发者工具/iOS/Android 真机、人工部署回滚或固定 24 小时生产观察前置；上述未执行项不得记为通过。首版仅交付 H5 与 PC Web，小程序工程已取消，不再作为交付或验收依赖。
- 支撑 H5 链路的自动 API、安全、双 TTL/hash、两 profile、产品允许格式、镜像/声明与自动恢复检查仍随对应 Issue 交付；当前静态原型的 jsdom 检查不能替代真实 H5 E2E。
- 本版本不再因通用工作流的人工验收/观察要求阻塞首版交付或再次向用户索要同类确认。发布/交付事实、自动化证据仍需记录；Milestone 仅在本版本实际交付完成后经版本关闭流程收口，不能现在提前关闭。
- `main` 的用户 Web UI 合并、项目外禁止写入与 Kubernetes 只读边界不变；真实业务接入反馈进入后续 Bug/需求迭代，不修改其他业务仓库来代替本项目交付。

## 永久格式范围与当前交付顺序（2026-09-09）

- 用户明确本服务只支持常见图片、PDF 和微软新旧版/金山 WPS 办公文件；其他格式永久不支持，不转入后续版本或 backlog。不能把转换器的能力清单、历史代码或 WPS 能打开的格式作为产品要求。
- 精确清单以 `docs/architecture/formats.md` 和唯一 PRD 为准；其中待用户答复的图片清单及办公第四项不能由 Agent 自行扩张或标为已确认。
- 已合入的超范围支持需要实际禁用，签发、旧 token 预览和缓存命中都执行允许集合；旧代码/旧 CI 不证明新范围已实现。Microsoft Works 的 `.wps` 样例不能当作金山 WPS 验收。
- 范围修正 #36 完成后，剩余实施按 #37（允许格式与真实 WPS）→ #25（H5/PC Web）→ #26（镜像、部署模板与接入交付）串行执行。#20–#24 和 #27 的旧计划取消；#19 只保留历史交付事实。版本与 AC/TC 当前状态以 Milestone/Issue 为准。
- 首版要交付可部署镜像，不以业务环境/Fleet 目标登记、外部项目接入或人工生产观察阻塞；正式 main 合并仍由用户 Web UI 执行。
