# 文件预览服务产品真相

> 当前状态：`v1.0.0` 产品基线审查中（[Issue #7](https://git.shw.top/shw-project/file-preview-server/issues/7)）；功能实现仍由 [Issue #1](https://git.shw.top/shw-project/file-preview-server/issues/1) 交付，尚未进入 `dev`。本文不将目标架构表述为已上线能力。

## 用户、问题与价值

本服务供项目内 API server 和管理后台使用，统一处理受控文件预览。它把 Office 文档转换为 PDF、对 PDF/图片执行既有处理能力，并将可访问资源包装为可撤销、会过期的预览链接。`v1.0.0` 支持浏览器和**微信小程序**：小程序通过调用方自有 H5 预览页的 `web-view` 打开资源；本服务不提供或托管该 H5 页面。原生 App 的支持验证明确延期到下一版本。

它解决以下已确认问题：源服务端点裸露、并发转换锁只限单实例、没有预览链接生命周期管理、Gotenberg 没有随服务部署，以及调用方存在错误的 `/preview/preview` 拼接路径。

## 目标与非目标

### 本期目标（Issue #1 checklist 1–7；调用方契约由 [Issue #5](https://git.shw.top/shw-project/file-preview-server/issues/5) 和 [Issue #7](https://git.shw.top/shw-project/file-preview-server/issues/7) 定义）

- 从 `xlzb-project` 的 `src/document-processing/` 抽出可复用的独立服务。
- 为文件资源签发、查询和撤销预览 token；资源不直接由业务前端裸露。
- 通过 `GET /v/{token}` 提供唯一的、可导航的跨端预览入口；图片、PDF 与转换后的 Office PDF 均以 302 跳转到短时签名资源 URL。调用方持久化和再次打开的只能是 `/v/{token}`，不得保存或复用跳转目标 URL。
- 微信小程序是 v1.0.0 必须支持的调用方：它通过调用方 H5 预览页和 `web-view` 使用 `/v/{token}`；图片经 H5 图片元素展示，PDF/Office 经 H5 PDF 阅读器展示。
- 保留既有 PDF 脱敏、图片处理和历史预览能力，并以分布式转换锁支持多实例。
- 交付文档处理镜像及同 Pod 的 Gotenberg sidecar 部署声明。

### 明确非目标

- 不在本仓库修改调用方前端路径、`xlzb-project` 的旧目录/CI/deploy。微信小程序所需的 B 端详情/预览接口和 H5 页面不在本仓库实现，但它们必须由 `xlzb-project` 的独立 v1.0.0 Issue 交付，不能延后为本服务可用后的附带工作。
- 本服务不保存业务 SQL 数据；不引入 PostgreSQL 作为本服务的数据源。
- 不把 Gotenberg 做成独立 Deployment，也不把它与服务二进制打进同一镜像。
- 原生 App WebView、原生下载和本地查看器的 SDK/容器验证不属于 v1.0.0，下一版本再定义支持范围与验收。

## 角色与权限

| 角色 | 能力 | 认证边界 |
| --- | --- | --- |
| API server（内部调用方） | 提交资源信息并签发 token | 独立的 `INTERNAL_TOKEN` Bearer 密钥 |
| 管理后台调用方 | 列出活跃 token、撤销 token | 独立的 `ADMIN_TOKEN` Bearer 密钥 |
| 浏览器或调用方 H5 | 将 `/v/{token}` 交给图片元素、PDF 容器或导航容器 | 只持有 token；不能调用内部或管理 API |
| 微信小程序 | 以附件 ID 调用自身业务的详情/预览接口，打开自有 H5 `web-view` | 只接收 token、文件名和展示类型；不得取得资源 URL、内部或管理密钥 |

内部与管理密钥未配置时，对应端点必须拒绝请求（503），不得降级为匿名访问。

## 业务规则

- 签发资源必须提供 `url`、带支持扩展名的 `filename` 和 `ttl`；TTL 没有默认值，允许范围为 60–86400 秒。
- token 使用服务端状态而非 JWT：它是加密随机的 128-bit hex 值，在 TTL 内可重复使用；无效、过期或已撤销时统一返回 404，避免泄露存在性。
- `/v/{token}` 是导航 URL，不是返回文件字节、JSON 或供客户端读取 `Location` 的 API。它必须以 302 返回 HTTPS 的短时签名目标 URL；目标响应须保留正确 `Content-Type`，并以内联方式呈现。
- 图片和 PDF token 解析后重定向至签名资源 URL；Office 文件先检查转换缓存，未命中时加锁转换、上传缓存，再重定向。签名目标 URL 的有效期不得超过对应 token 的剩余有效期。
- 撤销或过期会阻止后续访问 `/v/{token}`；已被浏览器或客户端取得的签名目标 URL 可访问至其自身过期。这是短时签名 URL 的边界，调用方不得把它当作可撤销的业务链接。
- 微信小程序打开预览时，必须以附件 ID 请求调用方的详情/预览接口，不得把列表行或裸资源 URL 直接带入页面。该调用方接口负责用内部密钥签发 token，并仅返回 `token`、`filename`、`preview_type`（`image` 或 `pdf`）和 `expires_at`；Office 的展示类型为 `pdf`。
- 小程序只通过 `web-view` 加载调用方自有 HTTPS H5 预览页。调用方将 token 等展示信息放入 H5 URL fragment，H5 读取后立即清除 fragment，再以 `/v/{token}` 加载图片或 PDF 阅读器；不得把 token 写入 query、日志、埋点或分享参数。`wx.previewImage`、`wx.downloadFile` 和 `wx.openDocument` 不构成 v1.0.0 的预览契约。
- 管理端列表只展示仍有效的 token；撤销幂等，即使 token 不存在也返回成功。
- 历史 `/preview` 可在调用方迁移期间保留；启用 `DISABLE_LEGACY_PREVIEW=1` 后必须返回 404。`/v/{token}` 是后续统一入口。

## 关键流程与异常

1. API server 带内部 Bearer 密钥提交资源 URL、文件名和 TTL，取得 token 与过期时间。
2. 浏览器调用方直接使用 `/v/{token}`；微信小程序则以附件 ID 调自身详情/预览接口，获得 token、文件名、展示类型和过期时间。
3. 小程序打开自有 H5 `web-view`；H5 从 URL fragment 读取展示信息并立即清除，图片使用图片元素，PDF/Office 使用 PDF 阅读器。
4. H5 或浏览器访问 `/v/{token}`；服务按文件类型直接签名重定向或执行 Office→PDF 转换并重定向。
5. 管理后台可列出现存 token 并随时撤销；访问被撤销或自然过期的链接得到 404。
6. 缺失/错误内部或管理密钥为 401；参数或不支持格式为 422；未配置对应密钥为 503；基础设施故障为 5xx。

## 可验收结果

- token 签发、必填 TTL、内部认证、可复用与失效 404 均有 CI API 用例。
- 图片/PDF 直接重定向、Office 转换、缓存和转换竞争锁均有 CI 覆盖。
- 管理端列出、撤销、过期懒清理及管理认证均有 CI 覆盖。
- 302 响应不缓存、不泄露 token 至跳转目标；微信小程序的详情/预览接口、H5 `web-view`、图片、PDF/Office、失效反馈、域名/CORS 配置均有开发者工具和真机验收证据。原生 App 不在 v1.0.0 验收范围。
- 构建镜像和双容器 Deployment 声明进入同一交付 PR；真实部署、Fleet 同步和运行健康度由发布/部署流程独立取证。

实现目录、端点字段、数据键和部署契约见 [architecture index](../architecture/index.md)。
