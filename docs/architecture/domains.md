# Preview 领域

`PreviewFile` 是核心领域对象，负责由 `filename` 和已验证内容 hash 判定 PDF、可浏览器直接显示的图片、需转换的 Gotenberg 格式或不支持格式。完整集合以 [formats.md](formats.md) 为准。领域层定义以下抽象而不绑定具体驱动：

- `TokenStore`：保存、读取、列出与删除预览 token。
- `ConvertLock`：同一转换资源的互斥获取与安全释放。
- `ObjectStorage`：按配置 profile 读取原对象、上传/删除转换产物、执行缓存生命周期并签名访问；阿里 OSS 与标准 S3 实现同一抽象。
- `DocConverter`：Gotenberg 支持的文档、图形与图片→PDF 转换及健康检查。

Token 签发、预览解析、管理列表/撤销和转换缓存检查是 application UseCase；HTTP 路由是 interfaces；Valkey、阿里 OSS、标准 S3 和 Gotenberg 适配器属于 infrastructure。业务规则包括双 TTL 必传、内容 hash、token 可复用、撤销幂等与失效 404，详见 [PRD](../prd/product.md)。
