# R3：可部署镜像、Web 链路与业务接入交付

## 基线

从 dev@7a29f81 切出 `chore/26-work`。#37（允许格式）与 #25（H5/PC 阅读页、Dockerfile、image.yml）已在 dev；本项只补完整部署组合、镜像级证据与接入说明，不重做前序范围。

## 关键实现决定

- **部署声明装配**：`deploy/app/kustomization.yaml` 用 patch 把 S03 已交付的 `deploy/components/gotenberg/sidecar.yaml` 装配进应用 Deployment，避免复制一份 sidecar 定义。dev overlay 额外声明自管 Valkey/silo，test 与生产连接外部中间件。
- **只用 ConfigMap**：运行配置分两层，仓库内 `file-preview-server-defaults` 只放非敏感默认值，凭据由基础设施预置的 `file-preview-server-runtime` 提供。全目录没有 Secret 资源或 Secret 引用，`check-deploy.py` 在 PR 门禁上机械校验。
- **未登记事实用显式占位符**：应用镜像 digest（`UNPINNED-SEE-RELEASE-RECORD`）和业务/对象存储域名（`*.invalid`）都保留占位符，由 GitOps publisher 按发布记录写入 fleet 分支。既不编造 digest，也不退回浮动标签；占位符写进检查脚本的白名单，改成别的值就会被门禁拦下。
- **镜像级证据放非阻断回归报告**：按 #45 裁决，服务/容器级验证不进 PR required 门禁。`tests/image/run.sh` 构建实际镜像并在一次性 docker 网络里编排 Valkey/Gotenberg/silo，新增 `imagee2e` job 写入回归报告 JSON。
- **HTTPS 约束的解法**：受保护 API 要求实际 TLS，silo 的 preview endpoint 也要求 HTTPS。测试前置程序 `tests/image/prepare` 生成一次性 CA 与两张证书（`app`、`harness`），应用容器通过 `SSL_CERT_FILE` 信任该 CA；测试容器以 `harness` 别名同时提供源文件和保持 Host 的 silo 反向代理，使 SigV4 签名按签名中的 Host 校验成功。
- **探针脚本不依赖 curl**：不确定工具链镜像是否带 curl，改用 `tests/image/probe` 这个只打印真实状态码的小程序，连接失败打印 000，不把失败伪装成其他状态。

## 事实与边界

- 允许格式的 Content-Type 约束：原样图片按扩展名要求精确媒体类型（`.png` 必须 `image/png`），Office 输入按内容校验、忽略源站 Content-Type。测试的源文件服务按此设置响应头。
- 镜像级旅程使用生成的 PNG（原样路径）与仓库内 `testdata/office/document.docx`（转换路径）。这不替代已有的 18 后缀格式矩阵，后者仍在回归报告的集成 job。
- 本次没有取得任何 Kubernetes 部署、rollout、Fleet 同步或生产观察证据；Fleet 目标仍未登记。

## 交付

PR [#49](https://git.shw.top/shw-project/file-preview-server/pulls/49)，目标 dev，`Closes #26`。本地通过 gofmt、`go vet ./...`、`go vet -tags image ./tests/image`、`go build ./...`、`go test ./internal/module/preview/...`、`check-docs.py` 与 `check-deploy.py`。

## 遗留

- v1.0.0 tag 构建出正式镜像后，应把发布记录中的真实 digest 交给 publisher；仓库内占位符是否改为固定 digest 由发布后的实际做法决定。
- `image.yml` 仍使用 `CI_HARBOR_*` Secret 名（沿用 #45 的记录，未核实公共 `HARBOR_*` 变量）。
