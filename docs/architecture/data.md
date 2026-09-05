# 数据与存储

本服务没有业务 SQL 数据模型。token、nonce、转换锁和缓存索引使用 **Valkey**（RESP 与 Redis 客户端兼容）；签名调用方提交任意 HTTPS 源 URL，转换缓存使用由 `storage_profile` 选择的 ObjectStorage 适配器。v1.0.0 仅有 `aliyun-oss` 与 `silo` 两个验收 profile；其他供应商不在本期范围。

| 键 | 类型 | 数据与生命周期 | 用途 |
| --- | --- | --- | --- |
| `preview:token:{token}` | string | JSON 资源元数据（源 URL、缓存 profile、文件名、内容 hash）；TTL 等于签发请求的必填 TTL | token 到受控资源的映射 |
| `preview:tokens:index` | sorted set | member 为 token；score 为创建时间 | 管理端分页与过期懒清理 |
| `preview:nonce:{key_id}:{nonce}` | string | TTL 300 秒 | 防止内部签名请求重放 |
| `preview:convert:{content_sha256}:{converter_version}` | string | 缓存对象引用与绝对过期时间；由必填 `cache_ttl` 延长，不得超过 `MAX_CACHE_TTL`（默认 86400 秒） | 内容寻址转换缓存 |
| `preview:convert-lock:{content_sha256}:{converter_version}` | 分布式锁键 | 覆盖单次转换的受控 lease | 同一内容只转换一次，其他请求等待结果 |

`GET` 或列表发现主 token 键不存在时，必须清理索引中的残留 member。缓存命中以内容 hash 和固定转换器版本为准；服务从签名调用方给出的源 URL 下载时必须重新计算 SHA-256 并拒绝不匹配内容，防止调用方错误或对象被替换。缓存过期后不可重用，物理对象由 profile 的生命周期规则或应用有界清理删除。token 是服务端状态而非 JWT，所以撤销会立即阻止后续 `/v/{token}` 解析；已经由此前 302 取得的对象存储签名目标仍受其自身短时有效期控制。

每个 storage profile 的 endpoint、bucket、访问凭据和生命周期策略只由部署环境注入；`MAX_CACHE_TTL` 默认 86400 秒，可在启动时用环境变量覆盖。仓库不保存访问密钥或生产对象。当前 `dev` 尚未有 Valkey 或对象存储配置/实际数据。
