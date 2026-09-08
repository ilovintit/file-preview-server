//go:build integration

package preview_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTC_S04_AC01_SiloPreviewContract(t *testing.T) {
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(rawPDF)
	}))
	defer source.Close()
	f := setupS02(t, source.Client())
	input := resource()
	input["storage_profile"], input["url"], input["content_sha256"] = "silo", source.URL, sha256Hex(rawPDF)
	token := issueResource(t, f, input)
	client := f.server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r, err := client.Get(f.server.URL + "/v/" + token)
	if err != nil {
		t.Fatal("silo preview request failed")
	}
	defer r.Body.Close()
	if r.StatusCode != 302 {
		t.Fatalf("silo preview expected302 got%d", r.StatusCode)
	}
}
