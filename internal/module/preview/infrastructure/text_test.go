package infrastructure

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestTC_S05_AC04_LegacyDocumentDTD(t *testing.T) {
	original, err := os.ReadFile("../testdata/text/source.sxw")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, declaration string
		bundled, accepted bool
	}{
		{"standard", `<!DOCTYPE office:document-content PUBLIC "-//OpenOffice.org//DTD OfficeDocument 1.0//EN" "office.dtd">`, false, true},
		{"external", `<!DOCTYPE office:document-content SYSTEM "https://resource.invalid/office.dtd">`, false, false},
		{"local", `<!DOCTYPE office:document-content SYSTEM "file:///etc/passwd">`, false, false},
		{"bundled", "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			archive, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
			if err != nil {
				t.Fatal(err)
			}
			var buffer bytes.Buffer
			writer := zip.NewWriter(&buffer)
			for _, entry := range archive.File {
				stream, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				payload, err := io.ReadAll(stream)
				stream.Close()
				if err != nil {
					t.Fatal(err)
				}
				if entry.Name == "content.xml" {
					payload = bytes.Replace(payload, []byte("?>"), []byte("?>"+tc.declaration), 1)
				}
				part, err := writer.Create(entry.Name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := part.Write(payload); err != nil {
					t.Fatal(err)
				}
			}
			if tc.bundled {
				part, err := writer.Create("office.dtd")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(part, `<!ENTITY external SYSTEM "file:///etc/passwd">`); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			prepared, _, err := prepareText(context.Background(), buffer.Bytes(), "source.sxw")
			if (err == nil) != tc.accepted {
				t.Fatalf("accepted=%t, error=%v", tc.accepted, err)
			}
			if tc.accepted {
				archive, err := zip.NewReader(bytes.NewReader(prepared), int64(len(prepared)))
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range archive.File {
					stream, err := entry.Open()
					if err != nil {
						t.Fatal(err)
					}
					body, err := io.ReadAll(stream)
					stream.Close()
					if err != nil || bytes.Contains(body, []byte("<!DOCTYPE")) {
						t.Fatal("DTD survived normalization")
					}
				}
			}
		})
	}
}

func TestTC_S05_AC04_TextEscapingAndLimits(t *testing.T) {
	for _, input := range []string{`<xml><image href="file:///etc/passwd"/></xml>`, `{\rtf1 suspicious}`} {
		body, name, err := prepareText(context.Background(), []byte(input), "source.txt")
		if err != nil || name != "source.fodt" || bytes.Contains(body, []byte("<xml>")) {
			t.Fatal("plain text was not escaped into controlled FODT")
		}
		if _, err := validateTextXML(context.Background(), body); err != nil {
			t.Fatal("invalid text wrapper")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := prepareText(ctx, []byte("text"), "source.txt"); err == nil {
		t.Fatal("canceled preparation continued")
	}
	if _, _, err := prepareText(context.Background(), []byte{0xff, 0xfe, 0x00}, "source.txt"); err == nil {
		t.Fatal("odd UTF16 accepted")
	}
	if _, _, err := prepareText(context.Background(), []byte(strings.Repeat("&", 7<<20)), "source.txt"); err == nil {
		t.Fatal("escaped payload exceeds bound")
	}
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	chunk := make([]byte, 1<<20)
	for i := 0; i < 65; i++ {
		if _, err := compressed.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepareText(context.Background(), buffer.Bytes(), "source.zabw"); err == nil {
		t.Fatal("gzip expansion exceeds bound")
	}
}

func TestTC_S05_AC01_StarWriterMasterIdentity(t *testing.T) {
	data, err := os.ReadFile("../testdata/text/legacy.sgl")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepareText(context.Background(), data, "master.sgl"); err != nil {
		t.Fatal("native master CLSID not accepted")
	}
	if _, _, err := prepareText(context.Background(), data, "not-a-template.vor"); err == nil {
		t.Fatal("master document accepted as ordinary template")
	}
}
