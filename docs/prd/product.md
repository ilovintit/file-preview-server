# 文件预览服务产品真相

> 当前状态：`v1.0.0` 产品基线审查中（[Issue #5](https://git.shw.top/shw-project/file-preview-server/issues/5)）；功能实现仍由 [Issue #1](https://git.shw.top/shw-project/file-preview-server/issues/1) 交付，尚未进入 `dev`。本文不将目标架构表述为已上线能力。

## 用户、问题与价值

本服务供项目内 API server 和管理后台使用，统一处理受控文件预览。它把 Office 文档转换为 PDF、对 PDF/图片执行既有处理能力，并将可访问资源包装为可撤销、会过期的预览链接。最终用户只通过调用方交付的预览 URL 打开资源；浏览器、H5、WebView、小程序和 App 的嵌入方式由调用方选择并遵循 [调用方边界](../architecture/clients.md)，本服务不提供或托管前端预览页面。

它解决以下已确认问题：源服务端点裸露、并发转换锁只限单实例、没有预览链接生命周期管理、Gotenberg 没有随服务部署，以及调用方存在错误的 `/preview/preview` 拼接路径。

## 目标与非目标

### 本期目标（Issue #1 checklist 1–7；跨端接入契约由 [Issue #5](https://git.shw.top/shw-project/file-preview-server/issues/5) 定义）

- 从 `xlzb-project` 的 `src/document-processing/` 抽出可复用的独立服务。
- 为文件资源签发、查询和撤销预览 token；资源不直接由业务前端裸露。
- 通过 `GET /v/{token}` 提供唯一的、可导航的跨端预览入口；图片、PDF 与转换后的 Office PDF 均以 302 跳转到短时签名资源 URL。调用方持久化和再次打开的只能是 `/v/{token}`，不得保存或复用跳转目标 URL。
- 保留既有 PDF 脱敏、图片处理和历史预览能力，并以分布式转换锁支持多实例。
- 交付文档处理镜像及同 Pod 的 Gotenberg sidecar 部署声明。

### 明确非目标

- 不在本仓库修改调用方前端路径、`xlzb-project` 的旧目录/CI/deploy，或 B 端免登详情接口字段；这些需要在本服务可用后由 `xlzb-project` 另建 Issue。
- 本服务不保存业务 SQL 数据；不引入 PostgreSQL 作为本服务的数据源。
- 不把 Gotenberg 做成独立 Deployment，也不把它与服务二进制打进同一镜像。

## 角色与权限

| 角色 | 能力 | 认证边界 |
| --- | --- | --- |
| API server（内部调用方） | 提交资源信息并签发 token | 独立的 `INTERNAL_TOKEN` Bearer 密钥 |
| 管理后台调用方 | 列出活跃 token、撤销 token | 独立的 `ADMIN_TOKEN` Bearer 密钥 |
| 调用方页面、H5 或 WebView | 将 `/v/{token}` 交给图片元素、PDF 容器或导航容器 | 只持有 token；不能调用内部或管理 API |
| 小程序或原生 App | 使用自己的 H5/WebView，或经已验证的原生下载再交给本地查看器 | 只持有 token；不得取得内部或管理密钥 |

内部与管理密钥未配置时，对应端点必须拒绝请求（503），不得降级为匿名访问。

## 业务规则

- 签发资源必须提供 `url`、带支持扩展名的 `filename` 和 `ttl`；TTL 没有默认值，允许范围为 60–86400 秒。
- token 使用服务端状态而非 JWT：它是加密随机的 128-bit hex 值，在 TTL 内可重复使用；无效、过期或已撤销时统一返回 404，避免泄露存在性。
- `/v/{token}` 是导航 URL，不是返回文件字节、JSON 或供客户端读取 `Location` 的 API。它必须以 302 返回 HTTPS 的短时签名目标 URL；目标响应须保留正确 `Content-Type`，并以内联方式呈现。
- 图片和 PDF token 解析后重定向至签名资源 URL；Office 文件先检查转换缓存，未命中时加锁转换、上传缓存，再重定向。签名目标 URL 的有效期不得超过对应 token 的剩余有效期。
- 撤销或过期会阻止后续访问 `/v/{token}`；已被浏览器或客户端取得的签名目标 URL 可访问至其自身过期。这是短时签名 URL 的边界，调用方不得把它当作可撤销的业务链接。
- 管理端列表只展示仍有效的 token；撤销幂等，即使 token 不存在也返回成功。
- 历史 `/preview` 可在调用方迁移期间保留；启用 `DISABLE_LEGACY_PREVIEW=1` 后必须返回 404。`/v/{token}` 是后续统一入口。

## 关键流程与异常

1. API server 带内部 Bearer 密钥提交资源 URL、文件名和 TTL，取得 token 与过期时间。
2. 调用方把 `/v/{token}` 交给页面；页面刷新继续使用同一 token。
3. 调用方按所在容器选择嵌入：浏览器图片使用图片元素，浏览器 PDF 使用导航型容器或具备 CORS 条件的 PDF 阅读器；小程序和 App 优先使用自有 H5/WebView，原生下载模式须单独通过该容器的验收。
4. 服务读取有效 token，按文件类型直接签名重定向或执行 Office→PDF 转换并重定向。
5. 管理后台可列出现存 token 并随时撤销；访问被撤销或自然过期的链接得到 404。
6. 缺失/错误内部或管理密钥为 401；参数或不支持格式为 422；未配置对应密钥为 503；基础设施故障为 5xx。

## 可验收结果

- token 签发、必填 TTL、内部认证、可复用与失效 404 均有 CI API 用例。
- 图片/PDF 直接重定向、Office 转换、缓存和转换竞争锁均有 CI 覆盖。
- 管理端列出、撤销、过期懒清理及管理认证均有 CI 覆盖。
- 302 响应不缓存、不泄露 token 至跳转目标；浏览器图片、浏览器 PDF、H5/WebView 与已选择的小程序/App 集成模式均有对应的 CI 或人工验收证据。
- 构建镜像和双容器 Deployment 声明进入同一交付 PR；真实部署、Fleet 同步和运行健康度由发布/部署流程独立取证。

实现目录、端点字段、数据键和部署契约见 [architecture index](../architecture/index.md)。
