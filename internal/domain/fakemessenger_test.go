package domain_test

import (
	"context"
	"sync"

	"github.com/igorrius/homelander-bot/internal/domain"
)

type sentMessage struct{ ChatID, Text string }

type sentTo struct {
	To  domain.Destination
	Msg domain.OutMessage
}

// fakeMessenger records what it is asked to send. sendErrs are returned by
// Send one per call, in order; nil entries and an empty queue mean success.
type fakeMessenger struct {
	mu       sync.Mutex
	sent     []sentMessage
	sends    []sentTo
	sendErrs []error
	left     []string
	leaveErr error
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

func (f *fakeMessenger) Send(_ context.Context, to domain.Destination, m domain.OutMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sendErrs) > 0 {
		err := f.sendErrs[0]
		f.sendErrs = f.sendErrs[1:]
		if err != nil {
			return err
		}
	}
	f.sends = append(f.sends, sentTo{to, m})
	return nil
}

func (f *fakeMessenger) LeaveChat(_ context.Context, chatID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.left = append(f.left, chatID)
	return f.leaveErr
}

func (f *fakeMessenger) messages() []sentMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentMessage(nil), f.sent...)
}

func (f *fakeMessenger) delivered() []sentTo {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentTo(nil), f.sends...)
}

func (f *fakeMessenger) leftChats() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.left...)
}
