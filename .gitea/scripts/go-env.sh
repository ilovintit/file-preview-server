#!/usr/bin/env bash
set -euo pipefail
# All Go build/cache/temp outputs stay under the checkout, locally and in CI.
preview_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export GOPATH="$preview_root/.cache/go"
export GOMODCACHE="$GOPATH/mod"
export GOCACHE="$GOPATH/build"
export GOTMPDIR="$preview_root/.cache/tmp"
export TMPDIR="$GOTMPDIR"
export GOTOOLCHAIN=local
export GONOSUMDB=git.shw.top
export GOPRIVATE=
export GONOPROXY=
mkdir -p "$GOMODCACHE" "$GOCACHE" "$GOTMPDIR"
# Company proxy anti-abuse credential is public per goproxy-auth; never print the URL.
export GOPROXY="https://gop:606fad8a50b9656d24aed5158251cc5a@gop.shw.top,direct"
