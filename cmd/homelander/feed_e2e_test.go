package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/igorrius/homelander-bot/internal/messenger/telegram"
)

// uploadPNG uploads a tiny PNG to the data space and returns its asset id.
func uploadPNG(t *testing.T, base, cookie string) int64 {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "a.png")
	_, _ = fw.Write([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"))
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/admin/data/assets", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Requested-With", "homelander")
	req.Header.Set("Cookie", cookie)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Body.Close() }()
	var out struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&out); err != nil || r.StatusCode != 201 {
		t.Fatalf("upload = %d, %v", r.StatusCode, err)
	}
	return out.ID
}

// sentTo counts the messages the fake Telegram sent to a chat.
func (f *fakeTelegram) sentToChat(chat int64) []sentMsg {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []sentMsg
	for _, m := range f.sentTo {
		if m.chat == chat {
			out = append(out, m)
		}
	}
	return out
}

// TestFeedPostReachesSharedChatOnce runs the service with a fake Telegram: a
// chat carrying both chosen labels gets a published post once, and a post
// with an image goes as a photo.
func TestFeedPostReachesSharedChatOnce(t *testing.T) {
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

	admin := &gotgbot.User{Id: 42, FirstName: "Admin", LanguageCode: "uk"}
	tg.push(&gotgbot.Message{MessageId: 1, Chat: gotgbot.Chat{Id: 42, Type: "private"}, From: admin,
		Contact: &gotgbot.Contact{PhoneNumber: "380500000001", UserId: 42}})
	tg.waitFor(t, regexp.MustCompile(`/admin`))
	post(t, base+"/api/v1/auth/code", `{"phone":"+380500000001"}`, "", 202)
	code := tg.waitFor(t, regexp.MustCompile(`\b\d{6}\b`))
	cookie := post(t, base+"/api/v1/auth/verify", `{"phone":"+380500000001","code":"`+code+`"}`, "", 200).cookie

	// The admin adds the bot to a group: accepted at once.
	yard := gotgbot.Chat{Id: -1001000000005, Type: "supergroup", Title: "yard"}
	tg.pushUpdate(gotgbot.Update{MyChatMember: &gotgbot.ChatMemberUpdated{Chat: yard, From: *admin, Date: 1764244800,
		OldChatMember: member("left"), NewChatMember: member("member")}})
	var chats []struct {
		ID    int64  `json:"id"`
		State string `json:"state"`
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		_ = json.Unmarshal(do(t, "GET", base+"/api/v1/admin/chats", "", cookie, 200), &chats)
		if len(chats) == 1 && chats[0].State == "accepted" {
			break
		}
	}
	if len(chats) != 1 || chats[0].State != "accepted" {
		t.Fatalf("chats = %+v", chats)
	}
	var labels [2]int64
	for i, name := range []string{"residents", "news"} {
		var l struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(do(t, "POST", base+"/api/v1/admin/labels", `{"name":"`+name+`","description":""}`, cookie, 201), &l)
		labels[i] = l.ID
	}
	do(t, "PUT", fmt.Sprintf("%s/api/v1/admin/chats/%d/labels", base, chats[0].ID), fmt.Sprintf(`{"labelIds":[%d,%d]}`, labels[0], labels[1]), cookie, 204)

	publish := func(body string, image int64) {
		t.Helper()
		img := "null"
		if image != 0 {
			img = fmt.Sprint(image)
		}
		var p struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(do(t, "POST", base+"/api/v1/admin/feed/posts", fmt.Sprintf(`{"body":%q,"imageAssetId":%s}`, body, img), cookie, 201), &p)
		do(t, "POST", fmt.Sprintf("%s/api/v1/admin/feed/posts/%d/publish", base, p.ID), fmt.Sprintf(`{"labelIds":[%d,%d]}`, labels[0], labels[1]), cookie, 200)
	}
	publish("Water is off on Monday", 0)
	tg.waitForChat(t, yard.Id, regexp.MustCompile(`^Water is off on Monday$`))
	publish("New playground", uploadPNG(t, base, cookie))
	tg.waitForChat(t, yard.Id, regexp.MustCompile(`^\[photo\] New playground$`))
	time.Sleep(300 * time.Millisecond) // a duplicate would have been sent by now
	if got := tg.sentToChat(yard.Id); len(got) != 2 {
		t.Fatalf("sent to the yard = %+v", got)
	}

	// The cabinet shows both posts, newest first.
	var feed struct {
		Items []struct {
			Body string `json:"body"`
		} `json:"items"`
	}
	_ = json.Unmarshal(do(t, "GET", base+"/api/v1/feed/posts", "", cookie, 200), &feed)
	if len(feed.Items) != 2 || feed.Items[0].Body != "New playground" {
		t.Fatalf("feed = %+v", feed)
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
