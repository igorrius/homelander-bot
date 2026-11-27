package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"unicode"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Users implements domain.UserRepo.
type Users struct{ DB DB }

var _ domain.UserRepo = Users{}

const userColumns = `id, phone, first_name, last_name, locale, is_resident, is_admin, status, source,
	approved_by, approved_at, activated_at, created_at, updated_at`

func searchText(u *domain.User) string {
	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, u.Phone)
	return strings.ToLower(u.FirstName + " " + u.LastName + " " + digits)
}

// Create inserts u and sets its ID and timestamps.
func (r Users) Create(ctx context.Context, u *domain.User) error {
	t := now()
	u.CreatedAt, u.UpdatedAt = t, t
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO users (phone, first_name, last_name, locale, is_resident,
		is_admin, status, source, approved_by, approved_at, activated_at, created_at, updated_at, search_text)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Phone, u.FirstName, u.LastName, u.Locale, boolInt(u.IsResident), boolInt(u.IsAdmin), u.Status, u.Source,
		nullInt(u.ApprovedBy), fmtTimePtr(u.ApprovedAt), fmtTimePtr(u.ActivatedAt), fmtTime(t), fmtTime(t), searchText(u))
	if err != nil {
		return mapErr(err)
	}
	u.ID, err = res.LastInsertId()
	return err
}

// Update writes every mutable field of u.
func (r Users) Update(ctx context.Context, u *domain.User) error {
	u.UpdatedAt = now()
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE users SET phone = ?, first_name = ?, last_name = ?, locale = ?,
		is_resident = ?, is_admin = ?, status = ?, source = ?, approved_by = ?, approved_at = ?, activated_at = ?,
		updated_at = ?, search_text = ? WHERE id = ?`,
		u.Phone, u.FirstName, u.LastName, u.Locale, boolInt(u.IsResident), boolInt(u.IsAdmin), u.Status, u.Source,
		nullInt(u.ApprovedBy), fmtTimePtr(u.ApprovedAt), fmtTimePtr(u.ActivatedAt), fmtTime(u.UpdatedAt), searchText(u), u.ID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type scanner interface{ Scan(dest ...any) error }

// userRow scans userColumns into a domain.User.
type userRow struct {
	u                     *domain.User
	resident, admin       int
	approvedBy            sql.NullInt64
	approvedAt, activated sql.NullString
	createdAt, updatedAt  string
}

func (r *userRow) dest() []any {
	u := r.u
	return []any{&u.ID, &u.Phone, &u.FirstName, &u.LastName, &u.Locale, &r.resident, &r.admin, &u.Status, &u.Source,
		&r.approvedBy, &r.approvedAt, &r.activated, &r.createdAt, &r.updatedAt}
}

func (r *userRow) finish() {
	u := r.u
	u.IsResident, u.IsAdmin = r.resident == 1, r.admin == 1
	u.ApprovedBy = intPtr(r.approvedBy)
	u.ApprovedAt, u.ActivatedAt = parseTimePtr(r.approvedAt), parseTimePtr(r.activated)
	u.CreatedAt, u.UpdatedAt = parseTime(r.createdAt), parseTime(r.updatedAt)
}

func scanUser(s scanner) (domain.User, error) {
	var u domain.User
	row := &userRow{u: &u}
	if err := s.Scan(row.dest()...); err != nil {
		return domain.User{}, mapErr(err)
	}
	row.finish()
	return u, nil
}

// prefixed qualifies a comma-separated column list with a table alias.
func prefixed(alias, columns string) string {
	parts := strings.Split(columns, ",")
	for i, p := range parts {
		parts[i] = alias + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}

// Get returns the user with id, or domain.ErrNotFound.
func (r Users) Get(ctx context.Context, id int64) (domain.User, error) {
	return scanUser(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

// GetByPhone returns the user with an E.164 phone, or domain.ErrNotFound.
func (r Users) GetByPhone(ctx context.Context, phone string) (domain.User, error) {
	return scanUser(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE phone = ?`, phone))
}

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// List returns one page of users matching f and the total number of matches.
func (r Users) List(ctx context.Context, f domain.UserFilter) ([]domain.User, int, error) {
	where := []string{"1 = 1"}
	var args []any
	for _, tok := range strings.Fields(strings.ToLower(f.Query)) {
		where = append(where, `search_text LIKE ? ESCAPE '\'`)
		args = append(args, "%"+likeEscape(tok)+"%")
	}
	if f.HouseID != nil {
		where = append(where, `id IN (SELECT user_id FROM residencies WHERE house_id = ?)`)
		args = append(args, *f.HouseID)
	}
	if f.Status != nil {
		where = append(where, `status = ?`)
		args = append(args, *f.Status)
	}
	if f.IsAdmin != nil {
		where = append(where, `is_admin = ?`)
		args = append(args, boolInt(*f.IsAdmin))
	}
	if f.IsResident != nil {
		where = append(where, `is_resident = ?`)
		args = append(args, boolInt(*f.IsResident))
	}
	cond := strings.Join(where, " AND ")
	q := r.DB.Q(ctx)
	var total int
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := f.Limit
	if limit <= 0 {
		limit = -1
	}
	rows, err := q.QueryContext(ctx, `SELECT `+userColumns+` FROM users WHERE `+cond+
		` ORDER BY last_name, first_name, id LIMIT ? OFFSET ?`, append(args, limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

// CountActiveAdmins counts admins whose status is active.
func (r Users) CountActiveAdmins(ctx context.Context) (int, error) {
	var n int
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT count(*) FROM users WHERE is_admin = 1 AND status = 'active'`).Scan(&n)
	return n, err
}

// CountByStatus counts users in status s.
func (r Users) CountByStatus(ctx context.Context, s domain.UserStatus) (int, error) {
	var n int
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT count(*) FROM users WHERE status = ?`, s).Scan(&n)
	return n, err
}

// Channels implements domain.ChannelRepo.
type Channels struct{ DB DB }

var _ domain.ChannelRepo = Channels{}

// Create links a channel; a taken (type, external id) or a second channel of
// the same type for the user is domain.ErrConflict.
func (r Channels) Create(ctx context.Context, c *domain.Channel) error {
	if c.LinkedAt.IsZero() {
		c.LinkedAt = now()
	}
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO channels (user_id, type, external_id, chat_id, username, linked_at)
		VALUES (?, ?, ?, ?, ?, ?)`, c.UserID, c.Type, c.ExternalID, c.ChatID, c.Username, fmtTime(c.LinkedAt))
	if err != nil {
		return mapErr(err)
	}
	c.ID, err = res.LastInsertId()
	return err
}

func scanChannel(s scanner) (domain.Channel, error) {
	var c domain.Channel
	var linked string
	if err := s.Scan(&c.ID, &c.UserID, &c.Type, &c.ExternalID, &c.ChatID, &c.Username, &linked); err != nil {
		return domain.Channel{}, mapErr(err)
	}
	c.LinkedAt = parseTime(linked)
	return c, nil
}

const channelColumns = `id, user_id, type, external_id, chat_id, username, linked_at`

// ListByUser returns the user's channels.
func (r Channels) ListByUser(ctx context.Context, userID int64) ([]domain.Channel, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT `+channelColumns+` FROM channels WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Channel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetByExternal finds a channel by messenger type and external id.
func (r Channels) GetByExternal(ctx context.Context, typ, externalID string) (domain.Channel, error) {
	return scanChannel(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+channelColumns+` FROM channels WHERE type = ? AND external_id = ?`, typ, externalID))
}

// Delete removes a channel.
func (r Channels) Delete(ctx context.Context, id int64) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM channels WHERE id = ?`, id)
	return err
}
