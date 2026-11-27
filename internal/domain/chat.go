package domain

import (
	"context"
	"time"
)

// ChatState is where a chat stands with the system.
type ChatState string

// Chat states: new (the bot is there, nobody confirmed it), accepted (receives
// messages), left (the bot is not there any more).
const (
	ChatNew      ChatState = "new"
	ChatAccepted ChatState = "accepted"
	ChatLeft     ChatState = "left"
)

// Chat kinds and bot statuses as stored.
const (
	ChatKindGroup      = "group"
	ChatKindSupergroup = "supergroup"
	ChatKindChannel    = "channel"

	BotMember        = "member"
	BotAdministrator = "administrator"
	BotRestricted    = "restricted"
	BotUnknown       = "unknown"
)

// ChatInfo is what a messenger reports about a chat.
type ChatInfo struct {
	Messenger, ExternalID, Kind, Title string
	IsForum                            bool
}

// BotRights is the bot's own membership in a chat.
type BotRights struct {
	Status  string // member|administrator|restricted|unknown
	CanSend bool
}

// Chat is a group, forum or channel the bot is or was in.
type Chat struct {
	ID                                 int64
	Messenger, ExternalID, Kind, Title string
	IsForum                            bool
	State                              ChatState
	BotStatus                          string
	CanSend                            bool
	AddedByExternalID                  string
	AddedByUserID, AcceptedBy          *int64
	AcceptedAt, LeaveAt, LeftAt        *time.Time
	FirstSeenAt, LastSeenAt            time.Time
}

// Topic is a forum topic inside a chat. The General topic is the chat itself.
type Topic struct {
	ID, ChatID              int64
	ExternalID, Title       string
	TitleSetByAdmin, Gone   bool
	FirstSeenAt, LastSeenAt time.Time
}

// ChatRepo stores chats.
type ChatRepo interface {
	Create(ctx context.Context, c *Chat) error
	Update(ctx context.Context, c *Chat) error
	Get(ctx context.Context, id int64) (Chat, error)
	GetByExternal(ctx context.Context, messenger, externalID string) (Chat, error)
	// List returns new chats first, then accepted, then left; by title within each.
	List(ctx context.Context) ([]Chat, error)
	Delete(ctx context.Context, id int64) error
	// DueLeaves returns new chats whose leave_at has passed.
	DueLeaves(ctx context.Context, now time.Time) ([]Chat, error)
	CountByState(ctx context.Context, s ChatState) (int, error)
}

// TopicRepo stores forum topics.
type TopicRepo interface {
	Create(ctx context.Context, t *Topic) error
	Update(ctx context.Context, t *Topic) error
	Get(ctx context.Context, id int64) (Topic, error)
	GetByExternal(ctx context.Context, chatID int64, externalID string) (Topic, error)
	ListByChat(ctx context.Context, chatID int64) ([]Topic, error)
	// MoveToChat moves topics to another chat, leaving behind those whose
	// external id already exists there.
	MoveToChat(ctx context.Context, fromChatID, toChatID int64) error
}
