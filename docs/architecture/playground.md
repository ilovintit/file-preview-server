# dev playground 演示环境

playground 只在开发环境部署，供业务同事在浏览器里实际走通一次预览链路并提反馈。它是独立镜像，生产预览服务镜像不包含它，也没有 `/demo/` 路由——这条边界由 v1.0.0 的 S12-AC02 断言保护，本组件不放宽它。

## 链路

```
浏览器 ──/demo/──────────► playground ──签名 POST /internal/tokens──► 预览服务
   │                          │  (集群内 HTTPS)                          │
   │                          └──示例文件 GET /demo/source/<name>◄───────┘
   └──/reader/#token=… ─► 阅读页 ──GET /v/{token}──► 302 ──► 阿里云 OSS 签名 URL
```

playground 携带仓库内示例附件（`go:embed`），并自己以集群内 HTTPS 提供源文件，因此**不需要向 OSS 预灌数据，也不需要公开读权限**。预览产物写入指定的 OSS 测试 bucket。

示例覆盖产品允许范围内的几类输入：PNG 与 PDF 走原样路径，Word/Excel/PowerPoint、金山 WPS 三件套与多页 TIFF 走转换路径。清单由 `demo/playground` 的内嵌目录决定，扩展名必须落在 [formats](formats.md) 的 18 个后缀内，单元测试对此有断言。

## 为什么集群内要 TLS

受保护的 `/internal/tokens` 要求真实 HTTPS 或可信代理头，`demo/adapter` 也强制 `APIOrigin` 为 https。dev 用一张自签证书（SAN 覆盖两个 Service DNS 名），证书、私钥与 CA 由基础设施预置的 ConfigMap 提供并挂载到两个 Pod；仓库内不含任何密钥。入口按集群现有惯例以 HTTP 暴露，因此 Ingress 需要 `nginx.ingress.kubernetes.io/backend-protocol: HTTPS`。

## 访问控制

`demo/adapter` 用 cookie 校验访问口令，未带口令的请求一律 401。演示者从 `/demo/enter?key=<口令>` 进入，playground 校验后写 cookie 再跳转到列表页。口令由基础设施侧放进 ConfigMap，不在仓库内。

前端只拿到 `{token, filename, preview_type, expires_at}`，拿不到源 URL、OSS 签名目标或任何密钥；playground 只持有 internal 角色密钥，没有 admin 能力。

## 部署

声明在 [`deploy/dev`](../../deploy/dev/kustomization.yaml)，用 `kubectl kustomize deploy/dev` 可完整渲染，PR 门禁会真实渲染两个 overlay 并检查其中没有 Secret 引用。

基础设施侧需要提供的取值：

| 载体 | 键 | 说明 |
| --- | --- | --- |
| ConfigMap `file-preview-server-runtime` | `CALLER_KEYS_JSON` | 含 internal 与 admin 角色密钥 |
| | `ALIYUN_OSS_*` 对应环境变量 | 测试 bucket 的 endpoint、region、bucket、前缀、凭据与签名 TTL，见 [storage-profiles](storage-profiles.md) |
| ConfigMap `file-preview-playground-runtime` | `PREVIEW_KEY_ID`、`PREVIEW_INTERNAL_KEY_BASE64` | 与上面 internal 角色密钥一致 |
| | `PLAYGROUND_ACCESS_KEY` | 演示访问口令，至少 16 字节 |
| ConfigMap `file-preview-tls` | `tls.crt`、`tls.key`、`ca.crt` | 自签证书，SAN 需包含 `file-preview-server` 与 `file-preview-playground` |
| Fleet / publisher | 镜像 digest、对外 host | 替换声明中的 `UNPINNED-SEE-RELEASE-RECORD` 与 `*.invalid` 占位符 |

OSS 测试 bucket 还需要配置 CORS，允许演示 host 作为来源并暴露 Range 相关响应头，否则 PDF 阅读器无法分段读取；具体要求见 [clients](clients.md)。

## 边界

playground 不进入生产与 test 环境，`deploy/` 的结构门禁会拒绝在 dev 之外的 overlay 声明它。它使用仓库内示例文件，不接触真实业务数据。集群内的实际 rollout 与健康状态由用户登记 Fleet 后以只读方式观测，本文档不代表任何环境已经部署成功。
