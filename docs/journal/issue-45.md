# Issue #45：SHW 插件升级全量对齐

## 用户裁决
- 项目模式：`managed`。
- 服务集成（TLS API/Valkey/OSS）与 Chromium 浏览器导航迁出 PR required 门禁，改为非阻断 `regression-report.yml`；v1.0.0 以其真实结果为交付证据，不再阻断合并。

## 审计矩阵
| 项 | 现状证据 | 目标 | 动作 | 裁决 |
|---|---|---|---|---|
| 模式声明 | 无 `.shw/project.yaml` | 有效模式 | 新增 `mode: managed` | 用户选择 |
| `.shw-workflow-ignore` / `spec(s)/` | 不存在 | — | 无 | — |
| 集成/浏览器测试位于 PR Gate | `heavy`、`browser` 为阻断 job | 非阻断回归报告 | 拆到 `regression-report.yml`（ci-heavy）+ 汇总工件 | 用户选择迁出 |
| runner 标签 | 全部 `ubuntu-latest` | ci-fast/ci-heavy | pr-gate、image→ci-fast；回归→ci-heavy | 机械修复 |
| 公网服务镜像 | `valkey/valkey:8` | ci-cache digest | 改为 `reg.shw.top/ci-cache/valkey/valkey@sha256:e55eb7…` | 机械修复 |
| worktree 规范 | AGENTS.md 要求 `.worktrees/<n>/` | 插件 worktree MCP | 改写规范；旧目录不自动处理 | 按 #84 既有裁决 |
| 项目外只读/清理同步 | 仅一句根目录约束 | 完整边界 + #171 收口 | 改写 | 机械修复 |
| 运行配置 Secret | deployment.md 写“引用 Secret” | 全环境 ConfigMap | 改写；deploy/ 无 Secret 资源 | 机械修复 |
| Fleet 分支/发布器 | 未声明 | fleet/* + publisher | 写入规范；目标未登记，不编造 | 延期至 #26 |
| 回归基线 | `tests/vrt`、`e2e` 仅 .gitkeep | 无基线则不建 manifest | 无改动，记录无已接受基线 | — |
| 端口登记/Lucide/OpenDesign/rdev | 未发现引用 | — | 无 | — |
| 小程序上传 | 工程已取消，仅静态原型目录 | — | 无上传器，不适用 | — |
| 公共库 submodule/replace | go.mod 无 replace | — | 无 | — |

## 未完成 / 需用户处理
- **分支保护 required contexts**：Agent 无权限读取。需在 Gitea Web UI 核对 `dev`/`main` 保护规则，移除 `重型层（API/E2E/VRT）` 与 `S02 Chromium 图片与 PDF 跨域读取` 旧 context（若已设为 required），否则 PR 会永久 pending。
- `image.yml` 仍用 `CI_HARBOR_*` Secret 名；公共 `HARBOR_*` 变量是否已由管理员提供未核实，暂不切换以免断开 CI。
- Fleet 目标、GitOps publisher 接入随 #26 交付。
