# 部署声明

本目录是本项目自己维护的部署声明，由 Fleet 从固定 GitOps publisher 推进的 `fleet/*` 分支拉取。项目 Agent 对 Kubernetes 只读，本目录不包含任何集群写操作、kubeconfig 或凭据。

| 路径 | 用途 |
| --- | --- |
| `app/` | 应用基线：Deployment（应用 + Gotenberg sidecar）、Service、Ingress、默认 ConfigMap |
| `components/gotenberg/` | S03 交付的 sidecar 组件，由 `app/kustomization.yaml` 装配 |
| `dev/` | 开发环境 overlay：应用 + 自管 Valkey + playground 演示入口，存储用阿里云 OSS 测试 bucket；test 与生产不使用本 overlay，也不部署 playground |

约束：

- 运行配置只用 ConfigMap（`configMapRef`/`configMapKeyRef`）；不声明 Kubernetes Secret、`secretKeyRef` 或 Secret volume。镜像拉取凭据仅按名称引用基础设施预置的 `imagePullSecrets`。
- 镜像固定不可变 digest，不使用浮动标签。尚未登记的应用镜像 digest 与业务域名保留显式占位符，由 publisher 在 fleet 分支写入发布记录中的真实值。
- 转换器绑定 loopback，不经 Service 暴露；只有应用端口 9501 对外。
- 上述结构约束由 [check-deploy.py](../.gitea/scripts/check-deploy.py) 在 PR 门禁检查：它会真实渲染 `deploy/app` 与 `deploy/dev` 两个 overlay 并复查渲染结果，但只检查仓库内声明，不代表任何环境已经部署或健康。
- playground 只允许出现在 `dev/` overlay，生产与 test 不部署 demo fixture 入口；演示链路说明见 [playground.md](../docs/architecture/playground.md)。

Fleet 目标、集群与 namespace 由基础设施侧登记，当前未取得；接入说明见 [integration.md](../docs/architecture/integration.md)。
