package httpapi

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// mkZip builds a zip from name → body; a name ending in "/" is a directory.
func mkZip(t *testing.T, files [][2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.Create(f[0])
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(f[1]))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipPaths(t *testing.T, b []byte) map[string]string {
	t.Helper()
	files, err := readZip(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range files {
		out[f.Path] = string(f.Data)
	}
	return out
}

func wantZipErr(t *testing.T, err error, field, reason string) {
	t.Helper()
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Fields[field] != reason {
		t.Fatalf("err = %v; want %s: %s", err, field, reason)
	}
}

func TestReadZip(t *testing.T) {
	got := zipPaths(t, mkZip(t, [][2]string{
		{"site/", ""}, {"site/index.html", "i"}, {"site/css/", ""}, {"site/css/app.css", "c"},
		{"__MACOSX/site/._index.html", "junk"}, {"site/.DS_Store", "junk"}, {"site/img/.DS_Store", "junk"},
	}))
	if want := map[string]string{"index.html": "i", "css/app.css": "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("stripped = %v", got)
	}
	// No common folder: paths stay.
	got = zipPaths(t, mkZip(t, [][2]string{{"index.html", "i"}, {"css/app.css", "c"}}))
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"css/app.css", "index.html"}) {
		t.Fatalf("plain = %v", keys)
	}
	// Two different top folders: nothing is stripped.
	got = zipPaths(t, mkZip(t, [][2]string{{"a/index.html", "i"}, {"b/x.css", "c"}}))
	if _, ok := got["a/index.html"]; !ok {
		t.Fatalf("two folders = %v", got)
	}

	// 21 MiB of zeros compresses to a few KiB; the declared size rejects it.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("index.html")
	_, _ = w.Write(make([]byte, 21<<20))
	_ = zw.Close()
	_, err := readZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	wantZipErr(t, err, "files", "too_large")

	var many [][2]string
	for i := range 201 {
		many = append(many, [2]string{fmt.Sprintf("f%d.css", i), "x"})
	}
	b := mkZip(t, many)
	_, err = readZip(bytes.NewReader(b), int64(len(b)))
	wantZipErr(t, err, "files", "too_many")

	_, err = readZip(bytes.NewReader([]byte("not a zip")), 9)
	wantZipErr(t, err, "zip", "invalid")
}
