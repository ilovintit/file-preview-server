# 文件格式的永久产品边界

用户于 2026-09-09 在 [范围修正 #36](https://git.shw.top/shw-project/file-preview-server/issues/36) 明确：服务只支持常见图片、PDF 和微软新旧版/金山 WPS 办公文件，其他格式永久不支持。取消的格式不延期、不进入其他版本或 backlog；转换器升级、文件能被 WPS 打开或历史代码已有分支都不能自动增加产品支持。

## 精确允许集合

用户已明确办公为 Word、Excel、PPT 三件套，PDF 单列。图片按本次提出的常见图片清单实施。唯一允许集合共 **18 个后缀**：

| 类别 | 允许后缀 | 展示方式 |
| --- | --- | --- |
| 原样图片 | `.jpg`、`.jpeg`、`.png`、`.gif`、`.webp` | 校验后原样受控存储，以图片呈现 |
| 转换图片 | `.bmp`、`.tif`、`.tiff` | 转 PDF，以 PDF 阅读；TIFF 保留全部页面 |
| PDF | `.pdf` | PDF 阅读 |
| 微软 Word 新旧文档 | `.doc`、`.docx` | 转 PDF |
| 微软 Excel 新旧表格 | `.xls`、`.xlsx` | 转 PDF |
| 微软 PowerPoint 新旧演示 | `.ppt`、`.pptx` | 转 PDF |
| 金山 WPS 原生三类文件 | `.wps`、`.et`、`.dps` | 验证真实金山内容后转 PDF |

WPS 保存的上述微软格式也按实际内容验证。模板/宏专用格式、AVIF、其他办公软件家族及任何集合外格式均不支持，不作为后续版本需求。由 [R1 / #37](https://git.shw.top/shw-project/file-preview-server/issues/37) 在签发和预览路径执行这一集合。

## 服务规则

- 签发检查允许后缀；下载后验证真实内容、媒体类型和 `content_sha256`。后缀在集合内不代表任意同后缀内容都支持。
- 预览时重检当前允许集合，包括旧 token 与缓存命中路径；取消格式不能通过旧缓存继续取得短链。
- PDF 与可直接显示的安全图片原样受控存储；允许集合内需转换的办公文件或图片转为 PDF。转换能力只服务于产品允许集合。
- 集合外、伪装、损坏或不能处理的内容按既有 422 语义拒绝，不发布 ready，不回退源 URL。签名、TTL、profile 隔离与清理契约继续有效。
- 正向测试仅覆盖产品允许集合；集合外输入、伪装和旧缓存保留拒绝测试。转换器能力列表不再构成本服务的验收矩阵。

## 实施与验证边界

R1 / PR #39 已在签发、旧 token 解析与缓存准备之前执行同一 18 后缀集合，移除旧文字扩展及 AVIF 正向路径。核心 Office 缓存身份保留；WPS 和 TIFF 处理规则使用独立输出版本，不复用旧 Microsoft Works 或旧 TIFF 产物。

金山 WPS 原生 CFB 验证后分别按 WordDocument、Workbook、PowerPoint Document 送入对应转换路径；这不是把普通微软样例改后缀冒充 WPS。Microsoft Works 样例只保留在 `testdata/rejected/` 作拒绝检查。实际原生样例及来源见 [WPS 样例](../../internal/module/preview/testdata/wps/README.md)。文字/演示样例为空白，表格为内置数据图表；不宣称所有历史 WPS 版本与复杂排版已覆盖。

TIFF 逐 IFD 无损解码为 PNG 后送固定转换器合并，保持颜色与页序。限制 128 页、总 4000 万像素和总 32 MiB PNG 输出；循环、越界、异常尺寸和取消均受控拒绝，不静默只取第一页。BMP 单页转换与 TIFF 单/多页均有实际 PDF 图像内容断言。

OOXML 主内容类型必须与普通 docx/xlsx/pptx 相符，模板或宏专用内容不能仅改后缀绕过允许集合。所有格式仍做内容/hash、资源上限及错误验证，转换器不执行宏或外部资源。

CI12592 已通过原生WPS、BMP/TIFF两profile、页数/源值/像素与既有回归；后续自审修改仍以 [PR #39](https://git.shw.top/shw-project/file-preview-server/pulls/39) 当前 head CI 为准。真实 H5/PC 阅读页与服务发布镜像分别由 #25/#26 交付，CI工具镜像不等于服务已发布。

原 130 后缀矩阵仅在 Git/已取消 Issue 保留历史。固定转换器与部署见 [deployment](deployment.md)，验收见 [testing](testing.md)。
