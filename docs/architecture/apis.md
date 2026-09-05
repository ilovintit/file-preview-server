# HTTP API 契约

以下为 Issue #1 的已确认目标契约，尚未在 `dev` 注册：

| 端点 | 认证 | 结果 |
| --- | --- | --- |
| `POST /internal/tokens` | `Authorization: Bearer {INTERNAL_TOKEN}` | 校验 `url`、`filename`、`ttl`，返回 token 与 `expires_at` |
| `GET /admin/tokens?limit=&offset=` | `Authorization: Bearer {ADMIN_TOKEN}` | 返回有效 token 的分页列表 |
| `DELETE /admin/tokens/{token}` | Admin Bearer | 幂等撤销 token |
| `GET /v/{token}` | token 本身 | v1 新接入唯一支持的公开导航 URL。图片/PDF 或转换后 PDF 的 302；无效/过期/撤销均为 404 |
| `GET /preview?url=` | 迁移期历史端点 | 可由 `DISABLE_LEGACY_PREVIEW=1` 关闭为 404 |

签发请求：`url` 必须为 http(s) URL，`filename` 必须含支持的扩展名，`ttl` 为 60–86400 秒的必填整数，`metadata` 可选并仅用于管理展示。签发成功的 data 为 `token` 和 Unix 时间戳 `expires_at`。

## 预览跳转契约

`GET /v/{token}` 仅支持正常 HTTP 导航：成功时以 302 和 `Location` 指向 silo 中短时签名的 HTTPS 对象 URL；它不返回文件字节、JSON、跨域可读取的 `Location`，也不提供客户端解析跳转目标的替代接口。302 响应必须发送 `Cache-Control: no-store` 与 `Referrer-Policy: no-referrer`。

签名目标 URL 的有效期不得超过 token 剩余 TTL。撤销只保证后续 `GET /v/{token}` 返回 404：此前已经取得目标 URL 的客户端可访问到该签名 URL 自己过期。调用方只能缓存 `/v/{token}`，不能将目标 URL 写入业务数据、分享链路或日志。

最终 silo 对象必须以 HTTPS、正确的媒体类型和内联文档处置方式响应。需要由 JavaScript PDF 阅读器读取的对象还需按 [clients.md](clients.md) 配置精确 CORS 和 Range 支持。

## 微信小程序调用方契约

微信小程序不直接调用 `POST /internal/tokens`，也不接收资源 URL。调用方业务服务以附件 ID 为输入，先完成自己的鉴权和详情查询，再在服务端签发 preview token，并仅向小程序返回 `{token, filename, preview_type, expires_at}`。`preview_type` 仅为 `image` 或 `pdf`；PDF 与 Office 均为 `pdf`。

小程序将该 DTO 传入自有 H5 `web-view` 的 URL fragment，H5 清除 fragment 后才拼接 `GET /v/{token}`。这不是本服务新增的公开端点，也不改变内部 token 签发 API；业务详情/预览接口的具体 URL 由调用方仓库定义。`wx.previewImage`、`wx.downloadFile` 和 `wx.openDocument` 不属于 v1.0.0 对 `/v/{token}` 的兼容承诺。

既有 `/health`、`/img/`、`/redact`、`/redact-pdf`、`/pdf-to-images`、`/images-to-pdf` 需在迁移中保留其功能行为。测试辅助端点只在 `ALLOW_TEST_ENDPOINTS=1` 时注册。规范 JSON 响应和错误形态由服务全局中间件统一处理；302 不包装为 JSON 成功响应。
