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

## 装配与请求管道

入口 `main.go` 只启动 `internal/framework/cmd` 中的 `server` 子命令；启动装配先验证环境配置，再构造 Valkey/存储/转换/下载适配器，将 [领域端口](domains.md) 注入用例，最后注册规范 JSON 路由、原始 302 路由和探针。注册工作集中于模块入口，不能依赖散落的隐式初始化副作用。

Internal/Admin 请求管道依次执行可信 HTTPS 判断、有界读取 raw body、签名与角色验证、nonce 原子占用、JSON 解码与路由参数校验、领域规则/用例、响应映射。GoFrame 自动校验须位于验签之后；请求体只能按同一原始字节验签，不能提前重编码。公共预览路由从 token 读取授权，不挂 Internal/Admin 签名；探针不挂用户鉴权但由网络边界限制用途。

```text
api/preview/v1/                  HTTP Req/Res 与路由声明（字段权威见 apis）
internal/framework/cmd/          启动、配置装配与优雅退出
internal/module/preview/
  domain/entity/                token / 文件描述 / 缓存引用与状态规则
  domain/repository/            TokenStore / CacheRepository
  domain/contract/              ConvertLock / ObjectStorage / SourceFetcher / DocConverter
  application/                  签发、解析、查询、撤销、清理用例及 DTO
  interfaces/                   Controller、导航响应及路由装配
  infrastructure/               Valkey、profile、下载器、转换器、签名适配
  module.go                     显式构造与注入
```

以上是目标目录，不要求为每个端口创建无行为的文件。单领域能力留在 preview 模块；只有实际跨模块复用才下沉 shared，不为将来扩展提前制造空模块。无 SQL 模型，不生成 DAO、业务数据库迁移或 seed 角色。

## 错误与身份传递

已认证的 caller/key role 通过请求 context 传入应用层，不能从业务 body 取操作人。用例不从全局容器查找仓储或配置。预览 token 与 HMAC key 两种身份不能混用，demo 用户身份也不等于服务端调用方身份。

统一 HTTP 层通过 GoFrame 错误机制与明确映射保留底层错误语义；适配器不压缩所有故障为“成功但无结果”。本项目已确认 HTTP 状态与字段优先级见 [decisions](decisions.md)。302 成功路由只能写一次 Location/响应头，不被 JSON middleware 二次包装；已开始的流或重定向不能再追加 JSON。进程日志只记录脱敏上下文。

下载、转换、上传与清理调用都显式传入 context 和剩余预算。时钟与随机数生成可注入，以便 CI 验证时间窗、碰撞和取消；测试 seam 不暴露成服务端辅助 API。连接池与不可变配置在进程级共享，请求级文件/DTO 不共享；各副本依赖 Valkey 协调，不能把本机 map 当权威缓存。

运行时顺序见 [runtime](runtime.md)，容量和故障矩阵见 [quality](quality.md)。
