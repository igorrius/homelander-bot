package domain

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxLabelName = 40
	maxLabelDesc = 200
)

// NotificationsTarget is the audit target of the notification settings.
const NotificationsTarget = "notifications"

// LabelAuditTarget is the audit target of a label.
func LabelAuditTarget(id int64) string { return "label:" + strconv.FormatInt(id, 10) }

func (t LabelTarget) auditTarget() string {
	switch t.Type {
	case TargetTopic:
		return TopicTarget(t.ID)
	case TargetUser:
		return UserTarget(t.ID)
	}
	return ChatTarget(t.ID)
}

// LabelService manages labels, where they hang, and the notification settings
// that point at them.
type LabelService struct {
	R   Repos
	Now Clock
}

// LabelView is a label with its targets.
type LabelView struct {
	Label
	Targets            []LabelTarget
	Users              []LabelUser // one per user target, in Targets order
	UsedByApplications bool
}

// LabelUser is a user a label hangs on; Reachable when messages sent to the
// label would reach them now (active, with a linked messenger account).
type LabelUser struct {
	ID        int64
	Reachable bool
}

func (s *LabelService) audit(ctx context.Context, a Actor, action, target string, details map[string]any) error {
	return s.R.Audit.Add(ctx, AuditEntry{At: s.Now(), ActorUserID: a.UserID, Action: action, Target: target, Details: details, IP: a.IP})
}

func checkLabel(name, description string) (string, string, error) {
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	v := validation{}
	switch {
	case name == "":
		v.add("name", "required")
	case utf8.RuneCountInString(name) > maxLabelName:
		v.add("name", "too_long")
	}
	if utf8.RuneCountInString(description) > maxLabelDesc {
		v.add("description", "too_long")
	}
	return name, description, v.err()
}

func taken(err error) error {
	if errors.Is(err, ErrConflict) {
		return &ValidationError{Fields: map[string]string{"name": "taken"}}
	}
	return err
}

// List returns every label with where it hangs.
func (s *LabelService) List(ctx context.Context) ([]LabelView, error) {
	labels, err := s.R.Labels.List(ctx)
	if err != nil {
		return nil, err
	}
	n, err := s.R.Notifications.Get(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LabelView, 0, len(labels))
	for _, l := range labels {
		v := LabelView{Label: l, UsedByApplications: n.ApplicationsLabelID != nil && *n.ApplicationsLabelID == l.ID}
		if v.Targets, err = s.R.Labels.Targets(ctx, l.ID); err != nil {
			return nil, err
		}
		if v.Targets == nil {
			v.Targets = []LabelTarget{}
		}
		v.Users = []LabelUser{}
		for _, t := range v.Targets {
			if t.Type != TargetUser {
				continue
			}
			_, ok, err := userChannel(ctx, s.R, t.ID)
			if err != nil {
				return nil, err
			}
			v.Users = append(v.Users, LabelUser{ID: t.ID, Reachable: ok})
		}
		out = append(out, v)
	}
	return out, nil
}

// Create adds a label.
func (s *LabelService) Create(ctx context.Context, name, description string, a Actor) (Label, error) {
	name, description, err := checkLabel(name, description)
	if err != nil {
		return Label{}, err
	}
	now := s.Now()
	l := Label{Name: name, Description: description, CreatedAt: now, UpdatedAt: now}
	err = s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.R.Labels.Create(ctx, &l); err != nil {
			return taken(err)
		}
		return s.audit(ctx, a, ActLabelCreated, LabelAuditTarget(l.ID), map[string]any{"name": name})
	})
	return l, err
}

// Update renames or re-describes a label.
func (s *LabelService) Update(ctx context.Context, id int64, name, description string, a Actor) (Label, error) {
	name, description, err := checkLabel(name, description)
	if err != nil {
		return Label{}, err
	}
	var l Label
	err = s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if l, err = s.R.Labels.Get(ctx, id); err != nil {
			return err
		}
		l.Name, l.Description, l.UpdatedAt = name, description, s.Now()
		if err := s.R.Labels.Update(ctx, &l); err != nil {
			return taken(err)
		}
		return s.audit(ctx, a, ActLabelUpdated, LabelAuditTarget(id), map[string]any{"name": name})
	})
	return l, err
}

// Delete removes a label everywhere and clears settings that used it. Rows
// already queued for it keep their destinations.
func (s *LabelService) Delete(ctx context.Context, id int64, a Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		l, err := s.R.Labels.Get(ctx, id)
		if err != nil {
			return err
		}
		n, err := s.R.Notifications.Get(ctx)
		if err != nil {
			return err
		}
		if n.ApplicationsLabelID != nil && *n.ApplicationsLabelID == id {
			n.ApplicationsLabelID = nil
			if err := s.R.Notifications.Save(ctx, n); err != nil {
				return err
			}
			if err := s.audit(ctx, a, ActNotificationsSaved, NotificationsTarget, map[string]any{"applicationsLabelId": nil}); err != nil {
				return err
			}
		}
		if err := s.R.Labels.Delete(ctx, id); err != nil {
			return err
		}
		return s.audit(ctx, a, ActLabelDeleted, LabelAuditTarget(id), map[string]any{"name": l.Name})
	})
}

func (s *LabelService) targetExists(ctx context.Context, t LabelTarget) error {
	var err error
	switch t.Type {
	case TargetChat:
		_, err = s.R.Chats.Get(ctx, t.ID)
	case TargetTopic:
		_, err = s.R.Topics.Get(ctx, t.ID)
	case TargetUser:
		_, err = s.R.Users.Get(ctx, t.ID)
	default:
		err = ErrNotFound
	}
	return err
}

// SetLabels replaces the labels on a chat, topic or user.
func (s *LabelService) SetLabels(ctx context.Context, t LabelTarget, labelIDs []int64, a Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.targetExists(ctx, t); err != nil {
			return err
		}
		want := slices.Clone(labelIDs)
		slices.Sort(want)
		want = slices.Compact(want)
		for _, id := range want {
			if _, err := s.R.Labels.Get(ctx, id); errors.Is(err, ErrNotFound) {
				return &ValidationError{Fields: map[string]string{"labels": "invalid"}}
			} else if err != nil {
				return err
			}
		}
		had, err := s.R.Labels.LabelsOf(ctx, t)
		if err != nil {
			return err
		}
		if err := s.R.Labels.SetLabelsOf(ctx, t, want); err != nil {
			return err
		}
		for _, id := range want {
			if !slices.Contains(had, id) {
				if err := s.audit(ctx, a, ActLabelBound, t.auditTarget(), map[string]any{"labelId": id}); err != nil {
					return err
				}
			}
		}
		for _, id := range had {
			if !slices.Contains(want, id) {
				if err := s.audit(ctx, a, ActLabelUnbound, t.auditTarget(), map[string]any{"labelId": id}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// Notifications returns where the core's own notices go.
func (s *LabelService) Notifications(ctx context.Context) (NotificationSettings, error) {
	return s.R.Notifications.Get(ctx)
}

// SaveNotifications changes where the core's own notices go.
func (s *LabelService) SaveNotifications(ctx context.Context, n NotificationSettings, a Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if n.ApplicationsLabelID != nil {
			if _, err := s.R.Labels.Get(ctx, *n.ApplicationsLabelID); errors.Is(err, ErrNotFound) {
				return &ValidationError{Fields: map[string]string{"applicationsLabelId": "invalid"}}
			} else if err != nil {
				return err
			}
		}
		if err := s.R.Notifications.Save(ctx, n); err != nil {
			return err
		}
		return s.audit(ctx, a, ActNotificationsSaved, NotificationsTarget, map[string]any{"applicationsLabelId": n.ApplicationsLabelID})
	})
}
