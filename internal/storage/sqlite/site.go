package sqlite

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Site implements domain.SiteRepo. Settings live as one JSON value under the
// "site" key of the settings table.
type Site struct{ DB DB }

var _ domain.SiteRepo = Site{}

type siteJSON struct {
	CommunityName  domain.LocalizedText `json:"communityName"`
	WelcomeText    domain.LocalizedText `json:"welcomeText"`
	DefaultLocale  string               `json:"defaultLocale"`
	PictureAssetID *int64               `json:"pictureAssetId"`
}

// Get returns the site settings, with empty maps when nothing was saved yet.
func (r Site) Get(ctx context.Context) (domain.SiteSettings, error) {
	var raw string
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'site'`).Scan(&raw)
	var v siteJSON
	if err == nil {
		err = json.Unmarshal([]byte(raw), &v)
	} else if errors.Is(mapErr(err), domain.ErrNotFound) {
		err = nil
	}
	if v.CommunityName == nil {
		v.CommunityName = domain.LocalizedText{}
	}
	if v.WelcomeText == nil {
		v.WelcomeText = domain.LocalizedText{}
	}
	return domain.SiteSettings(v), err
}

// Save replaces the site settings.
func (r Site) Save(ctx context.Context, s domain.SiteSettings) error {
	raw, err := json.Marshal(siteJSON(s))
	if err != nil {
		return err
	}
	_, err = r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES ('site', ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, string(raw), fmtTime(now()))
	return err
}

// PutAsset stores a file.
func (r Site) PutAsset(ctx context.Context, a *domain.Asset) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO assets (kind, content_type, data, sha256, uploaded_by, uploaded_at)
		VALUES (?, ?, ?, ?, ?, ?)`, a.Kind, a.ContentType, a.Data, a.SHA256, a.UploadedBy, fmtTime(a.UploadedAt))
	if err != nil {
		return mapErr(err)
	}
	a.ID, err = res.LastInsertId()
	return err
}

// GetAsset returns a stored file or domain.ErrNotFound.
func (r Site) GetAsset(ctx context.Context, id int64) (domain.Asset, error) {
	var a domain.Asset
	var at string
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT id, kind, content_type, data, sha256, uploaded_by, uploaded_at
		FROM assets WHERE id = ?`, id).Scan(&a.ID, &a.Kind, &a.ContentType, &a.Data, &a.SHA256, &a.UploadedBy, &at)
	if err != nil {
		return a, mapErr(err)
	}
	a.UploadedAt = parseTime(at)
	return a, nil
}

// DeleteAsset removes a stored file.
func (r Site) DeleteAsset(ctx context.Context, id int64) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM assets WHERE id = ?`, id)
	return err
}
