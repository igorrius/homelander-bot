package httpapi

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// newSPA serves files of the built web app. Paths without a file extension
// that match no file get index.html, so client-side routes survive a reload.
func newSPA(static fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if static == nil {
			notBuilt(w)
			return
		}
		index, err := fs.ReadFile(static, "index.html")
		if err != nil {
			notBuilt(w)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if st, err := fs.Stat(static, name); err == nil && !st.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "no-cache")
				}
				http.ServeFileFS(w, r, static, name)
				return
			}
			if path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}

func notBuilt(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("web app is not built: run `make web`\n"))
}
