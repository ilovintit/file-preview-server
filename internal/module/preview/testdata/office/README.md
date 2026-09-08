# S03 Office fixtures

本仓库生成的合成内容，不含用户数据。`document.fodt`、`sheet.fods`、`slides.fodp` 是可审查的 flat ODF 源文件；使用 LibreOffice 将对应源导出为 DOC/DOCX、XLS/XLSX、PPT/PPTX，并提交真实二进制文件，不用重命名其他格式充当样例。

`manifest.json` 固定输入 SHA-256、预期页数及关键文字。六项均预期一页；表格单元格分别为 S03Sheet 和数值 42。生成文件不代表转换测试通过，验收使用 PR CI 中固定 Gotenberg、Valkey 和实际 OSS 服务。

重新生成时显式设置项目内 `TMPDIR`、`XDG_CACHE_HOME` 和 `-env:UserInstallation=file:///…/.cache/soffice-profile`，避免写入项目外路径。更新二进制必须一起审查 manifest，不能自动刷新预期值来掩盖转换差异。

`protected.docx` 由本目录 `document.docx` 使用 msoffcrypto-tool 5.4.2 的 OOXML 加密生成功能生成，测试密码 `S03-fixture-only`（不是部署凭据）。固定 SHA-256 为 `690a393bc9e7f6a370c1644f76f39b6ff54d89649714a50bca7564770c19068e`，用于密码保护输入 422 的负向验收，不在六格式成功 manifest 中冒充可转换文件。
