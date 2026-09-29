# 配置

所有配置经环境变量注入。结构无效的配置会让进程拒绝启动。凭据类变量请放在 Secret 或密钥管理系统中，不要写进镜像或代码仓库。

## 通用

| 变量 | 必填 | 说明 |
| --- | --- | --- |
| `LISTEN_ADDR` | 否 | 默认 `:9501` |
| `KEY_NAMESPACE` | 否 | 默认 `preview:v1`；不同环境必须使用不同取值以隔离状态 |
| `MAX_CACHE_TTL` | 否 | 默认 86400 秒；签发要求 `ttl ≤ cache_ttl ≤ MAX_CACHE_TTL` |
| `VALKEY_ADDR` / `VALKEY_USERNAME` / `VALKEY_PASSWORD` / `VALKEY_DB` / `VALKEY_TLS` | 是（地址） | token、nonce、缓存索引与锁 |
| `GOTENBERG_URL` | 是 | 转换器地址，同 Pod sidecar 时为 `http://localhost:3000` |
| `STORAGE_PROFILES` | 是 | `aliyun-oss`、`silo` 的子集，逗号分隔 |
| `CALLER_KEYS_JSON` | 是 | `[{"id","role","secret_base64"}]`，role 为 `internal` 或 `admin`，密钥 32–128 字节；同一角色可配置两把用于轮换 |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` / `TLS_CLIENT_CA_FILE` | 视入口 | 应用直接终止 TLS（可选 mTLS）时配置 |
| `TRUSTED_PROXY_CIDRS` | 视入口 | 由可信入口终止 TLS 时配置；只有来自这些网段且带唯一 `X-Forwarded-Proto: https` 的请求才视为 HTTPS |

## silo / MinIO（S3 兼容）

| 变量 | 说明 |
| --- | --- |
| `SILO_ENDPOINT` | SDK 上传、Stat、删除使用的 S3 API Origin；支持 HTTPS 或受控内网 HTTP |
| `SILO_PREVIEW_ENDPOINT` | 浏览器访问签名 URL 的 Origin，必须 HTTPS；未设置时使用 `SILO_ENDPOINT`（此时它也必须是 HTTPS） |
| `SILO_REGION` | S3 签名 region，默认 `us-east-1` |
| `SILO_BUCKET` / `SILO_PREFIX_BASE` | 目标 bucket 与对象前缀；服务不会自动创建 bucket |
| `SILO_CONFIG_GENERATION` | 可选，显式配置代次 |
| `SILO_ACCESS_KEY_ID` / `SILO_ACCESS_KEY_SECRET` / `SILO_SECURITY_TOKEN` | 读写删除凭据，token 可选 |
| `SILO_SIGNED_URL_MAX_TTL_SECONDS` | 必填，1–604800 |

## 阿里云 OSS

| 变量 | 说明 |
| --- | --- |
| `PREVIEW_CI_ALIYUN_OSS_ENDPOINT` | OSS endpoint |
| `PREVIEW_CI_ALIYUN_OSS_PREVIEW_ENDPOINT` | 可选，浏览器访问签名 URL 使用的 endpoint |
| `PREVIEW_CI_ALIYUN_OSS_REGION` | region |
| `PREVIEW_CI_ALIYUN_OSS_BUCKET` / `PREVIEW_CI_ALIYUN_OSS_PREFIX_BASE` | bucket 与对象前缀 |
| `PREVIEW_CI_ALIYUN_OSS_ACCESS_KEY_ID` / `PREVIEW_CI_ALIYUN_OSS_ACCESS_KEY_SECRET` / `PREVIEW_CI_ALIYUN_OSS_SECURITY_TOKEN` | 凭据，token 可选 |
| `PREVIEW_CI_OSS_SIGNED_URL_MAX_TTL_SECONDS` | 签名 URL 最长有效期 |

::: warning 变量名
阿里云 OSS 的变量名目前带有历史遗留的 `PREVIEW_CI_` 前缀，生产环境也使用这组名称。
:::

PDF 阅读器通过 JavaScript 分段读取文件，对象存储需要为阅读页 Origin 配置精确的 CORS，并允许 `Range` 请求头、暴露 `Content-Range` 等响应头。
