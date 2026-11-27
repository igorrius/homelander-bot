package domain

import (
	"context"
	"time"
)

// SiteSettings is what the resident page shows, all entered in the admin panel.
type SiteSettings struct {
	CommunityName  LocalizedText
	WelcomeText    LocalizedText
	DefaultLocale  string
	PictureAssetID *int64
}

// Asset is an uploaded file kept in the database.
type Asset struct {
	ID          int64
	Kind        string
	ContentType string
	Data        []byte
	SHA256      string
	UploadedBy  int64
	UploadedAt  time.Time
}

// SiteRepo stores site settings and assets.
type SiteRepo interface {
	Get(ctx context.Context) (SiteSettings, error)
	Save(ctx context.Context, s SiteSettings) error
	PutAsset(ctx context.Context, a *Asset) error
	GetAsset(ctx context.Context, id int64) (Asset, error)
	DeleteAsset(ctx context.Context, id int64) error
}
