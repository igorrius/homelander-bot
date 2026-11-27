package domain_test

import (
	"errors"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func TestFirstAdminProtected(t *testing.T) {
	e := newEnv(t)
	first := e.mkUser("+380500000099", admin, func(u *domain.User) { u.Source = domain.SourceBootstrap })
	second := e.mkUser("+380500000098", admin)
	if err := e.users.Disable(ctx, first.ID, actor(second)); !errors.Is(err, domain.ErrFirstAdmin) {
		t.Errorf("disable err = %v", err)
	}
	if _, err := e.users.Update(ctx, first.ID, domain.UserPatch{IsAdmin: ptr(false)}, actor(second)); !errors.Is(err, domain.ErrFirstAdmin) {
		t.Errorf("revoke err = %v", err)
	}
	if got := e.get(first.ID); got.Status != domain.StatusActive || !got.IsAdmin {
		t.Errorf("first admin changed: %+v", got)
	}
	if err := e.users.Disable(ctx, second.ID, actor(first)); err != nil {
		t.Errorf("disable other admin = %v", err)
	}
}
