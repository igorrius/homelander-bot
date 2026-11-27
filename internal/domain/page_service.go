package domain

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// PageService keeps home page versions: validates their files, keeps at most
// one published, and signs short-lived preview links for drafts.
type PageService struct {
	R      Repos
	Now    Clock
	Secret []byte
}

// PageFileInput is one file to store in a version; its type comes from the
// extension.
type PageFileInput struct {
	Path string
	Data []byte
}

// Page limits and the preview link lifetime.
const (
	MaxPageFile     = 5 << 20
	MaxPageVersion  = 20 << 20
	MaxPageFiles    = 200
	maxPagePath     = 200
	maxPageNote     = 200
	PageIndex       = "index.html"
	PagePreviewTTL  = 30 * time.Minute
	pagePreviewSalt = "page-preview:"
)

var (
	pagePathRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(/[A-Za-z0-9][A-Za-z0-9._-]*)*$`)
	pageTypes  = map[string]string{
		".html": "text/html; charset=utf-8", ".css": "text/css; charset=utf-8",
		".js": "text/javascript; charset=utf-8", ".json": "application/json", ".svg": "image/svg+xml",
		".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp",
		".gif": "image/gif", ".woff2": "font/woff2",
	}
)

// PageContentType returns the content type for a valid page path, or the
// field reason ("invalid" or "type") it is rejected with.
func PageContentType(p string) (string, string) {
	// The top folder "preview" is where drafts are served, so it is reserved.
	if len(p) > maxPagePath || !pagePathRe.MatchString(p) || strings.HasPrefix(p, "preview/") {
		return "", "invalid"
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." || seg == "." {
			return "", "invalid"
		}
	}
	ext := path.Ext(p)
	if ext != strings.ToLower(ext) {
		return "", "invalid"
	}
	ct, ok := pageTypes[ext]
	if !ok {
		return "", "type"
	}
	return ct, ""
}

func (s *PageService) audit(ctx context.Context, a Actor, action string, id int64, details map[string]any) error {
	return s.R.Audit.Add(ctx, AuditEntry{At: s.Now(), ActorUserID: a.UserID, Action: action, Target: PageTarget(id),
		Details: a.AuditDetails(details), IP: a.IP})
}

// toFiles validates inputs and turns them into files.
func toFiles(in []PageFileInput) ([]PageFile, error) {
	seen := map[string]bool{}
	out := make([]PageFile, 0, len(in))
	for _, f := range in {
		ct, reason := PageContentType(f.Path)
		switch {
		case reason != "":
		case seen[f.Path]:
			reason = "invalid"
		case len(f.Data) > MaxPageFile:
			reason = "too_large"
		}
		if reason != "" {
			// The first refused file is named, so the admin or agent knows what to fix.
			return nil, &ValidationError{Fields: map[string]string{"files": reason}, Details: map[string]string{"path": f.Path}}
		}
		seen[f.Path] = true
		sum := sha256.Sum256(f.Data)
		out = append(out, PageFile{Path: f.Path, ContentType: ct, Data: f.Data, SHA256: hex.EncodeToString(sum[:]), Size: len(f.Data)})
	}
	return out, nil
}

// checkTotals checks the whole version: files by path after the change.
func checkTotals(sizes map[string]int) error {
	v := validation{}
	total := 0
	for _, n := range sizes {
		total += n
	}
	switch {
	case len(sizes) > MaxPageFiles:
		v.add("files", "too_many")
	case total > MaxPageVersion:
		v.add("files", "too_large")
	default:
		if _, ok := sizes[PageIndex]; !ok {
			v.add("files", "index_required")
		}
	}
	return v.err()
}

func checkNote(note string) (string, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > maxPageNote {
		return "", &ValidationError{Fields: map[string]string{"note": "too_long"}}
	}
	return note, nil
}

// List returns every version, newest first.
func (s *PageService) List(ctx context.Context) ([]PageVersion, error) { return s.R.Pages.List(ctx) }

// Get returns one version with its file list.
func (s *PageService) Get(ctx context.Context, id int64) (PageVersion, error) {
	return s.R.Pages.Get(ctx, id)
}

// CreateDraft stores a new draft version.
func (s *PageService) CreateDraft(ctx context.Context, in []PageFileInput, note string, a Actor) (PageVersion, error) {
	note, err := checkNote(note)
	if err != nil {
		return PageVersion{}, err
	}
	if len(in) > MaxPageFiles {
		return PageVersion{}, &ValidationError{Fields: map[string]string{"files": "too_many"}}
	}
	files, err := toFiles(in)
	if err != nil {
		return PageVersion{}, err
	}
	sizes := map[string]int{}
	for _, f := range files {
		sizes[f.Path] = f.Size
	}
	if err := checkTotals(sizes); err != nil {
		return PageVersion{}, err
	}
	now := s.Now()
	v := PageVersion{Status: PageDraft, Note: note, CreatedBy: a.UserID, CreatedVia: a.Client, CreatedAt: now, UpdatedAt: now}
	err = s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.R.Pages.Create(ctx, &v); err != nil {
			return err
		}
		if err := s.R.Pages.PutFiles(ctx, v.ID, files); err != nil {
			return err
		}
		return s.audit(ctx, a, ActPageDraftCreated, v.ID, map[string]any{"files": len(files), "note": note})
	})
	if err != nil {
		return PageVersion{}, err
	}
	return s.R.Pages.Get(ctx, v.ID)
}

// PutFiles changes a draft: stores put (replacing by path), removes del.
// Published and archived versions → ErrConflict.
func (s *PageService) PutFiles(ctx context.Context, id int64, put []PageFileInput, del []string, a Actor) (PageVersion, error) {
	if len(put) > MaxPageFiles {
		return PageVersion{}, &ValidationError{Fields: map[string]string{"files": "too_many"}}
	}
	files, err := toFiles(put)
	if err != nil {
		return PageVersion{}, err
	}
	err = s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		v, err := s.R.Pages.Get(ctx, id)
		if err != nil {
			return err
		}
		if v.Status != PageDraft {
			return ErrConflict
		}
		sizes := map[string]int{}
		for _, f := range v.Files {
			sizes[f.Path] = f.Size
		}
		for _, p := range del {
			delete(sizes, p)
		}
		for _, f := range files {
			sizes[f.Path] = f.Size
		}
		if err := checkTotals(sizes); err != nil {
			return err
		}
		if err := s.R.Pages.DeleteFiles(ctx, id, del); err != nil {
			return err
		}
		if err := s.R.Pages.PutFiles(ctx, id, files); err != nil {
			return err
		}
		v.UpdatedAt = s.Now()
		if err := s.R.Pages.Update(ctx, &v); err != nil {
			return err
		}
		return s.audit(ctx, a, ActPageDraftUpdated, id, map[string]any{"put": len(files), "deleted": len(del)})
	})
	if err != nil {
		return PageVersion{}, err
	}
	return s.R.Pages.Get(ctx, id)
}

// CopyToDraft makes a new draft with the files of version id.
func (s *PageService) CopyToDraft(ctx context.Context, id int64, a Actor) (PageVersion, error) {
	src, err := s.R.Pages.Get(ctx, id)
	if err != nil {
		return PageVersion{}, err
	}
	files, err := s.R.Pages.Files(ctx, id)
	if err != nil {
		return PageVersion{}, err
	}
	now := s.Now()
	v := PageVersion{Status: PageDraft, Note: src.Note, CreatedBy: a.UserID, CreatedVia: a.Client, CreatedAt: now, UpdatedAt: now}
	err = s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.R.Pages.Create(ctx, &v); err != nil {
			return err
		}
		if err := s.R.Pages.PutFiles(ctx, v.ID, files); err != nil {
			return err
		}
		return s.audit(ctx, a, ActPageDraftCreated, v.ID, map[string]any{"files": len(files), "copiedFrom": id})
	})
	if err != nil {
		return PageVersion{}, err
	}
	return s.R.Pages.Get(ctx, v.ID)
}

// Publish makes version id the published one and archives the previous one;
// publishing the published version again does nothing.
func (s *PageService) Publish(ctx context.Context, id int64, a Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		v, err := s.R.Pages.Get(ctx, id)
		if err != nil {
			return err
		}
		if v.Status == PagePublished {
			return nil
		}
		now := s.Now()
		prev, err := s.R.Pages.Published(ctx)
		switch {
		case err == nil:
			prev.Status, prev.UpdatedAt = PageArchived, now
			if err := s.R.Pages.Update(ctx, &prev); err != nil {
				return err
			}
		case !errors.Is(err, ErrNotFound):
			return err
		}
		v.Status, v.PublishedBy, v.PublishedAt, v.UpdatedAt = PagePublished, a.UserID, &now, now
		if err := s.R.Pages.Update(ctx, &v); err != nil {
			return err
		}
		details := map[string]any{}
		if prev.ID != 0 {
			details["archived"] = prev.ID
		}
		return s.audit(ctx, a, ActPagePublished, id, details)
	})
}

// Unpublish archives the published version, so the built-in page shows; no-op
// when nothing is published.
func (s *PageService) Unpublish(ctx context.Context, a Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		v, err := s.R.Pages.Published(ctx)
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		v.Status, v.UpdatedAt = PageArchived, s.Now()
		if err := s.R.Pages.Update(ctx, &v); err != nil {
			return err
		}
		return s.audit(ctx, a, ActPageUnpublished, v.ID, nil)
	})
}

// Delete removes a draft or archived version; the published one → ErrConflict.
func (s *PageService) Delete(ctx context.Context, id int64, a Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		v, err := s.R.Pages.Get(ctx, id)
		if err != nil {
			return err
		}
		if v.Status == PagePublished {
			return ErrConflict
		}
		if err := s.R.Pages.Delete(ctx, id); err != nil {
			return err
		}
		return s.audit(ctx, a, ActPageDeleted, id, map[string]any{"status": string(v.Status)})
	})
}

// Published returns the published version, if any.
func (s *PageService) Published(ctx context.Context) (PageVersion, bool, error) {
	v, err := s.R.Pages.Published(ctx)
	if errors.Is(err, ErrNotFound) {
		return PageVersion{}, false, nil
	}
	return v, err == nil, err
}

// PublishedFile returns a file of the published version; ErrNotFound when
// nothing is published or the file is missing.
func (s *PageService) PublishedFile(ctx context.Context, p string) (PageFile, error) {
	v, ok, err := s.Published(ctx)
	if err != nil {
		return PageFile{}, err
	}
	if !ok {
		return PageFile{}, ErrNotFound
	}
	return s.R.Pages.File(ctx, v.ID, p)
}

func (s *PageService) sign(id, exp string) string {
	m := hmac.New(sha256.New, s.Secret)
	m.Write([]byte(pagePreviewSalt + id + "." + exp))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// PreviewToken returns a bearer token that opens version id for 30 minutes.
func (s *PageService) PreviewToken(id int64) string {
	ids := strconv.FormatInt(id, 10)
	exp := strconv.FormatInt(s.Now().Add(PagePreviewTTL).Unix(), 10)
	return ids + "." + exp + "." + s.sign(ids, exp)
}

// PreviewFile returns a file of the version a valid, unexpired token names;
// any bad token → ErrNotFound.
func (s *PageService) PreviewFile(ctx context.Context, token, p string) (PageFile, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return PageFile{}, ErrNotFound
	}
	id, err1 := strconv.ParseInt(parts[0], 10, 64)
	exp, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil || !hmac.Equal([]byte(parts[2]), []byte(s.sign(parts[0], parts[1]))) ||
		!s.Now().Before(time.Unix(exp, 0)) {
		return PageFile{}, ErrNotFound
	}
	return s.R.Pages.File(ctx, id, p)
}
