package sqlite_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite/sqlitetest"
)


type chatRepos struct {
	db     sqlite.DB
	chats  sqlite.Chats
	topics sqlite.Topics
	labels sqlite.Labels
}

func newChatRepos(t *testing.T) chatRepos {
	db := sqlitetest.New(t)
	return chatRepos{db, sqlite.Chats{DB: db}, sqlite.Topics{DB: db}, sqlite.Labels{DB: db}}
}

func (r chatRepos) chat(t *testing.T, ext, title string, st domain.ChatState) domain.Chat {
	t.Helper()
	c := domain.Chat{Messenger: "telegram", ExternalID: ext, Kind: domain.ChatKindGroup, Title: title, State: st,
		BotStatus: domain.BotUnknown, FirstSeenAt: t0, LastSeenAt: t0}
	if err := r.chats.Create(ctx, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestChatsCRUD(t *testing.T) {
	r := newChatRepos(t)
	u := domain.User{Phone: "+380500000001", Status: domain.StatusActive, Source: domain.SourceAdmin}
	if err := (sqlite.Users{DB: r.db}).Create(ctx, &u); err != nil {
		t.Fatal(err)
	}
	acc, leave := t0.Add(time.Minute), t0.Add(2*time.Minute)
	c := domain.Chat{Messenger: "telegram", ExternalID: "-1001000000001", Kind: domain.ChatKindSupergroup, Title: "log",
		IsForum: true, State: domain.ChatAccepted, BotStatus: domain.BotAdministrator, CanSend: true,
		AddedByExternalID: "1000001", AddedByUserID: &u.ID, AcceptedBy: &u.ID, AcceptedAt: &acc, LeaveAt: &leave,
		FirstSeenAt: t0, LastSeenAt: t0.Add(time.Hour)}
	if err := r.chats.Create(ctx, &c); err != nil {
		t.Fatal(err)
	}
	got, err := r.chats.Get(ctx, c.ID)
	if err != nil || !reflect.DeepEqual(got, c) {
		t.Fatalf("Get = %+v, %v; want %+v", got, err, c)
	}
	byExt, err := r.chats.GetByExternal(ctx, "telegram", "-1001000000001")
	if err != nil || byExt.ID != c.ID {
		t.Fatalf("GetByExternal = %+v, %v", byExt, err)
	}
	left := t0.Add(3 * time.Hour)
	c.State, c.LeftAt, c.LeaveAt, c.AcceptedAt, c.Title = domain.ChatLeft, &left, nil, nil, "log 2"
	if err := r.chats.Update(ctx, &c); err != nil {
		t.Fatal(err)
	}
	got, _ = r.chats.Get(ctx, c.ID)
	if !reflect.DeepEqual(got, c) {
		t.Fatalf("after Update = %+v; want %+v", got, c)
	}
	if _, err := r.chats.GetByExternal(ctx, "telegram", "-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing chat: %v", err)
	}
	dup := domain.Chat{Messenger: "telegram", ExternalID: "-1001000000001", Kind: domain.ChatKindGroup, State: domain.ChatNew,
		BotStatus: domain.BotUnknown, FirstSeenAt: t0, LastSeenAt: t0}
	if err := r.chats.Create(ctx, &dup); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate external id: %v", err)
	}
	if err := r.chats.Delete(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.chats.Get(ctx, c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after Delete: %v", err)
	}
}

func TestChatsListOrder(t *testing.T) {
	r := newChatRepos(t)
	r.chat(t, "-3", "b left", domain.ChatLeft)
	r.chat(t, "-1", "z accepted", domain.ChatAccepted)
	r.chat(t, "-2", "a accepted", domain.ChatAccepted)
	r.chat(t, "-4", "y new", domain.ChatNew)
	list, err := r.chats.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, c := range list {
		titles = append(titles, c.Title)
	}
	want := []string{"y new", "a accepted", "z accepted", "b left"}
	if !reflect.DeepEqual(titles, want) {
		t.Fatalf("order = %v; want %v", titles, want)
	}
	if n, err := r.chats.CountByState(ctx, domain.ChatAccepted); err != nil || n != 2 {
		t.Fatalf("CountByState = %d, %v", n, err)
	}
}

func TestChatsDueLeaves(t *testing.T) {
	r := newChatRepos(t)
	past, future := t0.Add(-time.Second), t0.Add(time.Minute)
	due := r.chat(t, "-1", "due", domain.ChatNew)
	due.LeaveAt = &past
	notYet := r.chat(t, "-2", "later", domain.ChatNew)
	notYet.LeaveAt = &future
	accepted := r.chat(t, "-3", "accepted", domain.ChatAccepted)
	accepted.LeaveAt = &past
	for _, c := range []*domain.Chat{&due, &notYet, &accepted} {
		if err := r.chats.Update(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	r.chat(t, "-4", "no leave", domain.ChatNew)
	list, err := r.chats.DueLeaves(ctx, t0)
	if err != nil || len(list) != 1 || list[0].ID != due.ID {
		t.Fatalf("DueLeaves = %+v, %v", list, err)
	}
}

func TestTopicsUniquePerChat(t *testing.T) {
	r := newChatRepos(t)
	a, b := r.chat(t, "-1", "a", domain.ChatAccepted), r.chat(t, "-2", "b", domain.ChatAccepted)
	tp := domain.Topic{ChatID: a.ID, ExternalID: "7", Title: "News", FirstSeenAt: t0, LastSeenAt: t0}
	if err := r.topics.Create(ctx, &tp); err != nil {
		t.Fatal(err)
	}
	if err := r.topics.Create(ctx, &domain.Topic{ChatID: a.ID, ExternalID: "7", FirstSeenAt: t0, LastSeenAt: t0}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate topic: %v", err)
	}
	if err := r.topics.Create(ctx, &domain.Topic{ChatID: b.ID, ExternalID: "7", FirstSeenAt: t0, LastSeenAt: t0}); err != nil {
		t.Fatalf("same thread id in another chat: %v", err)
	}
	tp.Title, tp.TitleSetByAdmin, tp.Gone, tp.LastSeenAt = "Renamed", true, true, t0.Add(time.Hour)
	if err := r.topics.Update(ctx, &tp); err != nil {
		t.Fatal(err)
	}
	got, err := r.topics.GetByExternal(ctx, a.ID, "7")
	if err != nil || !reflect.DeepEqual(got, tp) {
		t.Fatalf("GetByExternal = %+v, %v; want %+v", got, err, tp)
	}
	if g, err := r.topics.Get(ctx, tp.ID); err != nil || g.ID != tp.ID {
		t.Fatalf("Get = %+v, %v", g, err)
	}
}

func TestTopicsMoveToChat(t *testing.T) {
	r := newChatRepos(t)
	from, to := r.chat(t, "-1", "old", domain.ChatAccepted), r.chat(t, "-2", "new", domain.ChatAccepted)
	for _, tp := range []domain.Topic{{ChatID: from.ID, ExternalID: "5"}, {ChatID: from.ID, ExternalID: "6"}, {ChatID: to.ID, ExternalID: "6"}} {
		tp.FirstSeenAt, tp.LastSeenAt = t0, t0
		if err := r.topics.Create(ctx, &tp); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.topics.MoveToChat(ctx, from.ID, to.ID); err != nil {
		t.Fatal(err)
	}
	moved, _ := r.topics.ListByChat(ctx, to.ID)
	left, _ := r.topics.ListByChat(ctx, from.ID)
	if len(moved) != 2 || len(left) != 1 || left[0].ExternalID != "6" {
		t.Fatalf("moved = %+v; left behind = %+v", moved, left)
	}
}

func TestChatDeleteCascadesTopics(t *testing.T) {
	r := newChatRepos(t)
	c := r.chat(t, "-1", "a", domain.ChatAccepted)
	if err := r.topics.Create(ctx, &domain.Topic{ChatID: c.ID, ExternalID: "1", FirstSeenAt: t0, LastSeenAt: t0}); err != nil {
		t.Fatal(err)
	}
	if err := r.chats.Delete(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := r.topics.ListByChat(ctx, c.ID); len(list) != 0 {
		t.Fatalf("topics left after chat delete: %+v", list)
	}
}
