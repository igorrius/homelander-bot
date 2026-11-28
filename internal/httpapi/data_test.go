package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/httpapi"
)

var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func uploadFile(e *apiEnv, tok, path, field string, data []byte) resp {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile(field, "f.bin")
	_, _ = fw.Write(data)
	_ = mw.Close()
	req := httptest.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Requested-With", "homelander")
	if tok != "" {
		req.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: tok})
	}
	req.RemoteAddr = "10.8.0.2:1"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return resp{rec, e.t}
}

func fieldOf(r resp, field string) string {
	e, _ := r.json()["error"].(map[string]any)
	f, _ := e["fields"].(map[string]any)
	s, _ := f[field].(string)
	return s
}

// dataImage uploads a PNG for the data space and returns its id.
func dataImage(t *testing.T, e *apiEnv, tok string) int64 {
	t.Helper()
	r := uploadFile(e, tok, "/api/v1/admin/data/assets", "file", pngBytes)
	if r.Code != 201 {
		t.Fatalf("upload = %d %s", r.Code, r.Body)
	}
	id := int64(r.json()["id"].(float64))
	if r.json()["url"] != fmt.Sprintf("/api/v1/admin/assets/%d", id) {
		t.Fatalf("url = %v", r.json()["url"])
	}
	return id
}

func TestAssetAccess(t *testing.T) {
	e, tok := adminEnv(t)
	id := dataImage(t, e, tok)
	pub := fmt.Sprintf("/api/v1/assets/%d", id)
	if r := e.do("GET", pub, nil, "", ""); r.Code != 404 {
		t.Fatalf("guest before reference = %d", r.Code)
	}
	r := e.do("GET", fmt.Sprintf("/api/v1/admin/assets/%d", id), nil, tok, "")
	if r.Code != 200 || r.Body.String() != string(pngBytes) || r.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("admin asset = %d %q %q", r.Code, r.Body, r.Header().Get("Cache-Control"))
	}
	if r := e.do("GET", fmt.Sprintf("/api/v1/admin/assets/%d", id), nil, "", ""); r.Code != 401 {
		t.Fatalf("guest on admin route = %d", r.Code)
	}
	vt := domain.DataImage
	visPub, visRes := domain.VisPublic, domain.VisResidents
	if _, err := e.api.Data.Create(ctx, domain.DataInput{Key: "logo", Kind: domain.DataValue, ValueType: &vt,
		Value: json.RawMessage(fmt.Sprint(id)), Visibility: &visPub}, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	if r := e.do("GET", pub, nil, "", ""); r.Code != 200 || r.Body.String() != string(pngBytes) || r.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Fatalf("guest public = %d %q", r.Code, r.Header().Get("Cache-Control"))
	}
	if _, err := e.api.Data.Patch(ctx, "logo", domain.DataInput{Visibility: &visRes}, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	if r := e.do("GET", pub, nil, "", ""); r.Code != 404 {
		t.Fatalf("guest after private = %d", r.Code)
	}
}

func TestDataAssetUpload(t *testing.T) {
	e, tok := adminEnv(t)
	if r := uploadFile(e, tok, "/api/v1/admin/data/assets", "file", []byte("hello, plain text")); r.Code != 422 || fieldOf(r, "file") != "type" {
		t.Fatalf("text = %d %s", r.Code, r.Body)
	}
	big := append(append([]byte{}, pngBytes...), make([]byte, 5<<20)...)
	if r := uploadFile(e, tok, "/api/v1/admin/data/assets", "file", big); r.Code != 422 || fieldOf(r, "file") != "size" {
		t.Fatalf("oversize = %d %s", r.Code, r.Body)
	}
	if r := uploadFile(e, "", "/api/v1/admin/data/assets", "file", pngBytes); r.Code != 401 {
		t.Fatalf("guest = %d", r.Code)
	}
}

func TestAdminDataCRUD(t *testing.T) {
	e, tok := adminEnv(t)
	r := e.do("POST", "/api/v1/admin/data", map[string]any{"key": "house.address", "kind": "value", "valueType": "text",
		"value": "Main st 1", "description": "where we live"}, tok, "")
	if r.Code != 201 || r.json()["key"] != "house.address" || r.json()["value"] != "Main st 1" || r.json()["visibility"] != "admins" {
		t.Fatalf("create = %d %s", r.Code, r.Body)
	}
	var list []map[string]any
	_ = jsonUnmarshal(e.do("GET", "/api/v1/admin/data", nil, tok, "").Body.Bytes(), &list)
	if len(list) != 1 || list[0]["kind"] != "value" || list[0]["updatedBy"].(map[string]any)["name"] != "F L" {
		t.Fatalf("list = %v", list)
	}
	r = e.do("PATCH", "/api/v1/admin/data/house.address", map[string]any{"visibility": "residents", "valueType": "text", "value": "Main st 2"}, tok, "")
	if r.Code != 200 || r.json()["visibility"] != "residents" || r.json()["value"] != "Main st 2" {
		t.Fatalf("patch = %d %s", r.Code, r.Body)
	}
	if r := e.do("PATCH", "/api/v1/admin/data/house.address", map[string]any{"visibility": "everyone"}, tok, ""); r.Code != 422 || fieldOf(r, "visibility") != "invalid" {
		t.Fatalf("bad visibility = %d %s", r.Code, r.Body)
	}
	if r := e.do("PATCH", "/api/v1/admin/data/house.address", map[string]any{"public": true}, tok, ""); r.Code != 400 {
		t.Fatalf("old public field = %d %s", r.Code, r.Body)
	}
	r = e.do("GET", "/api/v1/admin/data/house.address", nil, tok, "")
	if r.Code != 200 || r.json()["item"].(map[string]any)["description"] != "where we live" {
		t.Fatalf("get = %d %s", r.Code, r.Body)
	}
	if r := e.do("DELETE", "/api/v1/admin/data/house.address", nil, tok, ""); r.Code != 204 {
		t.Fatalf("delete = %d %s", r.Code, r.Body)
	}
	if r := e.do("GET", "/api/v1/admin/data/house.address", nil, tok, ""); r.Code != 404 {
		t.Fatalf("get after delete = %d", r.Code)
	}
}

func TestAdminDataRows(t *testing.T) {
	e, tok := adminEnv(t)
	r := e.do("POST", "/api/v1/admin/data", map[string]any{"key": "contacts", "kind": "table",
		"columns": []map[string]string{{"name": "name", "label": "Name", "type": "text"}, {"name": "phone", "label": "Phone", "type": "phone"}}}, tok, "")
	if r.Code != 201 {
		t.Fatalf("create = %d %s", r.Code, r.Body)
	}
	r = e.do("PUT", "/api/v1/admin/data/contacts/rows", map[string]any{"rows": []map[string]any{
		{"position": 0, "cells": map[string]any{"name": "Office", "phone": "+380 44 000 00 00"}},
		{"position": 1, "cells": map[string]any{"name": "Guard"}},
	}}, tok, "")
	rows, _ := r.json()["rows"].([]any)
	if r.Code != 200 || len(rows) != 2 {
		t.Fatalf("put rows = %d %s", r.Code, r.Body)
	}
	id := int64(rows[0].(map[string]any)["id"].(float64))
	if r := e.do("DELETE", fmt.Sprintf("/api/v1/admin/data/contacts/rows/%d", id), nil, tok, ""); r.Code != 204 {
		t.Fatalf("delete row = %d %s", r.Code, r.Body)
	}
	r = e.do("GET", "/api/v1/admin/data/contacts", nil, tok, "")
	rows, _ = r.json()["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["cells"].(map[string]any)["name"] != "Guard" ||
		r.json()["item"].(map[string]any)["rowCount"] != float64(1) {
		t.Fatalf("get = %s", r.Body)
	}
}

func TestAdminDataValidation(t *testing.T) {
	e, tok := adminEnv(t)
	r := e.do("POST", "/api/v1/admin/data", map[string]any{"key": "Bad Key", "kind": "value", "valueType": "text"}, tok, "")
	if r.Code != 422 || fieldOf(r, "key") != "invalid" {
		t.Fatalf("bad key = %d %s", r.Code, r.Body)
	}
}

func TestAdminDataRequiresAdmin(t *testing.T) {
	e := newAPIEnv(t)
	tok := e.login(e.mkUser("+380500000001"))
	if r := e.do("GET", "/api/v1/admin/data", nil, tok, ""); r.Code != 403 {
		t.Fatalf("resident = %d", r.Code)
	}
	if r := e.do("GET", "/api/v1/admin/data", nil, "", ""); r.Code != 401 {
		t.Fatalf("guest = %d", r.Code)
	}
}

// publicFixture makes a public value, a public table with an image cell and a private value.
func publicFixture(t *testing.T) (*apiEnv, string, int64) {
	t.Helper()
	e, tok := adminEnv(t)
	img := dataImage(t, e, tok)
	for _, body := range []map[string]any{
		{"key": "house.address", "kind": "value", "valueType": "text", "value": "Main st 1", "visibility": "public"},
		{"key": "secret", "kind": "value", "valueType": "text", "value": "hidden"},
		{"key": "dues", "kind": "value", "valueType": "text", "value": "apartment 3 paid", "visibility": "residents"},
		{"key": "contacts", "kind": "table", "visibility": "public", "description": "not for the public",
			"columns": []map[string]string{{"name": "name", "label": "Name", "type": "text"}, {"name": "pic", "label": "Pic", "type": "image"}}},
	} {
		if r := e.do("POST", "/api/v1/admin/data", body, tok, ""); r.Code != 201 {
			t.Fatalf("create %v = %d %s", body["key"], r.Code, r.Body)
		}
	}
	r := e.do("PUT", "/api/v1/admin/data/contacts/rows", map[string]any{"rows": []map[string]any{
		{"position": 0, "cells": map[string]any{"name": "Office", "pic": img}}}}, tok, "")
	if r.Code != 200 {
		t.Fatalf("rows = %d %s", r.Code, r.Body)
	}
	return e, tok, img
}

func TestPublicData(t *testing.T) {
	e, _, img := publicFixture(t)
	r := e.do("GET", "/api/v1/public/data", nil, "", "")
	if r.Code != 200 {
		t.Fatalf("all = %d %s", r.Code, r.Body)
	}
	all := r.json()
	if _, ok := all["secret"]; ok || len(all) != 2 {
		t.Fatalf("all = %v", all)
	}
	addr := all["house.address"].(map[string]any)
	if addr["kind"] != "value" || addr["type"] != "text" || addr["value"] != "Main st 1" {
		t.Fatalf("address = %v", addr)
	}
	tbl := all["contacts"].(map[string]any)
	row := tbl["rows"].([]any)[0].(map[string]any)
	if tbl["kind"] != "table" || len(tbl["columns"].([]any)) != 2 || row["name"] != "Office" ||
		row["pic"] != fmt.Sprintf("/api/v1/assets/%d", img) || row["id"] == nil || tbl["description"] != nil {
		t.Fatalf("contacts = %v", tbl)
	}
	one := e.do("GET", "/api/v1/public/data/house.address", nil, "", "")
	if one.Code != 200 || one.json()["value"] != "Main st 1" {
		t.Fatalf("one = %d %s", one.Code, one.Body)
	}
	private := e.do("GET", "/api/v1/public/data/secret", nil, "", "")
	missing := e.do("GET", "/api/v1/public/data/missing", nil, "", "")
	if private.Code != 404 || private.Body.String() != missing.Body.String() {
		t.Fatalf("private = %d %s; missing = %d %s", private.Code, private.Body, missing.Code, missing.Body)
	}
}

func TestPublicDataCaching(t *testing.T) {
	e, tok, _ := publicFixture(t)
	r := e.do("GET", "/api/v1/public/data", nil, "", "")
	tag := r.Header().Get("ETag")
	if tag == "" || r.Header().Get("Cache-Control") != "public, max-age=30" {
		t.Fatalf("headers = %v", r.Header())
	}
	req := httptest.NewRequest("GET", "/api/v1/public/data", nil)
	req.Header.Set("If-None-Match", tag)
	req.RemoteAddr = "10.7.0.1:1"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != 304 || rec.Body.Len() != 0 || rec.Header().Get("ETag") != tag {
		t.Fatalf("conditional = %d %q", rec.Code, rec.Body)
	}
	if r := e.do("PATCH", "/api/v1/admin/data/house.address", map[string]any{"valueType": "text", "value": "Main st 2"}, tok, ""); r.Code != 200 {
		t.Fatalf("patch = %d", r.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/public/data", nil)
	req.Header.Set("If-None-Match", tag)
	req.RemoteAddr = "10.7.0.1:1"
	e.h.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("ETag") == tag {
		t.Fatalf("after change = %d etag %q", rec.Code, rec.Header().Get("ETag"))
	}
}

func TestPublicDataCORS(t *testing.T) {
	e, _, _ := publicFixture(t)
	for _, p := range []string{"/api/v1/public/data", "/api/v1/public/data/missing"} {
		if r := e.do("GET", p, nil, "", "10.6.0.1"); r.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("%s: no CORS header (%d)", p, r.Code)
		}
	}
	var last resp
	for range 121 {
		last = e.do("GET", "/api/v1/public/data", nil, "", "10.6.0.2")
	}
	if last.Code != 429 || last.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("429 = %d %v", last.Code, last.Header())
	}
}

func TestPublicDataRateLimit(t *testing.T) {
	e, _, _ := publicFixture(t)
	for i := range 120 {
		if r := e.do("GET", "/api/v1/public/data", nil, "", "10.5.0.1"); r.Code != 200 {
			t.Fatalf("request %d = %d", i+1, r.Code)
		}
	}
	if r := e.do("GET", "/api/v1/public/data", nil, "", "10.5.0.1"); r.Code != 429 || r.errCode() != "rate_limited" {
		t.Fatalf("121st = %d %s", r.Code, r.Body)
	}
	if r := e.do("GET", "/api/v1/public/data", nil, "", "10.5.0.2"); r.Code != 200 {
		t.Fatalf("other IP = %d", r.Code)
	}
}

func keysOf(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, " ")
}

func TestMyData(t *testing.T) {
	e, _, img := publicFixture(t)
	if r := e.do("GET", "/api/v1/me/data", nil, "", ""); r.Code != 401 {
		t.Fatalf("guest = %d", r.Code)
	}
	resident := e.login(e.mkUser("+380500000001"))
	r := e.do("GET", "/api/v1/me/data", nil, resident, "")
	if r.Code != 200 || keysOf(r.json()) != "contacts dues house.address" {
		t.Fatalf("resident = %d %s", r.Code, r.Body)
	}
	if r.Header().Get("Cache-Control") != "private, no-store" || r.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("headers = %v", r.Header())
	}
	if v := r.json()["dues"].(map[string]any); v["kind"] != "value" || v["value"] != "apartment 3 paid" {
		t.Fatalf("dues = %v", v)
	}
	// Public items keep plain asset URLs.
	row := r.json()["contacts"].(map[string]any)["rows"].([]any)[0].(map[string]any)
	if row["pic"] != fmt.Sprintf("/api/v1/assets/%d", img) {
		t.Fatalf("public image = %v", row["pic"])
	}
	adminOnly := e.login(e.mkUser("+380500000002", admin, func(u *domain.User) { u.IsResident = false }))
	if r := e.do("GET", "/api/v1/me/data", nil, adminOnly, ""); r.Code != 200 || keysOf(r.json()) != "contacts dues house.address" {
		t.Fatalf("admin = %d %s", r.Code, r.Body)
	}
	neither := e.login(e.mkUser("+380500000003", func(u *domain.User) { u.IsResident = false }))
	if r := e.do("GET", "/api/v1/me/data", nil, neither, ""); r.Code != 200 || keysOf(r.json()) != "contacts house.address" {
		t.Fatalf("not a resident = %d %s", r.Code, r.Body)
	}
}

func TestSignedAssetURLs(t *testing.T) {
	e, tok := adminEnv(t)
	img, other := dataImage(t, e, tok), dataImage(t, e, tok)
	if r := e.do("POST", "/api/v1/admin/data", map[string]any{"key": "house.photo", "kind": "value", "valueType": "image",
		"value": img, "visibility": "residents"}, tok, ""); r.Code != 201 {
		t.Fatalf("create = %d %s", r.Code, r.Body)
	}
	plain := fmt.Sprintf("/api/v1/assets/%d", img)
	if r := e.do("GET", plain, nil, "", ""); r.Code != 404 {
		t.Fatalf("residents image without signature = %d", r.Code)
	}
	r := e.do("GET", "/api/v1/me/data", nil, tok, "")
	signed, _ := r.json()["house.photo"].(map[string]any)["value"].(string)
	if !strings.HasPrefix(signed, plain+"?exp=") || !strings.Contains(signed, "&sig=") {
		t.Fatalf("signed url = %q", signed)
	}
	r = e.do("GET", signed, nil, "", "")
	if r.Code != 200 || r.Body.String() != string(pngBytes) || r.Header().Get("Cache-Control") != "private, max-age=600" {
		t.Fatalf("signed = %d %q", r.Code, r.Header().Get("Cache-Control"))
	}
	query := signed[len(plain):]
	if r := e.do("GET", fmt.Sprintf("/api/v1/assets/%d", other)+query, nil, "", ""); r.Code != 404 {
		t.Fatalf("signature of another asset = %d", r.Code)
	}
	if r := e.do("GET", strings.Replace(signed, "&sig=", "&sig=x", 1), nil, "", ""); r.Code != 404 {
		t.Fatalf("tampered = %d", r.Code)
	}
	e.now = e.now.Add(2 * time.Hour)
	if r := e.do("GET", signed, nil, "", ""); r.Code != 404 {
		t.Fatalf("expired = %d", r.Code)
	}
}
