# file-preview-server

[![CI](https://github.com/ilovintit/file-preview-server/actions/workflows/ci.yml/badge.svg)](https://github.com/ilovintit/file-preview-server/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

签名受控的文件在线预览服务。业务后端用 HMAC 签名签发短期 token，浏览器只拿到 `/v/{token}`，看不到源文件地址与存储凭据；服务负责下载、校验、把 Office/WPS/BMP/TIFF 转成 PDF、按内容 hash 缓存，并 302 跳转到对象存储的短时签名 URL。

文档：<https://ilovintit.github.io/file-preview-server/>

## 特性

- **18 种常见格式**：jpg/jpeg/png/gif/webp、bmp/tif/tiff、pdf、doc/docx、xls/xlsx、ppt/pptx 与金山 WPS 的 wps/et/dps，其余格式一律拒绝。
- **HMAC 签名 + 防重放**：签发与管理接口只接受 HTTPS 上的签名请求，nonce 原子占用，支持双密钥轮换。
- **内容 hash 转换缓存**：以内容 SHA-256 和转换器版本为身份，token 与缓存双 TTL，Valkey 管理生命周期与清理。
- **可替换对象存储**：阿里云 OSS 与 S3 兼容（silo/MinIO）两种 profile。
- **H5 / PC Web 阅读页**：内置基于 PDF.js 的阅读器，与 `/v/` 同源部署。

## 快速开始

```sh
docker pull ilovintit/file-preview-server:latest   # 或 ghcr.io/ilovintit/file-preview-server
```

运行需要 Valkey、[Gotenberg](https://gotenberg.dev/) 8.34.0、对象存储和 HTTPS 入口。完整步骤见 [快速开始](https://ilovintit.github.io/file-preview-server/guide/getting-started)，环境变量见 [配置](https://ilovintit.github.io/file-preview-server/guide/configuration)，Kubernetes 示例在 [`deploy/`](deploy/README.md)。

## 开发

需要 Go 1.25（CI 固定 1.25.14）、Python 3 和 Node.js 24。

```sh
. scripts/go-env.sh                # 构建缓存放在 .cache/，默认使用 proxy.golang.org
go test ./internal/module/preview/... ./demo/...
go build ./...
python3 scripts/check-docs.py && python3 scripts/check-deploy.py
```

国内网络可先 `export GOPROXY=https://goproxy.cn,direct` 再执行上面的命令。

集成测试（`-tags integration`）需要真实的 Valkey、Gotenberg 和 silo（S3 兼容），用 Docker 即可在本地拉起，启动方式和环境变量见 [`.github/workflows/integration.yml`](.github/workflows/integration.yml)。阿里云 OSS 适配器用例只在设置了 `PREVIEW_CI_ALIYUN_OSS_*` 时运行。

阅读页前端在 [`demo/preview-h5`](demo/preview-h5)，用 `sh scripts/build-web.sh` 构建后产物写入 `internal/module/preview/interfaces/reader-assets` 并随二进制嵌入。

设计与架构文档在 [`docs/`](docs/architecture/index.md)。文档站源码在 [`website/`](website)，使用 Hugo 与 [OINK](https://github.com/pgsty/oink) 主题，本地预览：`cd website && hugo server`（需要 Hugo Extended 0.165.0+ 与 Go 1.27+）。

## 贡献

欢迎 Issue 与 PR，请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md)。安全问题请按 [SECURITY.md](SECURITY.md) 私下报告。

## 许可

[Apache License 2.0](LICENSE)。内嵌的 PDF.js 资源同样以 Apache-2.0 发布，见 [`reader-assets/pdfjs`](internal/module/preview/interfaces/reader-assets/pdfjs)。
