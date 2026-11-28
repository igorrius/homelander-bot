package sqlite_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite/sqlitetest"
)

func TestModulesGetSave(t *testing.T) {
	r := sqlite.Modules{DB: sqlitetest.New(t)}
	if _, err := r.Get(ctx, "feed"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get missing = %v", err)
	}
	s := domain.ModuleState{Code: "feed", Enabled: false, Settings: json.RawMessage(`{"defaultLabelIds":[1]}`), UpdatedAt: t0}
	if err := r.Save(ctx, s); err != nil {
		t.Fatal(err)
	}
	s.Enabled = true
	if err := r.Save(ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, "feed")
	if err != nil || !reflect.DeepEqual(got, s) {
		t.Fatalf("Get = %+v, %v; want %+v", got, err, s)
	}
}

func mkPost(t *testing.T, r sqlite.Feed, body string, published *time.Time) domain.FeedPost {
	t.Helper()
	p := domain.FeedPost{Body: body, State: domain.PostDraft, LabelIDs: []int64{}, CreatedAt: t0, UpdatedAt: t0}
	if published != nil {
		p.State, p.PublishedAt = domain.PostPublished, published
	}
	if err := r.Create(ctx, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFeedPostRoundTrip(t *testing.T) {
	r := sqlite.Feed{DB: sqlitetest.New(t)}
	p := mkPost(t, r, "hello\nworld", nil)
	p.LabelIDs = []int64{3, 1}
	at := t0.Add(time.Minute)
	p.State, p.PublishedAt, p.UpdatedAt = domain.PostPublished, &at, at
	if err := r.Update(ctx, &p); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, p.ID)
	if err != nil || !reflect.DeepEqual(got, p) {
		t.Fatalf("Get = %+v, %v; want %+v", got, err, p)
	}
	p.DeletedAt = &at
	if err := r.Update(ctx, &p); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Get(ctx, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get deleted = %v", err)
	}
	if list, total, err := r.List(ctx, 10, 0); err != nil || total != 0 || len(list) != 0 {
		t.Errorf("List = %v, %d, %v", list, total, err)
	}
}

func TestFeedPublishedPagesOutOfIDOrder(t *testing.T) {
	r := sqlite.Feed{DB: sqlitetest.New(t)}
	at := func(m int) *time.Time { v := t0.Add(time.Duration(m) * time.Minute); return &v }
	// #1 is published after #2, #3 shares #2's time, #4 is a draft.
	p1 := mkPost(t, r, "one", at(5))
	p2 := mkPost(t, r, "two", at(1))
	p3 := mkPost(t, r, "three", at(1))
	mkPost(t, r, "draft", nil)
	var got []int64
	var after *domain.FeedCursor
	for range 5 {
		page, err := r.Published(ctx, after, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		got = append(got, page[0].ID)
		after = &domain.FeedCursor{At: *page[0].PublishedAt, ID: page[0].ID}
	}
	if want := []int64{p1.ID, p3.ID, p2.ID}; !reflect.DeepEqual(got, want) {
		t.Errorf("pages = %v, want %v", got, want)
	}
	if list, total, err := r.List(ctx, 2, 0); err != nil || total != 4 || len(list) != 2 || list[0].Body != "draft" {
		t.Errorf("List = %+v, %d, %v", list, total, err)
	}
}

func TestFeedCursorString(t *testing.T) {
	c := domain.FeedCursor{At: t0.Add(123), ID: 7}
	got, err := domain.ParseFeedCursor(c.String())
	if err != nil || !got.At.Equal(c.At) || got.ID != 7 {
		t.Fatalf("parse = %+v, %v", got, err)
	}
	for _, bad := range []string{"", "x", "1_x", "1_0", "x_1"} {
		if _, err := domain.ParseFeedCursor(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}
