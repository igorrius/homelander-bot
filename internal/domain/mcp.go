package domain

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
)

// OAuth errors of the MCP authorization server; their texts are the OAuth
// error codes.
var (
	ErrInvalidGrant    = errors.New("invalid_grant")
	ErrInvalidRedirect = errors.New("invalid_redirect_uri")
	ErrInvalidRequest  = errors.New("invalid_request")
)

// MCP audit actions.
const (
	ActMCPClientAuthorized = "mcp.client_authorized"
	ActMCPClientRevoked    = "mcp.client_revoked"
)

// MCPClient is an agent registered through dynamic client registration.
type MCPClient struct {
	ID           string
	Name         string // self-reported, shown on the consent screen
	RedirectURIs []string
	CreatedAt    time.Time
}

// MCPGrant is one connected agent: one user and one client.
type MCPGrant struct {
	ID         int64
	UserID     int64
	ClientID   string
	ClientName string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// MCPAuthRequest is what the agent asks for on the authorization endpoint.
type MCPAuthRequest struct {
	ClientID, RedirectURI, Challenge, Method, State, Resource string
}

// MCPTokens is a token endpoint answer.
type MCPTokens struct {
	Access, Refresh string
	ExpiresIn       int // seconds
}

// MCPPrincipal is who an access token acts as.
type MCPPrincipal struct {
	User      User
	Grant     MCPGrant
	ExpiresAt time.Time // of the access token
}

// MCPCode is a stored authorization code.
type MCPCode struct {
	Hash        string
	GrantID     int64
	RedirectURI string
	Challenge   string
	Resource    string
	ExpiresAt   time.Time
	UsedAt      *time.Time
}

// MCP token kinds.
const (
	MCPAccess  = "access"
	MCPRefresh = "refresh"
)

// MCPToken is a stored access or refresh token.
type MCPToken struct {
	Hash      string
	GrantID   int64
	Kind      string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// MCPRepo stores OAuth clients, grants, codes and tokens. Secrets are stored
// only as hashes.
type MCPRepo interface {
	CreateClient(ctx context.Context, c MCPClient) error
	GetClient(ctx context.Context, id string) (MCPClient, error)
	CreateGrant(ctx context.Context, g *MCPGrant) error
	GetGrant(ctx context.Context, id int64) (MCPGrant, error)
	GrantFor(ctx context.Context, userID int64, clientID string) (MCPGrant, error)
	// ListGrants returns the user's grants that are not revoked.
	ListGrants(ctx context.Context, userID int64) ([]MCPGrant, error)
	// SetGrantRevoked sets or (nil) clears revoked_at; setting it also deletes
	// the grant's tokens.
	SetGrantRevoked(ctx context.Context, id int64, at *time.Time) error
	// DeleteGrantTokens deletes every token of a grant.
	DeleteGrantTokens(ctx context.Context, grantID int64) error
	TouchGrant(ctx context.Context, id int64, at time.Time) error
	CreateCode(ctx context.Context, c MCPCode) error
	// UseCode marks an unused code used and returns it; ErrNotFound when the
	// code is unknown or was used before.
	UseCode(ctx context.Context, hash string, at time.Time) (MCPCode, error)
	CreateToken(ctx context.Context, t MCPToken) error
	GetToken(ctx context.Context, hash string) (MCPToken, error)
	// RevokeToken revokes a token that is not revoked yet; false when it was.
	RevokeToken(ctx context.Context, hash string, at time.Time) (bool, error)
}

// RedirectPolicy lists the redirect URIs agents may register. An entry
// matches by exact string, except http://localhost:* and http://127.0.0.1:*
// (a loopback host with a "*" port), which match any port and path.
type RedirectPolicy []string

var loopbackHosts = map[string]bool{"localhost": true, "127.0.0.1": true, "[::1]": true}

// Allows reports whether uri may be registered.
func (p RedirectPolicy) Allows(uri string) bool {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme == "" || u.Host == "" || u.Opaque != "" || u.User != nil || u.Fragment != "" || strings.Contains(uri, "#") {
		return false
	}
	for _, pat := range p {
		if pat == uri {
			return true
		}
		scheme, hostPort, ok := strings.Cut(pat, "://")
		host, ok2 := strings.CutSuffix(hostPort, ":*")
		if !ok || !ok2 || !loopbackHosts[host] {
			continue
		}
		if u.Scheme == scheme && u.Host == host || u.Scheme == scheme && strings.HasPrefix(u.Host, host+":") && u.Hostname() == strings.Trim(host, "[]") {
			return true
		}
	}
	return false
}
