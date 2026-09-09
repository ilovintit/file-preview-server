package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTC_R1_WPSContainerIdentity(t *testing.T) {
	data, err := os.ReadFile("../testdata/wps/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ File, SHA256, Source string }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 3 {
		t.Fatal("three native WPS families required")
	}
	for _, tc := range cases {
		t.Run(tc.File, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("../testdata/wps", tc.File))
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(body)
			if hex.EncodeToString(sum[:]) != tc.SHA256 || tc.Source == "" {
				t.Fatal("native source identity missing")
			}
			ext := officeInputExtension(tc.File)
			if ext == "" {
				t.Fatal("native WPS routing missing")
			}
			if err := validateOffice(context.Background(), body, ext); err != nil {
				t.Fatal("native WPS container rejected", err)
			}
			if outputVersion(tc.File) == outputVersion("source"+ext) || outputVersion(tc.File) == rawOutputVersion {
				t.Fatal("WPS interpretation shares an old cache identity")
			}
		})
	}
	body, err := os.ReadFile("../testdata/rejected/microsoft-works.wps")
	if err != nil {
		t.Fatal(err)
	}
	if validateOffice(context.Background(), body, officeInputExtension("file.wps")) == nil {
		t.Fatal("Microsoft Works accepted as Kingsoft WPS")
	}
}
