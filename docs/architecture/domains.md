# Preview 领域

`PreviewFile` 是核心领域对象，负责由 `filename` 判定图片、PDF、Office 或不支持格式。领域层定义以下抽象而不绑定具体驱动：

- `TokenStore`：保存、读取、列出与删除预览 token。
- `ConvertLock`：同一转换资源的互斥获取与安全释放。
- `FileStorage`：原文件与转换产物的存在检查、上传、读取和签名访问。
- `DocConverter`：Office→PDF 转换与健康检查。

Token 签发、预览解析、管理列表/撤销和转换缓存检查是 application UseCase；HTTP 路由是 interfaces；Valkey、Silo/S3 和 Gotenberg 适配器属于 infrastructure。业务规则包括 TTL 必传、范围、token 可复用、撤销幂等与失效 404，详见 [PRD](../prd/product.md)。
