# R1：允许格式与真实 WPS

## 基线与范围

#36 / PR #38 已由当前 head 8262143 的 CI12466 全绿后合入 dev@057aac6。用户明确办公为 Word/Excel/PPT 三件套；本项执行 formats.md 中的 18 后缀集合，其他格式永久拒绝。

## 首轮测试先行

新增允许/拒绝集合断言，以及已签发旧 token 不能读取被取消格式现成缓存的应用测试。当前代码仍有 130 格式白名单且不重检旧 token，预期在真实行为处失败，不以编译或网络问题充当红色证据。

改动涉及所有预览格式入口、转换/缓存分发与既有原样/核心 Office 行为；CI 相关模块为 internal/module/preview，后续删除已取消正向格式检查，保留允许格式及公共安全/生命周期回归。

## WPS 取证

现有 .wps 样例来自 Microsoft Works，不再作为 WPS 正向证据。本机 WPS 官方安装包含原生 newfile.wps/newfile.et/newfile.dps，可只读核对真实 CFB 结构。电脑控制权限不可用，未改用其他 UI 技术绕过；原生空白样例不等于非空正文/单元格/幻灯片已验证。

后续依次收紧入口、移除超范围实现/正向 fixture，补 BMP/TIFF 与真实 WPS 适配，再验证当前 PR CI。所有测试运行只在 CI，本地仅构建/类型确认。

## 首轮红色证据

PR #39 / head 0503c19 / CI12478 job20354 在行为断言处失败：.et/.dps 被拒绝、已取消格式仍允许，旧 .txt/.pages/.avif/.docm token 仍调用缓存 preparer。不是编译或环境失败。随后开始实现统一18后缀边界及预览重检，移除旧文字扩展与 AVIF 正向路径。

## 首轮实现

已删除旧文字扩展转换器、正向矩阵与专用生成脚本，移除 AVIF 解码依赖和正向用例，仅保留拒绝测试。核心 Office 缓存身份保持不变；WPS 与新增图片转换使用独立 allowed-v1 身份，避免旧 Works 产物被复用。

本地 go build ./... 与 go test -c -tags 'integration browser'（仅编译，不执行）通过。新增 WPS 三类原生资源的双 profile 转换、Microsoft Works 拒绝、BMP/TIFF 转换后红色图像像素验证，待当前 PR CI；空白 WPS 样例的限制在其 README 明示。

## CI 工具链根因与收敛

CI12490 首轮在 Go 下载阶段因 dl.google.com TLS 连接重置失败，单次重跑后进入真实测试。只读核对 Harbor：现成 shw-plugin-toolchain@sha256:4b71bd74caa70a65cde2120fa8438dd141a1afe7c8268367cbc76ab202d80b9e 固定 Go1.25.14、Node24、gcc/musl。使用该既有内网镜像运行 Go 检查，单独保留文档检查，删除运行期下载 Go 脚本；浏览器从同一 run 下载并校验编译测试产物，不再下载/编译工具链。未修改外部镜像仓库或 Action 镜像。

当前功能红项为 newchart.et 页数假设和 TIFF 图片内容。已保存失败 PDF/源文件并增加图片对象/像素诊断，先读实际证据再修复，不以缩减允许格式规避失败。
