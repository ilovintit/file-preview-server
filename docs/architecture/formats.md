# v1.0.0 文件格式支持

本表是待逐项验证的目标集合，不是已通过用例的支持清单。格式支持以部署时固定的 **Gotenberg 8.34.0** LibreOffice 转换器为唯一来源；升级镜像版本必须重新对比并运行整张格式矩阵。全部支持文件下载校验后进入指定 profile；PDF/安全图片原样存储，其余格式转换为 PDF 后存储预览；扩展名只是初筛，转换前后均以内容处理结果为准。仓库内 `demo/fixtures/` 维护每种格式的已知内容 hash fixture。密码保护、损坏或转换器不能处理的文件返回 422，不做降级直链。

## 原样存储后预览（仍须下载校验并签发受控短链）

- PDF：`.pdf`。
- 浏览器安全图片：`.jpg`、`.jpeg`、`.png`、`.gif`、`.webp`、`.avif`。

其余图形/图片格式走 Gotenberg 转换为 PDF，SVG 不直接嵌入浏览器，避免活动内容风险。

## Gotenberg 转换矩阵

| 家族 | 支持扩展名 |
| --- | --- |
| 文字 | `.doc`、`.docx`、`.docm`、`.dot`、`.dotm`、`.dotx`、`.odt`、`.fodt`、`.ott`、`.rtf`、`.txt`、`.wps`、`.wpd`、`.pages`、`.abw`、`.zabw`、`.lwp`、`.mw`、`.mcw`、`.hwp`、`.sxw`、`.stw`、`.sgl`、`.vor`、`.602`、`.bib`、`.xml`、`.cwk`、`.psw`、`.uof` |
| 表格 | `.xls`、`.xlsx`、`.xlsm`、`.xlsb`、`.xlt`、`.xltm`、`.xltx`、`.xlw`、`.ods`、`.fods`、`.ots`、`.csv`、`.numbers`、`.123`、`.wk1`、`.wks`、`.wb2`、`.dbf`、`.dif`、`.slk`、`.sxc`、`.stc`、`.uos`、`.pxl`、`.sdc` |
| 演示 | `.ppt`、`.pptx`、`.pptm`、`.pot`、`.potm`、`.potx`、`.pps`、`.odp`、`.fodp`、`.otp`、`.key`、`.sxi`、`.sti`、`.uop`、`.sdd`、`.sdp`、`.fopd` |
| 图形/绘图 | `.odg`、`.fodg`、`.otg`、`.vsd`、`.vsdx`、`.vsdm`、`.vdx`、`.cdr`、`.svg`、`.svm`、`.wmf`、`.emf`、`.cgm`、`.dxf`、`.std`、`.sxd`、`.pub`、`.wpg`、`.sda`、`.odd`、`.met`、`.cmx`、`.eps` |
| 图像 | `.bmp`、`.tif`、`.tiff`、`.pbm`、`.pgm`、`.ppm`、`.xbm`、`.xpm`、`.pcx`、`.pcd`、`.pct`、`.psd`、`.tga`、`.ras`、`.pwp` |
| Web/其他 | `.html`、`.htm`、`.xhtml`、`.epub`、`.pdb`、`.ltx`、`.mml`、`.smf`、`.sxm`、`.sxg`、`.oth`、`.odm`、`.swf` |

矩阵来源为 [Gotenberg LibreOffice Convert to PDF 支持扩展名](https://gotenberg.dev/docs/convert-with-libreoffice/convert-to-pdf)。每一个扩展名都必须有固定版本 CI 用例；某个格式在实际镜像失败时，在修复前从可签发集合移除，而不是向调用方承诺“理论支持”。

2026-09-05 复审：扩展名集合与官方当前文档相符；[v8.34.0 路由源码](https://github.com/gotenberg/gotenberg/blob/v8.34.0/pkg/modules/libreoffice/routes.go) 使用运行时 Extensions 校验文件。该证据不能替代固定镜像的逐格式转换、字体与版式验收，支持声明仍待 #1 CI。
