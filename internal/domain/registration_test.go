package domain_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/i18n"
	"github.com/igorrius/homelander-bot/internal/ratelimit"
	"github.com/igorrius/homelander-bot/locales"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

type regEnv struct {
	*chatEnv
	auth   *domain.AuthService
	notify *domain.Notifier
	tr     *i18n.Bundle
}

func newRegEnv(t *testing.T) *regEnv {
	t.Helper()
	c := newChatEnv(t)
	tr, err := i18n.New(locales.FS, "uk", []string{"uk", "en"})
	if err != nil {
		t.Fatal(err)
	}
	defLocale := func(context.Context) string { return "uk" }
	n := &domain.Notifier{R: c.r, Now: c.clock.Now, I18n: tr, DefaultLocale: defLocale}
	auth := &domain.AuthService{R: c.r, Now: c.clock.Now, Limits: ratelimit.New(c.clock.Now), Secret: testSecret,
		Region: "UA", Messenger: c.msgr, I18n: tr, CodeTTL: 5 * time.Minute, MaxAttempts: 5, IdleTTL: time.Hour, MaxTTL: time.Hour,
		Deliver: func(f func()) { f() }, PublicURL: "https://example.test", TokenTTL: time.Hour}
	c.users.Secret, c.users.Notifier, c.users.I18n, c.users.PublicURL, c.users.DefaultLocale = testSecret, n, tr, "https://example.test", defLocale
	return &regEnv{chatEnv: c, auth: auth, notify: n, tr: tr}
}

func (r *regEnv) share(external, phone string) domain.Activation {
	r.t.Helper()
	a, err := r.auth.ActivateChannel(ctx, "telegram", external, external, "user"+external, phone)
	if err != nil {
		r.t.Fatal(err)
	}
	return a
}

func tokenOf(t *testing.T, a domain.Activation) string {
	t.Helper()
	const prefix = "https://example.test/register#t="
	if a.Result != domain.ApplyLink || !strings.HasPrefix(a.RegistrationURL, prefix) {
		t.Fatalf("activation = %+v; want an apply link", a)
	}
	return strings.TrimPrefix(a.RegistrationURL, prefix)
}

func (r *regEnv) setStatus(phone string, s domain.UserStatus) domain.User {
	return r.mkUser(phone, func(u *domain.User) { u.Status = s })
}

func (r *regEnv) channelOf(external string) (domain.Channel, error) {
	return r.r.Channels.GetByExternal(ctx, "telegram", external)
}

func (r *regEnv) journal() domain.Label {
	r.t.Helper()
	r.must(r.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
	l := r.mkLabel("journal")
	r.bind(l, domain.LabelTarget{Type: domain.TargetChat, ID: r.chat(oldGroup).ID})
	r.must(r.r.Notifications.Save(ctx, domain.NotificationSettings{ApplicationsLabelID: &l.ID}))
	return l
}

func (r *regEnv) groupNotices() []domain.OutboxItem {
	var out []domain.OutboxItem
	for _, it := range r.pending() {
		if it.To.ChatID == oldGroup {
			out = append(out, it)
		}
	}
	return out
}

func (r *regEnv) userNotices(u domain.User) []domain.OutboxItem {
	var out []domain.OutboxItem
	for _, it := range r.pending() {
		if it.UserID != nil && *it.UserID == u.ID {
			out = append(out, it)
		}
	}
	return out
}

func (r *regEnv) withToken(token string) domain.Application {
	a := r.application("")
	a.Token = token
	return a
}

// --- the bot's decision order -------------------------------------------

func TestBotUnknownPhoneGetsLink(t *testing.T) {
	r := newRegEnv(t)
	tok := tokenOf(t, r.share("2001", "+380500000011"))
	if len(tok) < 40 {
		t.Fatalf("token %q too short", tok)
	}
	if _, err := r.channelOf("2001"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("nothing must be linked before the application")
	}
}

func TestBotRejectedGetsLink(t *testing.T) {
	r := newRegEnv(t)
	r.setStatus("+380500000011", domain.StatusRejected)
	tokenOf(t, r.share("2001", "+380500000011"))
}

func TestBotRejectedWithOtherAccountNeutral(t *testing.T) {
	r := newRegEnv(t)
	u := r.setStatus("+380500000011", domain.StatusRejected)
	r.link(u, "2002")
	if a := r.share("2001", "+380500000011"); a.Result != domain.NotRegistered {
		t.Fatalf("result = %s", a.Result)
	}
}

func TestBotRejectedWithThisAccountGetsLink(t *testing.T) {
	r := newRegEnv(t)
	u := r.setStatus("+380500000011", domain.StatusRejected)
	r.link(u, "2001")
	tokenOf(t, r.share("2001", "+380500000011"))
}

func TestBotPendingGetsLinked(t *testing.T) {
	r := newRegEnv(t)
	u := r.setStatus("+380500000011", domain.StatusPendingApproval)
	if a := r.share("2001", "+380500000011"); a.Result != domain.PendingLinked {
		t.Fatalf("result = %s", a.Result)
	}
	ch, err := r.channelOf("2001")
	if err != nil || ch.UserID != u.ID {
		t.Fatalf("channel = %+v, %v", ch, err)
	}
	if r.get(u.ID).Status != domain.StatusPendingApproval {
		t.Fatal("status must stay pending")
	}
	if a := r.share("2001", "+380500000011"); a.Result != domain.PendingApproval {
		t.Fatalf("second share = %s", a.Result)
	}
}

func TestBotPendingWithOtherAccountNeutral(t *testing.T) {
	r := newRegEnv(t)
	u := r.setStatus("+380500000011", domain.StatusPendingApproval)
	r.link(u, "2002")
	if a := r.share("2001", "+380500000011"); a.Result != domain.NotRegistered {
		t.Fatalf("result = %s", a.Result)
	}
}

func TestBotAccountOfRejectedOtherPhoneIsFree(t *testing.T) {
	r := newRegEnv(t)
	old := r.setStatus("+380500000012", domain.StatusRejected)
	r.link(old, "2001")
	tok := tokenOf(t, r.share("2001", "+380500000011"))
	r.must(r.users.Apply(ctx, r.withToken(tok), public))
	nu, err := r.r.Users.GetByPhone(ctx, "+380500000011")
	r.must(err)
	ch, err := r.channelOf("2001")
	if err != nil || ch.UserID != nu.ID {
		t.Fatalf("link not moved: %+v, %v", ch, err)
	}
	if r.auditCount(domain.ActChannelUnlinked) != 1 {
		t.Fatal("unlink from the rejected user not audited")
	}
}

func TestBotAccountOfActiveOtherPhoneNeutral(t *testing.T) {
	r := newRegEnv(t)
	other := r.mkUser("+380500000012")
	r.link(other, "2001")
	if a := r.share("2001", "+380500000011"); a.Result != domain.NotRegistered {
		t.Fatalf("result = %s", a.Result)
	}
}

func TestBotAwaitingActivationActivates(t *testing.T) {
	r := newRegEnv(t)
	r.setStatus("+380500000011", domain.StatusAwaitingActivation)
	if a := r.share("2001", "+380500000011"); a.Result != domain.Activated {
		t.Fatalf("result = %s", a.Result)
	}
}

func TestBotDisabledNeutral(t *testing.T) {
	r := newRegEnv(t)
	r.setStatus("+380500000011", domain.StatusDisabled)
	if a := r.share("2001", "+380500000011"); a.Result != domain.NotRegistered {
		t.Fatalf("result = %s", a.Result)
	}
}

// --- tokens ---------------------------------------------------------------

func TestNewLinkInvalidatesOld(t *testing.T) {
	r := newRegEnv(t)
	first := tokenOf(t, r.share("2001", "+380500000011"))
	second := tokenOf(t, r.share("2001", "+380500000011"))
	if _, err := r.users.PrefillRegistration(ctx, first); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("old token: %v", err)
	}
	if phone, err := r.users.PrefillRegistration(ctx, second); err != nil || phone != "+380500000011" {
		t.Fatalf("new token: %q, %v", phone, err)
	}
}

func TestPrefill(t *testing.T) {
	r := newRegEnv(t)
	tok := tokenOf(t, r.share("2001", "+380500000011"))
	if phone, err := r.users.PrefillRegistration(ctx, tok); err != nil || phone != "+380500000011" {
		t.Fatalf("Prefill = %q, %v", phone, err)
	}
	if _, err := r.users.PrefillRegistration(ctx, "nope"); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("unknown token: %v", err)
	}
	r.clock.Advance(time.Hour + time.Second)
	if _, err := r.users.PrefillRegistration(ctx, tok); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("expired token: %v", err)
	}
}

func TestApplyWithTokenUsesTokenPhoneAndLinks(t *testing.T) {
	r := newRegEnv(t)
	tok := tokenOf(t, r.share("2001", "+380500000011"))
	a := r.withToken(tok)
	a.Phone = "+380500000099" // ignored
	r.must(r.users.Apply(ctx, a, public))
	u, err := r.r.Users.GetByPhone(ctx, "+380500000011")
	if err != nil || u.Status != domain.StatusPendingApproval {
		t.Fatalf("user = %+v, %v", u, err)
	}
	if _, err := r.r.Users.GetByPhone(ctx, "+380500000099"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("form phone used instead of the token's")
	}
	ch, err := r.channelOf("2001")
	if err != nil || ch.UserID != u.ID || ch.ChatID != "2001" || ch.Username != "user2001" {
		t.Fatalf("channel = %+v, %v", ch, err)
	}
	if _, err := r.users.PrefillRegistration(ctx, tok); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatal("token still valid after use")
	}
	var via string
	_ = r.db.QueryRow(`SELECT json_extract(details, '$.via') FROM audit_log WHERE action = 'user.applied'`).Scan(&via)
	if via != "bot" {
		t.Fatalf("via = %q", via)
	}
}

func TestApplyWithInvalidToken(t *testing.T) {
	r := newRegEnv(t)
	if err := r.users.Apply(ctx, r.withToken("nope"), public); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("Apply = %v", err)
	}
}

func TestTokenReuseIsSilent(t *testing.T) {
	r := newRegEnv(t)
	tok := tokenOf(t, r.share("2001", "+380500000011"))
	r.must(r.users.Apply(ctx, r.withToken(tok), public))
	if err := r.users.Apply(ctx, r.withToken(tok), public); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("second Apply = %v", err)
	}
	if n := r.auditCount(domain.ActUserApplied); n != 1 {
		t.Fatalf("applied %d times", n)
	}
}

func TestApplyWithTokenAfterPhoneBecameActive(t *testing.T) {
	r := newRegEnv(t)
	tok := tokenOf(t, r.share("2001", "+380500000011"))
	u := r.mkUser("+380500000011") // registered by an admin meanwhile
	r.must(r.users.Apply(ctx, r.withToken(tok), public))
	if _, err := r.channelOf("2001"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("linked to an active user through an application")
	}
	if got := r.get(u.ID); got.Status != domain.StatusActive {
		t.Fatalf("user changed: %+v", got)
	}
}

// --- approval and notices -------------------------------------------------

func TestApproveWithChannelActivatesAndNotifies(t *testing.T) {
	r := newRegEnv(t)
	tok := tokenOf(t, r.share("2001", "+380500000011"))
	a := r.withToken(tok)
	a.Locale = "en"
	r.must(r.users.Apply(ctx, a, public))
	u, _ := r.r.Users.GetByPhone(ctx, "+380500000011")
	r.must(r.users.Approve(ctx, u.ID, actor(r.admin)))
	got := r.get(u.ID)
	if got.Status != domain.StatusActive || got.ActivatedAt == nil {
		t.Fatalf("user = %+v", got)
	}
	if r.auditCount(domain.ActUserActivated) != 1 {
		t.Fatal("user.activated not audited")
	}
	n := r.userNotices(u)
	if len(n) != 1 || n[0].To.ChatID != "2001" || n[0].Msg.Text != r.tr.T("en", "notify.approved.text", nil, nil) {
		t.Fatalf("notices = %+v", n)
	}
	if len(n[0].Msg.Buttons) != 1 || n[0].Msg.Buttons[0].URL != "https://example.test/login" {
		t.Fatalf("button = %+v", n[0].Msg.Buttons)
	}
}

func TestApproveWithoutChannelAwaitsActivation(t *testing.T) {
	r := newRegEnv(t)
	r.must(r.users.Apply(ctx, r.application("+380500000011"), public))
	u, _ := r.r.Users.GetByPhone(ctx, "+380500000011")
	r.must(r.users.Approve(ctx, u.ID, actor(r.admin)))
	if got := r.get(u.ID); got.Status != domain.StatusAwaitingActivation {
		t.Fatalf("user = %+v", got)
	}
	if len(r.userNotices(u)) != 0 {
		t.Fatal("notice queued for a user without a channel")
	}
}

func TestRejectWithChannelNotifies(t *testing.T) {
	r := newRegEnv(t)
	u := r.setStatus("+380500000011", domain.StatusPendingApproval)
	r.share("2001", "+380500000011") // links while pending
	r.must(r.users.Reject(ctx, u.ID, actor(r.admin)))
	n := r.userNotices(u)
	if len(n) != 1 || n[0].Msg.Text != r.tr.T("uk", "notify.rejected", nil, nil) {
		t.Fatalf("notices = %+v", n)
	}
	// The person can apply again from the bot.
	tokenOf(t, r.share("2001", "+380500000011"))
}

func TestApplicationNoticeQueuedWithoutPhone(t *testing.T) {
	r := newRegEnv(t)
	r.journal()
	a := r.application("+380500000011")
	a.FirstName, a.LastName = "Ivan", "Petrenko"
	r.must(r.users.Apply(ctx, a, public))
	n := r.groupNotices()
	if len(n) != 1 {
		t.Fatalf("group notices = %+v", n)
	}
	text := n[0].Msg.Text
	if !strings.Contains(text, "Ivan Petrenko") || strings.Contains(text, "500000011") || strings.Contains(text, "380") {
		t.Fatalf("notice text = %q", text)
	}
	want := r.tr.T("uk", "notify.application.text", map[string]any{"Name": "Ivan Petrenko", "House": r.house.Name,
		"Apartment": 3, "Status": r.owner.Names["uk"], "Via": r.tr.T("uk", "notify.via.site", nil, nil), "Pending": 1}, nil)
	if text != want {
		t.Fatalf("notice = %q\nwant %q", text, want)
	}
	if len(n[0].Msg.Buttons) != 1 || n[0].Msg.Buttons[0].URL != "https://example.test/admin/applications" {
		t.Fatalf("button = %+v", n[0].Msg.Buttons)
	}
}

func TestSameApplicationNotifiedOnce(t *testing.T) {
	r := newRegEnv(t)
	r.journal()
	r.must(r.users.Apply(ctx, r.application("+380500000011"), public))
	r.must(r.users.Apply(ctx, r.application("+380500000011"), domain.Actor{IP: "10.0.0.9"}))
	if n := r.groupNotices(); len(n) != 1 {
		t.Fatalf("notices = %d", len(n))
	}
}

func TestNoNoticeWhenSettingUnset(t *testing.T) {
	r := newRegEnv(t)
	r.must(r.users.Apply(ctx, r.application("+380500000011"), public))
	if n := r.pending(); len(n) != 0 {
		t.Fatalf("rows = %+v", n)
	}
}

func TestApplySucceedsWhenMessengerDown(t *testing.T) {
	r := newRegEnv(t)
	r.journal()
	r.msgr.sendErrs = []error{&domain.SendError{Kind: domain.SendRetry, Err: errors.New("down")}}
	r.must(r.users.Apply(ctx, r.application("+380500000011"), public))
	if _, err := r.worker().RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	n := r.groupNotices()
	if len(n) != 1 || n[0].Attempts != 1 || n[0].State != domain.OutboxPending {
		t.Fatalf("row = %+v", n)
	}
}

func TestPhoneEditRefusedForPendingWithChannel(t *testing.T) {
	r := newRegEnv(t)
	u := r.setStatus("+380500000011", domain.StatusPendingApproval)
	r.share("2001", "+380500000011")
	phone := "+380500000012"
	_, err := r.users.Update(ctx, u.ID, domain.UserPatch{Phone: &phone}, actor(r.admin))
	fieldErr(t, err, "phone", "linked")
}

func TestLinkButtonBecomesTextForPlainHTTP(t *testing.T) {
	r := newRegEnv(t)
	r.users.PublicURL = "http://localhost:8080"
	u := r.setStatus("+380500000011", domain.StatusPendingApproval)
	r.share("2001", "+380500000011")
	r.must(r.users.Approve(ctx, u.ID, actor(r.admin)))
	n := r.userNotices(u)
	if len(n) != 1 || len(n[0].Msg.Buttons) != 0 || !strings.HasSuffix(n[0].Msg.Text, "\nhttp://localhost:8080/login") {
		t.Fatalf("notice = %+v", n)
	}
}
