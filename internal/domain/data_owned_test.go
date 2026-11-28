package domain_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

var ownedCols = []domain.DataColumn{{Name: "text", Label: "Text", Type: domain.DataText}, {Name: "image", Label: "Image", Type: domain.DataImage}}

// ownedEnv gives the data service a feed module and an owned table
// "feed.posts" with one row whose image is pic.
func ownedEnv(t *testing.T) (e *env, m *domain.ModuleService, pic int64) {
	e = newEnv(t)
	m = &domain.ModuleService{R: e.r, Now: e.clock.Now, Defs: []domain.ModuleDef{{Code: "feed", DefaultEnabled: true}}}
	e.data.Modules = m
	if _, err := e.data.EnsureOwnedTable(ctx, "module:feed", "feed.posts", "Posts", ownedCols, domain.VisResidents); err != nil {
		t.Fatal(err)
	}
	pic = e.asset("image/png")
	if err := e.data.ReplaceOwnedRows(ctx, "module:feed", "feed.posts",
		[]domain.DataRow{{Cells: cells("text", `"hi"`, "image", strconv.FormatInt(pic, 10))}}); err != nil {
		t.Fatal(err)
	}
	return e, m, pic
}

func TestDataReservedKeys(t *testing.T) {
	e, _, _ := ownedEnv(t)
	a := e.dataAdmin()
	var ve *domain.ValidationError
	if _, err := e.data.Create(ctx, textValue("feed.anything"), a); !errors.As(err, &ve) || ve.Fields["key"] != "key_reserved" {
		t.Fatalf("reserved create = %v", err)
	}
	if _, err := e.data.Create(ctx, textValue("feedback.x"), a); err != nil {
		t.Errorf("feedback.x: %v", err)
	}
}

func TestDataOwnedItemIsReadOnly(t *testing.T) {
	e, _, _ := ownedEnv(t)
	a := e.dataAdmin()
	v, err := e.data.Get(ctx, "feed.posts")
	if err != nil || v.Item.Owner != "module:feed" || v.Item.Visibility != domain.VisResidents || len(v.Rows) != 1 {
		t.Fatalf("Get = %+v, %v", v, err)
	}
	if e.auditCount(domain.ActDataCreated) != 1 {
		t.Errorf("data.created audits = %d", e.auditCount(domain.ActDataCreated))
	}
	cols := []domain.DataColumn{}
	for name, in := range map[string]domain.DataInput{
		"description": {Description: ptr("x")},
		"columns":     {Columns: &cols},
		"value":       {Value: raw(`1`)},
	} {
		if _, err := e.data.Patch(ctx, "feed.posts", in, a); !errors.Is(err, domain.ErrOwned) {
			t.Errorf("patch %s = %v", name, err)
		}
	}
	if _, err := e.data.UpsertRows(ctx, "feed.posts", []domain.DataRow{{Cells: cells("text", `"x"`)}}, a); !errors.Is(err, domain.ErrOwned) {
		t.Errorf("UpsertRows = %v", err)
	}
	if _, err := e.data.PatchRows(ctx, "feed.posts", []domain.RowPatch{{Cells: cells("text", `"x"`)}}, a); !errors.Is(err, domain.ErrOwned) {
		t.Errorf("PatchRows = %v", err)
	}
	if err := e.data.DeleteRows(ctx, "feed.posts", []int64{v.Rows[0].ID}, a); !errors.Is(err, domain.ErrOwned) {
		t.Errorf("DeleteRows = %v", err)
	}
	if err := e.data.Delete(ctx, "feed.posts", a); !errors.Is(err, domain.ErrOwned) {
		t.Errorf("Delete = %v", err)
	}
	it, err := e.data.Patch(ctx, "feed.posts", domain.DataInput{Visibility: ptr(domain.VisPublic)}, a)
	if err != nil || it.Visibility != domain.VisPublic || e.auditCount(domain.ActDataVisibilitySet) != 1 {
		t.Fatalf("visibility patch = %+v, %v", it, err)
	}
	// Ensuring again keeps the admin's visibility and the rows.
	if it, err = e.data.EnsureOwnedTable(ctx, "module:feed", "feed.posts", "Posts", ownedCols, domain.VisResidents); err != nil || it.Visibility != domain.VisPublic {
		t.Fatalf("re-ensure = %+v, %v", it, err)
	}
	if v, _ := e.data.Get(ctx, "feed.posts"); len(v.Rows) != 1 {
		t.Errorf("rows after re-ensure = %d", len(v.Rows))
	}
	if err := e.data.ReplaceOwnedRows(ctx, "module:other", "feed.posts", nil); !errors.Is(err, domain.ErrOwned) {
		t.Errorf("other owner = %v", err)
	}
}

func TestDataOfDisabledModuleIsNotServed(t *testing.T) {
	e, m, pic := ownedEnv(t)
	a := e.dataAdmin()
	if _, err := e.data.Patch(ctx, "feed.posts", domain.DataInput{Visibility: ptr(domain.VisPublic)}, a); err != nil {
		t.Fatal(err)
	}
	resident := e.mkUser("+380500000002")
	keys := func(vs []domain.DataView, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		s := ""
		for _, v := range vs {
			s += v.Item.Key + " "
		}
		return s
	}
	if got := keys(e.data.Public(ctx, "10.0.0.1")); got != "feed.posts " {
		t.Errorf("on: public = %q", got)
	}
	if pub, blocked, err := e.data.AssetAccess(ctx, pic); err != nil || !pub || blocked {
		t.Errorf("on: asset = %v %v %v", pub, blocked, err)
	}
	if _, err := m.SetEnabled(ctx, "feed", false, a); err != nil {
		t.Fatal(err)
	}
	if got := keys(e.data.Public(ctx, "10.0.0.1")); got != "" {
		t.Errorf("off: public = %q", got)
	}
	if got := keys(e.data.ForUser(ctx, resident)); got != "" {
		t.Errorf("off: for user = %q", got)
	}
	if _, err := e.data.PublicOne(ctx, "feed.posts", "10.0.0.1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("off: public one = %v", err)
	}
	if pub, blocked, err := e.data.AssetAccess(ctx, pic); err != nil || pub || !blocked {
		t.Errorf("off: asset = %v %v %v", pub, blocked, err)
	}
	// Admins still see it.
	if list, _ := e.data.List(ctx); len(list) != 1 {
		t.Errorf("admin list = %d items", len(list))
	}
	// An asset also used by an admin's residents item is not blocked.
	shared := domain.DataInput{Key: "logo", Kind: domain.DataValue, ValueType: ptr(domain.DataImage), Value: raw(strconv.FormatInt(pic, 10)),
		Visibility: ptr(domain.VisResidents)}
	if _, err := e.data.Create(ctx, shared, a); err != nil {
		t.Fatal(err)
	}
	if pub, blocked, _ := e.data.AssetAccess(ctx, pic); pub || blocked {
		t.Errorf("shared asset = %v %v", pub, blocked)
	}
	if pub, blocked, _ := e.data.AssetAccess(ctx, e.asset("image/png")); pub || blocked {
		t.Errorf("unreferenced asset = %v %v", pub, blocked)
	}
}
