# 参与贡献

感谢你愿意改进 file-preview-server。

## 提交 Issue

- Bug：说明版本或镜像 tag、复现步骤、期望与实际结果，附请求的 `traceId`、时间和存储 profile。
- 不要在 Issue 中粘贴 token、签名 URL、HMAC 密钥或对象存储凭据。
- 安全漏洞请不要公开提交，按 [SECURITY.md](SECURITY.md) 私下报告。

## 产品范围

支持的文件格式永久限定为 [18 个扩展名](docs/architecture/formats.md)（常见图片、PDF、微软 Office 新旧版与金山 WPS）。增加其他格式的需求不会被接受，即使转换器本身支持。

## 开发流程

1. Fork 仓库，从最新 `main` 创建分支。
2. 保持改动聚焦，一个 PR 解决一件事。
3. 提交前在本地运行：

   ```sh
   . scripts/go-env.sh
   gofmt -l $(git ls-files '*.go')
   go vet ./...
   go test ./internal/module/preview/... ./demo/...
   python3 scripts/check-docs.py
   python3 scripts/check-deploy.py
   ```

4. 行为变更需要附带测试；影响接口、配置或部署的改动同步更新 `docs/` 与 `website/`。
5. 发起 PR 到 `main`，CI（`CI` workflow）必须通过。

集成测试（`Integration` workflow）依赖维护者配置的对象存储凭据，只在 `main` 上运行；fork 的 PR 不会触发它，维护者会在合并后跟进结果。

## 代码风格

- Go 代码使用 `gofmt`，遵循 `internal/module/preview` 下 domain / application / infrastructure / interfaces 的分层。
- 注释与文档可以用中文或英文，同一文件内保持一致。

## 许可

提交贡献即表示你同意以 [Apache License 2.0](LICENSE) 授权你的改动。
