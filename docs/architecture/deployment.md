# 部署与运行边界

## 目标形态（Issue #1）

一个 Deployment 包含两个容器：默认运行 `server` 子命令的 file-preview-server（端口 9501）和 Gotenberg sidecar（端口 3000）。它们共享 Pod 网络，因此未显式覆盖时 `GOTENBERG_URL=http://localhost:3000`。Service 暴露应用容器，不直接暴露 sidecar；liveness probe 使用 `/livez`，readiness probe 使用 `/readyz`。

应用镜像只包含 Go 二进制及必要运行资源；Gotenberg 8.34.0 使用独立 sidecar 镜像，不把其二进制或 Office 依赖打入应用镜像。两个镜像均在部署声明中固定不可变 digest；缓存使用 Valkey。对象存储以具名 `storage_profile` 配置：v1 仅有 `aliyun-oss` 与 `silo` 两个 profile。环境变量配置 profile endpoint、bucket、凭据、生命周期、`MAX_CACHE_TTL`（默认 86400）、调用方 current/next HMAC key；生产环境配置 HTTPS/mTLS；明文凭据不写入仓库。

调用方源 URL 与每个 storage profile 都可能成为预览 302 的最终域名。部署放行前基础设施侧必须确认：源对象与转换 PDF 均经 HTTPS 提供，媒体类型和 `Content-Disposition: inline` 元数据正确；仓库内微信 demo H5 的 Origin 已被精确配置到所有测试最终域名的 CORS，且支持 `GET`、`HEAD` 和单个 `Range`；缓存对象按请求 `cache_ttl` 通过 provider 生命周期/清理策略删除。demo 维护预览服务、`aliyun-oss`/`silo` 和 H5 的测试域名配置，项目仓库不保存生产凭据。原生 App 的域名与网络策略在下一版本再定义。

## 当前事实与待交付边界

`dev` 中尚无 Dockerfile 或 `deploy/` 声明；当前文档未提供 Harbor 制品、Fleet GitRepo 目标、集群或 namespace，本轮未取得 Kubernetes 工作负载与部署健康证据。上述部署物属于 Issue #1 的交付范围；创建它们后由基础设施侧登记 Fleet 目标。

项目 Agent 只能经统一只读 Kubernetes MCP 观察 Fleet、Deployment、Pod、Service、Events、日志、镜像 digest 和健康状态。禁止使用 kubeconfig、Token、kubectl 写操作、Helm 写操作或 Ansible。

## 声明、放行与回滚

Issue #1 在 `deploy/` 维护应用/sidecar 镜像、Deployment、Service、HTTPS 入口、探针、资源与临时目录限制及环境配置引用；配置只引用环境已提供的 Secret，不能写入真实 key。demo API 与 H5 的测试入口单独声明，H5 与 `/v/` 通过同一 HTTPS Origin 的路径路由接入以支持失败分类，生产不得启用 demo fixture 接口。TLS 可在可信入口终止；后端仅接受该入口，清除外部伪造的 Forwarded 头，不能把任意 X-Forwarded-Proto 当作 HTTPS 证明。

发布记录必须绑定 Git commit、应用 digest、Gotenberg digest、配置版本、Fleet 目标、namespace 与前一已验收版本。尚未登记目标属于 #1/发布的交付项，不编造目标或健康结论。Fleet 拉取项目声明，项目 Agent 仅只读取证。

回滚条件包括持续 readyz 503、签发/预览 5xx 激增、签名与内容完整性错误、转换结果或 profile 验收回归。由具备权限的基础设施人员恢复项目声明到前一验收组合，经 Fleet 同步；重新检查 rollout、探针、两 profile、签发→预览→撤销及 demo 冒烟。首次发布若无旧版本则停止放行并关闭流量，由基础设施侧处理，不能假称已有回滚目标。

本服务无 SQL 迁移；Valkey 数据结构须加 schema 命名空间并在升级前声明兼容性。回滚不得把旧转换产物当作新版本缓存，不清空防重放数据；密钥轮换保留验签窗口且不能恢复已撤销密钥。对象清理与缓存版本同步回滚，不能先删除仍被有效短链使用的对象。

记录请求 trace、HTTP 状态、profile、缓存命中/等待、转换耗时/失败、锁续租失败及清理失败；所有层（含网关）均屏蔽 token 路径、源 URL query、Location 与签名头。监控 readyz、5xx、转换等待与对象清理积压；日志不记录原始文件内容。
