#!/usr/bin/env bash
set -euo pipefail

required_version="$(awk '/^go /{print $2; exit}' go.mod 2>/dev/null || true)"
if [[ -z "$required_version" ]]; then
  echo "go.mod 缺少 go directive" >&2
  exit 1
fi

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  aarch64|arm64) goarch="arm64" ;;
  *)
    echo "unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

if command -v go >/dev/null 2>&1; then
  current="$(go env GOVERSION 2>/dev/null || true)"
  if [[ "$current" == "go${required_version}" ]]; then
    echo "using preinstalled go ${current}"
    exit 0
  fi
  echo "preinstalled go is ${current:-unknown}, need ${required_version}"
fi

# Keep go in workspace cache so all subsequent steps can consume it in this job.
# Keep using google.cn because upstream already aligned on that source.
cache_root=".cache/go/toolchain"
install_root="${cache_root}/go${required_version}.${goarch}"
archive="${cache_root}/go${required_version}.linux-${goarch}.tar.gz"

go_download_base="${GO_DOWNLOAD_BASE_URL:-https://golang.google.cn/dl}"
go_download_base="${go_download_base%/}"
url="${go_download_base}/go${required_version}.linux-${goarch}.tar.gz"

mkdir -p "$cache_root"

if [[ -x "$install_root/bin/go" ]]; then
  echo "using cached go in ${install_root}"
else
  if [[ -s "$archive" ]]; then
    echo "resuming ${required_version} download from ${url}"
    curl --silent --show-error --fail --location --continue-at - --connect-timeout 10 --max-time 600 --retry 3 --retry-delay 8 "$url" -o "$archive"
  else
    echo "downloading ${required_version} from ${url}"
    curl --silent --show-error --fail --location --connect-timeout 10 --max-time 600 --retry 3 --retry-delay 8 "$url" -o "$archive"
  fi
  tmp_dir="$(mktemp -d)"
  tar -C "$tmp_dir" -xzf "$archive"
  rm -rf "$install_root"
  mkdir -p "$(dirname "$install_root")"
  mv "$tmp_dir/go" "$install_root"
  rm -rf "$tmp_dir"
fi

echo "GOROOT=$(cd "$install_root" && pwd)" >>"${GITHUB_ENV:-/tmp/empty_env}"
if [[ -n "${GITHUB_PATH:-}" ]]; then
  echo "${install_root}/bin" >>"$GITHUB_PATH"
else
  export PATH="${install_root}/bin:$PATH"
fi
"${install_root}/bin/go" version
