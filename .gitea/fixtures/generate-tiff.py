"""Create a real, uncompressed two-IFD RGB TIFF with known page colors."""
from pathlib import Path
import struct

size = 16
entry_count = 10
frame_bytes = 2 + entry_count * 12 + 4 + 6 + size * size * 3
first = 8
second = first + frame_bytes

def frame(offset, following, color):
    bits = offset + 2 + entry_count * 12 + 4
    pixels = bits + 6
    entries = [(256, 4, 1, size), (257, 4, 1, size), (258, 3, 3, bits),
               (259, 3, 1, 1), (262, 3, 1, 2), (273, 4, 1, pixels),
               (277, 3, 1, 3), (278, 4, 1, size),
               (279, 4, 1, size * size * 3), (284, 3, 1, 1)]
    return (struct.pack('<H', entry_count)
            + b''.join(struct.pack('<HHII', *entry) for entry in entries)
            + struct.pack('<I', following) + struct.pack('<HHH', 8, 8, 8)
            + bytes(color) * (size * size))

body = (b'II' + struct.pack('<HI', 42, first)
        + frame(first, second, (240, 32, 64))
        + frame(second, 0, (32, 64, 240)))
Path('internal/module/preview/testdata/images/two-page.tiff').write_bytes(body)
