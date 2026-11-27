package domain

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// AnonymousAdminExternalID is Telegram's GroupAnonymousBot: an add reported
// from it cannot be attributed to a person.
const AnonymousAdminExternalID = "1087968824"

const maxTopicTitle = 128

// ChatEvents is what a messenger adapter reports; ChatService implements it.
type ChatEvents interface {
	BotAdded(ctx context.Context, c ChatInfo, addedByExternalID string, r BotRights) error
	BotRightsChanged(ctx context.Context, c ChatInfo, r BotRights) error
	BotRemoved(ctx context.Context, c ChatInfo) error
	ChatSeen(ctx context.Context, c ChatInfo) (created bool, err error)
	// TopicSeen records a forum topic. An authoritative name is Telegram's
	// current one (topic created or renamed); otherwise name is a hint that
	// only fills a topic without a name.
	TopicSeen(ctx context.Context, c ChatInfo, threadID, name string, authoritative bool) error
	ChatMoved(ctx context.Context, messenger, oldID, newID string) error
	IsAccepted(ctx context.Context, messenger, externalID string) (bool, error)
}

var _ ChatEvents = (*ChatService)(nil)

// ChatService keeps the list of chats the bot is in and decides which of them
// may receive messages.
type ChatService struct {
	R          Repos
	Now        Clock
	Messengers Messengers
	Log        *slog.Logger
	LeaveGrace time.Duration
}

// ChatView is a chat with its topics and labels.
type ChatView struct {
	Chat
	LabelIDs []int64
	Topics   []TopicView
}

// TopicView is a topic with its labels.
type TopicView struct {
	Topic
	LabelIDs []int64
}

// ChatTarget is the audit target of a chat.
func ChatTarget(id int64) string { return "chat:" + strconv.FormatInt(id, 10) }

// TopicTarget is the audit target of a forum topic.
func TopicTarget(id int64) string { return "topic:" + strconv.FormatInt(id, 10) }

func (s *ChatService) audit(ctx context.Context, a Actor, action, target string, details map[string]any) error {
	return s.R.Audit.Add(ctx, AuditEntry{At: s.Now(), ActorUserID: a.UserID, Action: action, Target: target, Details: details, IP: a.IP})
}

// activeAdmin returns the id of the active admin whose messenger account this is.
func (s *ChatService) activeAdmin(ctx context.Context, messenger, externalID string) (*int64, error) {
	ch, err := s.R.Channels.GetByExternal(ctx, messenger, externalID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	u, err := s.R.Users.Get(ctx, ch.UserID)
	if err != nil {
		return nil, err
	}
	if u.Status != StatusActive || !u.IsAdmin {
		return nil, nil
	}
	return &u.ID, nil
}

func applyInfo(ch *Chat, c ChatInfo) {
	if c.Title != "" {
		ch.Title = c.Title
	}
	if c.Kind != "" {
		ch.Kind = c.Kind
	}
	ch.IsForum = c.IsForum
}

// getOrCreate returns the chat, creating it as new (audited chat.added) when unknown.
func (s *ChatService) getOrCreate(ctx context.Context, c ChatInfo, details map[string]any) (Chat, bool, error) {
	ch, err := s.R.Chats.GetByExternal(ctx, c.Messenger, c.ExternalID)
	if err == nil || !errors.Is(err, ErrNotFound) {
		return ch, false, err
	}
	now := s.Now()
	ch = Chat{Messenger: c.Messenger, ExternalID: c.ExternalID, State: ChatNew, BotStatus: BotUnknown, FirstSeenAt: now, LastSeenAt: now}
	applyInfo(&ch, c)
	if err := s.R.Chats.Create(ctx, &ch); err != nil {
		return Chat{}, false, err
	}
	return ch, true, s.audit(ctx, Actor{}, ActChatAdded, ChatTarget(ch.ID), details)
}

// BotAdded records that the bot was added to a chat. Adds by an active admin
// are accepted; adds by anyone else wait for an admin or are left after the grace.
func (s *ChatService) BotAdded(ctx context.Context, c ChatInfo, addedBy string, r BotRights) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		adminID, err := s.activeAdmin(ctx, c.Messenger, addedBy)
		if err != nil {
			return err
		}
		ch, err := s.R.Chats.GetByExternal(ctx, c.Messenger, c.ExternalID)
		existed := err == nil
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		now := s.Now()
		if !existed {
			ch = Chat{Messenger: c.Messenger, ExternalID: c.ExternalID, FirstSeenAt: now}
		}
		applyInfo(&ch, c)
		ch.BotStatus, ch.CanSend, ch.LastSeenAt, ch.LeftAt = r.Status, r.CanSend, now, nil
		ch.AddedByExternalID, ch.AddedByUserID = addedBy, adminID
		accepted := false
		switch {
		case adminID != nil:
			ch.State, ch.AcceptedBy, ch.AcceptedAt, ch.LeaveAt = ChatAccepted, adminID, &now, nil
			accepted = true
		case existed && ch.State == ChatAccepted:
			// Still the chat an admin accepted (e.g. a removal we never heard of).
		case addedBy == AnonymousAdminExternalID:
			ch.State, ch.LeaveAt = ChatNew, nil
		default:
			leave := now.Add(s.LeaveGrace)
			ch.State, ch.LeaveAt = ChatNew, &leave
		}
		if existed {
			err = s.R.Chats.Update(ctx, &ch)
		} else {
			err = s.R.Chats.Create(ctx, &ch)
		}
		if err != nil {
			return err
		}
		a := Actor{UserID: adminID}
		if err := s.audit(ctx, a, ActChatAdded, ChatTarget(ch.ID), map[string]any{"addedBy": addedBy}); err != nil {
			return err
		}
		if accepted {
			return s.audit(ctx, a, ActChatAccepted, ChatTarget(ch.ID), nil)
		}
		return nil
	})
}

// BotRightsChanged records the bot's new rights in a chat it is in.
func (s *ChatService) BotRightsChanged(ctx context.Context, c ChatInfo, r BotRights) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		ch, _, err := s.getOrCreate(ctx, c, map[string]any{"via": "rights"})
		if err != nil {
			return err
		}
		applyInfo(&ch, c)
		ch.LastSeenAt = s.Now()
		if ch.State != ChatLeft { // a late update must not revive a chat the bot left
			ch.BotStatus, ch.CanSend = r.Status, r.CanSend
		}
		return s.R.Chats.Update(ctx, &ch)
	})
}

// BotRemoved records that the bot is no longer in a chat.
func (s *ChatService) BotRemoved(ctx context.Context, c ChatInfo) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		ch, err := s.R.Chats.GetByExternal(ctx, c.Messenger, c.ExternalID)
		if errors.Is(err, ErrNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		applyInfo(&ch, c)
		if err := s.setLeft(ctx, &ch); err != nil {
			return err
		}
		return s.audit(ctx, Actor{}, ActChatRemoved, ChatTarget(ch.ID), nil)
	})
}

func (s *ChatService) setLeft(ctx context.Context, ch *Chat) error {
	now := s.Now()
	ch.State, ch.LeftAt, ch.LeaveAt, ch.BotStatus, ch.CanSend = ChatLeft, &now, nil, BotUnknown, false
	return s.R.Chats.Update(ctx, ch)
}

// ChatSeen records a chat a message came from; created reports a new row.
func (s *ChatService) ChatSeen(ctx context.Context, c ChatInfo) (bool, error) {
	var created bool
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		created, err = s.seen(ctx, c)
		return err
	})
	return created, err
}

func (s *ChatService) seen(ctx context.Context, c ChatInfo) (bool, error) {
	ch, created, err := s.getOrCreate(ctx, c, map[string]any{"via": "message"})
	if err != nil || created {
		return created, err
	}
	// A left chat stays left: only a real add (BotAdded) brings it back, so a
	// late or redelivered message cannot put it into the "new" list.
	applyInfo(&ch, c)
	ch.LastSeenAt = s.Now()
	return false, s.R.Chats.Update(ctx, &ch)
}

// TopicSeen records a forum topic. An authoritative name (topic created or
// renamed) replaces the title, an admin's included; a hint (the creation
// message Telegram attaches to every plain topic message) only names a topic
// that has no name yet.
func (s *ChatService) TopicSeen(ctx context.Context, c ChatInfo, threadID, name string, authoritative bool) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if _, err := s.seen(ctx, c); err != nil {
			return err
		}
		ch, err := s.R.Chats.GetByExternal(ctx, c.Messenger, c.ExternalID)
		if err != nil {
			return err
		}
		now := s.Now()
		t, err := s.R.Topics.GetByExternal(ctx, ch.ID, threadID)
		if errors.Is(err, ErrNotFound) {
			t = Topic{ChatID: ch.ID, ExternalID: threadID, Title: name, FirstSeenAt: now, LastSeenAt: now}
			return s.R.Topics.Create(ctx, &t)
		} else if err != nil {
			return err
		}
		switch {
		case name != "" && authoritative:
			t.Title, t.TitleSetByAdmin = name, false
		case name != "" && t.Title == "":
			t.Title = name
		}
		t.Gone, t.LastSeenAt = false, now
		return s.R.Topics.Update(ctx, &t)
	})
}

// ChatMoved follows a chat to its new id (a group upgraded to a supergroup).
// Labels, topics, acceptance and queued messages follow; repeating is a no-op.
func (s *ChatService) ChatMoved(ctx context.Context, messenger, oldID, newID string) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		old, err := s.R.Chats.GetByExternal(ctx, messenger, oldID)
		if errors.Is(err, ErrNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		if err := s.R.Outbox.RepointChat(ctx, messenger, oldID, newID); err != nil {
			return err
		}
		details := map[string]any{"from": oldID, "to": newID}
		nw, err := s.R.Chats.GetByExternal(ctx, messenger, newID)
		if errors.Is(err, ErrNotFound) {
			old.ExternalID, old.Kind = newID, ChatKindSupergroup
			if s.leftForUpgrade(old) {
				old.State, old.LeftAt = ChatAccepted, nil
			}
			if err := s.R.Chats.Update(ctx, &old); err != nil {
				return err
			}
			return s.audit(ctx, Actor{}, ActChatMoved, ChatTarget(old.ID), details)
		} else if err != nil {
			return err
		}
		if err := s.merge(ctx, old, &nw); err != nil {
			return err
		}
		return s.audit(ctx, Actor{}, ActChatMoved, ChatTarget(nw.ID), details)
	})
}

// upgradeWindow is how long before a migration a "bot left" of an accepted
// chat is taken as part of the upgrade itself.
const upgradeWindow = 5 * time.Minute

// leftForUpgrade reports whether an accepted chat was reported left just
// before it moved: Telegram may announce the old group gone during an upgrade.
func (s *ChatService) leftForUpgrade(c Chat) bool {
	return c.State == ChatLeft && c.AcceptedAt != nil && c.LeftAt != nil && s.Now().Sub(*c.LeftAt) <= upgradeWindow
}

// merge folds old into nw and deletes old.
func (s *ChatService) merge(ctx context.Context, old Chat, nw *Chat) error {
	if err := s.R.Topics.MoveToChat(ctx, old.ID, nw.ID); err != nil {
		return err
	}
	leftovers, err := s.R.Topics.ListByChat(ctx, old.ID)
	if err != nil {
		return err
	}
	for _, t := range leftovers { // same thread id already known in the new chat
		dst, err := s.R.Topics.GetByExternal(ctx, nw.ID, t.ExternalID)
		if err != nil {
			return err
		}
		if err := s.R.Labels.Retarget(ctx, LabelTarget{TargetTopic, t.ID}, LabelTarget{TargetTopic, dst.ID}); err != nil {
			return err
		}
	}
	if err := s.R.Labels.Retarget(ctx, LabelTarget{TargetChat, old.ID}, LabelTarget{TargetChat, nw.ID}); err != nil {
		return err
	}
	if (old.State == ChatAccepted || s.leftForUpgrade(old)) && nw.State != ChatAccepted {
		nw.State, nw.AcceptedBy, nw.AcceptedAt = ChatAccepted, old.AcceptedBy, old.AcceptedAt
	}
	if nw.Title == "" {
		nw.Title = old.Title
	}
	if old.FirstSeenAt.Before(nw.FirstSeenAt) {
		nw.FirstSeenAt = old.FirstSeenAt
	}
	nw.LeaveAt = nil
	if err := s.R.Chats.Delete(ctx, old.ID); err != nil {
		return err
	}
	return s.R.Chats.Update(ctx, nw)
}

// IsAccepted reports whether a chat may receive messages.
func (s *ChatService) IsAccepted(ctx context.Context, messenger, externalID string) (bool, error) {
	ch, err := s.R.Chats.GetByExternal(ctx, messenger, externalID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil && ch.State == ChatAccepted, err
}

// Accept lets a new chat receive messages.
func (s *ChatService) Accept(ctx context.Context, id int64, a Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		ch, err := s.R.Chats.Get(ctx, id)
		if err != nil {
			return err
		}
		if ch.State != ChatNew {
			return ErrConflict
		}
		now := s.Now()
		ch.State, ch.AcceptedBy, ch.AcceptedAt, ch.LeaveAt = ChatAccepted, a.UserID, &now, nil
		if err := s.R.Chats.Update(ctx, &ch); err != nil {
			return err
		}
		return s.audit(ctx, a, ActChatAccepted, ChatTarget(id), nil)
	})
}

// Leave makes the bot leave a chat. The state changes only after the messenger confirms.
func (s *ChatService) Leave(ctx context.Context, id int64, a Actor) error {
	ch, err := s.R.Chats.Get(ctx, id)
	if err != nil {
		return err
	}
	if ch.State == ChatLeft {
		return ErrConflict
	}
	if err := s.leaveChat(ctx, ch); err != nil {
		return err
	}
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		ch, err := s.R.Chats.Get(ctx, id)
		if err != nil {
			return err
		}
		if err := s.setLeft(ctx, &ch); err != nil {
			return err
		}
		return s.audit(ctx, a, ActChatLeft, ChatTarget(id), nil)
	})
}

func (s *ChatService) leaveChat(ctx context.Context, ch Chat) error {
	m, err := s.Messengers.Get(ch.Messenger)
	if err != nil {
		return err
	}
	return m.LeaveChat(ctx, ch.ExternalID)
}

// LeaveDue leaves new chats whose grace has run out; a failed leave is retried next time.
func (s *ChatService) LeaveDue(ctx context.Context) (int, error) {
	due, err := s.R.Chats.DueLeaves(ctx, s.Now())
	if err != nil {
		return 0, err
	}
	n := 0
	for _, ch := range due {
		if err := s.leaveChat(ctx, ch); err != nil {
			s.Log.Warn("leaving a chat failed; will retry", "chat", ch.ID, "err", err)
			continue
		}
		err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
			if err := s.setLeft(ctx, &ch); err != nil {
				return err
			}
			return s.audit(ctx, Actor{}, ActChatAutoLeft, ChatTarget(ch.ID), map[string]any{"addedBy": ch.AddedByExternalID})
		})
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// RenameTopic sets an admin's name for a topic (until Telegram reports a rename).
func (s *ChatService) RenameTopic(ctx context.Context, id int64, title string, a Actor) error {
	title = strings.TrimSpace(title)
	v := validation{}
	switch {
	case title == "":
		v.add("title", "required")
	case utf8.RuneCountInString(title) > maxTopicTitle:
		v.add("title", "too_long")
	}
	if err := v.err(); err != nil {
		return err
	}
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		t, err := s.R.Topics.Get(ctx, id)
		if err != nil {
			return err
		}
		t.Title, t.TitleSetByAdmin = title, true
		if err := s.R.Topics.Update(ctx, &t); err != nil {
			return err
		}
		return s.audit(ctx, a, ActTopicRenamed, TopicTarget(id), map[string]any{"title": title})
	})
}

// MarkGone records that a send found the bot gone from a chat.
func (s *ChatService) MarkGone(ctx context.Context, messenger, chatID string) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		ch, err := s.R.Chats.GetByExternal(ctx, messenger, chatID)
		if errors.Is(err, ErrNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		if ch.State == ChatLeft {
			return nil
		}
		if err := s.setLeft(ctx, &ch); err != nil {
			return err
		}
		return s.audit(ctx, Actor{}, ActChatRemoved, ChatTarget(ch.ID), map[string]any{"reason": "send_failed"})
	})
}

// MarkTopicGone records that a send found a topic deleted.
func (s *ChatService) MarkTopicGone(ctx context.Context, messenger, chatID, threadID string) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		ch, err := s.R.Chats.GetByExternal(ctx, messenger, chatID)
		if errors.Is(err, ErrNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		t, err := s.R.Topics.GetByExternal(ctx, ch.ID, threadID)
		if errors.Is(err, ErrNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		t.Gone = true
		return s.R.Topics.Update(ctx, &t)
	})
}

// List returns every chat with its topics and labels.
func (s *ChatService) List(ctx context.Context) ([]ChatView, error) {
	chats, err := s.R.Chats.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ChatView, 0, len(chats))
	for _, ch := range chats {
		v := ChatView{Chat: ch, Topics: []TopicView{}}
		if v.LabelIDs, err = s.R.Labels.LabelsOf(ctx, LabelTarget{TargetChat, ch.ID}); err != nil {
			return nil, err
		}
		topics, err := s.R.Topics.ListByChat(ctx, ch.ID)
		if err != nil {
			return nil, err
		}
		for _, t := range topics {
			tv := TopicView{Topic: t}
			if tv.LabelIDs, err = s.R.Labels.LabelsOf(ctx, LabelTarget{TargetTopic, t.ID}); err != nil {
				return nil, err
			}
			v.Topics = append(v.Topics, tv)
		}
		out = append(out, v)
	}
	return out, nil
}

// NewCount is how many chats wait for an admin's decision.
func (s *ChatService) NewCount(ctx context.Context) (int, error) {
	return s.R.Chats.CountByState(ctx, ChatNew)
}
