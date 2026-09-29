# 存储 profile 实施契约（S04）

本文件记录存储适配的实施结构、配置和验收边界；业务期限/原子性以 [data](data.md) 为准，部署以 [deployment](deployment.md) 为准。

## 共享流程与隔离

Domain 定义 `ObjectStorage` 的 Put、Stat、Delete、SignGet 和 Check 端口。Infrastructure 的 `PreviewStore` 共用下载校验、Office 转换、lease、不可变对象、发布和发送前授权重检；OSS 和 silo 仅实现供应商操作。`ProfilePreparer` 按 token 的 `storage_profile` 选择已装配实例，两实例共享每进程 4 执行槽和 32 等待者的总上限，不因增加 profile 翻倍。

OSS 身份保持既有 endpoint/bucket/prefix 的 SHA-256，避免无必要地使已有 raw-v1 缓存失效。silo 身份包含 profile 名、endpoint、bucket、prefix 与配置代次；SDK 密钥滚动不改变内容身份。期限意图、缓存和 lease 都使用所选身份 + 输出版本 + 内容 hash，不跨 profile 转换后复制。

维护记录沿用现有独立 hash/zset，不丢弃旧配置的对象记录。每个启用身份使用独立、有界扫描游标，只认领当前身份的候选；旧配置/停用 profile 的积压不能堵住另一个 profile。每批最多认领 32 个对象，仍以 generation/期限条件更新，删除失败保留 30 秒重试间隔。停用配置的旧对象保留记录，需要恢复原配置或由有权限的运维按既有维护记录收尾，不能使用新 bucket 的凭据猜测删除目标。

## silo 运行配置

启用 `STORAGE_PROFILES=silo`，或与 `aliyun-oss` 逗号分隔组合。空配置的支持 profile 在签发时返回 503，未知 profile 返回 422；部分/非法配置拒绝启动。请求 body 不能覆盖这些配置。

| 环境变量 | 含义 |
| --- | --- |
| `SILO_ENDPOINT` | SDK 上传、Stat、删除的 S3 API Origin；支持 HTTPS 或受控内网 HTTP |
| `SILO_PREVIEW_ENDPOINT` | 浏览器签名 Origin，必须 HTTPS；未设置则使用 SDK endpoint，且它也必须 HTTPS |
| `SILO_REGION` | S3 签名 region，未设使用 `us-east-1` |
| `SILO_BUCKET` / `SILO_PREFIX_BASE` | 目标 bucket 与对象前缀；不自动创建生产 bucket |
| `SILO_CONFIG_GENERATION` | 可选显式配置代次；切换 bucket/endpoint/prefix 也会自动改变身份 |
| `SILO_ACCESS_KEY_ID` / `SILO_ACCESS_KEY_SECRET` | 环境注入的读写删除凭据，不写入仓库或响应 |
| `SILO_SECURITY_TOKEN` | 可选临时凭据 token |
| `SILO_SIGNED_URL_MAX_TTL_SECONDS` | 必填，1–604800；最终签名仍受 token/缓存绝对期限限制 |

silo 使用固定的 S3 Go SDK，兼容 import 路径包含 `minio-go`，不表示部署 MinIO 服务端。使用 path-style URL 和 SigV4，SDK endpoint 与预览 endpoint 独立构造；不能签名后改 Host。预览 Origin 必须转发到同一 bucket，并保留用于 SigV4 校验的 Host 和路径。SDK 单次网络操作最长 8 秒、禁用自动重试，仍受请求总 deadline 约束。

## 就绪与权限

`/readyz` 检查声明的 profile 是否完整装配、Internal/Admin key 是否具备、Valkey 授权状态是否连续，以及所选 bucket 的只读可达性；配置了 Gotenberg 时也检查其 health。缺失依赖返回通用 503，不暴露凭据/内部错误。首次授权状态冷启动仍遵守 601 秒保护窗口，不能为 ready 跳过。

OSS 就绪对已授权前缀内的保留探测对象做 HEAD：成功或供应商明确返回 `404/NoSuchKey` 才通过；`NoSuchBucket`、403 和无法分类的 404 均失败。不能只凭 HTTP 404 猜测桶存在，也不要求额外的 GetBucketLocation 权限。silo 使用认证 BucketExists。对象操作需要所选前缀的写入、读取/HEAD 和删除权限。liveness 不依赖外部存储。就绪检查不修改生产 CORS、bucket、对象或 ACL。

## CI 环境与证据边界

CI 使用 per-run 的真实 silo：Docker Hub 公开镜像 `pgsty/silo:RELEASE.2026-09-03T13-18-01Z`，按 digest 固定（见 [integration.yml](../../.github/workflows/integration.yml)），启动时显式传入 `server /data`。服务只绑定 runner 的 loopback，fixture 凭据仅用于该隔离服务。每个 fixture 创建自己的测试 bucket，结束后删除；不需要生产 silo 凭据。

集成测试默认以 silo 作为存储 profile。阿里云 OSS 适配器用例只在提供 `PREVIEW_CI_ALIYUN_OSS_*` 测试 bucket 配置时运行，否则以明确原因跳过；未配置 OSS 时 fixture 只声明 silo profile。

测试 TLS 代理向真实 silo 转发请求，保留 Host，不改写签名或 CORS。silo bucket 的 CORS 只允许当前测试预览 Origin；实际 OSS 沿用用户明确批准的无凭据通配 CORS。Chromium 对这两个真实链路执行图片解码、PDF Range 读取/原生阅读器检查。TLS 只信任测试证书的精确指纹，OSS 公共证书继续正常校验。

7 原样 + 6 Office 在两 profile 重用同一 fixture/断言；Office 检查页数与关键内容，不仅文件非空。另验同 hash 独立准备、期限/撤销/清理隔离、配置切换、真实签名到期、删除失败恢复与就绪。具体 head/run/job 以 #18 / PR #33 为准；CI fixture 运行不是生产部署或微信真机验收。
