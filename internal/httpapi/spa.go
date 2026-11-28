package httpapi

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// staticTypes covers extensions missing from Go's built-in MIME table; the
// distroless image has no /etc/mime.types to fill the gap.
var staticTypes = map[string]string{
	".ico":         "image/x-icon",
	".webmanifest": "application/manifest+json",
}

// fileExts are extensions of files a browser asks for; a missing one is a 404,
// not the web app.
var fileExts = map[string]bool{
	".js": true, ".mjs": true, ".css": true, ".map": true, ".json": true, ".webmanifest": true, ".html": true, ".txt": true,
	".xml": true, ".ico": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true,
	".avif": true, ".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".pdf": true, ".php": true, ".env": true,
}

// newSPA serves files of the built web app. Paths that match no file and do
// not look like a file get index.html, so client-side routes survive a reload.
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
		if name != "" && name != "index.html" && fs.ValidPath(name) {
			if st, err := fs.Stat(static, name); err == nil && !st.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "no-cache")
				}
				if ct, ok := staticTypes[path.Ext(name)]; ok {
					w.Header().Set("Content-Type", ct)
				}
				// name is cleaned, rooted and checked by fs.ValidPath; an fs.FS cannot
				// reach outside its root anyway.
				http.ServeFileFS(w, r, static, name) //nolint:gosec // G703: see above

				return
			}
			// A missing file is 404. Dotted paths of the admin panel and the
			// cabinet are app routes whatever they end with (a data key such
			// as /admin/data/rules.txt); elsewhere only file-like ones are 404.
			appRoute := strings.HasPrefix(name, "admin/") || strings.HasPrefix(name, "my/")
			if !appRoute && (fileExts[strings.ToLower(path.Ext(name))] || strings.HasPrefix(name, "assets/")) {
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
