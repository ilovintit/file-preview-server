# 安全边界

- 内部 token 签发和管理 API 使用角色隔离的调用方密钥，而非静态 Bearer 值。每次请求在 HTTPS 上使用 HMAC-SHA256 规范请求签名、时间窗和单次 nonce；生产环境优先 mTLS。key role、验签、时间窗和重放任一失败统一 401。
- `key_id` 绑定调用方角色，支持 current/next 双 HMAC key 轮换；密钥与对象存储凭据仅通过运行环境注入，绝不提交至 Git、Actions 日志、镜像或 Kubernetes YAML。缺失激活密钥或 storage profile 返回 503，不能降级为匿名访问。
- 源 URL 的授权来自已验签调用方，而不是域名 allowlist：仅接受 HTTPS URL，所有支持文件在缓存未命中下载时重新校验 `content_sha256`，并拒绝调用方提交对象存储 endpoint、bucket 或凭据。
- PDF/安全图片校验后原样入库，其余支持格式转换后入库；源 URL 仅由服务端使用，302 只能指向所选 profile 的短时签名对象。
- 最终用户 token 只授权到一个预览资源及其 TTL；它不等同于 API server 或管理后台身份。
- 无效、过期和撤销 token 一律为 404，避免向浏览器泄露 token 是否曾存在。
- `/v/{token}` 是 bearer URL。成功的 302 必须带 `Cache-Control: no-store` 和 `Referrer-Policy: no-referrer`，避免客户端缓存 token→目标 URL 映射或在请求对象存储时把 token 放入 Referer。
- 302 会把短时签名目标 URL 交给客户端；撤销只影响后续 token 解析，不能撤回已经取得的目标 URL。因此签名有效期不得超过 token 的剩余 TTL，调用方不得记录、分享或把它作为业务资源链接。
- 若调用方使用 JavaScript PDF 阅读器，对象存储 CORS 只允许经审批的业务 Origin 读取；图片元素、iframe 和导航流程不得藉由宽松 CORS 取得不必要的读取权限。
- 微信小程序仅从已鉴权的调用方详情/预览接口获得 token、文件名、展示类型和过期时间，不获得资源 URL。小程序传给 H5 的 token 必须位于 URL fragment；H5 读取后立即清除，且日志、埋点、分享和本地存储均须脱敏或排除 token。
- 统一 trace 与错误处理中间件记录基础设施失败，返回响应不得泄露对象存储凭据、内部对象引用或调用方密钥材料。

当前仓库没有安全中间件或配置实现；实现需要在 Issue #1 中按此边界和 GoFrame 错误处理约定落地。

下载端每一跳都须保持 HTTPS 并验证证书，限制重定向次数、响应字节数、下载/转换总时间与临时空间；不得向源站发送 Internal/Admin 的 HMAC 头。已签名来源证明调用方身份，不证明文件安全或源站可达；网络出站权限必须由部署方限定在已批准环境，不能把签名当作基础设施网络隔离。转换器不执行宏或主动启用外部资源，输出按支持格式验证媒体类型，不能只信 filename。具体预算由 #1 结合固定 fixture 测量后落入配置与 CI，超限用已定义的 422/5xx 语义处理。

nonce 的精确时间、占用顺序和原子性只在 [API 文档](apis.md) 定义；所有 401/404/422/5xx 与 token 签发/管理响应也使用 no-store，并禁止从错误响应泄露文件存在性以外的敏感字段。签名轮换不改变 Internal/Admin 的角色边界；nonce 存储丢失的恢复期间须 fail closed，覆盖残余验签时间窗后才重新开放受保护请求，不能以清空 Valkey 作为普通回滚步骤。
