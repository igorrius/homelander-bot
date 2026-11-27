package telegram

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

const maxWebhookBody = 1 << 20

// WebhookPath is the unguessable path Telegram posts updates to.
func WebhookPath(secret []byte) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("telegram-webhook"))
	return "/telegram/webhook/" + hex.EncodeToString(m.Sum(nil))[:32]
}

// StartWebhook registers url with Telegram; updates then arrive at WebhookHandler.
func (b *Bot) StartWebhook(_ context.Context, url, secret string) error {
	_, err := b.api.SetWebhook(url, &gotgbot.SetWebhookOpts{SecretToken: secret, AllowedUpdates: allowedUpdates})
	return err
}

// WebhookHandler accepts updates whose secret header matches. A body that does
// not decode is answered 200 so Telegram does not retry it forever.
func (b *Bot) WebhookHandler(secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		got := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
		if secret == "" || subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var u gotgbot.Update
		if err := json.Unmarshal(body, &u); err != nil {
			b.log.Warn("telegram webhook: undecodable update", "err", err)
			w.WriteHeader(http.StatusOK)
			return
		}
		b.HandleUpdate(context.WithoutCancel(r.Context()), u)
		w.WriteHeader(http.StatusOK)
	})
}
