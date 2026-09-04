# HTTP API 契约

以下为 Issue #1 的已确认目标契约，尚未在 `dev` 注册：

| 端点 | 认证 | 结果 |
| --- | --- | --- |
| `POST /internal/tokens` | `Authorization: Bearer {INTERNAL_TOKEN}` | 校验 `url`、`filename`、`ttl`，返回 token 与 `expires_at` |
| `GET /admin/tokens?limit=&offset=` | `Authorization: Bearer {ADMIN_TOKEN}` | 返回有效 token 的分页列表 |
| `DELETE /admin/tokens/{token}` | Admin Bearer | 幂等撤销 token |
| `GET /v/{token}` | token 本身 | 图片/PDF 或转换后 PDF 的 302；无效/过期/撤销均为 404 |
| `GET /preview?url=` | 迁移期历史端点 | 可由 `DISABLE_LEGACY_PREVIEW=1` 关闭为 404 |

签发请求：`url` 必须为 http(s) URL，`filename` 必须含支持的扩展名，`ttl` 为 60–86400 秒的必填整数，`metadata` 可选并仅用于管理展示。签发成功的 data 为 `token` 和 Unix 时间戳 `expires_at`。

既有 `/health`、`/img/`、`/redact`、`/redact-pdf`、`/pdf-to-images`、`/images-to-pdf` 需在迁移中保留其功能行为。测试辅助端点只在 `ALLOW_TEST_ENDPOINTS=1` 时注册。规范 JSON 响应和错误形态由服务全局中间件统一处理；302 不包装为 JSON 成功响应。
