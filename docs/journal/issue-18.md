# Issue 18 — S04 silo parity

## 起点

- 前序 #17 / PR #32 已合入 dev `9714a7d67859898079c71a692c24aa8ab6a66da9`，候选 `756c7a8e2894c339923a720c5818a50336de9198` 的 CI10684 三个 job 全绿。
- 本 Issue 独立工作区 `.worktrees/18`，分支 `codex/18-silo-profile-parity`，不修改其他项目。
- 当前实现只装配 OSS：Application 显式拒绝 silo，Valkey 期限身份只绑定 OSS，OSS 准备/清理流程直接依赖 OSS SDK。不能仅替换上传函数就声称 profile 隔离。

## 计划

1. 对 silo 签发/预览添加测试先行断言，在同一草稿 PR 取得功能缺失的 CI 红色证据。
2. 提取公共准备流程与存储端口，OSS/silo 分别实现上传、HEAD、签名、删除；统一的有界调度、转换、lease 和期限逻辑不复制。
3. 期限/锁/维护记录按 profile/config 身份隔离；清理只认领当前配置拥有的记录，防止不同 profile 提前消费他人的清理候选。
4. per-run 真实 silo 容器，内网传输 + 仓库测试 TLS 代理提供浏览器 HTTPS 目标；不降低外部短链 HTTPS 要求，不索要生产凭据。
5. 双 profile 的原样/六 Office、缓存隔离、代次、CORS/Range、短链真实到期和删除失败恢复均在 PR CI 验证后才合并。

## 环境调查

Harbor ci-cache/library 首页未发现 silo 镜像；开始按用户已授权的镜像同步方式缓存上游 `pgsty/silo:RELEASE.2026-09-03T13-18-01Z`，最终 digest 待同步后记录。上游默认 CMD 为 `silo`，需要为 CI 固定 server `/data` 启动参数，不把缺省帮助输出当作服务健康。

同步已完成：上游 amd64 原图 `sha256:885275e0f42acfdf80304c577d2e46c2c3978a276619d60b9eedd7486f104b30` 保留在本地 Harbor 同版本 tag。CI 派生仅将 CMD 固定为 `["server","/data"]`，没有新增/替换文件层，得到 `reg.shw.top/ci-cache/ci-silo@sha256:15099b2b174fc842a9b323ecfc6fe1c506a2b67ff23e1c49cf4464675f367b0d`（tag `preview-20260903-server`）；原镜像 tag 不被覆盖。镜像配置已回读核实。

## 测试先行证据

PR #33，head `39678f93408fe4518c8490830182fe8679bb9035`，CI10700/job18229：`TestTC_S04_AC01_SiloPreviewContract` 在签发 silo 时失败 `expected 200/code0, got 503/code50301`，证明当前缺少该 profile 装配。前序 S03 的合并 dev CI10693 亦成功。

首轮实现提取 domain ObjectStorage 端口与公共 PreviewStore，OSS SDK 保留原签名行为；silo 使用 S3 SDK v7.0.95（仅客户端，不部署 MinIO 服务端）。两 profile 统一调度上限、转换、lease、期限和签名前复查。配置/签发身份按 profile 分开，清理按当前身份过滤。仍需补完整矩阵与队列隔离边界，当前不可合并。
