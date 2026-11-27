package domain_test

import (
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

const (
	adminTG    = "1000001"
	strangerTG = "1000099"
	oldGroup   = "-1000000001"
	superGroup = "-1001000000001"
)

type chatEnv struct {
	*env
	msgr  *fakeMessenger
	chats *domain.ChatService
	admin domain.User
}

func newChatEnv(t *testing.T) *chatEnv {
	t.Helper()
	e := newEnv(t)
	m := &fakeMessenger{}
	c := &chatEnv{env: e, msgr: m}
	c.chats = &domain.ChatService{R: e.r, Now: e.clock.Now, Messengers: domain.Messengers{"telegram": m},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), LeaveGrace: 2 * time.Minute}
	c.admin = e.mkUser("+380500000001", admin)
	e.link(c.admin, adminTG)
	return c
}

func group(ext string) domain.ChatInfo {
	return domain.ChatInfo{Messenger: "telegram", ExternalID: ext, Kind: domain.ChatKindGroup, Title: "log"}
}

func forum(ext string) domain.ChatInfo {
	return domain.ChatInfo{Messenger: "telegram", ExternalID: ext, Kind: domain.ChatKindSupergroup, Title: "yard", IsForum: true}
}

var member = domain.BotRights{Status: domain.BotMember, CanSend: true}

func (c *chatEnv) chat(ext string) domain.Chat {
	c.t.Helper()
	ch, err := c.r.Chats.GetByExternal(ctx, "telegram", ext)
	if err != nil {
		c.t.Fatalf("chat %s: %v", ext, err)
	}
	return ch
}

func (c *chatEnv) must(err error) {
	c.t.Helper()
	if err != nil {
		c.t.Fatal(err)
	}
}

func (c *chatEnv) mkLabel(name string) domain.Label {
	c.t.Helper()
	l := domain.Label{Name: name, CreatedAt: c.clock.t, UpdatedAt: c.clock.t}
	c.must(c.r.Labels.Create(ctx, &l))
	return l
}

func TestAddedByAdminIsAccepted(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
	ch := c.chat(oldGroup)
	if ch.State != domain.ChatAccepted || ch.AcceptedBy == nil || *ch.AcceptedBy != c.admin.ID || ch.LeaveAt != nil {
		t.Fatalf("chat = %+v", ch)
	}
	if ch.AddedByUserID == nil || *ch.AddedByUserID != c.admin.ID || ch.AddedByExternalID != adminTG || !ch.CanSend || ch.BotStatus != domain.BotMember {
		t.Fatalf("added-by/rights not recorded: %+v", ch)
	}
	if c.auditCount("chat.added") != 1 || c.auditCount("chat.accepted") != 1 {
		t.Fatal("audit entries missing")
	}
}

func TestAddedByDisabledAdminIsNotTrusted(t *testing.T) {
	c := newChatEnv(t)
	u := c.mkUser("+380500000002", admin, func(u *domain.User) { u.Status = domain.StatusDisabled })
	c.link(u, "1000002")
	c.must(c.chats.BotAdded(ctx, group(oldGroup), "1000002", member))
	if ch := c.chat(oldGroup); ch.State != domain.ChatNew || ch.LeaveAt == nil {
		t.Fatalf("chat = %+v", ch)
	}
}

func TestAddedByStrangerLeavesAfterGrace(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), strangerTG, member))
	ch := c.chat(oldGroup)
	if ch.State != domain.ChatNew || ch.LeaveAt == nil || !ch.LeaveAt.Equal(c.clock.t.Add(2*time.Minute)) {
		t.Fatalf("chat = %+v", ch)
	}
	c.clock.Advance(time.Minute)
	if n, err := c.chats.LeaveDue(ctx); err != nil || n != 0 || len(c.msgr.leftChats()) != 0 {
		t.Fatalf("left before the grace ran out: %d, %v", n, err)
	}
	c.clock.Advance(time.Minute)
	if n, err := c.chats.LeaveDue(ctx); err != nil || n != 1 {
		t.Fatalf("LeaveDue = %d, %v", n, err)
	}
	if got := c.msgr.leftChats(); !reflect.DeepEqual(got, []string{oldGroup}) {
		t.Fatalf("left = %v", got)
	}
	if ch := c.chat(oldGroup); ch.State != domain.ChatLeft || ch.LeaveAt != nil || ch.LeftAt == nil {
		t.Fatalf("after leave = %+v", ch)
	}
	if c.auditCount("chat.auto_left") != 1 {
		t.Fatal("chat.auto_left not audited")
	}
}

func TestLeaveDueKeepsChatWhenLeaveFails(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), strangerTG, member))
	c.clock.Advance(3 * time.Minute)
	c.msgr.leaveErr = errors.New("telegram down")
	if n, _ := c.chats.LeaveDue(ctx); n != 0 {
		t.Fatalf("counted a failed leave: %d", n)
	}
	if ch := c.chat(oldGroup); ch.State != domain.ChatNew || ch.LeaveAt == nil {
		t.Fatalf("chat must wait for the next try: %+v", ch)
	}
}

func TestAddedByAnonymousAdminWaitsAsNew(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), domain.AnonymousAdminExternalID, member))
	if ch := c.chat(oldGroup); ch.State != domain.ChatNew || ch.LeaveAt != nil {
		t.Fatalf("chat = %+v", ch)
	}
}

func TestReAddedByAdminKeepsLabels(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
	ch := c.chat(oldGroup)
	l := c.mkLabel("journal")
	target := domain.LabelTarget{Type: domain.TargetChat, ID: ch.ID}
	c.must(c.r.Labels.SetLabelsOf(ctx, target, []int64{l.ID}))
	c.must(c.chats.BotRemoved(ctx, group(oldGroup)))
	if ch := c.chat(oldGroup); ch.State != domain.ChatLeft || ch.LeftAt == nil || ch.CanSend {
		t.Fatalf("after removal = %+v", ch)
	}
	c.must(c.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
	if ch := c.chat(oldGroup); ch.State != domain.ChatAccepted || ch.LeftAt != nil {
		t.Fatalf("after re-add = %+v", ch)
	}
	if got, _ := c.r.Labels.LabelsOf(ctx, target); !reflect.DeepEqual(got, []int64{l.ID}) {
		t.Fatalf("labels after re-add = %v", got)
	}
}

func TestSeenWithoutAddIsNew(t *testing.T) {
	c := newChatEnv(t)
	created, err := c.chats.ChatSeen(ctx, group(oldGroup))
	if err != nil || !created {
		t.Fatalf("ChatSeen = %v, %v", created, err)
	}
	if ch := c.chat(oldGroup); ch.State != domain.ChatNew || ch.LeaveAt != nil || ch.BotStatus != domain.BotUnknown {
		t.Fatalf("chat = %+v", ch)
	}
	if n, _ := c.chats.NewCount(ctx); n != 1 {
		t.Fatalf("NewCount = %d", n)
	}
}

func TestSeenAfterLeftBecomesNew(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
	c.must(c.chats.BotRemoved(ctx, group(oldGroup)))
	if _, err := c.chats.ChatSeen(ctx, group(oldGroup)); err != nil {
		t.Fatal(err)
	}
	if ch := c.chat(oldGroup); ch.State != domain.ChatNew {
		t.Fatalf("chat = %+v", ch)
	}
}

func TestAcceptCancelsGrace(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), strangerTG, member))
	c.must(c.chats.Accept(ctx, c.chat(oldGroup).ID, actor(c.admin)))
	ch := c.chat(oldGroup)
	if ch.State != domain.ChatAccepted || ch.LeaveAt != nil || ch.AcceptedBy == nil || *ch.AcceptedBy != c.admin.ID {
		t.Fatalf("chat = %+v", ch)
	}
	c.clock.Advance(time.Hour)
	if n, _ := c.chats.LeaveDue(ctx); n != 0 {
		t.Fatal("accepted chat was left")
	}
	if err := c.chats.Accept(ctx, ch.ID, actor(c.admin)); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("accepting twice: %v", err)
	}
}

func TestLeaveCallsMessengerThenSetsLeft(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
	id := c.chat(oldGroup).ID
	c.msgr.leaveErr = errors.New("telegram down")
	if err := c.chats.Leave(ctx, id, actor(c.admin)); err == nil {
		t.Fatal("Leave must report a messenger failure")
	}
	if ch := c.chat(oldGroup); ch.State != domain.ChatAccepted {
		t.Fatalf("state changed although the bot did not leave: %+v", ch)
	}
	c.msgr.leaveErr = nil
	c.must(c.chats.Leave(ctx, id, actor(c.admin)))
	if ch := c.chat(oldGroup); ch.State != domain.ChatLeft || ch.LeftAt == nil {
		t.Fatalf("chat = %+v", ch)
	}
	if c.auditCount("chat.left") != 1 {
		t.Fatal("chat.left not audited")
	}
	if err := c.chats.Leave(ctx, id, actor(c.admin)); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("leaving a left chat: %v", err)
	}
}

func TestLeaveWhenAlreadyGone(t *testing.T) {
	// The messenger answers nil when the bot is already not there (Task 3).
	c := newChatEnv(t)
	if _, err := c.chats.ChatSeen(ctx, group(oldGroup)); err != nil {
		t.Fatal(err)
	}
	c.must(c.chats.Leave(ctx, c.chat(oldGroup).ID, actor(c.admin)))
	if ch := c.chat(oldGroup); ch.State != domain.ChatLeft {
		t.Fatalf("chat = %+v", ch)
	}
}

func TestBotRightsChanged(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
	c.must(c.chats.BotRightsChanged(ctx, group(oldGroup), domain.BotRights{Status: domain.BotRestricted, CanSend: false}))
	if ch := c.chat(oldGroup); ch.BotStatus != domain.BotRestricted || ch.CanSend || ch.State != domain.ChatAccepted {
		t.Fatalf("chat = %+v", ch)
	}
}

func TestUpgradeByNonAdminKeepsBot(t *testing.T) {
	for _, order := range []string{"added-then-moved", "moved-then-added"} {
		t.Run(order, func(t *testing.T) {
			c := newChatEnv(t)
			c.must(c.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
			l := c.mkLabel("journal")
			c.must(c.r.Labels.SetLabelsOf(ctx, domain.LabelTarget{Type: domain.TargetChat, ID: c.chat(oldGroup).ID}, []int64{l.ID}))
			added := func() { c.must(c.chats.BotAdded(ctx, forum(superGroup), strangerTG, member)) }
			moved := func() { c.must(c.chats.ChatMoved(ctx, "telegram", oldGroup, superGroup)) }
			if order == "added-then-moved" {
				added()
				moved()
			} else {
				moved()
				added()
			}
			ch := c.chat(superGroup)
			if ch.State != domain.ChatAccepted || ch.LeaveAt != nil {
				t.Fatalf("chat = %+v", ch)
			}
			if got, _ := c.r.Labels.LabelsOf(ctx, domain.LabelTarget{Type: domain.TargetChat, ID: ch.ID}); !reflect.DeepEqual(got, []int64{l.ID}) {
				t.Fatalf("labels = %v", got)
			}
			if _, err := c.r.Chats.GetByExternal(ctx, "telegram", oldGroup); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("old row still there: %v", err)
			}
			c.clock.Advance(5 * time.Minute)
			if n, _ := c.chats.LeaveDue(ctx); n != 0 || len(c.msgr.leftChats()) != 0 {
				t.Fatal("bot left the upgraded group")
			}
		})
	}
}

func TestChatMovedEitherOrderAndTwice(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
	old := c.chat(oldGroup)
	c.must(c.r.Topics.Create(ctx, &domain.Topic{ChatID: old.ID, ExternalID: "5", Title: "News", FirstSeenAt: c.clock.t, LastSeenAt: c.clock.t}))
	// The new supergroup's first message arrived before the migration message.
	if _, err := c.chats.ChatSeen(ctx, forum(superGroup)); err != nil {
		t.Fatal(err)
	}
	c.must(c.chats.ChatMoved(ctx, "telegram", oldGroup, superGroup))
	c.must(c.chats.ChatMoved(ctx, "telegram", oldGroup, superGroup))
	ch := c.chat(superGroup)
	if ch.State != domain.ChatAccepted {
		t.Fatalf("chat = %+v", ch)
	}
	if tps, _ := c.r.Topics.ListByChat(ctx, ch.ID); len(tps) != 1 || tps[0].Title != "News" {
		t.Fatalf("topics = %+v", tps)
	}
	if c.auditCount("chat.moved") != 1 {
		t.Fatalf("chat.moved audited %d times", c.auditCount("chat.moved"))
	}
}

func TestChatMovedRenamesInPlace(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
	id := c.chat(oldGroup).ID
	c.must(c.chats.ChatMoved(ctx, "telegram", oldGroup, superGroup))
	ch := c.chat(superGroup)
	if ch.ID != id || ch.Kind != domain.ChatKindSupergroup {
		t.Fatalf("chat = %+v", ch)
	}
}

func TestChatMovedRepointsPendingOutbox(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
	it := domain.OutboxItem{Messenger: "telegram", To: domain.Destination{ChatID: oldGroup}, Msg: domain.OutMessage{Text: "x"},
		State: domain.OutboxPending, NextAttemptAt: c.clock.t, CreatedAt: c.clock.t}
	c.must(c.r.Outbox.Add(ctx, &it))
	c.must(c.chats.ChatMoved(ctx, "telegram", oldGroup, superGroup))
	if got, _ := c.r.Outbox.Get(ctx, it.ID); got.To.ChatID != superGroup {
		t.Fatalf("outbox row points at %s", got.To.ChatID)
	}
}

func TestTopicNaming(t *testing.T) {
	c := newChatEnv(t)
	f := forum(superGroup)
	c.must(c.chats.TopicSeen(ctx, f, "9", ""))
	ch := c.chat(superGroup)
	tp, err := c.r.Topics.GetByExternal(ctx, ch.ID, "9")
	if err != nil || tp.Title != "" {
		t.Fatalf("unnamed topic = %+v, %v", tp, err)
	}
	c.must(c.chats.TopicSeen(ctx, f, "9", "News"))
	c.must(c.chats.RenameTopic(ctx, tp.ID, "  Announcements  ", actor(c.admin)))
	tp, _ = c.r.Topics.Get(ctx, tp.ID)
	if tp.Title != "Announcements" || !tp.TitleSetByAdmin {
		t.Fatalf("after admin rename = %+v", tp)
	}
	c.must(c.chats.TopicSeen(ctx, f, "9", "")) // a message without a name keeps the admin's
	if tp, _ = c.r.Topics.Get(ctx, tp.ID); tp.Title != "Announcements" {
		t.Fatalf("unnamed message overwrote the title: %+v", tp)
	}
	c.must(c.chats.TopicSeen(ctx, f, "9", "Notices")) // a Telegram rename wins
	if tp, _ = c.r.Topics.Get(ctx, tp.ID); tp.Title != "Notices" || tp.TitleSetByAdmin {
		t.Fatalf("after Telegram rename = %+v", tp)
	}
	var ve *domain.ValidationError
	if err := c.chats.RenameTopic(ctx, tp.ID, "  ", actor(c.admin)); !errors.As(err, &ve) || ve.Fields["title"] != "required" {
		t.Fatalf("empty title: %v", err)
	}
	if c.auditCount("topic.renamed") != 1 {
		t.Fatal("topic.renamed not audited")
	}
}

func TestChatSeenAndTopicSeenAreIdempotent(t *testing.T) {
	c := newChatEnv(t)
	for i := 0; i < 2; i++ {
		if _, err := c.chats.ChatSeen(ctx, forum(superGroup)); err != nil {
			t.Fatal(err)
		}
		c.must(c.chats.TopicSeen(ctx, forum(superGroup), "9", "News"))
	}
	list, err := c.chats.List(ctx)
	if err != nil || len(list) != 1 || len(list[0].Topics) != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if c.auditCount("chat.added") != 1 {
		t.Fatalf("chat.added audited %d times", c.auditCount("chat.added"))
	}
}

func TestIsAcceptedOnlyForAccepted(t *testing.T) {
	c := newChatEnv(t)
	if _, err := c.chats.ChatSeen(ctx, group(oldGroup)); err != nil {
		t.Fatal(err)
	}
	if ok, err := c.chats.IsAccepted(ctx, "telegram", oldGroup); err != nil || ok {
		t.Fatalf("new chat IsAccepted = %v, %v", ok, err)
	}
	c.must(c.chats.Accept(ctx, c.chat(oldGroup).ID, actor(c.admin)))
	if ok, _ := c.chats.IsAccepted(ctx, "telegram", oldGroup); !ok {
		t.Fatal("accepted chat not accepted")
	}
	if ok, err := c.chats.IsAccepted(ctx, "telegram", "-42"); err != nil || ok {
		t.Fatalf("unknown chat IsAccepted = %v, %v", ok, err)
	}
}

func TestMarkGoneAndTopicGone(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, forum(superGroup), adminTG, member))
	c.must(c.chats.TopicSeen(ctx, forum(superGroup), "9", "News"))
	c.must(c.chats.MarkTopicGone(ctx, "telegram", superGroup, "9"))
	if tp, _ := c.r.Topics.GetByExternal(ctx, c.chat(superGroup).ID, "9"); !tp.Gone {
		t.Fatal("topic not gone")
	}
	c.must(c.chats.TopicSeen(ctx, forum(superGroup), "9", ""))
	if tp, _ := c.r.Topics.GetByExternal(ctx, c.chat(superGroup).ID, "9"); tp.Gone {
		t.Fatal("a topic seen again must not stay gone")
	}
	c.must(c.chats.MarkGone(ctx, "telegram", superGroup))
	if ch := c.chat(superGroup); ch.State != domain.ChatLeft {
		t.Fatalf("chat = %+v", ch)
	}
}
