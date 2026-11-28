package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func newModules(e *env, ensured *int) *domain.ModuleService {
	return &domain.ModuleService{R: e.r, Now: e.clock.Now, Defs: []domain.ModuleDef{{
		Code: "feed", DefaultEnabled: true,
		Ensure: func(context.Context) error { *ensured++; return nil },
	}}}
}

func TestModuleDefaultsAndToggle(t *testing.T) {
	e := newEnv(t)
	ensured := 0
	m := newModules(e, &ensured)
	adm := e.mkUser("+380500000001", admin)
	if on, err := m.Enabled(ctx, "feed"); err != nil || !on {
		t.Fatalf("default = %v, %v", on, err)
	}
	if on, _ := m.Enabled(ctx, "nope"); on {
		t.Error("unknown module is on")
	}
	if _, err := m.Get(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get unknown = %v", err)
	}
	v, err := m.SetEnabled(ctx, "feed", false, actor(adm))
	if err != nil || v.Enabled || v.UpdatedAt == nil || *v.UpdatedBy != adm.ID {
		t.Fatalf("SetEnabled(false) = %+v, %v", v, err)
	}
	if err := m.Require(ctx, "feed"); !errors.Is(err, domain.ErrModuleOff) {
		t.Errorf("Require off = %v", err)
	}
	if off, _ := m.DisabledOwners(ctx); !off["module:feed"] {
		t.Errorf("DisabledOwners = %v", off)
	}
	if ensured != 0 || e.auditCount(domain.ActModuleDisabled) != 1 {
		t.Errorf("ensured %d, disabled audits %d", ensured, e.auditCount(domain.ActModuleDisabled))
	}
	if _, err := m.SetEnabled(ctx, "feed", true, actor(adm)); err != nil {
		t.Fatal(err)
	}
	if ensured != 1 || e.auditCount(domain.ActModuleEnabled) != 1 || m.Require(ctx, "feed") != nil {
		t.Errorf("after on: ensured %d, audits %d", ensured, e.auditCount(domain.ActModuleEnabled))
	}
	var target string
	_ = e.db.QueryRow(`SELECT target FROM audit_log WHERE action = 'module.enabled'`).Scan(&target)
	if target != "module:feed" {
		t.Errorf("target = %q", target)
	}
	// Switching to the state it already has records nothing.
	if _, err := m.SetEnabled(ctx, "feed", true, actor(adm)); err != nil || e.auditCount(domain.ActModuleEnabled) != 1 {
		t.Errorf("repeat on: %v, audits %d", err, e.auditCount(domain.ActModuleEnabled))
	}
	if err := m.EnsureAll(ctx); err != nil || ensured != 3 {
		t.Errorf("EnsureAll: %v, ensured %d", err, ensured)
	}
}

func TestModuleReservedKeys(t *testing.T) {
	e := newEnv(t)
	n := 0
	m := newModules(e, &n)
	for key, want := range map[string]bool{"feed.posts": true, "feed.x.y": true, "feedback.x": false, "custom.feed.x": false, "feed": false} {
		if got := m.Reserved(key); got != want {
			t.Errorf("Reserved(%q) = %v", key, got)
		}
	}
	var none *domain.ModuleService
	if none.Reserved("feed.x") {
		t.Error("nil service reserves keys")
	}
}

func TestModuleSaveSettingsAudits(t *testing.T) {
	e := newEnv(t)
	n := 0
	m := newModules(e, &n)
	if err := m.SaveSettings(ctx, "feed", []byte(`{"defaultLabelIds":[2]}`), public); err != nil {
		t.Fatal(err)
	}
	v, _ := m.Get(ctx, "feed")
	if string(v.Settings) != `{"defaultLabelIds":[2]}` || !v.Enabled || e.auditCount(domain.ActModuleSettingsSaved) != 1 {
		t.Errorf("view = %+v", v)
	}
}
