package telegram

import (
	"context"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func chatInfo(c gotgbot.Chat) domain.ChatInfo {
	return domain.ChatInfo{Messenger: Type, ExternalID: strconv.FormatInt(c.Id, 10), Kind: c.Type, Title: c.Title, IsForum: c.IsForum}
}

// present reports whether a membership means the bot is in the chat.
func present(m gotgbot.MergedChatMember) bool {
	switch m.Status {
	case "creator", "administrator", "member":
		return true
	case "restricted":
		return m.IsMember
	}
	return false
}

func rights(m gotgbot.MergedChatMember, chatType string) domain.BotRights {
	switch m.Status {
	case "administrator", "creator":
		canSend := true
		if chatType == "channel" {
			canSend = m.CanPostMessages
		}
		return domain.BotRights{Status: domain.BotAdministrator, CanSend: canSend}
	case "member":
		return domain.BotRights{Status: domain.BotMember, CanSend: true}
	case "restricted":
		return domain.BotRights{Status: domain.BotRestricted, CanSend: m.CanSendMessages}
	}
	return domain.BotRights{Status: domain.BotUnknown}
}

// handleMyChatMember reports the bot being added to, changed in, or removed from a chat.
func (b *Bot) handleMyChatMember(ctx context.Context, u *gotgbot.ChatMemberUpdated) {
	if u.Chat.Type == "private" {
		return
	}
	info := chatInfo(u.Chat)
	oldM, newM := u.OldChatMember.MergeChatMember(), u.NewChatMember.MergeChatMember()
	var err error
	switch was, is := present(oldM), present(newM); {
	case !was && is:
		err = b.chats.BotAdded(ctx, info, strconv.FormatInt(u.From.Id, 10), rights(newM, u.Chat.Type))
	case was && is:
		err = b.chats.BotRightsChanged(ctx, info, rights(newM, u.Chat.Type))
	case was && !is:
		err = b.chats.BotRemoved(ctx, info)
	}
	if err != nil {
		b.log.Error("telegram chat membership not recorded", "chat", info.ExternalID, "err", err)
	}
}

// topicName is the topic's name when this message carries it.
func topicName(m *gotgbot.Message) string {
	switch {
	case m.ForumTopicCreated != nil:
		return m.ForumTopicCreated.Name
	case m.ForumTopicEdited != nil:
		return m.ForumTopicEdited.Name
	case m.ReplyToMessage != nil && m.ReplyToMessage.ForumTopicCreated != nil:
		return m.ReplyToMessage.ForumTopicCreated.Name
	}
	return ""
}

// handleGroup records the chat (and topic) a group message came from and
// passes messages from accepted chats to the sink.
func (b *Bot) handleGroup(ctx context.Context, m *gotgbot.Message) {
	info := chatInfo(m.Chat)
	switch {
	case m.MigrateToChatId != 0:
		b.moved(ctx, info.ExternalID, strconv.FormatInt(m.MigrateToChatId, 10))
		return
	case m.MigrateFromChatId != 0:
		b.moved(ctx, strconv.FormatInt(m.MigrateFromChatId, 10), info.ExternalID)
		return
	}
	created, err := b.chats.ChatSeen(ctx, info)
	if err != nil {
		b.log.Error("telegram chat not recorded", "chat", info.ExternalID, "err", err)
		return
	}
	if created {
		b.fetchRights(ctx, m.Chat)
	}
	if m.MessageThreadId != 0 && (m.IsTopicMessage || m.ForumTopicCreated != nil) {
		if err := b.chats.TopicSeen(ctx, info, strconv.FormatInt(m.MessageThreadId, 10), topicName(m)); err != nil {
			b.log.Error("telegram topic not recorded", "chat", info.ExternalID, "err", err)
		}
	}
	if m.From == nil {
		return
	}
	if ok, err := b.chats.IsAccepted(ctx, Type, info.ExternalID); err != nil {
		b.log.Error("telegram chat state unknown", "chat", info.ExternalID, "err", err)
	} else if ok {
		b.sink.OnGroupMessage(ctx, groupMessage(m))
	}
}

func (b *Bot) moved(ctx context.Context, oldID, newID string) {
	if err := b.chats.ChatMoved(ctx, Type, oldID, newID); err != nil {
		b.log.Error("telegram chat migration not recorded", "from", oldID, "to", newID, "err", err)
	}
}

// fetchRights asks Telegram for the bot's own membership in a chat first seen
// from a message; a failure leaves the rights unknown.
func (b *Bot) fetchRights(ctx context.Context, c gotgbot.Chat) {
	member, err := b.api.GetChatMember(c.Id, b.botID, nil)
	if err != nil {
		b.log.Warn("telegram getChatMember failed", "chat", c.Id, "err", err)
		return
	}
	if err := b.chats.BotRightsChanged(ctx, chatInfo(c), rights(member.MergeChatMember(), c.Type)); err != nil {
		b.log.Error("telegram chat rights not recorded", "chat", c.Id, "err", err)
	}
}
