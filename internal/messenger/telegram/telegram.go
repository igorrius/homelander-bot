// Package telegram connects Homelander to Telegram: it implements
// domain.Messenger and turns updates into domain calls. Private chats handle
// activation; chats and topics the bot is in are reported to domain.ChatEvents;
// messages from accepted groups go to a GroupSink and are never answered (how
// the bot reacts to them is configured later as flows).
package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/i18n"
)

// API is the subset of *gotgbot.Bot this package uses.
type API interface {
	GetMe(opts *gotgbot.GetMeOpts) (*gotgbot.User, error)
	GetUpdates(opts *gotgbot.GetUpdatesOpts) ([]gotgbot.Update, error)
	SendMessage(chatID int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error)
	SetWebhook(url string, opts *gotgbot.SetWebhookOpts) (bool, error)
	DeleteWebhook(opts *gotgbot.DeleteWebhookOpts) (bool, error)
	SetMyCommands(commands []gotgbot.BotCommand, opts *gotgbot.SetMyCommandsOpts) (bool, error)
	DeleteMyCommands(opts *gotgbot.DeleteMyCommandsOpts) (bool, error)
	LeaveChat(chatId int64, opts *gotgbot.LeaveChatOpts) (bool, error)
	GetChatMember(chatId int64, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error)
}

// Activator links a Telegram account to the user with the verified phone and
// hands a linked, active user a one-time login link to the cabinet.
type Activator interface {
	ActivateChannel(ctx context.Context, typ, externalID, chatID, username, verifiedPhone string) (domain.Activation, error)
	// CabinetLink gives domain.ErrNotFound when the account is not linked to an active user.
	CabinetLink(ctx context.Context, typ, externalID string) (domain.User, string, error)
}

// GroupMessage is a messenger-neutral copy of a message seen in a group.
type GroupMessage struct {
	ChatID, ChatTitle, MessageID, ThreadID               string // ThreadID: forum topic, "" outside topics
	FromExternalID, FromUsername, FromName, LanguageCode string
	Text                                                 string
	At                                                   time.Time
}

// GroupSink receives group messages.
type GroupSink interface {
	OnGroupMessage(ctx context.Context, m GroupMessage)
}

// NopSink drops group messages; it is what M1 wires.
type NopSink struct{}

// OnGroupMessage does nothing.
func (NopSink) OnGroupMessage(context.Context, GroupMessage) {}

// Type is the channel type stored for Telegram accounts.
const Type = "telegram"

// Bot is the Telegram adapter.
type Bot struct {
	api       API
	act       Activator
	chats     domain.ChatEvents
	sink      GroupSink
	tr        *i18n.Bundle
	locales   []string
	publicURL string
	username  string
	botID     int64
	log       *slog.Logger
}

var _ domain.Messenger = (*Bot)(nil)

// New asks Telegram for the bot's username and returns the adapter.
func New(api API, act Activator, chats domain.ChatEvents, sink GroupSink, tr *i18n.Bundle, publicURL string, log *slog.Logger) (*Bot, error) {
	return NewWithLocales(api, act, chats, sink, tr, []string{"uk", "en"}, publicURL, log)
}

// NewWithLocales is New with the list of locales the bot's command menu is set for.
func NewWithLocales(api API, act Activator, chats domain.ChatEvents, sink GroupSink, tr *i18n.Bundle, locales []string, publicURL string, log *slog.Logger) (*Bot, error) {
	me, err := api.GetMe(nil)
	if err != nil {
		return nil, fmt.Errorf("telegram getMe: %w", err)
	}
	if sink == nil {
		sink = NopSink{}
	}
	return &Bot{api: api, act: act, chats: chats, sink: sink, tr: tr, locales: locales, publicURL: strings.TrimRight(publicURL, "/"),
		username: me.Username, botID: me.Id, log: log}, nil
}

// Type returns "telegram".
func (b *Bot) Type() string { return Type }

// BotURL is the public link to the bot.
func (b *Bot) BotURL() string { return "https://t.me/" + b.username }

// SendText sends a plain text message.
func (b *Bot) SendText(_ context.Context, chatID, text string) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram chat id %q: %w", chatID, err)
	}
	_, err = b.api.SendMessage(id, text, nil)
	return err
}

func (b *Bot) reply(chatID int64, text string, markup gotgbot.ReplyMarkup) {
	if _, err := b.api.SendMessage(chatID, text, &gotgbot.SendMessageOpts{ReplyMarkup: markup}); err != nil {
		b.log.Warn("telegram send failed", "err", err)
	}
}

// HandleUpdate processes one update.
func (b *Bot) HandleUpdate(ctx context.Context, u gotgbot.Update) {
	if u.MyChatMember != nil {
		b.handleMyChatMember(ctx, u.MyChatMember)
		return
	}
	m := u.Message
	if m == nil {
		return
	}
	switch m.Chat.Type {
	case "private":
		if m.From != nil {
			b.handlePrivate(ctx, m)
		}
	case "group", "supergroup":
		b.handleGroup(ctx, m)
	}
}

func groupMessage(m *gotgbot.Message) GroupMessage {
	g := GroupMessage{
		ChatID: strconv.FormatInt(m.Chat.Id, 10), ChatTitle: m.Chat.Title, MessageID: strconv.FormatInt(m.MessageId, 10),
		FromExternalID: strconv.FormatInt(m.From.Id, 10), FromUsername: m.From.Username,
		FromName: strings.TrimSpace(m.From.FirstName + " " + m.From.LastName), LanguageCode: m.From.LanguageCode,
		Text: m.Text, At: time.Unix(m.Date, 0).UTC(),
	}
	if m.IsTopicMessage && m.MessageThreadId != 0 {
		g.ThreadID = strconv.FormatInt(m.MessageThreadId, 10)
	}
	return g
}

func (b *Bot) handlePrivate(ctx context.Context, m *gotgbot.Message) {
	lang := b.tr.Pick(m.From.LanguageCode)
	switch {
	case m.Contact != nil:
		b.handleContact(ctx, m, lang)
	case m.Text == "/start" || strings.HasPrefix(m.Text, "/start "):
		b.askContact(m.Chat.Id, lang)
	case b.isCommand(m.Text, "cabinet"):
		b.handleCabinet(ctx, m, lang)
	default:
		b.reply(m.Chat.Id, b.tr.T(lang, "bot.help", nil, nil), nil)
	}
}

// isCommand reports whether text is /name, /name@<this bot> or either
// followed by arguments.
func (b *Bot) isCommand(text, name string) bool {
	word, _, _ := strings.Cut(text, " ")
	return word == "/"+name || strings.EqualFold(word, "/"+name+"@"+b.username)
}

// askContact asks for the phone number with the contact button (the /start answer).
func (b *Bot) askContact(chatID int64, lang string) {
	b.reply(chatID, b.tr.T(lang, "bot.start.prompt", nil, nil), gotgbot.ReplyKeyboardMarkup{
		Keyboard:        [][]gotgbot.KeyboardButton{{{Text: b.tr.T(lang, "bot.start.button", nil, nil), RequestContact: true}}},
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	})
}

// handleCabinet answers /cabinet with a one-time login link for a linked,
// active user; anyone else is asked to connect their account as on /start.
func (b *Bot) handleCabinet(ctx context.Context, m *gotgbot.Message, lang string) {
	user, url, err := b.act.CabinetLink(ctx, Type, strconv.FormatInt(m.From.Id, 10))
	switch {
	case errors.Is(err, domain.ErrNotFound):
		b.askContact(m.Chat.Id, lang)
		return
	case errors.Is(err, domain.ErrRateLimited):
		b.reply(m.Chat.Id, b.tr.T(lang, "bot.rateLimited", nil, nil), nil)
		return
	case err != nil:
		b.log.Error("telegram cabinet link failed", "err", err)
		b.reply(m.Chat.Id, b.tr.T(lang, "common.error.internal", nil, nil), nil)
		return
	}
	if user.Locale != "" {
		lang = b.tr.Pick(user.Locale)
	}
	b.offerLink(m.Chat.Id, b.tr.T(lang, "bot.cabinet.offer", nil, nil), b.tr.T(lang, "bot.cabinet.button", nil, nil), url, nil)
}

func (b *Bot) handleContact(ctx context.Context, m *gotgbot.Message, lang string) {
	remove := gotgbot.ReplyKeyboardRemove{RemoveKeyboard: true}
	if m.Contact.UserId == 0 || m.Contact.UserId != m.From.Id {
		b.reply(m.Chat.Id, b.tr.T(lang, "bot.contact.notOwn", nil, nil), remove)
		return
	}
	act, err := b.act.ActivateChannel(ctx, Type, strconv.FormatInt(m.From.Id, 10), strconv.FormatInt(m.Chat.Id, 10),
		m.From.Username, m.Contact.PhoneNumber)
	res, user := act.Result, act.User
	if err == nil && res != domain.NotRegistered && user.Locale != "" {
		lang = b.tr.Pick(user.Locale)
	}
	if err == nil && res == domain.ApplyLink {
		b.offerApplication(m.Chat.Id, lang, act.RegistrationURL)
		return
	}
	var key string
	var data map[string]any
	switch {
	case errors.Is(err, domain.ErrRateLimited):
		key = "bot.rateLimited"
	case err != nil:
		b.log.Error("telegram activation failed", "err", err)
		key = "common.error.internal"
	case res == domain.Activated:
		key, data = "bot.activated", map[string]any{"Name": user.FirstName, "URL": b.publicURL}
	case res == domain.BootstrapAdmin:
		key, data = "bot.bootstrap", map[string]any{"URL": b.publicURL + "/admin"}
	case res == domain.AlreadyConnected:
		key = "bot.alreadyConnected"
	case res == domain.PendingApproval:
		key = "bot.pending"
	case res == domain.PendingLinked:
		key = "bot.pendingLinked"
	default:
		key = "bot.notRegistered"
	}
	b.reply(m.Chat.Id, b.tr.T(lang, key, data, nil), remove)
}

// offerApplication sends the registration link as a button, or in the text
// when it is not https (Telegram refuses such buttons). The contact keyboard
// was one-time, so it is already hidden.
func (b *Bot) offerApplication(chatID int64, lang, url string) {
	b.offerLink(chatID, b.tr.T(lang, "bot.apply.offer", nil, nil), b.tr.T(lang, "bot.apply.button", nil, nil), url,
		gotgbot.ReplyKeyboardRemove{RemoveKeyboard: true})
}

// offerLink sends text with url as an inline button, or url in the text when
// it is not https (Telegram refuses such buttons); plain is the markup of the
// text-only message.
func (b *Bot) offerLink(chatID int64, text, button, url string, plain gotgbot.ReplyMarkup) {
	if !strings.HasPrefix(url, "https://") {
		b.reply(chatID, text+"\n"+url, plain)
		return
	}
	b.reply(chatID, text, gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{Text: button, Url: url}}}})
}

// legacyMenuLocales are languages an earlier program may have set a default
// menu for; their default-scope menus are removed.
var legacyMenuLocales = []string{"", "uk", "en", "ru"}

// SetCommands sets the bot's command menu for private chats in every locale and
// removes menus other software left in the default scope (shown in groups, where
// the bot does not take commands in M1).
func (b *Bot) SetCommands(_ context.Context) error {
	for _, lang := range legacyMenuLocales {
		if _, err := b.api.DeleteMyCommands(&gotgbot.DeleteMyCommandsOpts{Scope: gotgbot.BotCommandScopeDefault{}, LanguageCode: lang}); err != nil {
			return fmt.Errorf("telegram deleteMyCommands %q: %w", lang, err)
		}
	}
	menu := func(lang string) []gotgbot.BotCommand {
		return []gotgbot.BotCommand{
			{Command: "start", Description: b.tr.T(lang, "bot.commands.start", nil, nil)},
			{Command: "cabinet", Description: b.tr.T(lang, "bot.commands.cabinet", nil, nil)},
		}
	}
	if _, err := b.api.SetMyCommands(menu(b.tr.Default()), &gotgbot.SetMyCommandsOpts{Scope: gotgbot.BotCommandScopeAllPrivateChats{}}); err != nil {
		return fmt.Errorf("telegram setMyCommands: %w", err)
	}
	for _, lang := range b.locales {
		if _, err := b.api.SetMyCommands(menu(lang), &gotgbot.SetMyCommandsOpts{Scope: gotgbot.BotCommandScopeAllPrivateChats{}, LanguageCode: lang}); err != nil {
			return fmt.Errorf("telegram setMyCommands %q: %w", lang, err)
		}
	}
	return nil
}
