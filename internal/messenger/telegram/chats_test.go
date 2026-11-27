package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// fakeChats records ChatEvents calls as readable strings.
type fakeChats struct {
	mu       sync.Mutex
	calls    []string
	accepted map[string]bool
	known    map[string]bool // chats ChatSeen has reported before
}

var _ domain.ChatEvents = (*fakeChats)(nil)

func (f *fakeChats) record(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, s)
}

func info(c domain.ChatInfo) string {
	return fmt.Sprintf("%s/%s/%s/%q/forum=%v", c.Messenger, c.ExternalID, c.Kind, c.Title, c.IsForum)
}

func (f *fakeChats) BotAdded(_ context.Context, c domain.ChatInfo, by string, r domain.BotRights) error {
	f.record(fmt.Sprintf("added %s by %s %s send=%v", info(c), by, r.Status, r.CanSend))
	return nil
}

func (f *fakeChats) BotRightsChanged(_ context.Context, c domain.ChatInfo, r domain.BotRights) error {
	f.record(fmt.Sprintf("rights %s %s send=%v", info(c), r.Status, r.CanSend))
	return nil
}

func (f *fakeChats) BotRemoved(_ context.Context, c domain.ChatInfo) error {
	f.record("removed " + info(c))
	return nil
}

func (f *fakeChats) ChatSeen(_ context.Context, c domain.ChatInfo) (bool, error) {
	f.record("seen " + info(c))
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.known == nil {
		f.known = map[string]bool{}
	}
	created := !f.known[c.ExternalID]
	f.known[c.ExternalID] = true
	return created, nil
}

func (f *fakeChats) TopicSeen(_ context.Context, c domain.ChatInfo, thread, name string, authoritative bool) error {
	f.record(fmt.Sprintf("topic %s %s %q authoritative=%v", c.ExternalID, thread, name, authoritative))
	return nil
}

func (f *fakeChats) ChatMoved(_ context.Context, messenger, oldID, newID string) error {
	f.record(fmt.Sprintf("moved %s %s→%s", messenger, oldID, newID))
	return nil
}

func (f *fakeChats) IsAccepted(_ context.Context, _, ext string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.accepted[ext], nil
}

func (f *fakeChats) got() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func fixture(t *testing.T, name string) gotgbot.Update {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var u gotgbot.Update
	if err := json.Unmarshal(raw, &u); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return u
}

func (r *rig) feed(t *testing.T, names ...string) []string {
	t.Helper()
	for _, n := range names {
		r.bot.HandleUpdate(ctx, fixture(t, n))
	}
	return r.chats.got()
}

func want(t *testing.T, got []string, w ...string) {
	t.Helper()
	if !reflect.DeepEqual(got, w) {
		t.Fatalf("calls:\n%q\nwant:\n%q", got, w)
	}
}

func TestAllowedUpdates(t *testing.T) {
	r := newRig(t)
	r.api.getCalls = make(chan struct{}, 1)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { _ = r.bot.RunPolling(runCtx) }()
	<-r.api.getCalls
	cancel()
	r.api.mu.Lock()
	gotPoll := slices.Clone(r.api.allowed)
	r.api.mu.Unlock()
	if err := r.bot.StartWebhook(ctx, "https://example.com/hook", "s"); err != nil {
		t.Fatal(err)
	}
	w := []string{"message", "my_chat_member"}
	if !reflect.DeepEqual(gotPoll, w) || !reflect.DeepEqual(r.api.webhookAllowed, w) {
		t.Fatalf("polling %v, webhook %v; want %v", gotPoll, r.api.webhookAllowed, w)
	}
}

func TestMyChatMemberAdded(t *testing.T) {
	r := newRig(t)
	want(t, r.feed(t, "my_chat_member_added.json"),
		`added telegram/-1000000001/group/"log"/forum=false by 1000001 member send=true`)
}

func TestMyChatMemberPromoted(t *testing.T) {
	r := newRig(t)
	want(t, r.feed(t, "my_chat_member_promoted.json", "my_chat_member_restricted.json"),
		`rights telegram/-1001000000001/supergroup/"yard"/forum=true administrator send=true`,
		`rights telegram/-1001000000001/supergroup/"yard"/forum=true restricted send=false`)
}

func TestMyChatMemberRemoved(t *testing.T) {
	r := newRig(t)
	want(t, r.feed(t, "my_chat_member_removed.json"), `removed telegram/-1000000001/group/"log"/forum=false`)
}

func TestMyChatMemberChannelAdmin(t *testing.T) {
	r := newRig(t)
	want(t, r.feed(t, "my_chat_member_channel_admin.json"),
		`added telegram/-1002000000001/channel/"news"/forum=false by 1000001 administrator send=true`)
}

func TestPrivateMyChatMemberIgnored(t *testing.T) {
	r := newRig(t)
	want(t, r.feed(t, "my_chat_member_private.json"))
}

func TestGroupMessageSeenAndRightsFetchedOnce(t *testing.T) {
	r := newRig(t)
	r.api.members = map[int64]gotgbot.ChatMember{-1000000001: gotgbot.ChatMemberAdministrator{User: gotgbot.User{Id: 1}}}
	got := r.feed(t, "group_message.json", "group_message.json")
	want(t, got,
		`seen telegram/-1000000001/group/"log"/forum=false`,
		`rights telegram/-1000000001/group/"log"/forum=false administrator send=true`,
		`seen telegram/-1000000001/group/"log"/forum=false`)
	if r.api.memberN != 1 {
		t.Fatalf("GetChatMember called %d times", r.api.memberN)
	}
}

func TestGroupMessageRightsFetchFailureIsLogged(t *testing.T) {
	r := newRig(t) // no members configured: GetChatMember fails
	want(t, r.feed(t, "group_message.json"), `seen telegram/-1000000001/group/"log"/forum=false`)
}

func TestTopicNameFromCreatedEditedAndReply(t *testing.T) {
	r := newRig(t)
	r.chats.known = map[string]bool{"-1001000000001": true} // skip the rights fetch
	got := r.feed(t, "topic_created.json", "topic_edited.json", "topic_message_reply.json", "topic_message_plain.json")
	seen := `seen telegram/-1001000000001/supergroup/"yard"/forum=true`
	want(t, got,
		seen, `topic -1001000000001 50 "News" authoritative=true`,
		seen, `topic -1001000000001 50 "Announcements" authoritative=true`,
		seen, `topic -1001000000001 40 "Parking" authoritative=false`,
		seen, `topic -1001000000001 30 "" authoritative=false`)
}

func TestMigrateMessages(t *testing.T) {
	r := newRig(t)
	want(t, r.feed(t, "migrate_to.json", "migrate_from.json"),
		"moved telegram -1000000001→-1001000000001",
		"moved telegram -1000000001→-1001000000001")
	if len(r.sink.msgs) != 0 {
		t.Fatal("migration messages reached the sink")
	}
}

func TestSinkOnlyForAcceptedChats(t *testing.T) {
	r := newRig(t)
	r.chats.known = map[string]bool{"-1000000001": true}
	r.feed(t, "group_message.json")
	if len(r.sink.msgs) != 0 {
		t.Fatal("message from a chat that is not accepted reached the sink")
	}
	r.chats.accepted = map[string]bool{"-1000000001": true}
	r.feed(t, "group_message.json")
	if len(r.sink.msgs) != 1 {
		t.Fatalf("sink got %d messages", len(r.sink.msgs))
	}
}

func TestBotOwnLeftMessageIgnored(t *testing.T) {
	r := newRig(t)
	r.bot.HandleUpdate(ctx, gotgbot.Update{UpdateId: 9, Message: &gotgbot.Message{MessageId: 9, Date: 1764244800,
		Chat: gotgbot.Chat{Id: -1000000001, Type: "group", Title: "log"}, From: &gotgbot.User{Id: 1000099},
		LeftChatMember: &gotgbot.User{Id: 1, IsBot: true}}})
	want(t, r.chats.got())
}
