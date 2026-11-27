package httpapi

import (
	"context"
	"database/sql"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/igorrius/homelander-bot/internal/config"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
)

const indexHTML = `<!doctype html><div id="root"></div>`

func testStatic() fs.FS {
	return fstest.MapFS{
		"index.html":    {Data: []byte(indexHTML)},
		"assets/app.js": {Data: []byte(`console.log(1)`)},
	}
}

func newTestServer(t *testing.T, static fs.FS) (*Server, *sql.DB) {
	t.Helper()
	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := New(config.Config{}, Deps{DB: db, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Static: static})
	return s, db
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthzOK(t *testing.T) {
	s, _ := newTestServer(t, testStatic())
	rec := get(t, s.Handler(), "/healthz")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestHealthzDBDown(t *testing.T) {
	s, db := newTestServer(t, testStatic())
	_ = db.Close()
	rec := get(t, s.Handler(), "/healthz")
	if rec.Code != 503 || strings.TrimSpace(rec.Body.String()) != `{"status":"unavailable"}` {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	s, _ := newTestServer(t, testStatic())
	want := map[string]string{
		"Content-Security-Policy": "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
	}
	for _, p := range []string{"/healthz", "/", "/api/v1/nope", "/assets/missing.js"} {
		rec := get(t, s.Handler(), p)
		for k, v := range want {
			if got := rec.Header().Get(k); got != v {
				t.Errorf("%s: %s = %q, want %q", p, k, got, v)
			}
		}
	}
}

func TestUnknownAPIReturnsJSON404(t *testing.T) {
	s, _ := newTestServer(t, testStatic())
	for _, p := range []string{"/api/v1/nope", "/api", "/api/"} {
		rec := get(t, s.Handler(), p)
		if rec.Code != 404 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") ||
			strings.TrimSpace(rec.Body.String()) != `{"error":{"code":"not_found"}}` {
			t.Errorf("%s: got %d %q %q", p, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
		}
	}
}

func TestSPAFallback(t *testing.T) {
	s, _ := newTestServer(t, testStatic())
	for _, p := range []string{"/", "/admin/users/5", "/register"} {
		rec := get(t, s.Handler(), p)
		if rec.Code != 200 || rec.Body.String() != indexHTML || rec.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("%s: got %d %q cache=%q", p, rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"))
		}
	}
}

func TestMissingAssetIs404(t *testing.T) {
	s, _ := newTestServer(t, testStatic())
	for _, p := range []string{"/assets/missing.js", "/favicon.ico"} {
		rec := get(t, s.Handler(), p)
		if rec.Code != 404 || strings.Contains(rec.Body.String(), indexHTML) {
			t.Errorf("%s: got %d %q", p, rec.Code, rec.Body.String())
		}
	}
}

func TestAssetCached(t *testing.T) {
	s, _ := newTestServer(t, testStatic())
	rec := get(t, s.Handler(), "/assets/app.js")
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Errorf("got %d cache=%q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

func TestIndexHTMLPathNotCachedForever(t *testing.T) {
	s, _ := newTestServer(t, testStatic())
	rec := get(t, s.Handler(), "/index.html")
	if rec.Code != 200 && rec.Code != 301 {
		t.Errorf("got %d", rec.Code)
	}
	if rec.Code == 200 && rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("cache=%q", rec.Header().Get("Cache-Control"))
	}
}

func TestNoWebBuild(t *testing.T) {
	s, _ := newTestServer(t, fstest.MapFS{".gitkeep": {Data: nil}})
	rec := get(t, s.Handler(), "/")
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "web app is not built") {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(t, s.Handler(), "/healthz"); rec.Code != 200 {
		t.Errorf("healthz %d", rec.Code)
	}
}

func TestPanicRecovered(t *testing.T) {
	s, _ := newTestServer(t, testStatic())
	s.API().HandleFunc("GET /api/v1/boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
	rec := get(t, s.Handler(), "/api/v1/boom")
	if rec.Code != 500 || strings.TrimSpace(rec.Body.String()) != `{"error":{"code":"internal"}}` {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestAPIRouteServed(t *testing.T) {
	s, _ := newTestServer(t, testStatic())
	s.API().HandleFunc("GET /api/v1/ping", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, 200, map[string]string{"pong": "yes"})
	})
	rec := get(t, s.Handler(), "/api/v1/ping")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"pong":"yes"}` {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}
