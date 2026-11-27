package domain

import (
	"context"
	"time"
)

// LoginCode is a one-time code sent to a user's messenger; only its hash is kept.
type LoginCode struct {
	ID        int64
	UserID    int64
	CodeHash  []byte
	ExpiresAt time.Time
	Attempts  int
	UsedAt    *time.Time
	IP        string
}

// Session is a signed-in browser; the cookie holds the raw token, the store its hash.
type Session struct {
	ID         int64
	UserID     int64
	TokenHash  []byte
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	IP         string
	UserAgent  string
}

// CodeRepo stores login codes.
type CodeRepo interface {
	// Replace deletes the user's unused codes and inserts c.
	Replace(ctx context.Context, c *LoginCode) error
	// Active returns the user's unused, unexpired code, or ErrNotFound.
	Active(ctx context.Context, userID int64, now time.Time) (LoginCode, error)
	IncAttempts(ctx context.Context, id int64) (int, error)
	// MarkUsed reports false when the code was already used.
	MarkUsed(ctx context.Context, id int64, at time.Time) (bool, error)
}

// SessionRepo stores sessions.
type SessionRepo interface {
	Create(ctx context.Context, s *Session) error
	GetByHash(ctx context.Context, hash []byte) (Session, error)
	Touch(ctx context.Context, id int64, at time.Time) error
	Delete(ctx context.Context, id int64) error
	DeleteByUser(ctx context.Context, userID int64) error
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
}
