package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Houses implements domain.HouseRepo.
type Houses struct{ DB DB }

var _ domain.HouseRepo = Houses{}

const houseColumns = `id, name, address, latitude, longitude, floors, apartment_from, apartment_to, sort_order, archived`

func nullFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullIntVal(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func scanHouse(s scanner) (domain.House, error) {
	var (
		h        domain.House
		lat, lon sql.NullFloat64
		floors   sql.NullInt64
		archived int
	)
	if err := s.Scan(&h.ID, &h.Name, &h.Address, &lat, &lon, &floors, &h.ApartmentFrom, &h.ApartmentTo, &h.SortOrder, &archived); err != nil {
		return domain.House{}, mapErr(err)
	}
	if lat.Valid {
		h.Latitude = &lat.Float64
	}
	if lon.Valid {
		h.Longitude = &lon.Float64
	}
	if floors.Valid {
		f := int(floors.Int64)
		h.Floors = &f
	}
	h.Archived = archived == 1
	return h, nil
}

// Create inserts a house.
func (r Houses) Create(ctx context.Context, h *domain.House) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO houses (name, address, latitude, longitude, floors,
		apartment_from, apartment_to, sort_order, archived) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.Name, h.Address, nullFloat(h.Latitude), nullFloat(h.Longitude), nullIntVal(h.Floors),
		h.ApartmentFrom, h.ApartmentTo, h.SortOrder, boolInt(h.Archived))
	if err != nil {
		return mapErr(err)
	}
	h.ID, err = res.LastInsertId()
	return err
}

// Update writes every field of h.
func (r Houses) Update(ctx context.Context, h *domain.House) error {
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE houses SET name = ?, address = ?, latitude = ?, longitude = ?,
		floors = ?, apartment_from = ?, apartment_to = ?, sort_order = ?, archived = ? WHERE id = ?`,
		h.Name, h.Address, nullFloat(h.Latitude), nullFloat(h.Longitude), nullIntVal(h.Floors),
		h.ApartmentFrom, h.ApartmentTo, h.SortOrder, boolInt(h.Archived), h.ID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Get returns a house or domain.ErrNotFound.
func (r Houses) Get(ctx context.Context, id int64) (domain.House, error) {
	return scanHouse(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+houseColumns+` FROM houses WHERE id = ?`, id))
}

// List returns houses by sort order and name.
func (r Houses) List(ctx context.Context, includeArchived bool) ([]domain.House, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT `+houseColumns+` FROM houses WHERE archived = 0 OR ?
		ORDER BY sort_order, name, id`, includeArchived)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.House
	for rows.Next() {
		h, err := scanHouse(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// countedResidents selects the residencies of a house that statistics count.
const countedResidents = `FROM residencies r JOIN users u ON u.id = r.user_id
	WHERE r.house_id = ? AND u.is_resident = 1
	AND u.status IN ('pending_approval', 'awaiting_activation', 'active')`

// Stats summarises the house's residents.
func (r Houses) Stats(ctx context.Context, houseID int64) (domain.HouseStats, error) {
	h, err := r.Get(ctx, houseID)
	if err != nil {
		return domain.HouseStats{}, err
	}
	s := domain.HouseStats{
		ApartmentsTotal:   h.ApartmentTo - h.ApartmentFrom + 1,
		ByResidencyStatus: map[int64]int{},
		ByUserStatus:      map[domain.UserStatus]int{},
		Apartments:        map[int]int{},
	}
	q := r.DB.Q(ctx)
	if err := q.QueryRowContext(ctx, `SELECT count(DISTINCT r.user_id) `+countedResidents, houseID).Scan(&s.People); err != nil {
		return s, err
	}
	groups := []struct {
		sql string
		add func(k any, n int)
	}{
		{`SELECT r.apartment_number, count(DISTINCT r.user_id) ` + countedResidents + ` GROUP BY 1`,
			func(k any, n int) { s.Apartments[int(k.(int64))] = n }},
		{`SELECT r.status_id, count(DISTINCT r.user_id) ` + countedResidents + ` GROUP BY 1`,
			func(k any, n int) { s.ByResidencyStatus[k.(int64)] = n }},
		{`SELECT u.status, count(DISTINCT r.user_id) ` + countedResidents + ` GROUP BY 1`,
			func(k any, n int) { s.ByUserStatus[domain.UserStatus(k.(string))] = n }},
	}
	for _, g := range groups {
		if err := groupCount(ctx, q, g.sql, houseID, g.add); err != nil {
			return s, err
		}
	}
	s.ApartmentsWithResidents = len(s.Apartments)
	return s, nil
}

func groupCount(ctx context.Context, q Querier, query string, houseID int64, add func(k any, n int)) error {
	rows, err := q.QueryContext(ctx, query, houseID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var k any
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return err
		}
		add(k, n)
	}
	return rows.Err()
}

// Residents lists the users registered in one apartment.
func (r Houses) Residents(ctx context.Context, houseID int64, apartment int) ([]domain.ApartmentResident, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT `+prefixed("u.", userColumns)+`,
		r.id, r.user_id, r.house_id, r.apartment_number, r.status_id, r.created_at
		FROM residencies r JOIN users u ON u.id = r.user_id
		WHERE r.house_id = ? AND r.apartment_number = ? ORDER BY u.last_name, u.first_name, u.id`, houseID, apartment)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.ApartmentResident
	for rows.Next() {
		var ar domain.ApartmentResident
		var created string
		row := &userRow{u: &ar.User}
		dest := append(row.dest(), &ar.Residency.ID, &ar.Residency.UserID, &ar.Residency.HouseID,
			&ar.Residency.Apartment, &ar.Residency.StatusID, &created)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		row.finish()
		ar.Residency.CreatedAt = parseTime(created)
		out = append(out, ar)
	}
	return out, rows.Err()
}

// ResidenciesOutside counts the house's residencies outside from..to.
func (r Houses) ResidenciesOutside(ctx context.Context, houseID int64, from, to int) (int, error) {
	var n int
	err := r.DB.Q(ctx).QueryRowContext(ctx, `SELECT count(*) FROM residencies WHERE house_id = ?
		AND (apartment_number < ? OR apartment_number > ?)`, houseID, from, to).Scan(&n)
	return n, err
}

// Statuses implements domain.StatusRepo.
type Statuses struct{ DB DB }

var _ domain.StatusRepo = Statuses{}

func scanStatus(s scanner) (domain.ResidencyStatus, error) {
	var st domain.ResidencyStatus
	var names string
	var archived int
	if err := s.Scan(&st.ID, &st.Code, &names, &st.SortOrder, &archived); err != nil {
		return st, mapErr(err)
	}
	st.Archived = archived == 1
	return st, json.Unmarshal([]byte(names), &st.Names)
}

// Create inserts a status; a taken code is domain.ErrConflict.
func (r Statuses) Create(ctx context.Context, s *domain.ResidencyStatus) error {
	names, err := json.Marshal(s.Names)
	if err != nil {
		return err
	}
	res, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO residency_statuses (code, names, sort_order, archived)
		VALUES (?, ?, ?, ?)`, s.Code, string(names), s.SortOrder, boolInt(s.Archived))
	if err != nil {
		return mapErr(err)
	}
	s.ID, err = res.LastInsertId()
	return err
}

// Update writes every field of s.
func (r Statuses) Update(ctx context.Context, s *domain.ResidencyStatus) error {
	names, err := json.Marshal(s.Names)
	if err != nil {
		return err
	}
	res, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE residency_statuses SET code = ?, names = ?, sort_order = ?,
		archived = ? WHERE id = ?`, s.Code, string(names), s.SortOrder, boolInt(s.Archived), s.ID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Get returns a status or domain.ErrNotFound.
func (r Statuses) Get(ctx context.Context, id int64) (domain.ResidencyStatus, error) {
	return scanStatus(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT id, code, names, sort_order, archived
		FROM residency_statuses WHERE id = ?`, id))
}

// List returns statuses by sort order.
func (r Statuses) List(ctx context.Context, includeArchived bool) ([]domain.ResidencyStatus, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT id, code, names, sort_order, archived FROM residency_statuses
		WHERE archived = 0 OR ? ORDER BY sort_order, id`, includeArchived)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.ResidencyStatus
	for rows.Next() {
		s, err := scanStatus(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Residencies implements domain.ResidencyRepo.
type Residencies struct{ DB DB }

var _ domain.ResidencyRepo = Residencies{}

const residencyColumns = `id, user_id, house_id, apartment_number, status_id, created_at`

func scanResidency(s scanner) (domain.Residency, error) {
	var r domain.Residency
	var created string
	if err := s.Scan(&r.ID, &r.UserID, &r.HouseID, &r.Apartment, &r.StatusID, &created); err != nil {
		return r, mapErr(err)
	}
	r.CreatedAt = parseTime(created)
	return r, nil
}

// Create inserts a residency; a duplicate is domain.ErrConflict.
func (r Residencies) Create(ctx context.Context, res *domain.Residency) error {
	if res.CreatedAt.IsZero() {
		res.CreatedAt = now()
	}
	out, err := r.DB.Q(ctx).ExecContext(ctx, `INSERT INTO residencies (user_id, house_id, apartment_number, status_id, created_at)
		VALUES (?, ?, ?, ?, ?)`, res.UserID, res.HouseID, res.Apartment, res.StatusID, fmtTime(res.CreatedAt))
	if err != nil {
		return mapErr(err)
	}
	res.ID, err = out.LastInsertId()
	return err
}

// Update moves a residency to another house/apartment/status.
func (r Residencies) Update(ctx context.Context, res *domain.Residency) error {
	out, err := r.DB.Q(ctx).ExecContext(ctx, `UPDATE residencies SET house_id = ?, apartment_number = ?, status_id = ?
		WHERE id = ?`, res.HouseID, res.Apartment, res.StatusID, res.ID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := out.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Get returns a residency or domain.ErrNotFound.
func (r Residencies) Get(ctx context.Context, id int64) (domain.Residency, error) {
	return scanResidency(r.DB.Q(ctx).QueryRowContext(ctx, `SELECT `+residencyColumns+` FROM residencies WHERE id = ?`, id))
}

// Delete removes a residency.
func (r Residencies) Delete(ctx context.Context, id int64) error {
	_, err := r.DB.Q(ctx).ExecContext(ctx, `DELETE FROM residencies WHERE id = ?`, id)
	return err
}

// ListByUser returns a user's residencies.
func (r Residencies) ListByUser(ctx context.Context, userID int64) ([]domain.Residency, error) {
	rows, err := r.DB.Q(ctx).QueryContext(ctx, `SELECT `+residencyColumns+` FROM residencies WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Residency
	for rows.Next() {
		res, err := scanResidency(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, rows.Err()
}
