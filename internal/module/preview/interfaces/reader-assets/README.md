# 阅读器构建输出

由 `.gitea/scripts/build-web.sh` 从 `demo/preview-h5/` 生成。此目录的实际 HTML/JS/CSS/PDF.js 资源参与 Go embed；未构建时阅读入口返回 503，不用静态原型充当实际阅读器。

生成物不提交，CI 和应用镜像必须先构建 Web，再编译 Go。此说明文件不通过阅读器 HTTP 路径公开。
