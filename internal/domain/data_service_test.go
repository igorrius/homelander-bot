package domain_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func (e *env) dataAdmin() domain.Actor {
	e.t.Helper()
	return actor(e.mkUser("+380500000099", admin))
}

func (e *env) asset(ct string) int64 {
	e.t.Helper()
	a := domain.Asset{Kind: "data", ContentType: ct, Data: []byte{1}, SHA256: "x", UploadedBy: 1, UploadedAt: e.clock.t}
	if err := e.r.Site.PutAsset(ctx, &a); err != nil {
		e.t.Fatal(err)
	}
	return a.ID
}

func textValue(key string) domain.DataInput {
	return domain.DataInput{Key: key, Kind: domain.DataValue, ValueType: ptr(domain.DataText), Value: raw(`"x"`)}
}

func contactsTable() domain.DataInput {
	return domain.DataInput{Key: "contacts", Kind: domain.DataTable, Columns: &[]domain.DataColumn{
		{Name: "name", Label: "Name", Type: domain.DataText}, {Name: "phone", Label: "Phone", Type: domain.DataPhone}}}
}

func cells(kv ...string) map[string]json.RawMessage {
	m := map[string]json.RawMessage{}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i]] = raw(kv[i+1])
	}
	return m
}

func TestDataKeyValidation(t *testing.T) {
	e := newEnv(t)
	a := e.dataAdmin()
	for _, k := range []string{"contacts", "house.address", "a1.b_2"} {
		if _, err := e.data.Create(ctx, textValue(k), a); err != nil {
			t.Fatalf("%q: %v", k, err)
		}
	}
	for _, k := range []string{"Contacts", "contacts.", ".x", "a..b", "контакти", "1a", "", strings.Repeat("a", 101)} {
		_, err := e.data.Create(ctx, textValue(k), a)
		fieldErr(t, err, "key", "invalid")
	}
	_, err := e.data.Create(ctx, textValue("contacts"), a)
	fieldErr(t, err, "key", "taken")
	_, err = e.data.Create(ctx, domain.DataInput{Key: "k", Kind: "list"}, a)
	fieldErr(t, err, "kind", "invalid")
	_, err = e.data.Create(ctx, domain.DataInput{Key: "k", Kind: domain.DataValue, ValueType: ptr(domain.DataPhone)}, a)
	fieldErr(t, err, "valueType", "invalid")
	long := strings.Repeat("я", 1001)
	in := textValue("k")
	in.Description = &long
	_, err = e.data.Create(ctx, in, a)
	fieldErr(t, err, "description", "too_long")
}

func TestDataValueTypes(t *testing.T) {
	e := newEnv(t)
	a := e.dataAdmin()
	img, doc := e.asset("image/png"), e.asset("application/pdf")
	cases := []struct {
		typ    domain.DataType
		ok     string
		bad    string
		reason string
	}{
		{domain.DataText, `"hello"`, `"` + strings.Repeat("я", 2001) + `"`, "too_long"},
		{domain.DataText, `null`, `5`, "invalid"},
		{domain.DataLocalizedText, `{"uk":"а","en":"a"}`, `{"de":"x"}`, "invalid"},
		{domain.DataNumber, `12.5`, `"12"`, "invalid"},
		{domain.DataBool, `true`, `"yes"`, "invalid"},
		{domain.DataImage, fmt.Sprint(img), fmt.Sprint(doc), "invalid"},
		{domain.DataImage, `null`, `999`, "invalid"},
		{domain.DataJSON, `{"a":[1,2]}`, `"` + strings.Repeat("a", 64<<10) + `"`, "too_long"},
	}
	for i, c := range cases {
		key := fmt.Sprintf("v%d", i)
		if _, err := e.data.Create(ctx, domain.DataInput{Key: key, Kind: domain.DataValue, ValueType: ptr(c.typ), Value: raw(c.ok)}, a); err != nil {
			t.Fatalf("%s %s: %v", c.typ, c.ok, err)
		}
		_, err := e.data.Patch(ctx, key, domain.DataInput{ValueType: ptr(c.typ), Value: raw(c.bad)}, a)
		fieldErr(t, err, "value", c.reason)
	}
}

func TestDataColumnsAndRows(t *testing.T) {
	e := newEnv(t)
	a := e.dataAdmin()
	if _, err := e.data.Create(ctx, contactsTable(), a); err != nil {
		t.Fatal(err)
	}
	rows, err := e.data.UpsertRows(ctx, "contacts", []domain.DataRow{
		{Position: 0, Cells: cells("name", `"Admin office"`, "phone", `"+380 44 000 00 00"`)},
		{Position: 1, Cells: cells("name", `"Guard"`)},
	}, a)
	if err != nil || len(rows) != 2 || rows[0].ID == 0 {
		t.Fatalf("UpsertRows = %+v, %v", rows, err)
	}
	_, err = e.data.UpsertRows(ctx, "contacts", []domain.DataRow{{Cells: cells("phone", `"call me"`)}}, a)
	fieldErr(t, err, "cells.phone", "invalid")
	_, err = e.data.UpsertRows(ctx, "contacts", []domain.DataRow{{Cells: cells("x", `"1"`)}}, a)
	fieldErr(t, err, "cells.x", "invalid")

	retyped := []domain.DataColumn{{Name: "name", Label: "Name", Type: domain.DataNumber}, {Name: "phone", Label: "Phone", Type: domain.DataPhone}}
	_, err = e.data.Patch(ctx, "contacts", domain.DataInput{Columns: &retyped}, a)
	fieldErr(t, err, "columns", "type_change_not_empty")
	dupe := []domain.DataColumn{{Name: "name", Type: domain.DataText}, {Name: "name", Type: domain.DataText}}
	_, err = e.data.Patch(ctx, "contacts", domain.DataInput{Columns: &dupe}, a)
	fieldErr(t, err, "columns", "invalid")

	onlyName := []domain.DataColumn{{Name: "name", Label: "Name", Type: domain.DataText}}
	if _, err := e.data.Patch(ctx, "contacts", domain.DataInput{Columns: &onlyName}, a); err != nil {
		t.Fatal(err)
	}
	v, err := e.data.Get(ctx, "contacts")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range v.Rows {
		if _, ok := r.Cells["phone"]; ok {
			t.Fatalf("phone cell kept after column removal: %+v", r)
		}
	}

	many := make([]domain.DataRow, 999)
	for i := range many {
		many[i] = domain.DataRow{Position: i + 2, Cells: cells("name", `"n"`)}
	}
	_, err = e.data.UpsertRows(ctx, "contacts", many, a)
	fieldErr(t, err, "rows", "too_many")
	if v, _ := e.data.Get(ctx, "contacts"); len(v.Rows) != 2 {
		t.Fatalf("rows after refused batch = %d; want 2", len(v.Rows))
	}
	if _, err := e.data.UpsertRows(ctx, "contacts", many[:998], a); err != nil {
		t.Fatalf("1000 rows refused: %v", err)
	}
	_, err = e.data.UpsertRows(ctx, "contacts", []domain.DataRow{{Cells: cells("name", `"one more"`)}}, a)
	fieldErr(t, err, "rows", "too_many")
}

func TestDataAudit(t *testing.T) {
	e := newEnv(t)
	a := e.dataAdmin()
	if _, err := e.data.Create(ctx, contactsTable(), a); err != nil {
		t.Fatal(err)
	}
	desc := "important phones"
	if _, err := e.data.Patch(ctx, "contacts", domain.DataInput{Description: &desc}, a); err != nil {
		t.Fatal(err)
	}
	if _, err := e.data.Patch(ctx, "contacts", domain.DataInput{Public: ptr(true)}, a); err != nil {
		t.Fatal(err)
	}
	rows, err := e.data.UpsertRows(ctx, "contacts", []domain.DataRow{{Cells: cells("name", `"a"`)}}, a)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.data.DeleteRows(ctx, "contacts", []int64{rows[0].ID}, a); err != nil {
		t.Fatal(err)
	}
	if err := e.data.Delete(ctx, "contacts", a); err != nil {
		t.Fatal(err)
	}
	list, _, err := e.r.Audit.List(ctx, domain.AuditFilter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for i := len(list) - 1; i >= 0; i-- {
		en := list[i]
		if !strings.HasPrefix(en.Action, "data.") {
			continue
		}
		if en.Target != "data:contacts" || en.ActorUserID == nil || *en.ActorUserID != *a.UserID {
			t.Fatalf("entry %+v", en)
		}
		got = append(got, en.Action)
	}
	want := "data.created data.updated data.public_set data.rows_changed data.rows_changed data.deleted"
	if strings.Join(got, " ") != want {
		t.Fatalf("actions = %v; want %s", got, want)
	}
}

func TestDataPublic(t *testing.T) {
	e := newEnv(t)
	a := e.dataAdmin()
	if _, err := e.data.Create(ctx, textValue("house.address"), a); err != nil {
		t.Fatal(err)
	}
	if list, err := e.data.Public(ctx, "1.1.1.1"); err != nil || len(list) != 0 {
		t.Fatalf("Public = %+v, %v", list, err)
	}
	if _, err := e.data.PublicOne(ctx, "house.address", "1.1.1.1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("PublicOne private = %v", err)
	}
	if _, err := e.data.Patch(ctx, "house.address", domain.DataInput{Public: ptr(true)}, a); err != nil {
		t.Fatal(err)
	}
	if v, err := e.data.PublicOne(ctx, "house.address", "1.1.1.1"); err != nil || string(v.Item.Value) != `"x"` {
		t.Fatalf("PublicOne = %+v, %v", v, err)
	}
	for i := 3; i < 120; i++ {
		if _, err := e.data.Public(ctx, "1.1.1.1"); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if _, err := e.data.Public(ctx, "1.1.1.1"); !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("121st call = %v; want ErrRateLimited", err)
	}
	if _, err := e.data.Public(ctx, "2.2.2.2"); err != nil {
		t.Fatalf("other IP: %v", err)
	}
}

func TestDataAssetRefs(t *testing.T) {
	e := newEnv(t)
	a := e.dataAdmin()
	img1, img2 := e.asset("image/png"), e.asset("image/webp")
	in := domain.DataInput{Key: "logo", Kind: domain.DataValue, ValueType: ptr(domain.DataImage), Value: raw(fmt.Sprint(img1))}
	if _, err := e.data.Create(ctx, in, a); err != nil {
		t.Fatal(err)
	}
	check := func(id int64, want bool) {
		t.Helper()
		if got, err := e.data.AssetIsPublic(ctx, id); err != nil || got != want {
			t.Fatalf("AssetIsPublic(%d) = %v, %v; want %v", id, got, err, want)
		}
	}
	check(img1, false)
	if _, err := e.data.Patch(ctx, "logo", domain.DataInput{Public: ptr(true)}, a); err != nil {
		t.Fatal(err)
	}
	check(img1, true)
	if _, err := e.data.Patch(ctx, "logo", domain.DataInput{ValueType: ptr(domain.DataImage), Value: raw(fmt.Sprint(img2))}, a); err != nil {
		t.Fatal(err)
	}
	check(img1, false)
	check(img2, true)

	cols := []domain.DataColumn{{Name: "pic", Type: domain.DataImage}}
	if _, err := e.data.Create(ctx, domain.DataInput{Key: "gallery", Kind: domain.DataTable, Columns: &cols, Public: ptr(true)}, a); err != nil {
		t.Fatal(err)
	}
	if _, err := e.data.UpsertRows(ctx, "gallery", []domain.DataRow{{Cells: cells("pic", fmt.Sprint(img1))}}, a); err != nil {
		t.Fatal(err)
	}
	check(img1, true)
}
