# 微信小程序 demo 与测试环境

v1.0.0 的微信小程序验收在本仓库内闭环，不依赖任何其他项目、仓库、Issue 或生产调用方。生产调用方可以参考此 demo，但不构成 v1 交付前置条件。

## 固定目录

```text
demo/
  wechat-miniprogram/  微信小程序 demo：fixture 附件列表与 web-view 入口
  preview-h5/          H5 demo：图片/PDF 渲染、loading、失效与错误反馈
  fixtures/            已知内容 hash 的图片、PDF 和格式矩阵 fixture
```

demo API 由本仓库测试环境中的独立测试适配层提供，生产六条服务路由不注册它；测试环境以 fixture 白名单和测试访问控制限制调用方，只读仓库自有 fixture，不允许提交任意 URL。它对服务发起真正的 Internal 签名请求，不绕过服务鉴权：它只接受 demo fixture 附件 ID，完成签名调用后向小程序返回 `{token, filename, preview_type, expires_at}`。H5 接收 fragment DTO 后清除 fragment，再访问 `/v/{token}`；任何 token、调用方密钥、生产对象 URL 或生产凭据不得进入 fixture、demo 源码或日志。

## 环境与验收

- 启动环境提供本地/测试 Valkey、Gotenberg、`aliyun-oss` 或 `silo` test profile、demo API 和 H5 demo；所有配置与产物位于本项目根目录内。
- 微信开发者工具和 iOS/Android 真机使用本仓库 demo：验证图片、PDF/Office、失效 404、转换等待、CORS/Range、fragment 清理和重试反馈。
- CI 覆盖服务 API、两个 storage profile 与 fixture 格式矩阵；开发者工具/真机验收记录附于本仓库的交付 Issue/PR。
- 不得用外部仓库的页面、数据、凭据、Issue、CI 或部署来代替任一验收步骤。

## 设计与实现证据分开

[原型索引](../design/index.html) 下的小程序与 H5 入口仅演示产品交互，使用合成展示数据，不连接真实服务，不构成 demo 已实现或平台兼容证据。真实 `demo/` 代码、真实 API 鉴权、fixture、开发者工具及真机验证仍属于 #1。没有额外的服务管理后台 UI；Admin 是服务端集成 API。
