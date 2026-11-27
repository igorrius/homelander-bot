package i18n

import (
	"encoding/json"
	"io/fs"
	"slices"
	"sort"
	"testing"
	"testing/fstest"

	"github.com/igorrius/homelander-bot/locales"
)

var supported = []string{"uk", "en"}

func mustNew(t *testing.T, fsys fs.FS) *Bundle {
	t.Helper()
	b, err := New(fsys, "uk", supported)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}

func TestPickFallsBack(t *testing.T) {
	b := mustNew(t, locales.FS)
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"de"}, "uk"},
		{[]string{"en-GB"}, "en"},
		{[]string{"", "en"}, "en"},
		{[]string{"EN"}, "en"},
		{nil, "uk"},
	}
	for _, c := range cases {
		if got := b.Pick(c.in...); got != c.want {
			t.Errorf("Pick(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTUkrainianPlurals(t *testing.T) {
	b := mustNew(t, locales.FS)
	for n, want := range map[int]string{1: "1 квартира", 3: "3 квартири", 5: "5 квартир", 11: "11 квартир", 21: "21 квартира"} {
		if got := b.T("uk", "test.apartments", map[string]any{"Count": n}, n); got != want {
			t.Errorf("uk %d = %q, want %q", n, got, want)
		}
	}
	if got := b.T("en", "test.apartments", map[string]any{"Count": 2}, 2); got != "2 apartments" {
		t.Errorf("en 2 = %q", got)
	}
}

func TestTMissingInLocaleFallsBackToDefault(t *testing.T) {
	b := mustNew(t, fstest.MapFS{
		"uk.json": {Data: []byte(`{"only.uk": "Лише українською"}`)},
		"en.json": {Data: []byte(`{}`)},
	})
	if got := b.T("en", "only.uk", nil, nil); got != "Лише українською" {
		t.Errorf("got %q", got)
	}
}

func TestTMissingEverywhereReturnsID(t *testing.T) {
	b := mustNew(t, locales.FS)
	if got := b.T("uk", "no.such.key", nil, nil); got != "no.such.key" {
		t.Errorf("got %q", got)
	}
}

func flatten(prefix string, v any, out map[string]bool) {
	m, ok := v.(map[string]any)
	if !ok {
		out[prefix] = true
		return
	}
	// A plural message is an object of CLDR categories; it counts as one key.
	for k := range m {
		if slices.Contains([]string{"zero", "one", "two", "few", "many", "other"}, k) {
			out[prefix] = true
			return
		}
	}
	for k, child := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		flatten(key, child, out)
	}
}

func keysOf(t *testing.T, name string) map[string]bool {
	t.Helper()
	data, err := fs.ReadFile(locales.FS, name)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	out := map[string]bool{}
	flatten("", v, out)
	return out
}

func TestLocaleFilesHaveSameKeys(t *testing.T) {
	uk, en := keysOf(t, "uk.json"), keysOf(t, "en.json")
	var diff []string
	for k := range uk {
		if !en[k] {
			diff = append(diff, "missing in en.json: "+k)
		}
	}
	for k := range en {
		if !uk[k] {
			diff = append(diff, "missing in uk.json: "+k)
		}
	}
	sort.Strings(diff)
	for _, d := range diff {
		t.Error(d)
	}
}
