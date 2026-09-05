# HTTP API 契约

以下为 Issue #1 按当前产品真相重构的目标契约，尚未在 `dev` 注册：

| 端点 | 认证 | 结果 |
| --- | --- | --- |
| `POST /internal/tokens` | Internal role 的加密、签名、防重放信封 | 校验 storage profile、对象键、内容 hash、文件名、双 TTL，返回加密的 token 与 `expires_at` |
| `POST /admin/tokens/query` | Admin role 的加密、签名、防重放信封 | 在加密 body 中接收 `limit`/`offset`，返回加密的有效 token 分页列表 |
| `POST /admin/tokens/{token}/revoke` | Admin role 的加密、签名、防重放信封 | 加密请求下幂等撤销 token |
| `GET /v/{token}` | token 本身 | v1 新接入唯一支持的公开导航 URL。图片/PDF 或转换后 PDF 的 302；无效/过期/撤销均为 404 |
| `GET /livez` | 无 | 仅供编排存活探针；进程可服务时为 200 |
| `GET /readyz` | 无 | 仅供编排就绪探针；Valkey、Gotenberg 与所需 storage profile 就绪时为 200，否则为 503 |

签发明文（加密信封内）：`storage_profile` 必须匹配服务端已配置 profile，`object_key` 只能标识该 profile 内对象，`content_sha256` 为 64 位小写十六进制 SHA-256，`filename` 必须含 [支持扩展名](formats.md)，`ttl` 为 60–86400 秒的必填整数，`cache_ttl` 为不超过 `MAX_CACHE_TTL` 的必填正整数，`metadata` 可选且仅用于管理展示。服务端禁止接收任意外部 URL、endpoint、bucket 或凭据。签发成功的加密 data 为 `token` 和 Unix 时间戳 `expires_at`。

## 内部加密签名信封

所有内部和管理请求都必须为 HTTPS `POST`，带 `X-Preview-Key-Id`、`X-Preview-Timestamp`、`X-Preview-Nonce` 和 `X-Preview-Signature`。body 为 `{iv, ciphertext}` 的 Base64 AES-256-GCM 信封；`iv` 每次随机生成。`X-Preview-Signature` 是调用方密钥对 `METHOD + "\\n" + PATH + "\\n" + key_id + "\\n" + timestamp + "\\n" + nonce + "\\n" + SHA-256(raw_body)` 的 HMAC-SHA256。

服务先校验 300 秒时间窗、key role、签名和未使用 nonce，再解密业务 body；任一失败统一 401，避免泄露失败原因。nonce 在 Valkey 保留 300 秒。调用方配置 current/next key 与 `key_id`，服务同时接受两把激活 key 用于轮换；响应 data 使用请求对应 key 的 AES-256-GCM 信封。

## 预览跳转契约

`GET /v/{token}` 仅支持正常 HTTP 导航：成功时以 302 和 `Location` 指向配置对象存储 profile 中短时签名的 HTTPS 对象 URL；它不返回文件字节、JSON、跨域可读取的 `Location`，也不提供客户端解析跳转目标的替代接口。302 响应必须发送 `Cache-Control: no-store` 与 `Referrer-Policy: no-referrer`。

签名目标 URL 的有效期不得超过 token 剩余 TTL。撤销只保证后续 `GET /v/{token}` 返回 404：此前已经取得目标 URL 的客户端可访问到该签名 URL 自己过期。调用方只能缓存 `/v/{token}`，不能将目标 URL 写入业务数据、分享链路或日志。

最终对象存储必须以 HTTPS、正确的媒体类型和内联文档处置方式响应。需要由 JavaScript PDF 阅读器读取的对象还需按 [clients.md](clients.md) 配置精确 CORS 和 Range 支持。

## 微信小程序调用方契约

微信小程序不直接调用 `POST /internal/tokens`，也不接收资源 URL。调用方业务服务以附件 ID 为输入，先完成自己的鉴权和详情查询，再在服务端签发 preview token，并仅向小程序返回 `{token, filename, preview_type, expires_at}`。`preview_type` 仅为 `image` 或 `pdf`；PDF 与 Office 均为 `pdf`。

小程序将该 DTO 传入自有 H5 `web-view` 的 URL fragment，H5 清除 fragment 后才拼接 `GET /v/{token}`。这不是本服务新增的公开端点，也不改变内部 token 签发 API；业务详情/预览接口的具体 URL 由调用方仓库定义。`wx.previewImage`、`wx.downloadFile` 和 `wx.openDocument` 不属于 v1.0.0 对 `/v/{token}` 的兼容承诺。

没有历史路由、兼容开关或测试辅助端点。规范 JSON 响应和错误形态由服务全局中间件统一处理；302 不包装为 JSON 成功响应。
