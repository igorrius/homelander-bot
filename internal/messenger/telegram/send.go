package telegram

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
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
	var thread int64
	if to.ThreadID != "" {
		if thread, err = strconv.ParseInt(to.ThreadID, 10, 64); err != nil {
			return &domain.SendError{Kind: domain.SendPermanent, Err: fmt.Errorf("telegram thread id %q: %w", to.ThreadID, err)}
		}
	}
	var markup gotgbot.ReplyMarkup
	if len(m.Buttons) > 0 {
		kb := gotgbot.InlineKeyboardMarkup{}
		for _, btn := range m.Buttons {
			kb.InlineKeyboard = append(kb.InlineKeyboard, []gotgbot.InlineKeyboardButton{{Text: btn.Text, Url: btn.URL}})
		}
		markup = kb
	}
	if m.Image != nil {
		// The text is a plain caption, like a plain message: no parse mode.
		photo := gotgbot.InputFileByReader(photoName(m.Image.ContentType), bytes.NewReader(m.Image.Data))
		_, err = b.api.SendPhoto(chatID, photo, &gotgbot.SendPhotoOpts{Caption: m.Text, MessageThreadId: thread, ReplyMarkup: markup,
			RequestOpts: &gotgbot.RequestOpts{Timeout: b.photoTimeout}})
	} else {
		_, err = b.api.SendMessage(chatID, m.Text, &gotgbot.SendMessageOpts{MessageThreadId: thread, ReplyMarkup: markup,
			RequestOpts: &gotgbot.RequestOpts{Timeout: b.sendTimeout}})
	}
	if err != nil {
		return classify(err)
	}
	return nil
}

// photoName is the file name a photo is uploaded under; Telegram looks at
// its extension.
func photoName(contentType string) string {
	switch contentType {
	case "image/png":
		return "photo.png"
	case "image/webp":
		return "photo.webp"
	case "image/gif":
		return "photo.gif"
	}
	return "photo.jpg"
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

// timedOut reports whether a request was cut off by its timeout.
func timedOut(err error) bool {
	var ne net.Error
	return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout())
}

// classify sorts a Bot API failure into what the outbox should do.
func classify(err error) *domain.SendError {
	var te *gotgbot.TelegramError
	if !errors.As(err, &te) {
		if timedOut(err) {
			// The request may have reached Telegram before we stopped
			// waiting: no quick resend that would post it twice.
			return &domain.SendError{Kind: domain.SendRetry, RetryAfter: AmbiguousRetry,
				Err: fmt.Errorf("timed out, Telegram may have posted it already: %w", err)}
		}
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
