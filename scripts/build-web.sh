#!/bin/sh
set -eu
preview_root="$(git rev-parse --show-toplevel)"
export NUXT_TELEMETRY_DISABLED=1
export NPM_CONFIG_UPDATE_NOTIFIER=false
export NPM_CONFIG_CACHE="$preview_root/.cache/npm"
export TMPDIR="$preview_root/.cache/tmp"
mkdir -p "$NPM_CONFIG_CACHE" "$TMPDIR"
cd "$preview_root/demo/preview-h5"

if [ "${PREVIEW_WEB_INSTALL:-1}" = 1 ]; then
  npm ci --ignore-scripts --no-audit --no-fund --include=optional
elif [ "${CI:-}" = true ]; then
  echo "CI may not skip the locked frontend install" >&2
  exit 1
fi

node scripts/assets.mjs prepare
npm run typecheck
npm run build
node scripts/assets.mjs copy
