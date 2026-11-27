package domain_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func (c *chatEnv) labels() *domain.LabelService {
	return &domain.LabelService{R: c.r, Now: c.clock.Now}
}

func fieldErr(t *testing.T, err error, field, reason string) {
	t.Helper()
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Fields[field] != reason {
		t.Fatalf("err = %v; want %s=%s", err, field, reason)
	}
}

func TestLabelCreateValidation(t *testing.T) {
	c := newChatEnv(t)
	ls := c.labels()
	a := actor(c.admin)
	_, err := ls.Create(ctx, "  ", "", a)
	fieldErr(t, err, "name", "required")
	_, err = ls.Create(ctx, strings.Repeat("я", 41), "", a)
	fieldErr(t, err, "name", "too_long")
	_, err = ls.Create(ctx, "ok", strings.Repeat("x", 201), a)
	fieldErr(t, err, "description", "too_long")
	l, err := ls.Create(ctx, "  Журнал ", "admins", a)
	if err != nil || l.Name != "Журнал" || l.ID == 0 {
		t.Fatalf("Create = %+v, %v", l, err)
	}
	_, err = ls.Create(ctx, "журнал", "", a)
	fieldErr(t, err, "name", "taken")
	if c.auditCount("label.created") != 1 {
		t.Fatal("label.created not audited")
	}
}

func TestLabelRename(t *testing.T) {
	c := newChatEnv(t)
	ls := c.labels()
	a := actor(c.admin)
	l, _ := ls.Create(ctx, "journal", "", a)
	other, _ := ls.Create(ctx, "residents", "", a)
	got, err := ls.Update(ctx, l.ID, "Journal", "admins only", a)
	if err != nil || got.Name != "Journal" || got.Description != "admins only" {
		t.Fatalf("Update = %+v, %v", got, err)
	}
	_, err = ls.Update(ctx, other.ID, "JOURNAL", "", a)
	fieldErr(t, err, "name", "taken")
	if _, err := ls.Update(ctx, 999, "x", "", a); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown label: %v", err)
	}
}

func TestSetLabelsOnChatAndTopic(t *testing.T) {
	c := newChatEnv(t)
	ls := c.labels()
	a := actor(c.admin)
	c.must(c.chats.BotAdded(ctx, forum(superGroup), adminTG, member))
	c.must(c.chats.TopicSeen(ctx, forum(superGroup), "9", "News", true))
	ch := c.chat(superGroup)
	tp, _ := c.r.Topics.GetByExternal(ctx, ch.ID, "9")
	j, _ := ls.Create(ctx, "journal", "", a)
	r, _ := ls.Create(ctx, "residents", "", a)
	chatT := domain.LabelTarget{Type: domain.TargetChat, ID: ch.ID}
	c.must(ls.SetLabels(ctx, chatT, []int64{j.ID, r.ID}, a))
	c.must(ls.SetLabels(ctx, chatT, []int64{r.ID}, a))
	c.must(ls.SetLabels(ctx, domain.LabelTarget{Type: domain.TargetTopic, ID: tp.ID}, []int64{j.ID}, a))
	if got, _ := c.r.Labels.LabelsOf(ctx, chatT); !reflect.DeepEqual(got, []int64{r.ID}) {
		t.Fatalf("chat labels = %v", got)
	}
	if c.auditCount("label.bound") != 3 || c.auditCount("label.unbound") != 1 {
		t.Fatalf("bound %d, unbound %d", c.auditCount("label.bound"), c.auditCount("label.unbound"))
	}
	err := ls.SetLabels(ctx, chatT, []int64{999}, a)
	fieldErr(t, err, "labels", "invalid")
	views, err := ls.List(ctx)
	if err != nil || len(views) != 2 || len(views[0].Targets) != 1 || views[0].Targets[0].Type != domain.TargetTopic {
		t.Fatalf("List = %+v, %v", views, err)
	}
}

func TestSetLabelsUnknownTarget(t *testing.T) {
	c := newChatEnv(t)
	ls := c.labels()
	for _, tg := range []domain.LabelTarget{{Type: domain.TargetChat, ID: 42}, {Type: domain.TargetTopic, ID: 42}} {
		if err := ls.SetLabels(ctx, tg, nil, actor(c.admin)); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("%v: %v", tg, err)
		}
	}
}

func TestDeleteLabelClearsSettingKeepsQueue(t *testing.T) {
	c := newChatEnv(t)
	ls := c.labels()
	a := actor(c.admin)
	l, _ := ls.Create(ctx, "journal", "", a)
	c.must(ls.SaveNotifications(ctx, domain.NotificationSettings{ApplicationsLabelID: &l.ID}, a))
	views, _ := ls.List(ctx)
	if !views[0].UsedByApplications {
		t.Fatal("UsedByApplications not reported")
	}
	it := domain.OutboxItem{Messenger: "telegram", To: domain.Destination{ChatID: oldGroup}, Msg: domain.OutMessage{Text: "x"},
		LabelID: &l.ID, State: domain.OutboxPending, NextAttemptAt: c.clock.t, CreatedAt: c.clock.t}
	c.must(c.r.Outbox.Add(ctx, &it))
	c.must(ls.Delete(ctx, l.ID, a))
	if n, _ := ls.Notifications(ctx); n.ApplicationsLabelID != nil {
		t.Fatalf("setting still points at the deleted label: %+v", n)
	}
	got, err := c.r.Outbox.Get(ctx, it.ID)
	if err != nil || got.State != domain.OutboxPending || got.To.ChatID != oldGroup {
		t.Fatalf("queued row = %+v, %v", got, err)
	}
	if c.auditCount("label.deleted") != 1 || c.auditCount("notifications.updated") != 2 {
		t.Fatal("audit entries missing")
	}
}

func TestSaveNotificationsValidates(t *testing.T) {
	c := newChatEnv(t)
	ls := c.labels()
	bad := int64(999)
	err := ls.SaveNotifications(ctx, domain.NotificationSettings{ApplicationsLabelID: &bad}, actor(c.admin))
	fieldErr(t, err, "applicationsLabelId", "invalid")
	c.must(ls.SaveNotifications(ctx, domain.NotificationSettings{}, actor(c.admin)))
}
