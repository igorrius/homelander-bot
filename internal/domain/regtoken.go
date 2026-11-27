package domain

import (
	"context"
	"time"
)

// RegistrationToken is a link the bot handed out to apply with a proven phone.
type RegistrationToken struct {
	ID                                               int64
	TokenHash                                        []byte
	ChannelType, ExternalID, ChatID, Username, Phone string
	ExpiresAt                                        time.Time
	UsedAt                                           *time.Time
	CreatedAt                                        time.Time
}

// RegTokenRepo stores registration tokens.
type RegTokenRepo interface {
	Create(ctx context.Context, t *RegistrationToken) error
	GetByHash(ctx context.Context, hash []byte) (RegistrationToken, error)
	MarkUsed(ctx context.Context, id int64, at time.Time) error
	// InvalidateFor marks every unused token of a messenger account used.
	InvalidateFor(ctx context.Context, channelType, externalID string, at time.Time) error
}
