package domain_test

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/i18n"
	"github.com/igorrius/homelander-bot/internal/ratelimit"
	"github.com/igorrius/homelander-bot/locales"
)

type authEnv struct {
	*env
	auth *domain.AuthService
	msgr *fakeMessenger
}

func newAuthEnv(t *testing.T) *authEnv {
	e := newEnv(t)
	tr, err := i18n.New(locales.FS, "uk", []string{"uk", "en"})
	if err != nil {
		t.Fatal(err)
	}
	m := &fakeMessenger{}
	a := &domain.AuthService{
		R: e.r, Now: e.clock.Now, Limits: ratelimit.New(e.clock.Now), Secret: []byte("0123456789abcdef0123456789abcdef"),
		Region: "UA", Messenger: m, I18n: tr, CodeTTL: 5 * time.Minute, MaxAttempts: 5,
		IdleTTL: 12 * time.Hour, MaxTTL: 168 * time.Hour,
	}
	return &authEnv{env: e, auth: a, msgr: m}
}

var sixDigits = regexp.MustCompile(`\b\d{6}\b`)

// requestCode asks for a code and returns the one the messenger received.
func (a *authEnv) requestCode(phone string) string {
	a.t.Helper()
	before := len(a.msgr.messages())
	if err := a.auth.RequestCode(ctx, phone, public); err != nil {
		a.t.Fatal(err)
	}
	msgs := a.msgr.messages()
	if len(msgs) != before+1 {
		a.t.Fatalf("messages %d → %d", before, len(msgs))
	}
	code := sixDigits.FindString(msgs[len(msgs)-1].Text)
	if code == "" {
		a.t.Fatalf("no code in %q", msgs[len(msgs)-1].Text)
	}
	return code
}

func activate(a *authEnv, external, phone string) (domain.ActivationResult, domain.User, error) {
	return a.auth.ActivateChannel(ctx, "telegram", external, "chat-"+external, "user"+external, phone)
}

func TestActivateAwaitingUser(t *testing.T) {
	a := newAuthEnv(t)
	u := a.mkUser("+380500000001", func(u *domain.User) { u.Status = domain.StatusAwaitingActivation })
	res, got, err := activate(a, "111", "+380500000001")
	if err != nil || res != domain.Activated || got.ID != u.ID {
		t.Fatalf("activate = %s %+v %v", res, got, err)
	}
	stored := a.get(u.ID)
	if stored.Status != domain.StatusActive || stored.ActivatedAt == nil {
		t.Errorf("user = %+v", stored)
	}
	ch, err := a.r.Channels.GetByExternal(ctx, "telegram", "111")
	if err != nil || ch.ChatID != "chat-111" || ch.Username != "user111" {
		t.Errorf("channel = %+v, %v", ch, err)
	}
	if a.auditCount(domain.ActChannelLinked) != 1 {
		t.Error("no channel.linked audit")
	}
}

func TestActivateStatuses(t *testing.T) {
	a := newAuthEnv(t)
	set := func(s domain.UserStatus) func(*domain.User) { return func(u *domain.User) { u.Status = s } }
	a.mkUser("+380500000001", set(domain.StatusPendingApproval))
	a.mkUser("+380500000002", set(domain.StatusRejected))
	a.mkUser("+380500000003", set(domain.StatusDisabled))
	act := a.mkUser("+380500000004")
	a.link(act, "444")
	cases := []struct {
		external, phone string
		want            domain.ActivationResult
	}{
		{"1", "+380500000001", domain.PendingApproval},
		{"2", "+380500000002", domain.NotRegistered},
		{"3", "+380500000003", domain.NotRegistered},
		{"5", "+380500000005", domain.NotRegistered},
		{"other", "+380500000004", domain.NotRegistered},
		{"444", "+380500000004", domain.AlreadyConnected},
	}
	for _, c := range cases {
		if res, _, err := activate(a, c.external, c.phone); err != nil || res != c.want {
			t.Errorf("%s/%s = %s, %v; want %s", c.external, c.phone, res, err, c.want)
		}
	}
}

func TestActivatePhoneWithoutPlus(t *testing.T) {
	a := newAuthEnv(t)
	a.mkUser("+380500000001", func(u *domain.User) { u.Status = domain.StatusAwaitingActivation })
	if res, _, err := activate(a, "111", "380500000001"); err != nil || res != domain.Activated {
		t.Errorf("activate = %s, %v", res, err)
	}
}

func TestActivateTelegramAccountAlreadyUsed(t *testing.T) {
	a := newAuthEnv(t)
	other := a.mkUser("+380500000002")
	a.link(other, "111")
	a.mkUser("+380500000001", func(u *domain.User) { u.Status = domain.StatusAwaitingActivation })
	if res, _, err := activate(a, "111", "+380500000001"); err != nil || res != domain.NotRegistered {
		t.Errorf("activate with someone else's Telegram account = %s, %v", res, err)
	}
}

func TestActivateRateLimited(t *testing.T) {
	a := newAuthEnv(t)
	for range 5 {
		_, _, _ = activate(a, "111", "+380500000001")
	}
	if _, _, err := activate(a, "111", "+380500000001"); !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("6th err = %v", err)
	}
}

func TestBootstrapOnlyWithoutAdmins(t *testing.T) {
	a := newAuthEnv(t)
	a.auth.BootstrapPhone = "+380500000001"
	res, u, err := activate(a, "111", "380500000001")
	if err != nil || res != domain.BootstrapAdmin || !u.IsAdmin || u.IsResident || u.Status != domain.StatusActive || u.Source != domain.SourceBootstrap {
		t.Fatalf("bootstrap = %s %+v %v", res, u, err)
	}
	if a.auditCount(domain.ActBootstrapAdmin) != 1 {
		t.Error("no bootstrap audit")
	}
	// Once an admin exists the bootstrap phone gets the normal rules.
	a.auth.BootstrapPhone = "+380500000002"
	if res, _, _ := activate(a, "222", "+380500000002"); res != domain.NotRegistered {
		t.Errorf("second bootstrap = %s", res)
	}
}

func TestBootstrapReusesExistingRow(t *testing.T) {
	a := newAuthEnv(t)
	a.auth.BootstrapPhone = "+380500000001"
	u := a.mkUser("+380500000001", func(u *domain.User) { u.Status = domain.StatusPendingApproval })
	res, got, err := activate(a, "111", "+380500000001")
	if err != nil || res != domain.BootstrapAdmin || got.ID != u.ID || !got.IsAdmin {
		t.Errorf("bootstrap over a pending row = %s %+v %v", res, got, err)
	}
}

func TestRequestCodeSendsOnlyToActive(t *testing.T) {
	a := newAuthEnv(t)
	act := a.mkUser("+380500000001")
	a.link(act, "111")
	a.mkUser("+380500000002", func(u *domain.User) { u.Status = domain.StatusPendingApproval })
	dis := a.mkUser("+380500000003", func(u *domain.User) { u.Status = domain.StatusDisabled })
	a.link(dis, "333")
	a.mkUser("+380500000004") // active, but no channel
	for _, p := range []string{"+380500000001", "+380500000002", "+380500000003", "+380500000004", "+380500000005"} {
		if err := a.auth.RequestCode(ctx, p, domain.Actor{IP: p}); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	msgs := a.msgr.messages()
	if len(msgs) != 1 || msgs[0].ChatID != "111" {
		t.Errorf("messages = %+v", msgs)
	}
	err := a.auth.RequestCode(ctx, "12", public)
	wantFields(t, err, map[string]string{"phone": "invalid"})
}

func TestCodeMessageLocalized(t *testing.T) {
	a := newAuthEnv(t)
	u := a.mkUser("+380500000001", func(u *domain.User) { u.Locale = "en" })
	a.link(u, "111")
	code := a.requestCode("+380500000001")
	text := a.msgr.messages()[0].Text
	if !strings.Contains(text, code) || !strings.Contains(text, "12:00") || strings.ContainsAny(text, "абвгґдежзиіїйклмнопрстуфхцчшщьюя") {
		t.Errorf("English code message = %q", text)
	}
}

func activeWithChannel(a *authEnv, phone, external string) domain.User {
	u := a.mkUser(phone, admin)
	a.link(u, external)
	return u
}

func TestVerifyCodeWrongThenRight(t *testing.T) {
	a := newAuthEnv(t)
	u := activeWithChannel(a, "+380500000001", "111")
	code := a.requestCode("+380500000001")
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	if _, _, err := a.auth.VerifyCode(ctx, "+380500000001", wrong, "ua", public); !errors.Is(err, domain.ErrInvalidCode) {
		t.Errorf("wrong code err = %v", err)
	}
	got, token, err := a.auth.VerifyCode(ctx, "050 000 00 01", code, "ua", public)
	if err != nil || got.ID != u.ID || len(token) < 40 {
		t.Fatalf("verify = %+v %q %v", got, token, err)
	}
	au, s, err := a.auth.Authenticate(ctx, token)
	if err != nil || au.ID != u.ID || s.UserAgent != "ua" {
		t.Errorf("Authenticate = %+v %+v %v", au, s, err)
	}
	if a.auditCount(domain.ActAuthLogin) != 1 || a.auditCount(domain.ActAuthLoginFailed) != 1 {
		t.Error("login audit missing")
	}
}

func TestVerifyCodeExpires(t *testing.T) {
	a := newAuthEnv(t)
	activeWithChannel(a, "+380500000001", "111")
	code := a.requestCode("+380500000001")
	a.clock.Advance(5*time.Minute + time.Second)
	if _, _, err := a.auth.VerifyCode(ctx, "+380500000001", code, "", public); !errors.Is(err, domain.ErrInvalidCode) {
		t.Errorf("expired err = %v", err)
	}
}

func TestVerifyCodeAttemptsExhausted(t *testing.T) {
	a := newAuthEnv(t)
	activeWithChannel(a, "+380500000001", "111")
	code := a.requestCode("+380500000001")
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	for range 5 {
		_, _, _ = a.auth.VerifyCode(ctx, "+380500000001", wrong, "", public)
	}
	if _, _, err := a.auth.VerifyCode(ctx, "+380500000001", code, "", public); !errors.Is(err, domain.ErrInvalidCode) {
		t.Errorf("right code after 5 wrong = %v", err)
	}
}

func TestVerifyCodeIsSingleUse(t *testing.T) {
	a := newAuthEnv(t)
	activeWithChannel(a, "+380500000001", "111")
	code := a.requestCode("+380500000001")
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() { _, _, _ = a.auth.VerifyCode(ctx, "+380500000001", code, "", domain.Actor{IP: "x"}) })
	}
	wg.Wait()
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM sessions`).Scan(&n)
	if n != 1 {
		t.Errorf("sessions = %d, want 1", n)
	}
}

func TestNewCodeInvalidatesOld(t *testing.T) {
	a := newAuthEnv(t)
	activeWithChannel(a, "+380500000001", "111")
	a.auth.Rand = bytes.NewReader(append(bytes.Repeat([]byte{1}, 8), bytes.Repeat([]byte{2}, 8)...))
	first := a.requestCode("+380500000001")
	a.auth.Rand = nil
	second := a.requestCode("+380500000001")
	if first == second {
		t.Skip("codes collided")
	}
	if _, _, err := a.auth.VerifyCode(ctx, "+380500000001", first, "", public); !errors.Is(err, domain.ErrInvalidCode) {
		t.Errorf("old code err = %v", err)
	}
}

func TestAuthenticateIdleAndMax(t *testing.T) {
	a := newAuthEnv(t)
	activeWithChannel(a, "+380500000001", "111")
	_, token, err := a.auth.VerifyCode(ctx, "+380500000001", a.requestCode("+380500000001"), "", public)
	if err != nil {
		t.Fatal(err)
	}
	for range 13 { // 13 × 11h = 143h of activity, under the 168h maximum
		a.clock.Advance(11 * time.Hour)
		if _, _, err := a.auth.Authenticate(ctx, token); err != nil {
			t.Fatalf("active session refused: %v", err)
		}
	}
	a.clock.Advance(11 * time.Hour) // 154h
	_, _, _ = a.auth.Authenticate(ctx, token)
	a.clock.Advance(11 * time.Hour) // 165h
	_, _, _ = a.auth.Authenticate(ctx, token)
	a.clock.Advance(4 * time.Hour) // 169h: past the maximum
	if _, _, err := a.auth.Authenticate(ctx, token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("past max err = %v", err)
	}
	_, token2, _ := a.auth.VerifyCode(ctx, "+380500000001", a.requestCode("+380500000001"), "", domain.Actor{IP: "y"})
	a.clock.Advance(12*time.Hour + time.Second)
	if _, _, err := a.auth.Authenticate(ctx, token2); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("idle err = %v", err)
	}
}

func TestAuthenticateDisabledUser(t *testing.T) {
	a := newAuthEnv(t)
	u := activeWithChannel(a, "+380500000001", "111")
	_, token, _ := a.auth.VerifyCode(ctx, "+380500000001", a.requestCode("+380500000001"), "", public)
	u.Status = domain.StatusDisabled
	_ = a.r.Users.Update(ctx, &u)
	if _, _, err := a.auth.Authenticate(ctx, token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("disabled err = %v", err)
	}
	if _, _, err := a.auth.Authenticate(ctx, "garbage"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("garbage err = %v", err)
	}
}

func TestLogoutDeletesSession(t *testing.T) {
	a := newAuthEnv(t)
	activeWithChannel(a, "+380500000001", "111")
	_, token, _ := a.auth.VerifyCode(ctx, "+380500000001", a.requestCode("+380500000001"), "", public)
	if err := a.auth.Logout(ctx, token, public); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.auth.Authenticate(ctx, token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("after logout err = %v", err)
	}
}

func TestRequestCodeRateLimited(t *testing.T) {
	a := newAuthEnv(t)
	activeWithChannel(a, "+380500000001", "111")
	for i := range 3 {
		if err := a.auth.RequestCode(ctx, "+380500000001", domain.Actor{IP: string(rune('a' + i))}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.auth.RequestCode(ctx, "+380500000001", domain.Actor{IP: "z"}); !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("4th code for one phone err = %v", err)
	}
}
