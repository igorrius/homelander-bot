package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/igorrius/homelander-bot/internal/messenger/telegram"
)

// fakeTelegram serves queued updates and records sent messages.
type fakeTelegram struct {
	mu     sync.Mutex
	queue  []gotgbot.Update
	sent   []string
	sentTo []sentMsg
	nextID int64
}

type sentMsg struct {
	chat   int64
	thread int64
	text   string
}

func (f *fakeTelegram) GetMe(*gotgbot.GetMeOpts) (*gotgbot.User, error) {
	return &gotgbot.User{Id: 1, Username: "test_bot", IsBot: true}, nil
}

func (f *fakeTelegram) GetUpdates(*gotgbot.GetUpdatesOpts) ([]gotgbot.Update, error) {
	f.mu.Lock()
	q := f.queue
	f.queue = nil
	f.mu.Unlock()
	if len(q) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	return q, nil
}

func (f *fakeTelegram) SendMessage(chat int64, text string, o *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
	var thread int64
	if o != nil {
		thread = o.MessageThreadId
	}
	f.sentTo = append(f.sentTo, sentMsg{chat, thread, text})
	return &gotgbot.Message{}, nil
}

// SendPhoto records a photo as a sent message with its caption, marked "[photo] ".
func (f *fakeTelegram) SendPhoto(chat int64, _ gotgbot.InputFileOrString, o *gotgbot.SendPhotoOpts) (*gotgbot.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	text := "[photo] " + o.Caption
	f.sent = append(f.sent, text)
	f.sentTo = append(f.sentTo, sentMsg{chat, o.MessageThreadId, text})
	return &gotgbot.Message{}, nil
}

func (f *fakeTelegram) SetWebhook(string, *gotgbot.SetWebhookOpts) (bool, error) { return true, nil }
func (f *fakeTelegram) DeleteWebhook(*gotgbot.DeleteWebhookOpts) (bool, error)   { return true, nil }

func (f *fakeTelegram) SetMyCommands([]gotgbot.BotCommand, *gotgbot.SetMyCommandsOpts) (bool, error) {
	return true, nil
}
func (f *fakeTelegram) DeleteMyCommands(*gotgbot.DeleteMyCommandsOpts) (bool, error) {
	return true, nil
}
func (f *fakeTelegram) LeaveChat(int64, *gotgbot.LeaveChatOpts) (bool, error) { return true, nil }
func (f *fakeTelegram) GetChatMember(int64, int64, *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error) {
	return gotgbot.ChatMemberMember{}, nil
}

func (f *fakeTelegram) push(m *gotgbot.Message) { f.pushUpdate(gotgbot.Update{Message: m}) }

func (f *fakeTelegram) pushUpdate(u gotgbot.Update) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	u.UpdateId = f.nextID
	f.queue = append(f.queue, u)
}

// waitForChat waits for a message sent to chat whose text matches re.
func (f *fakeTelegram) waitForChat(t *testing.T, chat int64, re *regexp.Regexp) sentMsg {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		for _, m := range f.sentTo {
			if m.chat == chat && re.MatchString(m.text) {
				f.mu.Unlock()
				return m
			}
		}
		f.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no message to %d matching %s; sent %+v", chat, re, f.sentTo)
	return sentMsg{}
}

func (f *fakeTelegram) waitFor(t *testing.T, re *regexp.Regexp) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		for _, s := range f.sent {
			if m := re.FindString(s); m != "" {
				f.mu.Unlock()
				return m
			}
		}
		f.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no message matching %s; sent %q", re, f.sent)
	return ""
}

func freeAddr(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().String()
}

func TestRunEndToEnd(t *testing.T) {
	addr := freeAddr(t)
	env := map[string]string{
		"APP_SECRET": "0123456789abcdef0123456789abcdef", "PUBLIC_URL": "http://localhost:8080",
		"TELEGRAM_BOT_TOKEN": "1:test", "HTTP_ADDR": addr, "DATABASE_PATH": filepath.Join(t.TempDir(), "db.sqlite"),
		"BOOTSTRAP_ADMIN_PHONE": "+380500000001", "LOG_LEVEL": "error",
	}
	tg := &fakeTelegram{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runWith(ctx, func(k string) string { return env[k] }, io.Discard, func(string) (telegram.API, error) { return tg, nil })
	}()
	base := "http://" + addr
	waitUp(t, base+"/healthz")

	me := &gotgbot.User{Id: 42, FirstName: "Admin", LanguageCode: "uk"}
	chat := gotgbot.Chat{Id: 42, Type: "private"}
	tg.push(&gotgbot.Message{MessageId: 1, Chat: chat, From: me, Text: "/start"})
	tg.push(&gotgbot.Message{MessageId: 2, Chat: chat, From: me, Contact: &gotgbot.Contact{PhoneNumber: "380500000001", UserId: 42, FirstName: "Admin"}})
	tg.waitFor(t, regexp.MustCompile(`/admin`)) // the bootstrap greeting links to the admin panel

	post(t, base+"/api/v1/auth/code", `{"phone":"+380500000001"}`, "", 202)
	code := tg.waitFor(t, regexp.MustCompile(`\b\d{6}\b`))
	resp := post(t, base+"/api/v1/auth/verify", `{"phone":"+380500000001","code":"`+code+`"}`, "", 200)
	var verify struct {
		User struct {
			IsAdmin bool `json:"isAdmin"`
		} `json:"user"`
	}
	_ = json.Unmarshal(resp.body, &verify)
	if !verify.User.IsAdmin {
		t.Fatalf("verify body %s", resp.body)
	}
	cookie := resp.cookie
	post(t, base+"/api/v1/admin/houses", `{"name":"1","apartmentFrom":1,"apartmentTo":10}`, cookie, 201)
	opts := get(t, base+"/api/v1/register/options")
	if !strings.Contains(string(opts), `"name":"1"`) {
		t.Errorf("options %s", opts)
	}
	if prm := get(t, base+"/.well-known/oauth-protected-resource/mcp"); !strings.Contains(string(prm), `"resource":"http://localhost:8080/mcp"`) {
		t.Errorf("protected resource metadata %s", prm)
	}
	if r, err := http.Post(base+"/mcp", "application/json", strings.NewReader(`{}`)); err != nil || r.StatusCode != 401 ||
		!strings.Contains(r.Header.Get("WWW-Authenticate"), "resource_metadata=") {
		t.Errorf("/mcp without a token = %v %v", r, err)
	} else {
		_ = r.Body.Close()
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWith = %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("runWith did not stop")
	}
}

func waitUp(t *testing.T, url string) {
	t.Helper()
	for range 200 {
		if r, err := http.Get(url); err == nil {
			_ = r.Body.Close()
			if r.StatusCode == 200 {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("server did not start")
}

type result struct {
	body   []byte
	cookie string
}

func post(t *testing.T, url, body, cookie string, want int) result {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "homelander")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Body.Close() }()
	b, _ := io.ReadAll(r.Body)
	if r.StatusCode != want {
		t.Fatalf("POST %s = %d %s", url, r.StatusCode, b)
	}
	res := result{body: b}
	for _, c := range r.Cookies() {
		res.cookie = c.Name + "=" + c.Value
	}
	return res
}

func get(t *testing.T, url string) []byte {
	t.Helper()
	r, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Body.Close() }()
	b, _ := io.ReadAll(r.Body)
	return b
}
