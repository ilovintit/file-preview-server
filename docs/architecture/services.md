# 服务与实现分层

目标运行时为单个 GoFrame v2 HTTP 二进制，默认 `server` 子命令监听 9501（已登记全局端口表）；Gotenberg 作为同 Pod sidecar 监听 3000。项目不需要独立 worker 或数据库迁移角色；缓存过期对象由存储适配器的生命周期策略与应用内有界清理协同处理。

服务按 `internal/module/preview/` 组织四层：

```text
domain/          PreviewFile 规则、TokenStore/ConvertLock/ObjectStorage/DocConverter 抽象
application/     加密签名的 token 签发、解析、内容校验、转换与缓存编排用例
interfaces/      极薄加密信封 Controller 与函数式重定向路由
infrastructure/  Valkey、阿里 OSS、标准 S3、Gotenberg、加密签名和锁的适配器
module.go         路由注册与手工依赖注入
```

Domain 不得导入框架、Valkey 或外部 SDK；infrastructure 实现 Domain 接口；application 只依赖抽象；`module.go` 显式装配实现到 UseCase 和 Controller。不得用 DI 容器或 GoFrame service locator。

内部 JSON 路由在 HTTPS 上接收和返回 AES-256-GCM 信封；错误仍以不泄露鉴权细节的规范响应呈现。Controller 只验证信封、调用 UseCase 并组装响应；验签、业务规则、数据范围和基础设施异常不应塞入 Controller。

当前 `dev` 没有 `go.mod`、`main.go` 或服务文件。本文件不表示上述布局已创建。
