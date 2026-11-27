package telegram

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/i18n"
	"github.com/igorrius/homelander-bot/locales"
)

type sent struct {
	chatID int64
	text   string
	opts   *gotgbot.SendMessageOpts
}

type fakeAPI struct {
	mu       sync.Mutex
	sent     []sent
	updates  [][]gotgbot.Update
	offsets  []int64
	webhook  string
	secret   string
	deleted  int
	getCalls chan struct{}
	commands map[string][]gotgbot.BotCommand // "scope/lang" → commands
	deleted2 []string                        // "scope/lang" of deleted command lists
}

func scopeName(s gotgbot.BotCommandScope) string {
	switch s.(type) {
	case gotgbot.BotCommandScopeAllPrivateChats:
		return "private"
	case nil, gotgbot.BotCommandScopeDefault:
		return "default"
	}
	return "other"
}

func (f *fakeAPI) SetMyCommands(c []gotgbot.BotCommand, o *gotgbot.SetMyCommandsOpts) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.commands == nil {
		f.commands = map[string][]gotgbot.BotCommand{}
	}
	f.commands[scopeName(o.Scope)+"/"+o.LanguageCode] = c
	return true, nil
}

func (f *fakeAPI) DeleteMyCommands(o *gotgbot.DeleteMyCommandsOpts) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted2 = append(f.deleted2, scopeName(o.Scope)+"/"+o.LanguageCode)
	return true, nil
}

func (f *fakeAPI) GetMe(*gotgbot.GetMeOpts) (*gotgbot.User, error) {
	return &gotgbot.User{Id: 1, Username: "test_bot", IsBot: true}, nil
}

func (f *fakeAPI) GetUpdates(o *gotgbot.GetUpdatesOpts) ([]gotgbot.Update, error) {
	f.mu.Lock()
	f.offsets = append(f.offsets, o.Offset)
	var batch []gotgbot.Update
	if len(f.updates) > 0 {
		batch, f.updates = f.updates[0], f.updates[1:]
	}
	f.mu.Unlock()
	if f.getCalls != nil {
		f.getCalls <- struct{}{}
	}
	if batch == nil {
		time.Sleep(5 * time.Millisecond)
	}
	return batch, nil
}

func (f *fakeAPI) SendMessage(chatID int64, text string, o *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sent{chatID, text, o})
	return &gotgbot.Message{}, nil
}

func (f *fakeAPI) SetWebhook(url string, o *gotgbot.SetWebhookOpts) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.webhook, f.secret = url, o.SecretToken
	return true, nil
}

func (f *fakeAPI) DeleteWebhook(*gotgbot.DeleteWebhookOpts) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted++
	return true, nil
}

func (f *fakeAPI) messages() []sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sent(nil), f.sent...)
}

type activation struct{ typ, external, chat, username, phone string }

type fakeActivator struct {
	mu     sync.Mutex
	calls  []activation
	result domain.ActivationResult
	user   domain.User
	err    error
}

func (f *fakeActivator) ActivateChannel(_ context.Context, typ, ext, chat, username, phone string) (domain.ActivationResult, domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, activation{typ, ext, chat, username, phone})
	return f.result, f.user, f.err
}

type fakeSink struct {
	mu   sync.Mutex
	msgs []GroupMessage
}

func (f *fakeSink) OnGroupMessage(_ context.Context, m GroupMessage) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, m)
}

var ctx = context.Background()

type rig struct {
	api  *fakeAPI
	act  *fakeActivator
	sink *fakeSink
	bot  *Bot
	tr   *i18n.Bundle
}

func newRig(t *testing.T) *rig {
	t.Helper()
	tr, err := i18n.New(locales.FS, "uk", []string{"uk", "en"})
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{api: &fakeAPI{}, act: &fakeActivator{}, sink: &fakeSink{}, tr: tr}
	r.bot, err = New(r.api, r.act, r.sink, tr, "https://example.com", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func private(from int64, lang string) (gotgbot.Chat, *gotgbot.User) {
	return gotgbot.Chat{Id: from, Type: "private"}, &gotgbot.User{Id: from, FirstName: "Test", Username: "tester", LanguageCode: lang}
}

func textUpdate(id, from int64, lang, text string) gotgbot.Update {
	chat, user := private(from, lang)
	return gotgbot.Update{UpdateId: id, Message: &gotgbot.Message{MessageId: id, Chat: chat, From: user, Text: text, Date: 1764244800}}
}

func contactUpdate(from, contactUser int64, phone string) gotgbot.Update {
	chat, user := private(from, "uk")
	return gotgbot.Update{UpdateId: 2, Message: &gotgbot.Message{MessageId: 2, Chat: chat, From: user,
		Contact: &gotgbot.Contact{PhoneNumber: phone, UserId: contactUser, FirstName: "Test"}}}
}

func TestStartAsksForContact(t *testing.T) {
	r := newRig(t)
	r.bot.HandleUpdate(ctx, textUpdate(1, 42, "uk", "/start"))
	msgs := r.api.messages()
	if len(msgs) != 1 || msgs[0].chatID != 42 || msgs[0].text != r.tr.T("uk", "bot.start.prompt", nil, nil) {
		t.Fatalf("messages = %+v", msgs)
	}
	kb, ok := msgs[0].opts.ReplyMarkup.(gotgbot.ReplyKeyboardMarkup)
	if !ok || len(kb.Keyboard) != 1 || !kb.Keyboard[0][0].RequestContact || kb.Keyboard[0][0].Text != r.tr.T("uk", "bot.start.button", nil, nil) || !kb.OneTimeKeyboard {
		t.Errorf("keyboard = %#v", msgs[0].opts.ReplyMarkup)
	}
	r.bot.HandleUpdate(ctx, textUpdate(2, 43, "en", "/start payload"))
	if got := r.api.messages()[1].text; got != r.tr.T("en", "bot.start.prompt", nil, nil) {
		t.Errorf("English prompt = %q", got)
	}
}

func TestContactFromSomeoneElseIsRefused(t *testing.T) {
	r := newRig(t)
	r.bot.HandleUpdate(ctx, contactUpdate(42, 99, "+380500000001"))
	r.bot.HandleUpdate(ctx, contactUpdate(42, 0, "+380500000001"))
	if len(r.act.calls) != 0 {
		t.Fatalf("activator called: %+v", r.act.calls)
	}
	for _, m := range r.api.messages() {
		if m.text != r.tr.T("uk", "bot.contact.notOwn", nil, nil) {
			t.Errorf("reply = %q", m.text)
		}
	}
}

func TestOwnContactActivates(t *testing.T) {
	r := newRig(t)
	r.act.result = domain.Activated
	r.act.user = domain.User{FirstName: "Олена", Locale: "uk"}
	r.bot.HandleUpdate(ctx, contactUpdate(42, 42, "380500000001"))
	if len(r.act.calls) != 1 || r.act.calls[0] != (activation{"telegram", "42", "42", "tester", "380500000001"}) {
		t.Fatalf("calls = %+v", r.act.calls)
	}
	msg := r.api.messages()[0]
	want := r.tr.T("uk", "bot.activated", map[string]any{"Name": "Олена", "URL": "https://example.com"}, nil)
	if msg.text != want {
		t.Errorf("reply = %q, want %q", msg.text, want)
	}
	if _, ok := msg.opts.ReplyMarkup.(gotgbot.ReplyKeyboardRemove); !ok {
		t.Errorf("keyboard not removed: %#v", msg.opts.ReplyMarkup)
	}
}

func TestRepliesPerResult(t *testing.T) {
	cases := []struct {
		result domain.ActivationResult
		err    error
		key    string
		data   map[string]any
	}{
		{domain.BootstrapAdmin, nil, "bot.bootstrap", map[string]any{"URL": "https://example.com/admin"}},
		{domain.AlreadyConnected, nil, "bot.alreadyConnected", nil},
		{domain.PendingApproval, nil, "bot.pending", nil},
		{domain.NotRegistered, nil, "bot.notRegistered", nil},
		{"", domain.ErrRateLimited, "bot.rateLimited", nil},
		{"", errors.New("db down"), "common.error.internal", nil},
	}
	for _, c := range cases {
		r := newRig(t)
		r.act.result, r.act.err = c.result, c.err
		r.act.user = domain.User{Locale: "en"}
		r.bot.HandleUpdate(ctx, contactUpdate(42, 42, "+380500000001"))
		lang := "uk"
		if c.err == nil && c.result != domain.NotRegistered {
			lang = "en" // the linked user's language
		}
		if got := r.api.messages()[0].text; got != r.tr.T(lang, c.key, c.data, nil) {
			t.Errorf("%s/%v: reply %q, want key %s in %s", c.result, c.err, got, c.key, lang)
		}
	}
}

func TestGroupMessageGoesToSinkWithoutReply(t *testing.T) {
	r := newRig(t)
	u := gotgbot.Update{UpdateId: 5, Message: &gotgbot.Message{
		MessageId: 77, MessageThreadId: 12, IsTopicMessage: true, Date: 1764244800, Text: "Світла немає?",
		Chat: gotgbot.Chat{Id: -100123, Type: "supergroup", Title: "Будинок", IsForum: true},
		From: &gotgbot.User{Id: 42, FirstName: "Іван", LastName: "П", Username: "ivan", LanguageCode: "uk"},
	}}
	r.bot.HandleUpdate(ctx, u)
	r.bot.HandleUpdate(ctx, gotgbot.Update{UpdateId: 6, Message: &gotgbot.Message{Text: "/start", Chat: gotgbot.Chat{Id: -5, Type: "group"}, From: &gotgbot.User{Id: 1}}})
	if len(r.api.messages()) != 0 {
		t.Fatalf("bot replied in a group: %+v", r.api.messages())
	}
	if len(r.sink.msgs) != 2 {
		t.Fatalf("sink got %d messages", len(r.sink.msgs))
	}
	got := r.sink.msgs[0]
	want := GroupMessage{ChatID: "-100123", ChatTitle: "Будинок", MessageID: "77", ThreadID: "12", FromExternalID: "42",
		FromUsername: "ivan", FromName: "Іван П", LanguageCode: "uk", Text: "Світла немає?", At: time.Unix(1764244800, 0).UTC()}
	if got != want {
		t.Errorf("group message = %+v\nwant %+v", got, want)
	}
}

func TestOtherTextGetsHelp(t *testing.T) {
	r := newRig(t)
	r.bot.HandleUpdate(ctx, textUpdate(1, 42, "en", "hello"))
	if got := r.api.messages()[0].text; got != r.tr.T("en", "bot.help", nil, nil) {
		t.Errorf("reply = %q", got)
	}
	r.bot.HandleUpdate(ctx, gotgbot.Update{UpdateId: 9}) // not a message
	if len(r.api.messages()) != 1 {
		t.Error("replied to a non-message update")
	}
}

func TestWebhookRejectsWrongSecret(t *testing.T) {
	r := newRig(t)
	h := r.bot.WebhookHandler("s3cret")
	body := `{"update_id":1,"message":{"message_id":1,"date":1,"chat":{"id":42,"type":"private"},"from":{"id":42,"is_bot":false,"first_name":"T"},"contact":{"phone_number":"+380500000001","first_name":"T","user_id":42}}}`
	for _, secret := range []string{"", "wrong"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
		if secret != "" {
			req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
		}
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("secret %q: code %d", secret, rec.Code)
		}
	}
	if len(r.act.calls) != 0 {
		t.Error("update handled with a wrong secret")
	}
}

func TestWebhookHandlesUpdate(t *testing.T) {
	r := newRig(t)
	r.act.result = domain.NotRegistered
	body := `{"update_id":1,"message":{"message_id":1,"date":1,"chat":{"id":42,"type":"private"},"from":{"id":42,"is_bot":false,"first_name":"T"},"contact":{"phone_number":"+380500000001","first_name":"T","user_id":42}}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "s3cret")
	r.bot.WebhookHandler("s3cret").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || len(r.act.calls) != 1 {
		t.Errorf("code %d, calls %d", rec.Code, len(r.act.calls))
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("{broken"))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "s3cret")
	r.bot.WebhookHandler("s3cret").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("broken body code %d, want 200 so Telegram does not retry it", rec.Code)
	}
}

func TestWebhookPathStable(t *testing.T) {
	a, b, c := WebhookPath([]byte("secret-one")), WebhookPath([]byte("secret-one")), WebhookPath([]byte("secret-two"))
	if a != b || a == c || !strings.HasPrefix(a, "/telegram/webhook/") || len(a) != len("/telegram/webhook/")+32 {
		t.Errorf("paths %q %q %q", a, b, c)
	}
}

func TestStartWebhook(t *testing.T) {
	r := newRig(t)
	if err := r.bot.StartWebhook(ctx, "https://example.com/telegram/webhook/x", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if r.api.webhook != "https://example.com/telegram/webhook/x" || r.api.secret != "s3cret" {
		t.Errorf("webhook %q secret %q", r.api.webhook, r.api.secret)
	}
}

func TestPollingAdvancesOffset(t *testing.T) {
	r := newRig(t)
	r.act.result = domain.NotRegistered
	r.api.updates = [][]gotgbot.Update{{textUpdate(10, 42, "uk", "hi"), textUpdate(11, 42, "uk", "hi")}}
	r.api.getCalls = make(chan struct{}, 10)
	pctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- r.bot.RunPolling(pctx) }()
	<-r.api.getCalls
	<-r.api.getCalls
	cancel()
	go func() {
		for range r.api.getCalls {
		}
	}()
	if err := <-done; err != nil {
		t.Fatalf("RunPolling = %v", err)
	}
	r.api.mu.Lock()
	defer r.api.mu.Unlock()
	if r.api.deleted != 1 || len(r.api.offsets) < 2 || r.api.offsets[0] != 0 || r.api.offsets[1] != 12 {
		t.Errorf("deleted %d, offsets %v", r.api.deleted, r.api.offsets)
	}
	if len(r.api.sent) != 2 {
		t.Errorf("replies = %d", len(r.api.sent))
	}
}

func TestBotURLAndSendText(t *testing.T) {
	r := newRig(t)
	if r.bot.BotURL() != "https://t.me/test_bot" || r.bot.Type() != "telegram" {
		t.Errorf("BotURL %q", r.bot.BotURL())
	}
	if err := r.bot.SendText(ctx, "42", "hello"); err != nil || r.api.messages()[0].chatID != 42 {
		t.Errorf("SendText err %v", err)
	}
	if err := r.bot.SendText(ctx, "not-a-number", "x"); err == nil {
		t.Error("bad chat id accepted")
	}
}

func TestSetCommands(t *testing.T) {
	r := newRig(t)
	if err := r.bot.SetCommands(ctx); err != nil {
		t.Fatal(err)
	}
	for key, lang := range map[string]string{"private/": "uk", "private/uk": "uk", "private/en": "en"} {
		cmds := r.api.commands[key]
		if len(cmds) != 1 || cmds[0].Command != "start" || cmds[0].Description != r.tr.T(lang, "bot.commands.start", nil, nil) {
			t.Errorf("%s = %+v", key, cmds)
		}
	}
	// Menus left by earlier software in the default scope (used in groups) are removed.
	for _, want := range []string{"default/", "default/uk", "default/en", "default/ru"} {
		found := false
		for _, d := range r.api.deleted2 {
			found = found || d == want
		}
		if !found {
			t.Errorf("%s not deleted (deleted %v)", want, r.api.deleted2)
		}
	}
}
