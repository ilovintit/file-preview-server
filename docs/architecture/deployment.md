# 部署与运行边界

## 目标形态（Issue #1）

一个 Deployment 包含两个容器：默认运行 `server` 子命令的 file-preview-server（端口 9501）和 Gotenberg sidecar（端口 3000）。它们共享 Pod 网络，因此未显式覆盖时 `GOTENBERG_URL=http://localhost:3000`。Service 暴露应用容器，不直接暴露 sidecar。

镜像应为单一 Go 二进制镜像，包含现有处理依赖；对象存储使用 silo 的 S3 API，缓存使用 Valkey。环境变量配置端点、bucket、密钥、Valkey、内部/Admin/HMAC 密钥和历史端点开关；明文凭据不写入仓库。

## 当前事实与待交付边界

`dev` 中尚无 Dockerfile、`deploy/` 声明、Harbor 制品或 Fleet GitRepo 目标，集群和 namespace 也未被指定。因此没有可读取的 Kubernetes 工作负载或部署健康证据。上述部署物属于 Issue #1 的交付范围；创建它们后由基础设施侧登记 Fleet 目标。

项目 Agent 只能经统一只读 Kubernetes MCP 观察 Fleet、Deployment、Pod、Service、Events、日志、镜像 digest 和健康状态。禁止使用 kubeconfig、Token、kubectl 写操作、Helm 写操作或 Ansible。
