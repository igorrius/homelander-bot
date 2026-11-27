package domain_test

import (
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func TestAuditViaMCP(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	a := domain.Actor{UserID: &adm.ID, Via: "mcp", Client: "Claude"}
	if _, err := e.data.Create(ctx, textValue("house.address"), a); err != nil {
		t.Fatal(err)
	}
	v, err := e.pages.CreateDraft(ctx, []domain.PageFileInput{index()}, "from agent", a)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := e.r.Pages.Get(ctx, v.ID); got.CreatedVia != "Claude" || v.CreatedVia != "Claude" {
		t.Errorf("CreatedVia = %q / %q", got.CreatedVia, v.CreatedVia)
	}
	entries, _, err := e.r.Audit.List(ctx, domain.AuditFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, en := range entries {
		if en.Action == domain.ActDataCreated || en.Action == domain.ActPageDraftCreated {
			if en.Details["via"] != "mcp" || en.Details["client"] != "Claude" || *en.ActorUserID != adm.ID {
				t.Errorf("%s details = %v", en.Action, en.Details)
			}
			seen[en.Action] = true
		}
	}
	if len(seen) != 2 {
		t.Errorf("audit entries seen = %v", seen)
	}
	// A panel action carries no via.
	if _, err := e.data.Create(ctx, textValue("house.other"), actor(adm)); err != nil {
		t.Fatal(err)
	}
	entries, _, _ = e.r.Audit.List(ctx, domain.AuditFilter{Limit: 1})
	if _, ok := entries[0].Details["via"]; ok {
		t.Errorf("panel action has via: %v", entries[0].Details)
	}
	copied, err := e.pages.CopyToDraft(ctx, v.ID, actor(adm))
	if err != nil || copied.CreatedVia != "" {
		t.Errorf("panel copy CreatedVia = %q, %v", copied.CreatedVia, err)
	}
}
