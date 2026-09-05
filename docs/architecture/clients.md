# 客户端与调用方边界

本仓库不包含浏览器、PC、H5、小程序或 App UI。`GET /v/{token}` 是 v1 唯一支持的公开预览契约：浏览器安全图片/PDF 302 到签名调用方提供的源 URL，转换格式 302 到 `aliyun-oss` 或 `silo` cache profile 的短时 PDF URL。`v1.0.0` 支持浏览器和微信小程序的 H5 `web-view` 路径；没有历史 `/preview` 或其他兼容入口。服务保证这个 HTTP 响应；不会保证未经验收的原生 App SDK 或控件自动跟随重定向。

API server 是唯一可以提交 HTTPS 源 URL 并签发 token 的调用方；管理后台服务以独立 Admin role 的 HTTPS HMAC 签名调用读取或撤销 token。所有展示层只得到 `/v/{token}`，不得得到调用方密钥，也不得持久化 302 的 `Location`。

## 调用方集成矩阵

| 调用方与文件 | v1 推荐接入 | 必要条件 | 不应假设 |
| --- | --- | --- | --- |
| 浏览器图片 | `<img src="https://preview.example/v/{token}">` | 预览服务与调用方源 URL 都为 HTTPS；最终对象有正确图片 `Content-Type` | 用脚本读取跳转 `Location`，或把跨域图片绘制到 Canvas 后读取像素 |
| 浏览器 PDF / Office | iframe、object 或新窗口直接导航到 `/v/{token}`；Office 最终得到 PDF | 最终对象返回 `application/pdf` 与 `Content-Disposition: inline`；目标浏览器自身须支持内置 PDF 查看 | 所有浏览器都有相同 PDF 查看器；不支持时调用方需提供自己的阅读器或下载入口 |
| 浏览器 JavaScript PDF 阅读器 | 阅读器加载 `/v/{token}`，但仅在源 URL 或转换缓存 profile 已对调用方 Web Origin 开放 CORS 时采用 | 最终域名允许该 Origin 的 `GET`、`HEAD` 与单个 `Range` 请求，并暴露 `Accept-Ranges`、`Content-Length`、`Content-Range` | 初始预览服务同源即可绕过最终域名的 CORS |
| 微信小程序（v1 必须） | 小程序 demo 以 fixture 附件 ID 调用 demo API，再打开本仓库 H5 `web-view`；H5 按浏览器路径加载 `/v/{token}` | demo H5、预览服务与实际最终域名按调用方式完成微信域名配置；H5 PDF 阅读器满足 CORS/Range；开发者工具和 iOS/Android 真机验收 | 直接把 `/v/{token}` 交给 `wx.previewImage`、`wx.downloadFile` 或 `wx.openDocument` 在所有版本都能跟随跨域 302 |
| 原生 App | **下一版本非目标**：届时再定义 WebView、原生下载或本地查看器路线 | 不在 v1.0.0 建立 SDK、OS 或网络策略验收承诺 | 当前微信小程序支持等同于原生 App 支持 |

调用方构造的是稳定的 token URL，例如：

```text
previewURL = https://preview.example/v/{token}
```

它可以在 token TTL 内重复打开。发生 404 时，调用方不得猜测 token 状态或重试旧签名 URL，而是根据自身业务权限决定是否向 API server 请求新的 token。

## 微信小程序 v1 接入

微信小程序的受支持路径是本仓库 `demo/preview-h5/` HTTPS H5 预览页，不是直接调用小程序文件 API。小程序 demo 预览入口必须以 fixture 附件 ID 调用本仓库 demo 详情/预览 API；该接口自行通过 HTTPS HMAC 签名且防重放的 Internal 调用向本服务签发 token，再仅返回下列展示 DTO：

```text
{ token, filename, preview_type: "image" | "pdf", expires_at }
```

小程序 demo 把 DTO 放入本仓库 H5 页面 URL 的 fragment，例如 `https://preview-h5.example/file-preview#token=...&filename=...&preview_type=pdf`，再作为 `web-view` 的 `src`。fragment 不会随 H5 初始 HTTP 请求发送；H5 读取后必须立即以 `history.replaceState` 清除它，并禁止把 token 写入日志、埋点、分享链接或存储。H5 使用 `preview_type` 选择图片元素或 PDF 阅读器，构造唯一的服务 URL `https://preview.example/v/{token}`。Office 一律按 `pdf` 展示。

H5 必须在调用期间展示 loading；404 显示“预览链接已失效”，引导用户返回业务页面重新获取；5xx 显示可重试状态且只重试 `/v/{token}`。PDF 阅读器出现 CORS 或 Range 错误时，显示明确失败反馈，不得回退为裸资源 URL 或未验证的 `wx.downloadFile` 路径。

## 域名、CORS 与内容呈现

302 会让客户端继续访问调用方源域名或转换缓存 profile 的目标域名。因此发布前必须同时检查预览服务和实际最终域名：

- 预览服务、调用方源 URL 与转换缓存 profile 均使用 HTTPS；微信小程序的 H5、预览服务和实际最终域名均按调用模式完成微信域名配置。
- 源 URL 与缓存 profile 中转换 PDF 的元数据必须给出正确 `Content-Type`；文档预览使用 `Content-Disposition: inline`，而不是强制下载。
- 仅当 JavaScript 阅读器需要读取 PDF 字节时配置最终源域名或缓存 profile CORS；只通过图片元素、iframe 或正常导航呈现时，不把 CORS 当作绕过安全模型的手段。
- CORS 白名单只列实际业务 Web Origin，允许 `GET`、`HEAD` 与单个 `Range`；不得使用带凭据的通配 Origin。
- 微信小程序的域名登记与开发者工具、真机验收是 v1.0.0 上线前置条件，不由本服务用兼容路由规避；原生 App 规则留待下一版本。

## 失败处理与边界

| 现象 | 调用方处理 |
| --- | --- |
| `/v/{token}` 返回 404 | token 已失效、撤销或不存在；按业务权限重新签发，不显示底层对象 URL。 |
| `/v/{token}` 返回 5xx | 展示“预览暂不可用”，可按调用方重试策略重试 token URL；不得改用裸资源 URL。 |
| PDF 阅读器 CORS 或 Range 失败 | 先修最终源域名或缓存 profile 的精确 CORS/对象元数据；未完成前切换为 iframe/H5 导航方案。 |
| 微信小程序 H5 加载或打开失败 | 检查仓库内 demo API、H5/预览服务/最终域名配置、HTTPS、H5 loading/失败反馈以及 PDF 阅读器的 CORS/Range；在开发者工具和真机复现后修复。 |

微信小程序 demo API、H5 页面和 fixture 是本仓库 v1.0.0 交付的一部分，固定在 `demo/` 下；不得创建或依赖外部仓库实现。生产调用方后续自行参考 demo，但不改变本项目的测试边界。
