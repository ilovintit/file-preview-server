#!/usr/bin/env bash
set -euo pipefail
# All Go build/cache/temp outputs stay under the checkout, locally and in CI.
preview_root="$(git rev-parse --show-toplevel)"
export GOPATH="$preview_root/.cache/go"
export GOMODCACHE="$GOPATH/mod"
export GOCACHE="$GOPATH/build"
export GOTMPDIR="$preview_root/.cache/tmp"
export TMPDIR="$GOTMPDIR"
export GOTOOLCHAIN=local
mkdir -p "$GOMODCACHE" "$GOCACHE" "$GOTMPDIR"
# Public module proxy by default; set GOPROXY beforehand to use a mirror (e.g. https://goproxy.cn).
export GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}"
