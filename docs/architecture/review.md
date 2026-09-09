# 产品范围修正与证据边界

当前产品变更载体为 [#36](https://git.shw.top/shw-project/file-preview-server/issues/36)，基于 dev@37a01a0。用户于 2026-09-09 明确常见图片、PDF、微软新旧版与金山 WPS 办公格式是服务长期范围；其他格式永久不支持。首版仅 H5/PC Web，优先交付可部署镜像后收集业务问题。

## 当前裁决

| 项目 | 结论 | 权威位置 |
| --- | --- | --- |
| 允许文件 | 产品清单决定支持范围，不由转换器能力或历史代码决定；其他格式永久取消，不安排后续版本 | [PRD](../prd/product.md)、[formats](formats.md) |
| 客户端 | 一套 H5/PC Web 阅读能力；小程序工程退出首版，不增加管理/编辑 UI | [clients](clients.md)、[demo](demo.md) |
| 交付顺序 | #37 允许格式/WPS → #25 Web 阅读 → #26 镜像与部署接入 | [Milestone](https://git.shw.top/shw-project/file-preview-server/milestone/11) |
| 保留的底层契约 | 所有允许文件下载校验后受控存储；profile/config 隔离、内容 hash/输出版本去重；ttl ≤ cache_ttl ≤ MAX_CACHE_TTL；读取不重新起算授权 | [apis](apis.md)、[data](data.md)、[runtime](runtime.md) |
| 首版放行 | 真实 Web 与镜像自动化；不要求人工真机/生产演练或固定观察；main 仍由用户 Web UI 合并 | [testing](testing.md)、[deployment](deployment.md) |

## 精确范围冻结

用户明确办公是 Word、Excel、PPT 三件套，PDF 单列。图片按已提出的常见清单实施；总计 18 个后缀见 formats。没有第四种办公软件家族。#36 合入后 #37 开始实施。

## 代码核对

- #15–#18 已合入授权、受控图片/PDF、微软核心六格式和 OSS/silo；dev@37a01a0 的 CI 11247 三个 job 成功。
- #19 / PR #34 的 28 种文字扩展是历史实现，部分超出当前产品范围，#37 要在签发/旧 token/缓存路径实际禁用，不能仅删除文档。
- 现有 legacy.wps 的来源是 Microsoft Works，不构成金山 WPS 证据；.wps/.et/.dps 的真实样例与支持由 #37 交付。
- 原表格 PR #35 已取消而未合并；#20–#24 和 #27 取消不代表原 AC 通过。
- 真实 Web 页面、应用 Dockerfile、镜像与完整部署组合尚未交付；静态原型和旧 Chromium 导航检查不能替代这些结果。

## 原型和历史

#13 / PR #14 曾记录小程序/H5 原型交互确认，#28 调整首版为自动验收。历史记录不继续授予超范围格式或小程序工程的实施要求；当前原型索引明确当前 Web 与旧页面参考的区别。既有模拟页面不证明新范围的真实阅读或布局验收。

本轮逐项检查 PRD、全部适用架构、原型说明、Issue 和实际依赖，保留当前用户的边界，不恢复 130 格式/260 组合或 65 条旧 AC。文档/脚本/DOM 验证只支持本次计划修正，服务实现与镜像证据由后续三个 Issue 独立提供。
