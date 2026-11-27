package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

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
	vt, yes, no := domain.DataImage, true, false
	if _, err := e.api.Data.Create(ctx, domain.DataInput{Key: "logo", Kind: domain.DataValue, ValueType: &vt,
		Value: json.RawMessage(fmt.Sprint(id)), Public: &yes}, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	if r := e.do("GET", pub, nil, "", ""); r.Code != 200 || r.Body.String() != string(pngBytes) {
		t.Fatalf("guest public = %d", r.Code)
	}
	if _, err := e.api.Data.Patch(ctx, "logo", domain.DataInput{Public: &no}, domain.Actor{}); err != nil {
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
