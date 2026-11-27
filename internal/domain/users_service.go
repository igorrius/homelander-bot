package domain

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/igorrius/homelander-bot/internal/ratelimit"
	"github.com/igorrius/homelander-bot/internal/storage"
)

// Repos bundles the repositories services use.
type Repos struct {
	Tx            storage.TxRunner
	Users         UserRepo
	Channels      ChannelRepo
	Houses        HouseRepo
	Statuses      StatusRepo
	Residencies   ResidencyRepo
	Codes         CodeRepo
	Sessions      SessionRepo
	Site          SiteRepo
	Audit         AuditRepo
	Chats         ChatRepo
	Topics        TopicRepo
	Labels        LabelRepo
	Outbox        OutboxRepo
	RegTokens     RegTokenRepo
	Notifications NotificationRepo
}

// Actor is who makes a change: a signed-in user, or nobody (the public, the bot).
type Actor struct {
	UserID *int64
	IP     string
}

// Application is what a resident submits through the public registration form.
type Application struct {
	Phone     string
	FirstName string
	LastName  string
	HouseID   int64
	Apartment int
	StatusID  int64
	Consent   bool
	Locale    string
}

// NewUser is what an admin enters to add a user directly.
type NewUser struct {
	Phone      string
	FirstName  string
	LastName   string
	IsResident bool
	IsAdmin    bool
}

// UserPatch changes the fields that are set.
type UserPatch struct {
	FirstName  *string
	LastName   *string
	Phone      *string
	Locale     *string
	IsResident *bool
	IsAdmin    *bool
}

// UserCard is everything the admin user page shows.
type UserCard struct {
	User        User
	Channels    []Channel
	Residencies []Residency
	Audit       []AuditEntry
}

// UserService manages applications, review and admin changes to users.
type UserService struct {
	R          Repos
	Now        Clock
	Limits     *ratelimit.Limiter
	Region     string
	PendingCap int
	Locales    []string // supported locales; empty accepts any
}

const (
	maxNameLen    = 100
	applyLimit    = 5
	applyWindow   = time.Hour
	cardAuditSize = 20
)

func (s *UserService) audit(ctx context.Context, a Actor, action, target string, details map[string]any) error {
	return s.R.Audit.Add(ctx, AuditEntry{At: s.Now(), ActorUserID: a.UserID, Action: action, Target: target, Details: details, IP: a.IP})
}

func checkName(v validation, field, value string) string {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		v.add(field, "required")
	case utf8.RuneCountInString(value) > maxNameLen:
		v.add(field, "too_long")
	}
	return value
}

// checkResidency validates house, apartment and status of a residency.
func (s *UserService) checkResidency(ctx context.Context, v validation, houseID int64, apartment int, statusID int64) error {
	h, err := s.R.Houses.Get(ctx, houseID)
	switch {
	case errors.Is(err, ErrNotFound) || (err == nil && h.Archived):
		v.add("houseId", "invalid")
	case err != nil:
		return err
	case apartment < h.ApartmentFrom || apartment > h.ApartmentTo:
		v.add("apartment", "out_of_range")
	}
	st, err := s.R.Statuses.Get(ctx, statusID)
	switch {
	case errors.Is(err, ErrNotFound) || (err == nil && st.Archived):
		v.add("statusId", "invalid")
	case err != nil:
		return err
	}
	return nil
}

func (s *UserService) checkLocale(v validation, locale string) {
	if locale != "" && len(s.Locales) > 0 && !slices.Contains(s.Locales, locale) {
		v.add("locale", "invalid")
	}
}

// Apply registers an application. A phone that already belongs to a user who
// is not rejected gets the same nil answer and changes nothing, so the form
// cannot be used to probe which numbers are registered.
func (s *UserService) Apply(ctx context.Context, a Application, actor Actor) error {
	if !s.Limits.Allow("apply:"+actor.IP, applyLimit, applyWindow) {
		return ErrRateLimited
	}
	v := validation{}
	phone, err := NormalizePhone(a.Phone, s.Region)
	if err != nil {
		v.add("phone", "invalid")
	}
	first := checkName(v, "firstName", a.FirstName)
	last := checkName(v, "lastName", a.LastName)
	if err := s.checkResidency(ctx, v, a.HouseID, a.Apartment, a.StatusID); err != nil {
		return err
	}
	if !a.Consent {
		v.add("consent", "required")
	}
	s.checkLocale(v, a.Locale)
	if err := v.err(); err != nil {
		return err
	}
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		pending, err := s.R.Users.CountByStatus(ctx, StatusPendingApproval)
		if err != nil {
			return err
		}
		if s.PendingCap > 0 && pending >= s.PendingCap {
			return ErrTryLater
		}
		u, err := s.R.Users.GetByPhone(ctx, phone)
		switch {
		case err == nil && u.Status != StatusRejected:
			return nil
		case err == nil: // a rejected person applies again: reuse the row
			u.FirstName, u.LastName, u.Locale = first, last, a.Locale
			u.Status, u.IsResident, u.Source = StatusPendingApproval, true, SourceApplication
			u.ApprovedBy, u.ApprovedAt = nil, nil
			if err := s.R.Users.Update(ctx, &u); err != nil {
				return err
			}
			old, err := s.R.Residencies.ListByUser(ctx, u.ID)
			if err != nil {
				return err
			}
			for _, r := range old {
				if err := s.R.Residencies.Delete(ctx, r.ID); err != nil {
					return err
				}
			}
		case errors.Is(err, ErrNotFound):
			u = User{Phone: phone, FirstName: first, LastName: last, Locale: a.Locale, IsResident: true,
				Status: StatusPendingApproval, Source: SourceApplication}
			if err := s.R.Users.Create(ctx, &u); err != nil {
				return err
			}
		default:
			return err
		}
		r := Residency{UserID: u.ID, HouseID: a.HouseID, Apartment: a.Apartment, StatusID: a.StatusID, CreatedAt: s.Now()}
		if err := s.R.Residencies.Create(ctx, &r); err != nil {
			return err
		}
		return s.audit(ctx, actor, ActUserApplied, UserTarget(u.ID), map[string]any{"houseId": a.HouseID, "apartment": a.Apartment})
	})
}

func (s *UserService) review(ctx context.Context, id int64, actor Actor, approve bool) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		u, err := s.R.Users.Get(ctx, id)
		if err != nil {
			return err
		}
		if u.Status != StatusPendingApproval {
			return ErrConflict
		}
		action := ActUserRejected
		if approve {
			now := s.Now()
			u.Status, u.ApprovedBy, u.ApprovedAt, action = StatusAwaitingActivation, actor.UserID, &now, ActUserApproved
		} else {
			u.Status = StatusRejected
		}
		if err := s.R.Users.Update(ctx, &u); err != nil {
			return err
		}
		return s.audit(ctx, actor, action, UserTarget(id), nil)
	})
}

// Approve moves an application to awaiting activation.
func (s *UserService) Approve(ctx context.Context, id int64, actor Actor) error {
	return s.review(ctx, id, actor, true)
}

// Reject turns an application down; the person may apply again later.
func (s *UserService) Reject(ctx context.Context, id int64, actor Actor) error {
	return s.review(ctx, id, actor, false)
}

// AddUser creates a user directly (admins and other non-residents).
func (s *UserService) AddUser(ctx context.Context, n NewUser, actor Actor) (User, error) {
	v := validation{}
	phone, err := NormalizePhone(n.Phone, s.Region)
	if err != nil {
		v.add("phone", "invalid")
	}
	u := User{Phone: phone, FirstName: checkName(v, "firstName", n.FirstName), LastName: checkName(v, "lastName", n.LastName),
		IsResident: n.IsResident, IsAdmin: n.IsAdmin, Status: StatusAwaitingActivation, Source: SourceAdmin}
	if err := v.err(); err != nil {
		return User{}, err
	}
	err = s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if _, err := s.R.Users.GetByPhone(ctx, phone); err == nil {
			return &ValidationError{Fields: map[string]string{"phone": "taken"}}
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		if err := s.R.Users.Create(ctx, &u); err != nil {
			return err
		}
		return s.audit(ctx, actor, ActUserCreated, UserTarget(u.ID), map[string]any{"isAdmin": n.IsAdmin, "isResident": n.IsResident})
	})
	return u, err
}

// isLastActiveAdmin reports whether u is the only active admin.
func (s *UserService) isLastActiveAdmin(ctx context.Context, u User) (bool, error) {
	if !u.IsAdmin || u.Status != StatusActive {
		return false, nil
	}
	n, err := s.R.Users.CountActiveAdmins(ctx)
	return n <= 1, err
}

// Update applies a patch. Phone changes need a user without a linked channel.
func (s *UserService) Update(ctx context.Context, id int64, p UserPatch, actor Actor) (User, error) {
	var out User
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		u, err := s.R.Users.Get(ctx, id)
		if err != nil {
			return err
		}
		v := validation{}
		if p.FirstName != nil {
			u.FirstName = checkName(v, "firstName", *p.FirstName)
		}
		if p.LastName != nil {
			u.LastName = checkName(v, "lastName", *p.LastName)
		}
		if p.Locale != nil {
			s.checkLocale(v, *p.Locale)
			u.Locale = *p.Locale
		}
		endSessions := false
		if p.Phone != nil {
			phone, err := NormalizePhone(*p.Phone, s.Region)
			if err != nil {
				v.add("phone", "invalid")
			} else if phone != u.Phone {
				chans, err := s.R.Channels.ListByUser(ctx, u.ID)
				if err != nil {
					return err
				}
				if len(chans) > 0 {
					v.add("phone", "linked")
				} else if _, err := s.R.Users.GetByPhone(ctx, phone); err == nil {
					v.add("phone", "taken")
				} else if !errors.Is(err, ErrNotFound) {
					return err
				}
				u.Phone, endSessions = phone, true
			}
		}
		if err := v.err(); err != nil {
			return err
		}
		var actions []string
		if p.IsAdmin != nil && *p.IsAdmin != u.IsAdmin {
			if !*p.IsAdmin {
				last, err := s.isLastActiveAdmin(ctx, u)
				if err != nil {
					return err
				}
				if last {
					return ErrLastAdmin
				}
				actions, endSessions = append(actions, ActUserAdminRevoked), true
			} else {
				actions = append(actions, ActUserAdminGranted)
			}
			u.IsAdmin = *p.IsAdmin
		}
		if p.IsResident != nil && *p.IsResident != u.IsResident {
			u.IsResident = *p.IsResident
			if u.IsResident {
				actions = append(actions, ActUserResidentSet)
			} else {
				actions = append(actions, ActUserResidentUnset)
			}
		}
		if err := s.R.Users.Update(ctx, &u); err != nil {
			return err
		}
		if endSessions {
			if err := s.R.Sessions.DeleteByUser(ctx, u.ID); err != nil {
				return err
			}
		}
		if p.FirstName != nil || p.LastName != nil || p.Phone != nil || p.Locale != nil {
			actions = append(actions, ActUserUpdated)
		}
		for _, a := range actions {
			if err := s.audit(ctx, actor, a, UserTarget(u.ID), nil); err != nil {
				return err
			}
		}
		out = u
		return nil
	})
	return out, err
}

// Disable blocks a user and ends their sessions.
func (s *UserService) Disable(ctx context.Context, id int64, actor Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		u, err := s.R.Users.Get(ctx, id)
		if err != nil {
			return err
		}
		if u.Status == StatusDisabled {
			return nil
		}
		if last, err := s.isLastActiveAdmin(ctx, u); err != nil {
			return err
		} else if last {
			return ErrLastAdmin
		}
		u.Status = StatusDisabled
		if err := s.R.Users.Update(ctx, &u); err != nil {
			return err
		}
		if err := s.R.Sessions.DeleteByUser(ctx, u.ID); err != nil {
			return err
		}
		return s.audit(ctx, actor, ActUserDisabled, UserTarget(id), nil)
	})
}

// Enable unblocks a user: active again with a linked channel, else awaiting activation.
func (s *UserService) Enable(ctx context.Context, id int64, actor Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		u, err := s.R.Users.Get(ctx, id)
		if err != nil {
			return err
		}
		if u.Status != StatusDisabled {
			return ErrConflict
		}
		chans, err := s.R.Channels.ListByUser(ctx, u.ID)
		if err != nil {
			return err
		}
		u.Status = StatusAwaitingActivation
		if len(chans) > 0 {
			u.Status = StatusActive
		}
		if err := s.R.Users.Update(ctx, &u); err != nil {
			return err
		}
		return s.audit(ctx, actor, ActUserEnabled, UserTarget(id), nil)
	})
}

// UnlinkChannel removes a messenger link; an active user goes back to awaiting activation.
func (s *UserService) UnlinkChannel(ctx context.Context, userID, channelID int64, actor Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		u, err := s.R.Users.Get(ctx, userID)
		if err != nil {
			return err
		}
		chans, err := s.R.Channels.ListByUser(ctx, userID)
		if err != nil {
			return err
		}
		idx := slices.IndexFunc(chans, func(c Channel) bool { return c.ID == channelID })
		if idx < 0 {
			return ErrNotFound
		}
		if last, err := s.isLastActiveAdmin(ctx, u); err != nil {
			return err
		} else if last && len(chans) == 1 {
			return ErrLastAdmin
		}
		if err := s.R.Channels.Delete(ctx, channelID); err != nil {
			return err
		}
		if u.Status == StatusActive && len(chans) == 1 {
			u.Status = StatusAwaitingActivation
			if err := s.R.Users.Update(ctx, &u); err != nil {
				return err
			}
		}
		if err := s.R.Sessions.DeleteByUser(ctx, userID); err != nil {
			return err
		}
		return s.audit(ctx, actor, ActChannelUnlinked, UserTarget(userID), map[string]any{"type": chans[idx].Type})
	})
}

func (s *UserService) saveResidency(ctx context.Context, r *Residency, create bool) error {
	v := validation{}
	if err := s.checkResidency(ctx, v, r.HouseID, r.Apartment, r.StatusID); err != nil {
		return err
	}
	if err := v.err(); err != nil {
		return err
	}
	var err error
	if create {
		err = s.R.Residencies.Create(ctx, r)
	} else {
		err = s.R.Residencies.Update(ctx, r)
	}
	if errors.Is(err, ErrConflict) {
		return &ValidationError{Fields: map[string]string{"apartment": "taken"}}
	}
	return err
}

// AddResidency registers a user in another apartment.
func (s *UserService) AddResidency(ctx context.Context, userID int64, r Residency, actor Actor) (Residency, error) {
	r.UserID, r.CreatedAt = userID, s.Now()
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if _, err := s.R.Users.Get(ctx, userID); err != nil {
			return err
		}
		if err := s.saveResidency(ctx, &r, true); err != nil {
			return err
		}
		return s.audit(ctx, actor, ActUserUpdated, UserTarget(userID), map[string]any{"residencyAdded": r.ID})
	})
	return r, err
}

// UpdateResidency changes a residency's house, apartment or status.
func (s *UserService) UpdateResidency(ctx context.Context, r Residency, actor Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		old, err := s.R.Residencies.Get(ctx, r.ID)
		if err != nil {
			return err
		}
		r.UserID = old.UserID
		if err := s.saveResidency(ctx, &r, false); err != nil {
			return err
		}
		return s.audit(ctx, actor, ActUserUpdated, UserTarget(r.UserID), map[string]any{"residencyUpdated": r.ID})
	})
}

// RemoveResidency deletes a residency.
func (s *UserService) RemoveResidency(ctx context.Context, id int64, actor Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		r, err := s.R.Residencies.Get(ctx, id)
		if err != nil {
			return err
		}
		if err := s.R.Residencies.Delete(ctx, id); err != nil {
			return err
		}
		return s.audit(ctx, actor, ActUserUpdated, UserTarget(r.UserID), map[string]any{"residencyRemoved": id})
	})
}

// ResidencyOwner returns the user id a residency belongs to.
func (s *UserService) ResidencyOwner(ctx context.Context, id int64) (int64, error) {
	r, err := s.R.Residencies.Get(ctx, id)
	return r.UserID, err
}

// Card returns a user with channels, residencies and the latest audit entries.
func (s *UserService) Card(ctx context.Context, id int64) (UserCard, error) {
	var c UserCard
	var err error
	if c.User, err = s.R.Users.Get(ctx, id); err != nil {
		return c, err
	}
	if c.Channels, err = s.R.Channels.ListByUser(ctx, id); err != nil {
		return c, err
	}
	if c.Residencies, err = s.R.Residencies.ListByUser(ctx, id); err != nil {
		return c, err
	}
	c.Audit, _, err = s.R.Audit.List(ctx, AuditFilter{TargetUserID: &id, Limit: cardAuditSize})
	return c, err
}

// List returns users matching f.
func (s *UserService) List(ctx context.Context, f UserFilter) ([]User, int, error) {
	return s.R.Users.List(ctx, f)
}

// CountByStatus returns how many users have status st.
func (s *UserService) CountByStatus(ctx context.Context, st UserStatus) (int, error) {
	return s.R.Users.CountByStatus(ctx, st)
}

// AuditLog lists audit entries, newest first.
func (s *UserService) AuditLog(ctx context.Context, f AuditFilter) ([]AuditEntry, int, error) {
	return s.R.Audit.List(ctx, f)
}
