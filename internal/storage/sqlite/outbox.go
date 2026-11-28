package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Outbox implements domain.OutboxRepo.
type Outbox struct{ DB DB }

// RegTokens implements domain.RegTokenRepo.
type RegTokens struct{ DB DB }

// Notifications implements domain.NotificationRepo under the "notifications" settings key.
type Notifications struct{ DB DB }

var (
	_ domain.OutboxRepo       = Outbox{}
	_ domain.RegTokenRepo     = RegTokens{}
	_ domain.NotificationRepo = Notifications{}
)

const outboxColumns = `id, messenger, chat_id, thread_id, payload, origin, label_id, user_id, state, attempts,
	next_attempt_at, last_error, created_at, sent_at`

type payloadJSON struct {
	Text    string       `json:"text"`
	Buttons []buttonJSON `json:"buttons,omitempty"`
	Image   int64        `json:"image,omitempty"` // asset id of a photo
}

type buttonJSON struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

func encodePayload(m domain.OutMessage) (string, error) {
	p := payloadJSON{Text: m.Text, Image: m.ImageAssetID}
	for _, b := range m.Buttons {
		p.Buttons = append(p.Buttons, buttonJSON(b))
	}
	raw, err := json.Marshal(p)
	return string(raw), err
}

func scanOutbox(s scanner) (domain.OutboxItem, error) {
	var (
		it                     domain.OutboxItem
		payload, next, created string
		labelID, userID        sql.NullInt64
		sent                   sql.NullString
	)
	if err := s.Scan(&it.ID, &it.Messenger, &it.To.ChatID, &it.To.ThreadID, &payload, &it.Origin, &labelID, &userID,
		&it.State, &it.Attempts, &next, &it.LastError, &created, &sent); err != nil {
		return domain.OutboxItem{}, mapErr(err)
	}
	var p payloadJSON
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return domain.OutboxItem{}, err
	}
	it.Msg.Text, it.Msg.ImageAssetID = p.Text, p.Image
	for _, b := range p.Buttons {
		it.Msg.Buttons = append(it.Msg.Buttons, domain.LinkButton(b))
	}
	it.LabelID, it.UserID = intPtr(labelID), intPtr(userID)
	it.NextAttemptAt, it.CreatedAt, it.SentAt = parseTime(next), parseTime(created), parseTimePtr(sent)
	return it, nil
}

// Add inserts it and sets its ID.
func (r Outbox) Add(ctx context.Context, it *domain.OutboxItem) error {
	payload, err := encodePayload(it.Msg)
	if err != nil {
		return err
	}
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO outbox (messenger, chat_id, thread_id, payload, origin, label_id,
		user_id, state, attempts, next_attempt_at, last_error, created_at, sent_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		it.Messenger, it.To.ChatID, it.To.ThreadID, payload, it.Origin, nullInt(it.LabelID), nullInt(it.UserID), string(it.State),
		it.Attempts, fmtTime(it.NextAttemptAt), it.LastError, fmtTime(it.CreatedAt), fmtTimePtr(it.SentAt))
	if err != nil {
		return mapErr(err)
	}
	it.ID, err = res.LastInsertId()
	return err
}

// Get returns one row.
func (r Outbox) Get(ctx context.Context, id int64) (domain.OutboxItem, error) {
	return scanOutbox(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+outboxColumns+` FROM outbox WHERE id = ?`, id))
}

// Due returns, for each chat, its oldest pending row if that row's next
// attempt has come; oldest first.
func (r Outbox) Due(ctx context.Context, now time.Time, limit int) ([]domain.OutboxItem, error) {
	// Only the oldest pending row of each chat is a candidate, so a chat gets
	// its messages in order and a backlog in one chat does not hide others.
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT `+outboxColumns+` FROM outbox o
		WHERE state = 'pending' AND next_attempt_at <= ? AND NOT EXISTS (SELECT 1 FROM outbox p
			WHERE p.state = 'pending' AND p.messenger = o.messenger AND p.chat_id = o.chat_id AND p.id < o.id)
		ORDER BY id LIMIT ?`, fmtTime(now), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.OutboxItem
	for rows.Next() {
		it, err := scanOutbox(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ListByOrigin returns the rows of one origin by id.
func (r Outbox) ListByOrigin(ctx context.Context, origin string) ([]domain.OutboxItem, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT `+outboxColumns+` FROM outbox WHERE origin = ? ORDER BY id`, origin)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []domain.OutboxItem{}
	for rows.Next() {
		it, err := scanOutbox(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Update writes destination, state and delivery fields.
func (r Outbox) Update(ctx context.Context, it *domain.OutboxItem) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE outbox SET chat_id = ?, thread_id = ?, state = ?, attempts = ?,
		next_attempt_at = ?, last_error = ?, sent_at = ? WHERE id = ?`, it.To.ChatID, it.To.ThreadID, string(it.State),
		it.Attempts, fmtTime(it.NextAttemptAt), it.LastError, fmtTimePtr(it.SentAt), it.ID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// RepointChat moves pending rows from a chat's old id to its new one.
func (r Outbox) RepointChat(ctx context.Context, messenger, oldChatID, newChatID string) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE outbox SET chat_id = ? WHERE messenger = ? AND chat_id = ? AND state = 'pending'`,
		newChatID, messenger, oldChatID)
	return err
}

// DeleteFinishedBefore removes sent and failed rows created before t.
func (r Outbox) DeleteFinishedBefore(ctx context.Context, t time.Time) (int, error) {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM outbox WHERE state IN ('sent', 'failed') AND created_at < ?`, fmtTime(t))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// regTokenColumns lists columns, not credentials.
const regTokenColumns = `id, token_hash, channel_type, external_id, chat_id, username, phone, expires_at, used_at, created_at` //nolint:gosec // column names

// Create inserts t and sets its ID.
func (r RegTokens) Create(ctx context.Context, t *domain.RegistrationToken) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO registration_tokens (token_hash, channel_type, external_id, chat_id,
		username, phone, expires_at, used_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.TokenHash, t.ChannelType,
		t.ExternalID, t.ChatID, t.Username, t.Phone, fmtTime(t.ExpiresAt), fmtTimePtr(t.UsedAt), fmtTime(t.CreatedAt))
	if err != nil {
		return mapErr(err)
	}
	t.ID, err = res.LastInsertId()
	return err
}

// GetByHash finds a token by its hash.
func (r RegTokens) GetByHash(ctx context.Context, hash []byte) (domain.RegistrationToken, error) {
	var (
		t                domain.RegistrationToken
		expires, created string
		used             sql.NullString
	)
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+regTokenColumns+` FROM registration_tokens WHERE token_hash = ?`, hash).
		Scan(&t.ID, &t.TokenHash, &t.ChannelType, &t.ExternalID, &t.ChatID, &t.Username, &t.Phone, &expires, &used, &created)
	if err != nil {
		return domain.RegistrationToken{}, mapErr(err)
	}
	t.ExpiresAt, t.UsedAt, t.CreatedAt = parseTime(expires), parseTimePtr(used), parseTime(created)
	return t, nil
}

// MarkUsed stamps a token used.
func (r RegTokens) MarkUsed(ctx context.Context, id int64, at time.Time) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE registration_tokens SET used_at = ? WHERE id = ?`, fmtTime(at), id)
	return err
}

// InvalidateFor marks every unused token of a messenger account used.
func (r RegTokens) InvalidateFor(ctx context.Context, channelType, externalID string, at time.Time) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE registration_tokens SET used_at = ?
		WHERE channel_type = ? AND external_id = ? AND used_at IS NULL`, fmtTime(at), channelType, externalID)
	return err
}

// DeleteExpiredBefore removes tokens that expired before t.
func (r RegTokens) DeleteExpiredBefore(ctx context.Context, t time.Time) (int, error) {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM registration_tokens WHERE expires_at < ?`, fmtTime(t))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

type notificationsJSON struct {
	ApplicationsLabelID *int64 `json:"applicationsLabelId"`
}

// Get returns the settings; the zero value when nothing was saved.
func (r Notifications) Get(ctx context.Context) (domain.NotificationSettings, error) {
	var raw string
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'notifications'`).Scan(&raw)
	if errors.Is(mapErr(err), domain.ErrNotFound) {
		return domain.NotificationSettings{}, nil
	} else if err != nil {
		return domain.NotificationSettings{}, err
	}
	var v notificationsJSON
	err = json.Unmarshal([]byte(raw), &v)
	return domain.NotificationSettings(v), err
}

// Save writes the settings.
func (r Notifications) Save(ctx context.Context, s domain.NotificationSettings) error {
	raw, err := json.Marshal(notificationsJSON(s))
	if err != nil {
		return err
	}
	_, err = r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES ('notifications', ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, string(raw), fmtTime(now()))
	return err
}
