package domain

import (
	"context"
	"time"
)

// Destination is a place a messenger can post to: a chat, or a topic inside it.
type Destination struct {
	ChatID   string // messenger's chat id
	ThreadID string // forum topic; "" = the chat itself (General in a forum)
}

// LinkButton is a URL button under a message.
type LinkButton struct{ Text, URL string }

// OutMessage is a message to send, already localized.
type OutMessage struct {
	Text    string
	Buttons []LinkButton
}

// OutboxState is where a queued message stands.
type OutboxState string

// Outbox states.
const (
	OutboxPending OutboxState = "pending"
	OutboxSent    OutboxState = "sent"
	OutboxFailed  OutboxState = "failed"
)

// OutboxItem is one message to one destination.
type OutboxItem struct {
	ID              int64
	Messenger       string
	To              Destination
	Msg             OutMessage
	Origin          string // e.g. "application:42", "test"
	LabelID, UserID *int64
	State           OutboxState
	Attempts        int
	NextAttemptAt   time.Time
	LastError       string
	CreatedAt       time.Time
	SentAt          *time.Time
}

// OutboxRepo stores queued messages.
type OutboxRepo interface {
	Add(ctx context.Context, it *OutboxItem) error
	Get(ctx context.Context, id int64) (OutboxItem, error)
	// Due returns the oldest pending row of each chat whose next attempt has
	// come, oldest first: one chat's messages go out in order.
	Due(ctx context.Context, now time.Time, limit int) ([]OutboxItem, error)
	Update(ctx context.Context, it *OutboxItem) error
	// RepointChat moves pending rows from a chat's old id to its new one.
	RepointChat(ctx context.Context, messenger, oldChatID, newChatID string) error
	// DeleteFinishedBefore removes sent and failed rows created before t.
	DeleteFinishedBefore(ctx context.Context, t time.Time) (int, error)
}

// NotificationSettings says where the core's own notices go.
type NotificationSettings struct {
	ApplicationsLabelID *int64
}

// NotificationRepo stores NotificationSettings.
type NotificationRepo interface {
	Get(ctx context.Context) (NotificationSettings, error) // zero value when unset
	Save(ctx context.Context, s NotificationSettings) error
}
