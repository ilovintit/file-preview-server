# 本项目 CI 浏览器工具链

browser-go.Dockerfile 组合现有内网 Go 1.25.14 和 Playwright 1.61.1 镜像，不含应用源码或凭据；只用于本项目 CI，不是服务发布镜像。

- 镜像：reg.shw.top/ci-cache/file-preview-server-browser-go
- 标签：go1.25.14-playwright1.61.1
- 固定 digest：sha256:11509f51c17b96c37d67d0ec2d93ea4f903219b6a8b2e427b09382f2d19615f2
- Dockerfile sha256：22a2b0a4011227e173ff1eb2b9ad108fc2c3eab8c2310d19a4fa35092993f2a3
- linux/amd64 构建已核对 go1.25.14、Node24.17.0、Python3.12.3；CI 使用固定 digest。

构建上下文限此目录；基础镜像均按 digest 固定。使用 `docker buildx build --platform linux/amd64 -f .gitea/images/browser-go.Dockerfile --push -t <本项目CI镜像标签> .gitea/images` 构建，更新前核对实际镜像 digest，再同步 workflow。禁止覆盖共享基础镜像或在测试 job 中重新下载 Go。
