package domain

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// The information feed module.
const (
	FeedCode      = "feed"
	FeedPostsKey  = "feed.posts" // the module's data item: the latest posts for the site
	FeedPostsRows = 20
	// FeedPageSize is how many posts a page of the cabinet feed has.
	FeedPageSize   = 20
	maxPostBody    = 4000
	maxPostCaption = 1000 // Telegram caption limit is 1024
)

// FeedOwner is the data owner of the feed's items.
var FeedOwner = ModuleOwner(FeedCode)

var feedColumns = []DataColumn{
	{Name: "id", Label: "ID", Type: DataNumber},
	{Name: "date", Label: "Published", Type: DataText},
	{Name: "text", Label: "Text", Type: DataText},
	{Name: "image", Label: "Image", Type: DataImage},
	{Name: "url", Label: "Link", Type: DataURL},
}

const feedPostsDescription = "The latest published posts of the information feed. Kept by the feed module."

// FeedSettings are the feed's admin settings.
type FeedSettings struct {
	// DefaultLabelIDs prefill "Also post to" of a new post.
	DefaultLabelIDs []int64 `json:"defaultLabelIds"`
}

// PostInput is a post's content. LabelIDs nil leaves the labels as they are.
type PostInput struct {
	Body         string
	ImageAssetID *int64
	LabelIDs     *[]int64
}

// FeedService runs the information feed: admins write posts, a published
// post shows in the cabinet, goes once to every Telegram place its labels
// reach, and the latest posts are kept in the data item feed.posts.
type FeedService struct {
	R             Repos
	Now           Clock
	Modules       *ModuleService
	Data          *DataService
	Notifier      *Notifier
	I18n          Translator
	DefaultLocale func(ctx context.Context) string
	PublicURL     string
}

// Def is the feed's registry entry.
func (s *FeedService) Def() ModuleDef {
	return ModuleDef{Code: FeedCode, NameKey: "modules.feed.name", DescriptionKey: "modules.feed.description", DefaultEnabled: true,
		Nav:    []ModuleNav{{Key: "feed", Path: "/my/feed", LabelKey: "cabinet.nav.feed", Icon: "news"}},
		Ensure: s.ensure}
}

// ensure creates feed.posts when it is missing and fills it.
func (s *FeedService) ensure(ctx context.Context) error {
	if _, err := s.Data.EnsureOwnedTable(ctx, FeedOwner, FeedPostsKey, feedPostsDescription, slices.Clone(feedColumns), VisResidents); err != nil {
		return err
	}
	return s.refresh(ctx)
}

// PostURL is the absolute cabinet address of a post ("" without a public URL).
func (s *FeedService) PostURL(id int64) string {
	base := strings.TrimRight(s.PublicURL, "/")
	if u, err := url.Parse(base); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return base + "/my/feed/" + strconv.FormatInt(id, 10)
}

// refresh rewrites feed.posts from the latest published posts.
func (s *FeedService) refresh(ctx context.Context) error {
	posts, err := s.R.Feed.Published(ctx, nil, FeedPostsRows)
	if err != nil {
		return err
	}
	rows := make([]DataRow, 0, len(posts))
	for _, p := range posts {
		cells := map[string]json.RawMessage{
			"id":   json.RawMessage(strconv.FormatInt(p.ID, 10)),
			"date": jsonString(p.PublishedAt.UTC().Format(time.RFC3339)),
			"text": jsonString(p.Body),
		}
		if p.ImageAssetID != nil {
			cells["image"] = json.RawMessage(strconv.FormatInt(*p.ImageAssetID, 10))
		}
		if u := s.PostURL(p.ID); u != "" {
			cells["url"] = jsonString(u)
		}
		rows = append(rows, DataRow{Cells: cells})
	}
	return s.Data.ReplaceOwnedRows(ctx, FeedOwner, FeedPostsKey, rows)
}

func jsonString(v string) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func (s *FeedService) audit(ctx context.Context, a Actor, action string, id int64, details map[string]any) error {
	return s.R.Audit.Add(ctx, AuditEntry{At: s.Now(), ActorUserID: a.UserID, Action: action, Target: FeedPostTarget(id),
		Details: a.AuditDetails(details), IP: a.IP})
}

// Settings returns the feed's settings.
func (s *FeedService) Settings(ctx context.Context) (FeedSettings, error) {
	v, err := s.Modules.Get(ctx, FeedCode)
	if err != nil {
		return FeedSettings{}, err
	}
	var st FeedSettings
	_ = json.Unmarshal(v.Settings, &st)
	if st.DefaultLabelIDs == nil {
		st.DefaultLabelIDs = []int64{}
	}
	return st, nil
}

// checkLabels returns the labels without duplicates, or a validation error
// on field when one does not exist.
func (s *FeedService) checkLabels(ctx context.Context, field string, ids []int64) ([]int64, error) {
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if slices.Contains(out, id) {
			continue
		}
		if _, err := s.R.Labels.Get(ctx, id); errors.Is(err, ErrNotFound) {
			return nil, &ValidationError{Fields: map[string]string{field: "invalid"}}
		} else if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// SaveSettings changes the feed's settings.
func (s *FeedService) SaveSettings(ctx context.Context, st FeedSettings, a Actor) (FeedSettings, error) {
	if err := s.Modules.Require(ctx, FeedCode); err != nil {
		return FeedSettings{}, err
	}
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		ids, err := s.checkLabels(ctx, "defaultLabelIds", st.DefaultLabelIDs)
		if err != nil {
			return err
		}
		st.DefaultLabelIDs = ids
		raw, err := json.Marshal(st)
		if err != nil {
			return err
		}
		return s.Modules.SaveSettings(ctx, FeedCode, raw, a)
	})
	return st, err
}

// check validates and normalises a post's content.
func (s *FeedService) check(ctx context.Context, p *FeedPost) error {
	v := validation{}
	p.Body = strings.TrimSpace(p.Body)
	limit := maxPostBody
	if p.ImageAssetID != nil {
		limit = maxPostCaption
		as, err := s.R.Site.GetAsset(ctx, *p.ImageAssetID)
		if errors.Is(err, ErrNotFound) || (err == nil && !strings.HasPrefix(as.ContentType, "image/")) {
			v.add("imageAssetId", "invalid")
		} else if err != nil {
			return err
		}
	}
	switch n := utf8.RuneCountInString(p.Body); {
	case n == 0:
		v.add("body", "required")
	case n > limit:
		v.add("body", "too_long")
	}
	return v.err()
}

// Create adds a draft post.
func (s *FeedService) Create(ctx context.Context, in PostInput, a Actor) (FeedPost, error) {
	if err := s.Modules.Require(ctx, FeedCode); err != nil {
		return FeedPost{}, err
	}
	now := s.Now()
	p := FeedPost{Body: in.Body, ImageAssetID: in.ImageAssetID, State: PostDraft, LabelIDs: []int64{},
		CreatedBy: a.UserID, UpdatedBy: a.UserID, CreatedAt: now, UpdatedAt: now}
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, &p); err != nil {
			return err
		}
		if in.LabelIDs != nil {
			ids, err := s.checkLabels(ctx, "labelIds", *in.LabelIDs)
			if err != nil {
				return err
			}
			p.LabelIDs = ids
		}
		if err := s.R.Feed.Create(ctx, &p); err != nil {
			return err
		}
		return s.audit(ctx, a, ActFeedPostCreated, p.ID, nil)
	})
	return p, err
}

// Update changes a post. A published post keeps its labels; the cabinet and
// the site show the change, Telegram copies stay as they were sent.
func (s *FeedService) Update(ctx context.Context, id int64, in PostInput, a Actor) (FeedPost, error) {
	if err := s.Modules.Require(ctx, FeedCode); err != nil {
		return FeedPost{}, err
	}
	var p FeedPost
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if p, err = s.R.Feed.Get(ctx, id); err != nil {
			return err
		}
		var changed []string
		if strings.TrimSpace(in.Body) != p.Body {
			changed = append(changed, "body")
		}
		if !equalPtr(in.ImageAssetID, p.ImageAssetID) {
			changed = append(changed, "image")
		}
		p.Body, p.ImageAssetID = in.Body, in.ImageAssetID
		if err := s.check(ctx, &p); err != nil {
			return err
		}
		if in.LabelIDs != nil {
			ids, err := s.checkLabels(ctx, "labelIds", *in.LabelIDs)
			if err != nil {
				return err
			}
			if !slices.Equal(ids, p.LabelIDs) {
				if p.State == PostPublished {
					return &ValidationError{Fields: map[string]string{"labelIds": "published"}}
				}
				p.LabelIDs = ids
				changed = append(changed, "labels")
			}
		}
		if len(changed) == 0 {
			return nil
		}
		p.UpdatedBy, p.UpdatedAt = a.UserID, s.Now()
		if err := s.R.Feed.Update(ctx, &p); err != nil {
			return err
		}
		if err := s.audit(ctx, a, ActFeedPostUpdated, id, map[string]any{"fields": changed}); err != nil {
			return err
		}
		if p.State == PostPublished {
			return s.refresh(ctx)
		}
		return nil
	})
	return p, err
}

func equalPtr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// message is how a post goes to Telegram: the text (a photo's caption) and
// a button to the post in the cabinet, a plain address that logs nobody in.
func (s *FeedService) message(ctx context.Context, p FeedPost) OutMessage {
	m := OutMessage{Text: p.Body}
	if p.ImageAssetID != nil {
		m.ImageAssetID = *p.ImageAssetID
	}
	// Telegram refuses buttons with other than https links.
	if u := s.PostURL(p.ID); strings.HasPrefix(u, "https://") {
		loc := ""
		if s.DefaultLocale != nil {
			loc = s.DefaultLocale(ctx)
		}
		m.Buttons = []LinkButton{{Text: s.I18n.T(loc, "feed.openInCabinet", nil, nil), URL: u}}
	}
	return m
}

// Publish publishes a draft once: it shows in the cabinet and on the site,
// and is queued once for every place the chosen labels reach. The labels are
// fixed from then on.
func (s *FeedService) Publish(ctx context.Context, id int64, labelIDs []int64, a Actor) (FeedPost, error) {
	if err := s.Modules.Require(ctx, FeedCode); err != nil {
		return FeedPost{}, err
	}
	var p FeedPost
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if p, err = s.R.Feed.Get(ctx, id); err != nil {
			return err
		}
		if p.State != PostDraft {
			return ErrConflict
		}
		if err := s.check(ctx, &p); err != nil {
			return err
		}
		if p.LabelIDs, err = s.checkLabels(ctx, "labelIds", labelIDs); err != nil {
			return err
		}
		now := s.Now()
		p.State, p.PublishedAt, p.UpdatedBy, p.UpdatedAt = PostPublished, &now, a.UserID, now
		if err := s.R.Feed.Update(ctx, &p); err != nil {
			return err
		}
		queued, err := s.Notifier.SendToLabels(ctx, p.LabelIDs, s.message(ctx, p), "feed:"+strconv.FormatInt(p.ID, 10))
		if err != nil {
			return err
		}
		if err := s.audit(ctx, a, ActFeedPostPublished, id, map[string]any{"labelIds": p.LabelIDs, "destinations": len(queued)}); err != nil {
			return err
		}
		return s.refresh(ctx)
	})
	return p, err
}

// Delete removes a post from the cabinet and the site (Telegram copies stay).
func (s *FeedService) Delete(ctx context.Context, id int64, a Actor) error {
	if err := s.Modules.Require(ctx, FeedCode); err != nil {
		return err
	}
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		p, err := s.R.Feed.Get(ctx, id)
		if err != nil {
			return err
		}
		now := s.Now()
		p.DeletedAt, p.UpdatedBy, p.UpdatedAt = &now, a.UserID, now
		if err := s.R.Feed.Update(ctx, &p); err != nil {
			return err
		}
		if err := s.audit(ctx, a, ActFeedPostDeleted, id, map[string]any{"state": string(p.State)}); err != nil {
			return err
		}
		if p.State == PostPublished {
			return s.refresh(ctx)
		}
		return nil
	})
}

// Get returns a post for the admin panel.
func (s *FeedService) Get(ctx context.Context, id int64) (FeedPost, error) {
	if err := s.Modules.Require(ctx, FeedCode); err != nil {
		return FeedPost{}, err
	}
	return s.R.Feed.Get(ctx, id)
}

// List returns a page of posts for the admin panel, newest first.
func (s *FeedService) List(ctx context.Context, limit, offset int) ([]FeedPost, int, error) {
	if err := s.Modules.Require(ctx, FeedCode); err != nil {
		return nil, 0, err
	}
	return s.R.Feed.List(ctx, limit, offset)
}

// Deliveries returns the outbox rows of a post, one per place it went to.
func (s *FeedService) Deliveries(ctx context.Context, id int64) ([]OutboxItem, error) {
	if err := s.Modules.Require(ctx, FeedCode); err != nil {
		return nil, err
	}
	return s.R.Outbox.ListByOrigin(ctx, "feed:"+strconv.FormatInt(id, 10))
}

// DeliveryKept reports whether the outbox may still have a published post's
// rows: finished rows are dropped after outboxRetention.
func (s *FeedService) DeliveryKept(p FeedPost) bool {
	return p.PublishedAt == nil || s.Now().Sub(*p.PublishedAt) < outboxRetention
}

// Published returns a page of the cabinet feed after cursor ("" for the
// first page) and the cursor of the next page ("" when this is the last).
func (s *FeedService) Published(ctx context.Context, cursor string, limit int) ([]FeedPost, string, error) {
	if err := s.Modules.Require(ctx, FeedCode); err != nil {
		return nil, "", err
	}
	var after *FeedCursor
	if cursor != "" {
		c, err := ParseFeedCursor(cursor)
		if err != nil {
			return nil, "", &ValidationError{Fields: map[string]string{"cursor": "invalid"}}
		}
		after = &c
	}
	posts, err := s.R.Feed.Published(ctx, after, limit+1)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(posts) > limit {
		posts = posts[:limit]
		last := posts[limit-1]
		next = FeedCursor{At: *last.PublishedAt, ID: last.ID}.String()
	}
	return posts, next, nil
}

// PublishedOne returns one published post for the cabinet.
func (s *FeedService) PublishedOne(ctx context.Context, id int64) (FeedPost, error) {
	if err := s.Modules.Require(ctx, FeedCode); err != nil {
		return FeedPost{}, err
	}
	p, err := s.R.Feed.Get(ctx, id)
	if err == nil && p.State != PostPublished {
		err = ErrNotFound
	}
	return p, err
}
