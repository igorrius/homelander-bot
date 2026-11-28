package domain_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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
	_, err = e.data.Create(ctx, domain.DataInput{Key: "k", Kind: domain.DataValue, ValueType: ptr(domain.DataType("money"))}, a)
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
	if _, err := e.data.Patch(ctx, "contacts", domain.DataInput{Visibility: ptr(domain.VisResidents)}, a); err != nil {
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
		switch en.Action {
		case domain.ActDataCreated:
			if en.Details["visibility"] != "admins" {
				t.Errorf("created details = %v", en.Details)
			}
		case domain.ActDataVisibilitySet:
			if en.Details["from"] != "admins" || en.Details["to"] != "residents" {
				t.Errorf("visibility_set details = %v", en.Details)
			}
		}
		got = append(got, en.Action)
	}
	want := "data.created data.updated data.visibility_set data.rows_changed data.rows_changed data.deleted"
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
	if _, err := e.data.Patch(ctx, "house.address", domain.DataInput{Visibility: ptr(domain.VisResidents)}, a); err != nil {
		t.Fatal(err)
	}
	if _, err := e.data.PublicOne(ctx, "house.address", "1.1.1.1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("PublicOne residents = %v", err)
	}
	if _, err := e.data.Patch(ctx, "house.address", domain.DataInput{Visibility: ptr(domain.VisPublic)}, a); err != nil {
		t.Fatal(err)
	}
	if v, err := e.data.PublicOne(ctx, "house.address", "1.1.1.1"); err != nil || string(v.Item.Value) != `"x"` {
		t.Fatalf("PublicOne = %+v, %v", v, err)
	}
	for i := 4; i < 120; i++ {
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
	if _, err := e.data.Patch(ctx, "logo", domain.DataInput{Visibility: ptr(domain.VisPublic)}, a); err != nil {
		t.Fatal(err)
	}
	check(img1, true)
	if _, err := e.data.Patch(ctx, "logo", domain.DataInput{ValueType: ptr(domain.DataImage), Value: raw(fmt.Sprint(img2))}, a); err != nil {
		t.Fatal(err)
	}
	check(img1, false)
	check(img2, true)

	cols := []domain.DataColumn{{Name: "pic", Type: domain.DataImage}}
	if _, err := e.data.Create(ctx, domain.DataInput{Key: "gallery", Kind: domain.DataTable, Columns: &cols, Visibility: ptr(domain.VisPublic)}, a); err != nil {
		t.Fatal(err)
	}
	if _, err := e.data.UpsertRows(ctx, "gallery", []domain.DataRow{{Cells: cells("pic", fmt.Sprint(img1))}}, a); err != nil {
		t.Fatal(err)
	}
	check(img1, true)
}

func TestDataPatchRows(t *testing.T) {
	e := newEnv(t)
	a := e.dataAdmin()
	cols := []domain.DataColumn{{Name: "name", Label: "Name", Type: domain.DataText}, {Name: "phone", Label: "Phone", Type: domain.DataPhone}}
	if _, err := e.data.Create(ctx, domain.DataInput{Key: "contacts", Kind: domain.DataTable, Columns: &cols}, a); err != nil {
		t.Fatal(err)
	}
	rows, err := e.data.PatchRows(ctx, "contacts", []domain.RowPatch{
		{Cells: map[string]json.RawMessage{"name": raw(`"Office"`), "phone": raw(`"+380500000001"`)}},
		{Cells: map[string]json.RawMessage{"name": raw(`"Guard"`)}},
	}, a)
	if err != nil || len(rows) != 2 || rows[0].Position >= rows[1].Position {
		t.Fatalf("add = %+v, %v", rows, err)
	}
	// The panel changes the name meanwhile; the agent then changes only the phone.
	office := rows[0]
	office.Cells["name"] = raw(`"Front office"`)
	if _, err := e.data.UpsertRows(ctx, "contacts", []domain.DataRow{office}, a); err != nil {
		t.Fatal(err)
	}
	rows, err = e.data.PatchRows(ctx, "contacts", []domain.RowPatch{{ID: &office.ID, Cells: map[string]json.RawMessage{"phone": raw(`"+380500000009"`)}}}, a)
	if err != nil {
		t.Fatal(err)
	}
	got := rows[0]
	if string(got.Cells["name"]) != `"Front office"` || string(got.Cells["phone"]) != `"+380500000009"` || got.Position != office.Position {
		t.Errorf("patched row = %+v", got)
	}
	missing := int64(9999)
	var ve *domain.ValidationError
	if _, err := e.data.PatchRows(ctx, "contacts", []domain.RowPatch{{ID: &missing}}, a); !errors.As(err, &ve) || ve.Fields["rows"] != "unknown_id" {
		t.Errorf("unknown id = %v", err)
	}
}

func TestDataValueURLPhone(t *testing.T) {
	e := newEnv(t)
	a := e.dataAdmin()
	for _, c := range []struct {
		typ     domain.DataType
		ok, bad string
	}{
		{domain.DataURL, `"https://x.test"`, `"ftp://x"`},
		{domain.DataPhone, `"+380 44 000 00 00"`, `"call me"`},
	} {
		key := "v_" + string(c.typ)
		if _, err := e.data.Create(ctx, domain.DataInput{Key: key, Kind: domain.DataValue, ValueType: ptr(c.typ), Value: raw(c.ok)}, a); err != nil {
			t.Fatalf("%s %s: %v", c.typ, c.ok, err)
		}
		_, err := e.data.Patch(ctx, key, domain.DataInput{Value: raw(c.bad)}, a)
		fieldErr(t, err, "value", "invalid")
	}
}

func TestDataVisibility(t *testing.T) {
	e := newEnv(t)
	a := e.dataAdmin()
	it, err := e.data.Create(ctx, textValue("plain"), a)
	if err != nil || it.Visibility != domain.VisAdmins {
		t.Fatalf("default visibility = %q, %v; want admins", it.Visibility, err)
	}
	bad := textValue("bad")
	bad.Visibility = ptr(domain.Visibility("everyone"))
	_, err = e.data.Create(ctx, bad, a)
	fieldErr(t, err, "visibility", "invalid")
	_, err = e.data.Patch(ctx, "plain", domain.DataInput{Visibility: ptr(domain.Visibility(""))}, a)
	fieldErr(t, err, "visibility", "invalid")

	// A patch that changes the description and the visibility writes both entries.
	desc := "now for residents"
	if _, err := e.data.Patch(ctx, "plain", domain.DataInput{Description: &desc, Visibility: ptr(domain.VisResidents)}, a); err != nil {
		t.Fatal(err)
	}
	// The same visibility again is not a change.
	if _, err := e.data.Patch(ctx, "plain", domain.DataInput{Visibility: ptr(domain.VisResidents)}, a); err != nil {
		t.Fatal(err)
	}
	list, _, err := e.r.Audit.List(ctx, domain.AuditFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for i := len(list) - 1; i >= 0; i-- {
		if strings.HasPrefix(list[i].Action, "data.") {
			got = append(got, list[i].Action)
		}
	}
	if strings.Join(got, " ") != "data.created data.updated data.visibility_set" {
		t.Fatalf("actions = %v", got)
	}
	if v, _ := e.data.Get(ctx, "plain"); v.Item.Visibility != domain.VisResidents || v.Item.Description != desc {
		t.Fatalf("item = %+v", v.Item)
	}
}

func TestDataForUser(t *testing.T) {
	e := newEnv(t)
	a := e.dataAdmin()
	for key, vis := range map[string]domain.Visibility{"pub": domain.VisPublic, "res": domain.VisResidents, "adm": domain.VisAdmins} {
		in := textValue(key)
		in.Visibility = ptr(vis)
		if _, err := e.data.Create(ctx, in, a); err != nil {
			t.Fatal(err)
		}
	}
	keys := func(u domain.User) string {
		t.Helper()
		list, err := e.data.ForUser(ctx, u)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, v := range list {
			out = append(out, v.Item.Key)
		}
		return strings.Join(out, " ")
	}
	resident := e.mkUser("+380500000001")
	adminOnly := e.mkUser("+380500000002", admin, func(u *domain.User) { u.IsResident = false })
	neither := e.mkUser("+380500000003", func(u *domain.User) { u.IsResident = false })
	disabled := e.mkUser("+380500000004", func(u *domain.User) { u.Status = domain.StatusDisabled })
	pending := e.mkUser("+380500000005", func(u *domain.User) { u.Status = domain.StatusPendingApproval })
	for _, c := range []struct {
		name string
		u    domain.User
		want string
	}{
		{"resident", resident, "pub res"},
		{"admin", adminOnly, "pub res"},
		{"neither", neither, "pub"},
		{"disabled", disabled, "pub"},
		{"pending", pending, "pub"},
	} {
		if got := keys(c.u); got != c.want {
			t.Errorf("%s: %q; want %q", c.name, got, c.want)
		}
	}
	for i := 0; i < 119; i++ {
		if _, err := e.data.ForUser(ctx, resident); err != nil {
			t.Fatalf("call %d: %v", i+2, err)
		}
	}
	if _, err := e.data.ForUser(ctx, resident); !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("121st call = %v; want ErrRateLimited", err)
	}
}

func TestSignedAssetURL(t *testing.T) {
	e := newEnv(t)
	parse := func(u string) (id, exp, sig string) {
		t.Helper()
		path, query, ok := strings.Cut(u, "?")
		if !ok || !strings.HasPrefix(path, "/api/v1/assets/") {
			t.Fatalf("url = %q", u)
		}
		var vals = map[string]string{}
		for _, kv := range strings.Split(query, "&") {
			k, v, _ := strings.Cut(kv, "=")
			vals[k] = v
		}
		return strings.TrimPrefix(path, "/api/v1/assets/"), vals["exp"], vals["sig"]
	}
	id, exp, sig := parse(e.data.SignedAssetURL(7))
	if id != "7" || exp == "" || sig == "" {
		t.Fatalf("parts = %q %q %q", id, exp, sig)
	}
	if !e.data.ValidAssetSig(7, exp, sig) {
		t.Fatal("fresh signature refused")
	}
	if e.data.ValidAssetSig(8, exp, sig) {
		t.Error("signature accepted for another asset")
	}
	if e.data.ValidAssetSig(7, exp+"0", sig) {
		t.Error("signature accepted with another expiry")
	}
	tampered := "A" + sig[1:]
	if tampered == sig {
		tampered = "B" + sig[1:]
	}
	if e.data.ValidAssetSig(7, exp, tampered) {
		t.Error("tampered signature accepted")
	}
	if e.data.ValidAssetSig(7, "", "") || e.data.ValidAssetSig(7, "soon", sig) {
		t.Error("empty or bad expiry accepted")
	}
	// Stable within the hour, so browsers can cache the image.
	e.clock.Advance(20 * time.Minute)
	if _, exp2, _ := parse(e.data.SignedAssetURL(7)); exp2 != exp {
		t.Errorf("expiry moved within the hour: %s → %s", exp, exp2)
	}
	e.clock.Advance(100 * time.Minute) // 2 h after the URL was made
	if e.data.ValidAssetSig(7, exp, sig) {
		t.Error("expired signature accepted")
	}
}
