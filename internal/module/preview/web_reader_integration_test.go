//go:build integration

package preview_test

import (
	"io"
	"strings"
	"testing"
)

func TestTC_S11_ProductionReaderEntry(t *testing.T) {
	f := setup(t)
	r, err := f.server.Client().Get(f.server.URL + "/reader/")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != 200 || !strings.HasPrefix(r.Header.Get("Content-Type"), "text/html") || !strings.Contains(string(body), "__nuxt") {
		t.Fatalf("actual Web reader entry missing: HTTP %d type %q", r.StatusCode, r.Header.Get("Content-Type"))
	}
	if r.Header.Get("Cache-Control") != "no-store" || r.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("reader entry must not cache or leak navigation context")
	}
}

func TestTC_S11_TestFixtureAPIAbsentInProduction(t *testing.T) {
	f := setup(t)
	for _, path := range []string{"/demo/fixtures", "/demo/fixtures/pdf/preview"} {
		r, err := f.server.Client().Get(f.server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != 404 {
			t.Fatalf("production exposed test API: %s", path)
		}
	}
}
