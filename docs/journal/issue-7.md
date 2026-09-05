# Issue #7：v1.0.0 微信小程序支持边界

> 历史记录：对象存储由 Issue #9 改为适配层；本记录仅保留微信小程序接入裁决。

## 稳定结论

- 用户裁决：微信小程序必须在 v1.0.0 支持并验收；原生 App 的 WebView、下载和本地查看器验证明确延期到下一版本。
- 微信小程序的唯一受支持路径是调用方自有 HTTPS H5 `web-view`。它不依赖 `wx.previewImage`、`wx.downloadFile` 或 `wx.openDocument` 对跨域 302 的默认行为。
- 小程序预览入口只能用附件 ID 调用调用方详情/预览接口；该接口完成鉴权、签发 token，并返回最小展示 DTO。列表行和裸资源 URL 不能进入详情预览。
- token 经 H5 URL fragment 传递，H5 读取后立即清除；H5 PDF 阅读器使用对象存储 CORS/Range，图片使用 `/v/{token}`。开发者工具和 iOS/Android 真机验收是 v1.0.0 的发布前置条件。
