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
