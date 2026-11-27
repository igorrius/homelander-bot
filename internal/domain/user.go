package domain

import (
	"context"
	"time"
)

// UserStatus is where a user is on the way from application to active account.
type UserStatus string

// User statuses (spec 5.2).
const (
	StatusPendingApproval    UserStatus = "pending_approval"
	StatusAwaitingActivation UserStatus = "awaiting_activation"
	StatusActive             UserStatus = "active"
	StatusRejected           UserStatus = "rejected"
	StatusDisabled           UserStatus = "disabled"
)

// UserSource tells how a user came into the system.
type UserSource string

// User sources.
const (
	SourceApplication UserSource = "application"
	SourceAdmin       UserSource = "admin"
	SourceBootstrap   UserSource = "bootstrap"
)

// User is a person known to the system.
type User struct {
	ID          int64
	Phone       string // E.164
	FirstName   string
	LastName    string
	Locale      string
	IsResident  bool
	IsAdmin     bool
	Status      UserStatus
	Source      UserSource
	ApprovedBy  *int64
	ApprovedAt  *time.Time
	ActivatedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Channel is a linked messenger account of a user.
type Channel struct {
	ID         int64
	UserID     int64
	Type       string // "telegram"
	ExternalID string // the messenger's user id
	ChatID     string // where the bot writes to the user
	Username   string
	LinkedAt   time.Time
}

// UserFilter narrows a user listing.
type UserFilter struct {
	Query      string
	HouseID    *int64
	Status     *UserStatus
	IsAdmin    *bool
	IsResident *bool
	Limit      int
	Offset     int
}

// UserRepo stores users.
type UserRepo interface {
	Create(ctx context.Context, u *User) error
	Update(ctx context.Context, u *User) error
	Get(ctx context.Context, id int64) (User, error)
	GetByPhone(ctx context.Context, phone string) (User, error)
	List(ctx context.Context, f UserFilter) ([]User, int, error)
	CountActiveAdmins(ctx context.Context) (int, error)
	CountByStatus(ctx context.Context, s UserStatus) (int, error)
}

// ChannelRepo stores messenger channels.
type ChannelRepo interface {
	Create(ctx context.Context, c *Channel) error
	ListByUser(ctx context.Context, userID int64) ([]Channel, error)
	GetByExternal(ctx context.Context, typ, externalID string) (Channel, error)
	Delete(ctx context.Context, id int64) error
}
