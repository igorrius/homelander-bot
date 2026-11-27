package sqlite_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite/sqlitetest"
)

type dataRepos struct {
	db   sqlite.DB
	data sqlite.Data
	site sqlite.Site
}

func newDataRepos(t *testing.T) dataRepos {
	db := sqlitetest.New(t)
	return dataRepos{db, sqlite.Data{DB: db}, sqlite.Site{DB: db}}
}

func (r dataRepos) item(t *testing.T, it domain.DataItem) domain.DataItem {
	t.Helper()
	if it.CreatedAt.IsZero() {
		it.CreatedAt, it.UpdatedAt = t0, t0
	}
	if err := r.data.Create(ctx, &it); err != nil {
		t.Fatal(err)
	}
	return it
}

func (r dataRepos) asset(t *testing.T) int64 {
	t.Helper()
	a := domain.Asset{Kind: "data", ContentType: "image/png", Data: []byte{1}, SHA256: "x", UploadedBy: 1, UploadedAt: t0}
	if err := r.site.PutAsset(ctx, &a); err != nil {
		t.Fatal(err)
	}
	return a.ID
}

func TestDataItemCRUD(t *testing.T) {
	r := newDataRepos(t)
	it := r.item(t, domain.DataItem{Key: "house.address", Kind: domain.DataValue, Description: "where",
		ValueType: domain.DataText, Value: json.RawMessage(`"Main st"`)})
	got, err := r.data.GetByKey(ctx, "house.address")
	if err != nil || !reflect.DeepEqual(got, it) {
		t.Fatalf("GetByKey = %+v, %v; want %+v", got, err, it)
	}
	dup := domain.DataItem{Key: "house.address", Kind: domain.DataValue, ValueType: domain.DataText,
		Value: json.RawMessage(`null`), CreatedAt: t0, UpdatedAt: t0}
	if err := r.data.Create(ctx, &dup); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate = %v; want ErrConflict", err)
	}
	it.Public, it.Description, it.UpdatedAt = true, "", t0.Add(time.Hour)
	if err := r.data.Update(ctx, &it); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.data.GetByKey(ctx, it.Key); !reflect.DeepEqual(got, it) {
		t.Fatalf("after update = %+v; want %+v", got, it)
	}
	if err := r.data.Delete(ctx, it.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.data.GetByKey(ctx, it.Key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after delete = %v; want ErrNotFound", err)
	}
}

func TestDataRows(t *testing.T) {
	r := newDataRepos(t)
	cols := []domain.DataColumn{{Name: "name", Label: "Name", Type: domain.DataText}}
	tbl := r.item(t, domain.DataItem{Key: "contacts", Kind: domain.DataTable, Columns: cols})
	other := r.item(t, domain.DataItem{Key: "other", Kind: domain.DataTable, Columns: cols})
	rows := []domain.DataRow{
		{Position: 1, Cells: map[string]json.RawMessage{"name": json.RawMessage(`"b"`)}},
		{Position: 0, Cells: map[string]json.RawMessage{"name": json.RawMessage(`"a"`)}},
	}
	if err := r.data.PutRows(ctx, tbl.ID, rows); err != nil {
		t.Fatal(err)
	}
	if rows[0].ID == 0 || rows[1].ID == 0 {
		t.Fatalf("ids not set: %+v", rows)
	}
	rows[0].Cells["name"] = json.RawMessage(`"bb"`)
	if err := r.data.PutRows(ctx, tbl.ID, rows[:1]); err != nil {
		t.Fatal(err)
	}
	got, err := r.data.Rows(ctx, tbl.ID)
	if err != nil || len(got) != 2 || got[0].ID != rows[1].ID || string(got[1].Cells["name"]) != `"bb"` {
		t.Fatalf("Rows = %+v, %v", got, err)
	}
	if err := r.data.PutRows(ctx, other.ID, rows[:1]); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign row = %v; want ErrNotFound", err)
	}
	if err := r.data.DeleteRows(ctx, tbl.ID, []int64{rows[0].ID}); err != nil {
		t.Fatal(err)
	}
	list, err := r.data.List(ctx, false)
	if err != nil || len(list) != 2 || list[0].Key != "contacts" || list[0].RowCount != 1 || !reflect.DeepEqual(list[0].Columns, cols) {
		t.Fatalf("List = %+v, %v", list, err)
	}
}

func TestDataListPublicOnly(t *testing.T) {
	r := newDataRepos(t)
	r.item(t, domain.DataItem{Key: "a", Kind: domain.DataValue, ValueType: domain.DataText, Value: json.RawMessage(`null`)})
	r.item(t, domain.DataItem{Key: "b", Kind: domain.DataValue, ValueType: domain.DataText, Value: json.RawMessage(`null`), Public: true})
	list, err := r.data.List(ctx, true)
	if err != nil || len(list) != 1 || list[0].Key != "b" {
		t.Fatalf("List(public) = %+v, %v", list, err)
	}
}

func TestAssetIsPublic(t *testing.T) {
	r := newDataRepos(t)
	ref, lone := r.asset(t), r.asset(t)
	it := r.item(t, domain.DataItem{Key: "logo", Kind: domain.DataValue, ValueType: domain.DataImage, Value: json.RawMessage(`null`)})
	if err := r.data.SetAssetRefs(ctx, it.ID, []int64{ref}); err != nil {
		t.Fatal(err)
	}
	check := func(id int64, want bool) {
		t.Helper()
		if got, err := r.data.AssetIsPublic(ctx, id); err != nil || got != want {
			t.Fatalf("AssetIsPublic(%d) = %v, %v; want %v", id, got, err, want)
		}
	}
	check(ref, false)
	check(lone, false)
	it.Public = true
	if err := r.data.Update(ctx, &it); err != nil {
		t.Fatal(err)
	}
	check(ref, true)
	if err := r.data.SetAssetRefs(ctx, it.ID, nil); err != nil {
		t.Fatal(err)
	}
	check(ref, false)
}

func TestOldSitePictureIgnored(t *testing.T) {
	r := newDataRepos(t)
	pic := r.asset(t)
	raw := fmt.Sprintf(`{"communityName":{"uk":"Дім"},"welcomeText":{},"defaultLocale":"uk","pictureAssetId":%d}`, pic)
	if _, err := r.db.Q(ctx).ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES ('site', ?, '2026-11-27T00:00:00Z')`, raw); err != nil {
		t.Fatal(err)
	}
	st, err := r.site.Get(ctx)
	if err != nil || st.CommunityName["uk"] != "Дім" || st.DefaultLocale != "uk" {
		t.Fatalf("Get = %+v, %v", st, err)
	}
	if got, err := r.data.AssetIsPublic(ctx, pic); err != nil || got {
		t.Fatalf("old site picture public = %v, %v", got, err)
	}
}
