package domain

import (
	"context"
	"time"
)

// LocalizedText maps a locale ("uk", "en") to text entered by an admin.
type LocalizedText map[string]string

// House is one building (or one section of a big building).
type House struct {
	ID            int64
	Name          string
	Address       string
	Latitude      *float64
	Longitude     *float64
	Floors        *int
	ApartmentFrom int
	ApartmentTo   int
	SortOrder     int
	Archived      bool
}

// ResidencyStatus is a dictionary entry such as owner or tenant.
type ResidencyStatus struct {
	ID        int64
	Code      string
	Names     LocalizedText
	SortOrder int
	Archived  bool
}

// Residency ties a user to an apartment with a status.
type Residency struct {
	ID        int64
	UserID    int64
	HouseID   int64
	Apartment int
	StatusID  int64
	CreatedAt time.Time
}

// HouseStats summarises who lives in a house. Only residents who are pending
// approval, awaiting activation or active are counted.
type HouseStats struct {
	ApartmentsTotal         int
	ApartmentsWithResidents int
	People                  int
	ByResidencyStatus       map[int64]int
	ByUserStatus            map[UserStatus]int
	Apartments              map[int]int // apartment number → people; only non-zero
}

// ApartmentResident is a user registered in an apartment.
type ApartmentResident struct {
	User      User
	Residency Residency
}

// HouseRepo stores houses.
type HouseRepo interface {
	Create(ctx context.Context, h *House) error
	Update(ctx context.Context, h *House) error
	Get(ctx context.Context, id int64) (House, error)
	List(ctx context.Context, includeArchived bool) ([]House, error)
	Stats(ctx context.Context, houseID int64) (HouseStats, error)
	Residents(ctx context.Context, houseID int64, apartment int) ([]ApartmentResident, error)
	ResidenciesOutside(ctx context.Context, houseID int64, from, to int) (int, error)
}

// StatusRepo stores residency statuses.
type StatusRepo interface {
	Create(ctx context.Context, s *ResidencyStatus) error
	Update(ctx context.Context, s *ResidencyStatus) error
	Get(ctx context.Context, id int64) (ResidencyStatus, error)
	List(ctx context.Context, includeArchived bool) ([]ResidencyStatus, error)
}

// ResidencyRepo stores residencies.
type ResidencyRepo interface {
	Create(ctx context.Context, r *Residency) error
	Update(ctx context.Context, r *Residency) error
	Get(ctx context.Context, id int64) (Residency, error)
	Delete(ctx context.Context, id int64) error
	ListByUser(ctx context.Context, userID int64) ([]Residency, error)
}
