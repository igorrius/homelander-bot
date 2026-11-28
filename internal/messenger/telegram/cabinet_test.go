package telegram

import (
	"errors"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func TestCabinetSendsLoginButton(t *testing.T) {
	r := newRig(t)
	r.act.user, r.act.cabinetURL = domain.User{Locale: "en"}, "https://example.com/login/link?t=abc"
	r.bot.HandleUpdate(ctx, textUpdate(1, 42, "uk", "/cabinet"))
	if len(r.act.cabinetCalls) != 1 || r.act.cabinetCalls[0] != "telegram:42" {
		t.Fatalf("calls = %v", r.act.cabinetCalls)
	}
	msg := r.api.messages()[0]
	if msg.chatID != 42 || msg.text != r.tr.T("en", "bot.cabinet.offer", nil, nil) {
		t.Fatalf("reply = %+v", msg)
	}
	kb, ok := msg.opts.ReplyMarkup.(gotgbot.InlineKeyboardMarkup)
	if !ok || len(kb.InlineKeyboard) != 1 || kb.InlineKeyboard[0][0].Url != "https://example.com/login/link?t=abc" ||
		kb.InlineKeyboard[0][0].Text != r.tr.T("en", "bot.cabinet.button", nil, nil) {
		t.Fatalf("markup = %#v", msg.opts.ReplyMarkup)
	}
}

func TestCabinetCommandForms(t *testing.T) {
	r := newRig(t)
	r.act.cabinetURL = "https://example.com/login/link?t=abc"
	for i, text := range []string{"/cabinet@test_bot", "/cabinet now"} {
		r.bot.HandleUpdate(ctx, textUpdate(int64(i+1), 42, "uk", text))
	}
	if len(r.act.cabinetCalls) != 2 {
		t.Errorf("calls = %v", r.act.cabinetCalls)
	}
	r.bot.HandleUpdate(ctx, textUpdate(5, 42, "uk", "/cabinetx"))
	if len(r.act.cabinetCalls) != 2 {
		t.Error("/cabinetx treated as /cabinet")
	}
}

func TestCabinetLinkAsTextForHTTP(t *testing.T) {
	r := newRig(t)
	r.act.cabinetURL = "http://localhost:8080/login/link?t=abc"
	r.bot.HandleUpdate(ctx, textUpdate(1, 42, "uk", "/cabinet"))
	msg := r.api.messages()[0]
	if msg.text != r.tr.T("uk", "bot.cabinet.offer", nil, nil)+"\nhttp://localhost:8080/login/link?t=abc" {
		t.Fatalf("reply = %q", msg.text)
	}
}

func TestCabinetUnknownGetsStart(t *testing.T) {
	r := newRig(t)
	r.act.cabinetErr = domain.ErrNotFound
	r.bot.HandleUpdate(ctx, textUpdate(1, 42, "en", "/cabinet"))
	msg := r.api.messages()[0]
	if msg.text != r.tr.T("en", "bot.start.prompt", nil, nil) {
		t.Fatalf("reply = %q", msg.text)
	}
	if kb, ok := msg.opts.ReplyMarkup.(gotgbot.ReplyKeyboardMarkup); !ok || !kb.Keyboard[0][0].RequestContact {
		t.Errorf("markup = %#v", msg.opts.ReplyMarkup)
	}
}

func TestCabinetErrors(t *testing.T) {
	for err, key := range map[error]string{domain.ErrRateLimited: "bot.rateLimited", errors.New("db down"): "common.error.internal"} {
		r := newRig(t)
		r.act.cabinetErr = err
		r.bot.HandleUpdate(ctx, textUpdate(1, 42, "uk", "/cabinet"))
		if got := r.api.messages()[0].text; got != r.tr.T("uk", key, nil, nil) {
			t.Errorf("%v: reply %q", err, got)
		}
	}
}
