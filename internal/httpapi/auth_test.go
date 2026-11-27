package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

type fakeAuth map[string]domain.User

func (f fakeAuth) Authenticate(_ context.Context, raw string) (domain.User, domain.Session, error) {
	if u, ok := f[raw]; ok {
		return u, domain.Session{UserID: u.ID}, nil
	}
	return domain.User{}, domain.Session{}, domain.ErrUnauthorized
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.9:5555"
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
	if got := ClientIP(r, false); got != "10.0.0.9" {
		t.Errorf("untrusted = %q", got)
	}
	if got := ClientIP(r, true); got != "2.2.2.2" {
		t.Errorf("trusted = %q", got)
	}
	r.Header.Del("X-Forwarded-For")
	if got := ClientIP(r, true); got != "10.0.0.9" {
		t.Errorf("trusted without header = %q", got)
	}
}

func authServer(t *testing.T) *Server {
	s, _ := newTestServer(t, testStatic())
	s.WithSession(fakeAuth{
		"resident": {ID: 1, IsResident: true, Status: domain.StatusActive},
		"admin":    {ID: 2, IsAdmin: true, Status: domain.StatusActive},
	})
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := CurrentUser(r)
		WriteJSON(w, 200, map[string]int64{"id": u.ID})
	})
	s.API().Handle("POST /api/v1/thing", ok)
	s.API().Handle("GET /api/v1/me-test", RequireUser(ok))
	s.API().Handle("GET /api/v1/admin-test", RequireAdmin(ok))
	return s
}

func call(t *testing.T, s *Server, method, path, token string, csrf bool) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader("{}"))
	if token != "" {
		r.AddCookie(&http.Cookie{Name: SessionCookie, Value: token})
	}
	if csrf {
		r.Header.Set("X-Requested-With", "homelander")
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

func TestCSRFHeaderRequired(t *testing.T) {
	s := authServer(t)
	if rec := call(t, s, "POST", "/api/v1/thing", "", false); rec.Code != 403 || !strings.Contains(rec.Body.String(), `"csrf"`) {
		t.Errorf("POST without header: %d %s", rec.Code, rec.Body)
	}
	if rec := call(t, s, "POST", "/api/v1/thing", "", true); rec.Code != 200 {
		t.Errorf("POST with header: %d", rec.Code)
	}
	if rec := call(t, s, "GET", "/api/v1/me-test", "resident", false); rec.Code != 200 {
		t.Errorf("GET without header: %d", rec.Code)
	}
}

func TestRequireUser(t *testing.T) {
	s := authServer(t)
	if rec := call(t, s, "GET", "/api/v1/me-test", "", false); rec.Code != 401 || !strings.Contains(rec.Body.String(), "unauthorized") {
		t.Errorf("no session: %d %s", rec.Code, rec.Body)
	}
	if rec := call(t, s, "GET", "/api/v1/me-test", "bogus", false); rec.Code != 401 {
		t.Errorf("bad session: %d", rec.Code)
	}
	if rec := call(t, s, "GET", "/api/v1/me-test", "resident", false); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"id":1`) {
		t.Errorf("resident: %d %s", rec.Code, rec.Body)
	}
}

func TestRequireAdmin(t *testing.T) {
	s := authServer(t)
	if rec := call(t, s, "GET", "/api/v1/admin-test", "", false); rec.Code != 401 {
		t.Errorf("no session: %d", rec.Code)
	}
	if rec := call(t, s, "GET", "/api/v1/admin-test", "resident", false); rec.Code != 403 || !strings.Contains(rec.Body.String(), "forbidden") {
		t.Errorf("resident: %d %s", rec.Code, rec.Body)
	}
	if rec := call(t, s, "GET", "/api/v1/admin-test", "admin", false); rec.Code != 200 {
		t.Errorf("admin: %d", rec.Code)
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, "tok", 168*time.Hour)
	c := rec.Result().Cookies()[0]
	if c.Name != "__Host-session" || c.Value != "tok" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != 168*3600 {
		t.Errorf("cookie = %+v", c)
	}
	rec = httptest.NewRecorder()
	ClearSessionCookie(rec)
	if c := rec.Result().Cookies()[0]; c.MaxAge >= 0 || c.Value != "" {
		t.Errorf("cleared cookie = %+v", c)
	}
}
