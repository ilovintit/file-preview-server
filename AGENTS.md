# file-preview-server

独立文件预览服务：签名且加密的 token 签发、Office/图片→PDF 或浏览器预览、内容 hash 转换缓存、可替换对象存储适配与 Valkey 生命周期管理。

<!-- shw-workflow:v7 -->
## 工作流（Issue 驱动）

- **真相源**：`docs/prd/product.md` 是唯一产品级当前真相；`docs/design/index.html` 索引全部适用交互入口与跨入口旅程；`docs/architecture/index.md` 索引服务、领域、数据、接口、安全、测试与部署文档。Issue 只承载摘要、状态、验收和链接，不复制文档正文。
- **生命周期命令**：只使用 `/init`、`/update`、`/explore`、`/product`、`/prototype`、`/architecture`、`/product-review`、`/version`、`/split`、`/test-plan`、`/roadmap`、`/work`、`/bug`、`/release`、`/deploy`、`/version-close`。`/explore` 是免 Issue 的纯读调查；首次接入与插件升级统一用 `/update`。不保留任何旧入口、别名、change 草稿或兼容 wrapper。
- **Issue 与 worktree 闸门**：除 `/init` 引导期和纯读 `/explore` 外，文件、CI、部署声明或 Gitea 状态的变更必须先关联一个 Issue；随后从最新 `dev` 在一个独立编号分支/worktree 中工作。一个 Issue 对应一个分支和一个 PR，依赖先进入 `dev`，禁止堆叠 PR 或 feature 分支互相合并。
- **分支与 PR**：`dev` 是唯一开发主线；`main` 永远可发布，只接受 `dev→main` 放行 PR，且只由用户在 Web UI 合并。普通 PR 目标为 `dev`，正文含 `Closes #N`；hotfix 从 `main` 切出、合回后再同步 `dev`。`test` 仅在实际被创建为集成部署源时使用。
- **版本推进**：产品基线由 `/product`、`/prototype`、`/architecture` 与 `/product-review` 收敛；`/version` 选择可发布范围，`/split` 和 `/test-plan` 建立独立验收 Issue 与映射，`/roadmap` 严格串行推进，`/work` 交付一个 Issue。Bug 由 `/bug` 建单；`/release` 只创建放行 PR，`/deploy` 只读观测，生产观察完成后才由 `/version-close` 关闭 Milestone。
- **验证**：本地只做构建/类型级确认；lint、单元、集成/API、E2E、VRT 和安全检查都由 PR CI 对当前 head 取证。没有新鲜命令输出或对应 PR job 成功，不得宣称通过。CI 失败按证据排查；`/roadmap` 可等到 CI 绿后合入 `dev`，其余情形遵循当前交付命令的人工验收边界。
- **Gitea 状态**：使用 `backlog`、`status/方案中`、`status/待开发`、`status/开发中`、`status/待评审` 和 closed 表达生命周期，任一 Issue 只保留一个状态 label。每次 Gitea 写入后重新读取验证。
- **部署边界**：项目在 `deploy/` 维护声明；Fleet 负责拉取和写入集群。项目 Agent 只经统一多集群 MCP 只读观察 Kubernetes，绝不保存 kubeconfig、Token 或执行集群写操作。
- **工作记忆**：过程性结论记录在 `docs/journal/issue-N.md`；稳定结论回写本文件或对应产品/架构真相文档。
