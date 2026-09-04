# 客户端与调用方边界

本仓库不包含浏览器、PC、H5、小程序或 App UI。`GET /v/{token}` 是 v1 新接入唯一支持的公开预览契约：成功时返回 302 到 silo 中短时签名的图片、PDF 或转换后 PDF。历史 `/preview` 仅供迁移期兼容，不能用于新接入。服务保证这个 HTTP 响应；不会保证某个未声明的 SDK、原生控件或小程序组件一定自动跟随重定向。

API server 是唯一可以提交资源 URL 并签发 token 的调用方；管理后台服务以独立 Admin Bearer 密钥读取或撤销 token。所有展示层只得到 `/v/{token}`，不得得到内部/Admin 密钥，也不得持久化 302 的 `Location`。

## 调用方集成矩阵

| 调用方与文件 | v1 推荐接入 | 必要条件 | 不应假设 |
| --- | --- | --- | --- |
| 浏览器图片 | `<img src="https://preview.example/v/{token}">` | 预览服务与 silo 都为 HTTPS；最终对象有正确图片 `Content-Type` | 用脚本读取跳转 `Location`，或把跨域图片绘制到 Canvas 后读取像素 |
| 浏览器 PDF / Office | iframe、object 或新窗口直接导航到 `/v/{token}`；Office 最终得到 PDF | 最终对象返回 `application/pdf` 与 `Content-Disposition: inline`；目标浏览器自身须支持内置 PDF 查看 | 所有浏览器都有相同 PDF 查看器；不支持时调用方需提供自己的阅读器或下载入口 |
| 浏览器 JavaScript PDF 阅读器 | 阅读器加载 `/v/{token}`，但仅在 silo 已对调用方 Web Origin 开放 CORS 时采用 | silo 允许该 Origin 的 `GET`、`HEAD` 与单个 `Range` 请求，并暴露 `Accept-Ranges`、`Content-Length`、`Content-Range` | 初始预览服务同源即可绕过最终 silo 域名的 CORS |
| App 内 H5 / WebView | 调用方拥有一个 HTTPS H5 预览页，页面按浏览器路径加载 `/v/{token}` | WebView 导航策略允许预览服务和 silo 的 HTTPS 域名；调用方在其支持的 iOS/Android 版本真机验收 PDF 呈现 | 原生 WebView 必然内置 PDF 查看器；需要时 H5 自带阅读器并满足 CORS 条件 |
| 小程序 | 优先使用调用方自有 H5 预览页配合 `web-view`，由 H5 采用浏览器路径 | 在小程序后台按实际模式登记 H5、预览服务和最终 silo 域名；上线前真机验证跳转和 PDF 呈现 | 小程序 `image`、`downloadFile` 或 `openDocument` 对跨域 302 的跟随行为在所有版本都一致 |
| 原生 App 本地查看器 | 网络库明确启用 HTTPS 重定向跟随，下载到临时文件后交图片/PDF 本地查看器 | 网络安全策略允许预览服务和 silo 域名；应用在目标 OS、SDK 与文件类型上完成验收 | 网络库默认跟随 302，或能从跨域响应中安全读取 `Location` |

调用方构造的是稳定的 token URL，例如：

```text
previewURL = https://preview.example/v/{token}
```

它可以在 token TTL 内重复打开。发生 404 时，调用方不得猜测 token 状态或重试旧签名 URL，而是根据自身业务权限决定是否向 API server 请求新的 token。

## 域名、CORS 与内容呈现

302 会让客户端继续访问 silo 的目标域名。因此发布前必须同时检查预览服务和 silo：

- 两者均使用 HTTPS，且均处于调用方网络、WebView 和小程序/App 域名白名单允许的范围。
- silo 中源文件和转换 PDF 的元数据必须给出正确 `Content-Type`；文档预览使用 `Content-Disposition: inline`，而不是强制下载。
- 仅当 JavaScript 阅读器需要读取 PDF 字节时配置 silo CORS；只通过图片元素、iframe 或正常导航呈现时，不把 CORS 当作绕过安全模型的手段。
- CORS 白名单只列实际业务 Web Origin，允许 `GET`、`HEAD` 与单个 `Range`；不得使用带凭据的通配 Origin。
- 小程序、原生 App 和各类 WebView 的具体域名登记规则属于调用方平台配置；其真实终端验收是接入前置条件，不由本服务用兼容路由规避。

## 失败处理与边界

| 现象 | 调用方处理 |
| --- | --- |
| `/v/{token}` 返回 404 | token 已失效、撤销或不存在；按业务权限重新签发，不显示底层对象 URL。 |
| `/v/{token}` 返回 5xx | 展示“预览暂不可用”，可按调用方重试策略重试 token URL；不得改用裸资源 URL。 |
| PDF 阅读器 CORS 或 Range 失败 | 先修 silo 的精确 CORS/对象元数据；未完成前切换为 iframe/H5 导航方案。 |
| 小程序/App 跳转或打开失败 | 检查预览服务和 silo 域名白名单、HTTPS、SDK 重定向策略与目标文件类型；完成真机 POC 后才把该模式作为该业务的上线路径。 |

`/preview/preview` 路径错误、调用方切换至 `/v/{token}` 以及 B 端免登详情补字段均不属于本仓库交付；它们需要在 `xlzb-project` 中单独排期。该边界对应 PRD 的非目标，不能通过本服务的兼容路由悄悄扩大范围。
