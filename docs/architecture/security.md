# 安全边界

- 内部 token 签发和管理 API 使用角色隔离的调用方密钥，而非静态 Bearer 值。每次请求在 HTTPS 上使用 AES-256-GCM 加密 body、HMAC-SHA256 规范请求签名、时间窗和单次 nonce；key role、验签、解密、时间窗和重放任一失败统一 401。
- `key_id` 绑定调用方角色，支持 current/next 双 key 轮换；密钥、对象存储凭据与 HMAC/AES 材料仅通过运行环境注入，绝不提交至 Git、Actions 日志、镜像或 Kubernetes YAML。缺失激活密钥或 storage profile 返回 503，不能降级为匿名访问。
- 最终用户 token 只授权到一个预览资源及其 TTL；它不等同于 API server 或管理后台身份。
- 无效、过期和撤销 token 一律为 404，避免向浏览器泄露 token 是否曾存在。
- `/v/{token}` 是 bearer URL。成功的 302 必须带 `Cache-Control: no-store` 和 `Referrer-Policy: no-referrer`，避免客户端缓存 token→目标 URL 映射或在请求对象存储时把 token 放入 Referer。
- 302 会把短时签名目标 URL 交给客户端；撤销只影响后续 token 解析，不能撤回已经取得的目标 URL。因此签名有效期不得超过 token 的剩余 TTL，调用方不得记录、分享或把它作为业务资源链接。
- 若调用方使用 JavaScript PDF 阅读器，对象存储 CORS 只允许经审批的业务 Origin 读取；图片元素、iframe 和导航流程不得藉由宽松 CORS 取得不必要的读取权限。
- 微信小程序仅从已鉴权的调用方详情/预览接口获得 token、文件名、展示类型和过期时间，不获得资源 URL。小程序传给 H5 的 token 必须位于 URL fragment；H5 读取后立即清除，且日志、埋点、分享和本地存储均须脱敏或排除 token。
- 统一 trace 与错误处理中间件记录基础设施失败，返回响应不得泄露对象存储凭据、内部对象引用或调用方密钥材料。

当前仓库没有安全中间件或配置实现；实现需要在 Issue #1 中按此边界和 GoFrame 错误处理约定落地。
