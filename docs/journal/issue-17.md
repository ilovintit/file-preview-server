# Issue 17 — S03 Office conversion

## 恢复点

- 前序 #16 / PR #31 已合入 dev：`1950a82ef23793681f1a4314677dcd10b0c8bb56`。候选 `21ef001afbdee1bd69ca068b55873635a9cf60d9` 的 CI 10559 三个 job 成功。
- 本 Issue 工作区 `.worktrees/17`，分支 `codex/17-core-office-conversion`，从上述 dev 创建。项目明确要求项目内工作树，不能采用插件默认的项目外目录。
- 测试先行红色证据：PR #32，head `1eac5191f3da30fec07928cdc3909e7e4ae3f28b`，CI 10582 / job 18099 六种 Office 均 `Office conversion expected302 got422`。快速 job 18098 成功，失败不是编译或环境问题。不运行本地测试。
- Gotenberg 版本按产品基线固定 8.34.0，现有 Harbor `ci-gotenberg:8` 为 8.36.0，不能冒充所需版本。8.34.0 linux/amd64 已同步至 Harbor，digest `sha256:0ec4b0a125c55ff1dfaed071cab30e0f37cc4326f7d53d903ee5b24e4f27fd85`，CI 与组件固定此 digest。首次同步 HTTP/2 连接关闭，强制 HTTP/1.1 续传后完成；跨仓 layer mount 拒绝不等于 push 拒绝，普通 blob 上传成功。
- Harbor Actions 拉取凭据已由用户配置并通过 S02 Chromium job；不回显值。

## 计划

1. PR CI 证明核心格式转换缺失。
2. 扩展统一缓存输出版本、输入类型验证及 Gotenberg 有界调用，保留现有 lease/发布/签名授权重检。
3. 实际 Gotenberg + Valkey + OSS 检查六格式的页数、关键内容及错误/取消/并发契约，补充固定镜像和同 Pod 声明。
4. 当前 head CI 全绿后自审、合入 dev、回填关闭本 Issue，再推进 #18。
