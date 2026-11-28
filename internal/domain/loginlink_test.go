package domain_test

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// linkToken returns the raw token of a login link URL.
func linkToken(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil || u.Path != "/login/link" || u.Query().Get("t") == "" {
		t.Fatalf("link = %q", link)
	}
	return u.Query().Get("t")
}

func newLinkEnv(t *testing.T) *authEnv {
	a := newAuthEnv(t)
	a.auth.PublicURL = "https://example.com/"
	return a
}

func TestLoginLinkSingleUse(t *testing.T) {
	a := newLinkEnv(t)
	u := a.mkUser("+380500000001")
	link, err := a.auth.CreateLoginLink(ctx, u.ID, "/my/feed/3")
	if err != nil || !strings.HasPrefix(link, "https://example.com/login/link?t=") {
		t.Fatalf("CreateLoginLink = %q, %v", link, err)
	}
	tok := linkToken(t, link)
	got, session, next, err := a.auth.UseLoginLink(ctx, tok, "test-agent", public)
	if err != nil || got.ID != u.ID || session == "" || next != "/my/feed/3" {
		t.Fatalf("UseLoginLink = %+v %q %q %v", got, session, next, err)
	}
	if a.sessions(u) != 1 {
		t.Errorf("sessions = %d", a.sessions(u))
	}
	if _, _, err := a.auth.Authenticate(ctx, session); err != nil {
		t.Errorf("session does not authenticate: %v", err)
	}
	var details string
	_ = a.db.QueryRow(`SELECT details FROM audit_log WHERE action = 'auth.login'`).Scan(&details)
	if !strings.Contains(details, `"via":"bot_link"`) {
		t.Errorf("audit details = %q", details)
	}
	if _, _, _, err := a.auth.UseLoginLink(ctx, tok, "", public); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("second use err = %v", err)
	}
	if _, _, _, err := a.auth.UseLoginLink(ctx, "nonsense", "", public); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("unknown token err = %v", err)
	}
	if _, _, _, err := a.auth.UseLoginLink(ctx, "", "", public); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("empty token err = %v", err)
	}
}

func TestLoginLinkExpires(t *testing.T) {
	a := newLinkEnv(t)
	u := a.mkUser("+380500000001")
	early, _ := a.auth.CreateLoginLink(ctx, u.ID, "/my")
	late, _ := a.auth.CreateLoginLink(ctx, u.ID, "/my")
	a.clock.Advance(domain.LoginLinkTTL - time.Second)
	if _, _, _, err := a.auth.UseLoginLink(ctx, linkToken(t, early), "", public); err != nil {
		t.Fatalf("link a second before expiry: %v", err)
	}
	a.clock.Advance(time.Second)
	if _, _, _, err := a.auth.UseLoginLink(ctx, linkToken(t, late), "", public); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("expired link err = %v", err)
	}
	if n, err := a.auth.CleanLoginLinks(ctx); err != nil || n != 0 {
		t.Errorf("clean at expiry = %d, %v", n, err)
	}
	a.clock.Advance(time.Second)
	if n, err := a.auth.CleanLoginLinks(ctx); err != nil || n != 2 {
		t.Errorf("clean after expiry = %d, %v", n, err)
	}
}

func TestLoginLinkNextValidation(t *testing.T) {
	a := newLinkEnv(t)
	u := a.mkUser("+380500000001")
	for _, next := range []string{"/my", "/my/", "/my/feed/3", "/my?tab=1", "/my/feed?x=%2F"} {
		if _, err := a.auth.CreateLoginLink(ctx, u.ID, next); err != nil {
			t.Errorf("%q refused: %v", next, err)
		}
	}
	for _, next := range []string{"", "/", "/admin", "/myfoo", "/my//evil.example", "//evil.example/my", "/my\\x", "/my/a b",
		"https://evil.example/my", "my", "/my/\nx"} {
		_, err := a.auth.CreateLoginLink(ctx, u.ID, next)
		var ve *domain.ValidationError
		if !errors.As(err, &ve) || ve.Fields["next"] != "invalid" {
			t.Errorf("%q: err = %v", next, err)
		}
	}
}

func TestLoginLinkUserDisabledAfterMint(t *testing.T) {
	a := newLinkEnv(t)
	u := a.mkUser("+380500000001")
	link, _ := a.auth.CreateLoginLink(ctx, u.ID, "/my")
	u.Status = domain.StatusDisabled
	if err := a.r.Users.Update(ctx, &u); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := a.auth.UseLoginLink(ctx, linkToken(t, link), "", public); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("disabled user err = %v", err)
	}
	if a.sessions(u) != 0 {
		t.Error("session opened for a disabled user")
	}
	if _, err := a.auth.CreateLoginLink(ctx, u.ID, "/my"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("link minted for a disabled user: %v", err)
	}
}

func TestLoginLinkRateLimitedPerUser(t *testing.T) {
	a := newLinkEnv(t)
	u := a.mkUser("+380500000001")
	other := a.mkUser("+380500000002")
	for i := range 10 {
		if _, err := a.auth.CreateLoginLink(ctx, u.ID, "/my"); err != nil {
			t.Fatalf("link %d: %v", i, err)
		}
	}
	if _, err := a.auth.CreateLoginLink(ctx, u.ID, "/my"); !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("11th link err = %v", err)
	}
	if _, err := a.auth.CreateLoginLink(ctx, other.ID, "/my"); err != nil {
		t.Errorf("another user limited: %v", err)
	}
}

func TestCabinetLink(t *testing.T) {
	a := newLinkEnv(t)
	u := a.mkUser("+380500000001")
	a.link(u, "111")
	pending := a.mkUser("+380500000002", func(u *domain.User) { u.Status = domain.StatusPendingApproval })
	a.link(pending, "222")
	got, link, err := a.auth.CabinetLink(ctx, "telegram", "111")
	if err != nil || got.ID != u.ID {
		t.Fatalf("CabinetLink = %+v %q %v", got, link, err)
	}
	if _, _, next, err := a.auth.UseLoginLink(ctx, linkToken(t, link), "", public); err != nil || next != "/my" {
		t.Errorf("UseLoginLink next %q err %v", next, err)
	}
	for _, ext := range []string{"222", "333"} {
		if _, _, err := a.auth.CabinetLink(ctx, "telegram", ext); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s: err = %v", ext, err)
		}
	}
}
