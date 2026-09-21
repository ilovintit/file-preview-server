package infrastructure

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image/png"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
	"golang.org/x/image/tiff"
)

type pngBudget struct {
	bytes.Buffer
	ctx       context.Context
	remaining int
}

func (w *pngBudget) Write(data []byte) (int, error) {
	if w.ctx.Err() != nil {
		return 0, entity.ErrUnavailable
	}
	if len(data) > w.remaining {
		return 0, entity.ErrInvalid
	}
	w.remaining -= len(data)
	return w.Buffer.Write(data)
}

// Normalize each TIFF frame before LibreOffice import. The fixed converter loses
// colors for some valid deflate/alpha TIFF inputs; PNG keeps the decoded pixels.
// Preserve every IFD as a page, with one shared pixel/output budget.
func prepareTIFF(ctx context.Context, body []byte) ([]officeInput, error) {
	if len(body) < 8 || len(body) > 32<<20 {
		return nil, entity.ErrInvalid
	}
	var order binary.ByteOrder
	switch string(body[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return nil, entity.ErrInvalid
	}
	if order.Uint16(body[2:4]) != 42 {
		return nil, entity.ErrInvalid
	}
	offset := order.Uint32(body[4:8])
	input := append([]byte(nil), body...)
	seen := make(map[uint32]bool)
	var files []officeInput
	var pixels int64
	remaining := 32 << 20
	widths := [...]uint64{0, 1, 1, 2, 4, 8, 1, 1, 2, 4, 8, 4, 8, 4}
	for offset != 0 {
		if ctx.Err() != nil {
			return nil, entity.ErrUnavailable
		}
		if seen[offset] || len(seen) >= 128 || uint64(offset)+2 > uint64(len(body)) {
			return nil, entity.ErrInvalid
		}
		seen[offset] = true
		count := uint64(order.Uint16(body[offset:]))
		end := uint64(offset) + 2 + count*12
		if count > 4096 || end+4 > uint64(len(body)) {
			return nil, entity.ErrInvalid
		}
		tags := make(map[uint16]bool)
		var metadata uint64
		for pos := uint64(offset) + 2; pos < end; pos += 12 {
			tag := order.Uint16(body[pos:])
			kind := order.Uint16(body[pos+2:])
			var width uint64
			if int(kind) < len(widths) {
				width = widths[kind]
			}
			size := uint64(order.Uint32(body[pos+4:])) * width
			metadata += size
			if tags[tag] || width == 0 || metadata > 32<<20 {
				return nil, entity.ErrInvalid
			}
			tags[tag] = true
			if size > 4 && uint64(order.Uint32(body[pos+8:]))+size > uint64(len(body)) {
				return nil, entity.ErrInvalid
			}
		}
		order.PutUint32(input[4:8], offset)
		config, err := tiff.DecodeConfig(bytes.NewReader(input))
		if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width) > (40000000-pixels)/int64(config.Height) {
			return nil, entity.ErrInvalid
		}
		pixels += int64(config.Width) * int64(config.Height)
		decoded, err := tiff.Decode(bytes.NewReader(input))
		if err != nil {
			return nil, entity.ErrInvalid
		}
		encoded := &pngBudget{ctx: ctx, remaining: remaining}
		if err := png.Encode(encoded, decoded); err != nil {
			if ctx.Err() != nil {
				return nil, entity.ErrUnavailable
			}
			return nil, entity.ErrInvalid
		}
		remaining = encoded.remaining
		files = append(files, officeInput{name: fmt.Sprintf("page-%03d.png", len(files)), body: encoded.Bytes()})
		offset = order.Uint32(body[end : end+4])
	}
	if len(files) == 0 {
		return nil, entity.ErrInvalid
	}
	return files, nil
}
