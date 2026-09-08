# Gotenberg sidecar component (S03)

`sidecar.yaml` 是供 S12 应用 Deployment 组装的 strategic merge patch，不是可独立部署的应用。不创建 Service/Ingress，不执行集群写入。应用和转换器同 Pod，只有应用暴露给业务；转换器绑定 loopback。Harbor 拉取凭据由部署环境已有 imagePullSecret 注入，不能把 CI Secret 值写入声明。

固定上游 Gotenberg 8.34.0 linux/amd64 镜像 digest `sha256:0ec4b0a125c55ff1dfaed071cab30e0f37cc4326f7d53d903ee5b24e4f27fd85`，使用镜像内原装字体，不挂载覆盖字体。输出身份包含该 digest、禁止外链及固定请求选项版本，升级任一项须更新缓存版本并重新验证格式矩阵。此组件只支持该镜像架构，其他架构须单独固定并验收。

转换器无 OSS/Valkey/HMAC 凭据。禁用外部下载、LibreOffice 网络引用和 Chromium 路由；只有应用提交的文件字节进入转换。CI 容器采用相同镜像及渲染设置，通过服务 DNS 访问；部署绑定 loopback，所以 CI 的网络绑定与部署不同。资源值是明确上限而非已经测得的性能 SLO。read-only 根和两个限容可写目录的真实运行检查随 S12 组装交付，不在此处声称已经部署通过。
