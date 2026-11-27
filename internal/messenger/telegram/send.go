package telegram

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Send posts m to a chat or a forum topic; failures are *domain.SendError.
func (b *Bot) Send(_ context.Context, to domain.Destination, m domain.OutMessage) error {
	chatID, err := strconv.ParseInt(to.ChatID, 10, 64)
	if err != nil {
		return &domain.SendError{Kind: domain.SendPermanent, Err: fmt.Errorf("telegram chat id %q: %w", to.ChatID, err)}
	}
	opts := &gotgbot.SendMessageOpts{}
	if to.ThreadID != "" {
		thread, err := strconv.ParseInt(to.ThreadID, 10, 64)
		if err != nil {
			return &domain.SendError{Kind: domain.SendPermanent, Err: fmt.Errorf("telegram thread id %q: %w", to.ThreadID, err)}
		}
		opts.MessageThreadId = thread
	}
	if len(m.Buttons) > 0 {
		kb := gotgbot.InlineKeyboardMarkup{}
		for _, btn := range m.Buttons {
			kb.InlineKeyboard = append(kb.InlineKeyboard, []gotgbot.InlineKeyboardButton{{Text: btn.Text, Url: btn.URL}})
		}
		opts.ReplyMarkup = kb
	}
	if _, err := b.api.SendMessage(chatID, m.Text, opts); err != nil {
		return classify(err)
	}
	return nil
}

// LeaveChat makes the bot leave a chat; nil when it is already not there.
func (b *Bot) LeaveChat(_ context.Context, chatID string) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram chat id %q: %w", chatID, err)
	}
	if _, err := b.api.LeaveChat(id, nil); err != nil {
		if se := classify(err); se.Kind == domain.SendChatGone {
			return nil
		}
		return err
	}
	return nil
}

// classify sorts a Bot API failure into what the outbox should do.
func classify(err error) *domain.SendError {
	var te *gotgbot.TelegramError
	if !errors.As(err, &te) {
		return &domain.SendError{Kind: domain.SendRetry, Err: err}
	}
	desc := strings.ToLower(te.Description)
	switch {
	case te.ResponseParams != nil && te.ResponseParams.MigrateToChatId != 0:
		return &domain.SendError{Kind: domain.SendChatMoved, NewChatID: strconv.FormatInt(te.ResponseParams.MigrateToChatId, 10), Err: err}
	case te.Code == 429:
		var after time.Duration
		if te.ResponseParams != nil {
			after = time.Duration(te.ResponseParams.RetryAfter) * time.Second
		}
		return &domain.SendError{Kind: domain.SendRetry, RetryAfter: after, Err: err}
	case te.Code >= 500:
		return &domain.SendError{Kind: domain.SendRetry, Err: err}
	case te.Code == 403, strings.Contains(desc, "chat not found"):
		return &domain.SendError{Kind: domain.SendChatGone, Err: err}
	case strings.Contains(desc, "message thread not found"), strings.Contains(desc, "topic_deleted"):
		return &domain.SendError{Kind: domain.SendThreadGone, Err: err}
	}
	return &domain.SendError{Kind: domain.SendPermanent, Err: err}
}
