package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// OAuth lifetimes of the MCP authorization server.
const (
	MCPCodeTTL    = 60 * time.Second
	MCPAccessTTL  = time.Hour
	MCPRefreshTTL = 30 * 24 * time.Hour

	mcpMaxRedirects  = 10
	mcpMaxClientName = 100
	mcpTouchEvery    = time.Minute
)

// McpAuthService is Homelander's OAuth 2.1 authorization server for MCP
// agents: PKCE S256 only, public clients, hashed single-use codes and
// rotating refresh tokens. Every token acts as one admin through one client,
// and is good only while that admin is active, Admin and MCP.
type McpAuthService struct {
	R       Repos
	Now     Clock
	Allowed RedirectPolicy
	// Resource is the MCP endpoint URL; a request naming another resource
	// (RFC 8707) is refused. Empty accepts any.
	Resource string
}

func (s *McpAuthService) audit(ctx context.Context, a Actor, action string, userID int64, details map[string]any) error {
	return s.R.Audit.Add(ctx, AuditEntry{At: s.Now(), ActorUserID: a.UserID, Action: action, Target: UserTarget(userID),
		Details: details, IP: a.IP})
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

// Register stores a new public client. Every redirect URI must be allowed by
// the policy.
func (s *McpAuthService) Register(ctx context.Context, name string, redirectURIs []string) (MCPClient, error) {
	if len(redirectURIs) == 0 || len(redirectURIs) > mcpMaxRedirects {
		return MCPClient{}, ErrInvalidRedirect
	}
	for _, u := range redirectURIs {
		if !s.Allowed.Allows(u) {
			return MCPClient{}, ErrInvalidRedirect
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Unnamed client"
	}
	if utf8.RuneCountInString(name) > mcpMaxClientName {
		name = string([]rune(name)[:mcpMaxClientName])
	}
	c := MCPClient{ID: randomToken()[:22], Name: name, RedirectURIs: slices.Clone(redirectURIs), CreatedAt: s.Now()}
	return c, s.R.MCP.CreateClient(ctx, c)
}

// Client returns a registered client.
func (s *McpAuthService) Client(ctx context.Context, id string) (MCPClient, error) {
	return s.R.MCP.GetClient(ctx, id)
}

// CheckRequest validates an authorization request against the registered
// client; ErrInvalidRequest or ErrInvalidRedirect when it does not fit.
func (s *McpAuthService) CheckRequest(ctx context.Context, req MCPAuthRequest) (MCPClient, error) {
	c, err := s.R.MCP.GetClient(ctx, req.ClientID)
	if errors.Is(err, ErrNotFound) {
		return c, ErrInvalidRequest
	}
	if err != nil {
		return c, err
	}
	if !slices.Contains(c.RedirectURIs, req.RedirectURI) {
		return c, ErrInvalidRedirect
	}
	return c, nil
}

func (s *McpAuthService) resourceOK(r string) bool {
	return r == "" || s.Resource == "" || strings.TrimRight(r, "/") == strings.TrimRight(s.Resource, "/")
}

// Approve records u's consent and returns a single-use authorization code.
// Only a user with MCP enabled may approve. The grant of (user, client) is
// reused; a new or reactivated grant is audited.
func (s *McpAuthService) Approve(ctx context.Context, u User, req MCPAuthRequest, a Actor) (string, error) {
	c, err := s.CheckRequest(ctx, req)
	if err != nil {
		return "", err
	}
	if req.Method != "S256" || len(req.Challenge) < 43 || len(req.Challenge) > 128 || !s.resourceOK(req.Resource) {
		return "", ErrInvalidRequest
	}
	if !u.MCPEnabled() {
		return "", ErrForbidden
	}
	code := randomToken()
	err = s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		fresh, err := s.useGrant(ctx, u.ID, c.ID)
		if err != nil {
			return err
		}
		g, err := s.R.MCP.GrantFor(ctx, u.ID, c.ID)
		if err != nil {
			return err
		}
		if fresh {
			host := ""
			if ru, err := url.Parse(req.RedirectURI); err == nil {
				host = ru.Host
			}
			if err := s.audit(ctx, a, ActMCPClientAuthorized, u.ID, map[string]any{"client": c.Name, "redirectHost": host}); err != nil {
				return err
			}
		}
		return s.R.MCP.CreateCode(ctx, MCPCode{Hash: hashToken(code), GrantID: g.ID, RedirectURI: req.RedirectURI,
			Challenge: req.Challenge, Resource: req.Resource, ExpiresAt: s.Now().Add(MCPCodeTTL)})
	})
	return code, err
}

// useGrant makes sure an active grant exists; true when it is new or was revoked.
func (s *McpAuthService) useGrant(ctx context.Context, userID int64, clientID string) (bool, error) {
	g, err := s.R.MCP.GrantFor(ctx, userID, clientID)
	switch {
	case errors.Is(err, ErrNotFound):
		return true, s.R.MCP.CreateGrant(ctx, &MCPGrant{UserID: userID, ClientID: clientID, CreatedAt: s.Now()})
	case err != nil:
		return false, err
	case g.RevokedAt != nil:
		return true, s.R.MCP.SetGrantRevoked(ctx, g.ID, nil)
	}
	return false, nil
}

// activeGrant returns the grant and its user when both still allow MCP.
func (s *McpAuthService) activeGrant(ctx context.Context, id int64) (MCPPrincipal, bool, error) {
	g, err := s.R.MCP.GetGrant(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return MCPPrincipal{}, false, nil
	}
	if err != nil {
		return MCPPrincipal{}, false, err
	}
	if g.RevokedAt != nil {
		return MCPPrincipal{}, false, nil
	}
	u, err := s.R.Users.Get(ctx, g.UserID)
	if errors.Is(err, ErrNotFound) {
		return MCPPrincipal{}, false, nil
	}
	if err != nil {
		return MCPPrincipal{}, false, err
	}
	return MCPPrincipal{User: u, Grant: g}, u.MCPEnabled(), nil
}

func (s *McpAuthService) issue(ctx context.Context, grantID int64) (MCPTokens, error) {
	t := MCPTokens{Access: randomToken(), Refresh: randomToken(), ExpiresIn: int(MCPAccessTTL / time.Second)}
	now := s.Now()
	if err := s.R.MCP.CreateToken(ctx, MCPToken{Hash: hashToken(t.Access), GrantID: grantID, Kind: MCPAccess, ExpiresAt: now.Add(MCPAccessTTL)}); err != nil {
		return MCPTokens{}, err
	}
	if err := s.R.MCP.CreateToken(ctx, MCPToken{Hash: hashToken(t.Refresh), GrantID: grantID, Kind: MCPRefresh, ExpiresAt: now.Add(MCPRefreshTTL)}); err != nil {
		return MCPTokens{}, err
	}
	return t, nil
}

func pkceOK(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(want), []byte(challenge)) == 1
}

// Exchange trades an authorization code for tokens. Any mismatch (unknown,
// used or expired code, other client or redirect URI, wrong verifier) is
// ErrInvalidGrant, and the code is spent either way.
func (s *McpAuthService) Exchange(ctx context.Context, code, clientID, redirectURI, verifier string) (MCPTokens, error) {
	c, err := s.R.MCP.UseCode(ctx, hashToken(code), s.Now()) // spent before any check, outside the transaction
	if errors.Is(err, ErrNotFound) {
		return MCPTokens{}, ErrInvalidGrant
	}
	if err != nil {
		return MCPTokens{}, err
	}
	if !s.Now().Before(c.ExpiresAt) || c.RedirectURI != redirectURI || !pkceOK(verifier, c.Challenge) {
		return MCPTokens{}, ErrInvalidGrant
	}
	var out MCPTokens
	err = s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		p, ok, err := s.activeGrant(ctx, c.GrantID)
		if err != nil {
			return err
		}
		if !ok || p.Grant.ClientID != clientID {
			return ErrInvalidGrant
		}
		out, err = s.issue(ctx, c.GrantID)
		return err
	})
	return out, err
}

// Refresh rotates a refresh token: the old one is revoked and a new pair is
// issued. Access tokens already issued stay valid until they expire.
func (s *McpAuthService) Refresh(ctx context.Context, refresh, clientID string) (MCPTokens, error) {
	var out MCPTokens
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		t, err := s.R.MCP.GetToken(ctx, hashToken(refresh))
		if errors.Is(err, ErrNotFound) {
			return ErrInvalidGrant
		}
		if err != nil {
			return err
		}
		if t.Kind != MCPRefresh || t.RevokedAt != nil || !s.Now().Before(t.ExpiresAt) {
			return ErrInvalidGrant
		}
		p, ok, err := s.activeGrant(ctx, t.GrantID)
		if err != nil {
			return err
		}
		if !ok || p.Grant.ClientID != clientID {
			return ErrInvalidGrant
		}
		if revoked, err := s.R.MCP.RevokeToken(ctx, t.Hash, s.Now()); err != nil {
			return err
		} else if !revoked {
			return ErrInvalidGrant
		}
		out, err = s.issue(ctx, t.GrantID)
		return err
	})
	return out, err
}

// Verify returns who an access token acts as. The user must still be active,
// Admin and MCP, and the grant not revoked; otherwise ErrUnauthorized.
func (s *McpAuthService) Verify(ctx context.Context, access string) (MCPPrincipal, error) {
	t, err := s.R.MCP.GetToken(ctx, hashToken(access))
	if errors.Is(err, ErrNotFound) {
		return MCPPrincipal{}, ErrUnauthorized
	}
	if err != nil {
		return MCPPrincipal{}, err
	}
	now := s.Now()
	if t.Kind != MCPAccess || t.RevokedAt != nil || !now.Before(t.ExpiresAt) {
		return MCPPrincipal{}, ErrUnauthorized
	}
	p, ok, err := s.activeGrant(ctx, t.GrantID)
	if err != nil {
		return MCPPrincipal{}, err
	}
	if !ok {
		return MCPPrincipal{}, ErrUnauthorized
	}
	if p.Grant.LastUsedAt == nil || now.Sub(*p.Grant.LastUsedAt) >= mcpTouchEvery {
		if err := s.R.MCP.TouchGrant(ctx, p.Grant.ID, now); err != nil {
			return MCPPrincipal{}, err
		}
		p.Grant.LastUsedAt = &now
	}
	p.ExpiresAt = t.ExpiresAt
	return p, nil
}

// RevokeToken revokes one token (RFC 7009); unknown tokens are ignored.
func (s *McpAuthService) RevokeToken(ctx context.Context, token string) error {
	_, err := s.R.MCP.RevokeToken(ctx, hashToken(token), s.Now())
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// Grants lists a user's connected agents.
func (s *McpAuthService) Grants(ctx context.Context, userID int64) ([]MCPGrant, error) {
	return s.R.MCP.ListGrants(ctx, userID)
}

// Revoke disconnects an agent: the grant and all its tokens are revoked.
func (s *McpAuthService) Revoke(ctx context.Context, grantID int64, a Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		g, err := s.R.MCP.GetGrant(ctx, grantID)
		if err != nil {
			return err
		}
		if g.RevokedAt != nil {
			return ErrNotFound
		}
		now := s.Now()
		if err := s.R.MCP.SetGrantRevoked(ctx, g.ID, &now); err != nil {
			return err
		}
		return s.audit(ctx, a, ActMCPClientRevoked, g.UserID, map[string]any{"client": g.ClientName})
	})
}
