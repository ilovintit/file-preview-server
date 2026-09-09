package infrastructure

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestTC_R1_RejectDisguisedOOXMLTypes(t *testing.T) {
	cases := []struct{ file, ext, from, to string }{
		{"document.docx", ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml", "application/vnd.ms-word.document.macroEnabled.main+xml"},
		{"document.docx", ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.template.main+xml"},
		{"sheet.xlsx", ".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml", "application/vnd.ms-excel.sheet.macroEnabled.main+xml"},
		{"slides.pptx", ".pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml", "application/vnd.ms-powerpoint.presentation.macroEnabled.main+xml"},
	}
	for _, tc := range cases {
		t.Run(tc.to, func(t *testing.T) {
			original, err := os.ReadFile("../testdata/office/" + tc.file)
			if err != nil {
				t.Fatal(err)
			}
			archive, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			writer := zip.NewWriter(&output)
			changed := false
			for _, entry := range archive.File {
				reader, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(reader)
				_ = reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				if entry.Name == "[Content_Types].xml" {
					if !bytes.Contains(data, []byte(tc.from)) {
						t.Fatal("source fixture content type changed")
					}
					data = []byte(strings.ReplaceAll(string(data), tc.from, tc.to))
					changed = true
				}
				part, err := writer.Create(entry.Name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = part.Write(data); err != nil {
					t.Fatal(err)
				}
			}
			if err = writer.Close(); err != nil {
				t.Fatal(err)
			}
			if !changed {
				t.Fatal("fixture mutation did not occur")
			}
			if validateOffice(context.Background(), output.Bytes(), tc.ext) == nil {
				t.Fatal("macro/template OOXML accepted under a normal Office suffix")
			}
		})
	}
}
