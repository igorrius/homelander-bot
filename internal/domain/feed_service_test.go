package domain_test

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

type feedEnv struct {
	*chatEnv
	modules *domain.ModuleService
	feed    *domain.FeedService
	a       domain.Actor
}

func newFeedEnv(t *testing.T, publicURL string) *feedEnv {
	t.Helper()
	c := newChatEnv(t)
	f := &feedEnv{chatEnv: c, a: actor(c.admin)}
	f.modules = &domain.ModuleService{R: c.r, Now: c.clock.Now}
	n := c.notifier()
	f.feed = &domain.FeedService{R: c.r, Now: c.clock.Now, Modules: f.modules, Data: c.data, Notifier: n, I18n: n.I18n,
		DefaultLocale: n.DefaultLocale, PublicURL: publicURL}
	f.modules.Defs = []domain.ModuleDef{f.feed.Def()}
	c.data.Modules = f.modules
	if err := f.modules.EnsureAll(ctx); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *feedEnv) post(body string, image *int64) domain.FeedPost {
	f.t.Helper()
	p, err := f.feed.Create(ctx, domain.PostInput{Body: body, ImageAssetID: image}, f.a)
	f.must(err)
	return p
}

// feedRows returns the cells of feed.posts, decoded.
func (f *feedEnv) feedRows() []map[string]any {
	f.t.Helper()
	v, err := f.data.Get(ctx, domain.FeedPostsKey)
	f.must(err)
	out := make([]map[string]any, 0, len(v.Rows))
	for _, r := range v.Rows {
		m := map[string]any{}
		for k, c := range r.Cells {
			var x any
			f.must(json.Unmarshal(c, &x))
			m[k] = x
		}
		out = append(out, m)
	}
	return out
}

func TestFeedEnsureMakesOwnedTable(t *testing.T) {
	f := newFeedEnv(t, "https://example.com")
	v, err := f.data.Get(ctx, "feed.posts")
	if err != nil || v.Item.Owner != "module:feed" || v.Item.Visibility != domain.VisResidents || v.Item.Kind != domain.DataTable {
		t.Fatalf("feed.posts = %+v, %v", v.Item, err)
	}
	var names []string
	for _, c := range v.Item.Columns {
		names = append(names, c.Name+":"+string(c.Type))
	}
	if got := strings.Join(names, " "); got != "id:number date:text text:text image:image url:url" {
		t.Errorf("columns = %s", got)
	}
	if len(v.Rows) != 0 {
		t.Errorf("rows = %d", len(v.Rows))
	}
}

func TestFeedPostValidation(t *testing.T) {
	f := newFeedEnv(t, "https://example.com")
	pic, doc := f.asset("image/png"), f.asset("application/pdf")
	cases := map[string]struct {
		in            domain.PostInput
		field, reason string
	}{
		"empty":        {domain.PostInput{Body: "  \n "}, "body", "required"},
		"long":         {domain.PostInput{Body: strings.Repeat("я", 4001)}, "body", "too_long"},
		"long caption": {domain.PostInput{Body: strings.Repeat("я", 1001), ImageAssetID: &pic}, "body", "too_long"},
		"not image":    {domain.PostInput{Body: "x", ImageAssetID: &doc}, "imageAssetId", "invalid"},
		"no asset":     {domain.PostInput{Body: "x", ImageAssetID: ptr(int64(999))}, "imageAssetId", "invalid"},
		"bad label":    {domain.PostInput{Body: "x", LabelIDs: &[]int64{77}}, "labelIds", "invalid"},
	}
	for name, tc := range cases {
		var ve *domain.ValidationError
		if _, err := f.feed.Create(ctx, tc.in, f.a); !errors.As(err, &ve) || ve.Fields[tc.field] != tc.reason {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := f.feed.Create(ctx, domain.PostInput{Body: strings.Repeat("я", 4000)}, f.a); err != nil {
		t.Errorf("4000 runes: %v", err)
	}
	if p := f.post("  hello  ", &pic); p.Body != "hello" || p.State != domain.PostDraft || f.auditCount(domain.ActFeedPostCreated) != 2 {
		t.Errorf("post = %+v", p)
	}
}

func TestFeedPublishQueuesOncePerPlace(t *testing.T) {
	f := newFeedEnv(t, "https://example.com")
	ch, topic := f.acceptedForum(superGroup, "9")
	residents, journal := f.mkLabel("residents"), f.mkLabel("journal")
	chat := domain.LabelTarget{Type: domain.TargetChat, ID: ch.ID}
	f.bind(residents, chat, domain.LabelTarget{Type: domain.TargetTopic, ID: topic.ID})
	f.bind(journal, chat)
	pic := f.asset("image/png")
	p := f.post("Water is off on Monday", &pic)
	f.clock.Advance(time.Minute)
	got, err := f.feed.Publish(ctx, p.ID, []int64{journal.ID, residents.ID, journal.ID}, f.a)
	if err != nil || got.State != domain.PostPublished || got.PublishedAt == nil || len(got.LabelIDs) != 2 {
		t.Fatalf("Publish = %+v, %v", got, err)
	}
	rows := f.pending()
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	origin := "feed:" + strconv.FormatInt(p.ID, 10)
	for _, r := range rows {
		if r.Origin != origin || r.Msg.Text != "Water is off on Monday" || r.Msg.ImageAssetID != pic ||
			len(r.Msg.Buttons) != 1 || r.Msg.Buttons[0].URL != "https://example.com/my/feed/"+strconv.FormatInt(p.ID, 10) {
			t.Errorf("row = %+v", r)
		}
	}
	if _, err := f.feed.Publish(ctx, p.ID, nil, f.a); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("publish again = %v", err)
	}
	var ve *domain.ValidationError
	if _, err := f.feed.Update(ctx, p.ID, domain.PostInput{Body: "x", LabelIDs: &[]int64{residents.ID}}, f.a); !errors.As(err, &ve) || ve.Fields["labelIds"] != "published" {
		t.Errorf("relabel published = %v", err)
	}
	if f.auditCount(domain.ActFeedPostPublished) != 1 {
		t.Errorf("published audits = %d", f.auditCount(domain.ActFeedPostPublished))
	}
	d, err := f.feed.Deliveries(ctx, p.ID)
	if err != nil || len(d) != 2 {
		t.Errorf("deliveries = %+v, %v", d, err)
	}
	rs := f.feedRows()
	if len(rs) != 1 || rs[0]["text"] != "Water is off on Monday" || rs[0]["image"] != float64(pic) ||
		rs[0]["url"] != "https://example.com/my/feed/"+strconv.FormatInt(p.ID, 10) || rs[0]["date"] != "2026-11-27T12:01:00Z" {
		t.Errorf("feed.posts = %+v", rs)
	}
}

func TestFeedEditDeleteRewriteWithoutSending(t *testing.T) {
	f := newFeedEnv(t, "https://example.com")
	ch, _ := f.acceptedForum(superGroup, "9")
	l := f.mkLabel("residents")
	f.bind(l, domain.LabelTarget{Type: domain.TargetChat, ID: ch.ID})
	a, b := f.post("first", nil), f.post("second", nil)
	for _, p := range []domain.FeedPost{a, b} {
		f.clock.Advance(time.Minute)
		if _, err := f.feed.Publish(ctx, p.ID, []int64{l.ID}, f.a); err != nil {
			t.Fatal(err)
		}
	}
	queued := len(f.pending())
	if _, err := f.feed.Update(ctx, a.ID, domain.PostInput{Body: "first, fixed"}, f.a); err != nil {
		t.Fatal(err)
	}
	if err := f.feed.Delete(ctx, b.ID, f.a); err != nil {
		t.Fatal(err)
	}
	if len(f.pending()) != queued {
		t.Errorf("edit/delete queued messages: %d → %d", queued, len(f.pending()))
	}
	rs := f.feedRows()
	if len(rs) != 1 || rs[0]["text"] != "first, fixed" {
		t.Errorf("feed.posts = %+v", rs)
	}
	if _, err := f.feed.Get(ctx, b.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("deleted post = %v", err)
	}
	if f.auditCount(domain.ActFeedPostUpdated) != 1 || f.auditCount(domain.ActFeedPostDeleted) != 1 {
		t.Errorf("audits updated %d deleted %d", f.auditCount(domain.ActFeedPostUpdated), f.auditCount(domain.ActFeedPostDeleted))
	}
}

func TestFeedKeepsLatest20AndPages(t *testing.T) {
	f := newFeedEnv(t, "http://localhost:8080")
	for i := range 25 {
		p := f.post("post "+strconv.Itoa(i), nil)
		f.clock.Advance(time.Second)
		if _, err := f.feed.Publish(ctx, p.ID, nil, f.a); err != nil {
			t.Fatal(err)
		}
	}
	rs := f.feedRows()
	if len(rs) != 20 || rs[0]["text"] != "post 24" || rs[0]["url"] != "http://localhost:8080/my/feed/25" {
		t.Errorf("feed.posts: %d rows, first %+v", len(rs), rs[0])
	}
	page, next, err := f.feed.Published(ctx, "", 20)
	if err != nil || len(page) != 20 || next == "" {
		t.Fatalf("page 1 = %d, %q, %v", len(page), next, err)
	}
	page2, next2, err := f.feed.Published(ctx, next, 20)
	if err != nil || len(page2) != 5 || next2 != "" || page2[4].Body != "post 0" {
		t.Fatalf("page 2 = %d, %q, %v", len(page2), next2, err)
	}
	if _, _, err := f.feed.Published(ctx, "nope", 20); err == nil {
		t.Error("bad cursor accepted")
	}
	draft := f.post("draft", nil)
	if _, err := f.feed.PublishedOne(ctx, draft.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("draft in cabinet = %v", err)
	}
}

func TestFeedHTTPPublicURLHasNoButton(t *testing.T) {
	f := newFeedEnv(t, "http://localhost:8080")
	ch, _ := f.acceptedForum(superGroup, "9")
	l := f.mkLabel("residents")
	f.bind(l, domain.LabelTarget{Type: domain.TargetChat, ID: ch.ID})
	p := f.post("hi", nil)
	if _, err := f.feed.Publish(ctx, p.ID, []int64{l.ID}, f.a); err != nil {
		t.Fatal(err)
	}
	if rows := f.pending(); len(rows) != 1 || len(rows[0].Msg.Buttons) != 0 || rows[0].Msg.Text != "hi" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestFeedOffRefusesEverything(t *testing.T) {
	f := newFeedEnv(t, "https://example.com")
	p := f.post("hi", nil)
	if _, err := f.modules.SetEnabled(ctx, "feed", false, f.a); err != nil {
		t.Fatal(err)
	}
	errs := map[string]error{}
	_, errs["create"] = f.feed.Create(ctx, domain.PostInput{Body: "x"}, f.a)
	_, errs["update"] = f.feed.Update(ctx, p.ID, domain.PostInput{Body: "x"}, f.a)
	_, errs["publish"] = f.feed.Publish(ctx, p.ID, nil, f.a)
	errs["delete"] = f.feed.Delete(ctx, p.ID, f.a)
	_, errs["get"] = f.feed.Get(ctx, p.ID)
	_, _, errs["list"] = f.feed.List(ctx, 10, 0)
	_, _, errs["published"] = f.feed.Published(ctx, "", 20)
	_, errs["one"] = f.feed.PublishedOne(ctx, p.ID)
	_, errs["settings"] = f.feed.SaveSettings(ctx, domain.FeedSettings{}, f.a)
	_, errs["deliveries"] = f.feed.Deliveries(ctx, p.ID)
	for name, err := range errs {
		if !errors.Is(err, domain.ErrModuleOff) {
			t.Errorf("%s = %v", name, err)
		}
	}
}

func TestFeedSettings(t *testing.T) {
	f := newFeedEnv(t, "https://example.com")
	l := f.mkLabel("residents")
	var ve *domain.ValidationError
	if _, err := f.feed.SaveSettings(ctx, domain.FeedSettings{DefaultLabelIDs: []int64{l.ID, 99}}, f.a); !errors.As(err, &ve) || ve.Fields["defaultLabelIds"] != "invalid" {
		t.Fatalf("bad label = %v", err)
	}
	if _, err := f.feed.SaveSettings(ctx, domain.FeedSettings{DefaultLabelIDs: []int64{l.ID, l.ID}}, f.a); err != nil {
		t.Fatal(err)
	}
	st, err := f.feed.Settings(ctx)
	if err != nil || len(st.DefaultLabelIDs) != 1 || st.DefaultLabelIDs[0] != l.ID || f.auditCount(domain.ActModuleSettingsSaved) != 1 {
		t.Errorf("settings = %+v, %v", st, err)
	}
}
