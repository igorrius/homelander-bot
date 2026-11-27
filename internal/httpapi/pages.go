package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// pageCSP replaces the global policy on page files: the page runs sandboxed
// (an opaque origin, so no cookies or session API), loads only its own files
// and our public data, and only our own pages may frame it.
const pageCSP = "sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation; " +
	"default-src 'self' data: blob:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'self'"

// publishedPageURL is where the shell frames the published page.
const publishedPageURL = "/site/index.html"

// SiteHandler serves page files: the published version at /site/{path} and a
// draft at /site/preview/{token}/{path}. Anything missing answers 404.
func SiteHandler(api API) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/site/")
		var (
			f       domain.PageFile
			err     error
			preview bool
		)
		if tail, ok := strings.CutPrefix(rest, "preview/"); ok {
			preview = true
			token, p, found := strings.Cut(tail, "/")
			if !found || token == "" {
				http.NotFound(w, r)
				return
			}
			f, err = api.Pages.PreviewFile(r.Context(), token, indexIfEmpty(p))
		} else {
			f, err = api.Pages.PublishedFile(r.Context(), indexIfEmpty(rest))
		}
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("Content-Security-Policy", pageCSP)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Type", f.ContentType)
		if preview {
			h.Set("Cache-Control", "private, no-store")
		} else {
			h.Set("Cache-Control", "no-cache")
			h.Set("ETag", `"`+f.SHA256+`"`)
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(f.Data))
	})
}

func indexIfEmpty(p string) string {
	if p == "" {
		return domain.PageIndex
	}
	return p
}

// pageURL is the published page's address for the home page shell, or nil.
func (h handlerCtx) pageURL(ctx context.Context) (any, error) {
	if h.api.Pages == nil {
		return nil, nil
	}
	_, ok, err := h.api.Pages.Published(ctx)
	if err != nil || !ok {
		return nil, err
	}
	return publishedPageURL, nil
}

// maxPageUpload caps a page zip upload.
const maxPageUpload = 25 << 20

func registerPages(h handlerCtx, routes map[string]http.HandlerFunc) {
	routes["GET /api/v1/admin/pages"] = h.listPages
	routes["POST /api/v1/admin/pages"] = h.uploadPage
	routes["GET /api/v1/admin/pages/{id}"] = h.getPage
	routes["DELETE /api/v1/admin/pages/{id}"] = h.deletePage
	routes["POST /api/v1/admin/pages/{id}/copy"] = h.copyPage
	routes["POST /api/v1/admin/pages/{id}/publish"] = h.publishPage
	routes["POST /api/v1/admin/pages/{id}/preview"] = h.previewPage
	routes["POST /api/v1/admin/pages/unpublish"] = h.unpublishPage
}

type pageFileDTO struct {
	Path        string `json:"path"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
}

type pageVersionDTO struct {
	ID          int64             `json:"id"`
	Status      domain.PageStatus `json:"status"`
	Note        string            `json:"note"`
	Files       []pageFileDTO     `json:"files"`
	TotalSize   int               `json:"totalSize"`
	CreatedAt   time.Time         `json:"createdAt"`
	CreatedBy   *userRef          `json:"createdBy"`
	CreatedVia  string            `json:"createdVia"`
	PublishedAt *time.Time        `json:"publishedAt"`
	PublishedBy *userRef          `json:"publishedBy"`
}

func toPageVersion(ctx context.Context, n *namer, v domain.PageVersion) pageVersionDTO {
	d := pageVersionDTO{ID: v.ID, Status: v.Status, Note: v.Note, Files: make([]pageFileDTO, 0, len(v.Files)),
		TotalSize: v.TotalSize, CreatedAt: v.CreatedAt, CreatedVia: v.CreatedVia, PublishedAt: v.PublishedAt}
	for _, f := range v.Files {
		d.Files = append(d.Files, pageFileDTO{f.Path, f.ContentType, f.Size})
	}
	ref := func(id *int64) *userRef {
		if id == nil {
			return nil
		}
		return &userRef{ID: *id, Name: n.userName(ctx, *id)}
	}
	d.CreatedBy, d.PublishedBy = ref(v.CreatedBy), ref(v.PublishedBy)
	return d
}

func (h handlerCtx) writePage(w http.ResponseWriter, r *http.Request, status int, v domain.PageVersion) {
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, status, toPageVersion(r.Context(), n, v))
}

func pageID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, domain.ErrNotFound
	}
	return id, nil
}

func (h handlerCtx) listPages(w http.ResponseWriter, r *http.Request) {
	vs, err := h.api.Pages.List(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	out := make([]pageVersionDTO, 0, len(vs))
	for _, v := range vs {
		out = append(out, toPageVersion(r.Context(), n, v))
	}
	WriteJSON(w, http.StatusOK, out)
}

func (h handlerCtx) getPage(w http.ResponseWriter, r *http.Request) {
	id, err := pageID(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	v, err := h.api.Pages.Get(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writePage(w, r, http.StatusOK, v)
}

// uploadPage stores a zip from the multipart field "zip" as a new draft.
func (h handlerCtx) uploadPage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPageUpload)
	// Parts past 8 MiB spill to temporary files; the body cap bounds them.
	if err := r.ParseMultipartForm(8 << 20); err != nil { //nolint:gosec // G120: the body is capped by MaxBytesReader above
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			h.fail(w, zipErr("zip", "too_large"))
			return
		}
		h.fail(w, errBadRequest)
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	f, hdr, err := r.FormFile("zip")
	if err != nil {
		h.fail(w, zipErr("zip", "required"))
		return
	}
	defer func() { _ = f.Close() }()
	files, err := readZip(f, hdr.Size)
	if err != nil {
		h.fail(w, err)
		return
	}
	v, err := h.api.Pages.CreateDraft(r.Context(), files, r.FormValue("note"), h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writePage(w, r, http.StatusCreated, v)
}

func (h handlerCtx) copyPage(w http.ResponseWriter, r *http.Request) {
	id, err := pageID(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	v, err := h.api.Pages.CopyToDraft(r.Context(), id, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writePage(w, r, http.StatusCreated, v)
}

func (h handlerCtx) publishPage(w http.ResponseWriter, r *http.Request) {
	id, err := pageID(r)
	if err == nil {
		err = h.api.Pages.Publish(r.Context(), id, h.actor(r))
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h handlerCtx) unpublishPage(w http.ResponseWriter, r *http.Request) {
	if err := h.api.Pages.Unpublish(r.Context(), h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h handlerCtx) deletePage(w http.ResponseWriter, r *http.Request) {
	id, err := pageID(r)
	if err == nil {
		err = h.api.Pages.Delete(r.Context(), id, h.actor(r))
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// previewPage mints a 30-minute link to a version for the admin's preview.
func (h handlerCtx) previewPage(w http.ResponseWriter, r *http.Request) {
	id, err := pageID(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	if _, err := h.api.Pages.Get(r.Context(), id); err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"url": "/site/preview/" + h.api.Pages.PreviewToken(id) + "/" + domain.PageIndex})
}
