package telegram

import (
	"context"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

const (
	pollTimeout = 50 // seconds Telegram holds a getUpdates call open
	maxBackoff  = 30 * time.Second
)

var allowedUpdates = []string{"message", "my_chat_member"}

// RunPolling receives updates by long polling until ctx is done. It removes a
// webhook first, because Telegram allows only one way at a time.
func (b *Bot) RunPolling(ctx context.Context) error {
	if _, err := b.api.DeleteWebhook(nil); err != nil {
		b.log.Warn("telegram deleteWebhook failed", "err", err)
	}
	var offset int64
	backoff := time.Second
	for ctx.Err() == nil {
		updates, err := b.api.GetUpdates(&gotgbot.GetUpdatesOpts{
			Offset: offset, Timeout: pollTimeout, AllowedUpdates: allowedUpdates,
			RequestOpts: &gotgbot.RequestOpts{Timeout: (pollTimeout + 10) * time.Second},
		})
		if err != nil {
			b.log.Warn("telegram getUpdates failed", "err", err, "retry_in", backoff)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}
		backoff = time.Second
		for _, u := range updates {
			if ctx.Err() != nil {
				return nil
			}
			b.HandleUpdate(ctx, u)
			offset = u.UpdateId + 1
		}
	}
	return nil
}
