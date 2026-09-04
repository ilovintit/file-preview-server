# 数据与存储

本服务没有业务 SQL 数据模型。token 和转换锁使用 **Valkey**（RESP 与 Redis 客户端兼容）；对象文件和转换缓存使用 **silo** 提供的 S3 兼容 API。代码或配置不得把新的服务端依赖写为 Redis 或 MinIO。

| 键 | 类型 | 数据与生命周期 | 用途 |
| --- | --- | --- | --- |
| `preview:token:{token}` | string | JSON 资源元数据；TTL 等于签发请求的必填 TTL | token 到资源的映射 |
| `preview:tokens:index` | sorted set | member 为 token；score 为创建时间 | 管理端分页与过期懒清理 |
| `redis-lock:preview-convert:{filename}` | 分布式锁键 | 600 秒 | Office 转换互斥；键前缀由既有锁组件保持兼容 |

`GET` 或列表发现主 token 键不存在时，必须清理索引中的残留 member。token 是服务端状态而非 JWT，所以撤销会立即阻止后续 `/v/{token}` 解析；已经由此前 302 取得的 silo 签名目标仍受其自身短时有效期控制。

Silo endpoint、bucket 和访问凭据只由部署环境注入；仓库不保存访问密钥或生产对象。当前 `dev` 尚未有 Valkey/Silo 配置或实际数据。
