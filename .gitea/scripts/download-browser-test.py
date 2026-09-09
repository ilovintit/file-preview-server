"""Download this run's checked browser test binary from the same Gitea repository."""
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
from urllib.parse import urlsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener
import zipfile

root = Path.cwd()
run = os.environ["PREVIEW_CI_RUN_ID"]
if not run.isdecimal():
    raise SystemExit("Invalid workflow run ID")
base = "https://git.shw.top/api/v1/repos/shw-project/file-preview-server/actions"
token = os.environ["PREVIEW_CI_REPOSITORY_TOKEN"]

class SameOriginRedirect(HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, url):
        destination = urlsplit(url)
        if destination.scheme != "https" or destination.netloc != "git.shw.top":
            raise SystemExit("Unexpected artifact redirect origin")
        return super().redirect_request(request, response, code, message, headers, url)

opener = build_opener(SameOriginRedirect())

def read(url, limit):
    request = Request(url, headers={"Authorization": "token " + token})
    with opener.open(request, timeout=60) as response:
        if not response.url.startswith("https://git.shw.top/"):
            raise SystemExit("Unexpected artifact origin")
        data = response.read(limit + 1)
        if len(data) > limit:
            raise SystemExit("Artifact exceeds its size budget")
        return data

items = json.loads(read(base + "/runs/" + run + "/artifacts", 1 << 20))["artifacts"]
artifacts = [a for a in items if a["name"] == "preview-browser-tests" and not a.get("expired")]
if len(artifacts) != 1:
    raise SystemExit("Expected exactly one browser test artifact for this run")
data = read(base + "/artifacts/" + str(artifacts[0]["id"]) + "/zip", 150 << 20)
expected = {"preview-browser.test", "head.txt", "sha256.txt"}
files = {}
with zipfile.ZipFile(io.BytesIO(data)) as archive:
    for item in archive.infolist():
        if item.is_dir():
            continue
        name = Path(item.filename).name
        if name not in expected or name in files or item.file_size > 150 << 20:
            raise SystemExit("Unexpected artifact contents")
        files[name] = archive.read(item)
if set(files) != expected:
    raise SystemExit("Incomplete browser test artifact")
head = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
if files["head.txt"].decode().strip() != head:
    raise SystemExit("Browser artifact belongs to another commit")
if hashlib.sha256(files["preview-browser.test"]).hexdigest() != files["sha256.txt"].decode().strip():
    raise SystemExit("Browser artifact checksum mismatch")
target = root / ".cache/browser-runner/preview-browser.test"
target.parent.mkdir(parents=True, exist_ok=True)
target.write_bytes(files["preview-browser.test"])
target.chmod(0o755)
print("Browser test artifact verified for commit " + head)
