# 快速开始

file-preview-server 是一个独立的 Go 服务。业务后端用 HMAC 签名调用它签发短期 token，前端打开 `/v/{token}` 即可预览；服务负责下载源文件、校验内容、必要时转换为 PDF，并跳转到对象存储上的短时签名地址。

## 运行依赖

| 组件 | 作用 |
| --- | --- |
| [Valkey](https://valkey.io/)（或兼容 Redis 协议的服务） | token、nonce、缓存索引与分布式锁 |
| [Gotenberg](https://gotenberg.dev/) 8.34.0 | Office 与 BMP/TIFF 转 PDF，建议与应用同 Pod 并绑定 loopback |
| 对象存储 | 阿里云 OSS，或 S3 兼容的 silo / MinIO；需要 HTTPS 访问地址 |
| TLS | 受保护接口要求 HTTPS：应用直接终止 TLS，或由可信入口终止并配置 `TRUSTED_PROXY_CIDRS` |

## 获取镜像

```sh
docker pull ilovintit/file-preview-server:latest
# 或使用 GitHub Container Registry
docker pull ghcr.io/ilovintit/file-preview-server:latest
```

镜像只包含一个静态 Go 二进制，以非 root 用户运行，默认监听 `9501`。转换器使用官方镜像 `gotenberg/gotenberg:8.34.0`，本项目不重新打包。

## 启动

```sh
docker run -d --name file-preview-server -p 9501:9501 \
  -e VALKEY_ADDR=valkey:6379 \
  -e GOTENBERG_URL=http://gotenberg:3000 \
  -e STORAGE_PROFILES=silo \
  -e SILO_ENDPOINT=http://minio:9000 \
  -e SILO_PREVIEW_ENDPOINT=https://files.example.com \
  -e SILO_REGION=us-east-1 \
  -e SILO_BUCKET=preview \
  -e SILO_PREFIX_BASE=preview \
  -e SILO_SIGNED_URL_MAX_TTL_SECONDS=300 \
  -e SILO_ACCESS_KEY_ID=... -e SILO_ACCESS_KEY_SECRET=... \
  -e CALLER_KEYS_JSON='[{"id":"backend","role":"internal","secret_base64":"..."}]' \
  -e TRUSTED_PROXY_CIDRS=10.0.0.0/8 \
  ilovintit/file-preview-server:latest
```

配置结构无效时服务会拒绝启动，而不是降级运行。全部变量见 [配置](./configuration)。

::: tip 冷启动保护窗口
使用全新的 `KEY_NAMESPACE` 首次启动时，为保证防重放状态完整，`/readyz` 会在约 601 秒内返回 503，这是预期行为。
:::

## 健康检查

- `GET /livez`：进程可服务时返回 200。
- `GET /readyz`：Valkey、Gotenberg 与已启用的存储 profile 都就绪时返回 200，否则 503。

下一步：[接入业务系统](./integration)。
