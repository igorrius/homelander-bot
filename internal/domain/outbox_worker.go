package domain

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"unicode/utf8"
)

const (
	outboxBatch   = 50
	maxErrorRunes = 500
)

// OutboxWorker delivers queued messages, one at a time, retrying what can
// get better and giving up on what cannot.
type OutboxWorker struct {
	R          Repos
	Now        Clock
	Messengers Messengers
	Chats      *ChatService
	Log        *slog.Logger

	Tick       time.Duration // how often to look for due rows
	Spacing    time.Duration // minimum gap between two sends to one chat
	MaxAge     time.Duration // give up this long after a row was queued
	BackoffMin time.Duration
	BackoffMax time.Duration

	wake     chan struct{}
	lastSent map[string]time.Time
}

// NewOutboxWorker returns a worker with the default timings.
func NewOutboxWorker(r Repos, now Clock, m Messengers, chats *ChatService, log *slog.Logger) *OutboxWorker {
	return &OutboxWorker{R: r, Now: now, Messengers: m, Chats: chats, Log: log,
		Tick: 5 * time.Second, Spacing: 3 * time.Second, MaxAge: 24 * time.Hour,
		BackoffMin: 10 * time.Second, BackoffMax: 30 * time.Minute,
		wake: make(chan struct{}, 1), lastSent: map[string]time.Time{}}
}

// Wake asks the worker to look for work now; it never blocks.
func (w *OutboxWorker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Run delivers until ctx is done.
func (w *OutboxWorker) Run(ctx context.Context) {
	t := time.NewTicker(w.Tick)
	defer t.Stop()
	for {
		if _, err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
			w.Log.Warn("outbox round failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-w.wake:
		}
	}
}

// RunOnce sends every due row it may send now and returns how many it attempted.
func (w *OutboxWorker) RunOnce(ctx context.Context) (int, error) {
	now := w.Now()
	due, err := w.R.Outbox.Due(ctx, now, outboxBatch)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, it := range due {
		if ctx.Err() != nil {
			return n, nil
		}
		key := it.Messenger + "/" + it.To.ChatID
		if last, ok := w.lastSent[key]; ok && now.Sub(last) < w.Spacing {
			continue
		}
		w.lastSent[key] = now
		n++
		if err := w.deliver(ctx, it, now); err != nil {
			return n, err
		}
	}
	return n, nil
}

func (w *OutboxWorker) deliver(ctx context.Context, it OutboxItem, now time.Time) error {
	m, err := w.Messengers.Get(it.Messenger)
	if err == nil {
		err = m.Send(ctx, it.To, it.Msg)
	}
	it.Attempts++
	if err == nil {
		it.State, it.SentAt, it.LastError = OutboxSent, &now, ""
		return w.R.Outbox.Update(ctx, &it)
	}
	it.LastError = truncateRunes(err.Error(), maxErrorRunes)
	var se *SendError
	if !errors.As(err, &se) {
		se = &SendError{Kind: SendRetry, Err: err}
	}
	switch se.Kind {
	case SendChatMoved:
		if err := w.Chats.ChatMoved(ctx, it.Messenger, it.To.ChatID, se.NewChatID); err != nil {
			return err
		}
		it.To.ChatID, it.NextAttemptAt = se.NewChatID, now
	case SendChatGone:
		if err := w.Chats.MarkGone(ctx, it.Messenger, it.To.ChatID); err != nil {
			return err
		}
		it.State = OutboxFailed
	case SendThreadGone:
		if err := w.Chats.MarkTopicGone(ctx, it.Messenger, it.To.ChatID, it.To.ThreadID); err != nil {
			return err
		}
		it.State = OutboxFailed
	case SendRetry:
		if now.Sub(it.CreatedAt) >= w.MaxAge {
			it.State = OutboxFailed
			break
		}
		it.NextAttemptAt = now.Add(max(se.RetryAfter, w.backoff(it.Attempts)))
	default:
		it.State = OutboxFailed
	}
	if it.State == OutboxFailed {
		w.Log.Warn("outbox message failed", "id", it.ID, "origin", it.Origin, "err", err)
	}
	return w.R.Outbox.Update(ctx, &it)
}

// backoff is BackoffMin doubled per earlier attempt, capped at BackoffMax.
func (w *OutboxWorker) backoff(attempts int) time.Duration {
	d := w.BackoffMin
	for i := 1; i < attempts && d < w.BackoffMax; i++ {
		d *= 2
	}
	return min(d, w.BackoffMax)
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
