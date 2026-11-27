package domain

import (
	"context"
	"time"
)

// Label is an admin-defined tag; whoever sends writes to a label, and the
// system delivers to every accepted place carrying it.
type Label struct {
	ID                   int64
	Name, Description    string
	CreatedAt, UpdatedAt time.Time
}

// Label target types.
const (
	TargetChat  = "chat"
	TargetTopic = "chat_topic"
)

// LabelTarget is something a label hangs on.
type LabelTarget struct {
	Type string
	ID   int64
}

// LabelRepo stores labels and where they hang.
type LabelRepo interface {
	// Create inserts l; ErrConflict when a label with the same case-folded name exists.
	Create(ctx context.Context, l *Label) error
	Update(ctx context.Context, l *Label) error
	Get(ctx context.Context, id int64) (Label, error)
	List(ctx context.Context) ([]Label, error)
	Delete(ctx context.Context, id int64) error
	Targets(ctx context.Context, labelID int64) ([]LabelTarget, error)
	LabelsOf(ctx context.Context, t LabelTarget) ([]int64, error)
	SetLabelsOf(ctx context.Context, t LabelTarget, labelIDs []int64) error
	// Retarget moves every binding of from to to, without duplicates.
	Retarget(ctx context.Context, from, to LabelTarget) error
	DeleteTarget(ctx context.Context, t LabelTarget) error
}
