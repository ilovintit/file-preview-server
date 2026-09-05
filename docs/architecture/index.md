# 文件预览服务架构

> 状态：这是依据 [Issue #1](https://git.shw.top/shw-project/file-preview-server/issues/1) 的交付范围、[Issue #5](https://git.shw.top/shw-project/file-preview-server/issues/5) 的调用方接入契约、[Issue #7](https://git.shw.top/shw-project/file-preview-server/issues/7) 的微信小程序裁决与 [Issue #9](https://git.shw.top/shw-project/file-preview-server/issues/9) 的完整重构裁决迁入的目标架构；`dev` 目前仍是无 Go module 的工程骨架。下面的文件描述目标，不代替实现、CI 或部署证据。

| 边界 | 权威文档 | 当前实现状态 |
| --- | --- | --- |
| 调用方与浏览器 | [clients.md](clients.md) | 外部仓库拥有；本仓库尚无客户端代码 |
| 服务与分层 | [services.md](services.md) | 目标已定义，未进入 `dev` |
| 领域 | [domains.md](domains.md) | 目标已定义，未进入 `dev` |
| 数据与对象存储 | [data.md](data.md) | 目标已定义，未创建数据配置 |
| 文件格式 | [formats.md](formats.md) | 目标已定义，未接入 Gotenberg 镜像 |
| HTTP API | [apis.md](apis.md) | 目标已定义，未注册路由 |
| 安全 | [security.md](security.md) | 目标已定义，未实现中间件 |
| 测试与 CI | [testing.md](testing.md) | PR Gate 骨架存在；实际用例待 Issue #1 |
| 部署 | [deployment.md](deployment.md) | `deploy/` 与 Fleet 目标尚不存在，待 Issue #1 |

产品价值、范围与验收以 [PRD](../prd/product.md) 为准；交互入口边界见 [design index](../design/index.html)。
