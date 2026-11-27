package domain_test

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

const claudeCB = "https://claude.ai/api/mcp/auth_callback"

var defaultPolicy = domain.RedirectPolicy{claudeCB, "https://claude.com/api/mcp/auth_callback", "http://localhost:*", "http://127.0.0.1:*"}

func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

const verifier = "a-verifier-that-is-long-enough-for-pkce-0123456789"

type mcpEnv struct {
	*env
	auth   *domain.McpAuthService
	admin  domain.User
	client domain.MCPClient
}

func newMCPEnv(t *testing.T) *mcpEnv {
	t.Helper()
	e := newEnv(t)
	m := &mcpEnv{env: e, auth: &domain.McpAuthService{R: e.r, Now: e.clock.Now, Allowed: defaultPolicy, Resource: "https://home.example/mcp"}}
	m.admin = e.mkUser("+380500000099", admin, func(u *domain.User) { u.IsMCP = true })
	e.mkUser("+380500000098", admin) // a second admin so the first can lose Admin
	c, err := m.auth.Register(ctx, "Claude", []string{claudeCB})
	if err != nil {
		t.Fatal(err)
	}
	m.client = c
	return m
}

func (m *mcpEnv) req() domain.MCPAuthRequest {
	return domain.MCPAuthRequest{ClientID: m.client.ID, RedirectURI: claudeCB, Challenge: challenge(verifier), Method: "S256",
		State: "st", Resource: "https://home.example/mcp"}
}

func (m *mcpEnv) code() string {
	m.t.Helper()
	code, err := m.auth.Approve(ctx, m.admin, m.req(), actor(m.admin))
	if err != nil {
		m.t.Fatal(err)
	}
	return code
}

func (m *mcpEnv) tokens() domain.MCPTokens {
	m.t.Helper()
	tk, err := m.auth.Exchange(ctx, m.code(), m.client.ID, claudeCB, verifier)
	if err != nil {
		m.t.Fatal(err)
	}
	return tk
}

func TestRedirectPolicy(t *testing.T) {
	for uri, want := range map[string]bool{
		claudeCB: true,
		"https://claude.com/api/mcp/auth_callback":         true,
		"http://localhost:8080/callback":                   true,
		"http://localhost/callback":                        true,
		"http://127.0.0.1:33418/cb":                        true,
		"https://claude.ai.evil.com/api/mcp/auth_callback": false,
		"https://claude.ai/api/mcp/auth_callback/../x":     false,
		"https://claude.ai/api/mcp/auth_callback?x=1":      false,
		"http://localhost.evil.com:8080/cb":                false,
		"https://localhost:8080/cb":                        false,
		"http://user@localhost:8080/cb":                    false,
		"http://localhost:8080/cb#frag":                    false,
		"http://127.0.0.2:8080/cb":                         false,
		"javascript:alert(1)":                              false,
		"":                                                 false,
	} {
		if got := defaultPolicy.Allows(uri); got != want {
			t.Errorf("Allows(%q) = %v, want %v", uri, got, want)
		}
	}
	if (domain.RedirectPolicy{"https://evil.example:*"}).Allows("https://evil.example:9/cb") {
		t.Error("a wildcard port is allowed only for loopback hosts")
	}
}

func TestMCPRegister(t *testing.T) {
	m := newMCPEnv(t)
	if _, err := m.auth.Register(ctx, "Evil", []string{"https://evil.example/cb"}); !errors.Is(err, domain.ErrInvalidRedirect) {
		t.Errorf("foreign redirect = %v", err)
	}
	if _, err := m.auth.Register(ctx, "None", nil); !errors.Is(err, domain.ErrInvalidRedirect) {
		t.Errorf("no redirect = %v", err)
	}
	got, err := m.auth.Client(ctx, m.client.ID)
	if err != nil || got.Name != "Claude" || m.client.ID == "" {
		t.Errorf("Client = %+v, %v", got, err)
	}
}

func TestMCPAuthFlow(t *testing.T) {
	m := newMCPEnv(t)
	tk := m.tokens()
	if tk.Access == "" || tk.Refresh == "" || tk.ExpiresIn != 3600 {
		t.Fatalf("tokens = %+v", tk)
	}
	if m.auditCount(domain.ActMCPClientAuthorized) != 1 {
		t.Error("no mcp.client_authorized audit")
	}
	p, err := m.auth.Verify(ctx, tk.Access)
	if err != nil || p.User.ID != m.admin.ID || p.Grant.ClientName != "Claude" || p.Grant.ClientID != m.client.ID {
		t.Fatalf("Verify = %+v, %v", p, err)
	}
	if _, err := m.auth.Verify(ctx, tk.Refresh); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("refresh token used as access = %v", err)
	}
	next, err := m.auth.Refresh(ctx, tk.Refresh, m.client.ID)
	if err != nil || next.Refresh == tk.Refresh || next.Access == tk.Access {
		t.Fatalf("Refresh = %+v, %v", next, err)
	}
	if _, err := m.auth.Refresh(ctx, tk.Refresh, m.client.ID); !errors.Is(err, domain.ErrInvalidGrant) {
		t.Errorf("old refresh = %v", err)
	}
	if _, err := m.auth.Verify(ctx, tk.Access); err != nil {
		t.Errorf("old access before expiry = %v", err)
	}
	if _, err := m.auth.Refresh(ctx, next.Refresh, "other-client"); !errors.Is(err, domain.ErrInvalidGrant) {
		t.Errorf("refresh by another client = %v", err)
	}
	// A second approval reuses the grant and writes no new audit entry.
	m.tokens()
	if gs, _ := m.auth.Grants(ctx, m.admin.ID); len(gs) != 1 || m.auditCount(domain.ActMCPClientAuthorized) != 1 {
		t.Errorf("grants = %v, audits = %d", gs, m.auditCount(domain.ActMCPClientAuthorized))
	}
	m.clock.Advance(31 * 24 * time.Hour)
	if _, err := m.auth.Refresh(ctx, next.Refresh, m.client.ID); !errors.Is(err, domain.ErrInvalidGrant) {
		t.Errorf("refresh after 30 days = %v", err)
	}
}

func TestMCPCodeRules(t *testing.T) {
	m := newMCPEnv(t)
	code := m.code()
	if _, err := m.auth.Exchange(ctx, code, m.client.ID, claudeCB, "wrong-verifier-wrong-verifier-wrong-verifier-00"); !errors.Is(err, domain.ErrInvalidGrant) {
		t.Errorf("wrong verifier = %v", err)
	}
	if _, err := m.auth.Exchange(ctx, code, m.client.ID, claudeCB, verifier); !errors.Is(err, domain.ErrInvalidGrant) {
		t.Errorf("code reused after a failed exchange = %v", err)
	}
	good := m.tokens()
	code = m.code()
	if _, err := m.auth.Exchange(ctx, code, m.client.ID, "https://claude.com/api/mcp/auth_callback", verifier); !errors.Is(err, domain.ErrInvalidGrant) {
		t.Errorf("other redirect = %v", err)
	}
	code = m.code()
	if _, err := m.auth.Exchange(ctx, code, "other", claudeCB, verifier); !errors.Is(err, domain.ErrInvalidGrant) {
		t.Errorf("other client = %v", err)
	}
	code = m.code()
	if _, err := m.auth.Exchange(ctx, code, m.client.ID, claudeCB, verifier); err != nil {
		t.Fatal(err)
	}
	if _, err := m.auth.Exchange(ctx, code, m.client.ID, claudeCB, verifier); !errors.Is(err, domain.ErrInvalidGrant) {
		t.Errorf("replayed code = %v", err)
	}
	if _, err := m.auth.Verify(ctx, good.Access); err != nil {
		t.Errorf("a replay revoked other tokens: %v", err)
	}
	code = m.code()
	m.clock.Advance(61 * time.Second)
	if _, err := m.auth.Exchange(ctx, code, m.client.ID, claudeCB, verifier); !errors.Is(err, domain.ErrInvalidGrant) {
		t.Errorf("expired code = %v", err)
	}
	if _, err := m.auth.Exchange(ctx, "nonsense", m.client.ID, claudeCB, verifier); !errors.Is(err, domain.ErrInvalidGrant) {
		t.Errorf("unknown code = %v", err)
	}
}

func TestMCPApproveRequest(t *testing.T) {
	m := newMCPEnv(t)
	bad := m.req()
	bad.Method = "plain"
	if _, err := m.auth.Approve(ctx, m.admin, bad, actor(m.admin)); !errors.Is(err, domain.ErrInvalidRequest) {
		t.Errorf("plain PKCE = %v", err)
	}
	bad = m.req()
	bad.Challenge = ""
	if _, err := m.auth.Approve(ctx, m.admin, bad, actor(m.admin)); !errors.Is(err, domain.ErrInvalidRequest) {
		t.Errorf("no challenge = %v", err)
	}
	bad = m.req()
	bad.RedirectURI = "https://claude.com/api/mcp/auth_callback" // allowed by policy, not registered by this client
	if _, err := m.auth.Approve(ctx, m.admin, bad, actor(m.admin)); !errors.Is(err, domain.ErrInvalidRedirect) {
		t.Errorf("unregistered redirect = %v", err)
	}
	bad = m.req()
	bad.Resource = "https://elsewhere.example/mcp"
	if _, err := m.auth.Approve(ctx, m.admin, bad, actor(m.admin)); !errors.Is(err, domain.ErrInvalidRequest) {
		t.Errorf("foreign resource = %v", err)
	}
	bad = m.req()
	bad.ClientID = "nope"
	if _, err := m.auth.Approve(ctx, m.admin, bad, actor(m.admin)); !errors.Is(err, domain.ErrInvalidRequest) {
		t.Errorf("unknown client = %v", err)
	}
	ok := m.req()
	ok.Resource = ""
	if _, err := m.auth.Approve(ctx, m.admin, ok, actor(m.admin)); err != nil {
		t.Errorf("no resource = %v", err)
	}
}

func TestMCPApproveRequiresFlag(t *testing.T) {
	m := newMCPEnv(t)
	resident := m.mkUser("+380500000001")
	noMCP := m.mkUser("+380500000002", admin)
	for _, u := range []domain.User{resident, noMCP} {
		if _, err := m.auth.Approve(ctx, u, m.req(), actor(u)); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("approve by %s = %v", u.Phone, err)
		}
	}
}

func TestMCPVerifyChecksUser(t *testing.T) {
	m := newMCPEnv(t)
	other := m.mkUser("+380500000097", admin)
	steps := []struct {
		name string
		cut  func() error
		back func() error
	}{
		{"admin revoked", func() error {
			_, err := m.users.Update(ctx, m.admin.ID, domain.UserPatch{IsAdmin: ptr(false)}, actor(other))
			return err
		}, func() error {
			_, err := m.users.Update(ctx, m.admin.ID, domain.UserPatch{IsAdmin: ptr(true)}, actor(other))
			return err
		}},
		{"mcp revoked", func() error {
			_, err := m.users.Update(ctx, m.admin.ID, domain.UserPatch{IsMCP: ptr(false)}, actor(other))
			return err
		}, func() error {
			_, err := m.users.Update(ctx, m.admin.ID, domain.UserPatch{IsMCP: ptr(true)}, actor(other))
			return err
		}},
		{"disabled", func() error { return m.users.Disable(ctx, m.admin.ID, actor(other)) },
			func() error { m.link(m.admin, "777"); return m.users.Enable(ctx, m.admin.ID, actor(other)) }},
	}
	for _, s := range steps {
		tk := m.tokens()
		if err := s.cut(); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if _, err := m.auth.Verify(ctx, tk.Access); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("%s: Verify = %v", s.name, err)
		}
		if _, err := m.auth.Refresh(ctx, tk.Refresh, m.client.ID); !errors.Is(err, domain.ErrInvalidGrant) {
			t.Errorf("%s: Refresh = %v", s.name, err)
		}
		if err := s.back(); err != nil {
			t.Fatalf("%s back: %v", s.name, err)
		}
		m.admin = m.get(m.admin.ID)
	}
	tk := m.tokens()
	gs, _ := m.auth.Grants(ctx, m.admin.ID)
	if err := m.auth.Revoke(ctx, gs[0].ID, actor(other)); err != nil {
		t.Fatal(err)
	}
	if _, err := m.auth.Verify(ctx, tk.Access); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("revoked grant: Verify = %v", err)
	}
	if m.auditCount(domain.ActMCPClientRevoked) != 1 {
		t.Error("no mcp.client_revoked audit")
	}
	if gs, _ := m.auth.Grants(ctx, m.admin.ID); len(gs) != 0 {
		t.Errorf("revoked grant still listed")
	}
	tk = m.tokens() // approving again reactivates the grant
	if m.auditCount(domain.ActMCPClientAuthorized) != 2 {
		t.Errorf("reactivation audits = %d", m.auditCount(domain.ActMCPClientAuthorized))
	}
	m.clock.Advance(time.Hour + time.Second)
	if _, err := m.auth.Verify(ctx, tk.Access); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("access after 1 h = %v", err)
	}
	if _, err := m.auth.Verify(ctx, "garbage"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("garbage = %v", err)
	}
}
