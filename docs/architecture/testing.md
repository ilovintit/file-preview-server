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
3. H5/WebView：目标 iOS/Android 版本允许预览域名与 silo 域名的跳转，并对 PDF 呈现或阅读器回退路径验收。
4. 小程序或原生 App：在目标真机、SDK 与版本上验证所选 `web-view` 或原生下载模式；检查域名白名单、HTTPS、302、最终文件类型和失效处理。

未通过上述对应验收的容器不得宣称支持；调用方应采用已验证的 H5/WebView 或下载查看器路径，而不是依赖未记录的 SDK 默认行为。

所有 lint、单元、集成/API、E2E 和 VRT 结论以 PR CI 中当前 head 的 job 为准；本地只允许构建/类型级确认。PR Gate 使用每个 run 独立的 Valkey service，不能把 CI 指向共享的长寿缓存实例。
