package domain

import "context"

// Messenger delivers texts to a user's linked messenger account. Texts arrive
// already localized. Implemented by internal/messenger/telegram.
type Messenger interface {
	Type() string
	SendText(ctx context.Context, chatID, text string) error
	BotURL() string
}

// Translator renders localized texts (implemented by internal/i18n.Bundle).
type Translator interface {
	T(locale, id string, data map[string]any, count any) string
	Pick(candidates ...string) string
}
