package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Codes implements domain.CodeRepo.
type Codes struct{ DB DB }

var _ domain.CodeRepo = Codes{}

// Replace deletes the user's unused codes and inserts c.
func (r Codes) Replace(ctx context.Context, c *domain.LoginCode) error {
	return r.DB.InTx(ctx, func(ctx context.Context) error {
		q := r.DB.Q(ctx)
		if _, err := q.ExecContext(ctx, `DELETE FROM login_codes WHERE user_id = ? AND used_at IS NULL`, c.UserID); err != nil {
			return err
		}
		res, err := q.ExecContext(ctx, `INSERT INTO login_codes (user_id, code_hash, expires_at, attempts, ip)
			VALUES (?, ?, ?, ?, ?)`, c.UserID, c.CodeHash, fmtTime(c.ExpiresAt), c.Attempts, c.IP)
		if err != nil {
			return mapErr(err)
		}
		c.ID, err = res.LastInsertId()
		return err
	})
}

// Active returns the user's unused, unexpired code.
func (r Codes) Active(ctx context.Context, userID int64, now time.Time) (domain.LoginCode, error) {
	var c domain.LoginCode
	var expires string
	var used sql.NullString
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT id, user_id, code_hash, expires_at, attempts, used_at, ip
		FROM login_codes WHERE user_id = ? AND used_at IS NULL AND expires_at > ? ORDER BY id DESC LIMIT 1`,
		userID, fmtTime(now)).Scan(&c.ID, &c.UserID, &c.CodeHash, &expires, &c.Attempts, &used, &c.IP)
	if err != nil {
		return c, mapErr(err)
	}
	c.ExpiresAt, c.UsedAt = parseTime(expires), parseTimePtr(used)
	return c, nil
}

// IncAttempts adds one failed attempt and returns the new count.
func (r Codes) IncAttempts(ctx context.Context, id int64) (int, error) {
	var n int
	err := r.DB.Q(ctx).QueryRowContext(ctx, `UPDATE login_codes SET attempts = attempts + 1 WHERE id = ? RETURNING attempts`, id).Scan(&n)
	return n, mapErr(err)
}

// MarkUsed marks the code used; false means it already was.
func (r Codes) MarkUsed(ctx context.Context, id int64, at time.Time) (bool, error) {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE login_codes SET used_at = ? WHERE id = ? AND used_at IS NULL`, fmtTime(at), id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// Sessions implements domain.SessionRepo.
type Sessions struct{ DB DB }

var _ domain.SessionRepo = Sessions{}

// Create inserts a session.
func (r Sessions) Create(ctx context.Context, s *domain.Session) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO sessions (user_id, token_hash, created_at, last_seen_at,
		expires_at, ip, user_agent) VALUES (?, ?, ?, ?, ?, ?, ?)`, s.UserID, s.TokenHash, fmtTime(s.CreatedAt),
		fmtTime(s.LastSeenAt), fmtTime(s.ExpiresAt), s.IP, s.UserAgent)
	if err != nil {
		return mapErr(err)
	}
	s.ID, err = res.LastInsertId()
	return err
}

// GetByHash finds a session by its token hash.
func (r Sessions) GetByHash(ctx context.Context, hash []byte) (domain.Session, error) {
	var s domain.Session
	var created, seen, expires string
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT id, user_id, token_hash, created_at, last_seen_at, expires_at, ip,
		user_agent FROM sessions WHERE token_hash = ?`, hash).Scan(&s.ID, &s.UserID, &s.TokenHash, &created, &seen,
		&expires, &s.IP, &s.UserAgent)
	if err != nil {
		return s, mapErr(err)
	}
	s.CreatedAt, s.LastSeenAt, s.ExpiresAt = parseTime(created), parseTime(seen), parseTime(expires)
	return s, nil
}

// Touch records activity on a session.
func (r Sessions) Touch(ctx context.Context, id int64, at time.Time) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE sessions SET last_seen_at = ? WHERE id = ?`, fmtTime(at), id)
	return err
}

// Delete removes one session.
func (r Sessions) Delete(ctx context.Context, id int64) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DeleteByUser ends every session of a user.
func (r Sessions) DeleteByUser(ctx context.Context, userID int64) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// DeleteExpired removes sessions past their absolute lifetime.
func (r Sessions) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, fmtTime(now))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// LoginLinks implements domain.LoginLinkRepo.
type LoginLinks struct{ DB DB }

var _ domain.LoginLinkRepo = LoginLinks{}

// Create inserts l and sets its ID.
func (r LoginLinks) Create(ctx context.Context, l *domain.LoginLink) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO login_links (token_hash, user_id, next, expires_at, used_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, l.TokenHash, l.UserID, l.Next, fmtTime(l.ExpiresAt), fmtTimePtr(l.UsedAt), fmtTime(l.CreatedAt))
	if err != nil {
		return mapErr(err)
	}
	l.ID, err = res.LastInsertId()
	return err
}

// GetByHash finds a link by its token hash.
func (r LoginLinks) GetByHash(ctx context.Context, hash []byte) (domain.LoginLink, error) {
	var (
		l                domain.LoginLink
		expires, created string
		used             sql.NullString
	)
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT id, token_hash, user_id, next, expires_at, used_at, created_at
		FROM login_links WHERE token_hash = ?`, hash).Scan(&l.ID, &l.TokenHash, &l.UserID, &l.Next, &expires, &used, &created)
	if err != nil {
		return domain.LoginLink{}, mapErr(err)
	}
	l.ExpiresAt, l.UsedAt, l.CreatedAt = parseTime(expires), parseTimePtr(used), parseTime(created)
	return l, nil
}

// MarkUsed stamps a link used; false when it already was.
func (r LoginLinks) MarkUsed(ctx context.Context, id int64, at time.Time) (bool, error) {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE login_links SET used_at = ? WHERE id = ? AND used_at IS NULL`, fmtTime(at), id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// DeleteExpiredBefore removes links that expired before t.
func (r LoginLinks) DeleteExpiredBefore(ctx context.Context, t time.Time) (int, error) {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM login_links WHERE expires_at < ?`, fmtTime(t))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}
