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

func newOutbox(t *testing.T) sqlite.Outbox { return sqlite.Outbox{DB: sqlitetest.New(t)} }

func queue(t *testing.T, o sqlite.Outbox, chat string, state domain.OutboxState, next, created time.Time) domain.OutboxItem {
	t.Helper()
	it := domain.OutboxItem{Messenger: "telegram", To: domain.Destination{ChatID: chat}, Msg: domain.OutMessage{Text: "hi"},
		State: state, NextAttemptAt: next, CreatedAt: created}
	if err := o.Add(ctx, &it); err != nil {
		t.Fatal(err)
	}
	return it
}

func TestOutboxRoundTrip(t *testing.T) {
	o := newOutbox(t)
	sent := t0.Add(time.Minute)
	it := domain.OutboxItem{Messenger: "telegram", To: domain.Destination{ChatID: "-1001000000001", ThreadID: "7"},
		Msg:    domain.OutMessage{Text: "📝 New application", Buttons: []domain.LinkButton{{Text: "Open", URL: "https://example.test/admin"}}},
		Origin: "application:1", State: domain.OutboxPending, NextAttemptAt: t0, CreatedAt: t0}
	if err := o.Add(ctx, &it); err != nil {
		t.Fatal(err)
	}
	got, err := o.Get(ctx, it.ID)
	if err != nil || !reflect.DeepEqual(got, it) {
		t.Fatalf("Get = %+v, %v; want %+v", got, err, it)
	}
	it.State, it.Attempts, it.LastError, it.SentAt = domain.OutboxSent, 2, "boom", &sent
	if err := o.Update(ctx, &it); err != nil {
		t.Fatal(err)
	}
	if got, _ = o.Get(ctx, it.ID); !reflect.DeepEqual(got, it) {
		t.Fatalf("after Update = %+v; want %+v", got, it)
	}
	if _, err := o.Get(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing row: %v", err)
	}
}

func TestOutboxDueOrderAndFilter(t *testing.T) {
	o := newOutbox(t)
	a := queue(t, o, "-1", domain.OutboxPending, t0.Add(-time.Minute), t0)
	queue(t, o, "-1", domain.OutboxPending, t0.Add(time.Minute), t0) // not yet
	queue(t, o, "-1", domain.OutboxSent, t0.Add(-time.Minute), t0)   // done
	b := queue(t, o, "-2", domain.OutboxPending, t0, t0)
	due, err := o.Due(ctx, t0, 10)
	if err != nil || len(due) != 2 || due[0].ID != a.ID || due[1].ID != b.ID {
		t.Fatalf("Due = %+v, %v", due, err)
	}
	if due, _ := o.Due(ctx, t0, 1); len(due) != 1 {
		t.Fatalf("limit ignored: %d rows", len(due))
	}
}

func TestOutboxRepointChatPendingOnly(t *testing.T) {
	o := newOutbox(t)
	p := queue(t, o, "-1", domain.OutboxPending, t0, t0)
	s := queue(t, o, "-1", domain.OutboxSent, t0, t0)
	if err := o.RepointChat(ctx, "telegram", "-1", "-1001"); err != nil {
		t.Fatal(err)
	}
	if got, _ := o.Get(ctx, p.ID); got.To.ChatID != "-1001" {
		t.Fatalf("pending row not repointed: %+v", got.To)
	}
	if got, _ := o.Get(ctx, s.ID); got.To.ChatID != "-1" {
		t.Fatalf("sent row repointed: %+v", got.To)
	}
}

func TestOutboxDeleteFinishedBefore(t *testing.T) {
	o := newOutbox(t)
	old := t0.Add(-31 * 24 * time.Hour)
	queue(t, o, "-1", domain.OutboxSent, old, old)
	queue(t, o, "-1", domain.OutboxFailed, old, old)
	p := queue(t, o, "-1", domain.OutboxPending, old, old)
	recent := queue(t, o, "-1", domain.OutboxSent, t0, t0)
	n, err := o.DeleteFinishedBefore(ctx, t0.Add(-30*24*time.Hour))
	if err != nil || n != 2 {
		t.Fatalf("DeleteFinishedBefore = %d, %v", n, err)
	}
	for _, id := range []int64{p.ID, recent.ID} {
		if _, err := o.Get(ctx, id); err != nil {
			t.Fatalf("row %d deleted: %v", id, err)
		}
	}
}

func TestRegTokenGetByHashAndMarkUsed(t *testing.T) {
	r := sqlite.RegTokens{DB: sqlitetest.New(t)}
	tok := domain.RegistrationToken{TokenHash: []byte("hash-1"), ChannelType: "telegram", ExternalID: "1000001", ChatID: "1000001",
		Username: "someone", Phone: "+380500000001", ExpiresAt: t0.Add(time.Hour), CreatedAt: t0}
	if err := r.Create(ctx, &tok); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetByHash(ctx, []byte("hash-1"))
	if err != nil || !reflect.DeepEqual(got, tok) {
		t.Fatalf("GetByHash = %+v, %v; want %+v", got, err, tok)
	}
	if err := r.MarkUsed(ctx, tok.ID, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetByHash(ctx, []byte("hash-1")); got.UsedAt == nil || !got.UsedAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("UsedAt = %v", got.UsedAt)
	}
	if _, err := r.GetByHash(ctx, []byte("nope")); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown hash: %v", err)
	}
}

func TestRegTokenInvalidateFor(t *testing.T) {
	r := sqlite.RegTokens{DB: sqlitetest.New(t)}
	mk := func(hash, ext string) domain.RegistrationToken {
		tok := domain.RegistrationToken{TokenHash: []byte(hash), ChannelType: "telegram", ExternalID: ext, ChatID: ext,
			Phone: "+380500000001", ExpiresAt: t0.Add(time.Hour), CreatedAt: t0}
		if err := r.Create(ctx, &tok); err != nil {
			t.Fatal(err)
		}
		return tok
	}
	mk("a", "1000001")
	mk("b", "1000002")
	if err := r.InvalidateFor(ctx, "telegram", "1000001", t0); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetByHash(ctx, []byte("a")); got.UsedAt == nil {
		t.Fatal("token of the account not invalidated")
	}
	if got, _ := r.GetByHash(ctx, []byte("b")); got.UsedAt != nil {
		t.Fatal("token of another account invalidated")
	}
}

func TestNotificationsDefaultAndSave(t *testing.T) {
	r := sqlite.Notifications{DB: sqlitetest.New(t)}
	got, err := r.Get(ctx)
	if err != nil || got.ApplicationsLabelID != nil {
		t.Fatalf("default = %+v, %v", got, err)
	}
	id := int64(7)
	if err := r.Save(ctx, domain.NotificationSettings{ApplicationsLabelID: &id}); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Get(ctx); got.ApplicationsLabelID == nil || *got.ApplicationsLabelID != 7 {
		t.Fatalf("after Save = %+v", got)
	}
	if err := r.Save(ctx, domain.NotificationSettings{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Get(ctx); got.ApplicationsLabelID != nil {
		t.Fatalf("after clearing = %+v", got)
	}
}

func TestRegTokenDeleteExpiredBefore(t *testing.T) {
	r := sqlite.RegTokens{DB: sqlitetest.New(t)}
	for i, exp := range []time.Time{t0.Add(-time.Minute), t0.Add(time.Minute)} {
		tok := domain.RegistrationToken{TokenHash: []byte{byte(i)}, ChannelType: "telegram", ExternalID: "1", ChatID: "1",
			Phone: "+380500000001", ExpiresAt: exp, CreatedAt: t0.Add(-time.Hour)}
		if err := r.Create(ctx, &tok); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := r.DeleteExpiredBefore(ctx, t0); err != nil || n != 1 {
		t.Fatalf("DeleteExpiredBefore = %d, %v", n, err)
	}
	if _, err := r.GetByHash(ctx, []byte{1}); err != nil {
		t.Fatalf("live token deleted: %v", err)
	}
}

func TestOutboxImageAndOrigin(t *testing.T) {
	o := newOutbox(t)
	for _, origin := range []string{"feed:1", "feed:2", "feed:1"} {
		it := domain.OutboxItem{Messenger: "telegram", To: domain.Destination{ChatID: "-1"}, Msg: domain.OutMessage{Text: "x", ImageAssetID: 9},
			Origin: origin, State: domain.OutboxPending, NextAttemptAt: t0, CreatedAt: t0}
		if err := o.Add(ctx, &it); err != nil {
			t.Fatal(err)
		}
	}
	got, err := o.ListByOrigin(ctx, "feed:1")
	if err != nil || len(got) != 2 || got[0].ID >= got[1].ID || got[0].Msg.ImageAssetID != 9 {
		t.Fatalf("ListByOrigin = %+v, %v", got, err)
	}
	if none, err := o.ListByOrigin(ctx, "feed:3"); err != nil || len(none) != 0 {
		t.Errorf("none = %+v, %v", none, err)
	}
}
