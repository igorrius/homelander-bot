package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Audit implements domain.AuditRepo.
type Audit struct{ DB DB }

var _ domain.AuditRepo = Audit{}

// Add appends an entry.
func (r Audit) Add(ctx context.Context, e domain.AuditEntry) error {
	details := []byte("{}")
	if len(e.Details) > 0 {
		var err error
		if details, err = json.Marshal(e.Details); err != nil {
			return err
		}
	}
	if e.At.IsZero() {
		e.At = now()
	}
	_, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO audit_log (at, actor_user_id, action, target, details, ip)
		VALUES (?, ?, ?, ?, ?, ?)`, fmtTime(e.At), nullInt(e.ActorUserID), e.Action, e.Target, string(details), e.IP)
	return err
}

// List returns entries newest first and the total count.
func (r Audit) List(ctx context.Context, f domain.AuditFilter) ([]domain.AuditEntry, int, error) {
	where, args := "1 = 1", []any{}
	if f.TargetUserID != nil {
		where, args = "target = ?", append(args, domain.UserTarget(*f.TargetUserID))
	}
	q := r.DB.Q(ctx)
	var total int
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM audit_log WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := f.Limit
	if limit <= 0 {
		limit = -1
	}
	rows, err := q.QueryContext(ctx, `SELECT id, at, actor_user_id, action, target, details, ip FROM audit_log
		WHERE `+where+` ORDER BY at DESC, id DESC LIMIT ? OFFSET ?`, append(args, limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.AuditEntry
	for rows.Next() {
		var e domain.AuditEntry
		var at, details string
		var actor sql.NullInt64
		if err := rows.Scan(&e.ID, &at, &actor, &e.Action, &e.Target, &details, &e.IP); err != nil {
			return nil, 0, err
		}
		e.At, e.ActorUserID = parseTime(at), intPtr(actor)
		if err := json.Unmarshal([]byte(details), &e.Details); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}
