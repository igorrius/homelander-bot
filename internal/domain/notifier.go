package domain

import (
	"context"
	"errors"
	"time"
)

const outboxRetention = 30 * 24 * time.Hour

// ErrChatNotAccepted is returned when a test send finds nothing that may receive it.
var ErrChatNotAccepted = errors.New("chat_not_accepted")

// Notifier queues messages in the outbox. Call it inside the transaction of
// the event the message is about, so both are kept or lost together.
type Notifier struct {
	R             Repos
	Now           Clock
	I18n          Translator
	DefaultLocale func(ctx context.Context) string // site setting, else the configured default
	Wake          func()                           // tells the worker there is work; may be nil
}

type destination struct {
	messenger string
	to        Destination
}

// resolve returns the accepted places a label hangs on, without duplicates.
func (n *Notifier) resolve(ctx context.Context, labelID int64) ([]destination, error) {
	targets, err := n.R.Labels.Targets(ctx, labelID)
	if err != nil {
		return nil, err
	}
	var out []destination
	seen := map[destination]bool{}
	for _, t := range targets {
		d, ok, err := n.target(ctx, t)
		if err != nil {
			return nil, err
		}
		if ok && !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out, nil
}

// target resolves one chat or topic; ok is false when it receives nothing.
func (n *Notifier) target(ctx context.Context, t LabelTarget) (destination, bool, error) {
	var thread string
	chatID := t.ID
	if t.Type == TargetTopic {
		tp, err := n.R.Topics.Get(ctx, t.ID)
		if err != nil {
			return destination{}, false, err
		}
		if tp.Gone {
			return destination{}, false, nil
		}
		chatID, thread = tp.ChatID, tp.ExternalID
	}
	ch, err := n.R.Chats.Get(ctx, chatID)
	if err != nil {
		return destination{}, false, err
	}
	if ch.State != ChatAccepted {
		return destination{}, false, nil
	}
	return destination{ch.Messenger, Destination{ChatID: ch.ExternalID, ThreadID: thread}}, true, nil
}

func (n *Notifier) add(ctx context.Context, d destination, m OutMessage, origin string, labelID, userID *int64) (int64, error) {
	now := n.Now()
	it := OutboxItem{Messenger: d.messenger, To: d.to, Msg: m, Origin: origin, LabelID: labelID, UserID: userID,
		State: OutboxPending, NextAttemptAt: now, CreatedAt: now}
	if err := n.R.Outbox.Add(ctx, &it); err != nil {
		return 0, err
	}
	if n.Wake != nil {
		n.Wake()
	}
	return it.ID, nil
}

// SendToLabel queues m for every accepted place carrying the label and
// returns how many rows it queued.
func (n *Notifier) SendToLabel(ctx context.Context, labelID int64, m OutMessage, origin string) (int, error) {
	ids, err := n.sendToLabel(ctx, labelID, m, origin)
	return len(ids), err
}

func (n *Notifier) sendToLabel(ctx context.Context, labelID int64, m OutMessage, origin string) ([]int64, error) {
	return n.SendToLabels(ctx, []int64{labelID}, m, origin)
}

// SendToLabels queues m once for every accepted place carrying any of the
// labels: a chat or topic with two of them gets one message. Each row records
// the first of the labels that reaches its place. It returns the queued ids.
func (n *Notifier) SendToLabels(ctx context.Context, labelIDs []int64, m OutMessage, origin string) ([]int64, error) {
	var ids []int64
	seen := map[destination]bool{}
	for _, labelID := range labelIDs {
		dests, err := n.resolve(ctx, labelID)
		if err != nil {
			return nil, err
		}
		for _, d := range dests {
			if seen[d] {
				continue
			}
			seen[d] = true
			id, err := n.add(ctx, d, m, origin, &labelID, nil)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// SendToUser queues m for the user's linked messenger account; false when there is none.
func (n *Notifier) SendToUser(ctx context.Context, userID int64, m OutMessage, origin string) (bool, error) {
	chans, err := n.R.Channels.ListByUser(ctx, userID)
	if err != nil || len(chans) == 0 {
		return false, err
	}
	c := chans[0]
	_, err = n.add(ctx, destination{c.Type, Destination{ChatID: c.ChatID}}, m, origin, nil, &userID)
	return err == nil, err
}

func (n *Notifier) testMessage(ctx context.Context) OutMessage {
	return OutMessage{Text: n.I18n.T(n.DefaultLocale(ctx), "notify.test", nil, nil)}
}

func (n *Notifier) testTarget(ctx context.Context, t LabelTarget) ([]int64, error) {
	var ids []int64
	err := n.R.Tx.InTx(ctx, func(ctx context.Context) error {
		d, ok, err := n.target(ctx, t)
		if err != nil {
			return err
		}
		if !ok {
			return ErrChatNotAccepted
		}
		id, err := n.add(ctx, d, n.testMessage(ctx), "test", nil, nil)
		ids = []int64{id}
		return err
	})
	return ids, err
}

// TestChat queues a test message to a chat.
func (n *Notifier) TestChat(ctx context.Context, chatID int64) ([]int64, error) {
	return n.testTarget(ctx, LabelTarget{TargetChat, chatID})
}

// TestTopic queues a test message to a forum topic.
func (n *Notifier) TestTopic(ctx context.Context, topicID int64) ([]int64, error) {
	return n.testTarget(ctx, LabelTarget{TargetTopic, topicID})
}

// TestLabel queues a test message to every place carrying the label.
func (n *Notifier) TestLabel(ctx context.Context, labelID int64) ([]int64, error) {
	var ids []int64
	err := n.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if _, err := n.R.Labels.Get(ctx, labelID); err != nil {
			return err
		}
		var err error
		if ids, err = n.sendToLabel(ctx, labelID, n.testMessage(ctx), "test"); err != nil {
			return err
		}
		if len(ids) == 0 {
			return ErrChatNotAccepted
		}
		return nil
	})
	return ids, err
}

// Status returns one queued message (for showing a test send's outcome).
func (n *Notifier) Status(ctx context.Context, id int64) (OutboxItem, error) {
	return n.R.Outbox.Get(ctx, id)
}

// Clean deletes sent and failed rows older than 30 days.
func (n *Notifier) Clean(ctx context.Context) (int, error) {
	return n.R.Outbox.DeleteFinishedBefore(ctx, n.Now().Add(-outboxRetention))
}
