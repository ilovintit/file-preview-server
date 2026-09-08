package infrastructure

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestTC_S03_AC02_OutputVersionAndDeployment(t *testing.T) {
	versions := map[string]bool{rawOutputVersion: true}
	for _, extension := range []string{"doc", "docx", "xls", "xlsx", "ppt", "pptx"} {
		version := outputVersion("fixture." + extension)
		if versions[version] || !strings.HasPrefix(version, officeOutputVersion) {
			t.Fatal("Office interpretation must have a distinct output identity")
		}
		versions[version] = true
		if !outputMIME("fixture."+extension, "application/pdf") {
			t.Fatal("Office output must be PDF")
		}
	}
	for _, extension := range []string{"pdf", "jpg", "jpeg", "png", "gif", "webp", "avif"} {
		if outputVersion("fixture."+extension) != rawOutputVersion || coreOffice("fixture."+extension) {
			t.Fatal("raw output version changed")
		}
	}
	const digest = "0ec4b0a125c55ff1dfaed071cab30e0f37cc4326f7d53d903ee5b24e4f27fd85"
	if !strings.Contains(officeOutputVersion, digest) {
		t.Fatal("output identity lost converter digest")
	}
	for _, file := range []string{"../../../../deploy/components/gotenberg/sidecar.yaml", "../../../../.gitea/workflows/pr-gate.yml"} {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte("reg.shw.top/ci-cache/ci-gotenberg@sha256:"+digest)) {
			t.Fatal("deployment and CI converter images differ")
		}
	}
	sidecar, err := os.ReadFile("../../../../deploy/components/gotenberg/sidecar.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"name: file-preview-server", "name: gotenberg", "http://localhost:3000", "--api-bind-ip=127.0.0.1", "--libreoffice-deny-list=.*", "readOnlyRootFilesystem: true", "sizeLimit: 512Mi"} {
		if !bytes.Contains(sidecar, []byte(required)) {
			t.Fatalf("missing sidecar constraint %s", required)
		}
	}
}

func TestTC_S03_AC04_InputValidationAndCancellation(t *testing.T) {
	for _, extension := range []string{".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx"} {
		if validateOffice(context.Background(), []byte("garbage"), extension) == nil {
			t.Fatal("invalid Office input accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if validateOffice(ctx, []byte("garbage"), ".docx") == nil {
		t.Fatal("canceled input accepted")
	}
}
