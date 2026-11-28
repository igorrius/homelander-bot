package telegram

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func TestSendPlainAndThread(t *testing.T) {
	r := newRig(t)
	if err := r.bot.Send(ctx, domain.Destination{ChatID: "-1001000000001"}, domain.OutMessage{Text: "hello"}); err != nil {
		t.Fatal(err)
	}
	if err := r.bot.Send(ctx, domain.Destination{ChatID: "-1001000000001", ThreadID: "7"}, domain.OutMessage{Text: "topic"}); err != nil {
		t.Fatal(err)
	}
	got := r.api.messages()
	if len(got) != 2 || got[0].chatID != -1001000000001 || got[0].text != "hello" {
		t.Fatalf("sent = %+v", got)
	}
	if got[0].opts != nil && got[0].opts.MessageThreadId != 0 {
		t.Errorf("plain send has thread %d", got[0].opts.MessageThreadId)
	}
	if got[1].opts == nil || got[1].opts.MessageThreadId != 7 {
		t.Errorf("thread send opts = %+v", got[1].opts)
	}
}

func TestSendButtonsAsInlineKeyboard(t *testing.T) {
	r := newRig(t)
	m := domain.OutMessage{Text: "x", Buttons: []domain.LinkButton{{Text: "A", URL: "https://example.com/a"}, {Text: "B", URL: "https://example.com/b"}}}
	if err := r.bot.Send(ctx, domain.Destination{ChatID: "1000001"}, m); err != nil {
		t.Fatal(err)
	}
	kb, ok := r.api.messages()[0].opts.ReplyMarkup.(gotgbot.InlineKeyboardMarkup)
	if !ok || len(kb.InlineKeyboard) != 2 || kb.InlineKeyboard[0][0].Url != "https://example.com/a" || kb.InlineKeyboard[1][0].Text != "B" {
		t.Fatalf("markup = %#v", r.api.messages()[0].opts.ReplyMarkup)
	}
}

func TestSendBadChatID(t *testing.T) {
	r := newRig(t)
	err := r.bot.Send(ctx, domain.Destination{ChatID: "abc"}, domain.OutMessage{Text: "x"})
	var se *domain.SendError
	if !errors.As(err, &se) || se.Kind != domain.SendPermanent {
		t.Fatalf("err = %v", err)
	}
}

func tgErr(code int, desc string, params *gotgbot.ResponseParameters) error {
	return &gotgbot.TelegramError{Method: "sendMessage", Code: code, Description: desc, ResponseParams: params}
}

func TestClassifyTelegramErrors(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		kind  domain.SendErrorKind
		after time.Duration
		newID string
	}{
		{"flood", tgErr(429, "Too Many Requests: retry after 5", &gotgbot.ResponseParameters{RetryAfter: 5}), domain.SendRetry, 5 * time.Second, ""},
		{"server", tgErr(500, "Internal Server Error", nil), domain.SendRetry, 0, ""},
		{"network", errors.New("dial tcp: connection refused"), domain.SendRetry, 0, ""},
		{"kicked", tgErr(403, "Forbidden: bot was kicked from the supergroup chat", nil), domain.SendChatGone, 0, ""},
		{"migrated", tgErr(400, "Bad Request: group chat was upgraded to a supergroup chat", &gotgbot.ResponseParameters{MigrateToChatId: -100123}), domain.SendChatMoved, 0, "-100123"},
		{"thread", tgErr(400, "Bad Request: message thread not found", nil), domain.SendThreadGone, 0, ""},
		{"no chat", tgErr(400, "Bad Request: chat not found", nil), domain.SendChatGone, 0, ""},
		{"other", tgErr(400, "Bad Request: not enough rights to send text messages to the chat", nil), domain.SendPermanent, 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			r.api.sendErr = c.err
			err := r.bot.Send(ctx, domain.Destination{ChatID: "-1"}, domain.OutMessage{Text: "x"})
			var se *domain.SendError
			if !errors.As(err, &se) {
				t.Fatalf("err = %v, want *domain.SendError", err)
			}
			if se.Kind != c.kind || se.RetryAfter != c.after || se.NewChatID != c.newID {
				t.Fatalf("got %+v; want kind %s after %v new %q", se, c.kind, c.after, c.newID)
			}
			if !errors.Is(err, c.err) {
				t.Error("SendError must wrap the original error")
			}
		})
	}
}

func TestLeaveChat(t *testing.T) {
	r := newRig(t)
	if err := r.bot.LeaveChat(ctx, "-1001000000001"); err != nil {
		t.Fatal(err)
	}
	if len(r.api.left) != 1 || r.api.left[0] != -1001000000001 {
		t.Fatalf("left = %v", r.api.left)
	}
}

func TestLeaveChatAlreadyGoneIsNil(t *testing.T) {
	for _, e := range []error{tgErr(403, "Forbidden: bot is not a member of the supergroup chat", nil), tgErr(400, "Bad Request: chat not found", nil)} {
		r := newRig(t)
		r.api.leaveErr = e
		if err := r.bot.LeaveChat(ctx, "-1"); err != nil {
			t.Errorf("LeaveChat with %v = %v, want nil", e, err)
		}
	}
	r := newRig(t)
	r.api.leaveErr = tgErr(500, "Internal Server Error", nil)
	if err := r.bot.LeaveChat(ctx, "-1"); err == nil {
		t.Error("LeaveChat must report a server error")
	}
}

func TestSendPhotoWithCaptionAndButton(t *testing.T) {
	r := newRig(t)
	m := domain.OutMessage{Text: "caption <b>plain</b>", Buttons: []domain.LinkButton{{Text: "Open", URL: "https://example.com/my/feed/1"}},
		ImageAssetID: 3, Image: &domain.OutImage{Data: []byte{1, 2}, ContentType: "image/png"}}
	if err := r.bot.Send(ctx, domain.Destination{ChatID: "-1001000000001", ThreadID: "7"}, m); err != nil {
		t.Fatal(err)
	}
	if len(r.api.messages()) != 0 {
		t.Fatalf("sendMessage used: %+v", r.api.messages())
	}
	r.api.mu.Lock()
	defer r.api.mu.Unlock()
	if len(r.api.photos) != 1 {
		t.Fatalf("photos = %+v", r.api.photos)
	}
	p := r.api.photos[0]
	if p.opts.RequestOpts == nil || p.opts.RequestOpts.Timeout != PhotoTimeout {
		t.Errorf("photo request opts = %+v", p.opts.RequestOpts)
	}
	if p.chatID != -1001000000001 || p.name != "photo.png" || string(p.data) != "\x01\x02" || p.opts.Caption != m.Text ||
		p.opts.ParseMode != "" || p.opts.MessageThreadId != 7 {
		t.Errorf("photo = %+v", p)
	}
	kb, ok := p.opts.ReplyMarkup.(gotgbot.InlineKeyboardMarkup)
	if !ok || kb.InlineKeyboard[0][0].Url != "https://example.com/my/feed/1" {
		t.Errorf("markup = %#v", p.opts.ReplyMarkup)
	}
}

func TestSendTextHasTimeout(t *testing.T) {
	r := newRig(t)
	if err := r.bot.Send(ctx, domain.Destination{ChatID: "1000001"}, domain.OutMessage{Text: "x"}); err != nil {
		t.Fatal(err)
	}
	if o := r.api.messages()[0].opts; o.RequestOpts == nil || o.RequestOpts.Timeout != SendTimeout {
		t.Errorf("request opts = %+v", o.RequestOpts)
	}
}

// TestSlowPhotoUploadIsNotResentSoon sends a photo through the real Bot API
// client to a Telegram stand-in that answers after the timeout, as a slow
// uplink would: the outbox must wait long before trying again, because
// Telegram may have posted the photo already.
func TestSlowPhotoUploadIsNotResentSoon(t *testing.T) {
	var photos atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/getMe") {
			_, _ = io.WriteString(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Bot","username":"test_bot"}}`)
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		photos.Add(1)
		select { // posted, but the answer comes too late
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":-1,"type":"group"}}}`)
	}))
	defer srv.Close()
	api, err := gotgbot.NewBot("1:test", &gotgbot.BotOpts{BotClient: &gotgbot.BaseBotClient{Client: http.Client{},
		DefaultRequestOpts: &gotgbot.RequestOpts{APIURL: srv.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	r := newRig(t)
	bot, err := New(api, r.act, r.chats, r.sink, r.tr, "https://example.com", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	bot.photoTimeout = 100 * time.Millisecond
	err = bot.Send(ctx, domain.Destination{ChatID: "-1001000000001"}, domain.OutMessage{Text: "caption",
		Image: &domain.OutImage{Data: bytes.Repeat([]byte{1}, 64<<10), ContentType: "image/jpeg"}})
	var se *domain.SendError
	if !errors.As(err, &se) || se.Kind != domain.SendRetry || se.RetryAfter != AmbiguousRetry || !strings.Contains(err.Error(), "may have posted") {
		t.Fatalf("err = %v", err)
	}
	if photos.Load() != 1 {
		t.Errorf("uploads = %d", photos.Load())
	}
}
