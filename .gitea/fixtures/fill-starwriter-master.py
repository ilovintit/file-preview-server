"""Author a nonempty native SGL fixture from the pinned empty StarWriter 5 master.

Preserves the master-document CLSID/header and all other streams. Updates the
native N/T paragraph records, document statistics, and CFB stream size. The new
text fits the stream's already allocated 13 mini-sectors; no FAT changes occur.
This is not a filename-only format substitution.
"""
from pathlib import Path
import hashlib
import json
import struct
import olefile

root = Path('internal/module/preview/testdata/text')
path = root / 'legacy.sgl'
provenance = json.loads((root / 'upstream.json').read_text())
entry = next(item for item in provenance if item['file'] == 'legacy.sgl')
original = path.read_bytes()
assert hashlib.sha1(b'blob ' + str(len(original)).encode() + b'\0' + original).hexdigest() == '5a5bcb7526a1879328555774e27fba5348155f83'
with olefile.OleFileIO(path) as compound:
    stream = compound.openstream('StarWriterDocument').read()
    clsid = compound.root.clsid
assert len(stream) == 808 and stream.startswith(b'SW5HDR\0')
empty = bytes.fromhex('4e1400000401000000540b0000040300feff0000')
assert stream.count(empty) == 1
text = b'File preview S05 Text 42'
assert len(text) == 24
paragraph = b'N' + (20 + len(text)).to_bytes(3, 'little') + bytes.fromhex('0401000000')
paragraph += b'T' + (11 + len(text)).to_bytes(3, 'little') + bytes.fromhex('040300feff')
paragraph += struct.pack('<H', len(text)) + text
updated = bytearray(stream.replace(empty, paragraph))
statistics = updated.index(bytes.fromhex('641b0000'))
assert struct.unpack_from('<IIII', updated, statistics + 10) == (1, 1, 0, 0)
struct.pack_into('<II', updated, statistics + 18, len(text.split()), len(text))
assert len(updated) == 832 and (len(stream) + 63) // 64 == len(updated) // 64

container = bytearray(original)
name = 'StarWriterDocument'.encode('utf-16le') + b'\0\0'
assert container.count(name) == 1
directory = container.index(name)
assert directory % 128 == 0 and container[directory + 66] == 2
assert struct.unpack_from('<Q', container, directory + 120)[0] == len(stream)
struct.pack_into('<Q', container, directory + 120, len(updated))
path.write_bytes(container)
with olefile.OleFileIO(path, write_mode=True) as compound:
    compound.write_stream('StarWriterDocument', bytes(updated))
with olefile.OleFileIO(path) as compound:
    assert compound.root.clsid == clsid
    assert compound.openstream('StarWriterDocument').read() == bytes(updated)

entry['originalSha256'] = entry['sha256']
entry['sha256'] = hashlib.sha256(path.read_bytes()).hexdigest()
entry['derivation'] = 'fill-starwriter-master.py: native paragraph/statistics/stream-size update; original master CLSID and header preserved'
(root / 'upstream.json').write_text(json.dumps(provenance, ensure_ascii=False, indent=2) + '\n')
print('Authored SGL text in native records, preserving master CLSID:', clsid)
