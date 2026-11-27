package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/igorrius/homelander-bot/internal/domain"
)

const (
	oauthLimit  = 30
	oauthWindow = time.Minute
	maxFormBody = 16 << 10
)

// MCPPath is where the MCP endpoint is served.
const MCPPath = "/mcp"

// RegisterOAuth adds Homelander's OAuth 2.1 authorization server for MCP
// agents: discovery metadata, dynamic client registration, the token and
// revocation endpoints (outside /api/, as OAuth clients call them), and the
// two API routes the consent page uses. GET /oauth/authorize is a page of the
// web app.
func RegisterOAuth(s *Server, api API) {
	h := handlerCtx{s, api}
	base := strings.TrimRight(api.Cfg.PublicURL.String(), "/")
	prm := auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource: base + MCPPath, AuthorizationServers: []string{base}, BearerMethodsSupported: []string{"header"},
		ResourceName: "Homelander"})
	s.Mount("/.well-known/oauth-protected-resource", prm)
	s.Mount("/.well-known/oauth-protected-resource"+MCPPath, prm)
	s.Mount("/.well-known/oauth-authorization-server", cors(http.MethodGet, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]any{
			"issuer":                                     base,
			"authorization_endpoint":                     base + "/oauth/authorize",
			"token_endpoint":                             base + "/oauth/token",
			"registration_endpoint":                      base + "/oauth/register",
			"revocation_endpoint":                        base + "/oauth/revoke",
			"response_types_supported":                   []string{"code"},
			"grant_types_supported":                      []string{"authorization_code", "refresh_token"},
			"code_challenge_methods_supported":           []string{"S256"},
			"token_endpoint_auth_methods_supported":      []string{"none"},
			"revocation_endpoint_auth_methods_supported": []string{"none"},
			// RFC 9207: every authorization response carries iss; ChatGPT then
			// uses its stable redirect URI.
			"authorization_response_iss_parameter_supported": true,
		})
	})))
	s.Mount("/oauth/register", cors(http.MethodPost, h.limited(http.HandlerFunc(h.oauthRegister))))
	s.Mount("/oauth/token", cors(http.MethodPost, h.limited(http.HandlerFunc(h.oauthToken))))
	s.Mount("/oauth/revoke", cors(http.MethodPost, h.limited(http.HandlerFunc(h.oauthRevoke))))

	m := s.API()
	m.HandleFunc("GET /api/v1/oauth/authorize-info", h.authorizeInfo)
	m.Handle("POST /api/v1/oauth/authorize", RequireUser(http.HandlerFunc(h.authorize)))
}

// cors answers preflight requests and lets any origin call an OAuth endpoint:
// they take no cookies, only what the client sends.
func cors(method string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("Access-Control-Allow-Origin", "*")
		hd.Set("Access-Control-Allow-Methods", method+", OPTIONS")
		hd.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Mcp-Protocol-Version")
		switch r.Method {
		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)
		case method:
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Allow", method+", OPTIONS")
			oauthError(w, http.StatusMethodNotAllowed, "invalid_request")
		}
	})
}

func (h handlerCtx) limited(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.api.Limits != nil && !h.api.Limits.Allow("oauth:"+ClientIP(r, h.api.Cfg.TrustProxy), oauthLimit, oauthWindow) {
			w.Header().Set("Retry-After", "60")
			oauthError(w, http.StatusTooManyRequests, "rate_limited")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// oauthError writes an RFC 6749 error body.
func oauthError(w http.ResponseWriter, status int, code string) {
	WriteJSON(w, status, map[string]string{"error": code})
}

func (h handlerCtx) oauthRegister(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClientName              string   `json:"client_name"`
		RedirectURIs            []string `json:"redirect_uris"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
		GrantTypes              []string `json:"grant_types"`
		ResponseTypes           []string `json:"response_types"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxJSONBody)).Decode(&in); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata")
		return
	}
	if in.TokenEndpointAuthMethod != "" && in.TokenEndpointAuthMethod != "none" {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata")
		return
	}
	for _, g := range in.GrantTypes {
		if g != "authorization_code" && g != "refresh_token" {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata")
			return
		}
	}
	c, err := h.api.MCPAuth.Register(r.Context(), in.ClientName, in.RedirectURIs)
	switch {
	case errors.Is(err, domain.ErrInvalidRedirect):
		oauthError(w, http.StatusBadRequest, "invalid_redirect_uri")
		return
	case err != nil:
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{
		"client_id": c.ID, "client_name": c.Name, "redirect_uris": c.RedirectURIs, "client_id_issued_at": c.CreatedAt.Unix(),
		"token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code", "refresh_token"},
		"response_types": []string{"code"},
	})
}

func (h handlerCtx) oauthToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBody)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	f := r.PostForm
	var (
		t   domain.MCPTokens
		err error
	)
	switch f.Get("grant_type") {
	case "authorization_code":
		if f.Get("code") == "" || f.Get("client_id") == "" || f.Get("redirect_uri") == "" || f.Get("code_verifier") == "" {
			oauthError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		t, err = h.api.MCPAuth.Exchange(r.Context(), f.Get("code"), f.Get("client_id"), f.Get("redirect_uri"), f.Get("code_verifier"))
	case "refresh_token":
		if f.Get("refresh_token") == "" || f.Get("client_id") == "" {
			oauthError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		t, err = h.api.MCPAuth.Refresh(r.Context(), f.Get("refresh_token"), f.Get("client_id"))
	case "":
		oauthError(w, http.StatusBadRequest, "invalid_request")
		return
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type")
		return
	}
	switch {
	case errors.Is(err, domain.ErrInvalidGrant):
		oauthError(w, http.StatusBadRequest, "invalid_grant")
		return
	case err != nil:
		h.fail(w, err)
		return
	}
	w.Header().Set("Pragma", "no-cache")
	WriteJSON(w, http.StatusOK, map[string]any{"access_token": t.Access, "token_type": "Bearer", "expires_in": t.ExpiresIn,
		"refresh_token": t.Refresh})
}

func (h handlerCtx) oauthRevoke(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBody)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if tok := r.PostForm.Get("token"); tok != "" {
		if err := h.api.MCPAuth.RevokeToken(r.Context(), tok); err != nil {
			h.fail(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// mcpReason tells why u may not approve an agent ("" when allowed).
func mcpReason(u domain.User) string {
	switch {
	case u.Status != domain.StatusActive || !u.IsAdmin:
		return "not_admin"
	case !u.IsMCP:
		return "mcp_off"
	}
	return ""
}

func (h handlerCtx) authorizeInfo(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	c, err := h.api.MCPAuth.CheckRequest(r.Context(), domain.MCPAuthRequest{ClientID: q.Get("client_id"), RedirectURI: q.Get("redirect_uri")})
	if h.oauthRequestFailed(w, err) {
		return
	}
	ru, _ := url.Parse(q.Get("redirect_uri"))
	out := map[string]any{"clientName": c.Name, "redirectHost": ru.Host, "signedIn": false, "allowed": false}
	if u, ok := CurrentUser(r); ok {
		reason := mcpReason(u)
		out["signedIn"], out["allowed"] = true, reason == ""
		if reason != "" {
			out["reason"] = reason
		}
	}
	WriteJSON(w, http.StatusOK, out)
}

// oauthRequestFailed answers 400 invalid_request for a request that does not
// fit the registered client, and reports whether it answered.
func (h handlerCtx) oauthRequestFailed(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, domain.ErrInvalidRequest) || errors.Is(err, domain.ErrInvalidRedirect):
		WriteError(w, http.StatusBadRequest, "invalid_request")
	default:
		h.fail(w, err)
	}
	return true
}

func (h handlerCtx) authorize(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClientID            string `json:"clientId"`
		RedirectURI         string `json:"redirectUri"`
		CodeChallenge       string `json:"codeChallenge"`
		CodeChallengeMethod string `json:"codeChallengeMethod"`
		State               string `json:"state"`
		Resource            string `json:"resource"`
		Approve             bool   `json:"approve"`
	}
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	req := domain.MCPAuthRequest{ClientID: in.ClientID, RedirectURI: in.RedirectURI, Challenge: in.CodeChallenge,
		Method: in.CodeChallengeMethod, State: in.State, Resource: in.Resource}
	// The redirect URI must belong to the client even for a denial: we never
	// send the browser anywhere the client did not register.
	if _, err := h.api.MCPAuth.CheckRequest(r.Context(), req); h.oauthRequestFailed(w, err) {
		return
	}
	// RFC 9207: the issuer, exactly as in the authorization-server metadata.
	params := url.Values{"iss": {strings.TrimRight(h.api.Cfg.PublicURL.String(), "/")}}
	if in.Approve {
		u, _ := CurrentUser(r)
		code, err := h.api.MCPAuth.Approve(r.Context(), u, req, h.actor(r))
		if errors.Is(err, domain.ErrForbidden) {
			WriteError(w, http.StatusForbidden, "forbidden")
			return
		}
		if h.oauthRequestFailed(w, err) {
			return
		}
		params.Set("code", code)
	} else {
		params.Set("error", "access_denied")
	}
	if in.State != "" {
		params.Set("state", in.State)
	}
	WriteJSON(w, http.StatusOK, map[string]string{"redirect": withQuery(in.RedirectURI, params)})
}

// withQuery appends params to a URI that may already have a query.
func withQuery(uri string, params url.Values) string {
	sep := "?"
	if strings.Contains(uri, "?") {
		sep = "&"
	}
	return uri + sep + params.Encode()
}
