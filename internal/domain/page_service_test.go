package domain_test

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func pin(path, body string) domain.PageFileInput {
	return domain.PageFileInput{Path: path, Data: []byte(body)}
}

func index() domain.PageFileInput { return pin("index.html", "<h1>hi</h1>") }

func wantPageErr(t *testing.T, err error, reason string) {
	t.Helper()
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Fields["files"] != reason {
		t.Fatalf("err = %v; want files: %s", err, reason)
	}
}

func TestPagePaths(t *testing.T) {
	e := newEnv(t)
	a := actor(e.mkUser("+380501110001", admin))
	for path, reason := range map[string]string{
		"../x.html": "invalid", "/abs.html": "invalid", "a//b.css": "invalid", `a\b.css`: "invalid",
		"x.HTML": "invalid", "x.exe": "type", ".hidden": "invalid", "a/../b.css": "invalid", "": "invalid",
		strings.Repeat("a", 200) + ".css": "invalid", "noext": "type",
	} {
		_, err := e.pages.CreateDraft(ctx, []domain.PageFileInput{index(), pin(path, "x")}, "", a)
		if err == nil {
			t.Errorf("%q accepted", path)
			continue
		}
		var ve *domain.ValidationError
		if !errors.As(err, &ve) || ve.Fields["files"] != reason {
			t.Errorf("%q: err = %v; want files: %s", path, err, reason)
		}
	}
	v, err := e.pages.CreateDraft(ctx, []domain.PageFileInput{index(), pin("css/app.css", "b{}"), pin("img/a-1.png", "p")}, "", a)
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	for _, f := range v.Files {
		types[f.Path] = f.ContentType
	}
	want := map[string]string{"index.html": "text/html; charset=utf-8", "css/app.css": "text/css; charset=utf-8", "img/a-1.png": "image/png"}
	if !reflect.DeepEqual(types, want) || v.Status != domain.PageDraft {
		t.Fatalf("files = %v, status %s", types, v.Status)
	}
	if _, err := e.pages.CreateDraft(ctx, []domain.PageFileInput{index(), index()}, "", a); err == nil {
		t.Fatal("duplicate path accepted")
	}
	var ve *domain.ValidationError
	if _, err := e.pages.CreateDraft(ctx, []domain.PageFileInput{index()}, strings.Repeat("я", 201), a); !errors.As(err, &ve) || ve.Fields["note"] != "too_long" {
		t.Fatalf("long note = %v", err)
	}
}

func TestPageLimits(t *testing.T) {
	e := newEnv(t)
	a := actor(e.mkUser("+380501110001", admin))
	big := domain.PageFileInput{Path: "big.png", Data: make([]byte, 5<<20+1)}
	_, err := e.pages.CreateDraft(ctx, []domain.PageFileInput{index(), big}, "", a)
	wantPageErr(t, err, "too_large")

	many := []domain.PageFileInput{index()}
	for i := range 200 {
		many = append(many, pin(fmt.Sprintf("f%d.css", i), "x"))
	}
	_, err = e.pages.CreateDraft(ctx, many, "", a)
	wantPageErr(t, err, "too_many")

	five := []domain.PageFileInput{index()}
	for i := range 5 {
		five = append(five, domain.PageFileInput{Path: fmt.Sprintf("p%d.png", i), Data: make([]byte, 4<<20+512<<10)})
	}
	_, err = e.pages.CreateDraft(ctx, five, "", a)
	wantPageErr(t, err, "too_large")

	_, err = e.pages.CreateDraft(ctx, []domain.PageFileInput{pin("a.css", "x")}, "", a)
	wantPageErr(t, err, "index_required")

	v, err := e.pages.CreateDraft(ctx, []domain.PageFileInput{index(), pin("a.css", "x")}, "", a)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.pages.PutFiles(ctx, v.ID, nil, []string{"index.html"}, a)
	wantPageErr(t, err, "index_required")
	// The version total counts files already stored.
	var put []domain.PageFileInput
	for i := range 5 {
		put = append(put, domain.PageFileInput{Path: fmt.Sprintf("p%d.png", i), Data: make([]byte, 4<<20)})
	}
	if _, err := e.pages.PutFiles(ctx, v.ID, put[:4], nil, a); err != nil {
		t.Fatal(err)
	}
	_, err = e.pages.PutFiles(ctx, v.ID, put[4:], nil, a)
	wantPageErr(t, err, "too_large")
	got, err := e.pages.PutFiles(ctx, v.ID, []domain.PageFileInput{pin("a.css", "yy")}, []string{"p0.png"}, a)
	if err != nil || len(got.Files) != 5 || e.auditCount(domain.ActPageDraftUpdated) != 2 {
		t.Fatalf("PutFiles = %+v, %v", got.Files, err)
	}
}

func auditActions(t *testing.T, e *env) []string {
	t.Helper()
	rows, err := e.db.Query(`SELECT action FROM audit_log WHERE action LIKE 'page.%' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		out = append(out, s)
	}
	return out
}

func TestPageLifecycle(t *testing.T) {
	e := newEnv(t)
	u := e.mkUser("+380501110001", admin)
	a := actor(u)
	va, err := e.pages.CreateDraft(ctx, []domain.PageFileInput{pin("index.html", "A"), pin("a.css", "a")}, " first ", a)
	if err != nil || va.Note != "first" || va.CreatedBy == nil || *va.CreatedBy != u.ID {
		t.Fatalf("CreateDraft = %+v, %v", va, err)
	}
	vb, err := e.pages.CreateDraft(ctx, []domain.PageFileInput{pin("index.html", "B")}, "", a)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := e.pages.Published(ctx); ok {
		t.Fatal("published before publish")
	}
	if err := e.pages.Publish(ctx, va.ID, a); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(time.Minute)
	if err := e.pages.Publish(ctx, vb.ID, a); err != nil {
		t.Fatal(err)
	}
	if err := e.pages.Publish(ctx, vb.ID, a); err != nil {
		t.Fatalf("publish again = %v", err)
	}
	ga, _ := e.pages.Get(ctx, va.ID)
	gb, _ := e.pages.Get(ctx, vb.ID)
	if ga.Status != domain.PageArchived || gb.Status != domain.PagePublished || gb.PublishedBy == nil ||
		gb.PublishedAt == nil || !gb.PublishedAt.Equal(e.clock.t) {
		t.Fatalf("statuses A=%s B=%+v", ga.Status, gb)
	}
	p, ok, err := e.pages.Published(ctx)
	if err != nil || !ok || p.ID != vb.ID {
		t.Fatalf("Published = %+v, %v, %v", p, ok, err)
	}
	f, err := e.pages.PublishedFile(ctx, "index.html")
	if err != nil || string(f.Data) != "B" {
		t.Fatalf("PublishedFile = %+v, %v", f, err)
	}
	if _, err := e.pages.PutFiles(ctx, vb.ID, []domain.PageFileInput{pin("x.css", "x")}, nil, a); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("PutFiles published = %v", err)
	}
	if _, err := e.pages.PutFiles(ctx, va.ID, []domain.PageFileInput{pin("x.css", "x")}, nil, a); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("PutFiles archived = %v", err)
	}
	if err := e.pages.Delete(ctx, vb.ID, a); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("Delete published = %v", err)
	}
	vc, err := e.pages.CopyToDraft(ctx, va.ID, a)
	if err != nil || vc.Status != domain.PageDraft || len(vc.Files) != 2 || vc.ID == va.ID {
		t.Fatalf("CopyToDraft = %+v, %v", vc, err)
	}
	if err := e.pages.Unpublish(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := e.pages.Published(ctx); ok {
		t.Fatal("still published")
	}
	if gb, _ := e.pages.Get(ctx, vb.ID); gb.Status != domain.PageArchived {
		t.Fatalf("unpublished B = %s", gb.Status)
	}
	if _, err := e.pages.PublishedFile(ctx, "index.html"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("PublishedFile none = %v", err)
	}
	if err := e.pages.Unpublish(ctx, a); err != nil {
		t.Fatalf("Unpublish none = %v", err)
	}
	want := []string{domain.ActPageDraftCreated, domain.ActPageDraftCreated, domain.ActPagePublished, domain.ActPagePublished,
		domain.ActPageDraftCreated, domain.ActPageUnpublished}
	if got := auditActions(t, e); !reflect.DeepEqual(got, want) {
		t.Fatalf("audit = %v; want %v", got, want)
	}
	if err := e.pages.Delete(ctx, va.ID, a); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pages.Get(ctx, va.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get deleted = %v", err)
	}
	if e.auditCount(domain.ActPageDeleted) != 1 {
		t.Fatal("no delete audit")
	}
	if err := e.pages.Publish(ctx, 999, a); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Publish missing = %v", err)
	}
}

func TestPreviewToken(t *testing.T) {
	e := newEnv(t)
	a := actor(e.mkUser("+380501110001", admin))
	va, _ := e.pages.CreateDraft(ctx, []domain.PageFileInput{pin("index.html", "A")}, "", a)
	vb, _ := e.pages.CreateDraft(ctx, []domain.PageFileInput{pin("index.html", "B")}, "", a)
	tok := e.pages.PreviewToken(va.ID)
	f, err := e.pages.PreviewFile(ctx, tok, "index.html")
	if err != nil || !bytes.Equal(f.Data, []byte("A")) {
		t.Fatalf("PreviewFile = %+v, %v", f, err)
	}
	if _, err := e.pages.PreviewFile(ctx, tok, "none.css"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing file = %v", err)
	}
	parts := strings.SplitN(tok, ".", 2)
	foreign := fmt.Sprint(vb.ID) + "." + parts[1]
	if _, err := e.pages.PreviewFile(ctx, foreign, "index.html"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign token = %v", err)
	}
	tampered := tok[:len(tok)-2] + "AA"
	if tampered == tok {
		tampered = tok[:len(tok)-2] + "BB"
	}
	for _, bad := range []string{tampered, "", "x", "1.2", "1.2.3.4", tok + "x"} {
		if _, err := e.pages.PreviewFile(ctx, bad, "index.html"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("bad token %q = %v", bad, err)
		}
	}
	tb := e.pages.PreviewToken(vb.ID)
	if err := e.pages.Delete(ctx, vb.ID, a); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pages.PreviewFile(ctx, tb, "index.html"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted version token = %v", err)
	}
	e.clock.Advance(31 * time.Minute)
	if _, err := e.pages.PreviewFile(ctx, tok, "index.html"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expired token = %v", err)
	}
}
