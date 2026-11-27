package sqlite

import (
	"context"
	"strings"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Labels implements domain.LabelRepo.
type Labels struct{ DB DB }

var _ domain.LabelRepo = Labels{}

const labelColumns = `id, name, description, created_at, updated_at`

// labelKey folds a label name for uniqueness (SQLite's lower() only knows ASCII).
func labelKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

func scanLabel(s scanner) (domain.Label, error) {
	var (
		l                domain.Label
		created, updated string
	)
	if err := s.Scan(&l.ID, &l.Name, &l.Description, &created, &updated); err != nil {
		return domain.Label{}, mapErr(err)
	}
	l.CreatedAt, l.UpdatedAt = parseTime(created), parseTime(updated)
	return l, nil
}

// Create inserts l; ErrConflict when the case-folded name exists.
func (r Labels) Create(ctx context.Context, l *domain.Label) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO labels (name, name_key, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`, l.Name, labelKey(l.Name), l.Description, fmtTime(l.CreatedAt), fmtTime(l.UpdatedAt))
	if err != nil {
		return mapErr(err)
	}
	l.ID, err = res.LastInsertId()
	return err
}

// Update writes name, description and updated_at.
func (r Labels) Update(ctx context.Context, l *domain.Label) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE labels SET name = ?, name_key = ?, description = ?, updated_at = ?
		WHERE id = ?`, l.Name, labelKey(l.Name), l.Description, fmtTime(l.UpdatedAt), l.ID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Get returns one label.
func (r Labels) Get(ctx context.Context, id int64) (domain.Label, error) {
	return scanLabel(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+labelColumns+` FROM labels WHERE id = ?`, id))
}

// List returns every label by folded name.
func (r Labels) List(ctx context.Context) ([]domain.Label, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT `+labelColumns+` FROM labels ORDER BY name_key, id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Label
	for rows.Next() {
		l, err := scanLabel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Delete removes a label and its bindings.
func (r Labels) Delete(ctx context.Context, id int64) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM labels WHERE id = ?`, id)
	return err
}

// Targets lists where a label hangs.
func (r Labels) Targets(ctx context.Context, labelID int64) ([]domain.LabelTarget, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT target_type, target_id FROM label_bindings WHERE label_id = ?
		ORDER BY target_type, target_id`, labelID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.LabelTarget
	for rows.Next() {
		var t domain.LabelTarget
		if err := rows.Scan(&t.Type, &t.ID); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// LabelsOf lists the label ids on a target, ascending.
func (r Labels) LabelsOf(ctx context.Context, t domain.LabelTarget) ([]int64, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT label_id FROM label_bindings WHERE target_type = ? AND target_id = ?
		ORDER BY label_id`, t.Type, t.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetLabelsOf replaces the labels on a target.
func (r Labels) SetLabelsOf(ctx context.Context, t domain.LabelTarget, labelIDs []int64) error {
	q := r.DB.Q(ctx)
	if err := r.DeleteTarget(ctx, t); err != nil {
		return err
	}
	at := fmtTime(now())
	for _, id := range labelIDs {
		if _, err := q.ExecContext(ctx, `INSERT OR IGNORE INTO label_bindings (label_id, target_type, target_id, created_at)
			VALUES (?, ?, ?, ?)`, id, t.Type, t.ID, at); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

// Retarget moves every binding of from to to, without duplicates.
func (r Labels) Retarget(ctx context.Context, from, to domain.LabelTarget) error {
	if _, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT OR IGNORE INTO label_bindings (label_id, target_type, target_id, created_at)
		SELECT label_id, ?, ?, created_at FROM label_bindings WHERE target_type = ? AND target_id = ?`,
		to.Type, to.ID, from.Type, from.ID); err != nil {
		return mapErr(err)
	}
	return r.DeleteTarget(ctx, from)
}

// DeleteTarget removes every binding of a target.
func (r Labels) DeleteTarget(ctx context.Context, t domain.LabelTarget) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM label_bindings WHERE target_type = ? AND target_id = ?`, t.Type, t.ID)
	return err
}
