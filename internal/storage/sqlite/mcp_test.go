package sqlite_test

import (
	"errors"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite/sqlitetest"
)

func TestMCPRepoRoundTrip(t *testing.T) {
	db := sqlitetest.New(t)
	users := sqlite.Users{DB: db}
	r := sqlite.MCP{DB: db}
	u := domain.User{Phone: "+380500000001", FirstName: "A", LastName: "B", Status: domain.StatusActive, Source: domain.SourceAdmin}
	if err := users.Create(ctx, &u); err != nil {
		t.Fatal(err)
	}
	c := domain.MCPClient{ID: "cl1", Name: "Claude", RedirectURIs: []string{"https://claude.ai/api/mcp/auth_callback"}, CreatedAt: t0}
	if err := r.CreateClient(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetClient(ctx, "cl1")
	if err != nil || got.Name != "Claude" || len(got.RedirectURIs) != 1 || got.RedirectURIs[0] != c.RedirectURIs[0] || !got.CreatedAt.Equal(t0) {
		t.Fatalf("GetClient = %+v, %v", got, err)
	}
	if _, err := r.GetClient(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing client = %v", err)
	}

	g := domain.MCPGrant{UserID: u.ID, ClientID: "cl1", CreatedAt: t0}
	if err := r.CreateGrant(ctx, &g); err != nil || g.ID == 0 {
		t.Fatalf("CreateGrant = %v", err)
	}
	if err := r.CreateGrant(ctx, &domain.MCPGrant{UserID: u.ID, ClientID: "cl1", CreatedAt: t0}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate grant = %v", err)
	}
	byPair, err := r.GrantFor(ctx, u.ID, "cl1")
	if err != nil || byPair.ID != g.ID || byPair.ClientName != "Claude" {
		t.Fatalf("GrantFor = %+v, %v", byPair, err)
	}
	t1 := t0.Add(time.Minute)
	if err := r.TouchGrant(ctx, g.ID, t1); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetGrant(ctx, g.ID); got.LastUsedAt == nil || !got.LastUsedAt.Equal(t1) {
		t.Fatalf("touched = %+v", got)
	}
	list, err := r.ListGrants(ctx, u.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListGrants = %v, %v", list, err)
	}

	code := domain.MCPCode{Hash: "h1", GrantID: g.ID, RedirectURI: "https://x/cb", Challenge: "ch", Resource: "res", ExpiresAt: t1}
	if err := r.CreateCode(ctx, code); err != nil {
		t.Fatal(err)
	}
	used, err := r.UseCode(ctx, "h1", t0)
	if err != nil || used.GrantID != g.ID || used.Challenge != "ch" || used.RedirectURI != "https://x/cb" || !used.ExpiresAt.Equal(t1) {
		t.Fatalf("UseCode = %+v, %v", used, err)
	}
	if _, err := r.UseCode(ctx, "h1", t0); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("second UseCode = %v", err)
	}

	tok := domain.MCPToken{Hash: "t1", GrantID: g.ID, Kind: domain.MCPAccess, ExpiresAt: t1}
	if err := r.CreateToken(ctx, tok); err != nil {
		t.Fatal(err)
	}
	gotTok, err := r.GetToken(ctx, "t1")
	if err != nil || gotTok.Kind != domain.MCPAccess || gotTok.RevokedAt != nil || !gotTok.ExpiresAt.Equal(t1) {
		t.Fatalf("GetToken = %+v, %v", gotTok, err)
	}
	if ok, err := r.RevokeToken(ctx, "t1", t0); err != nil || !ok {
		t.Fatalf("RevokeToken = %v %v", ok, err)
	}
	if ok, _ := r.RevokeToken(ctx, "t1", t0); ok {
		t.Fatal("revoked twice")
	}
	if err := r.CreateToken(ctx, domain.MCPToken{Hash: "t2", GrantID: g.ID, Kind: domain.MCPRefresh, ExpiresAt: t1}); err != nil {
		t.Fatal(err)
	}
	at := t0
	if err := r.SetGrantRevoked(ctx, g.ID, &at); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetToken(ctx, "t2"); got.RevokedAt == nil {
		t.Fatal("revoking a grant keeps its tokens")
	}
	if list, _ := r.ListGrants(ctx, u.ID); len(list) != 0 {
		t.Fatalf("revoked grant listed: %v", list)
	}
	if err := r.SetGrantRevoked(ctx, g.ID, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetGrant(ctx, g.ID); got.RevokedAt != nil {
		t.Fatal("grant not reactivated")
	}
}
