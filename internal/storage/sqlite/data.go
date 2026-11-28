package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Data implements domain.DataRepo.
type Data struct{ DB DB }

var _ domain.DataRepo = Data{}

const dataColumns = `i.id, i.key, i.kind, i.description, i.visibility, i.value_type, i.value_json, i.columns_json,
	i.owner, i.created_by, i.updated_by, i.created_at, i.updated_at,
	(SELECT count(*) FROM data_rows r WHERE r.item_id = i.id)`

func scanDataItem(s scanner) (domain.DataItem, error) {
	var (
		it                   domain.DataItem
		kind, vis            string
		vtype, value, cols   sql.NullString
		owner                sql.NullString
		createdBy, updatedBy sql.NullInt64
		created, updated     string
	)
	if err := s.Scan(&it.ID, &it.Key, &kind, &it.Description, &vis, &vtype, &value, &cols,
		&owner, &createdBy, &updatedBy, &created, &updated, &it.RowCount); err != nil {
		return domain.DataItem{}, mapErr(err)
	}
	it.Kind, it.Visibility = domain.DataKind(kind), domain.Visibility(vis)
	it.ValueType, it.Owner = domain.DataType(vtype.String), owner.String
	if value.Valid {
		it.Value = json.RawMessage(value.String)
	}
	if cols.Valid {
		if err := json.Unmarshal([]byte(cols.String), &it.Columns); err != nil {
			return domain.DataItem{}, err
		}
	}
	it.CreatedBy, it.UpdatedBy = intPtr(createdBy), intPtr(updatedBy)
	it.CreatedAt, it.UpdatedAt = parseTime(created), parseTime(updated)
	return it, nil
}

func dataArgs(it *domain.DataItem) (vtype, value, cols any, err error) {
	if it.ValueType != "" {
		vtype = string(it.ValueType)
	}
	if it.Value != nil {
		value = string(it.Value)
	}
	if it.Columns != nil {
		b, e := json.Marshal(it.Columns)
		if e != nil {
			return nil, nil, nil, e
		}
		cols = string(b)
	}
	return vtype, value, cols, nil
}

// Create inserts it; ErrConflict when the key is taken.
func (r Data) Create(ctx context.Context, it *domain.DataItem) error {
	vtype, value, cols, err := dataArgs(it)
	if err != nil {
		return err
	}
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO data_items (key, kind, description, visibility, value_type,
		value_json, columns_json, owner, created_by, updated_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		it.Key, string(it.Kind), it.Description, visArg(it.Visibility), vtype, value, cols, nullStr(it.Owner),
		nullInt(it.CreatedBy), nullInt(it.UpdatedBy), fmtTime(it.CreatedAt), fmtTime(it.UpdatedAt))
	if err != nil {
		return mapErr(err)
	}
	it.ID, err = res.LastInsertId()
	return err
}

// Update writes everything but the key, kind and creation fields.
func (r Data) Update(ctx context.Context, it *domain.DataItem) error {
	vtype, value, cols, err := dataArgs(it)
	if err != nil {
		return err
	}
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE data_items SET description = ?, visibility = ?, value_type = ?,
		value_json = ?, columns_json = ?, updated_by = ?, updated_at = ? WHERE id = ?`,
		it.Description, visArg(it.Visibility), vtype, value, cols, nullInt(it.UpdatedBy), fmtTime(it.UpdatedAt), it.ID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete removes an item with its rows and asset references.
func (r Data) Delete(ctx context.Context, id int64) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM data_items WHERE id = ?`, id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// GetByKey returns one item or ErrNotFound.
func (r Data) GetByKey(ctx context.Context, key string) (domain.DataItem, error) {
	return scanDataItem(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+dataColumns+` FROM data_items i WHERE i.key = ?`, key))
}

// visArg stores an unset visibility as admins, the most private level.
func visArg(v domain.Visibility) string {
	if v == "" {
		return string(domain.VisAdmins)
	}
	return string(v)
}

// List returns items ordered by key: those with one of the given
// visibilities, or every item when only is empty.
func (r Data) List(ctx context.Context, only ...domain.Visibility) ([]domain.DataItem, error) {
	q := `SELECT ` + dataColumns + ` FROM data_items i`
	args := make([]any, 0, len(only))
	if len(only) > 0 {
		q += ` WHERE i.visibility IN (?` + strings.Repeat(`, ?`, len(only)-1) + `)`
		for _, v := range only {
			args = append(args, string(v))
		}
	}
	rows, err := r.DB.Q(ctx).QueryContext(ctx, q+` ORDER BY i.key`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []domain.DataItem{}
	for rows.Next() {
		it, err := scanDataItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Rows returns a table's rows by position, then id.
func (r Data) Rows(ctx context.Context, itemID int64) ([]domain.DataRow, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT id, position, cells_json FROM data_rows
		WHERE item_id = ? ORDER BY position, id`, itemID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []domain.DataRow{}
	for rows.Next() {
		var (
			row   domain.DataRow
			cells string
		)
		if err := rows.Scan(&row.ID, &row.Position, &cells); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(cells), &row.Cells); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// PutRows inserts rows without an ID (setting it) and updates the others;
// ErrNotFound when an ID is not a row of this item.
func (r Data) PutRows(ctx context.Context, itemID int64, rows []domain.DataRow) error {
	q := r.DB.Q(ctx)
	for i := range rows {
		cells := rows[i].Cells
		if cells == nil {
			cells = map[string]json.RawMessage{}
		}
		b, err := json.Marshal(cells)
		if err != nil {
			return err
		}
		if rows[i].ID == 0 {
			res, err := q.ExecContext(ctx, `INSERT INTO data_rows (item_id, position, cells_json) VALUES (?, ?, ?)`,
				itemID, rows[i].Position, string(b))
			if err != nil {
				return mapErr(err)
			}
			if rows[i].ID, err = res.LastInsertId(); err != nil {
				return err
			}
			continue
		}
		res, err := q.ExecContext(ctx, `UPDATE data_rows SET position = ?, cells_json = ? WHERE id = ? AND item_id = ?`,
			rows[i].Position, string(b), rows[i].ID, itemID)
		if err != nil {
			return mapErr(err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return domain.ErrNotFound
		}
	}
	return nil
}

// DeleteRows removes the given rows of an item; unknown ids are ignored.
func (r Data) DeleteRows(ctx context.Context, itemID int64, ids []int64) error {
	for _, id := range ids {
		if _, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM data_rows WHERE id = ? AND item_id = ?`, id, itemID); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

// SetAssetRefs replaces the assets an item refers to.
func (r Data) SetAssetRefs(ctx context.Context, itemID int64, assetIDs []int64) error {
	q := r.DB.Q(ctx)
	if _, err := q.ExecContext(ctx, `DELETE FROM data_asset_refs WHERE item_id = ?`, itemID); err != nil {
		return err
	}
	for _, id := range assetIDs {
		if _, err := q.ExecContext(ctx, `INSERT OR IGNORE INTO data_asset_refs (item_id, asset_id) VALUES (?, ?)`,
			itemID, id); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

// AssetRefs returns the visibility and owner of every item that refers to the asset.
func (r Data) AssetRefs(ctx context.Context, assetID int64) ([]domain.AssetRef, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT i.visibility, coalesce(i.owner, '') FROM data_asset_refs a
		JOIN data_items i ON i.id = a.item_id WHERE a.asset_id = ?`, assetID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.AssetRef
	for rows.Next() {
		var (
			ref domain.AssetRef
			vis string
		)
		if err := rows.Scan(&vis, &ref.Owner); err != nil {
			return nil, err
		}
		ref.Visibility = domain.Visibility(vis)
		out = append(out, ref)
	}
	return out, rows.Err()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// AssetIsPublic reports whether a public item refers to this asset.
func (r Data) AssetIsPublic(ctx context.Context, assetID int64) (bool, error) {
	var n int
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT count(*) FROM data_asset_refs a JOIN data_items i ON i.id = a.item_id
		WHERE a.asset_id = ? AND i.visibility = 'public'`, assetID).Scan(&n)
	return n > 0, err
}
