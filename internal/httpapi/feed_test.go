package httpapi_test

import (
	"fmt"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// feedChat makes an accepted forum chat whose chat and topic carry one label
// each, and returns the two label ids.
func feedChat(t *testing.T, e *apiEnv, tok string) (int64, int64) {
	t.Helper()
	c := e.seenChat("-1001000000001", "Our yard")
	if r := e.do("POST", fmt.Sprintf("/api/v1/admin/chats/%d/accept", c.ID), nil, tok, ""); r.Code != 204 {
		t.Fatalf("accept = %d %s", r.Code, r.Body)
	}
	tp, err := e.r.Topics.GetByExternal(ctx, c.ID, "9")
	if err != nil {
		t.Fatal(err)
	}
	var labels [2]int64
	for i, name := range []string{"residents", "news"} {
		r := e.do("POST", "/api/v1/admin/labels", map[string]any{"name": name}, tok, "")
		if r.Code != 201 {
			t.Fatalf("label = %d %s", r.Code, r.Body)
		}
		labels[i] = int64(r.json()["id"].(float64))
	}
	if r := e.do("PUT", fmt.Sprintf("/api/v1/admin/chats/%d/labels", c.ID), map[string]any{"labelIds": []int64{labels[0], labels[1]}}, tok, ""); r.Code != 204 {
		t.Fatalf("chat labels = %d %s", r.Code, r.Body)
	}
	if r := e.do("PUT", fmt.Sprintf("/api/v1/admin/topics/%d/labels", tp.ID), map[string]any{"labelIds": []int64{labels[1]}}, tok, ""); r.Code != 204 {
		t.Fatalf("topic labels = %d %s", r.Code, r.Body)
	}
	return labels[0], labels[1]
}

func TestFeedAdminPublishFlow(t *testing.T) {
	e, tok := adminEnv(t)
	l1, l2 := feedChat(t, e, tok)
	img := dataImage(t, e, tok)
	if r := e.do("PUT", "/api/v1/admin/feed/settings", map[string]any{"defaultLabelIds": []int64{l1}}, tok, ""); r.Code != 200 {
		t.Fatalf("settings = %d %s", r.Code, r.Body)
	}
	if r := e.do("GET", "/api/v1/admin/feed/settings", nil, tok, ""); r.Code != 200 || fmt.Sprint(r.json()["defaultLabelIds"]) != fmt.Sprint([]any{float64(l1)}) {
		t.Fatalf("get settings = %d %s", r.Code, r.Body)
	}
	r := e.do("POST", "/api/v1/admin/feed/posts", map[string]any{"body": "", "imageAssetId": nil, "labelIds": []int64{}}, tok, "")
	if r.Code != 422 || r.json()["error"].(map[string]any)["fields"].(map[string]any)["body"] != "required" {
		t.Fatalf("empty = %d %s", r.Code, r.Body)
	}
	r = e.do("POST", "/api/v1/admin/feed/posts", map[string]any{"body": "Hello yard", "imageAssetId": img, "labelIds": []int64{l1}}, tok, "")
	if r.Code != 201 || r.json()["state"] != "draft" || r.json()["imageUrl"] != fmt.Sprintf("/api/v1/admin/assets/%d", img) {
		t.Fatalf("create = %d %s", r.Code, r.Body)
	}
	id := int64(r.json()["id"].(float64))
	r = e.do("POST", fmt.Sprintf("/api/v1/admin/feed/posts/%d/publish", id), map[string]any{"labelIds": []int64{l1, l2}}, tok, "")
	if r.Code != 200 || r.json()["state"] != "published" {
		t.Fatalf("publish = %d %s", r.Code, r.Body)
	}
	// The chat carries both labels, the topic one: two places, not three.
	ds := r.json()["deliveries"].([]any)
	if len(ds) != 2 || ds[0].(map[string]any)["chatTitle"] != "Our yard" || ds[1].(map[string]any)["topicTitle"] != "News" ||
		ds[0].(map[string]any)["state"] != "pending" {
		t.Fatalf("deliveries = %v", ds)
	}
	if d := r.json()["delivery"].(map[string]any); d["pending"] != float64(2) {
		t.Fatalf("delivery = %v", d)
	}
	r = e.do("GET", "/api/v1/admin/feed/posts", nil, tok, "")
	list, total := items(t, r)
	if total != 1 || list[0].(map[string]any)["delivery"].(map[string]any)["pending"] != float64(2) {
		t.Fatalf("list = %d %s", r.Code, r.Body)
	}
	if r := e.do("POST", fmt.Sprintf("/api/v1/admin/feed/posts/%d/publish", id), map[string]any{"labelIds": []int64{}}, tok, ""); r.Code != 409 {
		t.Fatalf("publish twice = %d", r.Code)
	}
	r = e.do("PATCH", fmt.Sprintf("/api/v1/admin/feed/posts/%d", id), map[string]any{"body": "Hello yard!", "imageAssetId": nil}, tok, "")
	if r.Code != 200 || r.json()["body"] != "Hello yard!" || r.json()["imageAssetId"] != nil {
		t.Fatalf("patch = %d %s", r.Code, r.Body)
	}
	if r := e.do("DELETE", fmt.Sprintf("/api/v1/admin/feed/posts/%d", id), nil, tok, ""); r.Code != 204 {
		t.Fatalf("delete = %d", r.Code)
	}
	if r := e.do("GET", fmt.Sprintf("/api/v1/admin/feed/posts/%d", id), nil, tok, ""); r.Code != 404 {
		t.Fatalf("get deleted = %d", r.Code)
	}
}

func TestFeedCabinet(t *testing.T) {
	e, tok := adminEnv(t)
	img := dataImage(t, e, tok)
	resident := e.login(e.mkUser("+380500000001", func(u *domain.User) { u.FirstName = "Olena" }))
	var ids []int64
	for i := range 22 {
		body := map[string]any{"body": fmt.Sprintf("post %d", i), "imageAssetId": nil, "labelIds": []int64{}}
		if i == 21 {
			body["imageAssetId"] = img
		}
		r := e.do("POST", "/api/v1/admin/feed/posts", body, tok, "")
		id := int64(r.json()["id"].(float64))
		ids = append(ids, id)
		e.do("POST", fmt.Sprintf("/api/v1/admin/feed/posts/%d/publish", id), map[string]any{"labelIds": []int64{}}, tok, "")
	}
	draft := e.do("POST", "/api/v1/admin/feed/posts", map[string]any{"body": "draft"}, tok, "")
	draftID := int64(draft.json()["id"].(float64))

	if r := e.do("GET", "/api/v1/feed/posts", nil, "", ""); r.Code != 401 {
		t.Fatalf("guest = %d", r.Code)
	}
	r := e.do("GET", "/api/v1/feed/posts", nil, resident, "")
	page := r.json()["items"].([]any)
	if r.Code != 200 || len(page) != 20 || r.json()["nextCursor"] == nil {
		t.Fatalf("page 1 = %d %s", r.Code, r.Body)
	}
	first := page[0].(map[string]any)
	if first["body"] != "post 21" || first["authorName"] != "F" || first["imageUrl"] != fmt.Sprintf("/api/v1/feed/posts/%d/image", ids[21]) {
		t.Fatalf("first = %v", first)
	}
	r = e.do("GET", "/api/v1/feed/posts?cursor="+r.json()["nextCursor"].(string), nil, resident, "")
	if r.Code != 200 || len(r.json()["items"].([]any)) != 2 || r.json()["nextCursor"] != nil {
		t.Fatalf("page 2 = %d %s", r.Code, r.Body)
	}
	if r := e.do("GET", fmt.Sprintf("/api/v1/feed/posts/%d", ids[3]), nil, resident, ""); r.Code != 200 || r.json()["body"] != "post 3" {
		t.Fatalf("one = %d %s", r.Code, r.Body)
	}
	if r := e.do("GET", fmt.Sprintf("/api/v1/feed/posts/%d", draftID), nil, resident, ""); r.Code != 404 {
		t.Fatalf("draft = %d", r.Code)
	}
	if r := e.do("GET", fmt.Sprintf("/api/v1/feed/posts/%d/image", ids[21]), nil, resident, ""); r.Code != 200 || r.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("image = %d %s", r.Code, r.Header())
	}
	if r := e.do("GET", fmt.Sprintf("/api/v1/feed/posts/%d/image", ids[0]), nil, resident, ""); r.Code != 404 {
		t.Fatalf("no image = %d", r.Code)
	}
	r = e.do("GET", "/api/v1/modules", nil, resident, "")
	if r.Code != 200 || r.Body.String() != `[{"code":"feed","nav":[{"key":"feed","path":"/my/feed","labelKey":"cabinet.nav.feed","icon":"news"}]}]`+"\n" {
		t.Fatalf("modules = %d %s", r.Code, r.Body)
	}
}

func TestModuleOffEffects(t *testing.T) {
	e, tok := adminEnv(t)
	resident := e.login(e.mkUser("+380500000001"))
	img := dataImage(t, e, tok)
	r := e.do("POST", "/api/v1/admin/feed/posts", map[string]any{"body": "hi", "imageAssetId": img}, tok, "")
	id := int64(r.json()["id"].(float64))
	e.do("POST", fmt.Sprintf("/api/v1/admin/feed/posts/%d/publish", id), map[string]any{"labelIds": []int64{}}, tok, "")
	if r := e.do("PATCH", "/api/v1/admin/data/feed.posts", map[string]any{"visibility": "public"}, tok, ""); r.Code != 200 {
		t.Fatalf("visibility = %d %s", r.Code, r.Body)
	}
	if r := e.do("GET", "/api/v1/public/data/feed.posts", nil, "", ""); r.Code != 200 {
		t.Fatalf("public on = %d", r.Code)
	}
	if r := e.do("GET", fmt.Sprintf("/api/v1/assets/%d", img), nil, "", ""); r.Code != 200 {
		t.Fatalf("asset on = %d", r.Code)
	}
	if r := e.do("PUT", "/api/v1/admin/modules/feed", map[string]any{"enabled": false}, resident, ""); r.Code != 403 {
		t.Fatalf("resident toggles = %d", r.Code)
	}
	r = e.do("PUT", "/api/v1/admin/modules/feed", map[string]any{"enabled": false}, tok, "")
	if r.Code != 200 || r.json()["enabled"] != false || r.json()["updatedBy"] == nil {
		t.Fatalf("off = %d %s", r.Code, r.Body)
	}
	if r := e.do("PUT", "/api/v1/admin/modules/nope", map[string]any{"enabled": false}, tok, ""); r.Code != 404 {
		t.Fatalf("unknown module = %d", r.Code)
	}
	if r := e.do("GET", "/api/v1/modules", nil, resident, ""); r.Code != 200 || r.Body.String() != "[]\n" {
		t.Fatalf("modules = %d %s", r.Code, r.Body)
	}
	for _, path := range []string{"/api/v1/feed/posts", fmt.Sprintf("/api/v1/feed/posts/%d", id)} {
		if r := e.do("GET", path, nil, resident, ""); r.Code != 404 || r.errCode() != "module_disabled" {
			t.Fatalf("%s = %d %s", path, r.Code, r.Body)
		}
	}
	if r := e.do("GET", "/api/v1/admin/feed/posts", nil, tok, ""); r.Code != 404 || r.errCode() != "module_disabled" {
		t.Fatalf("admin posts = %d %s", r.Code, r.Body)
	}
	if r := e.do("GET", "/api/v1/public/data/feed.posts", nil, "", ""); r.Code != 404 {
		t.Fatalf("public off = %d", r.Code)
	}
	if r := e.do("GET", "/api/v1/public/data", nil, "", ""); r.Code != 200 || keysOf(r.json()) != "" {
		t.Fatalf("public list off = %s", r.Body)
	}
	if r := e.do("GET", fmt.Sprintf("/api/v1/assets/%d", img), nil, "", ""); r.Code != 404 {
		t.Fatalf("asset off = %d", r.Code)
	}
	r = e.do("GET", "/api/v1/admin/data/feed.posts", nil, tok, "")
	if it := r.json()["item"].(map[string]any); r.Code != 200 || it["owner"] != "feed" || it["ownerEnabled"] != false {
		t.Fatalf("admin item = %d %s", r.Code, r.Body)
	}
	if r := e.do("PUT", "/api/v1/admin/modules/feed", map[string]any{"enabled": true}, tok, ""); r.Code != 200 {
		t.Fatalf("on = %d", r.Code)
	}
	if r := e.do("GET", "/api/v1/public/data/feed.posts", nil, "", ""); r.Code != 200 {
		t.Fatalf("public back on = %d", r.Code)
	}
}

func TestOwnedDataItemHTTP(t *testing.T) {
	e, tok := adminEnv(t)
	if r := e.do("PATCH", "/api/v1/admin/data/feed.posts", map[string]any{"description": "mine"}, tok, ""); r.Code != 409 || r.errCode() != "module_owned" {
		t.Fatalf("patch = %d %s", r.Code, r.Body)
	}
	if r := e.do("DELETE", "/api/v1/admin/data/feed.posts", nil, tok, ""); r.Code != 409 {
		t.Fatalf("delete = %d", r.Code)
	}
	if r := e.do("PUT", "/api/v1/admin/data/feed.posts/rows", map[string]any{"rows": []any{}}, tok, ""); r.Code != 409 {
		t.Fatalf("rows = %d", r.Code)
	}
	r := e.do("POST", "/api/v1/admin/data", map[string]any{"key": "feed.mine", "kind": "value", "valueType": "text", "value": "x"}, tok, "")
	if r.Code != 422 || r.json()["error"].(map[string]any)["fields"].(map[string]any)["key"] != "key_reserved" {
		t.Fatalf("reserved = %d %s", r.Code, r.Body)
	}
}

func TestFeedDeliveryToPersonIsNamed(t *testing.T) {
	e, tok := adminEnv(t)
	u := e.mkUser("+380500000020", func(u *domain.User) { u.FirstName, u.LastName = "Ann", "Lee" })
	if err := e.r.Channels.Create(ctx, &domain.Channel{UserID: u.ID, Type: "telegram", ExternalID: "333", ChatID: "333"}); err != nil {
		t.Fatal(err)
	}
	r := e.do("POST", "/api/v1/admin/labels", map[string]any{"name": "testers"}, tok, "")
	l := int64(r.json()["id"].(float64))
	if r := e.do("PUT", fmt.Sprintf("/api/v1/admin/users/%d/labels", u.ID), map[string]any{"labelIds": []int64{l}}, tok, ""); r.Code != 204 {
		t.Fatalf("user labels = %d %s", r.Code, r.Body)
	}
	r = e.do("POST", "/api/v1/admin/feed/posts", map[string]any{"body": "Try this", "imageAssetId": nil, "labelIds": []int64{l}}, tok, "")
	id := int64(r.json()["id"].(float64))
	r = e.do("POST", fmt.Sprintf("/api/v1/admin/feed/posts/%d/publish", id), map[string]any{"labelIds": []int64{l}}, tok, "")
	ds, _ := r.json()["deliveries"].([]any)
	if r.Code != 200 || len(ds) != 1 || ds[0].(map[string]any)["chatTitle"] != "Ann Lee" {
		t.Fatalf("publish = %d deliveries %v", r.Code, ds)
	}
}
