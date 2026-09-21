# 金山 WPS 原生容器样例

样例逐字节取自本机已安装的 WPS Office macOS 12.1.28492 官方资源，不从 Microsoft Works、改后缀的微软文件或第三方业务文档冒充。manifest 记录原路径、完整 sha256 和来源；未打包任何 WPS 可执行文件或美术模板。

newfile.wps 与 newfile.dps 是原生空白文件，仅证明原生容器和空白页链路；newchart.et 是应用内置图表数据样本。非空正文/工作表/幻灯片的共用转换回归还使用原有核心 Office fixture，不能声称覆盖所有 WPS 版本和排版。实际业务文档反馈在首版交付后收集。

WPS 原生 CFB 分别包含 WordDocument、Workbook/ETExtData、PowerPoint Document。适配器验证容器后，使用对应 .doc/.xls/.ppt 名称提交同一字节给转换器，输入 fixture 和产品扩展名保持原样；这是受检容器适配，不是改后缀伪造样例。
