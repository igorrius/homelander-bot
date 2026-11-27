package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/igorrius/homelander-bot/internal/messenger/telegram"
)

func member(status string) gotgbot.ChatMember {
	bot := gotgbot.User{Id: 1, IsBot: true, FirstName: "Bot"}
	if status == "left" {
		return gotgbot.ChatMemberLeft{User: bot}
	}
	return gotgbot.ChatMemberMember{User: bot}
}

func do(t *testing.T, method, url, body, cookie string, want int) []byte {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "homelander")
	req.Header.Set("Cookie", cookie)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Body.Close() }()
	b, _ := io.ReadAll(r.Body)
	if r.StatusCode != want {
		t.Fatalf("%s %s = %d %s", method, url, r.StatusCode, b)
	}
	return b
}

// TestM2EndToEnd runs the whole service with a fake Telegram: the admin adds
// the bot to a group, the group is upgraded, a forum is discovered, a label
// routes application notices, and a person registers from the bot.
func TestM2EndToEnd(t *testing.T) {
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

	// The first admin activates and logs in (M1).
	admin := &gotgbot.User{Id: 42, FirstName: "Admin", LanguageCode: "uk"}
	adminChat := gotgbot.Chat{Id: 42, Type: "private"}
	tg.push(&gotgbot.Message{MessageId: 1, Chat: adminChat, From: admin, Contact: &gotgbot.Contact{PhoneNumber: "380500000001", UserId: 42}})
	tg.waitFor(t, regexp.MustCompile(`/admin`))
	post(t, base+"/api/v1/auth/code", `{"phone":"+380500000001"}`, "", 202)
	code := tg.waitFor(t, regexp.MustCompile(`\b\d{6}\b`))
	cookie := post(t, base+"/api/v1/auth/verify", `{"phone":"+380500000001","code":"`+code+`"}`, "", 200).cookie
	do(t, "POST", base+"/api/v1/admin/houses", `{"name":"A","apartmentFrom":1,"apartmentTo":10}`, cookie, 201)

	// The admin adds the bot to the journal group: accepted at once.
	journal := gotgbot.Chat{Id: -1000000001, Type: "group", Title: "journal"}
	tg.pushUpdate(gotgbot.Update{MyChatMember: &gotgbot.ChatMemberUpdated{Chat: journal, From: *admin, Date: 1764244800,
		OldChatMember: member("left"), NewChatMember: member("member")}})
	// The group is upgraded to a supergroup: it gets a new id.
	super := gotgbot.Chat{Id: -1001000000002, Type: "supergroup", Title: "journal"}
	tg.push(&gotgbot.Message{MessageId: 2, Chat: journal, From: admin, MigrateToChatId: super.Id})
	tg.push(&gotgbot.Message{MessageId: 3, Chat: super, From: admin, MigrateFromChatId: journal.Id})
	// A forum the bot was already in shows up from a topic message: new.
	forum := gotgbot.Chat{Id: -1001000000003, Type: "supergroup", Title: "yard", IsForum: true}
	tg.push(&gotgbot.Message{MessageId: 4, Chat: forum, From: admin, MessageThreadId: 7, IsTopicMessage: true, Text: "hi",
		ReplyToMessage: &gotgbot.Message{MessageId: 7, Chat: forum, ForumTopicCreated: &gotgbot.ForumTopicCreated{Name: "News"}}})

	type chatJSON struct {
		ID         int64  `json:"id"`
		State      string `json:"state"`
		ExternalID string `json:"-"`
		Title      string `json:"title"`
		Topics     []struct {
			Title string `json:"title"`
		} `json:"topics"`
	}
	var chats []chatJSON
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_ = json.Unmarshal(do(t, "GET", base+"/api/v1/admin/chats", "", cookie, 200), &chats)
		if len(chats) == 2 && len(chats[0].Topics) == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(chats) != 2 || chats[0].Title != "yard" || chats[0].State != "new" || chats[0].Topics[0].Title != "News" ||
		chats[1].Title != "journal" || chats[1].State != "accepted" {
		t.Fatalf("chats = %+v", chats)
	}
	journalID := chats[1].ID

	// A label routes application notices to the journal.
	var label struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(do(t, "POST", base+"/api/v1/admin/labels", `{"name":"journal","description":""}`, cookie, 201), &label)
	do(t, "PUT", fmt.Sprintf("%s/api/v1/admin/chats/%d/labels", base, journalID), fmt.Sprintf(`{"labelIds":[%d]}`, label.ID), cookie, 204)
	do(t, "PUT", base+"/api/v1/admin/settings/notifications", fmt.Sprintf(`{"applicationsLabelId":%d}`, label.ID), cookie, 200)

	// A resident shares an unknown phone in the bot and gets a link (as text: PUBLIC_URL is not https).
	person := &gotgbot.User{Id: 77, FirstName: "Olena", LanguageCode: "uk"}
	personChat := gotgbot.Chat{Id: 77, Type: "private"}
	tg.push(&gotgbot.Message{MessageId: 5, Chat: personChat, From: person, Contact: &gotgbot.Contact{PhoneNumber: "380500000011", UserId: 77}})
	link := tg.waitForChat(t, 77, regexp.MustCompile(`/register#t=`))
	token := regexp.MustCompile(`#t=([A-Za-z0-9_-]+)`).FindStringSubmatch(link.text)[1]
	var prefill struct {
		Phone string `json:"phone"`
	}
	_ = json.Unmarshal(do(t, "POST", base+"/api/v1/register/prefill", `{"token":"`+token+`"}`, "", 200), &prefill)
	if prefill.Phone != "+380500000011" {
		t.Fatalf("prefill = %+v", prefill)
	}
	var opts struct {
		Statuses []struct {
			ID int64 `json:"id"`
		} `json:"statuses"`
		Houses []struct {
			ID int64 `json:"id"`
		} `json:"houses"`
	}
	_ = json.Unmarshal(get(t, base+"/api/v1/register/options"), &opts)
	do(t, "POST", base+"/api/v1/register", fmt.Sprintf(`{"token":%q,"firstName":"Olena","lastName":"Test","houseId":%d,"apartment":3,"statusId":%d,"consent":true,"locale":"uk"}`,
		token, opts.Houses[0].ID, opts.Statuses[0].ID), "", 202)

	// The journal (now at the supergroup's id) gets the notice, without the phone.
	notice := tg.waitForChat(t, super.Id, regexp.MustCompile(`Olena Test`))
	if strings.Contains(notice.text, "500000011") {
		t.Fatalf("phone in the group notice: %q", notice.text)
	}

	// Approval activates at once and tells the person.
	var page struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	_ = json.Unmarshal(do(t, "GET", base+"/api/v1/admin/users?status=pending_approval", "", cookie, 200), &page)
	do(t, "POST", fmt.Sprintf("%s/api/v1/admin/users/%d/approve", base, page.Items[0].ID), "", cookie, 200)
	tg.waitForChat(t, 77, regexp.MustCompile(`схвалено`))
	post(t, base+"/api/v1/auth/code", `{"phone":"+380500000011"}`, "", 202)
	tg.waitForChat(t, 77, regexp.MustCompile(`\b\d{6}\b`))

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
