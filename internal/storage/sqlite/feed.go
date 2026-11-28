package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Feed implements domain.FeedRepo.
type Feed struct{ DB DB }

var _ domain.FeedRepo = Feed{}

const feedColumns = `id, body, image_asset_id, state, published_at, labels, created_by, updated_by, created_at, updated_at, deleted_at`

func scanPost(s scanner) (domain.FeedPost, error) {
	var (
		p                      domain.FeedPost
		image, by1, by2        sql.NullInt64
		published, deleted     sql.NullString
		state, labels, cr, upd string
	)
	if err := s.Scan(&p.ID, &p.Body, &image, &state, &published, &labels, &by1, &by2, &cr, &upd, &deleted); err != nil {
		return domain.FeedPost{}, mapErr(err)
	}
	if err := json.Unmarshal([]byte(labels), &p.LabelIDs); err != nil {
		return domain.FeedPost{}, err
	}
	if p.LabelIDs == nil {
		p.LabelIDs = []int64{}
	}
	p.ImageAssetID, p.State = intPtr(image), domain.PostState(state)
	p.PublishedAt, p.DeletedAt = parseTimePtr(published), parseTimePtr(deleted)
	p.CreatedBy, p.UpdatedBy = intPtr(by1), intPtr(by2)
	p.CreatedAt, p.UpdatedAt = parseTime(cr), parseTime(upd)
	return p, nil
}

func labelsArg(ids []int64) (string, error) {
	if ids == nil {
		ids = []int64{}
	}
	b, err := json.Marshal(ids)
	return string(b), err
}

// Create inserts p and sets its ID.
func (r Feed) Create(ctx context.Context, p *domain.FeedPost) error {
	labels, err := labelsArg(p.LabelIDs)
	if err != nil {
		return err
	}
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO feed_posts (body, image_asset_id, state, published_at, labels,
		created_by, updated_by, created_at, updated_at, deleted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Body, nullInt(p.ImageAssetID), string(p.State), fmtTimePtr(p.PublishedAt), labels,
		nullInt(p.CreatedBy), nullInt(p.UpdatedBy), fmtTime(p.CreatedAt), fmtTime(p.UpdatedAt), fmtTimePtr(p.DeletedAt))
	if err != nil {
		return mapErr(err)
	}
	p.ID, err = res.LastInsertId()
	return err
}

// Update writes everything but the creation fields.
func (r Feed) Update(ctx context.Context, p *domain.FeedPost) error {
	labels, err := labelsArg(p.LabelIDs)
	if err != nil {
		return err
	}
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE feed_posts SET body = ?, image_asset_id = ?, state = ?, published_at = ?,
		labels = ?, updated_by = ?, updated_at = ?, deleted_at = ? WHERE id = ?`,
		p.Body, nullInt(p.ImageAssetID), string(p.State), fmtTimePtr(p.PublishedAt), labels,
		nullInt(p.UpdatedBy), fmtTime(p.UpdatedAt), fmtTimePtr(p.DeletedAt), p.ID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Get returns a post that is not deleted.
func (r Feed) Get(ctx context.Context, id int64) (domain.FeedPost, error) {
	return scanPost(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+feedColumns+` FROM feed_posts WHERE id = ? AND deleted_at IS NULL`, id))
}

func (r Feed) query(ctx context.Context, q string, args ...any) ([]domain.FeedPost, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []domain.FeedPost{}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// List returns posts that are not deleted, newest first, and their number.
func (r Feed) List(ctx context.Context, limit, offset int) ([]domain.FeedPost, int, error) {
	var total int
	if err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT count(*) FROM feed_posts WHERE deleted_at IS NULL`).Scan(&total); err != nil {
		return nil, 0, err
	}
	out, err := r.query(ctx, `SELECT `+feedColumns+` FROM feed_posts WHERE deleted_at IS NULL ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	return out, total, err
}

// Published returns published, not deleted posts, newest published first.
func (r Feed) Published(ctx context.Context, after *domain.FeedCursor, limit int) ([]domain.FeedPost, error) {
	q := `SELECT ` + feedColumns + ` FROM feed_posts WHERE state = 'published' AND deleted_at IS NULL`
	args := []any{}
	if after != nil {
		q += ` AND (published_at < ? OR (published_at = ? AND id < ?))`
		at := fmtTime(after.At)
		args = append(args, at, at, after.ID)
	}
	return r.query(ctx, q+` ORDER BY published_at DESC, id DESC LIMIT ?`, append(args, limit)...)
}
