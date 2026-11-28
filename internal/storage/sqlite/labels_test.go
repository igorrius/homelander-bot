package sqlite_test

import (
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func (r chatRepos) label(t *testing.T, name string) domain.Label {
	t.Helper()
	l := domain.Label{Name: name, CreatedAt: t0, UpdatedAt: t0}
	if err := r.labels.Create(ctx, &l); err != nil {
		t.Fatal(err)
	}
	return l
}

func TestLabelCRUD(t *testing.T) {
	r := newChatRepos(t)
	l := domain.Label{Name: "Журнал", Description: "admins", CreatedAt: t0, UpdatedAt: t0}
	if err := r.labels.Create(ctx, &l); err != nil {
		t.Fatal(err)
	}
	got, err := r.labels.Get(ctx, l.ID)
	if err != nil || !reflect.DeepEqual(got, l) {
		t.Fatalf("Get = %+v, %v; want %+v", got, err, l)
	}
	l.Name, l.Description, l.UpdatedAt = "Журнал адмінів", "", t0.Add(time.Hour)
	if err := r.labels.Update(ctx, &l); err != nil {
		t.Fatal(err)
	}
	r.label(t, "announcements")
	list, err := r.labels.List(ctx)
	if err != nil || len(list) != 2 || list[0].Name != "announcements" || list[1].Name != "Журнал адмінів" {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if err := r.labels.Delete(ctx, l.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.labels.Get(ctx, l.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after Delete: %v", err)
	}
}

func TestLabelNameUniqueCaseFolded(t *testing.T) {
	r := newChatRepos(t)
	r.label(t, "Журнал")
	if err := r.labels.Create(ctx, &domain.Label{Name: "журнал", CreatedAt: t0, UpdatedAt: t0}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("case-folded duplicate on Create: %v", err)
	}
	other := r.label(t, "other")
	other.Name = "ЖУРНАЛ"
	if err := r.labels.Update(ctx, &other); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("case-folded duplicate on Update: %v", err)
	}
}

func ids(ts []domain.LabelTarget) []domain.LabelTarget {
	sort.Slice(ts, func(i, j int) bool {
		if ts[i].Type != ts[j].Type {
			return ts[i].Type < ts[j].Type
		}
		return ts[i].ID < ts[j].ID
	})
	return ts
}

func TestLabelBindings(t *testing.T) {
	r := newChatRepos(t)
	a, b := r.label(t, "a"), r.label(t, "b")
	chat := domain.LabelTarget{Type: domain.TargetChat, ID: 1}
	topic := domain.LabelTarget{Type: domain.TargetTopic, ID: 1}
	if err := r.labels.SetLabelsOf(ctx, chat, []int64{a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	if err := r.labels.SetLabelsOf(ctx, chat, []int64{b.ID}); err != nil {
		t.Fatal(err)
	}
	if got, err := r.labels.LabelsOf(ctx, chat); err != nil || !reflect.DeepEqual(got, []int64{b.ID}) {
		t.Fatalf("SetLabelsOf must replace: %v, %v", got, err)
	}
	if err := r.labels.SetLabelsOf(ctx, topic, []int64{b.ID}); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.labels.Targets(ctx, b.ID); !reflect.DeepEqual(ids(got), []domain.LabelTarget{chat, topic}) {
		t.Fatalf("Targets = %v", got)
	}

	moved := domain.LabelTarget{Type: domain.TargetChat, ID: 2}
	if err := r.labels.SetLabelsOf(ctx, moved, []int64{a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	if err := r.labels.Retarget(ctx, chat, moved); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.labels.LabelsOf(ctx, moved); !reflect.DeepEqual(got, []int64{a.ID, b.ID}) {
		t.Fatalf("after Retarget target has %v", got)
	}
	if got, _ := r.labels.LabelsOf(ctx, chat); len(got) != 0 {
		t.Fatalf("after Retarget source still has %v", got)
	}
	if err := r.labels.DeleteTarget(ctx, moved); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.labels.LabelsOf(ctx, moved); len(got) != 0 {
		t.Fatalf("after DeleteTarget: %v", got)
	}
}

func TestLabelDeleteCascadesBindings(t *testing.T) {
	r := newChatRepos(t)
	l := r.label(t, "a")
	chat := domain.LabelTarget{Type: domain.TargetChat, ID: 1}
	if err := r.labels.SetLabelsOf(ctx, chat, []int64{l.ID}); err != nil {
		t.Fatal(err)
	}
	if err := r.labels.Delete(ctx, l.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.labels.LabelsOf(ctx, chat); len(got) != 0 {
		t.Fatalf("bindings left after label delete: %v", got)
	}
}

func TestLabelBindingsOnUser(t *testing.T) {
	r := newChatRepos(t)
	a, b := r.label(t, "a"), r.label(t, "b")
	user := domain.LabelTarget{Type: domain.TargetUser, ID: 7}
	if err := r.labels.SetLabelsOf(ctx, user, []int64{a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	if got, err := r.labels.LabelsOf(ctx, user); err != nil || !reflect.DeepEqual(got, []int64{a.ID, b.ID}) {
		t.Fatalf("LabelsOf = %v, %v", got, err)
	}
	if got, _ := r.labels.Targets(ctx, a.ID); !reflect.DeepEqual(ids(got), []domain.LabelTarget{user}) {
		t.Fatalf("Targets = %v", got)
	}
	if err := r.labels.Delete(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.labels.LabelsOf(ctx, user); !reflect.DeepEqual(got, []int64{b.ID}) {
		t.Fatalf("after label delete user has %v", got)
	}
}
