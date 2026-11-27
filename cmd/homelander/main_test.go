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
	nextID int64
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

func (f *fakeTelegram) SendMessage(_ int64, text string, _ *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
	return &gotgbot.Message{}, nil
}

func (f *fakeTelegram) SetWebhook(string, *gotgbot.SetWebhookOpts) (bool, error) { return true, nil }
func (f *fakeTelegram) DeleteWebhook(*gotgbot.DeleteWebhookOpts) (bool, error)   { return true, nil }

func (f *fakeTelegram) push(m *gotgbot.Message) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	f.queue = append(f.queue, gotgbot.Update{UpdateId: f.nextID, Message: m})
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
