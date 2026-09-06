# 数据与存储

本服务没有业务 SQL 数据模型。token、nonce、转换锁和缓存索引使用 **Valkey**（RESP 与 Redis 客户端兼容）；签名调用方提交任意 HTTPS 源 URL，所有预览产物的缓存使用由 `storage_profile` 选择的 ObjectStorage 适配器。v1.0.0 仅有 `aliyun-oss` 与 `silo` 两个验收 profile；其他供应商不在本期范围。

| 键 | 类型 | 数据与生命周期 | 用途 |
| --- | --- | --- | --- |
| `preview:token:{token}` | string | JSON 资源元数据（源 URL、缓存 profile、文件名、内容 hash）；TTL 等于签发请求的必填 TTL | token 到受控资源的映射 |
| `preview:tokens:index` | sorted set | member 为 token；score 为创建时间 | 管理端分页与过期懒清理 |
| `preview:nonce:{key_id}:{nonce}` | string | 绝对过期时间为请求 timestamp + 301 秒，原子 SET NX；详见 [签名契约](apis.md) | 覆盖完整验签时间窗 |
| `preview:cache:{profile_identity}:{content_sha256}:{output_version}` | string | 缓存对象引用与绝对过期时间；由必填 `cache_ttl` 延长，不得超过 `MAX_CACHE_TTL`（默认 86400 秒） | 内容寻址预览缓存 |
| `preview:prepare-lock:{profile_identity}:{content_sha256}:{output_version}` | 分布式锁键 | 覆盖单次下载/原样存储或转换准备的受控 lease | 同 profile/内容/输出版本只准备一次，其他请求等待结果 |

`GET` 或列表发现主 token 键不存在时，必须清理索引中的残留 member。缓存命中以 profile 配置身份、内容 hash 和输出版本为准；服务从签名调用方给出的源 URL 下载时必须重新计算 SHA-256 并拒绝不匹配内容，防止调用方错误或对象被替换。缓存过期后不可重用，物理对象由 profile 的生命周期规则或应用有界清理删除。token 是服务端状态而非 JWT，所以撤销会立即阻止后续 `/v/{token}` 解析；已经由此前 302 取得的对象存储签名目标仍受其自身短时有效期控制。

每个 storage profile 的 endpoint、bucket、访问凭据和生命周期策略只由部署环境注入；`MAX_CACHE_TTL` 默认 86400 秒，可在启动时用环境变量覆盖。仓库不保存访问密钥或生产对象。当前 `dev` 尚未有 Valkey 或对象存储配置/实际数据。

## 缓存时间与并发不变量

签发时间记为 `issued_at`，token 记录同时保存 `expires_at = issued_at + ttl` 和 `cache_expires_at = issued_at + cache_ttl`。缓存期限从签发开始，预览、等待或转换结束不能重新起算。相同缓存身份的签发以原子 max 更新绝对期限；即使对象未生成也保存该期限，不能因并发先后而缩短。签发必须 cache_ttl ≥ ttl，因此正常状态下缓存期限不会先于 token 到期。若共享状态异常丢失期限/对象，按质量故障恢复规则处理，不把异常作为延长授权或重新起算期限的依据。

有效缓存状态为 preparing → ready → expired/deleting；失败不得写 ready。上传完成并确认对象存在后才发布索引，上传成功但发布失败的孤儿对象由有界清理回收。锁必须以 owner 标识续租和释放，失去 lease 的旧持有者不能提交结果或删除新对象；等待者有总超时，失败时返回 5xx，不能永久等待或向用户返回锁冲突。对象采用不可变 generation 引用，删除前原子核对 generation 与期限，避免清理和续期竞争。

发送 302 前重新检查 token 是否仍有效/未撤销，并校验缓存未过期。缓存目标的签名到期时间取 token、缓存有效期及 provider 允许上限的最小值；不足一个可签名秒时不发无效重定向。对象清理不能早于已发签名的到期时间；固定上传日起算且无法续期的 provider 生命周期规则不能单独实现本规则，必须配合应用有界清理。多次签发可以持续续期，MAX_CACHE_TTL 限制单次请求的期限增量，不是对象总寿命。

Valkey token 写入与索引登记、撤销与索引移除必须原子处理；读取主键为准。列表只返回仍有效条目，过期成员过滤后再按约定分页；并发撤销可使不同页发生移动，列表不提供快照一致性保证。运行环境使用独立键前缀，缓存 profile 配置变化不得把同名 profile 指向别的 bucket 后继续复用旧索引。

## 逻辑记录与物理键边界

上表键名为逻辑模板；部署命名空间必须隔离环境与数据 schema。花括号表示模板参数，不表示已经选定 Valkey Cluster hash tag。本期尚未承诺分片集群拓扑；实现需要原子更新的相关键必须处于可由同一原子操作访问的部署范围，不能在落地时引入跨槽多键写再假设其原子性。

| 记录 | 必须保存的信息 | 生命周期与敏感性 |
| --- | --- | --- |
| TokenGrant | token、创建/到期时间、caller 标识、源 URL、声明 hash、文件名、缓存 profile、缓存绝对期限、可选管理 metadata | 有效期只从签发起算；源 URL 不进入用户 DTO；token 只经最小展示 DTO/fragment 传递，两者均不写日志 |
| TokenIndex | token 与创建时间排序值 | 只作索引，不能替代主记录判活；清理残留后才形成有效列表 |
| CacheRecord | 状态、绝对期限、输出版本、profile 配置身份、generation、对象引用和实际媒体类型 | preparing 不可签名；ready 必须经过发布检查；deleting 不可续期复活 |
| ObjectMaintenance | 不可变对象候选 ID、profile、generation、清理时间、认领状态与最近失败 | 独立于可过期缓存主键保留，直到确认删除或证明未产生对象，防止索引 TTL 消失后找不到孤儿 |
| NonceEntry | caller key_id + nonce 的占用和过期时间 | 签名防重放状态，不是可任意驱逐的普通缓存 |

转换产物 output version 必须随 Gotenberg 镜像/固定转换参数及影响输出的字体组合变化；不由用户输入任意 converter_version。`profile_identity` 由启动配置中的 profile 名称与配置代次共同确定；不同 profile 或代次的缓存、准备锁、期限意图和清理记录全部隔离。PDF/安全图片原样存储使用固定 raw-v1 输出版本；转换产物使用含转换器/字体/固定参数的版本。profile 配置重定向至其他 bucket 时必须更换配置代次。调用方文件名用于显示与格式初筛，输出媒体类型从受检内容确定，避免 filename 与字节不一致时错误复用。

## 原子操作与部分成功

1. 签发：以不存在条件保存随机 token，并原子登记索引及相关缓存期限意图；若发生 token 碰撞重新生成，不能覆盖已有授权。缓存期限意图在尚未生成对象时也必须保存。相关 profile 内的期限意图与 token/索引需要处于同一可原子更新的键范围。
2. 发布：分配不可变对象候选 ID，先登记维护记录，再上传；上传确认后，以 owner/generation/期限条件发布 ready。上传结果未知也保留候选记录，清理者可对确定对象名幂等删除，不能靠列全桶猜测孤儿。
3. 命中：读取有效 CacheRecord 并在使用前确认对象引用可用；HEAD/读检查与最终客户端请求间仍有故障窗口，不能宣称强一致端到端交付。外部误删按 quality 恢复，不返回已知失效引用。
4. 清理：先条件认领旧 generation，逻辑失效后再物理删除；续期或新产物不能被旧清理者覆盖。删除失败保留维护记录与退避时间，成功或 provider 确认对象不存在后才清理记录。
5. 撤销：删除主 token 与索引形成线性化点；并发在途预览的有效性读取顺序见 runtime，不能把“立即撤销”解释为回收已发送 URL。

Valkey 与对象存储没有跨系统事务，本服务采用条件状态迁移与补偿清理，不能用无限重试伪装原子提交。状态丢失与恢复界限见 [quality](quality.md)，升级/回滚与配置身份见 [deployment](deployment.md)。

## S01 授权连续性（#15）

S01使用单主Valkey与独立命名空间，要求noeviction；应用检查服务器run_id和授权状态marker。marker丢失、服务器运行身份变化或不安全淘汰配置时关闭受保护操作。首次建立安全状态与状态恢复使用新随机epoch，并等待完整601秒重放残余窗口后允许新签发；正常连接/应用重启在同一连续marker/run_id下不重新等待。时钟由内部构造注入以供CI控制，不提供测试HTTP端点或生产跳过开关。

实际token、nonce、活跃索引置于当前epoch下，随机token仍仅为32位hex；所有写入Lua原子校验当前marker。新epoch不查询旧epoch授权，避免恢复旧快照复活已撤销token。运行中的单键恶意恢复不在正常备份恢复协议内；运维恢复必须建立新授权epoch/覆盖重放窗口，不能把授权状态当普通缓存任意导入。

索引以到期时间作score以便有界清理，列表按记录创建时间与token排序，token写入和索引登记原子完成。过期主键/索引可清理；nonce绝对期限按签名timestamp窗口保留，不能用固定300秒从收包起算。epoch缓存不是本地授权真相，数据变更仍以Valkey原子检查为准。
