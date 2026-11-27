package domain_test

import (
	"context"
	"sync"

	"github.com/igorrius/homelander-bot/internal/domain"
)

type sentMessage struct{ ChatID, Text string }

type fakeMessenger struct {
	mu   sync.Mutex
	sent []sentMessage
}

var _ domain.Messenger = (*fakeMessenger)(nil)

func (f *fakeMessenger) Type() string   { return "telegram" }
func (f *fakeMessenger) BotURL() string { return "https://t.me/test_bot" }
func (f *fakeMessenger) SendText(_ context.Context, chatID, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentMessage{chatID, text})
	return nil
}

func (f *fakeMessenger) messages() []sentMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentMessage(nil), f.sent...)
}
