# S05 text-family fixtures

本目录精确覆盖 Issue #19 的28种扩展名，不是已通过转换的支持声明。`manifest.json` 固定输入hash、预期页数、关键文字和来源；CI不能自动重写期望值。

## 自建样例

`source.fodt` 是本仓库合成的两段文字，含 ASCII、法文重音和中文。LibreOfficeDev `26.8.0.0.alpha0 / 2c87e51eeaa2b413ff4ae097b2705eea1995d8e5` 导出9种常规格式；本仓库生成器创建真实 DOTM 包类型、AbiWord/gzip、BibTeX、T602 和 OpenOffice XML 文档/模板结构。DOCM/DOTM为合法的宏可用格式，但不包含VBA项目，不能据此宣称宏安全已验证。

生成器分别是 `.gitea/fixtures/generate-text-fixtures.sh` 和 `.gitea/fixtures/generate-text-containers.go`。所有profile/cache/tmp输出留在当前worktree。它们只负责样例编写；固定Gotenberg的PDF转换验收仍只在PR CI运行。

## 上游样例与许可

`upstream.json` 保存每个原始URL、不可变revision、Git blob SHA、最终SHA-256和许可；获取脚本在写入前验证Git blob，不执行上游脚本。对应许可原文和Tika NOTICE在 `licenses/`。这些样例不来自业务系统，不含部署凭据，不纳入生产镜像。

`.mw` / `.mcw` 都是LibreOffice `writer_MacWrite` 注册的真实扩展名，分别采用MacWrite4.5与Pro1.0原件；原件来自Classic Mac、没有后缀，补正确扩展名不是将别的格式伪装成MacWrite。不能仅凭通用 `file` 工具的启发式结果判定这些老格式。

SGL来自freedesktop官方的原生StarWriter5主文档。上游原件是空文档；`fill-starwriter-master.py` 在保留主文档CLSID、原始header和其余stream的前提下，写入本仓库合成文字，更新N/T原生记录、统计和CFB stream长度。新增文字恰好使用原已分配mini-sector的空余24字节，不改FAT、不改后缀冒充普通SDW。原始与修改后的hash分别保留，修改步骤可重现。该作者工具使用olefile0.47，仅在项目内缓存安装。

完整编写顺序：原生导出 → Go容器生成 → `fetch-text-fixtures.mjs` → `fill-starwriter-master.py` → `freeze-text-manifest.mjs`。不能在CI失败后直接重跑冻结脚本来接受差异。

## 预期依据与失败边界

- 自建文稿固定一页并核对全部可见文字，不允许XML/RTF源码被当作正文蒙混过关；BibTeX明确按其源文本阅读。
- MacWrite4.5为2页、Pro1.0为3页、ClarisWorks为1页，依据对应libmwaw原始参考记录；关键文字覆盖标题/页眉页脚/表格。原始blob与LibreOffice所收录样例一致。
- Pages以活动正文和自带原始QuickLook预览为依据：一页 `Document Liberation link.`。其XML中还含模板的Lorem Ipsum，不能误当实际正文。
- HWP为一页英文/韩文；LWP具有明确分页符、两页。源文字/结构读取只用于确定期望，不算固定Gotenberg验收。
- WordPerfect关键文字沿用Apache Tika `WordPerfectTest` 对原件的断言；不要求恢复已删除文字。Pocket Word和Works关键文字来自原件文本记录。VOR的固定段落可直接在原始StarWriter stream中确认。
- 本机精简LibreOffice对部分旧格式会退回乱码纯文本。该结果已识别为无效参考，未采用其页数或正文；不能把这种“成功导出”当作转换成功。

最终必须通过28×两profile的真实转换、PDF解析、页数/文字、异常输入和已有能力回归；任何缺样例、损坏、乱码回退、固定镜像失败都阻断合并，不静默删减范围。
