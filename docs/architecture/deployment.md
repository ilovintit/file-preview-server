# 部署与运行边界

> v1.0.0 按 Issue #28 采用 H5 自动化验收并先交付业务接入。镜像、声明、配置与自动恢复检查仍交付；实际 Fleet/业务环境部署和人工回滚演练不再是首版验收前置。不因尚未取得生产 namespace、真机或固定观察记录阻塞首版制品交付，也不将这些事项标为已完成。

## 目标形态（Issue #1）

一个 Deployment 包含两个容器：默认运行 `server` 子命令的 file-preview-server（端口 9501）和 Gotenberg sidecar（端口 3000）。它们共享 Pod 网络，因此未显式覆盖时 `GOTENBERG_URL=http://localhost:3000`。Service 暴露应用容器，不直接暴露 sidecar；liveness probe 使用 `/livez`，readiness probe 使用 `/readyz`。

应用镜像只包含 Go 二进制及必要运行资源；Gotenberg 8.34.0 使用独立 sidecar 镜像，不把其二进制或 Office 依赖打入应用镜像。两个镜像均在部署声明中固定不可变 digest；缓存使用 Valkey。对象存储以具名 `storage_profile` 配置：v1 仅有 `aliyun-oss` 与 `silo` 两个 profile。环境变量配置 profile endpoint、bucket、凭据、生命周期、`MAX_CACHE_TTL`（默认 86400，签发要求 ttl ≤ cache_ttl ≤ MAX_CACHE_TTL）、调用方 current/next HMAC key；生产环境配置 HTTPS/mTLS；明文凭据不写入仓库。

只有所选 storage profile 是预览 302 的最终域名；调用方源 URL 仅供服务端下载，客户端不访问源站。部署放行前基础设施侧必须确认：profile 内的原样图片/PDF 与转换 PDF 均经 HTTPS 提供，媒体类型和 `Content-Disposition: inline` 元数据正确；共享 OSS 测试 bucket 对 H5 使用无凭据通配 CORS（`*`），支持 `GET`、`HEAD`、单个 `Range` 并暴露读取所需响应头；缓存对象按请求 `cache_ttl` 通过 provider 生命周期/清理策略删除。通配 CORS 不得与 Cookie/Authorization 等浏览器凭据组合使用。demo 维护预览服务、`aliyun-oss`/`silo` 和 H5 的测试域名配置，项目仓库不保存生产凭据。原生 App 的域名与网络策略在下一版本再定义。

## 当前事实与待交付边界

S03 新增 [Gotenberg sidecar 组件](../../deploy/components/gotenberg/README.md)，固定已同步至 Harbor 的 8.34.0 linux/amd64 digest、原装字体、同 Pod 连接与资源上限。它是待组装的 Deployment patch，不是可独立部署的应用。应用 Dockerfile、完整部署/入口与发布制品仍由 S12 交付；当前未指定 Fleet 目标、集群或 namespace，未取得实际 Kubernetes 部署健康证据。

项目 Agent 只能经统一只读 Kubernetes MCP 观察 Fleet、Deployment、Pod、Service、Events、日志、镜像 digest 和健康状态。禁止使用 kubeconfig、Token、kubectl 写操作、Helm 写操作或 Ansible。

## 声明、放行与回滚

Issue #1 在 `deploy/` 维护应用/sidecar 镜像、Deployment、Service、HTTPS 入口、探针、资源与临时目录限制及环境配置引用；配置只引用环境已提供的 Secret，不能写入真实 key。demo API 与 H5 的测试入口单独声明，H5 与 `/v/` 通过同一 HTTPS Origin 的路径路由接入以支持失败分类，生产不得启用 demo fixture 接口。TLS 可在可信入口终止；后端仅接受该入口，清除外部伪造的 Forwarded 头，不能把任意 X-Forwarded-Proto 当作 HTTPS 证明。

发布记录必须绑定 Git commit、应用 digest、Gotenberg digest、配置版本、Fleet 目标、namespace 与前一已验收版本。尚未登记目标属于 #1/发布的交付项，不编造目标或健康结论。Fleet 拉取项目声明，项目 Agent 仅只读取证。

回滚条件包括持续 readyz 503、签发/预览 5xx 激增、签名与内容完整性错误、转换结果或 profile 验收回归。由具备权限的基础设施人员恢复项目声明到前一验收组合，经 Fleet 同步；重新检查 rollout、探针、两 profile、签发→预览→撤销及 demo 冒烟。首次发布若无旧版本则停止放行并关闭流量，由基础设施侧处理，不能假称已有回滚目标。

本服务无 SQL 迁移；Valkey 数据结构须加 schema 命名空间并在升级前声明兼容性。回滚不得把旧转换产物当作新版本缓存，不清空防重放数据；密钥轮换保留验签窗口且不能恢复已撤销密钥。对象清理与缓存版本同步回滚，不能先删除仍被有效短链使用的对象。

记录请求 trace、HTTP 状态、profile、缓存命中/等待、转换耗时/失败、锁续租失败及清理失败；所有层（含网关）均屏蔽 token 路径、源 URL query、Location 与签名头。监控 readyz、5xx、转换等待与对象清理积压；日志不记录原始文件内容。

## 配置与制品清单

| 项目内声明 / 运行输入 | 内容与责任 | 当前状态 |
| --- | --- | --- |
| 应用 Dockerfile / 镜像 | 单 Go 二进制，默认 server；项目发布 CI 构建并推 Harbor，固定 commit 与 digest | 待 #1，未取得制品 |
| Gotenberg 镜像引用 | 固定版本/digest 与所需字体，独立 sidecar | 目标版本已定义，digest/字体与逐格式证据待 #1 |
| deploy 工作负载 / Service | 应用仅对 Service 暴露；sidecar 不单独公开；资源上限和临时目录、探针 | 待 #1 |
| deploy HTTPS 入口 | Internal/Admin 可信 TLS/mTLS 边界、H5 与 /v/ 同源路径、实际目标 Origin | 测试域名与生产目标待基础设施提供 |
| Valkey 连接 | 网络/TLS/认证、环境命名空间、容量与可靠性配置 | 待环境配置与故障恢复验证 |
| profile 配置 | 仅 aliyun-oss / silo；启用集合、endpoint、bucket、凭据引用、CORS、清理策略与配置版本 | 不能只给 provider 名称就宣称已接入 |
| caller keys | 按 Internal/Admin 分组的激活 key_id 与 current/next 密钥引用 | 环境注入；不在 PR 中填明文 |
| runtime 预算 | 缓存上限、请求/依赖/关闭预算、并发/队列、文件与临时空间、清理批次 | 约束见 quality，数值和测量待 #1 |
| Fleet GitRepo 与 target | 本项目 Git URL、发布分支、deploy 路径、cluster selector、namespace、同步状态 | 基础设施登记；本轮未指定目标 |

Harbor 地址、仓库名、机器人凭据与 Fleet target 没有现成证据，不能使用猜测地址或其他仓库资源补齐。PR CI 不注入部署/生产 key；发布 CI 的制品凭据仅由受保护发布环境提供。Fleet 拉取的是本项目内声明，不使用中央仓库承载本项目变更。首版 H5 自动化使用仓库内可复现的 CI 容器/HTTPS/provider 配置；实际业务域名、集群登记和运行反馈在交付接入时落实，不能以当前环境缺少真实业务目标要求人工前置验收。

## 启动、就绪与网络权限

双 profile 的环境变量、签名 Origin、最低操作权限与 `/readyz` 实施约束见 [storage-profiles](storage-profiles.md)。CI 的 silo/TLS 代理只属于每轮测试，不作为共享业务环境部署，也不向用户索要生产 silo 凭据。

### S02 OSS 预览域名

当前 OSS SDK 使用 V1 签名，`PREVIEW_CI_ALIYUN_OSS_REGION` 可省略；Endpoint 与 bucket 决定访问目标。若后续切换 V4 签名，需在同次变更中落实签名地域的推导或显式配置。

`PREVIEW_CI_ALIYUN_OSS_ENDPOINT` 是阿里 OSS 唯一的上传、HEAD、清理和签名 endpoint。签名 URL 明确签入 `response-content-disposition=inline`，不能依赖对象 metadata 或外部 CNAME/CDN 保持内联行为；这避免代理重写对象、丢弃查询串或返回 JSON 404。`PREVIEW_CI_ALIYUN_OSS_PREVIEW_ENDPOINT` 已移除，现有同名 Actions Variable 可以保留但服务不再读取。共享 bucket CORS 仍采用用户已确认的无凭据通配策略，不改变其他系统的桶配置。

### S02 运行预算

当前原样预览请求总预算10秒，源下载8秒且最多4次跟随重定向，所有跳转必须HTTPS；输入最多32MiB，图片最多4000万像素，每进程最多4个并行预览执行槽。准备锁3秒、每1秒续租；清理每5秒扫描最多32条，单批预算3秒，删除失败保留记录并至少30秒后重试。上述为实施资源上限，不代表已达到性能SLO；Office切片需按转换预算继续评估。

结构无效配置应拒绝启动；缺失某角色密钥时对应受保护端点按 503 关闭，不能省略中间件。readyz 检查当前声明启用的必要依赖，不把“支持两个 profile”误写为每个环境必须同时启用两个；两 profile 的产品验收仍都要完成。依赖详情不暴露给匿名探针调用方，内部观测记录脱敏原因。liveness 不依赖 Valkey/存储，避免外部故障引发无意义重启。

只向需要的进程注入凭据：应用可访问其 Valkey/profile/key；Gotenberg 不持有调用方密钥或存储 key，原文件作为已受控上传输入传递。临时目录按请求隔离并有容积上限，使用非 root、最小可写路径与部署侧出站限制；不通过关闭证书校验让测试环境“可用”。具体 runtime 预算和关闭次序见 quality/runtime。

## 升级与数据连续性

发布前固定应用、转换器、字体、配置与数据 schema 的兼容组合。兼容升级先确认新旧应用都能读取在役 token 数据；不可兼容时不能边滚动边共享误读状态，需要先定义版本隔离、停止新签发/排空策略及旧 token 影响，再经产品/发布审查。转换 output version 改变隔离缓存但不擅自重置 token 期限。

回滚验证包含在途请求取消、有效 token 是否受影响、nonce 连续性、旧 generation 清理与新旧 profile 配置身份。不能只恢复镜像而沿用不兼容数据/已退役密钥；也不能通过恢复旧授权快照复活已撤销 token。状态恢复边界见 [quality](quality.md)。

上述恢复机制在本期以自动化配置/镜像组合与 CI 服务容器故障恢复用例验证，实际业务/生产回滚保留操作说明，不要求提前人工演练。真实运行需要变更集群时仍由基础设施/Fleet 执行，项目 Agent 只读观测。首版交付不附加固定 24 小时观察期。
