package domain

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// LoginLinkTTL is how long a login link from the bot works.
const LoginLinkTTL = 5 * time.Minute

// LoginLink is a one-time link that logs a user in and opens a cabinet page;
// only the token's hash is kept.
type LoginLink struct {
	ID        int64
	UserID    int64
	TokenHash []byte
	Next      string // cabinet path, see ValidCabinetPath
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// LoginLinkRepo stores login links.
type LoginLinkRepo interface {
	Create(ctx context.Context, l *LoginLink) error
	GetByHash(ctx context.Context, hash []byte) (LoginLink, error)
	// MarkUsed reports false when the link was already used.
	MarkUsed(ctx context.Context, id int64, at time.Time) (bool, error)
	DeleteExpiredBefore(ctx context.Context, t time.Time) (int, error)
}

// ValidCabinetPath reports whether p is a same-site path inside the resident
// cabinet: "/my", "/my/…", "/my?…" or "/my#…". Anything that could lead out
// of /my once a browser resolves or decodes it is refused: a double slash
// (another host), a backslash, "." or ".." segments, percent-encoding, and
// spaces or control characters. Callers may build paths like
// "/my/feed/<id>" from their own values.
func ValidCabinetPath(p string) bool {
	if p != "/my" && !strings.HasPrefix(p, "/my/") && !strings.HasPrefix(p, "/my?") && !strings.HasPrefix(p, "/my#") {
		return false
	}
	if strings.Contains(p, "//") || strings.ContainsAny(p, `\%`) {
		return false
	}
	for _, r := range p {
		if r <= ' ' || r == 0x7f {
			return false
		}
	}
	path, _, _ := strings.Cut(p, "?")
	path, _, _ = strings.Cut(path, "#")
	for _, seg := range strings.Split(path, "/") {
		if seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

const (
	loginLinkLimit  = 10 // links minted per user per hour
	loginLinkWindow = time.Hour
	linkIPLimit     = 20 // links redeemed per IP per 15 minutes
)

// CreateLoginLink mints a one-time link that logs an active user in and opens
// next (a cabinet path). It is for messages to that user alone: group chats
// get plain /my addresses, never a link.
func (s *AuthService) CreateLoginLink(ctx context.Context, userID int64, next string) (string, error) {
	if !ValidCabinetPath(next) {
		return "", &ValidationError{Fields: map[string]string{"next": "invalid"}}
	}
	u, err := s.R.Users.Get(ctx, userID)
	if err != nil {
		return "", err
	}
	if u.Status != StatusActive {
		return "", ErrNotFound
	}
	if !s.Limits.Allow("loginlink:"+strconv.FormatInt(u.ID, 10), loginLinkLimit, loginLinkWindow) {
		return "", ErrRateLimited
	}
	token, err := s.newToken()
	if err != nil {
		return "", err
	}
	now := s.Now()
	l := LoginLink{UserID: u.ID, TokenHash: HashSecret(s.Secret, token), Next: next, ExpiresAt: now.Add(LoginLinkTTL), CreatedAt: now}
	if err := s.R.LoginLinks.Create(ctx, &l); err != nil {
		return "", err
	}
	return strings.TrimRight(s.PublicURL, "/") + "/login/link?t=" + token, nil
}

// CabinetLink returns the user of a linked messenger account and a login link
// to their cabinet home. ErrNotFound: the account is not linked to an active user.
func (s *AuthService) CabinetLink(ctx context.Context, channelType, externalID string) (User, string, error) {
	ch, err := s.R.Channels.GetByExternal(ctx, channelType, externalID)
	if err != nil {
		return User{}, "", err
	}
	u, err := s.R.Users.Get(ctx, ch.UserID)
	if err != nil {
		return User{}, "", err
	}
	if u.Status != StatusActive {
		return User{}, "", ErrNotFound
	}
	link, err := s.CreateLoginLink(ctx, u.ID, "/my")
	return u, link, err
}

// UseLoginLink spends a login link and opens a session. It returns the user,
// the raw session token and the cabinet path to open. A link that is unknown,
// used, expired or whose user is no longer active gives ErrTokenInvalid.
//
// When actor is signed in as another user, the link is left unspent and
// ErrLinkOtherUser is returned unless switchUser says the person confirmed
// the switch: a forwarded link must not quietly move someone into the
// sender's account (login CSRF).
func (s *AuthService) UseLoginLink(ctx context.Context, token, userAgent string, actor Actor, switchUser bool) (User, string, string, error) {
	if !s.Limits.Allow("linkip:"+actor.IP, linkIPLimit, codeWindow) {
		return User{}, "", "", ErrRateLimited
	}
	if token == "" {
		return User{}, "", "", ErrTokenInvalid
	}
	var (
		u       User
		session string
		next    string
	)
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		l, err := s.R.LoginLinks.GetByHash(ctx, HashSecret(s.Secret, token))
		if errors.Is(err, ErrNotFound) {
			return ErrTokenInvalid
		} else if err != nil {
			return err
		}
		now := s.Now()
		if l.UsedAt != nil || !now.Before(l.ExpiresAt) {
			return ErrTokenInvalid
		}
		if actor.UserID != nil && *actor.UserID != l.UserID && !switchUser {
			return ErrLinkOtherUser
		}
		if ok, err := s.R.LoginLinks.MarkUsed(ctx, l.ID, now); err != nil {
			return err
		} else if !ok {
			return ErrTokenInvalid
		}
		u, err = s.R.Users.Get(ctx, l.UserID)
		if errors.Is(err, ErrNotFound) || (err == nil && u.Status != StatusActive) {
			return ErrTokenInvalid
		} else if err != nil {
			return err
		}
		next = l.Next
		session, err = s.openSession(ctx, u, userAgent, actor, map[string]any{"via": "bot_link"})
		return err
	})
	if err != nil {
		return User{}, "", "", err
	}
	return u, session, next, nil
}

// CleanLoginLinks deletes expired login links.
func (s *AuthService) CleanLoginLinks(ctx context.Context) (int, error) {
	return s.R.LoginLinks.DeleteExpiredBefore(ctx, s.Now())
}
