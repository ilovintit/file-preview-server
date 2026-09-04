# 安全边界

- 内部 token 签发和管理 API 使用不同的静态 Bearer 密钥；两者均只通过运行环境注入，绝不提交至 Git、Actions 日志、镜像或 Kubernetes YAML。
- 未配置某个密钥时，该端点返回 503，不能以空值、默认值或匿名权限放行。
- 最终用户 token 只授权到一个预览资源及其 TTL；它不等同于 API server 或管理后台身份。
- 无效、过期和撤销 token 一律为 404，避免向浏览器泄露 token 是否曾存在。
- `/v/{token}` 是 bearer URL。成功的 302 必须带 `Cache-Control: no-store` 和 `Referrer-Policy: no-referrer`，避免客户端缓存 token→目标 URL 映射或在请求 silo 时把 token 放入 Referer。
- 302 会把短时签名目标 URL 交给客户端；撤销只影响后续 token 解析，不能撤回已经取得的目标 URL。因此签名有效期不得超过 token 的剩余 TTL，调用方不得记录、分享或把它作为业务资源链接。
- 若调用方使用 JavaScript PDF 阅读器，silo CORS 只允许经审批的业务 Origin 读取；图片元素、iframe 和导航流程不得藉由宽松 CORS 取得不必要的读取权限。
- 图像处理的 HMAC 仍依赖 `IMAGE_SECRET`；缺失时对应端点返回 503。
- 统一 trace 与错误处理中间件记录基础设施失败，返回响应不得泄露 S3 凭据、内部 URL 或 Bearer 值。

当前仓库没有安全中间件或配置实现；实现需要在 Issue #1 中按此边界和 GoFrame 错误处理约定落地。
