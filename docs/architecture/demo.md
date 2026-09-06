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
- 首版在 PR CI 的真实浏览器引擎中自动验证 H5 图片/PDF/Office、失效、等待、CORS/Range、fragment 清理和重试；不要求开发者工具或 iOS/Android 真机人工验收。
- CI 覆盖实际 H5、相关 API、安全、两个 storage profile 与完整 fixture 格式矩阵；交付后业务使用反馈回流本仓库 Issue。
- 不得用外部仓库的页面、数据、凭据、Issue、CI 或部署来代替任一验收步骤。

## 设计与实现证据分开

[原型索引](../design/index.html) 下的小程序与 H5 入口仅演示产品交互，使用合成展示数据，不连接真实服务，不构成 demo 已实现或平台兼容证据。真实 demo/H5 自动化由 #25 交付，小程序接入工程与构建/契约检查由 #27 交付；微信容器与真机验证不再是 v1.0.0 的前置。没有额外的服务管理后台 UI；Admin 是服务端集成 API。

## 测试适配层与前端映射

demo API 是本仓库维护的独立测试入口/进程或测试适配层，编排对生产协议的真实调用，不在生产 server 注册额外路由。fixture 列表只含合成 ID、文件名和显示摘要；点击后根据 ID 查询当前记录，不能把列表字段直接当详情。适配层持有测试 Internal key，H5、小程序和 fixture 文件都不持有该 key。

真实小程序负责附件选择、详情请求 loading/取消、防重复和 web-view 导航；真实 H5 负责 fragment 生命周期、图片容器、PDF 阅读器、翻页/缩放、失败分类和返回。原型的“审查场景”和“展示宽度”控件不进入真实用户界面。原型模拟定时器不代表服务器有任务进度事件，真实等待由请求生命周期驱动。

取消详情请求或返回阅读页时丢弃迟到 DTO/渲染结果；浏览器返回缓存页不能恢复已清理授权。H5 生命周期以实际浏览器自动化验证；真实 SDK 的取消语义和 web-view 返回机制待业务接入后反馈，不能把 jsdom 或 H5 通过等同于微信平台导航成功。

demo 的测试环境同时记录 fixture 内容 hash、格式、来源测试 URL 配置、使用的 profile 及 CI run 前缀；不在 fixture 中保存可访问生产对象的材料。阿里 OSS 与 silo 都要有真实适配器验证，mock 仅用于纯业务/失败分支，不能替代 provider 签名、CORS/Range 和清理语义验收。
