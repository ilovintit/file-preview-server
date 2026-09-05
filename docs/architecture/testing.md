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
| 预览跳转安全契约 | API 用例：302、HTTPS 签名目标、`Cache-Control: no-store`、`Referrer-Policy: no-referrer`、签名有效期不超过 token 剩余 TTL、撤销后的后续 token 访问 404 |

## 调用方人工验收

平台容器不是本仓库 CI 可以替代的运行环境。每个业务在上线前必须为其实际采用的模式留下验收记录：

1. 浏览器图片：`<img>` 经 `/v/{token}` 跳转后正常显示，token 失效后重新访问为 404。
2. 浏览器 PDF：iframe/object 或选定的 PDF 阅读器正常展示；阅读器模式额外验证 silo CORS、Range 和文件头。
3. 微信小程序：从附件 ID 打开时调用业务详情/预览接口，而非透传列表数据或裸资源 URL；接口仅返回 token、文件名、展示类型和过期时间。
4. 微信小程序：在开发者工具与 iOS、Android 真机中验证自有 H5 `web-view` 的域名配置、fragment 清理、图片、PDF/Office、loading、404 失效反馈、5xx 重试以及 silo CORS/Range。

原生 App WebView、原生下载和本地查看器不在 v1.0.0 验收范围；下一版本定义该路径后再建立对应验收。未通过上述微信小程序验收的业务不得宣称支持，且不得用未验证的小程序文件 API 替代 H5 路径。

所有 lint、单元、集成/API、E2E 和 VRT 结论以 PR CI 中当前 head 的 job 为准；本地只允许构建/类型级确认。PR Gate 使用每个 run 独立的 Valkey service，不能把 CI 指向共享的长寿缓存实例。
