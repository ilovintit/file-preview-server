"""CI-only document link and embedded JavaScript syntax gate."""
from html.parser import HTMLParser
from pathlib import Path
import re
import subprocess
from urllib.parse import unquote, urlsplit


class Document(HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = []
        self.scripts = []
        self.current = None

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        for key in ("href", "src"):
            if key in attrs:
                self.links.append(attrs[key])
        if tag == "script" and "src" not in attrs:
            self.current = []

    def handle_data(self, data):
        if self.current is not None:
            self.current.append(data)

    def handle_endtag(self, tag):
        if tag == "script" and self.current is not None:
            self.scripts.append("".join(self.current))
            self.current = None


root = Path.cwd().resolve()
documents = sorted(Path("docs").rglob("*.md")) + sorted(Path("docs").rglob("*.html"))
link_count = script_count = 0
for document in documents:
    source = document.read_text(encoding="utf-8")
    if document.suffix == ".html":
        parsed = Document()
        parsed.feed(source)
        links = parsed.links
        for script in parsed.scripts:
            subprocess.run(["node", "--check"], input=script, text=True, check=True)
            script_count += 1
    else:
        links = re.findall(r"\[[^\]]*\]\(([^\s)]+)\)", source)
    for link in links:
        target = urlsplit(link)
        if target.scheme or target.netloc or not target.path:
            continue
        resolved = (document.parent / unquote(target.path)).resolve()
        if root not in resolved.parents or not resolved.is_file():
            raise SystemExit(f"Invalid local link: {document}: {link}")
        link_count += 1
print(f"Documents: {len(documents)}; local links: {link_count}; JavaScript blocks: {script_count}; PASS")
print("Scope: local link targets and script syntax only; no browser, service or device acceptance.")
