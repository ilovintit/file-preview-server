# Issue 19 — S05 remaining text formats

## 起点与约束

- 前序 #18 / PR #33 已合入 dev `2268c5caab79fa2d07949e35144041b26f999cc8`；候选 `ee828ebe97c881813a8d9c66552d2acecdbaea5a` 的 CI10749 三项全绿，包含双profile矩阵/浏览器及无需新增OSS权限的严格readyz检查。
- 工作区 `.worktrees/19`，分支 `codex/19-text-format-matrix`；P1 的28种文字扩展名仍是本版本必交，不静默删减。
- 当前公共转换器只识别核心六Office；需要扩展文字格式识别及固定输出版本，但先落实合法fixture及可追溯来源，禁止仅改后缀冒充。

## 计划

1. 对照 #19 的精确28扩展名建立fixture清单；优先从本仓库合成文字导出，历史专有格式核实上游测试语料及许可。
2. 冻结每个文件hash、格式来源、页数/关键文字预期；有缺失或固定Gotenberg失败时保留明确阻断，不记通过。
3. 同一PR测试先行，然后扩展共享转换/识别，跑28×两profile与原格式回归及安全/边界检查。
4. 当前head真实CI全绿后才自审合并/关闭本Issue。

## 初步来源调查（未记为验收）

LibreOffice core 的总递归树被API截断，不能把首屏当完整fixture清单；改查sw/writerperfect/filter子树。libmwaw官方SourceForge提供独立regression语料，正在项目内缓存调查许可/格式；未将上游样例或转换结果记为已通过。

## Fixture 落实

现已建立全部28种：17份本仓库合成格式 + 11份版本固定/原始Git blob校验的历史格式。VOR来自Apache Tika；SGL来自freedesktop官方shared-mime-info，是区别于SDW的真实主文档，不是改后缀。上游SGL为空，已按原生N/T段落、d统计记录及CFB stream长度补写合成文字，保留主文档CLSID/header，使用原分配mini-sector的24字节空余，生成器可重现。

MacWrite两个后缀均属于同一真实格式族，分别采用4.5/Pro1.0原件；libmwaw参考记录明确2/3页，ClarisWorks为1页。HWP和LWP源结构分别为1/2页，LWP有显式分页符；Pages活动正文和自带预览为一页Document Liberation链接，不能误读XML里的Lorem模板内容。

本机精简LibreOffice对WPS/WPD/PSW/VOR/SGL会退回乱码纯文本，这些结果未用于预期、更不算Gotenberg验收。WordPerfect改用有上游明确文字断言的Tika原件；Works选用能直接核对文本记录的DOS2原件。manifest冻结28种hash、页数与关键/完整文字；CI必须拒绝源码/二进制乱码作为PDF正文。

下一步在同一PR取得新文字格式的缺失行为红色证据，再实现识别/转换。当前只完成样例与断言编写，不宣称固定镜像的28种格式已通过。

## 测试先行红色证据

PR #34 / head `90c21949a1ab5f13866e5bb915ca5823d8337175`，CI10840 / job18381：28种fixture完整性检查通过；56个文字格式/profile组合均 `text conversion expected302 got422`，确认失败是功能缺失，不是环境或样例hash错误。随后开始实现文字格式的有界识别/转换，TXT/BibTeX先作为转义文本封装为FODT，避免LibreOffice自动误读活动内容；原有raw/coreOffice输出身份保持不变。

## 首轮真实转换与修正依据

CI10857已有23种×两profile通过。其余5种逐项处理：

- SGL：mscfb的GUID.String返回带大括号形式，代码裸GUID比较错误；修复为规范化括号，并增加主文档不能冒充VOR的断言。
- Works：输入Git blob与libwps-reference的BLANK.WPS完全一致；上游raw参考确认只有空段落。此前strings读到的是不可见残留，不能作为正文。换用同一官方DOS2语料的LANDSCAP.WPS（明确的一页正文），保留BLANK负向回归，不删除WPS范围。
- MacWrite4.5：libmwaw源09e615baa557a528b259687e49e4057969af563f的MacWrtParser::createDocument使用numPages+1填页跨度，原span=2并不意味着两页内容；实际文档是一页。按该源码依据修正预期，全部文字断言保留。
- SXW/STW：对比Tika原生包，旧格式没有现代ODF的mimetype成员，而有styles/meta/settings。修正作者包结构，补旧manifest判定，严格移除已知标准DTD声明，拒绝自带DTD或未知外部声明。
- Chromium中OSS GIF单项首次解码失败，文本格式改动未改变raw路径；加脱敏HTTP状态/网络错误诊断，在新head重新验证，不直接放宽浏览器检查。
