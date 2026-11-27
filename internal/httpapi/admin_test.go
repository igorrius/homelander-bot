package httpapi_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/httpapi"
)

func TestAdminRoutesRequireAdmin(t *testing.T) {
	e := newAPIEnv(t)
	resident := e.login(e.mkUser("+380500000001"))
	routes := [][2]string{
		{"GET", "/api/v1/admin/summary"}, {"GET", "/api/v1/admin/users"}, {"POST", "/api/v1/admin/users"},
		{"GET", "/api/v1/admin/users/1"}, {"PATCH", "/api/v1/admin/users/1"}, {"POST", "/api/v1/admin/users/1/approve"},
		{"POST", "/api/v1/admin/users/1/reject"}, {"POST", "/api/v1/admin/users/1/disable"}, {"POST", "/api/v1/admin/users/1/enable"},
		{"DELETE", "/api/v1/admin/users/1/channels/1"}, {"POST", "/api/v1/admin/users/1/residencies"},
		{"PATCH", "/api/v1/admin/residencies/1"}, {"DELETE", "/api/v1/admin/residencies/1"},
		{"GET", "/api/v1/admin/houses"}, {"POST", "/api/v1/admin/houses"}, {"GET", "/api/v1/admin/houses/1"},
		{"PATCH", "/api/v1/admin/houses/1"}, {"GET", "/api/v1/admin/houses/1/apartments/3"},
		{"GET", "/api/v1/admin/statuses"}, {"POST", "/api/v1/admin/statuses"}, {"PATCH", "/api/v1/admin/statuses/1"},
		{"GET", "/api/v1/admin/site"}, {"PUT", "/api/v1/admin/site"}, {"PUT", "/api/v1/admin/site/picture"},
		{"DELETE", "/api/v1/admin/site/picture"}, {"GET", "/api/v1/admin/audit"},
	}
	for _, rt := range routes {
		if r := e.do(rt[0], rt[1], "{}", "", ""); r.Code != 401 {
			t.Errorf("%s %s anonymous = %d", rt[0], rt[1], r.Code)
		}
		if r := e.do(rt[0], rt[1], "{}", resident, ""); r.Code != 403 {
			t.Errorf("%s %s resident = %d", rt[0], rt[1], r.Code)
		}
	}
}

func adminEnv(t *testing.T) (*apiEnv, string) {
	e := newAPIEnv(t)
	return e, e.login(e.mkUser("+380500000099", admin))
}

func items(t *testing.T, r resp) ([]any, float64) {
	t.Helper()
	v := r.json()
	list, _ := v["items"].([]any)
	total, _ := v["total"].(float64)
	return list, total
}

func TestUsersListSearchAndPaging(t *testing.T) {
	e, tok := adminEnv(t)
	for i := range 30 {
		e.mkUser(fmt.Sprintf("+3805000001%02d", i), func(u *domain.User) { u.LastName = fmt.Sprintf("Петренко%02d", i) })
	}
	list, total := items(t, e.do("GET", "/api/v1/admin/users?q=петренко&page=2&pageSize=25", nil, tok, ""))
	if total != 30 || len(list) != 5 {
		t.Errorf("page 2: %d items, total %v", len(list), total)
	}
	list, total = items(t, e.do("GET", "/api/v1/admin/users?isAdmin=true", nil, tok, ""))
	if total != 1 || list[0].(map[string]any)["hasTelegram"] != true || list[0].(map[string]any)["status"] != "active" {
		t.Errorf("admins = %v", list)
	}
	if r := e.do("GET", "/api/v1/admin/users?pageSize=1000", nil, tok, ""); r.Code != 200 {
		t.Errorf("oversized page = %d", r.Code)
	}
	if r := e.do("GET", "/api/v1/admin/users?status=bogus", nil, tok, ""); r.Code != 422 {
		t.Errorf("bad status filter = %d", r.Code)
	}
}

func TestApproveFlow(t *testing.T) {
	e, tok := adminEnv(t)
	e.do("POST", "/api/v1/register", e.registration("+380500000001"), "", "")
	if v := e.do("GET", "/api/v1/admin/summary", nil, tok, "").json(); v["pendingApplications"] != float64(1) {
		t.Fatalf("summary = %v", v)
	}
	list, _ := items(t, e.do("GET", "/api/v1/admin/users?status=pending_approval", nil, tok, ""))
	id := int64(list[0].(map[string]any)["id"].(float64))
	r := e.do("POST", "/api/v1/admin/users/"+strconv.FormatInt(id, 10)+"/approve", nil, tok, "")
	if r.Code != 200 || r.json()["user"].(map[string]any)["status"] != "awaiting_activation" {
		t.Errorf("approve = %d %s", r.Code, r.Body)
	}
	if v := e.do("GET", "/api/v1/admin/summary", nil, tok, "").json(); v["pendingApplications"] != float64(0) {
		t.Errorf("summary after = %v", v)
	}
	if r := e.do("POST", "/api/v1/admin/users/"+strconv.FormatInt(id, 10)+"/approve", nil, tok, ""); r.Code != 409 {
		t.Errorf("approve twice = %d", r.Code)
	}
}

func TestAddUserAndCard(t *testing.T) {
	e, tok := adminEnv(t)
	r := e.do("POST", "/api/v1/admin/users", map[string]any{"phone": "0500000005", "firstName": "Ада", "lastName": "Лав", "isResident": false, "isAdmin": true}, tok, "")
	if r.Code != 201 {
		t.Fatalf("add = %d %s", r.Code, r.Body)
	}
	card := r.json()
	u := card["user"].(map[string]any)
	if u["phone"] != "+380500000005" || u["status"] != "awaiting_activation" || u["isAdmin"] != true {
		t.Errorf("user = %v", u)
	}
	id := strconv.Itoa(int(u["id"].(float64)))
	got := e.do("GET", "/api/v1/admin/users/"+id, nil, tok, "").json()
	if _, ok := got["channels"].([]any); !ok {
		t.Errorf("channels = %#v", got["channels"])
	}
	audit := got["audit"].([]any)
	if len(audit) != 1 || audit[0].(map[string]any)["action"] != "user.created" {
		t.Errorf("audit = %v", audit)
	}
	if r := e.do("GET", "/api/v1/admin/users/9999", nil, tok, ""); r.Code != 404 {
		t.Errorf("missing user = %d", r.Code)
	}
}

func TestPatchUserFlags(t *testing.T) {
	e, tok := adminEnv(t)
	me, _ := e.r.Users.GetByPhone(ctx, "+380500000099")
	r := e.do("PATCH", "/api/v1/admin/users/"+strconv.FormatInt(me.ID, 10), map[string]any{"isAdmin": false}, tok, "")
	if r.Code != 409 || r.errCode() != "last_admin" {
		t.Errorf("revoke last admin = %d %s", r.Code, r.Body)
	}
	other := e.mkUser("+380500000001")
	r = e.do("PATCH", "/api/v1/admin/users/"+strconv.FormatInt(other.ID, 10), map[string]any{"isAdmin": true, "firstName": "Нове"}, tok, "")
	if r.Code != 200 || r.json()["user"].(map[string]any)["isAdmin"] != true || r.json()["user"].(map[string]any)["firstName"] != "Нове" {
		t.Errorf("grant = %d %s", r.Code, r.Body)
	}
}

func TestUnlinkChannel(t *testing.T) {
	e, tok := adminEnv(t)
	u := e.mkUser("+380500000001")
	ch := domain.Channel{UserID: u.ID, Type: "telegram", ExternalID: "77", ChatID: "77"}
	_ = e.r.Channels.Create(ctx, &ch)
	r := e.do("DELETE", fmt.Sprintf("/api/v1/admin/users/%d/channels/%d", u.ID, ch.ID), nil, tok, "")
	if r.Code != 200 || r.json()["user"].(map[string]any)["status"] != "awaiting_activation" || len(r.json()["channels"].([]any)) != 0 {
		t.Errorf("unlink = %d %s", r.Code, r.Body)
	}
}

func TestResidencyCRUD(t *testing.T) {
	e, tok := adminEnv(t)
	u := e.mkUser("+380500000001")
	r := e.do("POST", fmt.Sprintf("/api/v1/admin/users/%d/residencies", u.ID), map[string]any{"houseId": e.house.ID, "apartment": 4, "statusId": e.owner.ID}, tok, "")
	if r.Code != 201 {
		t.Fatalf("add residency = %d %s", r.Code, r.Body)
	}
	res := r.json()["residencies"].([]any)[0].(map[string]any)
	rid := int64(res["id"].(float64))
	if res["houseName"] != "1" || res["apartment"] != float64(4) {
		t.Errorf("residency = %v", res)
	}
	r = e.do("PATCH", fmt.Sprintf("/api/v1/admin/residencies/%d", rid), map[string]any{"houseId": e.house.ID, "apartment": 5, "statusId": e.owner.ID}, tok, "")
	if r.Code != 200 || r.json()["residencies"].([]any)[0].(map[string]any)["apartment"] != float64(5) {
		t.Errorf("patch residency = %d %s", r.Code, r.Body)
	}
	r = e.do("DELETE", fmt.Sprintf("/api/v1/admin/residencies/%d", rid), nil, tok, "")
	if r.Code != 200 || len(r.json()["residencies"].([]any)) != 0 {
		t.Errorf("delete residency = %d %s", r.Code, r.Body)
	}
}

func TestHouseDetailStats(t *testing.T) {
	e, tok := adminEnv(t)
	r := e.do("POST", "/api/v1/admin/houses", map[string]any{"name": "24", "address": "Test st. 24", "latitude": 50.4, "longitude": 30.5,
		"floors": 16, "apartmentFrom": 121, "apartmentTo": 240, "sortOrder": 2, "archived": false}, tok, "")
	if r.Code != 201 {
		t.Fatalf("create house = %d %s", r.Code, r.Body)
	}
	hid := int64(r.json()["id"].(float64))
	e.do("POST", "/api/v1/register", map[string]any{"phone": "+380500000001", "firstName": "A", "lastName": "B", "houseId": hid,
		"apartment": 130, "statusId": e.owner.ID, "consent": true, "locale": "uk"}, "", "")
	v := e.do("GET", fmt.Sprintf("/api/v1/admin/houses/%d", hid), nil, tok, "").json()
	st := v["stats"].(map[string]any)
	if v["house"].(map[string]any)["floors"] != float64(16) || st["apartmentsTotal"] != float64(120) || st["people"] != float64(1) ||
		st["apartments"].(map[string]any)["130"] != float64(1) || st["byUserStatus"].(map[string]any)["pending_approval"] != float64(1) ||
		st["byResidencyStatus"].(map[string]any)[strconv.FormatInt(e.owner.ID, 10)] != float64(1) {
		t.Errorf("house detail = %v", v)
	}
	r = e.do("PATCH", fmt.Sprintf("/api/v1/admin/houses/%d", hid), map[string]any{"name": "24", "apartmentFrom": 121, "apartmentTo": 125}, tok, "")
	if r.Code != 422 {
		t.Errorf("narrowing = %d %s", r.Code, r.Body)
	}
	list := e.do("GET", "/api/v1/admin/houses", nil, tok, "")
	var hs []any
	_ = jsonUnmarshal(list.Body.Bytes(), &hs)
	if len(hs) != 2 {
		t.Errorf("houses = %v", hs)
	}
}

func TestApartmentResidents(t *testing.T) {
	e, tok := adminEnv(t)
	e.do("POST", "/api/v1/register", e.registration("+380500000001"), "", "")
	r := e.do("GET", fmt.Sprintf("/api/v1/admin/houses/%d/apartments/3", e.house.ID), nil, tok, "")
	var list []map[string]any
	_ = jsonUnmarshal(r.Body.Bytes(), &list)
	if r.Code != 200 || len(list) != 1 || list[0]["user"].(map[string]any)["phone"] != "+380500000001" {
		t.Errorf("residents = %d %s", r.Code, r.Body)
	}
}

func TestStatusCRUD(t *testing.T) {
	e, tok := adminEnv(t)
	r := e.do("POST", "/api/v1/admin/statuses", map[string]any{"code": "relative", "names": map[string]string{"uk": "Родич", "en": "Relative"}, "sortOrder": 3}, tok, "")
	if r.Code != 201 {
		t.Fatalf("create status = %d %s", r.Code, r.Body)
	}
	id := int64(r.json()["id"].(float64))
	r = e.do("PATCH", fmt.Sprintf("/api/v1/admin/statuses/%d", id), map[string]any{"code": "relative", "names": map[string]string{"uk": "Член родини"}, "archived": true}, tok, "")
	if r.Code != 200 || r.json()["archived"] != true {
		t.Errorf("patch status = %d %s", r.Code, r.Body)
	}
	var all []any
	_ = jsonUnmarshal(e.do("GET", "/api/v1/admin/statuses", nil, tok, "").Body.Bytes(), &all)
	if len(all) != 3 {
		t.Errorf("statuses incl. archived = %d", len(all))
	}
}

func upload(e *apiEnv, tok string, data []byte) resp {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("picture", "p.bin")
	_, _ = fw.Write(data)
	_ = mw.Close()
	req := httptest.NewRequest("PUT", "/api/v1/admin/site/picture", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Requested-With", "homelander")
	req.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: tok})
	req.RemoteAddr = "10.8.0.1:1"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return resp{rec, e.t}
}

func TestSitePictureUpload(t *testing.T) {
	e, tok := adminEnv(t)
	r := e.do("PUT", "/api/v1/admin/site", map[string]any{"communityName": map[string]string{"uk": "Наш ЖК"}, "welcomeText": map[string]string{"uk": "Вітаємо"}, "defaultLocale": "uk"}, tok, "")
	if r.Code != 200 || r.json()["communityName"].(map[string]any)["uk"] != "Наш ЖК" {
		t.Fatalf("save site = %d %s", r.Code, r.Body)
	}
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	r = upload(e, tok, png)
	url, _ := r.json()["pictureUrl"].(string)
	if r.Code != 200 || url == "" {
		t.Fatalf("upload = %d %s", r.Code, r.Body)
	}
	if got := e.do("GET", url, nil, "", ""); got.Code != 200 || got.Body.String() != string(png) {
		t.Errorf("served picture = %d", got.Code)
	}
	r = upload(e, tok, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	if r.Code != 422 || r.json()["error"].(map[string]any)["fields"].(map[string]any)["picture"] != "type" {
		t.Errorf("svg = %d %s", r.Code, r.Body)
	}
	r = upload(e, tok, append(append([]byte{}, png...), make([]byte, 5<<20)...))
	if r.Code != 422 {
		t.Errorf("oversize = %d %s", r.Code, r.Body)
	}
	r = e.do("DELETE", "/api/v1/admin/site/picture", nil, tok, "")
	if r.Code != 200 || r.json()["pictureUrl"] != nil {
		t.Errorf("delete picture = %d %s", r.Code, r.Body)
	}
}

func TestAuditList(t *testing.T) {
	e, tok := adminEnv(t)
	u := e.mkUser("+380500000001")
	e.do("POST", fmt.Sprintf("/api/v1/admin/users/%d/disable", u.ID), nil, tok, "")
	list, total := items(t, e.do("GET", fmt.Sprintf("/api/v1/admin/audit?userId=%d", u.ID), nil, tok, ""))
	if total != 1 || list[0].(map[string]any)["action"] != "user.disabled" || list[0].(map[string]any)["actorUserId"] == nil {
		t.Errorf("audit = %v", list)
	}
	_, all := items(t, e.do("GET", "/api/v1/admin/audit", nil, tok, ""))
	if all < 3 {
		t.Errorf("all audit total = %v", all)
	}
}
