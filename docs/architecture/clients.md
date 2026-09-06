# 客户端与调用方边界

生产调用方 UI 不在本服务范围内；本仓库必须交付微信小程序和 H5 demo，当前 `dev` 尚无其实现。`GET /v/{token}` 是 v1 唯一支持的公开预览契约：全部文件经下载校验后进入 `aliyun-oss` 或 `silo` profile，PDF/安全图片原样存储，其他格式转为 PDF；302 只返回该 profile 的短时签名 URL。`v1.0.0` 支持浏览器和微信小程序的 H5 `web-view` 路径；没有历史 `/preview` 或其他兼容入口。服务保证这个 HTTP 响应；不会保证未经验收的原生 App SDK 或控件自动跟随重定向。

API server 是唯一可以提交 HTTPS 源 URL 并签发 token 的调用方；管理后台服务以独立 Admin role 的 HTTPS HMAC 签名调用读取或撤销 token。所有展示层只得到 `/v/{token}`，不得得到调用方密钥，也不得持久化 302 的 `Location`。

## 调用方集成矩阵

| 调用方与文件 | v1 推荐接入 | 必要条件 | 不应假设 |
| --- | --- | --- | --- |
| 浏览器图片 | `<img src="https://preview.example/v/{token}">` | 预览服务与缓存 profile 都为 HTTPS；最终对象有正确图片 `Content-Type` | 用脚本读取跳转 `Location`，或把跨域图片绘制到 Canvas 后读取像素 |
| 浏览器 PDF / Office | iframe、object 或新窗口直接导航到 `/v/{token}`；Office 最终得到 PDF | 最终对象返回 `application/pdf` 与 `Content-Disposition: inline`；目标浏览器自身须支持内置 PDF 查看 | 所有浏览器都有相同 PDF 查看器；不支持时调用方需提供自己的阅读器或下载入口 |
| 浏览器 JavaScript PDF 阅读器 | 阅读器加载 `/v/{token}`，但仅在缓存 profile 已对调用方 Web Origin 开放 CORS 时采用 | 最终域名允许该 Origin 的 `GET`、`HEAD` 与单个 `Range` 请求，并暴露 `Accept-Ranges`、`Content-Length`、`Content-Range` | 初始预览服务同源即可绕过最终域名的 CORS |
| 微信小程序（v1 必须） | 小程序 demo 以 fixture 附件 ID 调用 demo API，再打开本仓库 H5 `web-view`；H5 按浏览器路径加载 `/v/{token}` | demo H5、预览服务与实际最终域名按调用方式完成微信域名配置；H5 PDF 阅读器满足 CORS/Range；H5 自动化验证；真实微信容器/设备兼容反馈留到业务接入后 | 直接把 `/v/{token}` 交给 `wx.previewImage`、`wx.downloadFile` 或 `wx.openDocument` 在所有版本都能跟随跨域 302 |
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

302 只让客户端访问所选缓存 profile 的目标域名；调用方源域名只供服务端下载。因此发布前必须同时检查预览服务和实际最终域名：

- 预览服务、调用方源 URL 与缓存 profile 均使用 HTTPS；微信小程序的 H5、预览服务和实际最终域名均按调用模式完成微信域名配置。
- profile 中原样图片/PDF 与转换 PDF 的元数据必须给出正确 `Content-Type`；文档预览使用 `Content-Disposition: inline`，而不是强制下载。
- 仅当 JavaScript 阅读器需要读取 PDF 字节时配置最终缓存 profile CORS；只通过图片元素、iframe 或正常导航呈现时，不把 CORS 当作绕过安全模型的手段。
- 共享 OSS 测试 bucket 使用无凭据通配 CORS（`Access-Control-Allow-Origin: *`），允许 `GET`、`HEAD` 与单个 `Range`，并暴露读取所需响应头；不得对通配 Origin 启用浏览器凭据。签名 URL 的短期限与不写入业务数据的约束不变。
- v1.0.0 以 H5 自动化为验收依据，不要求微信域名登记、开发者工具或真机人工验收先完成；实际业务使用时仍需由接入方配置其域名并验证平台行为。本仓库交付接入说明，不引入兼容路由规避平台约束。

## 失败处理与边界

| 现象 | 调用方处理 |
| --- | --- |
| `/v/{token}` 返回 404 | token 已失效、撤销或不存在；按业务权限重新签发，不显示底层对象 URL。 |
| `/v/{token}` 返回 5xx | 展示“预览暂不可用”，可按调用方重试策略重试 token URL；不得改用裸资源 URL。 |
| PDF 阅读器 CORS 或 Range 失败 | 展示“文件加载失败”，提供重试与返回；修复共享 bucket 的无凭据通配 CORS、Range 与对象元数据后再验收，不把 iframe 或裸 URL 当作微信小程序替代路径。 |
| 微信小程序 H5 加载或打开失败 | 检查仓库内 demo API、H5/预览服务/最终域名配置、HTTPS、H5 loading/失败反馈以及 PDF 阅读器的 CORS/Range；先以 H5 自动化复现已覆盖路径；真实容器问题由业务接入反馈后定位修复。 |

微信小程序 demo API、H5 页面和 fixture 是本仓库 v1.0.0 交付的一部分，固定在 `demo/` 下；不得创建或依赖外部仓库实现。生产调用方后续自行参考 demo，但不改变本项目的测试边界。

## 失败状态如何到达 H5

真实 demo 将 H5 与预览服务的 `/v/` 路由部署在同一 HTTPS Origin（入口按路径转发）。图片元素的 onerror 不能直接读取 HTTP 状态；PDF 阅读器也可能只报告网络失败。因此发生渲染失败时，H5 可对原 token URL 发起一次同源 GET，使用 `redirect: "manual"`，只分类非跳转的 404、422、5xx，不读取 Location、不获取替代资源 URL。302 的 opaque-redirect 结果不提供目标签名信息，归为文件加载失败；网络/CORS/Range 失败不能误标为 token 404。依据见 [MDN Response.type](https://developer.mozilla.org/en-US/docs/Web/API/Response/type)。此同源失败分类方案是依据浏览器响应过滤规则得出的设计，需要 #1 CI 和平台验收。

404 引导返回附件列表按 ID 重新获取；422 显示内容/格式不可用并返回；5xx 和网络故障允许重试原 token URL，重试期间防重复点击。fragment 缺失/无效或展示到期时间已过时，进入失效提示而不尝试旧目标签名 URL。展示到期时间只是 UX 提示，服务端 TTL 才是授权依据。阅读页关闭时取消在途请求并清除内存展示信息；重新进入必须重新获取 DTO。

## 原型到实现的职责映射

| 原型入口 / 动作 | 客户端责任 | 服务支撑 |
| --- | --- | --- |
| 小程序附件点击 | 只传 ID；请求期间防重复，取消后忽略旧回调 | 仓库内 demo 适配层鉴权后请求 Internal 签发 |
| 图片阅读 / 缩放 | 展示容器的本地状态，不签发新 token、不向源站泄漏 HMAC | token 导航与媒体类型；所有最终资源来自所选 profile |
| PDF 翻页 / 缩放 | 阅读器加载字节与页面状态；不可用时明确报错 | 最终对象 CORS/Range、内联 PDF 与有效短链 |
| Office 等待 | 不显示虚构百分比，不调用未定义任务 API | 同步转换与有界等待，见 runtime |
| 过期 / 撤销 | 两种结果使用相同提示，不推断底层存在性 | 404 与 token 状态边界 |
| 取消 / 返回 / 页面恢复 | 释放阅读器、取消请求、清内存 DTO；重新进入按 ID 获取 | 不保证已创建 token 随页面关闭立即删除；它仍按期限失效 |

prototype 目录中的脚本、DOM 检查与合成 SVG 都不进入生产文件处理链；特别是设计插画的 SVG 不改变 formats 中“用户 SVG 需转换”的约束。真实 H5 阅读器库及版本尚未进入仓库，#1 必须基于上述能力选择并固定依赖，完成内容呈现与安全验证后再宣称可用。
