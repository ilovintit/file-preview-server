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

MacWrite两个后缀均属于同一真实格式族，分别采用4.5/Pro1.0原件；初读libmwaw参考记录时曾将页跨度误作页数，后续源码核实为1/3页（依据见下方修正记录），ClarisWorks为1页。HWP和LWP源结构分别为1/2页，LWP有显式分页符；Pages活动正文和自带预览为一页Document Liberation链接，不能误读XML里的Lorem模板内容。

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

## 修正候选验证

候选 `34a84d33ebc712b20404e2fd53d1d88059fb2c67` 已推送 PR #34，CI10901 快速层通过，真实转换/浏览器层仍在运行。新增空白 Works 断言要求真实 PDF 仅一页且无正文；补充已知标准 DTD、未知网络/本地 DTD 与包内 DTD 的边界断言。本地仅 `go build` 和 `go test -c` 编译成功，未执行本地测试；不能据此宣称转换或安全验收通过。

CI10901 最终失败：silo 的全部28种文字格式转换/内容检查及7种浏览器原样格式通过；OSS 的原样、核心Office及文字转换后的读取共同失败（不是28种转换器全部退化）。浏览器显示 ORB/CORS 阻断，过期签名用例收到404而非OSS的403。对公开预览域名只读排查得到 nginx 404、正文 `{"error":"not_found"}`，DNS A 为112.74.37.78、未返回CNAME；已请求用户核查其DNS/反向代理目标与查询串保留，未变更CORS或外部基础设施。

宏样例单独返回422，尚不能断言宏防护已通过。候选 `23a61d42e379b042cd2809a8939dd1a89d543b3f` / CI10917 加入真实转换器代理计数/状态和Chromium重定向/额外响应状态，以判定输入校验与转换器错误边界；不记录签名URL、Token或对象正文。

CI10917 重试后28×双profile、空白Works通过；OSS的PNG原样读取及AVIF浏览器读取仍有间歇失败，浏览器确认token302后的目标404。宏失败为转换器调用1次返回400，不是应用输入校验拒绝。CI10938浏览器全绿，重型层仅宏失败；HTTP正文仅通用UnoException提示，故CI10960开启隔离Gotenberg调试日志，失败时只读取该job的转换器日志。

补齐密码保护验收：由真实source.docm经msoffcrypto-tool5.4.2生成Agile加密样例，作者工具核对解密后字节一致；加入重复422、无Location、未进转换器/无对象发布断言。依赖只装在本worktree的.cache/fixture-python，未执行本地服务测试。

宏样例修正假设：原manifest把Basic/目录标为扩展包媒体类型。LibreOffice固定源码 [namecont.cxx](https://github.com/LibreOffice/core/blob/11ccf4230941895f97e4fb6987631b65b401858b/basic/source/uno/namecont.cxx) 的ScriptExtensionIterator使用application/vnd.sun.star.basic-library识别扩展包，而文档宏库写入的是text/xml流。按原生文档包结构将Basic/和Basic/Standard/声明为空目录类型，宏代码/事件/原正文断言不变；需要下一轮真实CI确认，不能提前认定根因已修。
