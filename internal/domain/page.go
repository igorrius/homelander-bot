package domain

import (
	"context"
	"time"
)

// PageStatus is where a home page version is in its life.
type PageStatus string

// Page statuses: drafts are editable, published and archived ones are not.
const (
	PageDraft     PageStatus = "draft"
	PagePublished PageStatus = "published"
	PageArchived  PageStatus = "archived"
)

// PageFile is one file of a page version.
type PageFile struct {
	Path        string
	ContentType string
	Data        []byte // empty unless loaded with File or Files
	SHA256      string
	Size        int
}

// PageVersion is one uploaded set of home page files.
type PageVersion struct {
	ID          int64
	Status      PageStatus
	Note        string
	CreatedBy   *int64
	CreatedVia  string // the agent's client name when written through MCP
	CreatedAt   time.Time
	UpdatedAt   time.Time
	PublishedBy *int64
	PublishedAt *time.Time
	Files       []PageFile // metadata only, sorted by path
	TotalSize   int
}

// PageRepo stores page versions and their files.
type PageRepo interface {
	Create(ctx context.Context, v *PageVersion) error
	// Update writes status, note, published_* and updated_at; ErrConflict on a
	// second published version.
	Update(ctx context.Context, v *PageVersion) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (PageVersion, error)
	List(ctx context.Context) ([]PageVersion, error) // newest first
	Published(ctx context.Context) (PageVersion, error)
	PutFiles(ctx context.Context, id int64, files []PageFile) error
	DeleteFiles(ctx context.Context, id int64, paths []string) error
	File(ctx context.Context, id int64, path string) (PageFile, error)
	Files(ctx context.Context, id int64) ([]PageFile, error)
}
