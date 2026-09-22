# dev playground 与 shwkj-dev 演示环境

## 用户裁决

- playground 做成独立镜像，不在生产镜像里加开关：生产与 demo 的边界应体现为制品差异，而不是运行时配置。
- 存储用阿里云 OSS 测试 bucket（用户单独创建），不在集群内自管 silo。
- 上线由用户/基础设施侧登记 Fleet，Agent 只读观测。

## 集群现状（只读观测 shwkj-dev）

Rancher + Fleet 管理，应用按 `<项目>-dev` 分 namespace，dev 中间件各自在本 namespace 自管（如 `shyun-valkey`、`shyun-pgsql`）。Ingress 用 nginx class、`*.shwkj.cn`，**只有 80 端口，没有 TLS**。组织里已有 playground 先例（`taro-ui-playground`、`shw-ui-playground-h5`），都由 Fleet + Helm 管理。

## 关键设计

- **源文件不进 OSS**：一开始以为示例文件要预灌进 bucket 并开公开读。实际上签发只要求源 URL 是 HTTPS，而 playground 本来就在集群内，于是让它 `go:embed` 示例并自己以 HTTPS 提供 `/demo/source/<name>`。OSS 只承担预览产物的缓存与签名，bucket 侧只需建好并配 CORS。
- **集群内必须 TLS**：受保护 API 要求真实 HTTPS，`demo/adapter` 也强制 `APIOrigin` 为 https，所以即使入口是 HTTP，两个 Pod 之间仍需自签证书，Ingress 要加 `backend-protocol: HTTPS`。
- **复用 adapter**：`demo/adapter` 的静态 Files 映射正好够用，playground 只做内嵌、哈希、装配与访问口令入口，没有复制签名逻辑。

## 修正 #26 留下的缺陷

`kubectl kustomize deploy/app` 直接失败：kustomization 引用了目录外的 patch 文件（`../components/gotenberg/sidecar.yaml`），kustomize 出于安全拒绝。也就是说 #26 交付的声明**从来没有真正渲染成功过**，而当时的检查只做文本扫描，看不出来。

修法是把 sidecar 变成正规的 kustomize Component（自带 kustomization，patch 位于自身目录内），dev overlay 里同名 ConfigMap 从 resources 改为 patch。检查脚本现在会真实渲染两个 overlay 并复查渲染结果里没有 Secret 引用——纯文本检查挡不住结构错误，这是本轮最值得留下的教训。

## 待用户提供

namespace、对外 host、OSS endpoint/region/bucket/前缀/凭据、自签证书三件套、访问口令、bucket 的 CORS 允许来源。仓库内保留显式占位符。

## 渲染检查放在哪里

第一版把 `kustomize build` 塞进 `check-deploy.py`，CI 直接失败：ci-fast runner 上没有 kustomize 也没有 kubectl。按"不静默跳过"的原则，脚本是显式报错退出的，这次失败本身是符合预期的行为。

改法：纯文本检查留在文档任务（python3 在那里已验证可用），渲染检查用 shell + `go run sigs.k8s.io/kustomize/kustomize/v5@v5.7.1` 放进快速层（那里有内网 Go 工具链与代理，已本地实测可拉取）。没有把检查降级成"有就查、没有就跳过"，也没有引入公网下载。

工具链容器里是否有 python3 无法本地确认，所以不把 python 脚本放进那个容器——每项检查都放在依赖已被证实的地方。
