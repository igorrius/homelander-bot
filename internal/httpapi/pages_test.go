package httpapi_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/httpapi"
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
	if r.Code != 200 || !strings.HasPrefix(r.Body.String(), "A") || r.Header().Get("Cache-Control") != "private, no-store" ||
		r.Header().Get("Content-Security-Policy") != pageCSP || r.Header().Get("ETag") != "" {
		t.Fatalf("preview = %d %q %v", r.Code, r.Body, r.Header())
	}
	if r := e.get("/site/preview/" + tok + "/"); r.Code != 200 || !strings.HasPrefix(r.Body.String(), "A") {
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

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (e *apiEnv) uploadPage(tok string, z []byte, note string) resp {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if note != "" {
		_ = mw.WriteField("note", note)
	}
	fw, _ := mw.CreateFormFile("zip", "page.zip")
	_, _ = fw.Write(z)
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/admin/pages", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Requested-With", "homelander")
	req.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: tok})
	req.RemoteAddr = "10.8.0.3:1"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return resp{rec, e.t}
}

func idOf(r resp) string { return fmt.Sprint(int64(r.json()["id"].(float64))) }

func statusOf(t *testing.T, e *apiEnv, tok, id string) string {
	t.Helper()
	r := e.do("GET", "/api/v1/admin/pages/"+id, nil, tok, "")
	if r.Code != 200 {
		t.Fatalf("get %s = %d", id, r.Code)
	}
	return r.json()["status"].(string)
}

func TestAdminPagesFlow(t *testing.T) {
	e := newAPIEnv(t)
	u := e.mkUser("+380501110001", admin)
	tok := e.login(u)
	r := e.uploadPage(tok, zipOf(t, map[string]string{"site/index.html": "<p>one</p>", "site/app.js": "1"}), "first")
	if r.Code != 201 {
		t.Fatalf("upload = %d %s", r.Code, r.Body)
	}
	v := r.json()
	if v["status"] != "draft" || v["note"] != "first" || v["totalSize"] != float64(11) {
		t.Fatalf("version = %v", v)
	}
	files := v["files"].([]any)
	if len(files) != 2 || files[0].(map[string]any)["path"] != "app.js" ||
		files[1].(map[string]any)["contentType"] != "text/html; charset=utf-8" {
		t.Fatalf("files = %v", files)
	}
	if cb := v["createdBy"].(map[string]any); cb["id"] != float64(u.ID) || cb["name"] == "" {
		t.Fatalf("createdBy = %v", cb)
	}
	if v["publishedBy"] != nil || v["publishedAt"] != nil {
		t.Fatalf("published fields = %v", v)
	}
	first := idOf(r)
	if r := e.uploadPage(tok, zipOf(t, map[string]string{"a.css": "x"}), ""); r.Code != 422 || fieldOf(r, "files") != "index_required" {
		t.Fatalf("no index = %d %s", r.Code, r.Body)
	}
	bad := e.uploadPage(tok, zipOf(t, map[string]string{"index.html": "x", "favicon.ico": "x"}), "")
	if details, _ := bad.json()["error"].(map[string]any)["details"].(map[string]any); bad.Code != 422 ||
		fieldOf(bad, "files") != "type" || details["path"] != "favicon.ico" {
		t.Fatalf("bad type = %d %s", bad.Code, bad.Body)
	}
	if r := e.uploadPage(tok, []byte("nope"), ""); r.Code != 422 || fieldOf(r, "zip") != "invalid" {
		t.Fatalf("not zip = %d %s", r.Code, r.Body)
	}

	list := e.do("GET", "/api/v1/admin/pages", nil, tok, "")
	var vs []map[string]any
	if err := jsonUnmarshal(list.Body.Bytes(), &vs); err != nil || len(vs) != 1 {
		t.Fatalf("list = %s", list.Body)
	}

	pr := e.do("POST", "/api/v1/admin/pages/"+first+"/preview", nil, tok, "")
	url, _ := pr.json()["url"].(string)
	if pr.Code != 200 || !strings.HasPrefix(url, "/site/preview/") || !strings.HasSuffix(url, "/index.html") {
		t.Fatalf("preview = %d %s", pr.Code, pr.Body)
	}
	if r := e.get(url); r.Code != 200 || !strings.HasPrefix(r.Body.String(), "<p>one</p><script>") {
		t.Fatalf("preview page = %d %q", r.Code, r.Body)
	}
	if r := e.do("POST", "/api/v1/admin/pages/999/preview", nil, tok, ""); r.Code != 404 {
		t.Fatalf("preview missing = %d", r.Code)
	}
	if r := e.get("/site/index.html"); r.Code != 404 {
		t.Fatalf("before publish = %d", r.Code)
	}
	if r := e.do("POST", "/api/v1/admin/pages/"+first+"/publish", nil, tok, ""); r.Code != 204 {
		t.Fatalf("publish = %d %s", r.Code, r.Body)
	}
	if r := e.get("/site/index.html"); r.Code != 200 || r.Body.String() != "<p>one</p>" {
		t.Fatalf("published = %d", r.Code)
	}
	second := idOf(e.uploadPage(tok, zipOf(t, map[string]string{"index.html": "two"}), ""))
	if r := e.do("POST", "/api/v1/admin/pages/"+second+"/publish", nil, tok, ""); r.Code != 204 {
		t.Fatalf("publish second = %d", r.Code)
	}
	if s := statusOf(t, e, tok, first); s != "archived" {
		t.Fatalf("first = %s", s)
	}
	pv := e.do("GET", "/api/v1/admin/pages/"+second, nil, tok, "").json()
	if pv["publishedBy"].(map[string]any)["id"] != float64(u.ID) || pv["publishedAt"] == nil {
		t.Fatalf("published by = %v", pv)
	}
	cp := e.do("POST", "/api/v1/admin/pages/"+first+"/copy", nil, tok, "")
	if cp.Code != 201 || cp.json()["status"] != "draft" || len(cp.json()["files"].([]any)) != 2 {
		t.Fatalf("copy = %d %s", cp.Code, cp.Body)
	}
	if r := e.do("POST", "/api/v1/admin/pages/unpublish", nil, tok, ""); r.Code != 204 {
		t.Fatalf("unpublish = %d", r.Code)
	}
	if r := e.get("/site/index.html"); r.Code != 404 {
		t.Fatalf("after unpublish = %d", r.Code)
	}
	if got := e.do("GET", "/api/v1/site", nil, "", "").json()["pageUrl"]; got != nil {
		t.Fatalf("pageUrl after unpublish = %v", got)
	}
	if r := e.do("DELETE", "/api/v1/admin/pages/"+first, nil, tok, ""); r.Code != 204 {
		t.Fatalf("delete archived = %d", r.Code)
	}
	if r := e.do("POST", "/api/v1/admin/pages/"+second+"/publish", nil, tok, ""); r.Code != 204 {
		t.Fatalf("rollback = %d", r.Code)
	}
	if r := e.do("DELETE", "/api/v1/admin/pages/"+second, nil, tok, ""); r.Code != 409 || r.errCode() != "conflict" {
		t.Fatalf("delete published = %d %s", r.Code, r.Body)
	}
	if r := e.do("GET", "/api/v1/admin/pages/x", nil, tok, ""); r.Code != 404 {
		t.Fatalf("bad id = %d", r.Code)
	}
}

func TestAdminPagesRequireAdmin(t *testing.T) {
	e := newAPIEnv(t)
	resident := e.login(e.mkUser("+380501110002"))
	for _, rt := range [][2]string{{"GET", "/api/v1/admin/pages"}, {"POST", "/api/v1/admin/pages/1/publish"},
		{"POST", "/api/v1/admin/pages/1/preview"}, {"DELETE", "/api/v1/admin/pages/1"}, {"POST", "/api/v1/admin/pages/unpublish"}} {
		if r := e.do(rt[0], rt[1], nil, resident, ""); r.Code != 403 {
			t.Fatalf("resident %v = %d", rt, r.Code)
		}
		if r := e.do(rt[0], rt[1], nil, "", ""); r.Code != 401 {
			t.Fatalf("guest %v = %d", rt, r.Code)
		}
	}
}

func TestSitePagesList(t *testing.T) {
	e := newAPIEnv(t)
	if got := e.do("GET", "/api/v1/site", nil, "", "").json()["pages"]; fmt.Sprint(got) != "[]" {
		t.Fatalf("pages before publish = %#v", got)
	}
	v := e.draft(map[string]string{"index.html": "x", "contacts.html": "c", "css/a.html": "n", "app.css": "p{}"})
	if err := e.api.Pages.Publish(ctx, v.ID, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	if got := e.do("GET", "/api/v1/site", nil, "", "").json()["pages"]; fmt.Sprint(got) != "[/ /contacts]" {
		t.Fatalf("pages = %#v", got)
	}
	tok := e.login(e.mkUser("+380501110001", admin))
	if got := e.do("GET", "/api/v1/admin/site", nil, tok, "").json()["pages"]; fmt.Sprint(got) != "[/ /contacts]" {
		t.Fatalf("admin pages = %#v", got)
	}
}

func TestVersionDTOPages(t *testing.T) {
	e := newAPIEnv(t)
	tok := e.login(e.mkUser("+380501110001", admin))
	v := e.draft(map[string]string{"index.html": "x", "contacts.html": "c"})
	r := e.do("GET", "/api/v1/admin/pages/"+itoa(v.ID), nil, tok, "")
	want := "[map[name:index path:/] map[name:contacts path:/contacts]]"
	if got := fmt.Sprint(r.json()["pages"]); got != want {
		t.Fatalf("pages = %s", got)
	}
	list := e.do("GET", "/api/v1/admin/pages", nil, tok, "")
	if !strings.Contains(list.Body.String(), `"pages":[{"name":"index","path":"/"},{"name":"contacts","path":"/contacts"}]`) {
		t.Fatalf("list = %s", list.Body)
	}
}

func TestPreviewInjectsNav(t *testing.T) {
	e := newAPIEnv(t)
	v := e.draft(map[string]string{"index.html": "<html><body><p>hi</p></body></html>", "contacts.html": "<p>c</p>", "app.css": "p{}"})
	tok := e.api.Pages.PreviewToken(v.ID)
	r := e.get("/site/preview/" + tok + "/index.html")
	body := r.Body.String()
	if !strings.Contains(body, "homelander:navigate") || !strings.Contains(body, `["/","/contacts"]`) ||
		!strings.HasSuffix(body, "</script></body></html>") || !strings.HasPrefix(body, "<html><body><p>hi</p><script>") {
		t.Fatalf("preview index = %q", body)
	}
	if r.Header().Get("Content-Length") != fmt.Sprint(len(body)) {
		t.Fatalf("content-length %q for %d bytes", r.Header().Get("Content-Length"), len(body))
	}
	// No </body>: the script goes at the end.
	if c := e.get("/site/preview/" + tok + "/contacts.html").Body.String(); !strings.HasPrefix(c, "<p>c</p><script>") || !strings.HasSuffix(c, "</script>") {
		t.Fatalf("preview contacts = %q", c)
	}
	if c := e.get("/site/preview/" + tok + "/app.css").Body.String(); c != "p{}" {
		t.Fatalf("preview css = %q", c)
	}
	stored := "<html><body><p>hi</p></body></html>"
	if err := e.api.Pages.Publish(ctx, v.ID, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	p := e.get("/site/index.html")
	if p.Body.String() != stored || strings.Contains(p.Body.String(), "homelander:navigate") {
		t.Fatalf("published = %q", p.Body)
	}
	var sum string
	for _, f := range v.Files {
		if f.Path == "index.html" {
			sum = f.SHA256
		}
	}
	if tag := p.Header().Get("ETag"); sum == "" || tag != `"`+sum+`"` {
		t.Fatalf("ETag = %q", tag)
	}
}
