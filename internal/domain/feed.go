package domain

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// PostState says whether a feed post is still a draft.
type PostState string

// Post states.
const (
	PostDraft     PostState = "draft"
	PostPublished PostState = "published"
)

// FeedPost is one post of the information feed: plain text with an optional
// image. LabelIDs are the labels it goes (or went) to in Telegram.
type FeedPost struct {
	ID           int64
	Body         string
	ImageAssetID *int64
	State        PostState
	PublishedAt  *time.Time
	LabelIDs     []int64
	CreatedBy    *int64
	UpdatedBy    *int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

// FeedCursor is where a page of published posts ends: the last post's
// publishing time and id (posts are newest first).
type FeedCursor struct {
	At time.Time
	ID int64
}

// String encodes the cursor for an API ("<unix nanoseconds>_<id>").
func (c FeedCursor) String() string {
	return strconv.FormatInt(c.At.UnixNano(), 10) + "_" + strconv.FormatInt(c.ID, 10)
}

var errBadCursor = errors.New("bad cursor")

// ParseFeedCursor reads a cursor made by FeedCursor.String.
func ParseFeedCursor(s string) (FeedCursor, error) {
	at, id, ok := strings.Cut(s, "_")
	if !ok {
		return FeedCursor{}, errBadCursor
	}
	ns, err1 := strconv.ParseInt(at, 10, 64)
	n, err2 := strconv.ParseInt(id, 10, 64)
	if err1 != nil || err2 != nil || n <= 0 {
		return FeedCursor{}, errBadCursor
	}
	return FeedCursor{At: time.Unix(0, ns).UTC(), ID: n}, nil
}

// FeedRepo stores feed posts.
type FeedRepo interface {
	Create(ctx context.Context, p *FeedPost) error
	Update(ctx context.Context, p *FeedPost) error
	// Get returns a post that is not deleted; ErrNotFound otherwise.
	Get(ctx context.Context, id int64) (FeedPost, error)
	// List returns posts that are not deleted, newest first, and their number.
	List(ctx context.Context, limit, offset int) ([]FeedPost, int, error)
	// Published returns published, not deleted posts, newest published first,
	// after the cursor when one is given.
	Published(ctx context.Context, after *FeedCursor, limit int) ([]FeedPost, error)
}
