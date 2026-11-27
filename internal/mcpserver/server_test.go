package mcpserver_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/mcpserver"
	"github.com/igorrius/homelander-bot/internal/ratelimit"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite/sqlitetest"
)

var ctx = context.Background()

const (
	cb       = "https://claude.ai/api/mcp/auth_callback"
	verifier = "a-verifier-that-is-long-enough-for-pkce-0123456789"
	pubURL   = "https://example.com"
)

// clock follows real time (the SDK checks token expiry against time.Now) and
// can be moved forward.
type clock struct {
	mu  sync.Mutex
	off time.Duration
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().UTC().Add(c.off)
}

type env struct {
	t      *testing.T
	r      domain.Repos
	db     sqlite.DB
	deps   mcpserver.Deps
	users  *domain.UserService
	srv    *httptest.Server
	admin  domain.User
	grant  domain.MCPGrant
	access string
}

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db := sqlitetest.New(t)
	r := sqlite.NewRepos(db)
	c := &clock{}
	limits := ratelimit.New(c.Now)
	auth := &domain.McpAuthService{R: r, Now: c.Now, Allowed: domain.RedirectPolicy{cb}, Resource: pubURL + "/mcp"}
	e := &env{t: t, r: r, db: db}
	e.users = &domain.UserService{R: r, Now: c.Now, Limits: limits, Region: "UA"}
	e.deps = mcpserver.Deps{
		Auth:      auth,
		Data:      &domain.DataService{R: r, Now: c.Now, Limits: limits, Locales: []string{"uk", "en"}},
		Pages:     &domain.PageService{R: r, Now: c.Now, Secret: []byte("0123456789abcdef0123456789abcdef")},
		Catalog:   &domain.CatalogService{R: r, Now: c.Now, Locales: []string{"uk", "en"}},
		Limits:    limits,
		PublicURL: pubURL,
		Now:       c.Now,
	}
	e.srv = httptest.NewServer(mcpserver.New(e.deps))
	t.Cleanup(e.srv.Close)
	e.admin = e.mkUser("+380500000099", func(u *domain.User) { u.IsAdmin, u.IsMCP = true, true })
	e.mkUser("+380500000098", func(u *domain.User) { u.IsAdmin = true })
	e.access = e.token(e.admin)
	gs, _ := auth.Grants(ctx, e.admin.ID)
	e.grant = gs[0]
	return e
}

func (e *env) mkUser(phone string, mod func(*domain.User)) domain.User {
	e.t.Helper()
	u := domain.User{Phone: phone, FirstName: "F", LastName: "L", Status: domain.StatusActive, Source: domain.SourceAdmin}
	mod(&u)
	if err := e.r.Users.Create(ctx, &u); err != nil {
		e.t.Fatal(err)
	}
	return u
}

func (e *env) token(u domain.User) string {
	e.t.Helper()
	c, err := e.deps.Auth.Register(ctx, "Claude", []string{cb})
	if err != nil {
		e.t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(verifier))
	code, err := e.deps.Auth.Approve(ctx, u, domain.MCPAuthRequest{ClientID: c.ID, RedirectURI: cb,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]), Method: "S256"}, domain.Actor{UserID: &u.ID})
	if err != nil {
		e.t.Fatal(err)
	}
	tk, err := e.deps.Auth.Exchange(ctx, code, c.ID, cb, verifier)
	if err != nil {
		e.t.Fatal(err)
	}
	return tk.Access
}

func (e *env) session(token string) *mcp.ClientSession {
	e.t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	s, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: e.srv.URL, HTTPClient: &http.Client{Transport: bearer{token}}}, nil)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { _ = s.Close() })
	return s
}

// call runs a tool and returns its result; a protocol failure fails the test.
func call(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := s.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}

func text(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// ok runs a tool that must succeed and decodes its structured output.
func ok(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	res := call(t, s, name, args)
	if res.IsError {
		t.Fatalf("%s failed: %s", name, text(res))
	}
	var out map[string]any
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s output %s: %v", name, raw, err)
	}
	return out
}

func post(url, token string) *http.Response {
	req, _ := http.NewRequest("POST", url, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"x","version":"1"}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	_ = res.Body.Close()
	return res
}

func TestMCPRequiresToken(t *testing.T) {
	e := newEnv(t)
	res := post(e.srv.URL, "")
	if res.StatusCode != 401 || !strings.Contains(res.Header.Get("WWW-Authenticate"), `resource_metadata="https://example.com/.well-known/oauth-protected-resource/mcp"`) {
		t.Errorf("no token = %d %q", res.StatusCode, res.Header.Get("WWW-Authenticate"))
	}
	if res := post(e.srv.URL, "garbage"); res.StatusCode != 401 {
		t.Errorf("bad token = %d", res.StatusCode)
	}
	if res := post(e.srv.URL, e.access); res.StatusCode != 200 {
		t.Errorf("good token = %d", res.StatusCode)
	}
}

func TestMCPToolsList(t *testing.T) {
	e := newEnv(t)
	s := e.session(e.access)
	res, err := s.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
		if tl.Description == "" {
			t.Errorf("%s has no description", tl.Name)
		}
	}
	for _, want := range []string{"data_catalog", "data_get", "data_set_value", "data_create_table", "data_update_table_columns",
		"data_rows_upsert", "data_rows_delete", "data_set_public", "data_delete", "asset_upload"} {
		if !slices.Contains(names, want) {
			t.Errorf("tools/list lacks %s (has %v)", want, names)
		}
	}
	if init := s.InitializeResult(); init == nil || !strings.Contains(init.Instructions, "page_guide") {
		t.Errorf("instructions = %+v", init)
	}
}

func TestMCPSetValue(t *testing.T) {
	e := newEnv(t)
	s := e.session(e.access)
	out := ok(t, s, "data_set_value", map[string]any{"key": "house.address", "value_type": "text", "value": "1 Example Street",
		"description": "Postal address", "public": true})
	if out["key"] != "house.address" || out["value"] != "1 Example Street" || out["public"] != true {
		t.Errorf("created = %v", out)
	}
	var details string
	if err := e.db.QueryRow(`SELECT details FROM audit_log WHERE action = 'data.created'`).Scan(&details); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(details, `"via":"mcp"`) || !strings.Contains(details, `"client":"Claude"`) {
		t.Errorf("audit details = %s", details)
	}
	out = ok(t, s, "data_set_value", map[string]any{"key": "house.address", "value": "2 Example Street"})
	if out["value"] != "2 Example Street" || out["value_type"] != "text" || out["description"] != "Postal address" {
		t.Errorf("patched = %v", out)
	}
	res := call(t, s, "data_set_value", map[string]any{"key": "Bad Key", "value_type": "text", "value": "x"})
	if !res.IsError || !strings.Contains(text(res), "key: invalid") {
		t.Errorf("bad key = %v %q", res.IsError, text(res))
	}
	res = call(t, s, "data_set_value", map[string]any{"key": "new.one", "value": "x"})
	if !res.IsError || !strings.Contains(text(res), "value_type") {
		t.Errorf("missing type = %v %q", res.IsError, text(res))
	}
	res = call(t, s, "data_get", map[string]any{"key": "nope"})
	if !res.IsError || !strings.Contains(text(res), "not found") {
		t.Errorf("missing key = %v %q", res.IsError, text(res))
	}
}

func TestMCPTables(t *testing.T) {
	e := newEnv(t)
	s := e.session(e.access)
	ok(t, s, "data_create_table", map[string]any{"key": "contacts", "description": "Phones", "columns": []map[string]any{
		{"name": "name", "label": "Name", "type": "text"}, {"name": "phone", "label": "Phone", "type": "phone"}}})
	out := ok(t, s, "data_rows_upsert", map[string]any{"key": "contacts", "rows": []map[string]any{
		{"cells": map[string]any{"name": "Office", "phone": "+380500000001"}},
		{"cells": map[string]any{"name": "Guard", "phone": "+380500000002"}}}})
	rows, _ := out["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %v", out)
	}
	first := rows[0].(map[string]any)
	got := ok(t, s, "data_get", map[string]any{"key": "contacts", "limit": 1})
	if got["total"] != float64(2) || len(got["rows"].([]any)) != 1 || got["item"].(map[string]any)["kind"] != "table" {
		t.Errorf("data_get = %v", got)
	}
	ok(t, s, "data_rows_delete", map[string]any{"key": "contacts", "ids": []any{first["id"]}})
	ok(t, s, "data_update_table_columns", map[string]any{"key": "contacts", "columns": []map[string]any{
		{"name": "name", "label": "Name", "type": "text"}}})
	ok(t, s, "data_set_public", map[string]any{"key": "contacts", "public": true})
	cat := ok(t, s, "data_catalog", nil)
	items, _ := cat["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("catalog = %v", cat)
	}
	it := items[0].(map[string]any)
	if it["key"] != "contacts" || it["row_count"] != float64(1) || it["public"] != true || len(it["columns"].([]any)) != 1 {
		t.Errorf("catalog item = %v", it)
	}
	ok(t, s, "data_delete", map[string]any{"key": "contacts"})
	if cat := ok(t, s, "data_catalog", nil); len(cat["items"].([]any)) != 0 {
		t.Errorf("after delete = %v", cat)
	}
	res := call(t, s, "data_get", map[string]any{"key": "contacts", "limit": 500})
	if !res.IsError {
		t.Errorf("limit 500 accepted")
	}
}

func TestMCPAssetUpload(t *testing.T) {
	e := newEnv(t)
	s := e.session(e.access)
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")
	out := ok(t, s, "asset_upload", map[string]any{"filename": "a.png", "base64": base64.StdEncoding.EncodeToString(png)})
	id, _ := out["id"].(float64)
	if id == 0 || out["url"] != "/api/v1/assets/"+strconv.Itoa(int(id)) {
		t.Errorf("upload = %v", out)
	}
	res := call(t, s, "asset_upload", map[string]any{"filename": "a.png", "base64": "!!!"})
	if !res.IsError {
		t.Error("bad base64 accepted")
	}
}

func TestMCPCutOff(t *testing.T) {
	e := newEnv(t)
	other, _ := e.r.Users.GetByPhone(ctx, "+380500000098")
	if _, err := e.users.Update(ctx, e.admin.ID, domain.UserPatch{IsMCP: ptr(false)}, domain.Actor{UserID: &other.ID}); err != nil {
		t.Fatal(err)
	}
	if res := post(e.srv.URL, e.access); res.StatusCode != 401 {
		t.Errorf("MCP revoked after issue = %d", res.StatusCode)
	}
}

func TestMCPRateLimit(t *testing.T) {
	e := newEnv(t)
	key := "mcp:" + strconv.FormatInt(e.grant.ID, 10)
	for range 300 {
		e.deps.Limits.Allow(key, 300, time.Minute)
	}
	if res := post(e.srv.URL, e.access); res.StatusCode != 429 {
		t.Errorf("301st request = %d", res.StatusCode)
	}
}

func ptr[T any](v T) *T { return &v }

func TestMCPPageToolsList(t *testing.T) {
	e := newEnv(t)
	s := e.session(e.access)
	res, err := s.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
		if strings.Contains(tl.Name, "publish") || strings.Contains(tl.Name, "delete_page") || tl.Name == "page_delete" {
			t.Errorf("tool %s lets the agent publish or delete pages", tl.Name)
		}
	}
	for _, want := range []string{"page_guide", "page_list_versions", "page_get_version", "page_create_draft", "page_update_draft", "page_copy_to_draft"} {
		if !slices.Contains(names, want) {
			t.Errorf("tools/list lacks %s", want)
		}
	}
}

func TestMCPPageGuide(t *testing.T) {
	e := newEnv(t)
	s := e.session(e.access)
	g := text(call(t, s, "page_guide", nil))
	for _, want := range []string{"/api/v1/public/data", "homelander:height", "homelander:lang", "index.html", `target="_top"`, "connect-src 'self'"} {
		if !strings.Contains(g, want) {
			t.Errorf("guide lacks %q", want)
		}
	}
	rs, err := s.ListResources(ctx, nil)
	if err != nil || len(rs.Resources) != 1 || rs.Resources[0].URI != "homelander://guide/page" || rs.Resources[0].MIMEType != "text/markdown" {
		t.Fatalf("resources = %+v, %v", rs, err)
	}
	rr, err := s.ReadResource(ctx, &mcp.ReadResourceParams{URI: "homelander://guide/page"})
	if err != nil || len(rr.Contents) != 1 || rr.Contents[0].Text != g {
		t.Errorf("resource text differs from page_guide: %v", err)
	}
}

func TestMCPPageDrafts(t *testing.T) {
	e := newEnv(t)
	s := e.session(e.access)
	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n"))
	out := ok(t, s, "page_create_draft", map[string]any{"note": "first try", "files": []map[string]any{
		{"path": "index.html", "text": "<h1>Hello</h1>"}, {"path": "img/a.png", "base64": png}}})
	id := int64(out["id"].(float64))
	url, _ := out["admin_url"].(string)
	if url != pubURL+"/admin/pages/"+strconv.FormatInt(id, 10) || strings.Contains(url, "/site/preview/") {
		t.Errorf("admin_url = %q", url)
	}
	v, err := e.deps.Pages.Get(ctx, id)
	if err != nil || v.Status != domain.PageDraft || v.CreatedVia != "Claude" || len(v.Files) != 2 {
		t.Fatalf("version = %+v, %v", v, err)
	}

	res := call(t, s, "page_create_draft", map[string]any{"files": []map[string]any{{"path": "a.css", "text": "x"}}})
	if !res.IsError || !strings.Contains(text(res), "files: index_required") {
		t.Errorf("missing index = %v %q", res.IsError, text(res))
	}
	res = call(t, s, "page_create_draft", map[string]any{"files": []map[string]any{{"path": "index.html", "text": "x", "base64": png}}})
	if !res.IsError || !strings.Contains(text(res), "exactly one of text or base64") {
		t.Errorf("both text and base64 = %v %q", res.IsError, text(res))
	}
	res = call(t, s, "page_create_draft", map[string]any{"files": []map[string]any{{"path": "index.html", "text": "x"}, {"path": "../x.css", "text": "x"}}})
	if !res.IsError || !strings.Contains(text(res), "files: invalid") || !strings.Contains(text(res), "path=../x.css") {
		t.Errorf("bad path = %v %q", res.IsError, text(res))
	}

	out = ok(t, s, "page_update_draft", map[string]any{"id": id, "put": []map[string]any{{"path": "style.css", "text": "h1{}"}}, "delete": []string{"img/a.png"}})
	if out["admin_url"] != url {
		t.Errorf("update = %v", out)
	}
	got := ok(t, s, "page_get_version", map[string]any{"id": id})
	files, _ := got["files"].([]any)
	byPath := map[string]map[string]any{}
	for _, f := range files {
		m := f.(map[string]any)
		byPath[m["path"].(string)] = m
	}
	if len(files) != 2 || byPath["index.html"]["text"] != "<h1>Hello</h1>" || byPath["style.css"]["text"] != "h1{}" || got["created_via"] != "Claude" {
		t.Errorf("page_get_version = %v", got)
	}

	list := ok(t, s, "page_list_versions", nil)
	if vs, _ := list["versions"].([]any); len(vs) != 1 || vs[0].(map[string]any)["status"] != "draft" {
		t.Errorf("page_list_versions = %v", list)
	}

	adm := e.admin.ID
	if err := e.deps.Pages.Publish(ctx, id, domain.Actor{UserID: &adm}); err != nil {
		t.Fatal(err)
	}
	res = call(t, s, "page_update_draft", map[string]any{"id": id, "put": []map[string]any{{"path": "index.html", "text": "x"}}})
	if !res.IsError || !strings.Contains(text(res), "conflict") {
		t.Errorf("update published = %v %q", res.IsError, text(res))
	}
	cp := ok(t, s, "page_copy_to_draft", map[string]any{"id": id})
	cid := int64(cp["id"].(float64))
	if c, _ := e.deps.Pages.Get(ctx, cid); c.Status != domain.PageDraft || cid == id || c.CreatedVia != "Claude" {
		t.Errorf("copy = %+v", c)
	}
	many := make([]string, 201)
	for i := range many {
		many[i] = "f" + strconv.Itoa(i) + ".css"
	}
	res = call(t, s, "page_update_draft", map[string]any{"id": cid, "delete": many})
	if !res.IsError || !strings.Contains(text(res), "files: too_many") {
		t.Errorf("201 deletes = %v %q", res.IsError, text(res))
	}
}

func TestMCPSeveralPages(t *testing.T) {
	e := newEnv(t)
	s := e.session(e.access)
	out := ok(t, s, "page_create_draft", map[string]any{"files": []map[string]any{
		{"path": "index.html", "text": `<a href="/contacts" target="_top">Contacts</a>`}, {"path": "contacts.html", "text": "<h1>Contacts</h1>"}}})
	want := "[map[name:index path:/] map[name:contacts path:/contacts]]"
	if got := fmt.Sprint(out["pages"]); got != want {
		t.Errorf("create pages = %s", got)
	}
	id := out["id"]
	if got := fmt.Sprint(ok(t, s, "page_get_version", map[string]any{"id": id})["pages"]); got != want {
		t.Errorf("get pages = %s", got)
	}
	list := ok(t, s, "page_list_versions", nil)["versions"].([]any)
	if got := fmt.Sprint(list[0].(map[string]any)["pages"]); got != want {
		t.Errorf("list pages = %s", got)
	}
	upd := ok(t, s, "page_update_draft", map[string]any{"id": id, "put": []map[string]any{{"path": "rules.html", "text": "r"}}})
	if got := fmt.Sprint(upd["pages"]); !strings.Contains(got, "path:/rules") {
		t.Errorf("update pages = %s", got)
	}
	res := call(t, s, "page_create_draft", map[string]any{"files": []map[string]any{{"path": "index.html", "text": "x"}, {"path": "admin.html", "text": "x"}}})
	if !res.IsError || !strings.Contains(text(res), "reserved_page_name") || !strings.Contains(text(res), "admin.html") {
		t.Errorf("reserved = %v %q", res.IsError, text(res))
	}
	g := text(call(t, s, "page_guide", nil))
	for _, w := range []string{"Several pages", "contacts.html", "reserved", "homelander:navigate", "page_copy_to_draft", "`url`", "`phone`"} {
		if !strings.Contains(g, w) {
			t.Errorf("guide lacks %q", w)
		}
	}
}
