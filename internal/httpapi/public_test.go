package httpapi_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func (e *apiEnv) registration(phone string) map[string]any {
	return map[string]any{"phone": phone, "firstName": "Тест", "lastName": "Мешканець", "houseId": e.house.ID,
		"apartment": 3, "statusId": e.owner.ID, "consent": true, "locale": "uk"}
}

func TestSiteDefaults(t *testing.T) {
	e := newAPIEnv(t)
	r := e.do("GET", "/api/v1/site", nil, "", "")
	v := r.json()
	if r.Code != 200 || v["defaultLocale"] != "uk" || v["pictureUrl"] != nil || v["botUrl"] != "https://t.me/test_bot" ||
		v["registerUrl"] != "https://example.com/register" {
		t.Errorf("site = %d %v", r.Code, v)
	}
	if _, ok := v["communityName"].(map[string]any); !ok {
		t.Errorf("communityName = %#v", v["communityName"])
	}
}

func TestRegisterOptionsHidesArchived(t *testing.T) {
	e := newAPIEnv(t)
	old := domain.House{Name: "old", ApartmentFrom: 1, ApartmentTo: 2, Archived: true}
	_ = e.r.Houses.Create(ctx, &old)
	v := e.do("GET", "/api/v1/register/options", nil, "", "").json()
	houses := v["houses"].([]any)
	statuses := v["statuses"].([]any)
	h := houses[0].(map[string]any)
	if len(houses) != 1 || h["name"] != "1" || h["apartmentFrom"] != float64(1) || h["apartmentTo"] != float64(10) || len(statuses) != 2 {
		t.Errorf("options = %v", v)
	}
	if names := statuses[0].(map[string]any)["names"].(map[string]any); names["uk"] != "Власник" {
		t.Errorf("status names = %v", names)
	}
}

func TestRegisterAccepted(t *testing.T) {
	e := newAPIEnv(t)
	r := e.do("POST", "/api/v1/register", e.registration("0500000001"), "", "")
	if r.Code != 202 || strings.TrimSpace(r.Body.String()) != `{"status":"received"}` {
		t.Fatalf("register = %d %s", r.Code, r.Body)
	}
	if u, err := e.r.Users.GetByPhone(ctx, "+380500000001"); err != nil || u.Status != domain.StatusPendingApproval {
		t.Errorf("user = %+v %v", u, err)
	}
}

func TestRegisterValidation422(t *testing.T) {
	e := newAPIEnv(t)
	body := e.registration("12")
	body["apartment"] = 99
	r := e.do("POST", "/api/v1/register", body, "", "")
	v := r.json()
	fields, _ := v["error"].(map[string]any)["fields"].(map[string]any)
	if r.Code != 422 || r.errCode() != "validation" || fields["phone"] != "invalid" || fields["apartment"] != "out_of_range" {
		t.Errorf("register = %d %v", r.Code, v)
	}
}

func TestRegisterSameAnswerForKnownPhone(t *testing.T) {
	e := newAPIEnv(t)
	e.mkUser("+380500000002")
	a := e.do("POST", "/api/v1/register", e.registration("+380500000001"), "", "")
	b := e.do("POST", "/api/v1/register", e.registration("+380500000002"), "", "")
	if a.Code != b.Code || a.Body.String() != b.Body.String() {
		t.Errorf("new %d %q vs known %d %q", a.Code, a.Body, b.Code, b.Body)
	}
	ha, hb := a.Header().Clone(), b.Header().Clone()
	ha.Del("Date")
	hb.Del("Date")
	for k := range ha {
		if ha.Get(k) != hb.Get(k) {
			t.Errorf("header %s differs: %q vs %q", k, ha.Get(k), hb.Get(k))
		}
	}
}

func TestRegisterRateLimited429(t *testing.T) {
	e := newAPIEnv(t)
	for i := range 5 {
		e.do("POST", "/api/v1/register", e.registration("+38050000001"+string(rune('0'+i))), "", "10.0.0.1")
	}
	if r := e.do("POST", "/api/v1/register", e.registration("+380500000019"), "", "10.0.0.1"); r.Code != 429 || r.errCode() != "rate_limited" {
		t.Errorf("6th = %d %s", r.Code, r.Body)
	}
}

func TestAuthCodeSameAnswer(t *testing.T) {
	e := newAPIEnv(t)
	act := e.mkUser("+380500000001")
	ch := domain.Channel{UserID: act.ID, Type: "telegram", ExternalID: "1", ChatID: "1"}
	_ = e.r.Channels.Create(ctx, &ch)
	e.mkUser("+380500000003", func(u *domain.User) { u.Status = domain.StatusPendingApproval })
	var bodies []string
	for _, p := range []string{"+380500000001", "+380500000002", "+380500000003"} {
		r := e.do("POST", "/api/v1/auth/code", map[string]string{"phone": p}, "", "")
		if r.Code != 202 {
			t.Errorf("%s: %d", p, r.Code)
		}
		bodies = append(bodies, r.Body.String())
	}
	if bodies[0] != bodies[1] || bodies[1] != bodies[2] {
		t.Errorf("bodies differ: %q", bodies)
	}
	if len(e.msgr.sent) != 1 {
		t.Errorf("messages sent = %d", len(e.msgr.sent))
	}
}

func TestVerifySetsCookieAndMe(t *testing.T) {
	e := newAPIEnv(t)
	u := e.mkUser("+380500000001", admin)
	res := domain.Residency{UserID: u.ID, HouseID: e.house.ID, Apartment: 3, StatusID: e.owner.ID}
	_ = e.r.Residencies.Create(ctx, &res)
	token := e.login(u)
	r := e.do("GET", "/api/v1/me", nil, token, "")
	v := r.json()
	if r.Code != 200 || v["phone"] != "+380500000001" || v["isAdmin"] != true || v["isResident"] != true {
		t.Fatalf("me = %d %v", r.Code, v)
	}
	rs := v["residencies"].([]any)
	if len(rs) != 1 || rs[0].(map[string]any)["houseName"] != "1" || rs[0].(map[string]any)["apartment"] != float64(3) {
		t.Errorf("residencies = %v", rs)
	}
}

func TestVerifyWrongCode401(t *testing.T) {
	e := newAPIEnv(t)
	u := e.mkUser("+380500000001")
	ch := domain.Channel{UserID: u.ID, Type: "telegram", ExternalID: "1", ChatID: "1"}
	_ = e.r.Channels.Create(ctx, &ch)
	e.do("POST", "/api/v1/auth/code", map[string]string{"phone": u.Phone}, "", "")
	code := codeRe.FindString(e.msgr.last())
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	r := e.do("POST", "/api/v1/auth/verify", map[string]string{"phone": u.Phone, "code": wrong}, "", "")
	if r.Code != 401 || r.errCode() != "invalid_code" || len(r.Result().Cookies()) != 0 {
		t.Errorf("verify = %d %s", r.Code, r.Body)
	}
}

func TestMeRequiresSession(t *testing.T) {
	e := newAPIEnv(t)
	if r := e.do("GET", "/api/v1/me", nil, "", ""); r.Code != 401 {
		t.Errorf("me = %d", r.Code)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	e := newAPIEnv(t)
	token := e.login(e.mkUser("+380500000001"))
	r := e.do("POST", "/api/v1/auth/logout", nil, token, "")
	if r.Code != 204 {
		t.Fatalf("logout = %d", r.Code)
	}
	if c := r.Result().Cookies(); len(c) == 0 || c[0].MaxAge >= 0 {
		t.Errorf("cookie not cleared: %+v", c)
	}
	if r := e.do("GET", "/api/v1/me", nil, token, ""); r.Code != 401 {
		t.Errorf("me after logout = %d", r.Code)
	}
}

func TestAssetServed(t *testing.T) {
	e := newAPIEnv(t)
	adm := e.mkUser("+380500000099", admin)
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	a, err := e.api.Catalog.SetPicture(ctx, png, domain.Actor{UserID: &adm.ID})
	if err != nil {
		t.Fatal(err)
	}
	r := e.do("GET", "/api/v1/assets/"+itoa(a.ID), nil, "", "")
	if r.Code != 200 || r.Header().Get("Content-Type") != "image/png" || r.Header().Get("X-Content-Type-Options") != "nosniff" || r.Body.String() != string(png) {
		t.Errorf("asset = %d %v", r.Code, r.Header())
	}
	if v := e.do("GET", "/api/v1/site", nil, "", "").json(); v["pictureUrl"] != "/api/v1/assets/"+itoa(a.ID) {
		t.Errorf("pictureUrl = %v", v["pictureUrl"])
	}
	if r := e.do("GET", "/api/v1/assets/999", nil, "", ""); r.Code != http.StatusNotFound {
		t.Errorf("missing asset = %d", r.Code)
	}
}

func TestMalformedJSON400(t *testing.T) {
	e := newAPIEnv(t)
	if r := e.do("POST", "/api/v1/register", "{not json", "", ""); r.Code != 400 || r.errCode() != "bad_request" {
		t.Errorf("malformed = %d %s", r.Code, r.Body)
	}
}

func TestUnknownFieldRejected(t *testing.T) {
	e := newAPIEnv(t)
	body := e.registration("+380500000001")
	body["isAdmin"] = true
	if r := e.do("POST", "/api/v1/register", body, "", ""); r.Code != 400 {
		t.Errorf("unknown field = %d %s", r.Code, r.Body)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestRegisterOptionsFloors(t *testing.T) {
	e := newAPIEnv(t)
	nine := 9
	tall := domain.House{Name: "tall", ApartmentFrom: 100, ApartmentTo: 120, Floors: &nine}
	if err := e.r.Houses.Create(ctx, &tall); err != nil {
		t.Fatal(err)
	}
	body := e.do("GET", "/api/v1/register/options", nil, "", "").Body.String()
	if !strings.Contains(body, `"name":"tall","apartmentFrom":100,"apartmentTo":120,"floors":9`) || !strings.Contains(body, `"floors":null`) {
		t.Fatalf("options = %s", body)
	}
}
