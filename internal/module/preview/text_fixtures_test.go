package preview_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type textFixture struct {
	Extension string          `json:"extension"`
	File      string          `json:"file"`
	SHA256    string          `json:"sha256"`
	Pages     int             `json:"pages"`
	Text      []string        `json:"text"`
	ExactText string          `json:"exactText"`
	Source    json.RawMessage `json:"source"`
}

func textFixtures(t *testing.T) []textFixture {
	t.Helper()
	body, err := os.ReadFile("testdata/text/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []textFixture
	if err = json.Unmarshal(body, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func TestTC_S05_AC01_FixtureInventory(t *testing.T) {
	expected := strings.Fields("docm dot dotm dotx odt fodt ott rtf txt wps wpd pages abw zabw lwp mw mcw hwp sxw stw sgl vor 602 bib xml cwk psw uof")
	var actual []string
	seen := map[string]bool{}
	for _, fixture := range textFixtures(t) {
		if seen[fixture.Extension] || filepath.Ext(fixture.File) != "."+fixture.Extension || filepath.Base(fixture.File) != fixture.File {
			t.Fatal("duplicate or invalid text fixture")
		}
		seen[fixture.Extension] = true
		actual = append(actual, fixture.Extension)
		body, err := os.ReadFile(filepath.Join("testdata/text", fixture.File))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		if hex.EncodeToString(digest[:]) != fixture.SHA256 || len(body) == 0 || fixture.Pages < 1 || len(fixture.Text) == 0 || len(fixture.Source) == 0 {
			t.Fatalf("incomplete fixture hash/source/expectation: %s", fixture.File)
		}
	}
	sort.Strings(expected)
	sort.Strings(actual)
	if strings.Join(expected, ",") != strings.Join(actual, ",") {
		t.Fatalf("S05 exact28 inventory mismatch: %v", actual)
	}
}
