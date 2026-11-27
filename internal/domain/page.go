package domain

import (
	"context"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// PageStatus is where a home page version is in its life.
type PageStatus string

// Page statuses: drafts are editable, published and archived ones are not.
const (
	PageDraft     PageStatus = "draft"
	PagePublished PageStatus = "published"
	PageArchived  PageStatus = "archived"
)

// PageFile is one file of a page version.
type PageFile struct {
	Path        string
	ContentType string
	Data        []byte // empty unless loaded with File or Files
	SHA256      string
	Size        int
}

// PageVersion is one uploaded set of home page files.
type PageVersion struct {
	ID          int64
	Status      PageStatus
	Note        string
	CreatedBy   *int64
	CreatedVia  string // the agent's client name when written through MCP
	CreatedAt   time.Time
	UpdatedAt   time.Time
	PublishedBy *int64
	PublishedAt *time.Time
	Files       []PageFile // metadata only, sorted by path
	TotalSize   int
}

// PageRepo stores page versions and their files.
type PageRepo interface {
	Create(ctx context.Context, v *PageVersion) error
	// Update writes status, note, published_* and updated_at; ErrConflict on a
	// second published version.
	Update(ctx context.Context, v *PageVersion) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (PageVersion, error)
	List(ctx context.Context) ([]PageVersion, error) // newest first
	Published(ctx context.Context) (PageVersion, error)
	PutFiles(ctx context.Context, id int64, files []PageFile) error
	DeleteFiles(ctx context.Context, id int64, paths []string) error
	File(ctx context.Context, id int64, path string) (PageFile, error)
	Files(ctx context.Context, id int64) ([]PageFile, error)
}

// SitePage is one page of a site version: a root-level *.html file.
// index.html is the home page "/".
type SitePage struct {
	Name string // file name without .html
	Path string // site address: "/" or "/<name>"
	File string // file path in the version
}

// ReservedPageNames are page names that would take over an app or server
// route. Keep in sync with web/src/lib/reservedPages.ts.
var ReservedPageNames = []string{
	"admin", "login", "register", "oauth", "site", "api", "mcp", "healthz",
	"assets", "telegram", "favicon", "robots", "sitemap",
}

var pageNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// checkPageName returns why a root *.html file cannot be a page: "" (it can,
// or the file is not a root *.html), "invalid_page_name" or "reserved_page_name".
func checkPageName(file string) string {
	if strings.Contains(file, "/") || !strings.HasSuffix(file, ".html") {
		return ""
	}
	name := strings.TrimSuffix(file, ".html")
	if strings.HasPrefix(name, ".") {
		return "reserved_page_name"
	}
	if !pageNameRe.MatchString(name) {
		return "invalid_page_name"
	}
	if slices.Contains(ReservedPageNames, name) {
		return "reserved_page_name"
	}
	return ""
}

// PagesOf returns the pages of a version's files: "/" first, then by name.
// A root *.html whose name breaks the rules is not a page.
func PagesOf(files []PageFile) []SitePage {
	out := []SitePage{}
	for _, f := range files {
		if strings.Contains(f.Path, "/") || !strings.HasSuffix(f.Path, ".html") || checkPageName(f.Path) != "" {
			continue
		}
		name := strings.TrimSuffix(f.Path, ".html")
		p := "/" + name
		if name == "index" {
			p = "/"
		}
		out = append(out, SitePage{Name: name, Path: p, File: f.Path})
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Path == "/") != (out[j].Path == "/") {
			return out[i].Path == "/"
		}
		return out[i].Name < out[j].Name
	})
	return out
}
