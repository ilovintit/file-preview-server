"""Author a password-protected DOCM; not a preview-service test runner.

Requires msoffcrypto-tool 5.4.2 in the worktree-local fixture Python target.
Encryption is randomized: the committed ciphertext hash is frozen separately.
"""
from pathlib import Path
from io import BytesIO
import hashlib
import msoffcrypto

root = Path("internal/module/preview/testdata/text")
source = (root / "source.docm").read_bytes()
assert hashlib.sha256(source).hexdigest() == "d62c094cea9c3a617bcfb733700e03caca6401c294be9e53c3997b39208e9cd5"
output = BytesIO()
msoffcrypto.OfficeFile(BytesIO(source)).encrypt("S05-fixture-only", output)
ciphertext = output.getvalue()
# Authoring integrity: encrypted content must decode to the original DOCM.
document = msoffcrypto.OfficeFile(BytesIO(ciphertext))
assert document.is_encrypted()
document.load_key(password="S05-fixture-only")
plain = BytesIO()
document.decrypt(plain)
assert plain.getvalue() == source
(root / "negative" / "protected.docm").write_bytes(ciphertext)
print("Authored encrypted DOCM SHA-256:", hashlib.sha256(ciphertext).hexdigest())
