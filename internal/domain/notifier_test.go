package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/i18n"
	"github.com/igorrius/homelander-bot/locales"
)

func (c *chatEnv) notifier() *domain.Notifier {
	c.t.Helper()
	tr, err := i18n.New(locales.FS, "uk", []string{"uk", "en"})
	if err != nil {
		c.t.Fatal(err)
	}
	return &domain.Notifier{R: c.r, Now: c.clock.Now, I18n: tr, DefaultLocale: func(_ ctxT) string { return "en" }}
}

// acceptedForum makes an accepted forum with one topic and returns the chat and topic.
func (c *chatEnv) acceptedForum(ext, thread string) (domain.Chat, domain.Topic) {
	c.t.Helper()
	c.must(c.chats.BotAdded(ctx, forum(ext), adminTG, member))
	c.must(c.chats.TopicSeen(ctx, forum(ext), thread, "News", true))
	ch := c.chat(ext)
	tp, err := c.r.Topics.GetByExternal(ctx, ch.ID, thread)
	c.must(err)
	return ch, tp
}

func (c *chatEnv) bind(l domain.Label, ts ...domain.LabelTarget) {
	c.t.Helper()
	for _, t := range ts {
		have, _ := c.r.Labels.LabelsOf(ctx, t)
		c.must(c.r.Labels.SetLabelsOf(ctx, t, append(have, l.ID)))
	}
}

// pending lists every pending outbox row, oldest first.
func (c *chatEnv) pending() []domain.OutboxItem {
	c.t.Helper()
	rows, err := c.db.Query(`SELECT id FROM outbox WHERE state = 'pending' ORDER BY id`)
	c.must(err)
	var ids []int64
	for rows.Next() {
		var id int64
		c.must(rows.Scan(&id))
		ids = append(ids, id)
	}
	c.must(rows.Close())
	out := make([]domain.OutboxItem, 0, len(ids))
	for _, id := range ids {
		it, err := c.r.Outbox.Get(ctx, id)
		c.must(err)
		out = append(out, it)
	}
	return out
}

func TestSendToLabelResolvesAcceptedOnly(t *testing.T) {
	c := newChatEnv(t)
	n := c.notifier()
	accepted, topic := c.acceptedForum(superGroup, "9")
	gone := domain.Topic{ChatID: accepted.ID, ExternalID: "10", Gone: true, FirstSeenAt: c.clock.t, LastSeenAt: c.clock.t}
	c.must(c.r.Topics.Create(ctx, &gone))
	if _, err := c.chats.ChatSeen(ctx, group(oldGroup)); err != nil { // new, not accepted
		t.Fatal(err)
	}
	newChat := c.chat(oldGroup)
	l := c.mkLabel("journal")
	c.bind(l,
		domain.LabelTarget{Type: domain.TargetChat, ID: accepted.ID},
		domain.LabelTarget{Type: domain.TargetTopic, ID: topic.ID},
		domain.LabelTarget{Type: domain.TargetTopic, ID: gone.ID},
		domain.LabelTarget{Type: domain.TargetChat, ID: newChat.ID})
	got, err := n.SendToLabel(ctx, l.ID, domain.OutMessage{Text: "hi"}, "test")
	if err != nil || got != 2 {
		t.Fatalf("SendToLabel = %d, %v", got, err)
	}
	rows := c.pending()
	if len(rows) != 2 || rows[0].To != (domain.Destination{ChatID: superGroup}) || rows[1].To != (domain.Destination{ChatID: superGroup, ThreadID: "9"}) {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].LabelID == nil || *rows[0].LabelID != l.ID || rows[0].Origin != "test" || rows[0].Messenger != "telegram" {
		t.Fatalf("row fields = %+v", rows[0])
	}
	empty := c.mkLabel("empty")
	if got, err := n.SendToLabel(ctx, empty.ID, domain.OutMessage{Text: "hi"}, "test"); err != nil || got != 0 {
		t.Fatalf("empty label = %d, %v", got, err)
	}
}

func TestSendToUser(t *testing.T) {
	c := newChatEnv(t)
	n := c.notifier()
	ok, err := n.SendToUser(ctx, c.admin.ID, domain.OutMessage{Text: "approved"}, "user")
	if err != nil || !ok {
		t.Fatalf("SendToUser = %v, %v", ok, err)
	}
	rows := c.pending()
	if len(rows) != 1 || rows[0].To.ChatID != adminTG || rows[0].UserID == nil || *rows[0].UserID != c.admin.ID {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestSendToUserNoChannel(t *testing.T) {
	c := newChatEnv(t)
	u := c.mkUser("+380500000002")
	if ok, err := c.notifier().SendToUser(ctx, u.ID, domain.OutMessage{Text: "x"}, "user"); err != nil || ok {
		t.Fatalf("SendToUser = %v, %v", ok, err)
	}
	if len(c.pending()) != 0 {
		t.Fatal("row queued for a user without a channel")
	}
}

func TestTestSends(t *testing.T) {
	c := newChatEnv(t)
	n := c.notifier()
	ch, tp := c.acceptedForum(superGroup, "9")
	ids, err := n.TestChat(ctx, ch.ID)
	if err != nil || len(ids) != 1 {
		t.Fatalf("TestChat = %v, %v", ids, err)
	}
	row, _ := c.r.Outbox.Get(ctx, ids[0])
	if row.Origin != "test" || row.Msg.Text == "" || row.Msg.Text == "notify.test" {
		t.Fatalf("test row = %+v", row)
	}
	if ids, err := n.TestTopic(ctx, tp.ID); err != nil || len(ids) != 1 {
		t.Fatalf("TestTopic = %v, %v", ids, err)
	}
	l := c.mkLabel("journal")
	c.bind(l, domain.LabelTarget{Type: domain.TargetChat, ID: ch.ID})
	if ids, err := n.TestLabel(ctx, l.ID); err != nil || len(ids) != 1 {
		t.Fatalf("TestLabel = %v, %v", ids, err)
	}
}

func TestTestChatNotAccepted(t *testing.T) {
	c := newChatEnv(t)
	n := c.notifier()
	if _, err := c.chats.ChatSeen(ctx, group(oldGroup)); err != nil {
		t.Fatal(err)
	}
	if _, err := n.TestChat(ctx, c.chat(oldGroup).ID); !errors.Is(err, domain.ErrChatNotAccepted) {
		t.Fatalf("TestChat on a new chat: %v", err)
	}
	l := c.mkLabel("empty")
	if _, err := n.TestLabel(ctx, l.ID); !errors.Is(err, domain.ErrChatNotAccepted) {
		t.Fatalf("TestLabel on an empty label: %v", err)
	}
	if _, err := n.TestChat(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("TestChat on an unknown chat: %v", err)
	}
}

func TestNotifierCleanKeepsPending(t *testing.T) {
	c := newChatEnv(t)
	old := c.clock.t.Add(-31 * 24 * time.Hour)
	for _, st := range []domain.OutboxState{domain.OutboxSent, domain.OutboxPending} {
		it := domain.OutboxItem{Messenger: "telegram", To: domain.Destination{ChatID: "1"}, Msg: domain.OutMessage{Text: "x"},
			State: st, NextAttemptAt: old, CreatedAt: old}
		c.must(c.r.Outbox.Add(ctx, &it))
	}
	if n, err := c.notifier().Clean(ctx); err != nil || n != 1 {
		t.Fatalf("Clean = %d, %v", n, err)
	}
}
