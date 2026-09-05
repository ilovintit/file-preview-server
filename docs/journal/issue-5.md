# Issue #5：调用方预览接入契约

> 历史记录：对象存储单一实现与历史兼容路线已由 Issue #9 的产品裁决取代；本记录仅保留当时的审查证据。

## 稳定结论

- `GET /v/{token}` 的产品定位是导航型 bearer URL：成功时 302 到短时签名对象，调用方持久化的始终是 token URL，不是跳转目标。
- 浏览器对正常 HTTP 导航的重定向遵循 Fetch 的默认 `follow` 模式；图片元素和 iframe/object 可采用这个路径。JavaScript PDF 阅读器会在最终对象存储域名上受到 CORS 与 Range 条件约束。
- 小程序和原生 App 的 SDK/容器并没有统一的跨域 302 行为承诺。其默认接入是调用方自有 H5/WebView；原生下载查看器只能在目标平台、SDK、域名白名单和文件类型实测后纳入某个业务的上线方案。
- 302 已将签名 URL 暴露给客户端，撤销只能阻止下一次 token 解析。因此 302 使用 `Cache-Control: no-store`、`Referrer-Policy: no-referrer`，签名 URL 不超过 token 剩余 TTL，调用方不能持久化该 URL。

## 外部依据

- WHATWG Fetch Standard：请求默认 redirect mode 为 `follow`，同时说明跨域脚本读取遵循 CORS：<https://fetch.spec.whatwg.org/>。
- Android WebView 的导航由 `WebViewClient` 控制，应用可决定是否让导航留在 WebView：<https://developer.android.com/develop/ui/views/layout/webapps/webview>。
- Apple WKWebView 通过 navigation delegate 暴露服务端重定向事件：<https://developer.apple.com/documentation/webkit/wknavigationdelegate>。
- 微信小程序的 `web-view`/对象存储接入需要业务域名与域名白名单配置；该平台约束作为调用方真机验收前置条件记录，不把第三方文档当作本服务的 SDK 行为保证：<https://cloud.tencent.cn/document/product/460/84410>。
