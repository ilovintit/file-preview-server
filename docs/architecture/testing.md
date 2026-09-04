# 测试与 CI

## 当前事实

`.gitea/workflows/pr-gate.yml` 已包含 PR Gate：无 `go.mod` 时快速层和重型层将明确记录“骨架期不适用”。这不是服务测试通过的证据。服务模块一旦出现，重型层会失败，直到对应交付 Issue 接入真实 API/E2E/VRT 命令和 Gotenberg 依赖。

## Issue #1 的 CI 验收映射

| 能力 | 目标 CI 证据 |
| --- | --- |
| GoFrame 工程与历史端点迁移 | build、gofmt、vet、unit 用例 |
| token 签发/参数/认证 | API 用例：成功、TTL 边界、无认证、非法输入 |
| `/v/{token}` | API 用例：图片/PDF 重定向、Office 转换、无效 404、可重复访问 |
| 管理 API | API 用例：列表、撤销、过期懒清理、管理认证 |
| Valkey 锁与 token 仓储 | Valkey service 下的集成/API 用例 |
| Gotenberg 与部署物 | 启动 sidecar 后的 API 用例、镜像构建和 manifest 校验 |

所有 lint、单元、集成/API、E2E 和 VRT 结论以 PR CI 中当前 head 的 job 为准；本地只允许构建/类型级确认。PR Gate 使用每个 run 独立的 Valkey service，不能把 CI 指向共享的长寿缓存实例。
