#!/usr/bin/env bash
# 镜像级端到端编排：构建实际应用镜像，在一次性 CI 网络里连接隔离的
# Valkey/Gotenberg/silo，跑通 Web 关键旅程、运行预算与重启恢复。
# 只使用本仓库内 fixture 与一次性凭据，不连接任何业务环境或集群。
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"
# shellcheck source=/dev/null
. .gitea/scripts/go-env.sh

toolchain="${PREVIEW_TOOLCHAIN_IMAGE:?PREVIEW_TOOLCHAIN_IMAGE 未设置}"
valkey_image="${PREVIEW_VALKEY_IMAGE:?PREVIEW_VALKEY_IMAGE 未设置}"
silo_image="${PREVIEW_SILO_IMAGE:?PREVIEW_SILO_IMAGE 未设置}"
gotenberg_image="${PREVIEW_GOTENBERG_IMAGE:?PREVIEW_GOTENBERG_IMAGE 未设置}"

run_id="${GITHUB_RUN_ID:-local}-$$"
network="preview-image-$run_id"
app_image="file-preview-server-image-e2e:$run_id"
materials="$root/.cache/image-e2e"
evidence="$root/.cache/image-evidence"
bucket="preview-ci-$(date +%s)"
# 一次性 fixture 凭据：只存在于本次运行的隔离网络。
silo_user="preview-ci"
silo_password="preview-ci-isolated-service-only"
internal_secret="$(openssl rand -hex 32)"
admin_secret="$(openssl rand -hex 32)"
caller_keys="$(python3 - "$internal_secret" "$admin_secret" <<'PY'
import base64, json, sys
internal, admin = sys.argv[1], sys.argv[2]
print(json.dumps([
    {"id": "ci-internal", "role": "internal", "secret_base64": base64.b64encode(internal.encode()).decode()},
    {"id": "ci-admin", "role": "admin", "secret_base64": base64.b64encode(admin.encode()).decode()},
]))
PY
)"

rm -rf "$materials" "$evidence"
mkdir -p "$materials" "$evidence"

cleanup() {
  status=$?
  for container in app harness prepare valkey gotenberg silo; do
    name="preview-$container-$run_id"
    docker logs "$name" >"$evidence/$container.log" 2>&1 || true
    docker rm -f "$name" >/dev/null 2>&1 || true
  done
  docker network rm "$network" >/dev/null 2>&1 || true
  docker image rm -f "$app_image" >/dev/null 2>&1 || true
  exit "$status"
}
trap cleanup EXIT

step() { printf '\n=== %s\n' "$1"; }

step "构建实际应用镜像"
GO_PROXY="$GOPROXY" docker build --platform linux/amd64 \
  --secret id=go_proxy,env=GO_PROXY \
  --label org.opencontainers.image.revision="${GITHUB_SHA:-local}" \
  --tag "$app_image" .

step "TC:S12-AC01 镜像拓扑与可追溯性"
docker image inspect "$app_image" --format '{{.Config.User}} {{index .Config.Labels "org.opencontainers.image.revision"}} {{json .Config.Cmd}}' | tee "$evidence/image-config.txt"
test "$(docker image inspect "$app_image" --format '{{.Config.User}}')" = preview
test "$(docker image inspect "$app_image" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}')" = "${GITHUB_SHA:-local}"
# 应用镜像只含 Go 二进制：Office 引擎属于独立 Gotenberg 镜像。
if docker run --rm --entrypoint sh "$app_image" -c 'command -v soffice || command -v libreoffice || command -v chromium' >/dev/null 2>&1; then
  echo "应用镜像不得包含 Office/浏览器引擎" >&2
  exit 1
fi

step "启动一次性依赖服务"
docker network create "$network" >/dev/null
docker run -d --name "preview-valkey-$run_id" --network "$network" --network-alias valkey "$valkey_image" >/dev/null
docker run -d --name "preview-silo-$run_id" --network "$network" --network-alias silo \
  -e MINIO_ROOT_USER="$silo_user" -e MINIO_ROOT_PASSWORD="$silo_password" "$silo_image" >/dev/null
docker run -d --name "preview-gotenberg-$run_id" --network "$network" --network-alias gotenberg \
  --memory 2g --cpus 2 --pids-limit 256 --tmpfs /tmp:rw,nosuid,nodev,size=536870912 \
  -e API_TIMEOUT=30s -e API_BODY_LIMIT=34MB -e API_DISABLE_DOWNLOAD_FROM=true \
  -e LIBREOFFICE_AUTO_START=true -e LIBREOFFICE_MAX_QUEUE_SIZE=4 -e LIBREOFFICE_DENY_LIST='.*' \
  -e LIBREOFFICE_DENY_PRIVATE_IPS=true -e LIBREOFFICE_DENY_PUBLIC_IPS=true -e CHROMIUM_DISABLE_ROUTES=true \
  "$gotenberg_image" >/dev/null

step "准备一次性证书与隔离 bucket"
docker run --rm --name "preview-prepare-$run_id" --network "$network" \
  -v "$root:/work" -w /work "$toolchain" \
  sh -lc ". .gitea/scripts/go-env.sh && go run ./tests/image/prepare -out .cache/image-e2e -silo-endpoint http://silo:9000 -silo-access-key $silo_user -silo-secret-key $silo_password -bucket $bucket"

start_app() {
  docker rm -f "preview-app-$run_id" >/dev/null 2>&1 || true
  # 运行预算与部署声明一致：只读根文件系统、受限 /tmp、内存与 CPU 上限。
  docker run -d --name "preview-app-$run_id" --network "$network" --network-alias app \
    --read-only --tmpfs /tmp:rw,nosuid,nodev,size=536870912 \
    --memory 1g --cpus 2 --pids-limit 512 \
    -v "$materials:/certs:ro" \
    -e SSL_CERT_FILE=/certs/ca.pem \
    -e LISTEN_ADDR=:9501 \
    -e TLS_CERT_FILE=/certs/app-cert.pem -e TLS_KEY_FILE=/certs/app-key.pem \
    -e KEY_NAMESPACE="preview:image-e2e" \
    -e VALKEY_ADDR=valkey:6379 \
    -e GOTENBERG_URL=http://gotenberg:3000 \
    -e STORAGE_PROFILES=silo \
    -e SILO_ENDPOINT=http://silo:9000 \
    -e SILO_PREVIEW_ENDPOINT=https://harness:8443 \
    -e SILO_REGION=us-east-1 \
    -e SILO_BUCKET="$bucket" \
    -e SILO_PREFIX_BASE=ci/preview \
    -e SILO_SIGNED_URL_MAX_TTL_SECONDS=300 \
    -e SILO_ACCESS_KEY_ID="$silo_user" \
    -e SILO_ACCESS_KEY_SECRET="$silo_password" \
    -e CALLER_KEYS_JSON="$caller_keys" \
    "$app_image" >/dev/null
}

probe() {
  docker run --rm --network "$network" -v "$root:/work" -w /work "$toolchain" \
    sh -lc ". .gitea/scripts/go-env.sh && go run ./tests/image/probe -url 'https://app:9501$1' -ca .cache/image-e2e/ca.pem"
}

wait_probe() {
  expected="$1"
  deadline=$((SECONDS + 180))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if [ "$(probe /readyz)" = "$expected" ]; then return 0; fi
    sleep 2
  done
  echo "readyz 未在期限内变为 $expected" >&2
  return 1
}

step "启动实际镜像进程"
start_app
wait_probe 200
test "$(probe /livez)" = 200

step "TC:S12-AC03 运行预算与可观测性"
docker inspect "preview-app-$run_id" --format '{{.HostConfig.ReadonlyRootfs}} {{.HostConfig.Memory}} {{.HostConfig.PidsLimit}}' | tee "$evidence/app-runtime.txt"
test "$(docker inspect "preview-app-$run_id" --format '{{.HostConfig.ReadonlyRootfs}}')" = true
# 结构无效的配置必须拒绝启动，而不是以降级状态提供服务。
if docker run --rm -e KEY_NAMESPACE='invalid namespace!' "$app_image" >"$evidence/invalid-config.log" 2>&1; then
  echo "无效配置未被拒绝" >&2
  exit 1
fi
# 依赖故障：Valkey 不可用时就绪探针关闭，恢复后自动回到就绪。
docker stop "preview-valkey-$run_id" >/dev/null
wait_probe 503
docker start "preview-valkey-$run_id" >/dev/null
wait_probe 200

step "TC:S12-AC05 与 TC:S12-AC02 Web 关键旅程与生产路由边界"
docker run --rm --name "preview-harness-$run_id" --network "$network" --network-alias harness \
  -v "$root:/work" -w /work \
  -e PREVIEW_IMAGE_MATERIALS=/work/.cache/image-e2e \
  -e PREVIEW_IMAGE_BASE_URL=https://app:9501 \
  -e PREVIEW_IMAGE_SILO_ENDPOINT=http://silo:9000 \
  -e PREVIEW_IMAGE_INTERNAL_KEY_ID=ci-internal -e PREVIEW_IMAGE_INTERNAL_KEY="$internal_secret" \
  -e PREVIEW_IMAGE_ADMIN_KEY_ID=ci-admin -e PREVIEW_IMAGE_ADMIN_KEY="$admin_secret" \
  "$toolchain" sh -lc ". .gitea/scripts/go-env.sh && go test -tags image -count=1 -v -run TestTC_S12_ImageWebJourney ./tests/image"

step "日志脱敏"
docker logs "preview-app-$run_id" >"$evidence/app.log" 2>&1
live_token="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["live_token"])' "$materials/state.json")"
if grep -q "$live_token" "$evidence/app.log"; then
  echo "应用日志泄漏了 token" >&2
  exit 1
fi
if grep -qE "$silo_password|X-Amz-Signature|$internal_secret|$admin_secret" "$evidence/app.log"; then
  echo "应用日志泄漏了凭据或签名" >&2
  exit 1
fi

step "TC:S12-AC04 重启后的授权连续性"
docker restart "preview-app-$run_id" >/dev/null
wait_probe 200
docker run --rm --name "preview-harness-$run_id" --network "$network" --network-alias harness \
  -v "$root:/work" -w /work \
  -e PREVIEW_IMAGE_MATERIALS=/work/.cache/image-e2e \
  -e PREVIEW_IMAGE_BASE_URL=https://app:9501 \
  -e PREVIEW_IMAGE_SILO_ENDPOINT=http://silo:9000 \
  -e PREVIEW_IMAGE_INTERNAL_KEY_ID=ci-internal -e PREVIEW_IMAGE_INTERNAL_KEY="$internal_secret" \
  -e PREVIEW_IMAGE_ADMIN_KEY_ID=ci-admin -e PREVIEW_IMAGE_ADMIN_KEY="$admin_secret" \
  "$toolchain" sh -lc ". .gitea/scripts/go-env.sh && go test -tags image -count=1 -v -run TestTC_S12_RecoveryAfterRestart ./tests/image"

step "结果"
echo "镜像级端到端验证完成；证据位于 .cache/image-evidence。"
echo "范围：仓库内一次性 CI 网络，不含任何集群部署或生产观察证据。"
