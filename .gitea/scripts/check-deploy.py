"""CI-only structural gate for deploy/ declarations.

Scope: text-level structural constraints of the repository's own manifests.
Actual kustomize rendering is verified separately in the PR gate's fast job,
where the internal Go toolchain is available. Neither contacts any cluster,
proves a successful rollout, or is evidence that any environment is running.
"""
from pathlib import Path
import re
import sys

# 未登记的外部事实使用显式占位符，禁止编造 digest、域名或浮动标签。
PLACEHOLDERS = {
    "reg.shw.top/shw-project/file-preview-server:UNPINNED-SEE-RELEASE-RECORD",
    "reg.shw.top/shw-project/file-preview-playground:UNPINNED-SEE-RELEASE-RECORD",
    "preview-host-not-registered.invalid",
    "https://silo-host-not-registered.invalid",
}
FORBIDDEN = (
    (re.compile(r"^\s*kind:\s*Secret\s*$", re.M), "运行配置不得声明 Kubernetes Secret"),
    (re.compile(r"secretKeyRef|secretRef\b"), "运行配置不得使用 Secret 引用"),
    (re.compile(r"^\s*secret:\s*$", re.M), "不得挂载 Secret volume"),
)

manifests = sorted(Path("deploy").rglob("*.yaml"))
if not manifests:
    raise SystemExit("deploy/ 下没有声明文件")

images = []
for manifest in manifests:
    source = manifest.read_text(encoding="utf-8")
    for pattern, reason in FORBIDDEN:
        if pattern.search(source):
            raise SystemExit(f"{manifest}: {reason}")
    for value in re.findall(r"^\s*image:\s*(\S+)\s*$", source, re.M):
        images.append((manifest, value))

for manifest, value in images:
    if value in PLACEHOLDERS:
        continue
    if "@sha256:" not in value:
        raise SystemExit(f"{manifest}: 镜像必须固定 digest，不得使用浮动标签：{value}")
    if not value.startswith("reg.shw.top/"):
        raise SystemExit(f"{manifest}: 镜像必须来自内部仓库：{value}")

sidecar = Path("deploy/components/gotenberg/sidecar.yaml").read_text(encoding="utf-8")
for required in ("--api-bind-ip=127.0.0.1", "--api-disable-download-from=true", "--chromium-disable-routes=true"):
    if required not in sidecar:
        raise SystemExit(f"Gotenberg sidecar 缺少固定参数：{required}")

# playground 只允许出现在 dev overlay：生产与 test 不部署 demo fixture 入口。
for manifest in manifests:
    if "file-preview-playground" in manifest.read_text(encoding="utf-8") and not str(manifest).startswith("deploy/dev/"):
        raise SystemExit(f"{manifest}: playground 只能在 dev overlay 中声明")

service = Path("deploy/app/service.yaml").read_text(encoding="utf-8")
if "3000" in service:
    raise SystemExit("Service 不得暴露转换器端口")

deployment = Path("deploy/app/deployment.yaml").read_text(encoding="utf-8")
for required in ("/livez", "/readyz", "readOnlyRootFilesystem: true", "ephemeral-storage"):
    if required not in deployment:
        raise SystemExit(f"应用 Deployment 缺少必需声明：{required}")

unregistered = sorted({value for _, value in images if value in PLACEHOLDERS})
print(f"Manifests: {len(manifests)}; image references: {len(images)}; unpinned placeholders: {len(unregistered)}")
print("Scope: 仓库内声明的结构约束；未取得任何集群部署或健康证据。")
if unregistered:
    print("待发布记录写入 digest 的占位镜像：" + ", ".join(unregistered), file=sys.stderr)
