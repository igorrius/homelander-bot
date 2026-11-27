package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/config"
	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/httpapi"
	"github.com/igorrius/homelander-bot/internal/i18n"
	"github.com/igorrius/homelander-bot/internal/ratelimit"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite/sqlitetest"
	"github.com/igorrius/homelander-bot/locales"
)

var ctx = context.Background()

type fakeMessenger struct {
	mu   sync.Mutex
	sent []string
}

func (f *fakeMessenger) Type() string   { return "telegram" }
func (f *fakeMessenger) BotURL() string { return "https://t.me/test_bot" }
func (f *fakeMessenger) SendText(_ context.Context, _, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
	return nil
}

func (f *fakeMessenger) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		return ""
	}
	return f.sent[len(f.sent)-1]
}

type apiEnv struct {
	t     *testing.T
	db    sqlite.DB
	r     domain.Repos
	api   httpapi.API
	h     http.Handler
	msgr  *fakeMessenger
	now   time.Time
	house domain.House
	owner domain.ResidencyStatus
	ipSeq int
}

func newAPIEnv(t *testing.T) *apiEnv {
	t.Helper()
	db := sqlitetest.New(t)
	r := domain.Repos{
		Tx: db, Users: sqlite.Users{DB: db}, Channels: sqlite.Channels{DB: db}, Houses: sqlite.Houses{DB: db},
		Statuses: sqlite.Statuses{DB: db}, Residencies: sqlite.Residencies{DB: db}, Codes: sqlite.Codes{DB: db},
		Sessions: sqlite.Sessions{DB: db}, Site: sqlite.Site{DB: db}, Audit: sqlite.Audit{DB: db},
	}
	e := &apiEnv{t: t, db: db, r: r, msgr: &fakeMessenger{}, now: time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)}
	clock := func() time.Time { return e.now }
	tr, err := i18n.New(locales.FS, "uk", []string{"uk", "en"})
	if err != nil {
		t.Fatal(err)
	}
	limits := ratelimit.New(clock)
	pub, _ := url.Parse("https://example.com")
	cfg := config.Config{PublicURL: pub, SessionMaxTTL: 168 * time.Hour, DefaultLocale: "uk"}
	locs := []string{"uk", "en"}
	e.api = httpapi.API{
		Users:   &domain.UserService{R: r, Now: clock, Limits: limits, Region: "UA", PendingCap: 200, Locales: locs},
		Catalog: &domain.CatalogService{R: r, Now: clock, Locales: locs},
		Auth: &domain.AuthService{R: r, Now: clock, Limits: limits, Secret: []byte("0123456789abcdef0123456789abcdef"),
			Region: "UA", Messenger: e.msgr, I18n: tr, CodeTTL: 5 * time.Minute, MaxAttempts: 5,
			IdleTTL: 12 * time.Hour, MaxTTL: 168 * time.Hour},
		Messenger: e.msgr,
		Cfg:       cfg,
	}
	s := httpapi.New(cfg, httpapi.Deps{DB: db.DB, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	s.WithSession(e.api.Auth)
	httpapi.RegisterPublic(s, e.api)
	httpapi.RegisterAdmin(s, e.api)
	e.h = s.Handler()
	sts, _ := r.Statuses.List(ctx, false)
	e.owner = sts[0]
	e.house = domain.House{Name: "1", ApartmentFrom: 1, ApartmentTo: 10}
	if err := r.Houses.Create(ctx, &e.house); err != nil {
		t.Fatal(err)
	}
	return e
}

type resp struct {
	*httptest.ResponseRecorder
	t *testing.T
}

func (r resp) json() map[string]any {
	r.t.Helper()
	var v map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &v); err != nil {
		r.t.Fatalf("body %q: %v", r.Body.String(), err)
	}
	return v
}

func (r resp) errCode() string {
	e, _ := r.json()["error"].(map[string]any)
	s, _ := e["code"].(string)
	return s
}

// do sends a request; token is a session token or "", ip the client address.
func (e *apiEnv) do(method, path string, body any, token, ip string) resp {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	case []byte:
		rd = bytes.NewReader(b)
	default:
		raw, _ := json.Marshal(b)
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("X-Requested-With", "homelander")
	if rd != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: token})
	}
	if ip == "" {
		e.ipSeq++
		ip = "10.9." + string(rune('0'+e.ipSeq/10)) + "." + string(rune('0'+e.ipSeq%10))
	}
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return resp{rec, e.t}
}

func (e *apiEnv) mkUser(phone string, mod ...func(*domain.User)) domain.User {
	e.t.Helper()
	u := domain.User{Phone: phone, FirstName: "F", LastName: "L", Status: domain.StatusActive, Source: domain.SourceAdmin, IsResident: true}
	for _, m := range mod {
		m(&u)
	}
	if err := e.r.Users.Create(ctx, &u); err != nil {
		e.t.Fatal(err)
	}
	return u
}

var codeRe = regexp.MustCompile(`\b\d{6}\b`)

// login links a Telegram channel to u and returns a session token from the real login flow.
func (e *apiEnv) login(u domain.User) string {
	e.t.Helper()
	ch := domain.Channel{UserID: u.ID, Type: "telegram", ExternalID: u.Phone, ChatID: "1"}
	_ = e.r.Channels.Create(ctx, &ch)
	if r := e.do("POST", "/api/v1/auth/code", map[string]string{"phone": u.Phone}, "", ""); r.Code != 202 {
		e.t.Fatalf("auth/code %d %s", r.Code, r.Body)
	}
	code := codeRe.FindString(e.msgr.last())
	r := e.do("POST", "/api/v1/auth/verify", map[string]string{"phone": u.Phone, "code": code}, "", "")
	if r.Code != 200 {
		e.t.Fatalf("verify %d %s", r.Code, r.Body)
	}
	for _, c := range r.Result().Cookies() {
		if c.Name == httpapi.SessionCookie {
			return c.Value
		}
	}
	e.t.Fatal("no session cookie")
	return ""
}

func admin(u *domain.User) { u.IsAdmin = true }

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
