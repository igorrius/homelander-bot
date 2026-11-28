package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Modules implements domain.ModuleRepo.
type Modules struct{ DB DB }

var _ domain.ModuleRepo = Modules{}

// Get returns a module's row; ErrNotFound when it has none.
func (r Modules) Get(ctx context.Context, code string) (domain.ModuleState, error) {
	var (
		s                 domain.ModuleState
		settings, updated string
		by                sql.NullInt64
	)
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT code, enabled, settings, updated_by, updated_at FROM modules WHERE code = ?`, code).
		Scan(&s.Code, &s.Enabled, &settings, &by, &updated)
	if err != nil {
		return domain.ModuleState{}, mapErr(err)
	}
	s.Settings, s.UpdatedBy, s.UpdatedAt = json.RawMessage(settings), intPtr(by), parseTime(updated)
	return s, nil
}

// Save inserts or replaces a module's row.
func (r Modules) Save(ctx context.Context, s domain.ModuleState) error {
	settings := string(s.Settings)
	if settings == "" {
		settings = "{}"
	}
	_, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO modules (code, enabled, settings, updated_by, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (code) DO UPDATE SET enabled = excluded.enabled, settings = excluded.settings,
		updated_by = excluded.updated_by, updated_at = excluded.updated_at`,
		s.Code, s.Enabled, settings, nullInt(s.UpdatedBy), fmtTime(s.UpdatedAt))
	return mapErr(err)
}
