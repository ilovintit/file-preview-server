# 产品基线综合审查

审查载体：[Issue #13](https://git.shw.top/shw-project/file-preview-server/issues/13)。输入为 `dev@52d52a0`，最新已确认裁决为 #11（PR #12 已合并）。当前结论：**未放行**，D1–D3 待用户裁决；本轮修正不代表 #1 已实现。

## 阻断与需用户裁决

| 编号 | 矛盾与影响 | 待选契约 | 状态 |
| --- | --- | --- | --- |
| D1 | 任意 HTTPS 源 URL 无服务持有的签名凭据，PDF/图片直跳时不能保证目标到期时间或 hash 校验。HMAC 只证明调用方身份。 | 推荐所有文件下载校验，PDF/图片原样存入所选 profile 后签短链；或保留直跳，明确源签名有效期与内容一致性由调用方负责。前者扩大缓存范围与首读开销，后者降低服务保证。 | 已询问，未裁决 |
| D2 | 全局 hash 缓存键不包含 profile；同内容指定两种 profile 时，无法同时保证目的 profile 与单份全局产物。 | 推荐 profile 内 hash 去重和锁；或全局转换去重后按请求 profile 复制产物并编排失败恢复。converter_version 是输出版本隔离，不是另一种资源身份。 | 已询问，未裁决 |
| D3 | 双 TTL 独立，cache_ttl 小于 ttl 时，token 尚有效而缓存过期，重复预览行为未定义。 | 推荐签发要求 cache_ttl ≥ ttl；或允许提前过期后返回 422 并重新签发；或允许 token 有效期间重建并重新起算缓存期限（改变当前签发起算规则）。 | 已询问，未裁决 |

裁决前，PRD/API/数据/客户端中与 D1–D3 对应的原始规则仅用于展示冲突，不能作为已收敛实施契约。本轮不猜测答案、不把默认选项当作用户确认。裁决后必须同时更新上述文档、领域存储接口、原型索引、测试映射、部署 CORS/清理范围及 #1 摘要，再全量复审。

## 确定性修正与双向映射

| 需求 / 证据 | 原型 / 架构结果 | 复审结论 |
| --- | --- | --- |
| PRD 仓库内 demo，旧 clients 却称无 UI | [小程序入口](../design/wechat-miniprogram/index.html)、[H5 入口](../design/preview-h5/index.html)、[demo](demo.md)、[clients](clients.md) | 补齐设计入口；服务端 Admin 无 UI，不扩张管理页面范围 |
| 详情通过附件 ID 获取 | 小程序模拟详情读取后再进入 H5；原型只有合成 ID，不包含真实密钥/URL | 字段与动作可回指 PRD；真实 DTO/网络鉴权由 #1 验收 |
| 300 秒双向时间窗，但 nonce 固定 300 秒 | [apis](apis.md)、[data](data.md)、[security](security.md) 定义 timestamp + 301 的绝对过期、原子占用、依赖失败关闭 | 未来 timestamp 在旧 nonce 过期后仍可验签的漏洞已在设计中消除，服务测试待 #1 |
| HMAC 规范串编码不明；Controller 自相矛盾 | 明确 LF/raw bytes/hex/role/错误；[services](services.md) 中间件验签、Controller 只调 UseCase | 仅架构定义，不能替代跨语言 CI 签名向量 |
| 缓存续期、锁失效、发布/清理并发 | data 补原子 max、owner、generation、发布前 token 重检、孤儿回收与清理边界 | D2/D3 未定部分显式保留；无新增任务 API 或事件总线 |
| 图片 onerror 不能区分 HTTP 状态 | clients 定义 H5 与 /v/ 同源、失败后只对原 token URL 分类；opaque redirect 不读 Location | 404/422/5xx/网络反馈进入 H5 样稿；真实浏览器与真机证据待 #1 |
| 单一 Go 镜像与 Gotenberg sidecar 混写 | [deployment](deployment.md) 固定双独立镜像、可信 TLS 入口、Fleet 记录与回滚、日志脱敏 | 无部署声明/目标/集群健康证据；仍属 #1 |
| 格式矩阵把目标当支持保证 | [formats](formats.md) 标注目标集合、官方资料核查与固定版本 fixture 验收边界 | 官网列表不能证明实际镜像、字体或渲染成功 |

## 验证与可改进

- 本轮 CI 增加文档本地链接与原型内联 JavaScript 语法检查；不改变原有服务模块出现后重型层强制真实测试的规则。CI 成功只能证明这些实际运行的检查。
- 自动浏览器的 URL 安全策略拒绝访问本地原型文件。本轮没有渲染、点击、E2E/VRT 或真机通过证据；保留原型视觉/交互人工审阅项，不能声称高保真验收已完成。
- Go module、服务、真实 demo、两种存储的集成测试、镜像/部署和平台验收均由 [Issue #1](https://git.shw.top/shw-project/file-preview-server/issues/1) 交付，属于已知实施缺口，不伪装成此次文档 PR 的通过结果。
- 原生 App 明确延期到下一版本，尚未排期建交付 Issue；产品范围确定后由 version/split 建单。本轮没有其他静默延期项。
- #1 进入实现前细化管理 API 分页/DTO/错误码、资源预算和固定 CI fixture；它们不改变本轮已确认角色或公开路由。

只有 D1–D3 落文档、全部基线阻断清零、原型审阅完成且当前 PR CI 取证后，才可放行并进入 `/version`。
