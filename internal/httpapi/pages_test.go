package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

const pageCSP = "sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation; " +
	"default-src 'self' data: blob:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'self'"

// draft stores a draft page version through the service.
func (e *apiEnv) draft(files map[string]string) domain.PageVersion {
	e.t.Helper()
	var in []domain.PageFileInput
	for p, b := range files {
		in = append(in, domain.PageFileInput{Path: p, Data: []byte(b)})
	}
	v, err := e.api.Pages.CreateDraft(ctx, in, "", domain.Actor{})
	if err != nil {
		e.t.Fatal(err)
	}
	return v
}

func (e *apiEnv) get(path string, hdr ...string) resp {
	e.t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return resp{rec, e.t}
}

func TestSiteServesPublished(t *testing.T) {
	e := newAPIEnv(t)
	v := e.draft(map[string]string{"index.html": "<p>hi</p>", "css/app.css": "p{}", "img/a.png": "PNG"})
	if err := e.api.Pages.Publish(ctx, v.ID, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	for path, ct := range map[string]string{
		"/site/index.html": "text/html; charset=utf-8", "/site/": "text/html; charset=utf-8",
		"/site/css/app.css": "text/css; charset=utf-8", "/site/img/a.png": "image/png",
	} {
		r := e.get(path)
		if r.Code != 200 || r.Header().Get("Content-Type") != ct {
			t.Fatalf("%s = %d %q", path, r.Code, r.Header().Get("Content-Type"))
		}
		if got := r.Header().Get("Content-Security-Policy"); got != pageCSP {
			t.Fatalf("%s CSP = %q", path, got)
		}
		if strings.Contains(r.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Fatal("global CSP kept")
		}
		if r.Header().Get("X-Content-Type-Options") != "nosniff" || r.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("%s headers = %v", path, r.Header())
		}
	}
	r := e.get("/site/index.html")
	if r.Body.String() != "<p>hi</p>" {
		t.Fatalf("body = %q", r.Body)
	}
	tag := r.Header().Get("ETag")
	if tag == "" {
		t.Fatal("no ETag")
	}
	if r := e.get("/site/index.html", "If-None-Match", tag); r.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match = %d", r.Code)
	}
	req := httptest.NewRequest("HEAD", "/site/index.html", nil)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.Len() != 0 {
		t.Fatalf("HEAD = %d %q", rec.Code, rec.Body)
	}
	for _, p := range []string{"/site/none.css", "/site/../index.html", "/site/css"} {
		if r := e.get(p); r.Code != 404 {
			t.Fatalf("%s = %d", p, r.Code)
		}
	}
	if r := e.do("POST", "/site/index.html", nil, "", ""); r.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d", r.Code)
	}
}

func TestSiteNothingPublished(t *testing.T) {
	e := newAPIEnv(t)
	e.draft(map[string]string{"index.html": "draft"})
	if r := e.get("/site/index.html"); r.Code != 404 || strings.Contains(r.Body.String(), "draft") {
		t.Fatalf("unpublished = %d %q", r.Code, r.Body)
	}
	r := e.do("GET", "/api/v1/site", nil, "", "")
	if v, ok := r.json()["pageUrl"]; !ok || v != nil {
		t.Fatalf("pageUrl = %v, %v", v, ok)
	}
}

func TestSitePreview(t *testing.T) {
	e := newAPIEnv(t)
	a := e.draft(map[string]string{"index.html": "A"})
	b := e.draft(map[string]string{"index.html": "B"})
	tok := e.api.Pages.PreviewToken(a.ID)
	r := e.get("/site/preview/" + tok + "/index.html")
	if r.Code != 200 || r.Body.String() != "A" || r.Header().Get("Cache-Control") != "private, no-store" ||
		r.Header().Get("Content-Security-Policy") != pageCSP || r.Header().Get("ETag") != "" {
		t.Fatalf("preview = %d %q %v", r.Code, r.Body, r.Header())
	}
	if r := e.get("/site/preview/" + tok + "/"); r.Code != 200 || r.Body.String() != "A" {
		t.Fatalf("preview root = %d", r.Code)
	}
	parts := strings.SplitN(tok, ".", 2)
	foreign := strings.Replace(tok, parts[0], itoa(b.ID), 1)
	for _, p := range []string{"/site/preview/", "/site/preview/index.html", "/site/preview/x/index.html",
		"/site/preview/" + foreign + "/index.html"} {
		if r := e.get(p); r.Code != 404 {
			t.Fatalf("%s = %d", p, r.Code)
		}
	}
	e.now = e.now.Add(31 * time.Minute)
	if r := e.get("/site/preview/" + tok + "/index.html"); r.Code != 404 {
		t.Fatalf("expired = %d", r.Code)
	}
}

func TestSitePageURL(t *testing.T) {
	e := newAPIEnv(t)
	v := e.draft(map[string]string{"index.html": "x"})
	if err := e.api.Pages.Publish(ctx, v.ID, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	if got := e.do("GET", "/api/v1/site", nil, "", "").json()["pageUrl"]; got != "/site/index.html" {
		t.Fatalf("pageUrl = %v", got)
	}
	tok := e.login(e.mkUser("+380501110001", admin))
	if got := e.do("GET", "/api/v1/admin/site", nil, tok, "").json()["pageUrl"]; got != "/site/index.html" {
		t.Fatalf("admin pageUrl = %v", got)
	}
}
