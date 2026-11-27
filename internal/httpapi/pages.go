package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
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
