package domain

import (
	"context"
	"fmt"
	"time"
)

// Messenger delivers texts to users and chats. Texts arrive already
// localized. Implemented by internal/messenger/telegram.
type Messenger interface {
	Type() string
	// SendText sends a plain text (login codes).
	SendText(ctx context.Context, chatID, text string) error
	// Send posts m to a chat or topic; a failure is a *SendError.
	Send(ctx context.Context, to Destination, m OutMessage) error
	// LeaveChat makes the bot leave a chat; nil when it is already not there.
	LeaveChat(ctx context.Context, chatID string) error
	BotURL() string
}

// SendErrorKind tells the outbox what to do about a failed send.
type SendErrorKind string

// Send error kinds.
const (
	SendRetry      SendErrorKind = "retry"       // rate limit, server or network trouble
	SendChatGone   SendErrorKind = "chat_gone"   // the bot was removed or the chat does not exist
	SendThreadGone SendErrorKind = "thread_gone" // the forum topic was deleted
	SendChatMoved  SendErrorKind = "chat_moved"  // the chat has a new id (NewChatID)
	SendPermanent  SendErrorKind = "permanent"   // anything else that will not get better
)

// SendError is a classified send failure.
type SendError struct {
	Kind       SendErrorKind
	RetryAfter time.Duration
	NewChatID  string
	Err        error
}

func (e *SendError) Error() string { return fmt.Sprintf("send %s: %v", e.Kind, e.Err) }

// Unwrap returns the messenger's own error.
func (e *SendError) Unwrap() error { return e.Err }

// Messengers looks a messenger up by its type.
type Messengers map[string]Messenger

// Get returns the messenger of a type; ErrNotFound for an unknown one.
func (m Messengers) Get(typ string) (Messenger, error) {
	if v, ok := m[typ]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("messenger %q: %w", typ, ErrNotFound)
}

// Translator renders localized texts (implemented by internal/i18n.Bundle).
type Translator interface {
	T(locale, id string, data map[string]any, count any) string
	Pick(candidates ...string) string
}
