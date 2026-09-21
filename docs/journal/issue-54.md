# 镜像端到端的就绪等待与依赖故障用例

## 事实

`recoveryWindow = 601`（`internal/module/preview/infrastructure/valkey.go`）：全新命名空间冷启动时授权状态守卫把 `ReadyAt` 设为 `now + 601`，`/readyz` 在这 601 秒内正当返回 503。run 30791 的证据工件里应用日志只有正常启动行，三个依赖容器都健康，失败纯粹是编排脚本只等了 180 秒。

这个窗口是设计行为。修复只放宽等待期限，没有缩短、跳过或旁路它。

## 改动

- 首次就绪等待放宽到 900 秒，其余等待保持 300 秒默认值。
- 依赖故障用例从 `docker stop/start` 改为 `docker network disconnect/connect`。自管 Valkey 无持久化，停容器会连授权状态一起清掉，重连后又进入一个新的 601 秒窗口，既拖长 job 也测不出"依赖恢复后自动就绪"。
- 探针预编译成二进制放进一次性 volume，避免长轮询里每次 `go run` 重新编译。

## 到目前为止确认有效的部分

镜像构建、AC01 镜像拓扑断言（非 root、revision label、无 Office/浏览器引擎）、volume 工作区传递（#50）、容器内 Go 引导（#52）、一次性证书与隔离 bucket 准备都已在 CI 上真实跑通。
