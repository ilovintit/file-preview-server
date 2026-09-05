# 部署与运行边界

## 目标形态（Issue #1）

一个 Deployment 包含两个容器：默认运行 `server` 子命令的 file-preview-server（端口 9501）和 Gotenberg sidecar（端口 3000）。它们共享 Pod 网络，因此未显式覆盖时 `GOTENBERG_URL=http://localhost:3000`。Service 暴露应用容器，不直接暴露 sidecar；liveness probe 使用 `/livez`，readiness probe 使用 `/readyz`。

镜像应为单一 Go 二进制镜像，包含固定版本的 Gotenberg 处理依赖；缓存使用 Valkey。对象存储以具名 `storage_profile` 配置：阿里云 OSS 为首个验收 profile，标准 S3 API profile 用于符合适配合同的常见对象存储。环境变量配置 profile endpoint、bucket、凭据、生命周期、Valkey、调用方 current/next HMAC/AES key；明文凭据不写入仓库。

每个 storage profile 都是预览 302 的最终对象域名。部署放行前基础设施侧必须确认：对象与转换 PDF 均经 HTTPS 提供，媒体类型和 `Content-Disposition: inline` 元数据正确；微信小程序自有 H5 的 Origin 已被精确配置到对应对象存储 CORS，且支持 `GET`、`HEAD` 和单个 `Range`；缓存对象按请求 `cache_ttl` 通过 provider 生命周期/清理策略删除。微信小程序调用方负责在平台侧登记 H5、预览服务和实际对象存储域名，并持有相应配置凭据；项目仓库不保存这些平台凭据。原生 App 的域名与网络策略在下一版本再定义。

## 当前事实与待交付边界

`dev` 中尚无 Dockerfile、`deploy/` 声明、Harbor 制品或 Fleet GitRepo 目标，集群和 namespace 也未被指定。因此没有可读取的 Kubernetes 工作负载或部署健康证据。上述部署物属于 Issue #1 的交付范围；创建它们后由基础设施侧登记 Fleet 目标。

项目 Agent 只能经统一只读 Kubernetes MCP 观察 Fleet、Deployment、Pod、Service、Events、日志、镜像 digest 和健康状态。禁止使用 kubeconfig、Token、kubectl 写操作、Helm 写操作或 Ansible。
