# 服务与实现分层

目标运行时为单个 GoFrame v2 HTTP 二进制，默认 `server` 子命令监听 9501（已登记全局端口表）；Gotenberg 作为同 Pod sidecar 监听 3000。项目不需要独立 worker 或数据库迁移角色；缓存过期对象由存储适配器的生命周期策略与应用内有界清理协同处理。

服务按 `internal/module/preview/` 组织四层：

```text
domain/          PreviewFile 规则、TokenStore/ConvertLock/ObjectStorage/DocConverter 抽象
application/     HTTPS 签名的 token 签发、解析、内容校验、转换与缓存编排用例
interfaces/      极薄签名请求 Controller 与函数式重定向路由
infrastructure/  Valkey、阿里 OSS、silo、Gotenberg、HMAC 签名和锁的适配器
module.go         路由注册与手工依赖注入
```

Domain 不得导入框架、Valkey 或外部 SDK；infrastructure 实现 Domain 接口；application 只依赖抽象；`module.go` 显式装配实现到 UseCase 和 Controller。不得用 DI 容器或 GoFrame service locator。

内部 JSON 路由在 HTTPS（生产优先 mTLS）上接收和返回普通 JSON；错误仍以不泄露鉴权细节的规范响应呈现。Controller 只提取已通过签名中间件验证的请求、调用 UseCase 并组装响应；验签、业务规则、数据范围和基础设施异常不应塞入 Controller。

当前 `dev` 没有 `go.mod`、`main.go` 或服务文件。本文件不表示上述布局已创建。

预览解析是同步请求内的编排，不增加任务状态查询 API、事件总线或独立 worker。转换与缓存清理的内部状态不向 demo 暴露进度百分比；页面显示等待状态。
