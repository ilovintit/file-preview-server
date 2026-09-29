#!/usr/bin/env bash
# 兄弟容器内的引导：docker run 起的容器不会继承 runner 给 job 容器注入的 PATH，
# 所以这里补上常见 Go 安装目录；找不到 go 时打印真实 PATH 与搜索结果再失败，
# 不静默跳过验证。
set -eu
for directory in /usr/local/go/bin /usr/lib/go/bin /usr/lib/go/pkg/tool /opt/go/bin /usr/local/bin /usr/local/sbin; do
  if [ -d "$directory" ]; then
    PATH="$PATH:$directory"
  fi
done
export PATH
if ! command -v go >/dev/null 2>&1; then
  echo "容器内找不到 go；PATH=$PATH" >&2
  find / -maxdepth 6 -type f -name go -perm -u+x 2>/dev/null | head -5 >&2
  exit 127
fi
. scripts/go-env.sh
exec bash -c "$1"
