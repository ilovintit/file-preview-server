# 安全边界

- 内部 token 签发和管理 API 使用不同的静态 Bearer 密钥；两者均只通过运行环境注入，绝不提交至 Git、Actions 日志、镜像或 Kubernetes YAML。
- 未配置某个密钥时，该端点返回 503，不能以空值、默认值或匿名权限放行。
- 最终用户 token 只授权到一个预览资源及其 TTL；它不等同于 API server 或管理后台身份。
- 无效、过期和撤销 token 一律为 404，避免向浏览器泄露 token 是否曾存在。
- 图像处理的 HMAC 仍依赖 `IMAGE_SECRET`；缺失时对应端点返回 503。
- 统一 trace 与错误处理中间件记录基础设施失败，返回响应不得泄露 S3 凭据、内部 URL 或 Bearer 值。

当前仓库没有安全中间件或配置实现；实现需要在 Issue #1 中按此边界和 GoFrame 错误处理约定落地。
