package infrastructure

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
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
	for _, offset := range []int{40, 44, 64, 72} {
		body := make([]byte, 512)
		copy(body, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
		binary.LittleEndian.PutUint16(body[30:32], 9)
		binary.LittleEndian.PutUint32(body[offset:offset+4], 0x7fffffff)
		if !errors.Is(validateOffice(context.Background(), body, ".doc"), entity.ErrInvalid) {
			t.Fatal("unbounded CFB allocation accepted")
		}
	}
}

func TestTC_S03_AC03_AdmissionAndWaitingBound(t *testing.T) {
	store := &PreviewStore{admission: make(chan struct{}, 36), slots: make(chan struct{}, 4)}
	for i := 0; i < cap(store.admission); i++ {
		store.admission <- struct{}{}
	}
	start := time.Now()
	if _, err := store.Prepare(context.Background(), entity.Grant{}); !errors.Is(err, entity.ErrUnavailable) {
		t.Fatal("full queue accepted request")
	}
	if time.Since(start) > time.Second {
		t.Fatal("full queue waited")
	}
	<-store.admission
	for i := 0; i < cap(store.slots); i++ {
		store.slots <- struct{}{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := store.Prepare(ctx, entity.Grant{}); !errors.Is(err, entity.ErrUnavailable) {
		t.Fatal("canceled waiter accepted")
	}
	if len(store.admission) != 35 || len(store.slots) != 4 {
		t.Fatal("waiter leaked admission or released another execution slot")
	}
}
