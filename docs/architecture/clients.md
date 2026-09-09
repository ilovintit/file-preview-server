# 客户端与调用方边界

生产业务 UI 由调用方负责；本仓库交付一套 H5/PC Web 阅读页和最小附件接入示例。`GET /v/{token}` 保持唯一公开文件导航契约：允许格式经下载校验后进入 OSS/silo，安全图片/PDF 原样存储，需转换的允许文件转 PDF，302 只返回所选 profile 的短链。实际 Web 阅读页仍待 #25，镜像与部署交付待 #26。

API server 是唯一可以提交 HTTPS 源 URL 并签发 token 的调用方；管理后台服务以独立 Admin role 的 HTTPS HMAC 签名调用读取或撤销 token。所有展示层只得到 `/v/{token}`，不得得到调用方密钥，也不得持久化 302 的 `Location`。

## 调用方集成矩阵

| 调用方与文件 | v1 推荐接入 | 必要条件 | 不应假设 |
| --- | --- | --- | --- |
| 浏览器图片 | `<img src="https://preview.example/v/{token}">` | 预览服务与缓存 profile 都为 HTTPS；最终对象有正确图片 `Content-Type` | 用脚本读取跳转 `Location`，或把跨域图片绘制到 Canvas 后读取像素 |
| 浏览器 PDF / Office | iframe、object 或新窗口直接导航到 `/v/{token}`；Office 最终得到 PDF | 最终对象返回 `application/pdf` 与 `Content-Disposition: inline`；目标浏览器自身须支持内置 PDF 查看 | 所有浏览器都有相同 PDF 查看器；不支持时调用方需提供自己的阅读器或下载入口 |
| 浏览器 JavaScript PDF 阅读器 | 阅读器加载 `/v/{token}`，但仅在缓存 profile 已对调用方 Web Origin 开放 CORS 时采用 | 最终域名允许该 Origin 的 `GET`、`HEAD` 与单个 `Range` 请求，并暴露 `Accept-Ranges`、`Content-Length`、`Content-Range` | 初始预览服务同源即可绕过最终域名的 CORS |

调用方构造的是稳定的 token URL，例如：

```text
previewURL = https://preview.example/v/{token}
```

它可以在 token TTL 内重复打开。发生 404 时，调用方不得猜测 token 状态或重试旧签名 URL，而是根据自身业务权限决定是否向 API server 请求新的 token。

## H5 / PC Web 接入

Web 附件示例按 fixture ID 调用仓库内测试适配层，由该适配层进行真实 HTTPS HMAC Internal 签发，返回最小展示 DTO。生产调用方按相同契约实现自己的业务权限与详情接口：

```text
{ token, filename, preview_type: "image" | "pdf", expires_at }
```

Web 入口把 DTO 放入阅读页 URL fragment，例如 `https://preview-h5.example/file-preview#token=...&filename=...&preview_type=pdf`。fragment 不随初始 HTTP 请求发送；阅读页读取后立即以 `history.replaceState` 清除，禁止把 token 写入日志、埋点、分享或存储。页面以 `preview_type` 选择图片容器或 PDF 阅读器，只构造 `https://preview.example/v/{token}`；办公文件按 PDF 展示。

H5 必须在调用期间展示 loading；404 显示“预览链接已失效”，引导用户返回业务页面重新获取；5xx 显示可重试状态且只重试 `/v/{token}`。PDF 阅读器出现 CORS 或 Range 错误时，显示明确失败反馈，不得回退为裸资源 URL 或不受控的其他读取路径。

## 域名、CORS 与内容呈现

302 只让客户端访问所选缓存 profile 的目标域名；调用方源域名只供服务端下载。因此发布前必须同时检查预览服务和实际最终域名：

- 预览服务、调用方源 URL 和实际最终对象域名均使用 HTTPS；Web 与 `/v/` 同源路由用于失败分类，接入方配置自身业务域名。
- profile 中原样图片/PDF 与转换 PDF 的元数据必须给出正确 `Content-Type`；文档预览使用 `Content-Disposition: inline`，而不是强制下载。
- 仅当 JavaScript 阅读器需要读取 PDF 字节时配置最终缓存 profile CORS；只通过图片元素、iframe 或正常导航呈现时，不把 CORS 当作绕过安全模型的手段。
- 共享 OSS 测试 bucket 使用无凭据通配 CORS（`Access-Control-Allow-Origin: *`），允许 `GET`、`HEAD` 与单个 `Range`，并暴露读取所需响应头；不得对通配 Origin 启用浏览器凭据。签名 URL 的短期限与不写入业务数据的约束不变。

## 失败处理与边界

| 现象 | 调用方处理 |
| --- | --- |
| `/v/{token}` 返回 404 | token 已失效、撤销或不存在；按业务权限重新签发，不显示底层对象 URL。 |
| `/v/{token}` 返回 5xx | 展示“预览暂不可用”，可按调用方重试策略重试 token URL；不得改用裸资源 URL。 |
| PDF 阅读器 CORS 或 Range 失败 | 展示“文件加载失败”，提供重试与返回；修复共享 bucket 的无凭据通配 CORS、Range 与对象元数据后再验收，不回退到源 URL。 |

Web 阅读页、最小测试适配层与允许格式 fixture 均由本仓库交付，不依赖外部仓库；当前客户端范围仅 H5 和 PC Web。

## 失败状态如何到达 H5

真实 demo 将 H5 与预览服务的 `/v/` 路由部署在同一 HTTPS Origin（入口按路径转发）。图片元素的 onerror 不能直接读取 HTTP 状态；PDF 阅读器也可能只报告网络失败。因此发生渲染失败时，H5 可对原 token URL 发起一次同源 GET，使用 `redirect: "manual"`，只分类非跳转的 404、422、5xx，不读取 Location、不获取替代资源 URL。302 的 opaque-redirect 结果不提供目标签名信息，归为文件加载失败；网络/CORS/Range 失败不能误标为 token 404。依据见 [MDN Response.type](https://developer.mozilla.org/en-US/docs/Web/API/Response/type)。此同源失败分类方案由 #25 的实际 Web CI 验证。

404 引导返回附件列表按 ID 重新获取；422 显示内容/格式不可用并返回；5xx 和网络故障允许重试原 token URL，重试期间防重复点击。fragment 缺失/无效或展示到期时间已过时，进入失效提示而不尝试旧目标签名 URL。展示到期时间只是 UX 提示，服务端 TTL 才是授权依据。阅读页关闭时取消在途请求并清除内存展示信息；重新进入必须重新获取 DTO。

## 原型到实现的职责映射

| 原型入口 / 动作 | 客户端责任 | 服务支撑 |
| --- | --- | --- |
| Web 附件点击 | 只传 ID；请求期间防重复，取消后忽略旧回调 | 仓库内 demo 适配层鉴权后请求 Internal 签发 |
| 图片阅读 / 缩放 | 展示容器的本地状态，不签发新 token、不向源站泄漏 HMAC | token 导航与媒体类型；所有最终资源来自所选 profile |
| PDF 翻页 / 缩放 | 阅读器加载字节与页面状态；不可用时明确报错 | 最终对象 CORS/Range、内联 PDF 与有效短链 |
| Office 等待 | 不显示虚构百分比，不调用未定义任务 API | 同步转换与有界等待，见 runtime |
| 过期 / 撤销 | 两种结果使用相同提示，不推断底层存在性 | 404 与 token 状态边界 |
| 取消 / 返回 / 页面恢复 | 释放阅读器、取消请求、清内存 DTO；重新进入按 ID 获取 | 不保证已创建 token 随页面关闭立即删除；它仍按期限失效 |

原型的模拟脚本、三页文案和设计 SVG 不进入生产处理链，也不增加允许上传格式。真实 PDF 阅读器的固定依赖、内容呈现和自动化由 #25 交付。
