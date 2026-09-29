# 接入与交付说明

面向部署本服务并从业务系统接入的开发者。本文只描述本仓库交付的镜像、声明与契约，不包含任何集群部署、生产观察或回滚演练的结论——这些在实际部署时由部署方取证。

## 交付物

| 交付物 | 位置 | 说明 |
| --- | --- | --- |
| 应用镜像 | `docker.io/ilovintit/file-preview-server`（备用 `ghcr.io/ilovintit/file-preview-server`） | 单 Go 二进制，默认 `server` 子命令，监听 9501；由 [release.yml](../../.github/workflows/release.yml) 在 `v*` tag 时构建并推送，digest 写入该运行的摘要 |
| 转换器镜像 | `gotenberg/gotenberg:8.34.0@sha256:0ec4b0a1…` | Gotenberg 8.34.0 linux/amd64，作为同 Pod sidecar，见 [组件说明](../../deploy/components/gotenberg/README.md) |
| 部署声明 | [`deploy/app`](../../deploy/app/kustomization.yaml) | Deployment（应用 + sidecar）、Service、Ingress、默认 ConfigMap |
| 镜像级验证 | [`tests/image/run.sh`](../../tests/image/run.sh) | 用实际构建的镜像跑通 Web 旅程、运行预算与重启恢复，随回归报告执行 |

应用镜像版本与域名在仓库内保留显式占位符（`UNPINNED-SEE-RELEASE-RECORD`、`*.invalid`），部署时替换为已发布版本与实际域名，见 [deploy/README.md](../../deploy/README.md)。占位符不是可直接部署的取值，也不应改成浮动标签。

## 运行配置

运行配置经环境变量注入：仓库内 ConfigMap `file-preview-server-defaults` 提供非敏感默认值，部署方创建的 Secret `file-preview-server-runtime` 提供环境相关取值与凭据。本仓库不包含任何真实凭据。

| 变量 | 必填 | 说明 |
| --- | --- | --- |
| `LISTEN_ADDR` | 否 | 默认 `:9501` |
| `KEY_NAMESPACE` | 否 | 默认 `preview:v1`；不同环境必须使用不同取值以隔离状态 |
| `MAX_CACHE_TTL` | 否 | 默认 86400，签发要求 `ttl ≤ cache_ttl ≤ MAX_CACHE_TTL` |
| `VALKEY_ADDR` / `VALKEY_USERNAME` / `VALKEY_PASSWORD` / `VALKEY_DB` / `VALKEY_TLS` | 是 | token、nonce、缓存索引与锁 |
| `GOTENBERG_URL` | 是 | 同 Pod sidecar，固定 `http://localhost:3000` |
| `STORAGE_PROFILES` | 是 | `aliyun-oss`、`silo` 的子集；每个环境不要求同时启用两个 |
| `SILO_*` / 阿里云 OSS 对应变量 | 按启用的 profile | endpoint、bucket、前缀、凭据与签名 TTL，见 [storage-profiles](storage-profiles.md) |
| `CALLER_KEYS_JSON` | 是 | `[{"id","role","secret_base64"}]`，role 为 `internal` 或 `admin`，密钥 32–128 字节，支持 current/next 两把并存用于轮换 |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` / `TLS_CLIENT_CA_FILE` | 视入口 | 直接终止 TLS 时配置；由可信入口终止时改为配置 `TRUSTED_PROXY_CIDRS` |
| `TRUSTED_PROXY_CIDRS` | 视入口 | 只有来自这些网段且由该入口提供唯一 `X-Forwarded-Proto: https` 的请求才视为 HTTPS |

结构无效的配置会拒绝启动，而不是降级服务；`tests/image/run.sh` 对此有自动断言。

## 业务后端签发

业务后端持 `internal` 角色密钥，对每次签发请求签名：

```
canonical = "POST" + LF + path + LF + key_id + LF + timestamp + LF + nonce + LF + hex(sha256(raw_body))
X-Preview-Signature = hex(hmac_sha256(secret, canonical))
```

请求头还需 `X-Preview-Key-Id`、`X-Preview-Timestamp`（Unix 秒，±300 秒窗口）和 `X-Preview-Nonce`（128-bit 随机 hex，不可重用）。body 字段与错误码见 [apis.md](apis.md)；一个可运行的签名实现见 [demo 适配层](../../demo/adapter/adapter.go)，镜像级验证中的等价实现见 [e2e_test.go](../../tests/image/e2e_test.go)。

签发成功返回 `{token, expires_at}`。业务系统只把 `/v/{token}` 交给前端，不下发源 URL、签名目标 URL 或任何密钥。

## H5 / PC Web 接入

前端加载本服务的阅读页 `https://<预览域名>/reader/`，通过 URL fragment 传入 `{token, filename, preview_type, expires_at}`；阅读页清除 fragment 后再请求 `GET /v/{token}`。阅读页与 `/v/` 必须是同一 HTTPS Origin，否则无法按入口分类失败。PDF 阅读需要对象存储配置精确 CORS 与 Range，见 [clients.md](clients.md)。

允许的文件类型永久限定为常见图片、PDF 和微软新旧版/金山 WPS 办公文件，精确清单见 [formats.md](formats.md)；清单外的扩展名在签发阶段即被拒绝。

## 首次发布与停止

出现持续 `readyz` 503、签发/预览 5xx 激增或内容完整性错误时，停止入口流量，回滚到上一个已发布镜像版本或恢复配置。

## 反馈

使用中的问题提交到本仓库 [GitHub Issues](https://github.com/ilovintit/file-preview-server/issues)，附请求 traceId、时间、profile 与可复现输入。安全问题请按 [SECURITY.md](../../SECURITY.md) 私下报告。不要在 Issue 中粘贴 token、签名 URL 或凭据。
