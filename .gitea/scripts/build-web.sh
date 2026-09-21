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
  : "${NPM_TOKEN:?NPM_TOKEN is required for the company npm cache}"
  export NPM_CONFIG_USERCONFIG="$preview_root/.cache/web-npmrc"
  trap 'rm -f "$NPM_CONFIG_USERCONFIG"' EXIT
  node --input-type=module -e '
    import { writeFileSync } from "node:fs";
    const auth = Buffer.from("shared:" + process.env.NPM_TOKEN).toString("base64");
    writeFileSync(process.env.NPM_CONFIG_USERCONFIG, "registry=https://npm.shw.top/\n//npm.shw.top/:_auth=" + auth + "\n", { mode: 0o600 });
  '
  npm ci --ignore-scripts --no-audit --no-fund --include=optional
elif [ "${CI:-}" = true ]; then
  echo "CI may not skip the locked frontend install" >&2
  exit 1
fi

node scripts/assets.mjs prepare
npm run typecheck
npm run build
node scripts/assets.mjs copy
