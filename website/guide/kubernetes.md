# Kubernetes 部署

仓库的 [`deploy/app`](https://github.com/ilovintit/file-preview-server/tree/main/deploy/app) 提供 Kustomize 示例：应用与 Gotenberg sidecar 同 Pod，转换器只绑定 loopback，只有 9501 端口经 Service 暴露。

```sh
kubectl create secret generic file-preview-server-runtime \
  --from-literal=CALLER_KEYS_JSON='[...]' \
  --from-literal=STORAGE_PROFILES=silo \
  --from-literal=VALKEY_ADDR=valkey:6379 \
  --from-literal=SILO_ENDPOINT=... # 其余变量见“配置”

cd deploy/app
kustomize edit set image \
  docker.io/ilovintit/file-preview-server=docker.io/ilovintit/file-preview-server:1.2.0
# 把 ingress.yaml 中的 preview-host-not-registered.invalid 换成实际域名
kustomize build . | kubectl apply -f -
```

示例已包含只读根文件系统、非 root 运行、资源上限与 `/livez`、`/readyz` 探针。
