# 服务与实现分层

目标运行时为单个 GoFrame v2 HTTP 二进制，默认 `server` 子命令监听 9501（已登记全局端口表）；Gotenberg 作为同 Pod sidecar 监听 3000。项目不需要 worker 或数据库迁移角色，除非未来产品需求另行确认。

服务按 `internal/module/preview/` 组织四层：

```text
domain/          PreviewFile 规则、TokenStore/ConvertLock/FileStorage/DocConverter 抽象
application/     token 签发、解析、转换编排、图片/PDF 处理用例
interfaces/      极薄 HTTP Controller 与函数式重定向路由
infrastructure/  Valkey、S3 兼容存储、Gotenberg、图像处理和锁的适配器
module.go         路由注册与手工依赖注入
```

Domain 不得导入框架、Valkey 或外部 SDK；infrastructure 实现 Domain 接口；application 只依赖抽象；`module.go` 显式装配实现到 UseCase 和 Controller。不得用 DI 容器或 GoFrame service locator。

规范 JSON 路由返回 `{traceId, code, message, data}`。Controller 只接收已校验请求、调用 UseCase 并组装响应；验证、业务规则、数据范围和基础设施异常不应塞入 Controller。

当前 `dev` 没有 `go.mod`、`main.go` 或服务文件。本文件不表示上述布局已创建。
