package httpapi

import (
	"archive/zip"
	"io"
	"path"
	"strings"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func zipErr(field, reason string) error {
	return &domain.ValidationError{Fields: map[string]string{field: reason}}
}

// zipJunk reports entries archivers add that are not part of a page.
func zipJunk(name string) bool {
	return strings.HasSuffix(name, "/") || name == "__MACOSX" || strings.HasPrefix(name, "__MACOSX/") ||
		path.Base(name) == ".DS_Store"
}

// readZip turns an uploaded zip into page files. It skips directories and
// macOS junk, strips one folder that holds everything, and checks the entry
// count and declared sizes before it reads any data.
func readZip(r io.ReaderAt, size int64) ([]domain.PageFileInput, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, zipErr("zip", "invalid")
	}
	var entries []*zip.File
	var total uint64
	for _, f := range zr.File {
		if zipJunk(f.Name) || f.FileInfo().IsDir() {
			continue
		}
		entries = append(entries, f)
		total += f.UncompressedSize64
		if len(entries) > domain.MaxPageFiles {
			return nil, zipErr("files", "too_many")
		}
		if f.UncompressedSize64 > domain.MaxPageFile || total > domain.MaxPageVersion {
			return nil, zipErr("files", "too_large")
		}
	}
	prefix := commonFolder(entries)
	out := make([]domain.PageFileInput, 0, len(entries))
	for _, f := range entries {
		rc, err := f.Open()
		if err != nil {
			return nil, zipErr("zip", "invalid")
		}
		data, err := io.ReadAll(io.LimitReader(rc, domain.MaxPageFile+1))
		_ = rc.Close()
		if err != nil {
			return nil, zipErr("zip", "invalid")
		}
		out = append(out, domain.PageFileInput{Path: strings.TrimPrefix(f.Name, prefix), Data: data})
	}
	return out, nil
}

// commonFolder returns "dir/" when every entry sits under that one folder.
func commonFolder(entries []*zip.File) string {
	if len(entries) == 0 {
		return ""
	}
	dir, _, ok := strings.Cut(entries[0].Name, "/")
	if !ok || dir == "" {
		return ""
	}
	prefix := dir + "/"
	for _, f := range entries[1:] {
		if !strings.HasPrefix(f.Name, prefix) {
			return ""
		}
	}
	return prefix
}
