---
title: 业务系统接入
description: 签发 token、请求签名与前端打开预览。
weight: 30
---

接入分两步：业务后端签名调用签发接口拿到 token，前端用 token 打开阅读页。源文件地址、对象存储地址和密钥都只留在服务端。

## 1. 后端签发 token

`POST /internal/tokens`，调用方持 `internal` 角色密钥：

```json
{
  "url": "https://files.example.com/contract.docx",
  "storage_profile": "silo",
  "content_sha256": "<64 位小写十六进制>",
  "filename": "合同.docx",
  "ttl": 600,
  "cache_ttl": 86400,
  "metadata": { "biz_id": "123" }
}
```

| 字段 | 说明 |
| --- | --- |
| `url` | 源文件 HTTPS 地址，服务下载后按 `content_sha256` 校验 |
| `storage_profile` | `aliyun-oss` 或 `silo`，预览产物存放位置 |
| `filename` | 必须带 [支持的扩展名](../formats/) |
| `ttl` | token 有效期，60–86400 秒 |
| `cache_ttl` | 产物缓存期，满足 `ttl ≤ cache_ttl ≤ MAX_CACHE_TTL` |
| `metadata` | 可选，仅用于管理查询展示，最大 4096 字节 |

成功返回 `{"traceId": "...", "code": 0, "message": "...", "data": {"token": "...", "expires_at": 1790000000}}`。

### 请求签名

请求头：

| 请求头 | 取值 |
| --- | --- |
| `X-Preview-Key-Id` | 密钥 id |
| `X-Preview-Timestamp` | Unix 秒，与服务端时间相差不超过 300 秒 |
| `X-Preview-Nonce` | 128-bit 随机数的小写 hex，不可重用（重试也必须换新 nonce） |
| `X-Preview-Signature` | 见下 |

```
canonical = "POST" + "\n" + path + "\n" + key_id + "\n" + timestamp + "\n" + nonce + "\n" + hex(sha256(raw_body))
X-Preview-Signature = hex(hmac_sha256(secret, canonical))
```

`\n` 是单字节换行，末尾没有换行；`path` 不含域名和 query；`raw_body` 是实际发送的字节。鉴权失败统一返回 401，业务校验失败 422，依赖不可用 503。Go 实现可参考 [demo/adapter/adapter.go](https://github.com/ilovintit/file-preview-server/blob/main/demo/adapter/adapter.go)。

## 2. 前端打开预览

后端把 `{token, filename, preview_type, expires_at}` 交给前端（`preview_type` 为 `image` 或 `pdf`，Office 文档也是 `pdf`）。前端打开同源的阅读页，并通过 URL fragment 传入这些字段：

```
https://preview.example.com/reader/#token=...&filename=...&preview_type=pdf&expires_at=...
```

阅读页会先清除 fragment，再请求 `GET /v/{token}`。服务首次访问时下载、校验并按需转换，然后以 302 跳转到对象存储的短时签名 URL。阅读页与 `/v/` 必须在同一 HTTPS Origin 下。

## 管理接口

持 `admin` 角色密钥，签名方式相同：

- `POST /admin/tokens/query`：body `{"limit": 20, "offset": 0}`，分页列出有效 token。
- `POST /admin/tokens/{token}/revoke`：body `{}`，幂等撤销。撤销后 `/v/{token}` 返回 404，已发出的签名 URL 会在自身到期后失效。
