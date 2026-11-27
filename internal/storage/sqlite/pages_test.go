package sqlite_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite/sqlitetest"
)

func pageFile(path, body string) domain.PageFile {
	return domain.PageFile{Path: path, ContentType: "text/plain", Data: []byte(body), SHA256: "h-" + body, Size: len(body)}
}

func newPage(t *testing.T, r sqlite.Pages) domain.PageVersion {
	t.Helper()
	v := domain.PageVersion{Status: domain.PageDraft, Note: "n", CreatedAt: t0, UpdatedAt: t0}
	if err := r.Create(ctx, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestPageVersionCRUD(t *testing.T) {
	r := sqlite.Pages{DB: sqlitetest.New(t)}
	v := newPage(t, r)
	if v.ID == 0 {
		t.Fatal("no id")
	}
	if err := r.PutFiles(ctx, v.ID, []domain.PageFile{pageFile("style.css", "b"), pageFile("index.html", "aa")}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.PageFile{
		{Path: "index.html", ContentType: "text/plain", SHA256: "h-aa", Size: 2},
		{Path: "style.css", ContentType: "text/plain", SHA256: "h-b", Size: 1},
	}
	if !reflect.DeepEqual(got.Files, want) || got.TotalSize != 3 || got.Status != domain.PageDraft || got.Note != "n" ||
		!got.CreatedAt.Equal(t0) {
		t.Fatalf("Get = %+v", got)
	}
	f, err := r.File(ctx, v.ID, "index.html")
	if err != nil || string(f.Data) != "aa" || f.SHA256 != "h-aa" {
		t.Fatalf("File = %+v, %v", f, err)
	}
	if _, err := r.File(ctx, v.ID, "nope.html"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing file = %v", err)
	}
	if err := r.PutFiles(ctx, v.ID, []domain.PageFile{pageFile("index.html", "ccc")}); err != nil {
		t.Fatal(err)
	}
	if f, _ := r.File(ctx, v.ID, "index.html"); string(f.Data) != "ccc" {
		t.Fatalf("replaced = %q", f.Data)
	}
	all, err := r.Files(ctx, v.ID)
	if err != nil || len(all) != 2 || string(all[1].Data) != "b" {
		t.Fatalf("Files = %+v, %v", all, err)
	}
	if err := r.DeleteFiles(ctx, v.ID, []string{"style.css"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Get(ctx, v.ID); len(got.Files) != 1 || got.TotalSize != 3 {
		t.Fatalf("after DeleteFiles = %+v", got)
	}
	list, err := r.List(ctx)
	if err != nil || len(list) != 1 || len(list[0].Files) != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if err := r.Delete(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Get(ctx, v.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get deleted = %v", err)
	}
	if _, err := r.File(ctx, v.ID, "index.html"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("file of deleted = %v", err)
	}
	if err := r.Delete(ctx, v.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Delete twice = %v", err)
	}
}

func TestOnePublished(t *testing.T) {
	r := sqlite.Pages{DB: sqlitetest.New(t)}
	if _, err := r.Published(ctx); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Published none = %v", err)
	}
	a, b := newPage(t, r), newPage(t, r)
	at := t0.Add(time.Hour)
	a.Status, a.PublishedAt, a.UpdatedAt = domain.PagePublished, &at, at
	if err := r.Update(ctx, &a); err != nil {
		t.Fatal(err)
	}
	b.Status = domain.PagePublished
	if err := r.Update(ctx, &b); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second published = %v; want ErrConflict", err)
	}
	p, err := r.Published(ctx)
	if err != nil || p.ID != a.ID || p.PublishedAt == nil || !p.PublishedAt.Equal(at) {
		t.Fatalf("Published = %+v, %v", p, err)
	}
	list, _ := r.List(ctx)
	if len(list) != 2 || list[0].ID != b.ID {
		t.Fatalf("List order = %+v", list)
	}
}
