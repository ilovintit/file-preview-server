# Issue #13：产品基线综合审查

## 输入与作用域

- 2026-09-05 从最新 `origin/dev@52d52a0` 建立 `codex/13-product-review`，worktree 为项目内 `.worktrees/13/`。
- 已读取 #11、#1 完整 body、仓库全部 Issue/PR 概要、PRD、原型索引、全部架构子文档与历史 journal。
- `git ls-files` 证明当前 dev 只有文档、PR Gate 和 src/e2e/tests 占位；没有 Go module、真实 demo 或部署声明。旧 #1 外部 worktree 不作交付证据，也未修改。

## 审查与裁决

全量发现及闭环映射见 [综合审查](../architecture/review.md)。D1（源直跳与受控短链/hash）、D2（跨 profile 缓存）、D3（双 TTL 先后过期）已向用户提问，未收到答复；未将建议当作用户裁决。对应原始条款已加阻断标记，基线未放行。

确定性修正包括 nonce 完整时间窗、规范签名编码、中间件职责、缓存并发/清理原则、demo 与生产路由分离、错误反馈路径、双容器镜像和 Fleet 回滚边界。新增两份纯文档 HTML 原型；按附件 ID 模拟详情请求，覆盖图片/PDF、等待、404/422/5xx/网络失败与返回/重试，不传真实凭据或 URL。

## 证据与限制

- 本地 `node --check`：两份原型内联 JavaScript 均 exit 0；CI Python helper 通过 AST 语法解析。未在本地执行 lint、单元、集成/API、E2E 或 VRT。
- 新增 PR Gate 文档步骤，在 CI 检查 docs 相对文件链接与内联脚本语法；真实结果以本 Issue 对应 PR 的当前 head run 为准。
- Gotenberg 官方格式文档与 v8.34.0 路由源码已只读核查；它们不证明固定镜像所有 fixture 转换通过。
- 浏览器 URL 安全策略拒绝打开本地 HTML。未尝试绕过，也未取得渲染/交互验收证据。
- 大段 heredoc 曾遇到输入 UTF-8 解析错误，未执行写入；改为 apply_patch 创建原型，随后对实际文件做脚本语法确认。

## 恢复点

本 PR 保持草稿且不合并；等待 D1–D3，随后统一改 PRD/API/data/clients/domains/deployment/原型旅程/testing，再复审、推送并取当前 SHA CI。原型视觉审阅也未完成。功能交付留在 #1，原生 App 延期至下一版本；未放行 `/version`。

审查候选 commit 与 CI run 将回写 Issue/PR，避免文档自引用自身 commit。已接受的上一基线为 `52d52a0`（PR #12）；它不是本轮审查已通过的证明。

## prototype 续跑（2026-09-05）

用户明确执行 `/prototype`，沿用 #13、项目内 worktree 和草稿 PR #14。重新读取当前 PRD 与远端 Issue/PR，确认 dev 未新增提交。完善双入口样稿、角色矩阵与跨入口导航；附件 ID 模拟详情改为共享 fixture、脚本按入口拆分；加入取消迟到响应、失败重试、图片缩放、三页 PDF 翻页、Office 等待、过期/撤销统一提示、页面恢复失效及宽窄屏。

新增 `docs/design/acceptance.md` 记录逐入口待验收清单。原型 DOM 状态测试仅在 PR CI 运行（jsdom 26.1.0 + 锁文件），不加载外部资源、不启动浏览器、不执行真实跨页导航；不把它当作前次文件 URL 拒绝后的渲染替代路径。本地只生成依赖锁文件与语法确认，npm cache 放在本 worktree `.cache/`；Git 忽略项目内 worktree 和依赖缓存。

D1–D3 尚无用户答复，原型与 PR 继续保持待逐入口验收，不合并。新候选 SHA 与 CI 证据回写 Issue/PR。
