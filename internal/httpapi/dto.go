package httpapi

import (
	"context"
	"strings"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// residencyDTO is a residency with the names the web shows.
type residencyDTO struct {
	ID          int64                `json:"id"`
	HouseID     int64                `json:"houseId"`
	HouseName   string               `json:"houseName"`
	Apartment   int                  `json:"apartment"`
	StatusID    int64                `json:"statusId"`
	StatusNames domain.LocalizedText `json:"statusNames"`
}

type userDTO struct {
	ID          int64          `json:"id"`
	Phone       string         `json:"phone"`
	FirstName   string         `json:"firstName"`
	LastName    string         `json:"lastName"`
	Locale      string         `json:"locale"`
	IsResident  bool           `json:"isResident"`
	IsAdmin     bool           `json:"isAdmin"`
	IsMCP       *bool          `json:"isMcp,omitempty"`
	Status      string         `json:"status,omitempty"`
	Source      string         `json:"source,omitempty"`
	HasTelegram *bool          `json:"hasTelegram,omitempty"`
	CreatedAt   *time.Time     `json:"createdAt,omitempty"`
	ApprovedAt  *time.Time     `json:"approvedAt,omitempty"`
	ActivatedAt *time.Time     `json:"activatedAt,omitempty"`
	Residencies []residencyDTO `json:"residencies"`
}

type houseDTO struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Address       string   `json:"address"`
	Latitude      *float64 `json:"latitude"`
	Longitude     *float64 `json:"longitude"`
	Floors        *int     `json:"floors"`
	ApartmentFrom int      `json:"apartmentFrom"`
	ApartmentTo   int      `json:"apartmentTo"`
	SortOrder     int      `json:"sortOrder"`
	Archived      bool     `json:"archived"`
}

func toHouse(h domain.House) houseDTO {
	return houseDTO{h.ID, h.Name, h.Address, h.Latitude, h.Longitude, h.Floors, h.ApartmentFrom, h.ApartmentTo, h.SortOrder, h.Archived}
}

func (h houseDTO) domain() domain.House {
	return domain.House{ID: h.ID, Name: h.Name, Address: h.Address, Latitude: h.Latitude, Longitude: h.Longitude, Floors: h.Floors,
		ApartmentFrom: h.ApartmentFrom, ApartmentTo: h.ApartmentTo, SortOrder: h.SortOrder, Archived: h.Archived}
}

type statusDTO struct {
	ID        int64                `json:"id"`
	Code      string               `json:"code"`
	Names     domain.LocalizedText `json:"names"`
	SortOrder int                  `json:"sortOrder"`
	Archived  bool                 `json:"archived"`
}

func toStatus(s domain.ResidencyStatus) statusDTO {
	return statusDTO{s.ID, s.Code, s.Names, s.SortOrder, s.Archived}
}

// namer resolves house and status names for residencies, loading each list once.
type namer struct {
	api      API
	houses   map[int64]domain.House
	statuses map[int64]domain.ResidencyStatus
	users    map[int64]string // display names looked up so far
	// Chat, topic and label names, loaded on first use (nil until then).
	chats, topics, labels map[int64]string
}

func newNamer(ctx context.Context, api API) (*namer, error) {
	n := &namer{api: api, houses: map[int64]domain.House{}, statuses: map[int64]domain.ResidencyStatus{}, users: map[int64]string{}}
	hs, err := api.Catalog.Houses(ctx, true)
	if err != nil {
		return nil, err
	}
	for _, h := range hs {
		n.houses[h.ID] = h
	}
	ss, err := api.Catalog.Statuses(ctx, true)
	if err != nil {
		return nil, err
	}
	for _, s := range ss {
		n.statuses[s.ID] = s
	}
	return n, nil
}

// userName returns a user's full name, their phone when nameless, or "" when the user cannot be read.
func (n *namer) userName(ctx context.Context, id int64) string {
	if name, ok := n.users[id]; ok {
		return name
	}
	name := ""
	if u, err := n.api.Users.Get(ctx, id); err == nil {
		name = strings.TrimSpace(u.FirstName + " " + u.LastName)
		if name == "" {
			name = u.Phone
		}
	}
	n.users[id] = name
	return name
}

// chatNames fills the chat, topic ("Chat › Topic") and label name maps once per request.
// A list that cannot be read leaves its names empty; the client then shows the raw target.
func (n *namer) chatNames(ctx context.Context) {
	if n.chats != nil {
		return
	}
	n.chats, n.topics, n.labels = map[int64]string{}, map[int64]string{}, map[int64]string{}
	if cs, err := n.api.Chats.List(ctx); err == nil {
		for _, c := range cs {
			n.chats[c.ID] = c.Title
			for _, t := range c.Topics {
				n.topics[t.ID] = c.Title + " › " + t.Title
			}
		}
	}
	if ls, err := n.api.Labels.List(ctx); err == nil {
		for _, l := range ls {
			n.labels[l.ID] = l.Name
		}
	}
}

func (n *namer) residencies(rs []domain.Residency) []residencyDTO {
	out := make([]residencyDTO, 0, len(rs))
	for _, r := range rs {
		out = append(out, residencyDTO{ID: r.ID, HouseID: r.HouseID, HouseName: n.houses[r.HouseID].Name, Apartment: r.Apartment,
			StatusID: r.StatusID, StatusNames: n.statuses[r.StatusID].Names})
	}
	return out
}

// toUser builds the public view of a user (no status details).
func (n *namer) toUser(u domain.User, rs []domain.Residency) userDTO {
	return userDTO{ID: u.ID, Phone: u.Phone, FirstName: u.FirstName, LastName: u.LastName, Locale: u.Locale,
		IsResident: u.IsResident, IsAdmin: u.IsAdmin, Residencies: n.residencies(rs)}
}

// toAdminUser adds what only admins see.
func (n *namer) toAdminUser(u domain.User, rs []domain.Residency, hasTelegram bool) userDTO {
	d := n.toUser(u, rs)
	d.Status, d.Source, d.HasTelegram = string(u.Status), string(u.Source), &hasTelegram
	d.IsMCP = &u.IsMCP
	d.CreatedAt, d.ApprovedAt, d.ActivatedAt = &u.CreatedAt, u.ApprovedAt, u.ActivatedAt
	return d
}
