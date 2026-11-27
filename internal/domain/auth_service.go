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
	"log/slog"
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
	ApplyLink        ActivationResult = "apply_link"     // unknown or rejected phone: a registration link was issued
	PendingLinked    ActivationResult = "pending_linked" // pending application: the account was linked to it
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
	// Deliver runs code delivery; nil runs it in a new goroutine so the
	// answer to the caller does not depend on whether a code was sent.
	Deliver func(func())
	Log     *slog.Logger
	// PublicURL and TokenTTL shape the registration links the bot hands out.
	PublicURL string
	TokenTTL  time.Duration
}

const (
	activateLimit  = 5
	activateWindow = time.Hour
	codePhoneLimit = 3
	codeIPLimit    = 10
	verifyIPLimit  = 20
	// verifyPhoneLimit caps guesses at one phone's codes from any number of
	// IPs: 30 a day keeps a year of guessing near 1% against one account.
	verifyPhoneLimit  = 30
	verifyPhoneWindow = 24 * time.Hour
	codeWindow        = 15 * time.Minute
	tokenBytes        = 32
	touchEvery        = time.Minute
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

// Activation is the outcome of someone sharing their own phone in the bot.
type Activation struct {
	Result          ActivationResult
	User            User
	RegistrationURL string // set only for ApplyLink
}

// ActivateChannel handles a phone the messenger verified as the sender's own:
// it activates, links a pending application, or hands out a registration
// link, following the decision order of the M2 design (5.5). The caller has
// already checked that the person shared their own contact.
func (s *AuthService) ActivateChannel(ctx context.Context, typ, externalID, chatID, username, verifiedPhone string) (Activation, error) {
	if !s.Limits.Allow("activate:"+typ+":"+externalID, activateLimit, activateWindow) {
		return Activation{}, ErrRateLimited
	}
	phone, err := NormalizePhone(verifiedPhone, s.Region)
	if err != nil {
		return Activation{Result: NotRegistered}, nil
	}
	acct := account{typ: typ, externalID: externalID, chatID: chatID, username: username}
	var out Activation
	err = s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		out, err = s.activate(ctx, acct, phone)
		return err
	})
	if err != nil {
		return Activation{}, err
	}
	return out, nil
}

// account is a messenger account that proved a phone.
type account struct{ typ, externalID, chatID, username string }

func (s *AuthService) activate(ctx context.Context, acct account, phone string) (Activation, error) {
	neutral := Activation{Result: NotRegistered}
	// 1. The account is already linked to someone.
	held, err := s.R.Channels.GetByExternal(ctx, acct.typ, acct.externalID)
	switch {
	case err == nil:
		u, err := s.R.Users.Get(ctx, held.UserID)
		if err != nil {
			return Activation{}, err
		}
		if u.Phone == phone {
			switch u.Status {
			case StatusActive:
				return Activation{Result: AlreadyConnected, User: u}, nil
			case StatusPendingApproval:
				return Activation{Result: PendingApproval, User: u}, nil
			case StatusRejected:
				return s.issueLink(ctx, acct, phone)
			}
			return neutral, nil
		}
		if u.Status != StatusRejected {
			return neutral, nil
		}
		// Held by a rejected person with another phone: the account is free.
	case !errors.Is(err, ErrNotFound):
		return Activation{}, err
	}
	accountFree := err != nil // not linked at all (bootstrap needs a free account)

	if accountFree {
		if ok, err := s.bootstrapApplies(ctx, phone); err != nil {
			return Activation{}, err
		} else if ok {
			u, err := s.bootstrap(ctx, phone)
			if err != nil {
				return Activation{}, err
			}
			return Activation{Result: BootstrapAdmin, User: u}, s.link(ctx, u, acct.typ, acct.externalID, acct.chatID, acct.username)
		}
	}

	// 2. Look the phone up.
	u, err := s.R.Users.GetByPhone(ctx, phone)
	if errors.Is(err, ErrNotFound) {
		return s.issueLink(ctx, acct, phone)
	} else if err != nil {
		return Activation{}, err
	}
	linked, err := hasChannel(ctx, s.R, u.ID, acct.typ)
	if err != nil {
		return Activation{}, err
	}
	switch u.Status {
	case StatusRejected:
		if linked {
			return neutral, nil
		}
		return s.issueLink(ctx, acct, phone)
	case StatusPendingApproval:
		if linked {
			return neutral, nil
		}
		if err := linkAccount(ctx, s.R, s.Now, u, acct); err != nil {
			return Activation{}, err
		}
		return Activation{Result: PendingLinked, User: u}, nil
	case StatusAwaitingActivation:
		if linked {
			return neutral, nil
		}
		now := s.Now()
		u.Status, u.ActivatedAt = StatusActive, &now
		if err := s.R.Users.Update(ctx, &u); err != nil {
			return Activation{}, err
		}
		return Activation{Result: Activated, User: u}, linkAccount(ctx, s.R, s.Now, u, acct)
	}
	return neutral, nil
}

// issueLink hands out a one-time registration link for a proven phone; older
// unused links of the same account stop working.
func (s *AuthService) issueLink(ctx context.Context, acct account, phone string) (Activation, error) {
	raw := make([]byte, tokenBytes)
	if _, err := io.ReadFull(s.rand(), raw); err != nil {
		return Activation{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.Now()
	if err := s.R.RegTokens.InvalidateFor(ctx, acct.typ, acct.externalID, now); err != nil {
		return Activation{}, err
	}
	t := RegistrationToken{TokenHash: HashSecret(s.Secret, token), ChannelType: acct.typ, ExternalID: acct.externalID,
		ChatID: acct.chatID, Username: acct.username, Phone: phone, ExpiresAt: now.Add(s.TokenTTL), CreatedAt: now}
	if err := s.R.RegTokens.Create(ctx, &t); err != nil {
		return Activation{}, err
	}
	return Activation{Result: ApplyLink, RegistrationURL: strings.TrimRight(s.PublicURL, "/") + "/register#t=" + token}, nil
}

func hasChannel(ctx context.Context, r Repos, userID int64, typ string) (bool, error) {
	chans, err := r.Channels.ListByUser(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, c := range chans {
		if c.Type == typ {
			return true, nil
		}
	}
	return false, nil
}

// linkAccount links acct to u, first taking it away from a rejected person
// who held it. The caller has checked that this is allowed.
func linkAccount(ctx context.Context, r Repos, now Clock, u User, acct account) error {
	held, err := r.Channels.GetByExternal(ctx, acct.typ, acct.externalID)
	switch {
	case err == nil:
		if err := r.Channels.Delete(ctx, held.ID); err != nil {
			return err
		}
		if err := r.Audit.Add(ctx, AuditEntry{At: now(), ActorUserID: &u.ID, Action: ActChannelUnlinked, Target: UserTarget(held.UserID),
			Details: map[string]any{"type": acct.typ, "reason": "taken_by_applicant"}}); err != nil {
			return err
		}
	case !errors.Is(err, ErrNotFound):
		return err
	}
	c := Channel{UserID: u.ID, Type: acct.typ, ExternalID: acct.externalID, ChatID: acct.chatID, Username: acct.username, LinkedAt: now()}
	if err := r.Channels.Create(ctx, &c); err != nil {
		return err
	}
	return r.Audit.Add(ctx, AuditEntry{At: now(), ActorUserID: &u.ID, Action: ActChannelLinked, Target: UserTarget(u.ID),
		Details: map[string]any{"type": acct.typ}})
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
	chatID, user := ch.ChatID, u
	deliver := s.Deliver
	if deliver == nil {
		deliver = func(f func()) { go f() }
	}
	// The code is created and sent apart from the answer: a registered phone
	// must not answer slower, or fail when the messenger does.
	deliver(func() {
		if err := s.sendCode(context.WithoutCancel(ctx), user, chatID, actor); err != nil && s.Log != nil {
			s.Log.Warn("login code not delivered", "user", user.ID, "err", err)
		}
	})
	return nil
}

func (s *AuthService) sendCode(ctx context.Context, u User, chatID string, actor Actor) error {
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
	if err := s.Messenger.SendText(ctx, chatID, text); err != nil {
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
	if !s.Limits.Allow("verifyphone:"+phone, verifyPhoneLimit, verifyPhoneWindow) {
		return User{}, "", ErrRateLimited
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
