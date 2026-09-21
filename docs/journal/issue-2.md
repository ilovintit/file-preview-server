# Issue #2：v7 工作流更新审计

> 历史记录：跨仓库迁移假设已由 Issue #11 取代；v1.0.0 的 demo 与验收均在本仓库内闭环。

审计日期：2026-09-04。证据以远端 `dev`、远端 Issue/分支/Actions 和本地 worktree 为准。

| 范围 | 现状证据 | v7 要求 | 处理 | 状态 |
| --- | --- | --- | --- | --- |
| Agent 工作流 | `AGENTS.md` 含旧版本标记、旧 change 流程和旧 slash 命令族 | 仅 16 个生命周期命令，无旧 alias | 全段重写为 v7，保留服务说明 | 已修复 |
| 产品真相 | 仅 `docs/prd/README.md`；有效目标散落 Issue #1 和本地旧草稿 | 唯一 `docs/prd/product.md` | 把已确认范围、规则、角色、流程与验收迁入 PRD，并标明未实现 | 已修复 |
| 原型 | 没有 UI 源代码或入口 | 有产品级索引；无 UI 时说明不适用 | 新建 `docs/design/index.html`，记录外部调用方边界与跨入口流程 | 已修复 |
| 架构 | 只有 `.gitkeep` | index 覆盖 clients/services/domains/data/apis/security/testing/deployment | 新建架构文档族；目标/现状严格分离 | 已修复 |
| 退役流程 | `.gitignore` 忽略退役的本地 change 草稿，AGENTS 引用旧草稿 | 迁移有效知识并移除退役说明 | 知识已迁入；移除忽略规则和旧命令说明 | 已修复 |
| 代码现实 | 远端 `dev` 仅骨架；Issue #1 worktree 有未提交实现 | 不能把草稿或未提交工作称为交付 | 文档明确为目标；实现仍由 #1 独立交付 | 已记录 |
| 公司组件契约 | Issue #1 未提交 `go.mod` 已有 GoFrame v2，但尚无 gf-lib 直接依赖；历史设计要求按 gf-lib 组件规范落地 | 已确认的组件契约必须在实现中实际依赖并按库文档落地 | 不在 #2 偷改 #1 的未提交代码；#1 提 PR 前必须补齐或重新裁决该差距 | 延期至 #1 |
| 分支与 PR | `dev` protected；`main` reported unprotected；无 `test` 分支 | dev 开发主线；main 受保护且只由用户合并 | PR Gate 不再监听不存在的 `test`；保护修复建 #3 | 延期至 #3 |
| labels/Milestone/PR | labels 已含 v7 生命周期状态；无 Milestone、无 PR | Issue 状态与 PR 纪律 | #2 使用 chore + 开发中；无需为未排期功能伪建 Milestone | 已对齐 |
| Actions | Gate 用浮动 major action tag；重型 job 是成功的 TODO 占位 | 固定 Action 版本，适用测试真实取证 | 固定版本；骨架期显式不适用，服务模块出现但未配置真实重型测试时强制失败 | 已修复 |
| 测试 | 无 Go module/可运行服务；PR Gate 骨架在 | PR CI 覆盖适用层级 | 映射写入 `testing.md`；真实 API/E2E/VRT 是 #1 交付项 | 延期至 #1 |
| 部署/Fleet | 无 Dockerfile、`deploy/`、Harbor 制品、Fleet target 或集群目标 | 项目声明 + Fleet 拉取，Agent 只读观测 | 目标和缺口写入 `deployment.md`；不编造 target | 延期至 #1 |
| 服务端口 | 旧本地草稿写 3001，未登记全局端口表 | 新服务先查表、登记再使用 | 9501 在表中空闲，已登记为本服务 HTTP API；目标文档同步为 9501 | 已修复 |

## 不可由本 Issue 写入的事项

- [Issue #3](https://git.shw.top/shw-project/file-preview-server/issues/3)：需要仓库管理员保护 `main` 并复核 required checks/Actions 设置。
- [Issue #1](https://git.shw.top/shw-project/file-preview-server/issues/1)：实现服务、实际测试、Dockerfile、项目部署声明、Harbor、Fleet 目标以及本仓库内的 demo/测试环境。

没有删除 Issue #1 worktree 的本地草稿或未提交代码；它不属于 Issue #2，且还承载待交付实现。
