# 镜像端到端容器内的 Go PATH

`docker run` 起的兄弟容器不会继承 Gitea runner 给 job 容器注入的环境，同一 `shw-plugin-toolchain` 镜像在 pr-gate 里 `go` 可用，在我们自己起的容器里 `sh -lc` 下 `go: not found`（run 30780）。

把容器内引导收进 `tests/image/in-container.sh`：补常见 Go 安装目录到 PATH，找不到 `go` 时打印真实 PATH 和 `find` 结果再以 127 退出，便于下一轮定位，然后才 source `go-env.sh` 执行命令。没有放宽任何断言，也没有在找不到工具链时静默跳过验证。

#50 的 volume 传递本轮已确认生效：工作区副本与 `go-env.sh` 都能找到，镜像构建、AC01 断言和依赖服务启动都成功。
