package httpapi_test

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/httpapi"
)

func (e *apiEnv) loginLink(u domain.User, next string) string {
	e.t.Helper()
	link, err := e.api.Auth.CreateLoginLink(ctx, u.ID, next)
	if err != nil {
		e.t.Fatal(err)
	}
	p, _ := url.Parse(link)
	return p.Query().Get("t")
}

func sessionCookie(r resp) string {
	for _, c := range r.Result().Cookies() {
		if c.Name == httpapi.SessionCookie {
			return c.Value
		}
	}
	return ""
}

func TestLoginLinkRedeem(t *testing.T) {
	e := newAPIEnv(t)
	u := e.mkUser("+380500000002")
	tok := e.loginLink(u, "/my/feed/3")
	// The page Telegram may fetch for a preview does not spend the link.
	e.do("GET", "/login/link?t="+tok, nil, "", "")
	r := e.do("POST", "/api/v1/auth/link", map[string]string{"token": tok}, "", "")
	if r.Code != 200 {
		t.Fatalf("redeem %d %s", r.Code, r.Body)
	}
	body := r.json()
	if body["next"] != "/my/feed/3" || body["user"].(map[string]any)["phone"] != "+380500000002" {
		t.Errorf("body = %v", body)
	}
	cookie := sessionCookie(r)
	if cookie == "" {
		t.Fatal("no session cookie")
	}
	if me := e.do("GET", "/api/v1/me", nil, cookie, ""); me.Code != 200 {
		t.Errorf("/me %d", me.Code)
	}
	again := e.do("POST", "/api/v1/auth/link", map[string]string{"token": tok}, "", "")
	if again.Code != 422 || again.errCode() != "token_invalid" {
		t.Errorf("second redeem %d %s", again.Code, again.Body)
	}
	if bad := e.do("POST", "/api/v1/auth/link", map[string]string{"token": "nope"}, "", ""); bad.Code != 422 {
		t.Errorf("unknown token %d", bad.Code)
	}
}

func TestLoginLinkSwitchesUser(t *testing.T) {
	e := newAPIEnv(t)
	a := e.mkUser("+380500000001", admin)
	b := e.mkUser("+380500000002")
	old := e.login(a)
	r := e.do("POST", "/api/v1/auth/link", map[string]string{"token": e.loginLink(b, "/my")}, old, "")
	if r.Code != 200 {
		t.Fatalf("redeem %d %s", r.Code, r.Body)
	}
	me := e.do("GET", "/api/v1/me", nil, sessionCookie(r), "")
	if me.Code != 200 || me.json()["phone"] != "+380500000002" {
		t.Errorf("/me = %d %s", me.Code, me.Body)
	}
	// The session the browser had before is closed, not left behind.
	if e.do("GET", "/api/v1/me", nil, old, "").Code != 401 {
		t.Error("previous session still works")
	}
}

func TestMyHome(t *testing.T) {
	e := newAPIEnv(t)
	floors := 9
	e.house.Address, e.house.Floors = "1 Test Street", &floors
	if err := e.r.Houses.Update(ctx, &e.house); err != nil {
		t.Fatal(err)
	}
	u := e.mkUser("+380500000002")
	if err := e.r.Residencies.Create(ctx, &domain.Residency{UserID: u.ID, HouseID: e.house.ID, Apartment: 7, StatusID: e.owner.ID}); err != nil {
		t.Fatal(err)
	}
	tok := e.login(u) // links a Telegram channel
	r := e.do("GET", "/api/v1/me/home", nil, tok, "")
	if r.Code != 200 {
		t.Fatalf("home %d %s", r.Code, r.Body)
	}
	var got struct {
		TelegramLinked bool `json:"telegramLinked"`
		Homes          []struct {
			HouseName   string            `json:"houseName"`
			Address     string            `json:"address"`
			Floors      *int              `json:"floors"`
			Apartment   int               `json:"apartment"`
			StatusNames map[string]string `json:"statusNames"`
		} `json:"homes"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.TelegramLinked || len(got.Homes) != 1 {
		t.Fatalf("home = %+v", got)
	}
	h := got.Homes[0]
	if h.HouseName != "1" || h.Address != "1 Test Street" || h.Floors == nil || *h.Floors != 9 || h.Apartment != 7 || h.StatusNames["uk"] == "" {
		t.Errorf("home = %+v", h)
	}
	if guest := e.do("GET", "/api/v1/me/home", nil, "", ""); guest.Code != 401 {
		t.Errorf("guest %d", guest.Code)
	}
}

func TestMyHomeEmpty(t *testing.T) {
	e := newAPIEnv(t)
	u := e.mkUser("+380500000001", admin, func(u *domain.User) { u.IsResident = false })
	r := e.do("GET", "/api/v1/me/home", nil, e.login(u), "")
	if r.Code != 200 || r.Body.String() != `{"homes":[],"telegramLinked":true}`+"\n" {
		t.Errorf("home = %d %s", r.Code, r.Body)
	}
}

func TestManifest(t *testing.T) {
	e := newAPIEnv(t)
	if err := e.r.Site.Save(ctx, domain.SiteSettings{CommunityName: domain.LocalizedText{"uk": "Тестова громада", "en": "Test Community"},
		DefaultLocale: "uk"}); err != nil {
		t.Fatal(err)
	}
	r := e.do("GET", "/site.webmanifest", nil, "", "")
	if r.Code != 200 || r.Header().Get("Content-Type") != "application/manifest+json" || r.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("manifest %d %q %q", r.Code, r.Header().Get("Content-Type"), r.Header().Get("Cache-Control"))
	}
	m := r.json()
	want := map[string]any{"name": "Тестова громада", "short_name": "Тестова", "start_url": "/my", "scope": "/", "display": "standalone",
		"theme_color": "#0ca678", "background_color": "#ffffff", "lang": "uk"}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, want %v", k, m[k], v)
		}
	}
	icons, _ := m["icons"].([]any)
	if len(icons) != 2 || icons[0].(map[string]any)["src"] != "/icon-192.png" || icons[1].(map[string]any)["sizes"] != "512x512" {
		t.Errorf("icons = %v", icons)
	}

	if err := e.r.Site.Save(ctx, domain.SiteSettings{CommunityName: domain.LocalizedText{"en": "Green Garden Society"}, DefaultLocale: "uk"}); err != nil {
		t.Fatal(err)
	}
	m = e.do("GET", "/site.webmanifest", nil, "", "").json()
	if m["name"] != "Green Garden Society" || m["short_name"] != "Green" {
		t.Errorf("fallback name = %v / %v", m["name"], m["short_name"])
	}
	if err := e.r.Site.Save(ctx, domain.SiteSettings{}); err != nil {
		t.Fatal(err)
	}
	m = e.do("GET", "/site.webmanifest", nil, "", "").json()
	if m["name"] != "Homelander" || m["short_name"] != "Homelander" {
		t.Errorf("empty name = %v / %v", m["name"], m["short_name"])
	}
}
