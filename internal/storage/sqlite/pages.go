package sqlite

import (
	"context"
	"database/sql"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Pages implements domain.PageRepo.
type Pages struct{ DB DB }

var _ domain.PageRepo = Pages{}

const pageColumns = `id, status, note, created_by, published_by, created_at, updated_at, published_at`

func scanPage(s scanner) (domain.PageVersion, error) {
	var (
		v                      domain.PageVersion
		status                 string
		createdBy, publishedBy sql.NullInt64
		created, updated       string
		published              sql.NullString
	)
	if err := s.Scan(&v.ID, &status, &v.Note, &createdBy, &publishedBy, &created, &updated, &published); err != nil {
		return domain.PageVersion{}, mapErr(err)
	}
	v.Status = domain.PageStatus(status)
	v.CreatedBy, v.PublishedBy = intPtr(createdBy), intPtr(publishedBy)
	v.CreatedAt, v.UpdatedAt, v.PublishedAt = parseTime(created), parseTime(updated), parseTimePtr(published)
	return v, nil
}

// Create inserts v and sets its ID.
func (r Pages) Create(ctx context.Context, v *domain.PageVersion) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO page_versions (status, note, created_by, published_by,
		created_at, updated_at, published_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		string(v.Status), v.Note, nullInt(v.CreatedBy), nullInt(v.PublishedBy),
		fmtTime(v.CreatedAt), fmtTime(v.UpdatedAt), fmtTimePtr(v.PublishedAt))
	if err != nil {
		return mapErr(err)
	}
	v.ID, err = res.LastInsertId()
	return err
}

// Update writes the status, note, publication fields and updated_at;
// ErrConflict when it would make a second published version.
func (r Pages) Update(ctx context.Context, v *domain.PageVersion) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE page_versions SET status = ?, note = ?, published_by = ?,
		published_at = ?, updated_at = ? WHERE id = ?`,
		string(v.Status), v.Note, nullInt(v.PublishedBy), fmtTimePtr(v.PublishedAt), fmtTime(v.UpdatedAt), v.ID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete removes a version with its files.
func (r Pages) Delete(ctx context.Context, id int64) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM page_versions WHERE id = ?`, id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Get returns one version with its file metadata.
func (r Pages) Get(ctx context.Context, id int64) (domain.PageVersion, error) {
	v, err := scanPage(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+pageColumns+` FROM page_versions WHERE id = ?`, id))
	if err != nil {
		return v, err
	}
	return v, r.withFiles(ctx, &v)
}

// Published returns the published version or ErrNotFound.
func (r Pages) Published(ctx context.Context) (domain.PageVersion, error) {
	v, err := scanPage(r.DB.Q(ctx).QueryRowContext(ctx,
		`SELECT `+pageColumns+` FROM page_versions WHERE status = 'published'`))
	if err != nil {
		return v, err
	}
	return v, r.withFiles(ctx, &v)
}

// List returns every version, newest first, with file metadata.
func (r Pages) List(ctx context.Context) ([]domain.PageVersion, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT `+pageColumns+` FROM page_versions ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	out := []domain.PageVersion{}
	for rows.Next() {
		v, err := scanPage(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := r.withFiles(ctx, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r Pages) withFiles(ctx context.Context, v *domain.PageVersion) error {
	files, err := r.files(ctx, v.ID, false)
	if err != nil {
		return err
	}
	v.Files, v.TotalSize = files, 0
	for _, f := range files {
		v.TotalSize += f.Size
	}
	return nil
}

func (r Pages) files(ctx context.Context, id int64, withData bool) ([]domain.PageFile, error) {
	data := `NULL`
	if withData {
		data = `data`
	}
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT path, content_type, `+data+`, sha256, size
		FROM page_files WHERE version_id = ? ORDER BY path`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []domain.PageFile{}
	for rows.Next() {
		var f domain.PageFile
		if err := rows.Scan(&f.Path, &f.ContentType, &f.Data, &f.SHA256, &f.Size); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// PutFiles inserts files, replacing those with the same path.
func (r Pages) PutFiles(ctx context.Context, id int64, files []domain.PageFile) error {
	for _, f := range files {
		data := f.Data
		if data == nil {
			data = []byte{}
		}
		if _, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT OR REPLACE INTO page_files
			(version_id, path, content_type, data, sha256, size) VALUES (?, ?, ?, ?, ?, ?)`,
			id, f.Path, f.ContentType, data, f.SHA256, f.Size); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

// DeleteFiles removes the given paths; unknown paths are ignored.
func (r Pages) DeleteFiles(ctx context.Context, id int64, paths []string) error {
	for _, p := range paths {
		if _, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM page_files WHERE version_id = ? AND path = ?`,
			id, p); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

// File returns one file with its bytes, or ErrNotFound.
func (r Pages) File(ctx context.Context, id int64, path string) (domain.PageFile, error) {
	var f domain.PageFile
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT path, content_type, data, sha256, size FROM page_files
		WHERE version_id = ? AND path = ?`, id, path).Scan(&f.Path, &f.ContentType, &f.Data, &f.SHA256, &f.Size)
	return f, mapErr(err)
}

// Files returns every file of a version with its bytes, sorted by path.
func (r Pages) Files(ctx context.Context, id int64) ([]domain.PageFile, error) {
	return r.files(ctx, id, true)
}
