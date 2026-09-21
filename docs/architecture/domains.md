# Preview 领域

`PreviewFile` 是核心领域对象，负责由 `filename`、受检内容类型及 hash 校验结果判定 PDF、可浏览器直接显示的图片、需转换的产品允许格式或不支持格式。唯一允许集合以 [formats.md](formats.md) 为准。领域层定义以下抽象而不绑定具体驱动：

- `TokenStore`：保存、读取、列出与删除预览 token。
- `ConvertLock`：同一转换资源的互斥获取与安全释放。
- `ObjectStorage`：按配置 profile 上传/删除预览产物、执行缓存生命周期并签名访问；v1 仅实现阿里 OSS 与 silo 两个 profile。所有文件从签名调用方提交的 HTTPS URL 下载校验，PDF/安全图片原样上传，其余支持格式转换后上传。
- `DocConverter`：仅产品允许集合内文件的 PDF 转换及健康检查，不暴露转换器全部文件家族。

Token 签发、预览解析、管理列表/撤销和预览缓存检查是 application UseCase；HTTP 路由是 interfaces；Valkey、阿里 OSS、silo 和 Gotenberg 适配器属于 infrastructure。业务规则包括双 TTL 必传、内容 hash、token 可复用、撤销幂等与失效 404，详见 [PRD](../prd/product.md)。

## 领域对象与授权

| 对象 | 职责与不变量 |
| --- | --- |
| TokenGrant | 将一个随机 token 绑定到签发时的文件描述与绝对期限；不能因读取、转换或缓存命中延长 token 授权 |
| PreviewFile | 记录来源、声明 hash、文件名及展示分类；签发时的 claimed hash 与下载后的 verified hash 分开，未下载不能宣称内容已校验 |
| CacheRecord | 保存有效期和准备/可用/删除状态，ready 只能指向已确认的不可变对象；不包含最终用户授权 |
| ObjectRef | profile 配置身份、对象 generation、实际媒体类型与存储引用；不包含访问密钥或可持久化签名 URL |
| Lease | owner、代次与失效边界；旧持有者不能覆盖新结果或删除其他代次 |

hash 标识内容，不是权限证明。只有有效 TokenGrant 才能解析缓存；知道 hash 不能列出、签名或撤销其他 token。管理员能力仍以 Admin role 的受保护 API 为边界，不引入未定义的租户/文件所有权模型。

## 缺失端口与原子职责补全

| 端口 | 应用层需要的能力 | 实现约束 |
| --- | --- | --- |
| TokenStore | 签发并登记索引、读取有效记录、列表、幂等撤销 | 主记录与索引原子性见 data；生成碰撞时拒绝覆盖 |
| CacheRepository | 读取状态、原子延长绝对期限、校验 owner 发布、条件失效、认领清理 | 缓存状态不靠若干独立读写拼接；profile 内去重，跨 profile 独立准备 |
| ConvertLock | 获取、续租、释放指定 owner 的 lease | 原子验证持有者；不允许等待者释放他人锁 |
| SourceFetcher | 受控 HTTPS 下载与临时文件句柄 | 上限、证书、重定向、取消与 hash 校验流程受质量/安全契约约束 |
| ObjectStorage | 上传不可变对象、验证引用、签名、幂等删除 | 仅配置的 profile；签名不写回 CacheRecord；所有支持文件均校验后入库，PDF/安全图片不转换 |
| DocConverter | 受检输入转换、输出验证、健康状态 | 固定版本；参数不由未信任的 URL 或任意 body 透传 |

这些是职责端口，不是本轮发布的 Go interface 签名。副作用具体由 infrastructure 实现，application 编排；过期、授权、可发布和可删除条件属于领域规则。临时文件和 SDK 对象不作为领域实体泄漏。相同字节不能只因展示文件名变化就复用不相容的转换输出；转换器/固定参数变更隔离规则见 data。
