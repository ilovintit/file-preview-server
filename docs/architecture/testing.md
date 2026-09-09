# 测试与首版验收

## 当前口径与事实

[#36](https://git.shw.top/shw-project/file-preview-server/issues/36) 固定长期支持范围：常见图片、PDF、微软新旧版和金山 WPS 办公文件，精确集合见 [formats](formats.md)。其他格式永久不支持，没有后续大格式矩阵。

沿用 [#28](https://git.shw.top/shw-project/file-preview-server/issues/28)：实际 Web 自动化通过后交付镜像与接入资料，再收集业务反馈。人工浏览器/真机、微信开发者工具、人工部署回滚与固定 24 小时观察不作前置。

唯一版本计划为 [v1.0.0 Milestone](https://git.shw.top/shw-project/file-preview-server/milestone/11)，剩余实施顺序 #37 → #25 → #26。#1 只作汇总；各 Issue 维护 AC↔TC，不额外复制版本范围文件。

dev@37a01a0 的 [CI 11247](https://git.shw.top/shw-project/file-preview-server/actions/runs/11247) 三项 job 成功，包含集成和双 profile Chromium 图片/PDF 导航。这是旧实现证据，不证明新允许集合、金山 WPS、实际 H5 阅读器或镜像已经交付。jsdom 只检查静态原型状态，不能替代真实 Web E2E。

## 剩余交付与测试映射

| 用户结果 | Issue | 稳定 TC / 自动证据 |
| --- | --- | --- |
| 仅允许指定格式，微软/WPS 文件可预览 | #37 / R1 | `TC:R1:allowlist_enforcement`、`fixture_identity`、`allowed_profile_conversion`、`rejection_and_regression`、`scope_and_ci_selection`；允许集合的真实样例与双 profile 转换、集合外/旧缓存拒绝 |
| H5 与 PC Web 实际阅读文件 | #25 / R2 | 沿用 `TC:S11-AC01` 至 `TC:S11-AC05`；附件 ID、DTO/fragment、图片/PDF、取消/错误恢复及手机/桌面视口，不含小程序 |
| 可部署镜像与业务接入包 | #26 / R3 | 沿用 `TC:S12-AC01` 至 `TC:S12-AC05`；镜像/声明/配置、探针/退出/恢复、镜像内 Web 链路与接入资料 |

#15–#18 的签名/角色/nonce、双 TTL/hash、profile 隔离、缓存/lease/清理及核心 Office 测试按改动继续回归。#19 超范围正向测试随 #37 移除；#20–#24 旧矩阵和 #27 小程序构建不再是门槛。必要的集合外拒绝测试保留，取消测试不记为通过。

## Web 与镜像放行链路

对应 PR CI 启动实际服务、Valkey、固定 Gotenberg 和 OSS/silo 测试 profile，在固定 Chromium 的桌面及 320/390 px 视口验证：附件 ID → 测试适配层真实签发 → 清除 fragment → token URL → 所选 profile → 实际图片/PDF 内容。

覆盖允许办公格式的代表阅读旅程、转换等待、翻页/缩放、取消/迟到响应、404/撤销、422、5xx/网络/CORS/Range、返回后重新获取与页面恢复失效。记录内容断言及失败截图/trace。逐格式内容与 provider 正确性归 #37，页面状态归 #25，不把每个格式重复组合全部页面状态；#26 要用构建镜像启动服务跑同一链路。

## 数据、环境与执行

每个允许后缀提供真实 fixture、sha256、来源/生成工具和正文/单元格/幻灯片预期。金山 WPS 与 Microsoft Works 同后缀内容不能混用，不通过改后缀伪造样例；正向集合仅 18 个后缀，在两 profile 上验证；清单冻结不代表测试已通过。

错误覆盖 hash/类型不符、损坏/密码保护、超限、授权失效和基础设施故障。既有恢复、资源上限与脱敏要求随相应改动验证，不新设未确认的性能 SLO。首次发布无旧版时记录停止流量/恢复配置方案，不伪造历史镜像回滚成功。

每 run 采用隔离测试资源，凭据由环境注入。实际业务域名/Fleet 目标由接入方配置，不阻塞制品交付；本次自动测试所需 profile/fixture/证书缺失不能算通过。

本地只做构建/类型确认，lint、单元、API/集成、Web E2E/VRT 与安全检查在 PR CI 取当前 head 证据。Issue → dev 按变更和 AC 选择相关文件/模块/用例，文档及原型说明变更只需文档/脚本/既有 DOM 范围。#37/#25/#26 分别维护适用测试选择器；空选择或失效映射不能扩大到全量或当成功。

仅 dev → main 发布 PR 对当前产品允许范围执行全量门禁，“全量”不能恢复已取消格式。失败按真实日志修复，不通过跳过失败、扩大无关矩阵、重复环境重跑或静态样稿代替可用性。报告记录 commit/head、TC/AC、fixture/profile、镜像/浏览器版本与实际结果。

## 发布与关闭

剩余交付合入 dev、当前候选的 Web/镜像与必要自动检查通过后创建 dev → main 放行 PR。用户 Web UI 合并 main 后发布 tag 和可追溯镜像，交付 digest、部署模板、环境变量及业务调用说明。

Milestone 在实际镜像与接入交付完成前保持 open，随后按版本关闭流程收口，不添加人工真机或固定观察期。镜像发布不等于业务环境已部署，业务反馈在既定格式边界内迭代。
