package httpapi_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func (e *apiEnv) adminToken() (domain.User, string) {
	e.t.Helper()
	u := e.mkUser("+380500000090", admin)
	return u, e.login(u)
}

// seenChat records a chat the bot found from a message (state new).
func (e *apiEnv) seenChat(ext, title string) domain.Chat {
	e.t.Helper()
	info := domain.ChatInfo{Messenger: "telegram", ExternalID: ext, Kind: domain.ChatKindSupergroup, Title: title, IsForum: true}
	if _, err := e.api.Chats.ChatSeen(ctx, info); err != nil {
		e.t.Fatal(err)
	}
	if err := e.api.Chats.TopicSeen(ctx, info, "9", "News", true); err != nil {
		e.t.Fatal(err)
	}
	c, err := e.r.Chats.GetByExternal(ctx, "telegram", ext)
	if err != nil {
		e.t.Fatal(err)
	}
	return c
}

func TestChatsEndpointsAdminOnly(t *testing.T) {
	e := newAPIEnv(t)
	resident := e.login(e.mkUser("+380500000001"))
	routes := [][2]string{
		{"GET", "/api/v1/admin/chats"}, {"POST", "/api/v1/admin/chats/1/accept"}, {"POST", "/api/v1/admin/chats/1/leave"},
		{"PUT", "/api/v1/admin/chats/1/labels"}, {"PUT", "/api/v1/admin/topics/1/labels"}, {"PUT", "/api/v1/admin/users/1/labels"}, {"PATCH", "/api/v1/admin/topics/1"},
		{"POST", "/api/v1/admin/chats/1/test"}, {"POST", "/api/v1/admin/topics/1/test"}, {"POST", "/api/v1/admin/labels/1/test"},
		{"GET", "/api/v1/admin/outbox/1"}, {"GET", "/api/v1/admin/labels"}, {"POST", "/api/v1/admin/labels"},
		{"PATCH", "/api/v1/admin/labels/1"}, {"DELETE", "/api/v1/admin/labels/1"},
		{"GET", "/api/v1/admin/settings/notifications"}, {"PUT", "/api/v1/admin/settings/notifications"},
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

func TestListChatsShape(t *testing.T) {
	e := newAPIEnv(t)
	_, tok := e.adminToken()
	c := e.seenChat("-1001000000001", "yard")
	r := e.do("GET", "/api/v1/admin/chats", nil, tok, "")
	if r.Code != 200 {
		t.Fatalf("status %d %s", r.Code, r.Body)
	}
	body := r.Body.String()
	for _, want := range []string{`"id":` + fmt.Sprint(c.ID), `"messenger":"telegram"`, `"kind":"supergroup"`, `"title":"yard"`,
		`"isForum":true`, `"state":"new"`, `"botStatus":"unknown"`, `"canSend":false`, `"labelIds":[]`, `"leaveAt":null`,
		`"topics":[{`, `"externalId":"9"`, `"title":"News"`, `"gone":false`, `"addedBy":{"externalId":"","userId":null}`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in %s", want, body)
		}
	}
}

func TestAcceptLeave(t *testing.T) {
	e := newAPIEnv(t)
	_, tok := e.adminToken()
	c := e.seenChat("-1001000000001", "yard")
	path := fmt.Sprintf("/api/v1/admin/chats/%d", c.ID)
	if r := e.do("POST", path+"/accept", nil, tok, ""); r.Code != 204 {
		t.Fatalf("accept %d %s", r.Code, r.Body)
	}
	if r := e.do("POST", path+"/accept", nil, tok, ""); r.Code != 409 {
		t.Fatalf("second accept %d", r.Code)
	}
	if r := e.do("POST", path+"/leave", nil, tok, ""); r.Code != 204 {
		t.Fatalf("leave %d %s", r.Code, r.Body)
	}
	if got, _ := e.r.Chats.Get(ctx, c.ID); got.State != domain.ChatLeft {
		t.Fatalf("chat = %+v", got)
	}
	if r := e.do("POST", "/api/v1/admin/chats/999/accept", nil, tok, ""); r.Code != 404 {
		t.Fatalf("unknown chat %d", r.Code)
	}
}

func TestSetLabelsValidation(t *testing.T) {
	e := newAPIEnv(t)
	_, tok := e.adminToken()
	c := e.seenChat("-1001000000001", "yard")
	r := e.do("POST", "/api/v1/admin/labels", map[string]string{"name": "journal"}, tok, "")
	if r.Code != 201 {
		t.Fatalf("create label %d %s", r.Code, r.Body)
	}
	id := int64(r.json()["id"].(float64))
	path := fmt.Sprintf("/api/v1/admin/chats/%d/labels", c.ID)
	if r := e.do("PUT", path, map[string]any{"labelIds": []int64{id}}, tok, ""); r.Code != 204 {
		t.Fatalf("set labels %d %s", r.Code, r.Body)
	}
	if r := e.do("PUT", path, map[string]any{"labelIds": []int64{999}}, tok, ""); r.Code != 422 || r.errCode() != "validation" {
		t.Fatalf("unknown label %d %s", r.Code, r.Body)
	}
	tp, _ := e.r.Topics.GetByExternal(ctx, c.ID, "9")
	if r := e.do("PUT", fmt.Sprintf("/api/v1/admin/topics/%d/labels", tp.ID), map[string]any{"labelIds": []int64{id}}, tok, ""); r.Code != 204 {
		t.Fatalf("topic labels %d %s", r.Code, r.Body)
	}
	if r := e.do("PATCH", fmt.Sprintf("/api/v1/admin/topics/%d", tp.ID), map[string]string{"title": "Notices"}, tok, ""); r.Code != 204 {
		t.Fatalf("rename topic %d %s", r.Code, r.Body)
	}
	if got, _ := e.r.Topics.Get(ctx, tp.ID); got.Title != "Notices" {
		t.Fatalf("topic = %+v", got)
	}
}

func TestTestSendNotAccepted(t *testing.T) {
	e := newAPIEnv(t)
	_, tok := e.adminToken()
	c := e.seenChat("-1001000000001", "yard")
	r := e.do("POST", fmt.Sprintf("/api/v1/admin/chats/%d/test", c.ID), nil, tok, "")
	if r.Code != 409 || r.errCode() != "chat_not_accepted" {
		t.Fatalf("test send %d %s", r.Code, r.Body)
	}
}

func TestOutboxStatus(t *testing.T) {
	e := newAPIEnv(t)
	_, tok := e.adminToken()
	c := e.seenChat("-1001000000001", "yard")
	e.do("POST", fmt.Sprintf("/api/v1/admin/chats/%d/accept", c.ID), nil, tok, "")
	r := e.do("POST", fmt.Sprintf("/api/v1/admin/chats/%d/test", c.ID), nil, tok, "")
	if r.Code != 202 {
		t.Fatalf("test send %d %s", r.Code, r.Body)
	}
	ids := r.json()["outboxIds"].([]any)
	if len(ids) != 1 {
		t.Fatalf("ids = %v", ids)
	}
	r = e.do("GET", fmt.Sprintf("/api/v1/admin/outbox/%d", int64(ids[0].(float64))), nil, tok, "")
	if r.Code != 200 || r.json()["state"] != "pending" || r.json()["lastError"] != "" {
		t.Fatalf("outbox %d %s", r.Code, r.Body)
	}
}

func TestLabelsCRUDAndTaken(t *testing.T) {
	e := newAPIEnv(t)
	_, tok := e.adminToken()
	r := e.do("POST", "/api/v1/admin/labels", map[string]string{"name": "Журнал", "description": "admins"}, tok, "")
	if r.Code != 201 || r.json()["name"] != "Журнал" {
		t.Fatalf("create %d %s", r.Code, r.Body)
	}
	id := int64(r.json()["id"].(float64))
	r = e.do("POST", "/api/v1/admin/labels", map[string]string{"name": "журнал"}, tok, "")
	if f, _ := r.json()["error"].(map[string]any)["fields"].(map[string]any); r.Code != 422 || f["name"] != "taken" {
		t.Fatalf("duplicate %d %s", r.Code, r.Body)
	}
	if r := e.do("PATCH", fmt.Sprintf("/api/v1/admin/labels/%d", id), map[string]string{"name": "Journal", "description": ""}, tok, ""); r.Code != 200 || r.json()["name"] != "Journal" {
		t.Fatalf("rename %d %s", r.Code, r.Body)
	}
	r = e.do("GET", "/api/v1/admin/labels", nil, tok, "")
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"targets":[]`) || !strings.Contains(r.Body.String(), `"usedByApplications":false`) {
		t.Fatalf("list %d %s", r.Code, r.Body)
	}
	if r := e.do("DELETE", fmt.Sprintf("/api/v1/admin/labels/%d", id), nil, tok, ""); r.Code != 204 {
		t.Fatalf("delete %d %s", r.Code, r.Body)
	}
}

func TestNotificationsSettings(t *testing.T) {
	e := newAPIEnv(t)
	_, tok := e.adminToken()
	r := e.do("GET", "/api/v1/admin/settings/notifications", nil, tok, "")
	if r.Code != 200 || r.Body.String() != `{"applicationsLabelId":null}`+"\n" {
		t.Fatalf("get %d %q", r.Code, r.Body)
	}
	r = e.do("POST", "/api/v1/admin/labels", map[string]string{"name": "journal"}, tok, "")
	id := int64(r.json()["id"].(float64))
	if r := e.do("PUT", "/api/v1/admin/settings/notifications", map[string]any{"applicationsLabelId": id}, tok, ""); r.Code != 200 {
		t.Fatalf("put %d %s", r.Code, r.Body)
	}
	if r := e.do("PUT", "/api/v1/admin/settings/notifications", map[string]any{"applicationsLabelId": 999}, tok, ""); r.Code != 422 {
		t.Fatalf("bad label %d %s", r.Code, r.Body)
	}
	if r := e.do("GET", "/api/v1/admin/settings/notifications", nil, tok, ""); r.json()["applicationsLabelId"] != float64(id) {
		t.Fatalf("after put %s", r.Body)
	}
}

func TestSummaryNewChats(t *testing.T) {
	e := newAPIEnv(t)
	_, tok := e.adminToken()
	e.seenChat("-1001000000001", "yard")
	r := e.do("GET", "/api/v1/admin/summary", nil, tok, "")
	if r.Code != 200 || r.json()["newChats"] != float64(1) || r.json()["pendingApplications"] != float64(0) {
		t.Fatalf("summary %d %s", r.Code, r.Body)
	}
}

func (e *apiEnv) botLink(external, phone string) string {
	e.t.Helper()
	a, err := e.api.Auth.ActivateChannel(ctx, "telegram", external, external, "", phone)
	if err != nil || a.Result != domain.ApplyLink {
		e.t.Fatalf("activation = %+v, %v", a, err)
	}
	return a.RegistrationURL[strings.Index(a.RegistrationURL, "#t=")+3:]
}

func TestPrefillValidAndInvalid(t *testing.T) {
	e := newAPIEnv(t)
	tok := e.botLink("2001", "+380500000011")
	r := e.do("POST", "/api/v1/register/prefill", map[string]string{"token": tok}, "", "")
	if r.Code != 200 || r.json()["phone"] != "+380500000011" {
		t.Fatalf("prefill %d %s", r.Code, r.Body)
	}
	r = e.do("POST", "/api/v1/register/prefill", map[string]string{"token": "nope"}, "", "")
	if r.Code != 422 || r.errCode() != "token_invalid" {
		t.Fatalf("bad token %d %s", r.Code, r.Body)
	}
}

func TestRegisterWithToken(t *testing.T) {
	e := newAPIEnv(t)
	tok := e.botLink("2001", "+380500000011")
	body := map[string]any{"token": tok, "firstName": "Ivan", "lastName": "P", "houseId": e.house.ID, "apartment": 3,
		"statusId": e.owner.ID, "consent": true, "locale": "uk"}
	if r := e.do("POST", "/api/v1/register", body, "", ""); r.Code != 202 {
		t.Fatalf("register %d %s", r.Code, r.Body)
	}
	u, err := e.r.Users.GetByPhone(ctx, "+380500000011")
	if err != nil || u.Status != domain.StatusPendingApproval {
		t.Fatalf("user = %+v, %v", u, err)
	}
	if r := e.do("POST", "/api/v1/register", body, "", ""); r.Code != 422 || r.errCode() != "token_invalid" {
		t.Fatalf("reuse %d %s", r.Code, r.Body)
	}
}

func TestSetUserLabels(t *testing.T) {
	e := newAPIEnv(t)
	_, tok := e.adminToken()
	u := e.mkUser("+380500000002", func(u *domain.User) { u.FirstName, u.LastName = "Ann", "Lee" })
	if err := e.r.Channels.Create(ctx, &domain.Channel{UserID: u.ID, Type: "telegram", ExternalID: "222", ChatID: "222"}); err != nil {
		t.Fatal(err)
	}
	r := e.do("POST", "/api/v1/admin/labels", map[string]string{"name": "testers"}, tok, "")
	id := int64(r.json()["id"].(float64))
	path := fmt.Sprintf("/api/v1/admin/users/%d/labels", u.ID)
	if r := e.do("PUT", path, map[string]any{"labelIds": []int64{id}}, tok, ""); r.Code != 204 {
		t.Fatalf("set labels %d %s", r.Code, r.Body)
	}
	if r := e.do("GET", fmt.Sprintf("/api/v1/admin/users/%d", u.ID), nil, tok, ""); !strings.Contains(r.Body.String(), fmt.Sprintf(`"labelIds":[%d]`, id)) {
		t.Fatalf("card %d %s", r.Code, r.Body)
	}
	want := fmt.Sprintf(`"targets":[{"type":"user","id":%d}],"users":[{"id":%d,"name":"Ann Lee","reachable":true}]`, u.ID, u.ID)
	if r := e.do("GET", "/api/v1/admin/labels", nil, tok, ""); !strings.Contains(r.Body.String(), want) {
		t.Fatalf("labels %s; want %s", r.Body, want)
	}
	if r := e.do("PUT", "/api/v1/admin/users/9999/labels", map[string]any{"labelIds": []int64{id}}, tok, ""); r.Code != 404 {
		t.Fatalf("unknown user %d %s", r.Code, r.Body)
	}
	resident := e.login(e.mkUser("+380500000003"))
	if r := e.do("PUT", path, map[string]any{"labelIds": []int64{}}, resident, ""); r.Code != 403 {
		t.Fatalf("resident %d", r.Code)
	}
	if r := e.do("GET", fmt.Sprintf("/api/v1/admin/users/%d", e.mkUser("+380500000004").ID), nil, tok, ""); !strings.Contains(r.Body.String(), `"labelIds":[]`) {
		t.Fatalf("card without labels %s", r.Body)
	}
}
