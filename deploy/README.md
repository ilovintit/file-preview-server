# 部署声明

本目录提供 Kubernetes 部署示例（Kustomize），不包含任何集群凭据。

| 路径 | 用途 |
| --- | --- |
| `app/` | 应用基线：Deployment（应用 + Gotenberg sidecar）、Service、Ingress、默认 ConfigMap |
| `components/gotenberg/` | Gotenberg sidecar 组件，由 `app/kustomization.yaml` 装配 |

使用方式：

```sh
# 1. 创建凭据 Secret（键名见 docs/architecture/integration.md）
kubectl create secret generic file-preview-server-runtime \
  --from-literal=CALLER_KEYS_JSON='[...]' \
  --from-literal=STORAGE_PROFILES=silo ...

# 2. 固定镜像版本与域名后渲染部署
cd deploy/app
kustomize edit set image docker.io/ilovintit/file-preview-server=docker.io/ilovintit/file-preview-server:v1.2.0
kustomize build . | kubectl apply -f -
```

约束：

- 非敏感默认值放在 ConfigMap `file-preview-server-defaults`；凭据由部署方创建 Secret `file-preview-server-runtime` 提供，仓库内不声明 Secret 对象。
- 应用镜像与 Ingress 域名在仓库内是显式占位符（`UNPINNED-SEE-RELEASE-RECORD`、`preview-host-not-registered.invalid`），部署时替换为已发布版本和实际域名；其余镜像固定 digest。
- 转换器绑定 loopback，不经 Service 暴露；只有应用端口 9501 对外。
- 上述结构约束由 [check-deploy.py](../scripts/check-deploy.py) 在 CI 中检查，并真实渲染 `deploy/app`；只检查仓库内声明，不代表任何环境已经部署或健康。
