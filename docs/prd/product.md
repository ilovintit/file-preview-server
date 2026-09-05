# 文件预览服务产品真相

> 当前状态：`v1.0.0` 产品基线审查中（[Issue #11](https://git.shw.top/shw-project/file-preview-server/issues/11)）；功能实现仍由 [Issue #1](https://git.shw.top/shw-project/file-preview-server/issues/1) 交付，尚未进入 `dev`。本文不将目标架构表述为已上线能力。

## 用户、问题与价值

本服务供项目内 API server 和管理后台使用，统一处理受控文件预览。它通过 HTTPS 上签名、防重放的服务端调用签发 token，以文件内容 hash 缓存 Office/图片到 PDF 的转换产物，并将可访问资源包装为会过期的预览链接。`v1.0.0` 支持浏览器和**微信小程序**：小程序通过本仓库 demo H5 预览页的 `web-view` 打开资源。原生 App 的支持验证明确延期到下一版本。

它解决以下已确认问题：源服务端点裸露、并发转换锁只限单实例、没有预览链接与缓存生命周期管理、对象存储被单一实现绑定，以及 Gotenberg 没有随服务部署。`v1.0.0` 是完整重构，不保留任何历史 HTTP 兼容入口。

## 目标与非目标

### 本期目标（Issue #1 交付；调用方契约由 [Issue #5](https://git.shw.top/shw-project/file-preview-server/issues/5)、[Issue #7](https://git.shw.top/shw-project/file-preview-server/issues/7)、[Issue #9](https://git.shw.top/shw-project/file-preview-server/issues/9) 和 [Issue #11](https://git.shw.top/shw-project/file-preview-server/issues/11) 定义）

- 以全新服务替代历史 document-processing 实现；不迁移或兼容任何旧 HTTP 路由、数据键、缓存键或调用方式。
- 只有持有激活调用方密钥的服务端可以通过 HTTPS 上签名且防重放的请求签发、查询或撤销预览 token；资源不直接由业务前端裸露。
- 通过 `GET /v/{token}` 提供唯一的公开导航入口；图片、PDF 与转换后的文档均以 302 跳转到短时签名资源 URL。调用方持久化和再次打开的只能是 `/v/{token}`，不得保存或复用跳转目标 URL。
- 对象存储仅经两个已验收 profile 接入：`aliyun-oss` 与 `silo`。阿里云 OSS 是首个应用 profile，silo 是第二个 profile；其他供应商不属于 v1.0.0 范围。
- 按 [格式支持](../architecture/formats.md) 覆盖部署所固定 Gotenberg 版本能处理的最大文件集合；不以未验证的文件扩展名暗示支持。
- `content_sha256` 是转换缓存与锁的唯一资源身份；`cache_ttl` 由每次 token 签发请求显式给出，命中缓存时延长到请求允许的最长有效期；同一内容的并发预览等待首个转换完成。
- 为编排提供新的 `/livez` 和 `/readyz` 探针；它们不是历史 `/health` 兼容接口，也不属于用户预览入口。
- 微信小程序是 v1.0.0 必须支持的调用方：它通过本仓库 demo H5 预览页和 `web-view` 使用 `/v/{token}`；图片经 H5 图片元素展示，PDF/Office 经 H5 PDF 阅读器展示。
- 本仓库维护完整微信小程序测试环境与 demo：`demo/wechat-miniprogram/`、`demo/preview-h5/` 和 `demo/fixtures/`；开发者工具和真机验收不依赖任何其他项目。
- 交付文档处理镜像及同 Pod 的 Gotenberg sidecar 部署声明。

### 明确非目标

- 不保留 `/preview`、`/img/`、`/redact`、`/redact-pdf`、`/pdf-to-images`、`/images-to-pdf`、测试辅助端点或任何历史兼容开关；后续如需处理能力，必须以新的已鉴权产品契约立项。
- 不创建、修改或依赖任何外部仓库的代码、文档、CI、部署或 Gitea 状态；项目根目录外的文件禁止写入。微信小程序 demo、H5 与所有验收 fixture 均由本仓库交付。
- 本服务不保存业务 SQL 数据；不引入 PostgreSQL 作为本服务的数据源。
- 不把 Gotenberg 做成独立 Deployment，也不把它与服务二进制打进同一镜像。
- 原生 App WebView、原生下载和本地查看器的 SDK/容器验证不属于 v1.0.0，下一版本再定义支持范围与验收。

## 角色与权限

| 角色 | 能力 | 认证边界 |
| --- | --- | --- |
| API server（内部调用方） | 提交 HTTPS 源 URL、内容 hash 与 TTL 并签发 token | HTTPS（生产优先 mTLS）+ 已轮换调用方密钥的 HMAC-SHA256 签名 + nonce 防重放 |
| 管理后台调用方 | 列出活跃 token、撤销 token | 独立角色的已轮换调用方密钥；同样 HTTPS、签名且防重放 |
| 浏览器或 demo H5 | 将 `/v/{token}` 交给图片元素、PDF 容器或导航容器 | 只持有 token；不能调用内部或管理 API |
| 微信小程序 demo | 以 demo 附件 ID 调用本仓库 demo API，打开本仓库 H5 `web-view` | 只接收 token、文件名和展示类型；不得取得资源 URL、内部或管理密钥 |

内部与管理调用方密钥或 storage profile 未配置时，对应端点必须拒绝请求（503），不得降级为匿名访问。

## 业务规则

- 签发资源必须提供任意 `https` 源 `url`、缓存目的 `storage_profile`、`content_sha256`、带支持扩展名的 `filename`、`ttl` 和 `cache_ttl`；调用方签名已证明来源身份，因此服务不限制 URL 所属域名。两个 TTL 都没有默认值。`ttl` 为 60–86400 秒，`cache_ttl` 为正整数且不得超过启动环境的 `MAX_CACHE_TTL`。`MAX_CACHE_TTL` 默认 86400 秒（1 天），仅可在服务启动时通过环境变量覆盖。
- 内部与管理 API 仅接受 HTTPS（生产优先 mTLS）上的 HMAC-SHA256 签名：请求包含 `key_id`、Unix 秒级 `timestamp`、128-bit 随机 `nonce` 和对规范请求的签名；时间偏差超过 300 秒、未知/失效密钥、验签失败或 nonce 重放统一返回 401。HTTPS 负责传输加密，响应为普通 JSON；HMAC key 支持双 key 滚动，明文密钥只由运行环境注入。
- token 使用服务端状态而非 JWT：它是加密随机的 128-bit hex 值，在 TTL 内可重复使用；无效、过期或已撤销时统一返回 404，避免泄露存在性。
- `/v/{token}` 是导航 URL，不是返回文件字节、JSON 或供客户端读取 `Location` 的 API。它必须以 302 返回 HTTPS 的短时签名目标 URL；目标响应须保留正确 `Content-Type`，并以内联方式呈现。
- PDF 与浏览器安全图片直接重定向；其他 [支持格式](../architecture/formats.md)先按 `content_sha256` 查转换缓存。未命中时首个请求下载并校验内容 hash、持锁转换、上传 PDF；同 hash 的后续请求等待缓存可用，而非返回冲突。签名目标 URL 的有效期不得超过对应 token 的剩余有效期。
- `cache_ttl` 从每次签发请求计算缓存绝对过期时间；缓存命中只可把过期时间延长到 `max(当前过期时间, 当前时间 + cache_ttl)`，永不缩短。过期后缓存不可再用，物理对象由对应存储适配器的清理策略删除。
- 撤销或过期会阻止后续访问 `/v/{token}`；已被浏览器或客户端取得的签名目标 URL 可访问至其自身过期。这是短时签名 URL 的边界，调用方不得把它当作可撤销的业务链接。
- 微信小程序 demo 打开预览时，必须以 demo 附件 ID 调本仓库 demo API，不得把列表行或裸资源 URL 直接带入页面。demo API 负责用内部密钥签发 token，并仅返回 `token`、`filename`、`preview_type`（`image` 或 `pdf`）和 `expires_at`；Office 的展示类型为 `pdf`。
- 小程序 demo 只通过 `web-view` 加载本仓库 `demo/preview-h5/` 的 HTTPS H5 页面。demo 将 token 等展示信息放入 H5 URL fragment，H5 读取后立即清除 fragment，再以 `/v/{token}` 加载图片或 PDF 阅读器；不得把 token 写入 query、日志、埋点或分享参数。`wx.previewImage`、`wx.downloadFile` 和 `wx.openDocument` 不构成 v1.0.0 的预览契约。
- 管理端列表只展示仍有效的 token；撤销幂等，即使 token 不存在也返回成功。

## 关键流程与异常

1. API server 以调用方 `key_id` 在 HTTPS 上签名并提交源 URL、缓存目的 profile、内容 hash、文件名、token TTL 与缓存 TTL，取得 token 与过期时间。
2. 浏览器调用方直接使用 `/v/{token}`；微信小程序 demo 则以 fixture 附件 ID 调本仓库 demo API，获得 token、文件名、展示类型和过期时间。
3. 小程序 demo 打开本仓库 H5 `web-view`；H5 从 URL fragment 读取展示信息并立即清除，图片使用图片元素，PDF/Office 使用 PDF 阅读器。
4. H5 或浏览器访问 `/v/{token}`；服务按内容 hash 查缓存：PDF/浏览器安全图片重定向至已签名源 URL，其他格式必要时从源 URL 下载、等待或转换，并重定向至缓存 profile 的短时签名 URL。
5. 管理后台可列出现存 token 并随时撤销；访问被撤销或自然过期的链接得到 404。
6. 验签、密钥、时间窗或 nonce 失败为统一 401；参数、内容 hash 或不支持格式为 422；未配置存储 profile/密钥为 503；等待转换超时或基础设施故障为 5xx。

## 可验收结果

- HTTPS/mTLS 要求、HMAC 签名、nonce 重放、时间窗、密钥轮换、必填双 TTL、内容 hash 校验、可复用与失效 404 均有 CI API 用例。
- 阿里云 OSS 与 silo 两个 storage profile、PDF 直接重定向、全格式转换、内容 hash 缓存、缓存 TTL 和等待转换均有 CI 覆盖；其他供应商不在 v1.0.0 验收范围。
- 管理端列出、撤销、过期懒清理及签名认证均有 CI 覆盖。
- 302 响应不缓存、不泄露 token 至跳转目标；本仓库 demo 的微信小程序 API、H5 `web-view`、图片、PDF/Office、失效反馈、域名/CORS 配置均有开发者工具和真机验收证据。原生 App 不在 v1.0.0 验收范围。
- 构建镜像和双容器 Deployment 声明进入同一交付 PR；真实部署、Fleet 同步和运行健康度由发布/部署流程独立取证。

实现目录、端点字段、数据键和部署契约见 [architecture index](../architecture/index.md)。
