package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// MCP implements domain.MCPRepo.
type MCP struct{ DB DB }

var _ domain.MCPRepo = MCP{}

// CreateClient inserts a client.
func (r MCP) CreateClient(ctx context.Context, c domain.MCPClient) error {
	uris, err := json.Marshal(c.RedirectURIs)
	if err != nil {
		return err
	}
	_, err = r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO mcp_clients (id, name, redirect_uris, created_at) VALUES (?, ?, ?, ?)`,
		c.ID, c.Name, string(uris), fmtTime(c.CreatedAt))
	return mapErr(err)
}

// GetClient returns a client, or domain.ErrNotFound.
func (r MCP) GetClient(ctx context.Context, id string) (domain.MCPClient, error) {
	var c domain.MCPClient
	var uris, created string
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT id, name, redirect_uris, created_at FROM mcp_clients WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &uris, &created)
	if err != nil {
		return c, mapErr(err)
	}
	c.CreatedAt = parseTime(created)
	return c, json.Unmarshal([]byte(uris), &c.RedirectURIs)
}

// CreateGrant inserts a grant and sets its ID.
func (r MCP) CreateGrant(ctx context.Context, g *domain.MCPGrant) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO mcp_grants (user_id, client_id, created_at) VALUES (?, ?, ?)`,
		g.UserID, g.ClientID, fmtTime(g.CreatedAt))
	if err != nil {
		return mapErr(err)
	}
	g.ID, err = res.LastInsertId()
	return err
}

const grantSelect = `SELECT g.id, g.user_id, g.client_id, c.name, g.created_at, g.last_used_at, g.revoked_at
	FROM mcp_grants g JOIN mcp_clients c ON c.id = g.client_id `

func scanGrant(s scanner) (domain.MCPGrant, error) {
	var g domain.MCPGrant
	var created string
	var used, revoked sql.NullString
	if err := s.Scan(&g.ID, &g.UserID, &g.ClientID, &g.ClientName, &created, &used, &revoked); err != nil {
		return g, mapErr(err)
	}
	g.CreatedAt, g.LastUsedAt, g.RevokedAt = parseTime(created), parseTimePtr(used), parseTimePtr(revoked)
	return g, nil
}

// GetGrant returns a grant with its client name.
func (r MCP) GetGrant(ctx context.Context, id int64) (domain.MCPGrant, error) {
	return scanGrant(r.DB.Q(ctx).QueryRowContext(ctx, grantSelect+`WHERE g.id = ?`, id))
}

// GrantFor returns the grant of a user and a client.
func (r MCP) GrantFor(ctx context.Context, userID int64, clientID string) (domain.MCPGrant, error) {
	return scanGrant(r.DB.Q(ctx).QueryRowContext(ctx, grantSelect+`WHERE g.user_id = ? AND g.client_id = ?`, userID, clientID))
}

// ListGrants returns the user's grants that are not revoked, oldest first.
func (r MCP) ListGrants(ctx context.Context, userID int64) ([]domain.MCPGrant, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, grantSelect+`WHERE g.user_id = ? AND g.revoked_at IS NULL ORDER BY g.id`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []domain.MCPGrant{}
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// SetGrantRevoked sets or clears revoked_at; setting it deletes the grant's
// tokens, so a later reuse of one is not mistaken for a stolen rotated token.
func (r MCP) SetGrantRevoked(ctx context.Context, id int64, at *time.Time) error {
	return r.DB.InTx(ctx, func(ctx context.Context) error {
		q := r.DB.Q(ctx)
		res, err := q.ExecContext(ctx, `UPDATE mcp_grants SET revoked_at = ? WHERE id = ?`, fmtTimePtr(at), id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return domain.ErrNotFound
		}
		if at == nil {
			return nil
		}
		return r.DeleteGrantTokens(ctx, id)
	})
}

// DeleteGrantTokens deletes every token of a grant.
func (r MCP) DeleteGrantTokens(ctx context.Context, grantID int64) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM mcp_tokens WHERE grant_id = ?`, grantID)
	return err
}

// TouchGrant records when the grant was last used.
func (r MCP) TouchGrant(ctx context.Context, id int64, at time.Time) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE mcp_grants SET last_used_at = ? WHERE id = ?`, fmtTime(at), id)
	return err
}

// CreateCode inserts an authorization code.
func (r MCP) CreateCode(ctx context.Context, c domain.MCPCode) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO mcp_codes (hash, grant_id, redirect_uri, challenge, resource, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`, c.Hash, c.GrantID, c.RedirectURI, c.Challenge, c.Resource, fmtTime(c.ExpiresAt))
	return mapErr(err)
}

// UseCode marks an unused code used and returns it.
func (r MCP) UseCode(ctx context.Context, hash string, at time.Time) (domain.MCPCode, error) {
	var c domain.MCPCode
	var expires, used string
	err := r.DB.Q(ctx).QueryRowContext(ctx, `UPDATE mcp_codes SET used_at = ? WHERE hash = ? AND used_at IS NULL
		RETURNING hash, grant_id, redirect_uri, challenge, resource, expires_at, used_at`, fmtTime(at), hash).
		Scan(&c.Hash, &c.GrantID, &c.RedirectURI, &c.Challenge, &c.Resource, &expires, &used)
	if err != nil {
		return c, mapErr(err)
	}
	u := parseTime(used)
	c.ExpiresAt, c.UsedAt = parseTime(expires), &u
	return c, nil
}

// CreateToken inserts a token.
func (r MCP) CreateToken(ctx context.Context, t domain.MCPToken) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO mcp_tokens (hash, grant_id, kind, expires_at) VALUES (?, ?, ?, ?)`,
		t.Hash, t.GrantID, t.Kind, fmtTime(t.ExpiresAt))
	return mapErr(err)
}

// GetToken returns a token by hash.
func (r MCP) GetToken(ctx context.Context, hash string) (domain.MCPToken, error) {
	var t domain.MCPToken
	var expires string
	var revoked sql.NullString
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT hash, grant_id, kind, expires_at, revoked_at FROM mcp_tokens WHERE hash = ?`, hash).
		Scan(&t.Hash, &t.GrantID, &t.Kind, &expires, &revoked)
	if err != nil {
		return t, mapErr(err)
	}
	t.ExpiresAt, t.RevokedAt = parseTime(expires), parseTimePtr(revoked)
	return t, nil
}

// RevokeToken revokes a token that is not revoked yet.
func (r MCP) RevokeToken(ctx context.Context, hash string, at time.Time) (bool, error) {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE mcp_tokens SET revoked_at = ? WHERE hash = ? AND revoked_at IS NULL`, fmtTime(at), hash)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
