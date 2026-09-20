# 镜像端到端 job 的工作区传递

## 根因

Gitea act_runner 即使 job 未声明 `container:`，也把 job 跑在容器里。`tests/image/run.sh` 原先用 `-v "$root:/work"` 把工作区挂给兄弟容器，而 `$root` 是 job 容器内的路径，宿主 docker daemon 解析不到，于是兄弟容器拿到空目录，prepare 步骤报 `can't open '.gitea/scripts/go-env.sh'`。

失败发生在准备步骤之前的镜像构建、AC01 镜像拓扑断言与依赖服务启动都已成功，所以问题只在工作区与证书的传递方式，不是验证内容本身。

## 修复

改为一次性 docker volume：先把工作区（含 `.git`，因为 `go-env.sh` 用 `git rev-parse` 定位根目录；排除 `.cache`）用 `tar | docker cp -` 灌进 volume，prepare、harness、probe 容器都挂这个 volume，应用容器以只读方式挂同一 volume 读取一次性证书。清理时删除 volume。

日志脱敏检查不再在 shell 里解析 JSON：旅程阶段额外写一个 `live-token.txt` 纯文本文件供编排脚本读取。

## 边界

断言内容一条没有放宽。本地无法复现 Gitea runner 的容器内 docker 环境，真实结论以 dev 上的回归报告为准；失败如实记录。
