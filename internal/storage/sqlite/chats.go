package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Chats implements domain.ChatRepo.
type Chats struct{ DB DB }

// Topics implements domain.TopicRepo.
type Topics struct{ DB DB }

var (
	_ domain.ChatRepo  = Chats{}
	_ domain.TopicRepo = Topics{}
)

const chatColumns = `id, messenger, external_id, kind, is_forum, title, state, bot_status, can_send,
	added_by_external_id, added_by_user_id, accepted_by, accepted_at, leave_at, left_at, first_seen_at, last_seen_at`

func scanChat(s scanner) (domain.Chat, error) {
	var (
		c                         domain.Chat
		isForum, canSend          int
		addedBy, acceptedBy       sql.NullInt64
		acceptedAt, leaveAt, left sql.NullString
		first, last               string
	)
	if err := s.Scan(&c.ID, &c.Messenger, &c.ExternalID, &c.Kind, &isForum, &c.Title, &c.State, &c.BotStatus, &canSend,
		&c.AddedByExternalID, &addedBy, &acceptedBy, &acceptedAt, &leaveAt, &left, &first, &last); err != nil {
		return domain.Chat{}, mapErr(err)
	}
	c.IsForum, c.CanSend = isForum == 1, canSend == 1
	c.AddedByUserID, c.AcceptedBy = intPtr(addedBy), intPtr(acceptedBy)
	c.AcceptedAt, c.LeaveAt, c.LeftAt = parseTimePtr(acceptedAt), parseTimePtr(leaveAt), parseTimePtr(left)
	c.FirstSeenAt, c.LastSeenAt = parseTime(first), parseTime(last)
	return c, nil
}

func chatArgs(c *domain.Chat) []any {
	return []any{c.Messenger, c.ExternalID, c.Kind, boolInt(c.IsForum), c.Title, string(c.State), c.BotStatus, boolInt(c.CanSend),
		c.AddedByExternalID, nullInt(c.AddedByUserID), nullInt(c.AcceptedBy), fmtTimePtr(c.AcceptedAt), fmtTimePtr(c.LeaveAt),
		fmtTimePtr(c.LeftAt), fmtTime(c.FirstSeenAt), fmtTime(c.LastSeenAt)}
}

// Create inserts c and sets its ID.
func (r Chats) Create(ctx context.Context, c *domain.Chat) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO chats (messenger, external_id, kind, is_forum, title, state,
		bot_status, can_send, added_by_external_id, added_by_user_id, accepted_by, accepted_at, leave_at, left_at,
		first_seen_at, last_seen_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, chatArgs(c)...)
	if err != nil {
		return mapErr(err)
	}
	c.ID, err = res.LastInsertId()
	return err
}

// Update writes every field of c.
func (r Chats) Update(ctx context.Context, c *domain.Chat) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE chats SET messenger = ?, external_id = ?, kind = ?, is_forum = ?,
		title = ?, state = ?, bot_status = ?, can_send = ?, added_by_external_id = ?, added_by_user_id = ?, accepted_by = ?,
		accepted_at = ?, leave_at = ?, left_at = ?, first_seen_at = ?, last_seen_at = ? WHERE id = ?`,
		append(chatArgs(c), c.ID)...)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Get returns one chat.
func (r Chats) Get(ctx context.Context, id int64) (domain.Chat, error) {
	return scanChat(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+chatColumns+` FROM chats WHERE id = ?`, id))
}

// GetByExternal finds a chat by its messenger id.
func (r Chats) GetByExternal(ctx context.Context, messenger, externalID string) (domain.Chat, error) {
	return scanChat(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+chatColumns+` FROM chats WHERE messenger = ? AND external_id = ?`,
		messenger, externalID))
}

func (r Chats) list(ctx context.Context, where string, args ...any) ([]domain.Chat, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT `+chatColumns+` FROM chats `+where, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Chat
	for rows.Next() {
		c, err := scanChat(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// List returns new chats first, then accepted, then left; by title within each.
func (r Chats) List(ctx context.Context) ([]domain.Chat, error) {
	return r.list(ctx, `ORDER BY CASE state WHEN 'new' THEN 0 WHEN 'accepted' THEN 1 ELSE 2 END, title, id`)
}

// Delete removes a chat and its topics.
func (r Chats) Delete(ctx context.Context, id int64) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM chats WHERE id = ?`, id)
	return err
}

// DueLeaves returns new chats whose leave_at has passed.
func (r Chats) DueLeaves(ctx context.Context, now time.Time) ([]domain.Chat, error) {
	return r.list(ctx, `WHERE state = 'new' AND leave_at IS NOT NULL AND leave_at <= ? ORDER BY id`, fmtTime(now))
}

// CountByState counts chats in a state.
func (r Chats) CountByState(ctx context.Context, s domain.ChatState) (int, error) {
	var n int
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM chats WHERE state = ?`, string(s)).Scan(&n)
	return n, err
}

const topicColumns = `id, chat_id, external_id, title, title_set_by_admin, gone, first_seen_at, last_seen_at`

func scanTopic(s scanner) (domain.Topic, error) {
	var (
		t           domain.Topic
		byAdmin, g  int
		first, last string
	)
	if err := s.Scan(&t.ID, &t.ChatID, &t.ExternalID, &t.Title, &byAdmin, &g, &first, &last); err != nil {
		return domain.Topic{}, mapErr(err)
	}
	t.TitleSetByAdmin, t.Gone = byAdmin == 1, g == 1
	t.FirstSeenAt, t.LastSeenAt = parseTime(first), parseTime(last)
	return t, nil
}

// Create inserts t and sets its ID.
func (r Topics) Create(ctx context.Context, t *domain.Topic) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO chat_topics (chat_id, external_id, title, title_set_by_admin, gone,
		first_seen_at, last_seen_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, t.ChatID, t.ExternalID, t.Title, boolInt(t.TitleSetByAdmin),
		boolInt(t.Gone), fmtTime(t.FirstSeenAt), fmtTime(t.LastSeenAt))
	if err != nil {
		return mapErr(err)
	}
	t.ID, err = res.LastInsertId()
	return err
}

// Update writes every field of t.
func (r Topics) Update(ctx context.Context, t *domain.Topic) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE chat_topics SET chat_id = ?, external_id = ?, title = ?,
		title_set_by_admin = ?, gone = ?, first_seen_at = ?, last_seen_at = ? WHERE id = ?`, t.ChatID, t.ExternalID, t.Title,
		boolInt(t.TitleSetByAdmin), boolInt(t.Gone), fmtTime(t.FirstSeenAt), fmtTime(t.LastSeenAt), t.ID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Get returns one topic.
func (r Topics) Get(ctx context.Context, id int64) (domain.Topic, error) {
	return scanTopic(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+topicColumns+` FROM chat_topics WHERE id = ?`, id))
}

// GetByExternal finds a topic by its thread id inside a chat.
func (r Topics) GetByExternal(ctx context.Context, chatID int64, externalID string) (domain.Topic, error) {
	return scanTopic(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+topicColumns+` FROM chat_topics WHERE chat_id = ? AND external_id = ?`,
		chatID, externalID))
}

// ListByChat returns a chat's topics by title.
func (r Topics) ListByChat(ctx context.Context, chatID int64) ([]domain.Topic, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT `+topicColumns+` FROM chat_topics WHERE chat_id = ? ORDER BY title = '', title, id`, chatID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Topic
	for rows.Next() {
		t, err := scanTopic(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// MoveToChat moves topics to another chat, leaving behind those whose external id exists there.
func (r Topics) MoveToChat(ctx context.Context, fromChatID, toChatID int64) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE chat_topics SET chat_id = ? WHERE chat_id = ? AND external_id NOT IN
		(SELECT external_id FROM chat_topics WHERE chat_id = ?)`, toChatID, fromChatID, toChatID)
	return err
}
