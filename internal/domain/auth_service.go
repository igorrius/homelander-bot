package domain

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/igorrius/homelander-bot/internal/ratelimit"
)

// ActivationResult is what happened when someone shared their phone in the bot.
type ActivationResult string

// Activation results.
const (
	Activated        ActivationResult = "activated"
	AlreadyConnected ActivationResult = "already_connected"
	PendingApproval  ActivationResult = "pending_approval"
	NotRegistered    ActivationResult = "not_registered"
	BootstrapAdmin   ActivationResult = "bootstrap_admin"
)

// AuthService activates messenger channels, sends login codes and manages sessions.
type AuthService struct {
	R              Repos
	Now            Clock
	Limits         *ratelimit.Limiter
	Secret         []byte
	Region         string
	Messenger      Messenger
	I18n           Translator
	BootstrapPhone string
	CodeTTL        time.Duration
	MaxAttempts    int
	IdleTTL        time.Duration
	MaxTTL         time.Duration
	Rand           io.Reader // nil means crypto/rand.Reader
}

const (
	activateLimit  = 5
	activateWindow = time.Hour
	codePhoneLimit = 3
	codeIPLimit    = 10
	verifyIPLimit  = 20
	codeWindow     = 15 * time.Minute
	tokenBytes     = 32
	touchEvery     = time.Minute
)

// HashSecret returns HMAC-SHA256(secret, value).
func HashSecret(secret []byte, value string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(value))
	return m.Sum(nil)
}

func (s *AuthService) rand() io.Reader {
	if s.Rand != nil {
		return s.Rand
	}
	return rand.Reader
}

func (s *AuthService) audit(ctx context.Context, a Actor, action, target string, details map[string]any) error {
	return s.R.Audit.Add(ctx, AuditEntry{At: s.Now(), ActorUserID: a.UserID, Action: action, Target: target, Details: details, IP: a.IP})
}

// ActivateChannel links a messenger account to the user whose phone the
// messenger verified. The caller has already checked that the person shared
// their own contact.
func (s *AuthService) ActivateChannel(ctx context.Context, typ, externalID, chatID, username, verifiedPhone string) (ActivationResult, User, error) {
	if !s.Limits.Allow("activate:"+typ+":"+externalID, activateLimit, activateWindow) {
		return "", User{}, ErrRateLimited
	}
	phone, err := NormalizePhone(verifiedPhone, s.Region)
	if err != nil {
		return NotRegistered, User{}, nil
	}
	var (
		result ActivationResult
		user   User
	)
	err = s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		existing, err := s.R.Channels.GetByExternal(ctx, typ, externalID)
		switch {
		case err == nil:
			u, err := s.R.Users.Get(ctx, existing.UserID)
			if err != nil {
				return err
			}
			result = NotRegistered
			if u.Phone == phone && u.Status == StatusActive {
				result, user = AlreadyConnected, u
			}
			return nil
		case !errors.Is(err, ErrNotFound):
			return err
		}

		if ok, err := s.bootstrapApplies(ctx, phone); err != nil {
			return err
		} else if ok {
			user, err = s.bootstrap(ctx, phone)
			if err != nil {
				return err
			}
			result = BootstrapAdmin
			return s.link(ctx, user, typ, externalID, chatID, username)
		}

		u, err := s.R.Users.GetByPhone(ctx, phone)
		if errors.Is(err, ErrNotFound) {
			result = NotRegistered
			return nil
		} else if err != nil {
			return err
		}
		switch u.Status {
		case StatusPendingApproval:
			result, user = PendingApproval, u
			return nil
		case StatusAwaitingActivation:
		default:
			result = NotRegistered
			return nil
		}
		chans, err := s.R.Channels.ListByUser(ctx, u.ID)
		if err != nil {
			return err
		}
		for _, c := range chans {
			if c.Type == typ {
				result = NotRegistered
				return nil
			}
		}
		now := s.Now()
		u.Status, u.ActivatedAt = StatusActive, &now
		if err := s.R.Users.Update(ctx, &u); err != nil {
			return err
		}
		result, user = Activated, u
		return s.link(ctx, u, typ, externalID, chatID, username)
	})
	if err != nil {
		return "", User{}, err
	}
	return result, user, nil
}

func (s *AuthService) link(ctx context.Context, u User, typ, externalID, chatID, username string) error {
	c := Channel{UserID: u.ID, Type: typ, ExternalID: externalID, ChatID: chatID, Username: username, LinkedAt: s.Now()}
	if err := s.R.Channels.Create(ctx, &c); err != nil {
		return err
	}
	return s.audit(ctx, Actor{UserID: &u.ID}, ActChannelLinked, UserTarget(u.ID), map[string]any{"type": typ})
}

func (s *AuthService) bootstrapApplies(ctx context.Context, phone string) (bool, error) {
	if s.BootstrapPhone == "" {
		return false, nil
	}
	want, err := NormalizePhone(s.BootstrapPhone, s.Region)
	if err != nil || want != phone {
		return false, nil
	}
	n, err := s.R.Users.CountActiveAdmins(ctx)
	return n == 0, err
}

// bootstrap makes the first admin from BootstrapPhone, reusing a row with that phone.
func (s *AuthService) bootstrap(ctx context.Context, phone string) (User, error) {
	now := s.Now()
	u, err := s.R.Users.GetByPhone(ctx, phone)
	switch {
	case err == nil:
		u.IsAdmin, u.Status, u.Source, u.ActivatedAt = true, StatusActive, SourceBootstrap, &now
		err = s.R.Users.Update(ctx, &u)
	case errors.Is(err, ErrNotFound):
		u = User{Phone: phone, IsAdmin: true, Status: StatusActive, Source: SourceBootstrap, ActivatedAt: &now}
		err = s.R.Users.Create(ctx, &u)
	}
	if err != nil {
		return User{}, err
	}
	return u, s.audit(ctx, Actor{UserID: &u.ID}, ActBootstrapAdmin, UserTarget(u.ID), nil)
}

func (s *AuthService) newCode() (string, error) {
	n, err := rand.Int(s.rand(), big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// RequestCode sends a login code to an active user's messenger. It answers
// nil whether or not a code was sent, so callers cannot learn who has access.
func (s *AuthService) RequestCode(ctx context.Context, rawPhone string, actor Actor) error {
	phone, err := NormalizePhone(rawPhone, s.Region)
	if err != nil {
		return &ValidationError{Fields: map[string]string{"phone": "invalid"}}
	}
	if !s.Limits.Allow("codeip:"+actor.IP, codeIPLimit, codeWindow) || !s.Limits.Allow("code:"+phone, codePhoneLimit, codeWindow) {
		return ErrRateLimited
	}
	u, err := s.R.Users.GetByPhone(ctx, phone)
	if errors.Is(err, ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	if u.Status != StatusActive {
		return nil
	}
	chans, err := s.R.Channels.ListByUser(ctx, u.ID)
	if err != nil {
		return err
	}
	var ch *Channel
	for i := range chans {
		if chans[i].Type == s.Messenger.Type() {
			ch = &chans[i]
		}
	}
	if ch == nil {
		return nil
	}
	code, err := s.newCode()
	if err != nil {
		return err
	}
	now := s.Now()
	lc := LoginCode{UserID: u.ID, CodeHash: HashSecret(s.Secret, code), ExpiresAt: now.Add(s.CodeTTL), IP: actor.IP}
	if err := s.R.Codes.Replace(ctx, &lc); err != nil {
		return err
	}
	text := s.I18n.T(s.I18n.Pick(u.Locale), "bot.login.code", map[string]any{"Code": code, "Time": now.UTC().Format("15:04") + " UTC"}, nil)
	if err := s.Messenger.SendText(ctx, ch.ChatID, text); err != nil {
		return fmt.Errorf("send login code: %w", err)
	}
	return s.audit(ctx, actor, ActAuthCodeSent, UserTarget(u.ID), nil)
}

// VerifyCode checks a login code and opens a session; it returns the raw session token.
func (s *AuthService) VerifyCode(ctx context.Context, rawPhone, code, userAgent string, actor Actor) (User, string, error) {
	if !s.Limits.Allow("verifyip:"+actor.IP, verifyIPLimit, codeWindow) {
		return User{}, "", ErrRateLimited
	}
	phone, err := NormalizePhone(rawPhone, s.Region)
	if err != nil {
		return User{}, "", ErrInvalidCode
	}
	u, err := s.R.Users.GetByPhone(ctx, phone)
	if errors.Is(err, ErrNotFound) || (err == nil && u.Status != StatusActive) {
		return User{}, "", ErrInvalidCode
	} else if err != nil {
		return User{}, "", err
	}
	now := s.Now()
	lc, err := s.R.Codes.Active(ctx, u.ID, now)
	if errors.Is(err, ErrNotFound) {
		return User{}, "", ErrInvalidCode
	} else if err != nil {
		return User{}, "", err
	}
	if !hmac.Equal(lc.CodeHash, HashSecret(s.Secret, strings.TrimSpace(code))) {
		n, err := s.R.Codes.IncAttempts(ctx, lc.ID)
		if err != nil {
			return User{}, "", err
		}
		if n >= s.MaxAttempts {
			if _, err := s.R.Codes.MarkUsed(ctx, lc.ID, now); err != nil {
				return User{}, "", err
			}
		}
		if err := s.audit(ctx, actor, ActAuthLoginFailed, UserTarget(u.ID), map[string]any{"attempts": n}); err != nil {
			return User{}, "", err
		}
		return User{}, "", ErrInvalidCode
	}
	if ok, err := s.R.Codes.MarkUsed(ctx, lc.ID, now); err != nil {
		return User{}, "", err
	} else if !ok {
		return User{}, "", ErrInvalidCode
	}
	raw := make([]byte, tokenBytes)
	if _, err := io.ReadFull(s.rand(), raw); err != nil {
		return User{}, "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sess := Session{UserID: u.ID, TokenHash: HashSecret(s.Secret, token), CreatedAt: now, LastSeenAt: now,
		ExpiresAt: now.Add(s.MaxTTL), IP: actor.IP, UserAgent: truncate(userAgent, 300)}
	if err := s.R.Sessions.Create(ctx, &sess); err != nil {
		return User{}, "", err
	}
	if err := s.audit(ctx, Actor{UserID: &u.ID, IP: actor.IP}, ActAuthLogin, UserTarget(u.ID), nil); err != nil {
		return User{}, "", err
	}
	return u, token, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// Authenticate resolves a session token to its user. Expired, idle or
// orphaned sessions are deleted and give ErrUnauthorized.
func (s *AuthService) Authenticate(ctx context.Context, rawToken string) (User, Session, error) {
	if rawToken == "" {
		return User{}, Session{}, ErrUnauthorized
	}
	sess, err := s.R.Sessions.GetByHash(ctx, HashSecret(s.Secret, rawToken))
	if errors.Is(err, ErrNotFound) {
		return User{}, Session{}, ErrUnauthorized
	} else if err != nil {
		return User{}, Session{}, err
	}
	now := s.Now()
	u, err := s.R.Users.Get(ctx, sess.UserID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return User{}, Session{}, err
	}
	if err != nil || u.Status != StatusActive || !now.Before(sess.ExpiresAt) || now.Sub(sess.LastSeenAt) > s.IdleTTL {
		if derr := s.R.Sessions.Delete(ctx, sess.ID); derr != nil {
			return User{}, Session{}, derr
		}
		return User{}, Session{}, ErrUnauthorized
	}
	if now.Sub(sess.LastSeenAt) >= touchEvery {
		if err := s.R.Sessions.Touch(ctx, sess.ID, now); err != nil {
			return User{}, Session{}, err
		}
		sess.LastSeenAt = now
	}
	return u, sess, nil
}

// Logout ends the session of rawToken.
func (s *AuthService) Logout(ctx context.Context, rawToken string, actor Actor) error {
	sess, err := s.R.Sessions.GetByHash(ctx, HashSecret(s.Secret, rawToken))
	if errors.Is(err, ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	if err := s.R.Sessions.Delete(ctx, sess.ID); err != nil {
		return err
	}
	return s.audit(ctx, Actor{UserID: &sess.UserID, IP: actor.IP}, ActAuthLogout, UserTarget(sess.UserID), nil)
}

// CleanExpired deletes sessions past their maximum lifetime.
func (s *AuthService) CleanExpired(ctx context.Context) (int, error) {
	return s.R.Sessions.DeleteExpired(ctx, s.Now())
}
