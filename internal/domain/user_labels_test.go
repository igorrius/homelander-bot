package domain_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func userT(u domain.User) domain.LabelTarget {
	return domain.LabelTarget{Type: domain.TargetUser, ID: u.ID}
}

func withStatus(s domain.UserStatus) func(*domain.User) { return func(u *domain.User) { u.Status = s } }

func (c *chatEnv) auditOn(action, target string) int {
	var n int
	_ = c.db.QueryRow(`SELECT count(*) FROM audit_log WHERE action = ? AND target = ?`, action, target).Scan(&n)
	return n
}

func TestSetLabelsOnUser(t *testing.T) {
	c := newChatEnv(t)
	ls := c.labels()
	a := actor(c.admin)
	u := c.mkUser("+380500000010")
	x, _ := ls.Create(ctx, "testers", "", a)
	y, _ := ls.Create(ctx, "engineers", "", a)
	c.must(ls.SetLabels(ctx, userT(u), []int64{x.ID}, a))
	c.must(ls.SetLabels(ctx, userT(u), []int64{y.ID}, a))
	if got, _ := c.r.Labels.LabelsOf(ctx, userT(u)); !reflect.DeepEqual(got, []int64{y.ID}) {
		t.Fatalf("labels = %v", got)
	}
	target := domain.UserTarget(u.ID)
	if c.auditOn("label.bound", target) != 2 || c.auditOn("label.unbound", target) != 1 {
		t.Fatalf("bound %d, unbound %d on %s", c.auditOn("label.bound", target), c.auditOn("label.unbound", target), target)
	}
}

func TestSetLabelsUnknownUser(t *testing.T) {
	c := newChatEnv(t)
	ls := c.labels()
	l := c.mkLabel("testers")
	tg := domain.LabelTarget{Type: domain.TargetUser, ID: 9999}
	if err := ls.SetLabels(ctx, tg, []int64{l.ID}, actor(c.admin)); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if got, _ := c.r.Labels.LabelsOf(ctx, tg); len(got) != 0 {
		t.Fatalf("bound anyway: %v", got)
	}
}

func TestLabelListUsersReachable(t *testing.T) {
	c := newChatEnv(t)
	l := c.mkLabel("testers")
	linked := c.mkUser("+380500000011")
	c.link(linked, "111")
	unlinked := c.mkUser("+380500000012")
	disabled := c.mkUser("+380500000013", withStatus(domain.StatusDisabled))
	c.link(disabled, "113")
	c.bind(l, userT(linked), userT(unlinked), userT(disabled))
	views, err := c.labels().List(ctx)
	if err != nil || len(views) != 1 {
		t.Fatalf("List = %+v, %v", views, err)
	}
	want := []domain.LabelUser{{ID: linked.ID, Reachable: true}, {ID: unlinked.ID}, {ID: disabled.ID}}
	if !reflect.DeepEqual(views[0].Users, want) {
		t.Fatalf("Users = %+v; want %+v", views[0].Users, want)
	}
	empty := c.mkLabel("empty")
	views, _ = c.labels().List(ctx)
	for _, v := range views {
		if v.ID == empty.ID && (v.Users == nil || len(v.Users) != 0) {
			t.Fatalf("empty label Users = %#v; want []", v.Users)
		}
	}
}

func TestCardHasLabelIDs(t *testing.T) {
	c := newChatEnv(t)
	u := c.mkUser("+380500000014")
	card, err := c.users.Card(ctx, u.ID)
	if err != nil || card.LabelIDs == nil || len(card.LabelIDs) != 0 {
		t.Fatalf("no labels: %#v, %v", card.LabelIDs, err)
	}
	l := c.mkLabel("testers")
	c.bind(l, userT(u))
	if card, _ = c.users.Card(ctx, u.ID); !reflect.DeepEqual(card.LabelIDs, []int64{l.ID}) {
		t.Fatalf("LabelIDs = %v", card.LabelIDs)
	}
}

func TestSendToLabelReachesUsers(t *testing.T) {
	c := newChatEnv(t)
	ch, _ := c.acceptedForum(superGroup, "9")
	u := c.mkUser("+380500000015")
	c.link(u, "555")
	l := c.mkLabel("testers")
	c.bind(l, domain.LabelTarget{Type: domain.TargetChat, ID: ch.ID}, userT(u))
	if got, err := c.notifier().SendToLabel(ctx, l.ID, domain.OutMessage{Text: "hi"}, "test"); err != nil || got != 2 {
		t.Fatalf("SendToLabel = %d, %v", got, err)
	}
	rows := c.pending()
	if len(rows) != 2 || rows[0].UserID != nil {
		t.Fatalf("rows = %+v", rows)
	}
	r := rows[1]
	if r.To != (domain.Destination{ChatID: "555"}) || r.UserID == nil || *r.UserID != u.ID || r.LabelID == nil || *r.LabelID != l.ID || r.Messenger != "telegram" {
		t.Fatalf("user row = %+v", r)
	}
}

func TestSendToLabelSkipsUnreachableUsers(t *testing.T) {
	c := newChatEnv(t)
	l := c.mkLabel("testers")
	for i, s := range []domain.UserStatus{domain.StatusDisabled, domain.StatusRejected, domain.StatusAwaitingActivation, domain.StatusPendingApproval} {
		u := c.mkUser("+38050000002"+string(rune('0'+i)), withStatus(s))
		c.link(u, "60"+string(rune('0'+i)))
		c.bind(l, userT(u))
	}
	c.bind(l, userT(c.mkUser("+380500000029"))) // active, no channel
	if got, err := c.notifier().SendToLabel(ctx, l.ID, domain.OutMessage{Text: "hi"}, "test"); err != nil || got != 0 {
		t.Fatalf("SendToLabel = %d, %v", got, err)
	}
}

func TestSendToLabelUserDisabledAfterBinding(t *testing.T) {
	c := newChatEnv(t)
	l := c.mkLabel("testers")
	u := c.mkUser("+380500000030")
	c.link(u, "700")
	c.bind(l, userT(u))
	u.Status = domain.StatusDisabled
	c.must(c.r.Users.Update(ctx, &u))
	if got, err := c.notifier().SendToLabel(ctx, l.ID, domain.OutMessage{Text: "hi"}, "test"); err != nil || got != 0 {
		t.Fatalf("SendToLabel = %d, %v", got, err)
	}
}

func TestSendToLabelsUserOnce(t *testing.T) {
	c := newChatEnv(t)
	a, b := c.mkLabel("testers"), c.mkLabel("engineers")
	u := c.mkUser("+380500000031")
	c.link(u, "800")
	c.bind(a, userT(u))
	c.bind(b, userT(u))
	ids, err := c.notifier().SendToLabels(ctx, []int64{a.ID, b.ID}, domain.OutMessage{Text: "hi"}, "feed:1")
	if err != nil || len(ids) != 1 {
		t.Fatalf("SendToLabels = %v, %v", ids, err)
	}
	if rows := c.pending(); *rows[0].LabelID != a.ID {
		t.Fatalf("row label = %d; want %d", *rows[0].LabelID, a.ID)
	}
}

func TestSendToLabelUserAndTheirChatBoth(t *testing.T) {
	c := newChatEnv(t)
	ch, _ := c.acceptedForum(superGroup, "9")
	l := c.mkLabel("residents")
	// The admin added the bot to the chat, so they are in it too.
	c.bind(l, domain.LabelTarget{Type: domain.TargetChat, ID: ch.ID}, userT(c.admin))
	if got, err := c.notifier().SendToLabel(ctx, l.ID, domain.OutMessage{Text: "hi"}, "test"); err != nil || got != 2 {
		t.Fatalf("SendToLabel = %d, %v", got, err)
	}
}

func TestTestLabelUserOnly(t *testing.T) {
	c := newChatEnv(t)
	l := c.mkLabel("testers")
	c.bind(l, userT(c.admin))
	ids, err := c.notifier().TestLabel(ctx, l.ID)
	if err != nil || len(ids) != 1 {
		t.Fatalf("TestLabel = %v, %v", ids, err)
	}
}
