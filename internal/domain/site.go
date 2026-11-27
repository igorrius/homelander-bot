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
	// HomeItems are the public data items shown as cards on the built-in
	// page, in order. A key that is not (or no longer) public is skipped there.
	HomeItems []HomeItem
}

// HomeItem is one card of the built-in page: a data key and its card title
// (empty: the page derives one from the key).
type HomeItem struct {
	Key   string
	Title LocalizedText
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
