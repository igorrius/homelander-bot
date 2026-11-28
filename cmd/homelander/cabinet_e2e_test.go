package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/igorrius/homelander-bot/internal/messenger/telegram"
)

// TestCabinetLoginLinkEndToEnd: /cabinet in the bot gives a link; opening its
// page spends nothing, the POST logs in once, and the manifest opens /my.
func TestCabinetLoginLinkEndToEnd(t *testing.T) {
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
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(20 * time.Second):
			t.Error("runWith did not stop")
		}
	}()
	base := "http://" + addr
	waitUp(t, base+"/healthz")

	from := &gotgbot.User{Id: 42, FirstName: "Admin", LanguageCode: "uk"}
	chat := gotgbot.Chat{Id: 42, Type: "private"}
	tg.push(&gotgbot.Message{MessageId: 1, Chat: chat, From: from, Contact: &gotgbot.Contact{PhoneNumber: "380500000001", UserId: 42, FirstName: "Admin"}})
	tg.waitFor(t, regexp.MustCompile(`/admin`))
	tg.push(&gotgbot.Message{MessageId: 2, Chat: chat, From: from, Text: "/cabinet"})
	link := tg.waitFor(t, regexp.MustCompile(`http://localhost:8080/login/link\?t=[A-Za-z0-9_-]+`))
	u, _ := url.Parse(link)
	token := u.Query().Get("t")

	// What a link preview would fetch: the app's page, nothing spent.
	if r, err := http.Get(base + u.RequestURI()); err == nil {
		_ = r.Body.Close()
	}
	res := post(t, base+"/api/v1/auth/link", `{"token":"`+token+`"}`, "", 200)
	var body struct {
		Next string `json:"next"`
	}
	if err := json.Unmarshal(res.body, &body); err != nil || body.Next != "/my" || res.cookie == "" {
		t.Fatalf("redeem = %s cookie %q", res.body, res.cookie)
	}
	post(t, base+"/api/v1/auth/link", `{"token":"`+token+`"}`, "", 422)

	var manifest struct {
		StartURL string `json:"start_url"`
		Name     string `json:"name"`
	}
	if err := json.Unmarshal(get(t, base+"/site.webmanifest"), &manifest); err != nil || manifest.StartURL != "/my" || manifest.Name == "" {
		t.Errorf("manifest = %+v, %v", manifest, err)
	}
}
