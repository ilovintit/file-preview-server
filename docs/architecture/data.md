# 数据与存储

> 审查未放行：与任意源直跳、跨 profile 缓存及双 TTL 有关的原始条款存在 D1–D3 冲突，见 [审查裁决表](review.md)。裁决前不能据此开始相关实现。

本服务没有业务 SQL 数据模型。token、nonce、转换锁和缓存索引使用 **Valkey**（RESP 与 Redis 客户端兼容）；签名调用方提交任意 HTTPS 源 URL，转换缓存使用由 `storage_profile` 选择的 ObjectStorage 适配器。v1.0.0 仅有 `aliyun-oss` 与 `silo` 两个验收 profile；其他供应商不在本期范围。

| 键 | 类型 | 数据与生命周期 | 用途 |
| --- | --- | --- | --- |
| `preview:token:{token}` | string | JSON 资源元数据（源 URL、缓存 profile、文件名、内容 hash）；TTL 等于签发请求的必填 TTL | token 到受控资源的映射 |
| `preview:tokens:index` | sorted set | member 为 token；score 为创建时间 | 管理端分页与过期懒清理 |
| `preview:nonce:{key_id}:{nonce}` | string | 绝对过期时间为请求 timestamp + 301 秒，原子 SET NX；详见 [签名契约](apis.md) | 覆盖完整验签时间窗 |
| `preview:convert:{content_sha256}:{converter_version}` | string | 缓存对象引用与绝对过期时间；由必填 `cache_ttl` 延长，不得超过 `MAX_CACHE_TTL`（默认 86400 秒） | 内容寻址转换缓存 |
| `preview:convert-lock:{content_sha256}:{converter_version}` | 分布式锁键 | 覆盖单次转换的受控 lease | 同一内容只转换一次，其他请求等待结果 |

`GET` 或列表发现主 token 键不存在时，必须清理索引中的残留 member。缓存命中以内容 hash 和固定转换器版本为准；服务从签名调用方给出的源 URL 下载时必须重新计算 SHA-256 并拒绝不匹配内容，防止调用方错误或对象被替换。缓存过期后不可重用，物理对象由 profile 的生命周期规则或应用有界清理删除。token 是服务端状态而非 JWT，所以撤销会立即阻止后续 `/v/{token}` 解析；已经由此前 302 取得的对象存储签名目标仍受其自身短时有效期控制。

每个 storage profile 的 endpoint、bucket、访问凭据和生命周期策略只由部署环境注入；`MAX_CACHE_TTL` 默认 86400 秒，可在启动时用环境变量覆盖。仓库不保存访问密钥或生产对象。当前 `dev` 尚未有 Valkey 或对象存储配置/实际数据。

## 缓存时间与并发不变量

签发时间记为 `issued_at`，token 记录同时保存 `expires_at = issued_at + ttl` 和 `cache_expires_at = issued_at + cache_ttl`。缓存期限从签发开始，预览、等待或转换结束不能重新起算。相同缓存身份的签发以原子 max 更新绝对期限；即使对象未生成也保存该期限，不能因并发先后而缩短。缓存期限耗尽但 token 仍有效时的行为属于 D3 待裁决，不能自行默认 422、续期或重建。

有效缓存状态为 preparing → ready → expired/deleting；失败不得写 ready。上传完成并确认对象存在后才发布索引，上传成功但发布失败的孤儿对象由有界清理回收。锁必须以 owner 标识续租和释放，失去 lease 的旧持有者不能提交结果或删除新对象；等待者有总超时，失败时返回 5xx，不能永久等待或向用户返回锁冲突。对象采用不可变 generation 引用，删除前原子核对 generation 与期限，避免清理和续期竞争。

发送 302 前重新检查 token 是否仍有效/未撤销，并校验缓存未过期。缓存目标的签名到期时间取 token、缓存有效期及 provider 允许上限的最小值；不足一个可签名秒时不发无效重定向。对象清理不能早于已发签名的到期时间；固定上传日起算且无法续期的 provider 生命周期规则不能单独实现本规则，必须配合应用有界清理。多次签发可以持续续期，MAX_CACHE_TTL 限制单次请求的期限增量，不是对象总寿命。

Valkey token 写入与索引登记、撤销与索引移除必须原子处理；读取主键为准。列表只返回仍有效条目，过期成员过滤后再按约定分页；并发撤销可使不同页发生移动，列表不提供快照一致性保证。运行环境使用独立键前缀，缓存 profile 配置变化不得把同名 profile 指向别的 bucket 后继续复用旧索引。
