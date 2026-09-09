# 架构决策与权衡

本文件记录当前权衡；用户范围以 [PRD](../prd/product.md) 和 #36 为准。此前 #13 的安全/缓存契约继续适用，旧最大格式集合和小程序首版交付不再有效。

| 决策 | 依据与选择 | 代价 / 约束 | 权威落点 |
| --- | --- | --- | --- |
| A1：单服务与同 Pod 转换器 | 已确认独立服务和双容器形态；业务二进制不打包 Gotenberg | 随应用副本一起扩缩容，转换压力占用同 Pod 资源；应用和 sidecar 升级需联合验收 | [services](services.md)、[deployment](deployment.md) |
| A2：服务端 token 状态 | 已确认可撤销、可复用、有期限的随机 token | 依赖 Valkey；不是离线 JWT；状态丢失需重新签发，不能恢复旧快照复活撤销授权 | [data](data.md)、[quality](quality.md) |
| A3：HTTPS HMAC 与角色隔离 | #11 保留 HMAC、时间窗、nonce、双 key；无应用层 AES | 客户端要规范化签名和同步时间；nonce 存储不可当普通可丢缓存 | [apis](apis.md)、[security](security.md) |
| A4：同步导航解析 | 已确认唯一公开 token 导航入口和等待首个转换 | 首次请求承受下载/转换延迟，必须有界等待；不增加轮询任务 API、消息队列、SQL 任务系统 | [runtime](runtime.md)、[quality](quality.md) |
| A5：内容寻址与不可变 generation | PRD 内容 hash 目标；本轮细化条件发布与清理 | hash 不等于授权；对象代次防止旧 owner 删除新产物；profile/config 身份隔离，跨 profile 独立准备 | [data](data.md) |
| A6：仓库内 Web 接入与阅读 | #36 仅要求 H5/PC Web，复用一套阅读能力 | 测试适配层与生产路由分离；真实 Web E2E，静态原型/DOM 不替代；小程序首版取消 | [demo](demo.md)、[clients](clients.md) |
| A7：无历史兼容与受控升级 | #9 完整重构，旧入口/键不迁移；后续本服务升级仍须兼容分析 | 禁止以保留旧路由降低迁移成本；未来不兼容数据需版本隔离和明确回滚界限 | [apis](apis.md)、[deployment](deployment.md) |
| A8：Fleet 写入、项目只读观察 | 项目 AGENTS 的部署边界 | Agent 不能现场修改集群来绕过声明漂移；基础设施必须提供目标和运行证据 | [deployment](deployment.md) |

## 技术栈适用边界

GoFrame v2 的模块四层、手工装配、极薄 Controller 与统一错误处理适用于本服务；具体依赖版本和公司组件接入以 #1 的实际 go.mod 与库契约核验为准。当前 dev 已有 GoFrame 模块与运行实现，依赖按实际 go.mod 核验；不读取或复用项目外 worktree 作为依赖。

本项目已确认的 `content_sha256`、`storage_profile`、`cache_ttl`、`expires_at` 等 snake_case 字段优先于通用 lowerCamelCase 示例；不能擅自重命名。内部鉴权使用 HMAC，不能套用滑动会话 Bearer 模式；预览无效使用 HTTP 404，不能套用通用业务异常 HTTP 200；同步等待转换不能替换为 SQL 后台任务。需要这些变更时必须重新进行产品裁决。

文档原型是无构建依赖的 HTML/CSS/JavaScript 样稿，不是已选定或交付的 Taro/H5 工程。原型的 SVG、三页合成 PDF 排版和审查面板只用于设计验收：它们不扩大产品允许的上传格式，也不成为真实 PDF 引擎或生产用户功能。

## 放行与后续落实

#36 完成精确允许清单与当前计划修正，#37 实施允许集合及真实金山 WPS，#25 实际 Web 阅读，#26 镜像与部署交付。旧扩展格式永久取消，不能以转换器清单自动扩张。原型确认、文档 CI、服务 CI、镜像与实际部署分别取证；实际交付前不关闭 Milestone。

首版按 #28 使用自动验收，不追加人工真实验收或固定观察期；main 的用户 Web UI 合并与 Kubernetes 只读边界继续有效。
