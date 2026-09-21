# HTTP API 契约

以下为 Issue #1 按当前产品真相重构的目标契约，尚未在 `dev` 注册：

| 端点 | 认证 | 结果 |
| --- | --- | --- |
| `POST /internal/tokens` | Internal role 的 HTTPS、HMAC 签名、防重放 | 校验 HTTPS 源 URL、缓存 profile、内容 hash、文件名、双 TTL，返回 token 与 `expires_at` |
| `POST /admin/tokens/query` | Admin role 的 HTTPS、HMAC 签名、防重放 | 接收 `limit`/`offset`，返回有效 token 分页列表 |
| `POST /admin/tokens/{token}/revoke` | Admin role 的 HTTPS、HMAC 签名、防重放 | 幂等撤销 token |
| `GET /v/{token}` | token 本身 | v1 新接入唯一支持的公开导航 URL。图片/PDF 或转换后 PDF 的 302；无效/过期/撤销均为 404 |
| `GET /livez` | 无 | 仅供编排存活探针；进程可服务时为 200 |
| `GET /readyz` | 无 | 仅供编排就绪探针；Valkey、Gotenberg 与所需 storage profile 就绪时为 200，否则为 503 |

签发 JSON body：`url` 为任意 HTTPS 源 URL；因调用方签名已验证身份，服务不对源域名作 allowlist 限制。`storage_profile` 必须是 `aliyun-oss` 或 `silo`，作为所有预览产物的受控缓存目的地；`content_sha256` 为 64 位小写十六进制 SHA-256，`filename` 必须含 [支持扩展名](formats.md)，`ttl` 为 60–86400 秒的必填整数，`cache_ttl` 为满足 `ttl ≤ cache_ttl ≤ MAX_CACHE_TTL` 的必填整数，不满足返回 422，`MAX_CACHE_TTL` 默认 86400 秒且仅可通过启动环境覆盖，`metadata` 可选且仅用于管理展示。服务端不接受调用方传入对象存储 endpoint、bucket 或凭据。签发成功的 data 为 `token` 和 Unix 时间戳 `expires_at`。

## 内部签名请求

所有内部和管理请求都必须为 HTTPS `POST`，生产环境优先使用 mTLS，带 `X-Preview-Key-Id`、`X-Preview-Timestamp`、`X-Preview-Nonce` 和 `X-Preview-Signature`。body 是普通 JSON。`X-Preview-Signature` 是调用方密钥对 `METHOD + LF + PATH + LF + key_id + LF + timestamp + LF + nonce + LF + SHA-256(raw_body)` 的 HMAC-SHA256。

服务先校验 HTTPS/mTLS、300 秒时间窗、key role、签名和未使用 nonce，再校验业务 body；任一失败统一 401，避免泄露失败原因。nonce 必须原子占用（SET NX）并保留至该请求最后可接受时刻之后：`expires_at = timestamp + 301`（Unix 秒，覆盖包含边界的 ±300 秒窗口），使用服务端当前时间计算剩余 TTL。未来 300 秒的 timestamp 最多需要保留 601 秒；固定保留 300 秒会允许其在验签窗口内再次重放。验签成功后先占 nonce，再执行业务校验；重试必须使用新 nonce。Valkey 故障返回 503，不能绕过防重放。调用方配置 current/next HMAC key 与 `key_id`，服务同时接受两把激活 key 用于轮换；响应为普通 HTTPS JSON。

## 预览跳转契约

`GET /v/{token}` 仅支持正常 HTTP 导航：所有文件在缓存未命中时先由服务下载并校验内容，PDF/安全图片原样存储、其余支持格式转换为 PDF，最后以 302 和 `Location` 指向所选 profile 的短时签名 HTTPS URL，禁止返回源 URL。它不返回文件字节、JSON、跨域可读取的 `Location`，也不提供客户端解析跳转目标的替代接口。302 响应必须发送 `Cache-Control: no-store` 与 `Referrer-Policy: no-referrer`。

签名目标 URL 的有效期不得超过 token 剩余 TTL。撤销只保证后续 `GET /v/{token}` 返回 404：此前已经取得目标 URL 的客户端可访问到该签名 URL 自己过期。调用方只能缓存 `/v/{token}`，不能将目标 URL 写入业务数据、分享链路或日志。

最终对象存储必须以 HTTPS、正确的媒体类型和内联文档处置方式响应。需要由 JavaScript PDF 阅读器读取的对象还需按 [clients.md](clients.md) 配置精确 CORS 和 Range 支持。

## 微信小程序调用方契约

微信小程序 demo 不直接调用 `POST /internal/tokens`，也不接收资源 URL。仓库内 demo API 以 fixture 附件 ID 为输入，完成签名调用后返回 `{token, filename, preview_type, expires_at}`。`preview_type` 仅为 `image` 或 `pdf`；PDF 与 Office 均为 `pdf`。

小程序 demo 将该 DTO 传入本仓库 H5 `web-view` 的 URL fragment，H5 清除 fragment 后才拼接 `GET /v/{token}`。这不是本服务新增的公开端点，也不改变内部 token 签发 API；demo API 与 H5 均由本仓库维护。`wx.previewImage`、`wx.downloadFile` 和 `wx.openDocument` 不属于 v1.0.0 对 `/v/{token}` 的兼容承诺。

没有历史路由、兼容开关或测试辅助端点。规范 JSON 响应和错误形态由服务全局中间件统一处理；302 不包装为 JSON 成功响应。

## 签名规范化与错误边界

规范串以单字节 LF（0x0A）连接六段，无末尾换行；上文 LF 表示换行，不能签入反斜杠和字母 n。METHOD 为大写 POST；PATH 为路由中的绝对路径，不含域名，当前 POST 不接受 query；raw_body 为实际发送的 UTF-8 字节，不重新序列化 JSON。SHA-256(raw_body) 与 HMAC 结果均编码为小写 hex；timestamp 使用十进制 Unix 秒，nonce 为 128-bit 随机值的小写 hex。key_id 与请求头值不得含控制字符或额外空白，验签采用常量时间比较。跨语言客户端与服务端需在 #1 CI 共用固定签名向量。

业务 body 校验失败为 422；鉴权失败为 401；缺失角色密钥、已支持但未配置的 profile、Valkey 故障为 503。未知 profile 属于参数错误 422。签发成功只表示 token 已保存，不代表源下载或转换已经成功；这些失败在预览请求中反馈。撤销成功不代表已有签名 URL 被撤回。重放拦截不是业务幂等：新 nonce 重试签发可能创建新 token，调用方须考虑超时后重复签发；撤销天然幂等。具体 JSON DTO/错误码、分页边界与排序在 #1 接口实现前以此文档补齐，并保持上述 HTTP 语义。

## 请求字段与返回边界

| 字段 | 类型与验证 | 消费方 |
| --- | --- | --- |
| url | HTTPS URL；解析时拒绝无 host、控制字符或内嵌 userinfo；所有重定向继续满足 HTTPS 约束 | 只供服务下载，不返回 demo DTO |
| storage_profile | 固定支持名称，是否实际启用由部署环境决定 | 所有文件的受控存储；缓存与锁按 profile/config 身份隔离 |
| content_sha256 | 固定编码的声明内容 hash；下载后重新计算 | 内容身份与完整性校验，不是授权凭据 |
| filename | 含支持扩展名的显示文件名；不得作为本地路径，拒绝控制字符/路径穿越；响应头安全编码 | 展示、格式初筛与输出文件名 |
| ttl / cache_ttl | 必填整数，边界见前文；必须 cache_ttl ≥ ttl | token 与缓存期限，不能由客户端展示状态续期 |
| metadata | 可选管理展示信息，不参与签名身份/数据范围/格式选择 | 仅管理员展示；结构、体积限额在 #1 编码前细化，不向 H5 透传 |

请求、文件名与 metadata 都需有资源上限，预算归 [quality](quality.md)。签发时不接收转换器参数、对象存储 endpoint/bucket/密钥或客户端伪造的操作者 role；拒绝未知控制字段，不能悄悄透传给 provider。业务 body 只允许 JSON，不解析表单兼容格式。

Internal/Admin 的 JSON envelope 采用 `traceId`、`code`、`message`、`data`，成功 code 为 0；内部 data 的已确认字段保持 snake_case。签发 data 为 `{token, expires_at}`；撤销 data 不返回原文件信息，也不暴露“此前是否存在”。错误 envelope 保留 HTTP 状态语义，不能用 HTTP 200 掩盖 401/404/422/503。具体业务 code 数值须由 #1 与实际错误组件契约对齐，不能从数字范围推断错误类型。

Admin 查询只接受 limit/offset 分页参数，不能借 metadata 动态构造过滤器、任意排序表达式或跨角色签发。响应只展示活跃 token 所需管理信息，不得返回访问密钥或未脱敏内部异常；最终 limit 上限、排序/DTO 字段在 #1 编码前补齐并审查。当前不新增分页 cursor、状态查询或事件订阅接口。

## 导航响应与兼容

预览成功仅为 302，失败使用前述真实 HTTP 状态；H5 按 clients 处理。`/v/` 的 404 只表示 token 无效，不直接透传源站/存储 404；源文件丢失属于预览基础设施或来源故障。等待/转换失败不会返回“任务已接受”或部分 PDF。

Range 与最终文件 HEAD 属于目标签名资源的读取能力；本服务没有新增代理字节流、multipart Range 或签名目标解析 API。H5 的诊断 GET 不读取 Location，不能被扩展为绕过 token 的公开解析接口。客户端不能把已有签名 URL 当作刷新入口。

v1 不接受旧 HTTP 路由、旧静态 Bearer、旧数据键或 AES 信封兼容。未来本服务自己的契约变更需显式评估客户端与滚动升级，而非把“无历史兼容”理解为后续可任意改变已发布协议。

## S01 接口实施参数（#15）

- 管理查询 body 的 `limit` 可省略，默认20、范围1–100；`offset` 默认0、非负整数；未知字段拒绝。按 `created_at` 倒序，同秒按 token 字典序倒序。data 为 `{items, total, limit, offset}`；先判活/清残留再分页，items 为 `{token, filename, storage_profile, created_at, expires_at, metadata}`，不含源 URL。并发变更不保证分页快照。
- 撤销 body 为 `{}`，data 为 `{}`；合法格式但不存在的 token 也成功。token 路径仅允许32位小写hex。
- JSON body 上限64KiB，filename上限255字节，metadata如提供必须为JSON object且编码上限4096字节。所有数值按整数处理，不接受null/小数代替必填TTL；拒绝重复JSON字段和尾随JSON。
- 响应 success 为 HTTP200/code0；401/code40101、404/code40401、422/code42201、503/code50301 对应中文通用消息，由gcode detail映射HTTP，不在错误体泄漏输入、nonce命中原因或基础设施配置。
- 受保护API必须实际TLS，或来自显式可信代理CIDR且由该入口提供唯一的 `X-Forwarded-Proto: https`；不信任任意客户端转发头，不接受签名POST的query或编码路径变体。
- S01只交付授权生命周期。`GET /v/{token}` 对无效授权返回404，对有效但尚未接入文件能力返回503；不会裸跳源站。完整资源预览由S02后续交付，S01的readyz不会谎称整个预览能力就绪。
