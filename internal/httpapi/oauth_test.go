package httpapi_test

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

const (
	claudeCB      = "https://claude.ai/api/mcp/auth_callback"
	oauthVerifier = "a-verifier-that-is-long-enough-for-pkce-0123456789"
)

func pkceChallenge(v string) string {
	s := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(s[:])
}

// form posts an application/x-www-form-urlencoded body without the CSRF header.
func (e *apiEnv) form(path string, v url.Values) resp {
	e.t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	e.ipSeq++
	req.RemoteAddr = "10.8.0." + strconv.Itoa(e.ipSeq%250) + ":1"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return resp{rec, e.t}
}

// raw sends a request without the CSRF header (as an OAuth client does).
func (e *apiEnv) raw(method, path, body string) resp {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	e.ipSeq++
	req.RemoteAddr = "10.7.0." + strconv.Itoa(e.ipSeq%250) + ":1"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return resp{rec, e.t}
}

func (e *apiEnv) registerClient(redirect string) string {
	e.t.Helper()
	r := e.raw("POST", "/oauth/register", `{"client_name":"Claude","redirect_uris":["`+redirect+`"],"token_endpoint_auth_method":"none"}`)
	if r.Code != 201 {
		e.t.Fatalf("register = %d %s", r.Code, r.Body)
	}
	return r.json()["client_id"].(string)
}

func mcpAdmin(u *domain.User) { u.IsAdmin, u.IsMCP = true, true }

func TestOAuthMetadata(t *testing.T) {
	e := newAPIEnv(t)
	for _, p := range []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp"} {
		r := e.raw("GET", p, "")
		j := r.json()
		if r.Code != 200 || j["resource"] != "https://example.com/mcp" || r.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("%s = %d %s", p, r.Code, r.Body)
		}
		if as, _ := j["authorization_servers"].([]any); len(as) != 1 || as[0] != "https://example.com" {
			t.Errorf("%s authorization_servers = %v", p, j["authorization_servers"])
		}
	}
	r := e.raw("GET", "/.well-known/oauth-authorization-server", "")
	j := r.json()
	want := map[string]string{"issuer": "https://example.com", "authorization_endpoint": "https://example.com/oauth/authorize",
		"token_endpoint": "https://example.com/oauth/token", "registration_endpoint": "https://example.com/oauth/register",
		"revocation_endpoint": "https://example.com/oauth/revoke"}
	for k, v := range want {
		if j[k] != v {
			t.Errorf("%s = %v", k, j[k])
		}
	}
	if m, _ := j["code_challenge_methods_supported"].([]any); len(m) != 1 || m[0] != "S256" {
		t.Errorf("code_challenge_methods_supported = %v", j["code_challenge_methods_supported"])
	}
	if m, _ := j["token_endpoint_auth_methods_supported"].([]any); len(m) != 1 || m[0] != "none" {
		t.Errorf("token_endpoint_auth_methods_supported = %v", j["token_endpoint_auth_methods_supported"])
	}
	if r.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("no CORS on AS metadata")
	}
	if r := e.raw("OPTIONS", "/oauth/token", ""); r.Code != 204 || r.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("preflight = %d", r.Code)
	}
}

func TestOAuthRegister(t *testing.T) {
	e := newAPIEnv(t)
	for _, ok := range []string{claudeCB, "https://claude.com/api/mcp/auth_callback", "http://localhost:3118/callback", "http://127.0.0.1:9/cb"} {
		e.registerClient(ok)
	}
	for _, bad := range []string{"https://claude.ai.evil.com/api/mcp/auth_callback", "https://claude.ai/api/mcp/auth_callback/../x",
		"http://localhost.evil.com:8080/cb", "https://localhost:8080/cb", "https://evil.example/cb"} {
		r := e.raw("POST", "/oauth/register", `{"client_name":"Claude","redirect_uris":["`+bad+`"]}`)
		if r.Code != 400 || r.json()["error"] != "invalid_redirect_uri" {
			t.Errorf("register %s = %d %s", bad, r.Code, r.Body)
		}
	}
	r := e.raw("POST", "/oauth/register", `{"client_name":"C","redirect_uris":["`+claudeCB+`"],"token_endpoint_auth_method":"client_secret_basic"}`)
	if r.Code != 400 || r.json()["error"] != "invalid_client_metadata" {
		t.Errorf("confidential client = %d %s", r.Code, r.Body)
	}
	if r := e.raw("POST", "/oauth/register", `nonsense`); r.Code != 400 || r.json()["error"] != "invalid_client_metadata" {
		t.Errorf("bad json = %d %s", r.Code, r.Body)
	}
	r = e.raw("POST", "/oauth/register", `{"client_name":"Claude","redirect_uris":["`+claudeCB+`"]}`)
	j := r.json()
	if j["token_endpoint_auth_method"] != "none" || j["client_name"] != "Claude" || r.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("register body = %s", r.Body)
	}
}

func authorizeBody(clientID string, approve bool) map[string]any {
	return map[string]any{"clientId": clientID, "redirectUri": claudeCB, "codeChallenge": pkceChallenge(oauthVerifier),
		"codeChallengeMethod": "S256", "state": "xyz", "resource": "https://example.com/mcp", "approve": approve}
}

func TestOAuthFullFlow(t *testing.T) {
	e := newAPIEnv(t)
	adm := e.mkUser("+380500000099", mcpAdmin)
	clientID := e.registerClient(claudeCB)
	info := "/api/v1/oauth/authorize-info?client_id=" + clientID + "&redirect_uri=" + url.QueryEscape(claudeCB)
	r := e.do("GET", info, nil, "", "")
	if j := r.json(); r.Code != 200 || j["clientName"] != "Claude" || j["redirectHost"] != "claude.ai" || j["signedIn"] != false || j["allowed"] != false {
		t.Fatalf("guest info = %d %s", r.Code, r.Body)
	}
	tok := e.login(adm)
	r = e.do("GET", info, nil, tok, "")
	if j := r.json(); j["signedIn"] != true || j["allowed"] != true {
		t.Fatalf("admin info = %s", r.Body)
	}
	if r := e.do("GET", "/api/v1/oauth/authorize-info?client_id=nope&redirect_uri="+url.QueryEscape(claudeCB), nil, tok, ""); r.Code != 400 || r.errCode() != "invalid_request" {
		t.Errorf("unknown client info = %d %s", r.Code, r.Body)
	}
	if r := e.do("GET", "/api/v1/oauth/authorize-info?client_id="+clientID+"&redirect_uri="+url.QueryEscape("https://claude.com/api/mcp/auth_callback"), nil, tok, ""); r.Code != 400 {
		t.Errorf("unregistered redirect info = %d %s", r.Code, r.Body)
	}

	r = e.do("POST", "/api/v1/oauth/authorize", authorizeBody(clientID, false), tok, "")
	if red, _ := r.json()["redirect"].(string); r.Code != 200 || red != claudeCB+"?error=access_denied&state=xyz" {
		t.Errorf("deny = %d %s", r.Code, r.Body)
	}
	r = e.do("POST", "/api/v1/oauth/authorize", authorizeBody(clientID, true), tok, "")
	red, _ := r.json()["redirect"].(string)
	ru, err := url.Parse(red)
	if r.Code != 200 || err != nil || !strings.HasPrefix(red, claudeCB+"?") || ru.Query().Get("state") != "xyz" || ru.Query().Get("code") == "" {
		t.Fatalf("approve = %d %s", r.Code, r.Body)
	}
	code := ru.Query().Get("code")

	tr := e.form("/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {claudeCB},
		"client_id": {clientID}, "code_verifier": {oauthVerifier}})
	j := tr.json()
	if tr.Code != 200 || j["token_type"] != "Bearer" || j["expires_in"] != float64(3600) || j["access_token"] == nil || j["refresh_token"] == nil ||
		tr.Header().Get("Cache-Control") != "no-store" || tr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("token = %d %s", tr.Code, tr.Body)
	}
	if p, err := e.api.MCPAuth.Verify(ctx, j["access_token"].(string)); err != nil || p.User.ID != adm.ID {
		t.Fatalf("Verify = %v", err)
	}
	replay := e.form("/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {claudeCB},
		"client_id": {clientID}, "code_verifier": {oauthVerifier}})
	if replay.Code != 400 || replay.json()["error"] != "invalid_grant" {
		t.Errorf("replay = %d %s", replay.Code, replay.Body)
	}
	rf := e.form("/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {j["refresh_token"].(string)}, "client_id": {clientID}})
	if rf.Code != 200 || rf.json()["refresh_token"] == j["refresh_token"] {
		t.Fatalf("refresh = %d %s", rf.Code, rf.Body)
	}
	if r := e.form("/oauth/token", url.Values{"grant_type": {"password"}}); r.Code != 400 || r.json()["error"] != "unsupported_grant_type" {
		t.Errorf("password grant = %d %s", r.Code, r.Body)
	}
	if r := e.form("/oauth/token", url.Values{"grant_type": {"authorization_code"}}); r.Code != 400 || r.json()["error"] != "invalid_request" {
		t.Errorf("missing params = %d %s", r.Code, r.Body)
	}
	newAccess := rf.json()["access_token"].(string)
	if r := e.form("/oauth/revoke", url.Values{"token": {newAccess}}); r.Code != 200 {
		t.Errorf("revoke = %d", r.Code)
	}
	if _, err := e.api.MCPAuth.Verify(ctx, newAccess); err == nil {
		t.Error("revoked access token still works")
	}
	if r := e.form("/oauth/revoke", url.Values{"token": {"unknown"}}); r.Code != 200 {
		t.Errorf("revoke unknown = %d", r.Code)
	}
}

func TestOAuthAuthorizeForbidden(t *testing.T) {
	e := newAPIEnv(t)
	clientID := e.registerClient(claudeCB)
	if r := e.do("POST", "/api/v1/oauth/authorize", authorizeBody(clientID, true), "", ""); r.Code != 401 {
		t.Errorf("guest = %d %s", r.Code, r.Body)
	}
	noMCP := e.mkUser("+380500000098", admin)
	tok := e.login(noMCP)
	info := "/api/v1/oauth/authorize-info?client_id=" + clientID + "&redirect_uri=" + url.QueryEscape(claudeCB)
	if j := e.do("GET", info, nil, tok, "").json(); j["allowed"] != false || j["reason"] != "mcp_off" {
		t.Errorf("admin without MCP info = %v", j)
	}
	if r := e.do("POST", "/api/v1/oauth/authorize", authorizeBody(clientID, true), tok, ""); r.Code != 403 {
		t.Errorf("admin without MCP = %d %s", r.Code, r.Body)
	}
	resident := e.mkUser("+380500000097")
	rtok := e.login(resident)
	if j := e.do("GET", info, nil, rtok, "").json(); j["allowed"] != false || j["reason"] != "not_admin" {
		t.Errorf("resident info = %v", j)
	}
	if r := e.do("POST", "/api/v1/oauth/authorize", authorizeBody(clientID, true), rtok, ""); r.Code != 403 {
		t.Errorf("resident = %d %s", r.Code, r.Body)
	}
	// Even a denial must not redirect to a URI the client did not register.
	body := authorizeBody(clientID, false)
	body["redirectUri"] = "https://evil.example/cb"
	if r := e.do("POST", "/api/v1/oauth/authorize", body, rtok, ""); r.Code != 400 {
		t.Errorf("deny to foreign redirect = %d %s", r.Code, r.Body)
	}
}

func TestOAuthRateLimit(t *testing.T) {
	e := newAPIEnv(t)
	var last int
	for range 31 {
		req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader("grant_type=password"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "10.6.0.1:1"
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("31st token request = %d", last)
	}
}
