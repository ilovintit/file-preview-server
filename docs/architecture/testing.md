# 测试与 CI

## 当前事实

`.gitea/workflows/pr-gate.yml` 已包含 PR Gate：无 `go.mod` 时快速层和重型层将明确记录“骨架期不适用”。这不是服务测试通过的证据。服务模块一旦出现，重型层会失败，直到对应交付 Issue 接入真实 API/E2E/VRT 命令和 Gotenberg 依赖。

## Issue #1 的 CI 验收映射

| 能力 | 目标 CI 证据 |
| --- | --- |
| GoFrame 工程与无历史路由重构 | build、gofmt、vet、unit 用例；所有历史路由为 404 |
| token 签发/参数/认证 | API 用例：HTTPS/mTLS 要求、HMAC 验签、key role、时间窗、nonce 重放、密钥轮换、双 TTL、内容 hash 和非法输入 |
| `/v/{token}` | API 用例：对象存储短时重定向、PDF 直接预览、全格式转换、无效 404、可重复访问 |
| 管理 API | API 用例：HTTPS HMAC 签名下的列表、撤销、过期懒清理、角色隔离 |
| Valkey 锁、nonce 与 token 仓储 | Valkey service 下的集成/API 用例：内容 hash 锁、等待转换、nonce TTL 和缓存 TTL 延长 |
| ObjectStorage 适配器 | 阿里云 OSS 与 silo profile 用例：写、删、签名 URL、CORS/Range 元数据和生命周期 |
| Gotenberg 与格式矩阵 | 固定 Gotenberg 版本下 [formats.md](formats.md) 全部格式的转换用例、镜像构建和 manifest 校验 |
| 预览跳转安全契约 | API 用例：302、HTTPS 签名目标、`Cache-Control: no-store`、`Referrer-Policy: no-referrer`、签名有效期不超过 token 剩余 TTL、撤销后的后续 token 访问 404 |
| 运行探针 | API 用例：`/livez` 仅检查进程、`/readyz` 反映 Valkey/Gotenberg/storage profile 就绪状态 |

## 调用方人工验收

平台容器不是本仓库 CI 可以替代的运行环境。每个业务在上线前必须为其实际采用的模式留下验收记录：

1. 浏览器图片：`<img>` 经 `/v/{token}` 跳转后正常显示，token 失效后重新访问为 404。
2. 浏览器 PDF：iframe/object 或选定的 PDF 阅读器正常展示；阅读器模式额外验证对象存储 CORS、Range 和文件头。
3. 微信小程序：从本仓库 demo 附件 ID 打开时调用 demo 详情/预览 API，而非透传列表数据或裸资源 URL；接口仅返回 token、文件名、展示类型和过期时间。
4. 微信小程序：在开发者工具与 iOS、Android 真机中验证本仓库 demo H5 `web-view` 的域名配置、fragment 清理、图片、PDF/Office、loading、404 失效反馈、5xx 重试以及源 URL/缓存 profile 的 CORS/Range。

原生 App WebView、原生下载和本地查看器不在 v1.0.0 验收范围；下一版本定义该路径后再建立对应验收。未通过上述仓库内微信 demo 验收不得宣称支持，且不得用未验证的小程序文件 API 替代 H5 路径。

所有 lint、单元、集成/API、E2E 和 VRT 结论以 PR CI 中当前 head 的 job 为准；本地只允许构建/类型级确认。PR Gate 使用每个 run 独立的 Valkey service，不能把 CI 指向共享的长寿缓存实例。

## 本轮文档与原型审查

[review.md](review.md) 记录 D1–D3 待裁决项，裁决前不得把任一候选契约写成通过结论。PR Gate 的文档步骤实际运行 `.gitea/scripts/check-docs.py`，覆盖 docs 内文件链接目标和两份原型内联脚本语法；它不执行服务 API 或浏览器交互。

#1 的 CI 需补充 nonce 未来 timestamp 在首次请求 300 秒后仍不能重放、窗口边界与双 key；缓存续期/清理竞争、lease 丢失/旧 owner 发布、转换期间撤销/过期、两 profile 同 hash、上传成功索引失败，以及签名 URL 到期不晚于有效缓存。D1–D3 落文档后再固定相应断言，不对未裁决方案伪造测试通过。

真实 demo 浏览器用例应覆盖附件 ID 详情请求、重复点击和旧响应竞态、fragment 读取后清除、无 fragment 重入、图片/PDF/Office、等待、404、422、5xx 重试、不可辨识的网络/CORS 错误与返回列表。H5 的状态分类按 clients 中的同源方案验证，不能把图片 onerror 当作 HTTP 404。微信开发者工具及 iOS/Android 真机仍需本仓库人工验收记录。
