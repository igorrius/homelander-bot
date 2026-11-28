package domain_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

type ctxT = context.Context

func (c *chatEnv) worker() *domain.OutboxWorker {
	return domain.NewOutboxWorker(c.r, c.clock.Now, domain.Messengers{"telegram": c.msgr}, c.chats,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (c *chatEnv) enqueue(chat, thread string) domain.OutboxItem {
	c.t.Helper()
	it := domain.OutboxItem{Messenger: "telegram", To: domain.Destination{ChatID: chat, ThreadID: thread},
		Msg: domain.OutMessage{Text: "hi"}, State: domain.OutboxPending, NextAttemptAt: c.clock.t, CreatedAt: c.clock.t}
	c.must(c.r.Outbox.Add(ctx, &it))
	return it
}

func (c *chatEnv) row(id int64) domain.OutboxItem {
	c.t.Helper()
	it, err := c.r.Outbox.Get(ctx, id)
	c.must(err)
	return it
}

func (c *chatEnv) runOnce(w *domain.OutboxWorker) int {
	c.t.Helper()
	n, err := w.RunOnce(ctx)
	c.must(err)
	return n
}

func sendErr(kind domain.SendErrorKind) error {
	return &domain.SendError{Kind: kind, Err: errors.New(string(kind))}
}

func TestWorkerSendsAndMarksSent(t *testing.T) {
	c := newChatEnv(t)
	it := c.enqueue(superGroup, "9")
	if n := c.runOnce(c.worker()); n != 1 {
		t.Fatalf("attempted %d", n)
	}
	got := c.row(it.ID)
	if got.State != domain.OutboxSent || got.SentAt == nil || got.Attempts != 1 {
		t.Fatalf("row = %+v", got)
	}
	d := c.msgr.delivered()
	if len(d) != 1 || d[0].To != (domain.Destination{ChatID: superGroup, ThreadID: "9"}) || d[0].Msg.Text != "hi" {
		t.Fatalf("delivered = %+v", d)
	}
}

func TestWorkerRetryBackoff(t *testing.T) {
	c := newChatEnv(t)
	w := c.worker()
	it := c.enqueue(superGroup, "")
	for i, want := range []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second} {
		c.msgr.sendErrs = []error{sendErr(domain.SendRetry)}
		c.runOnce(w)
		got := c.row(it.ID)
		if got.State != domain.OutboxPending || !got.NextAttemptAt.Equal(c.clock.t.Add(want)) || got.Attempts != i+1 {
			t.Fatalf("try %d: row = %+v; want next in %v", i+1, got, want)
		}
		if got.LastError == "" {
			t.Fatal("LastError not recorded")
		}
		c.clock.Advance(want)
	}
	// RetryAfter longer than the backoff wins.
	c.msgr.sendErrs = []error{&domain.SendError{Kind: domain.SendRetry, RetryAfter: 10 * time.Minute, Err: errors.New("429")}}
	c.runOnce(w)
	if got := c.row(it.ID); !got.NextAttemptAt.Equal(c.clock.t.Add(10 * time.Minute)) {
		t.Fatalf("RetryAfter ignored: %+v", got)
	}
}

func TestWorkerBackoffCapped(t *testing.T) {
	c := newChatEnv(t)
	w := c.worker()
	it := c.enqueue(superGroup, "")
	it.Attempts = 20
	c.must(c.r.Outbox.Update(ctx, &it))
	c.msgr.sendErrs = []error{sendErr(domain.SendRetry)}
	c.runOnce(w)
	if got := c.row(it.ID); !got.NextAttemptAt.Equal(c.clock.t.Add(30 * time.Minute)) {
		t.Fatalf("next = %v; want +30m", got.NextAttemptAt.Sub(c.clock.t))
	}
}

func TestWorkerGivesUpAfter24h(t *testing.T) {
	c := newChatEnv(t)
	w := c.worker()
	it := c.enqueue(superGroup, "")
	c.clock.Advance(24 * time.Hour)
	c.msgr.sendErrs = []error{sendErr(domain.SendRetry)}
	c.runOnce(w)
	if got := c.row(it.ID); got.State != domain.OutboxFailed {
		t.Fatalf("row = %+v", got)
	}
}

func TestWorkerChatMovedRetriesAtNewID(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
	w := c.worker()
	it := c.enqueue(oldGroup, "")
	c.msgr.sendErrs = []error{&domain.SendError{Kind: domain.SendChatMoved, NewChatID: superGroup, Err: errors.New("moved")}}
	c.runOnce(w)
	got := c.row(it.ID)
	if got.State != domain.OutboxPending || got.To.ChatID != superGroup || !got.NextAttemptAt.Equal(c.clock.t) {
		t.Fatalf("row = %+v", got)
	}
	if ch := c.chat(superGroup); ch.State != domain.ChatAccepted {
		t.Fatalf("chat not moved: %+v", ch)
	}
	c.clock.Advance(5 * time.Second)
	c.runOnce(w)
	if got := c.row(it.ID); got.State != domain.OutboxSent {
		t.Fatalf("not sent at the new id: %+v", got)
	}
}

func TestWorkerChatGoneMarksChatLeft(t *testing.T) {
	c := newChatEnv(t)
	c.must(c.chats.BotAdded(ctx, group(oldGroup), adminTG, member))
	it := c.enqueue(oldGroup, "")
	c.msgr.sendErrs = []error{sendErr(domain.SendChatGone)}
	c.runOnce(c.worker())
	if got := c.row(it.ID); got.State != domain.OutboxFailed {
		t.Fatalf("row = %+v", got)
	}
	if ch := c.chat(oldGroup); ch.State != domain.ChatLeft {
		t.Fatalf("chat = %+v", ch)
	}
}

func TestWorkerThreadGoneMarksTopic(t *testing.T) {
	c := newChatEnv(t)
	ch, _ := c.acceptedForum(superGroup, "9")
	it := c.enqueue(superGroup, "9")
	c.msgr.sendErrs = []error{sendErr(domain.SendThreadGone)}
	c.runOnce(c.worker())
	if got := c.row(it.ID); got.State != domain.OutboxFailed {
		t.Fatalf("row = %+v", got)
	}
	if tp, _ := c.r.Topics.GetByExternal(ctx, ch.ID, "9"); !tp.Gone {
		t.Fatal("topic not marked gone")
	}
}

func TestWorkerPermanentFails(t *testing.T) {
	c := newChatEnv(t)
	it := c.enqueue(superGroup, "")
	c.msgr.sendErrs = []error{sendErr(domain.SendPermanent)}
	c.runOnce(c.worker())
	if got := c.row(it.ID); got.State != domain.OutboxFailed || got.LastError == "" {
		t.Fatalf("row = %+v", got)
	}
}

func TestWorkerSpacesSendsPerChat(t *testing.T) {
	c := newChatEnv(t)
	w := c.worker()
	a := c.enqueue(superGroup, "")
	b := c.enqueue(superGroup, "9") // same chat, another topic
	other := c.enqueue(oldGroup, "")
	if n := c.runOnce(w); n != 2 {
		t.Fatalf("attempted %d; want 2 (one per chat)", n)
	}
	if c.row(a.ID).State != domain.OutboxSent || c.row(other.ID).State != domain.OutboxSent || c.row(b.ID).State != domain.OutboxPending {
		t.Fatal("wrong rows sent in the first round")
	}
	c.clock.Advance(2 * time.Second)
	if n := c.runOnce(w); n != 0 {
		t.Fatalf("sent to the same chat after 2 s (%d)", n)
	}
	c.clock.Advance(time.Second)
	if n := c.runOnce(w); n != 1 || c.row(b.ID).State != domain.OutboxSent {
		t.Fatal("second row not sent after 3 s")
	}
}

func TestWorkerRetriesUntilMessengerRecovers(t *testing.T) {
	c := newChatEnv(t)
	w := c.worker()
	first := c.enqueue(superGroup, "")
	for i := 0; i < 6; i++ {
		c.msgr.sendErrs = []error{sendErr(domain.SendRetry)}
		c.runOnce(w)
		c.clock.Advance(30 * time.Minute)
	}
	second := c.enqueue(superGroup, "")
	c.runOnce(w)
	c.clock.Advance(5 * time.Second)
	c.runOnce(w)
	if c.row(first.ID).State != domain.OutboxSent || c.row(second.ID).State != domain.OutboxSent {
		t.Fatal("rows not delivered after the messenger recovered")
	}
	d := c.msgr.delivered()
	if len(d) != 2 {
		t.Fatalf("delivered %d", len(d))
	}
}

func TestWorkerRunStopsAndWakes(t *testing.T) {
	c := newChatEnv(t)
	w := c.worker()
	w.Tick = time.Hour
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { w.Run(runCtx); close(done) }()
	c.enqueue(superGroup, "")
	w.Wake()
	deadline := time.Now().Add(5 * time.Second)
	for len(c.msgr.delivered()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if len(c.msgr.delivered()) != 1 {
		t.Fatal("Wake did not trigger a send")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestWorkerKeepsOrderAfterOutage(t *testing.T) {
	c := newChatEnv(t)
	w := c.worker()
	var ids []int64
	for i := 0; i < 5; i++ {
		ids = append(ids, c.enqueue(superGroup, "").ID)
		for j := 0; j < 6; j++ { // Telegram is down for a while between notices
			c.msgr.sendErrs = []error{sendErr(domain.SendRetry), sendErr(domain.SendRetry), sendErr(domain.SendRetry)}
			c.runOnce(w)
			c.clock.Advance(7 * time.Minute)
		}
	}
	c.msgr.sendErrs = nil
	// The oldest row may still wait out its backoff (capped at 30 minutes);
	// the others follow it, in order, a few seconds apart.
	for i := 0; i < 700 && len(c.msgr.delivered()) < 5; i++ {
		c.runOnce(w)
		c.clock.Advance(3 * time.Second)
	}
	d := c.msgr.delivered()
	if len(d) != 5 {
		t.Fatalf("delivered %d of 5 within 35 minutes of recovery", len(d))
	}
	for i, id := range ids {
		if got := c.row(id); got.State != domain.OutboxSent {
			t.Fatalf("row %d = %+v", i, got)
		}
	}
	var order []int64
	rows, _ := c.db.Query(`SELECT id FROM outbox ORDER BY sent_at, id`)
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		order = append(order, id)
	}
	_ = rows.Close()
	for i := range ids {
		if order[i] != ids[i] {
			t.Fatalf("sent in order %v; want %v", order, ids)
		}
	}
}

func TestWorkerBacklogDoesNotStarveOtherChats(t *testing.T) {
	c := newChatEnv(t)
	w := c.worker()
	for i := 0; i < 60; i++ {
		c.enqueue(superGroup, "")
	}
	other := c.enqueue(oldGroup, "")
	c.runOnce(w)
	if got := c.row(other.ID); got.State != domain.OutboxSent {
		t.Fatalf("other chat starved: %+v", got)
	}
}

func TestWorkerSendsPhotoFromAsset(t *testing.T) {
	c := newChatEnv(t)
	pic := c.asset("image/png")
	it := domain.OutboxItem{Messenger: "telegram", To: domain.Destination{ChatID: superGroup},
		Msg: domain.OutMessage{Text: "caption", ImageAssetID: pic}, State: domain.OutboxPending, NextAttemptAt: c.clock.t, CreatedAt: c.clock.t}
	c.must(c.r.Outbox.Add(ctx, &it))
	c.runOnce(c.worker())
	d := c.msgr.delivered()
	if len(d) != 1 || d[0].Msg.Image == nil || d[0].Msg.Image.ContentType != "image/png" || len(d[0].Msg.Image.Data) != 1 {
		t.Fatalf("delivered = %+v", d)
	}
	if got := c.row(it.ID); got.State != domain.OutboxSent {
		t.Errorf("row = %+v", got)
	}
}

func TestWorkerMissingPhotoFailsAtOnce(t *testing.T) {
	c := newChatEnv(t)
	it := domain.OutboxItem{Messenger: "telegram", To: domain.Destination{ChatID: superGroup},
		Msg: domain.OutMessage{Text: "caption", ImageAssetID: 4242}, State: domain.OutboxPending, NextAttemptAt: c.clock.t, CreatedAt: c.clock.t}
	c.must(c.r.Outbox.Add(ctx, &it))
	c.runOnce(c.worker())
	got := c.row(it.ID)
	if got.State != domain.OutboxFailed || !strings.Contains(got.LastError, "image asset 4242") || len(c.msgr.delivered()) != 0 {
		t.Fatalf("row = %+v", got)
	}
}
