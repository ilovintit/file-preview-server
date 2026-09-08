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
