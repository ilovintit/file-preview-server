# file-preview-server

独立文件预览服务：HTTPS 上签名保护的 token 签发、Office/图片→PDF 或浏览器预览、内容 hash 转换缓存、可替换对象存储适配与 Valkey 生命周期管理。开源项目，托管在 GitHub（`github.com/ilovintit/file-preview-server`），Apache-2.0。

## 真相源

- `docs/prd/product.md`：产品需求；`docs/architecture/index.md`：服务、领域、数据、接口、安全、测试与部署文档索引；`docs/design/index.html`：交互原型。
- `website/`：面向使用者的文档站（VitePress，发布到 GitHub Pages）。接口、配置或部署变更时同步更新 `docs/` 与 `website/`。
- `docs/journal/` 是迁移到 GitHub 之前的历史过程记录，其中的 Issue/PR 编号指向旧的内部仓库，只作参考。

## 永久格式范围

- 只支持常见图片、PDF 和微软新旧版/金山 WPS 办公文件，精确 18 个后缀见 `docs/architecture/formats.md`。其他格式永久不支持，不能因为转换器能力、历史代码或 WPS 能打开而扩大。
- 签发、旧 token 预览和缓存命中路径都执行允许集合。Microsoft Works 的 `.wps` 样例不能当作金山 WPS 验收。

## 分支与 PR

- `main` 是唯一主线，发布通过推送 `v*` tag 触发 `.github/workflows/release.yml`。
- 贡献从 `main` 切分支，PR 目标为 `main`，`CI` workflow 必须全绿。

## CI

- `ci.yml`（必须通过）：文档链接/原型脚本、部署声明结构与 kustomize 渲染、gofmt/vet、单元测试与构建。
- `integration.yml`（PR、main 推送与手动触发）：一次性 Valkey/Gotenberg/silo 服务集成、Chromium 浏览器导航、实际镜像端到端，不需要任何 Secret。silo（S3 协议）是默认测试存储；阿里云 OSS 适配器用例只在仓库配置了 `PREVIEW_CI_ALIYUN_OSS_*` 时运行，否则显式跳过。失败如实记录，不写回仓库、不自动重录基线。当前 `tests/vrt`、`e2e` 无已接受基线。
- `release.yml`：构建应用镜像，推送 Docker Hub（主）与 GHCR（备），创建 GitHub Release。
- `docs.yml`：构建文档站，main 推送时发布 GitHub Pages。
- 所有第三方 Action 与镜像按 SHA/digest 固定；Go 使用公共模块代理，不依赖任何私有仓库、私有镜像或内部网络。

## 部署声明

- `deploy/app` 是通用 Kustomize 示例：应用与 Gotenberg sidecar 同 Pod，转换器绑定 loopback，只暴露 9501。
- 非敏感默认值放 ConfigMap；凭据由部署方创建 Secret `file-preview-server-runtime`，仓库内不声明 Secret 对象、不写任何真实凭据。
- 应用镜像与域名在仓库内保留显式占位符，由部署方替换；其余镜像固定 digest。
- playground 只用于本地演示，不进入部署声明。

## 验证

- 声称通过前必须有新鲜的命令输出或对应 CI job 结果。
- 本地快速检查：`. scripts/go-env.sh && go vet ./... && go test ./internal/module/preview/... ./demo/... && go build ./...`，以及 `python3 scripts/check-docs.py`、`python3 scripts/check-deploy.py`。
