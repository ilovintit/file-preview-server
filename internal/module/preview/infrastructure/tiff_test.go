package infrastructure

import (
	"bytes"
	"context"
	"encoding/binary"
	"image/png"
	"os"
	"testing"
)

func TestTC_R1_TIFFFrameNormalization(t *testing.T) {
	body, err := os.ReadFile("../testdata/images/two-page.tiff")
	if err != nil {
		t.Fatal(err)
	}
	files, err := prepareTIFF(context.Background(), body)
	if err != nil || len(files) != 2 {
		t.Fatalf("expected two TIFF frames: %v", err)
	}
	for i, want := range [][3]uint32{{240, 32, 64}, {32, 64, 240}} {
		im, err := png.Decode(bytes.NewReader(files[i].body))
		if err != nil {
			t.Fatal(err)
		}
		r, g, b, _ := im.At(8, 8).RGBA()
		if r>>8 != want[0] || g>>8 != want[1] || b>>8 != want[2] {
			t.Fatal("normalization changed native pixels")
		}
	}
	cycle := append([]byte(nil), body...)
	binary.LittleEndian.PutUint32(cycle[130:134], 8)
	if _, err := prepareTIFF(context.Background(), cycle); err == nil {
		t.Fatal("cyclic TIFF accepted")
	}
	huge := append([]byte(nil), body...)
	binary.LittleEndian.PutUint32(huge[18:22], 0xffffffff)
	if _, err := prepareTIFF(context.Background(), huge); err == nil {
		t.Fatal("oversized TIFF accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := prepareTIFF(ctx, body); err == nil {
		t.Fatal("cancelled TIFF accepted")
	}
}
