// Package httpapi serves the JSON API, the health check and the web app.
package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/igorrius/homelander-bot/internal/config"
	"github.com/igorrius/homelander-bot/internal/i18n"
)

// Deps are the server's collaborators.
type Deps struct {
	DB     *sql.DB
	I18n   *i18n.Bundle
	Log    *slog.Logger
	Static fs.FS
}

// Server routes requests to the API, the health check and the web app.
type Server struct {
	cfg  config.Config
	deps Deps
	api    *http.ServeMux
	spa    http.Handler
	mounts map[string]http.Handler
	auth   Authenticator
}

// New builds a server. API routes are registered later on API().
func New(cfg config.Config, d Deps) *Server {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Server{cfg: cfg, deps: d, api: http.NewServeMux(), spa: newSPA(d.Static), mounts: map[string]http.Handler{}}
}

// API returns the mux for JSON routes. Patterns are full paths, e.g.
// "POST /api/v1/register".
func (s *Server) API() *http.ServeMux { return s.api }

// Mount serves h at exactly path (e.g. the Telegram webhook). Other paths
// under /telegram/ answer 404 instead of the web app.
func (s *Server) Mount(path string, h http.Handler) { s.mounts[path] = h }

// Handler returns the server's root handler with middleware applied.
func (s *Server) Handler() http.Handler {
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch p := r.URL.Path; {
		case p == "/healthz":
			s.healthz(w, r)
		case p == "/api" || strings.HasPrefix(p, "/api/"):
			s.serveAPI(w, r)
		case s.mounts[p] != nil:
			s.mounts[p].ServeHTTP(w, r)
		case strings.HasPrefix(p, "/telegram/"):
			http.NotFound(w, r)
		default:
			s.spa.ServeHTTP(w, r)
		}
	})
	return securityHeaders(logRequests(s.deps.Log, recoverPanics(s.deps.Log, root)))
}

func (s *Server) serveAPI(w http.ResponseWriter, r *http.Request) {
	// Handler only tells whether a route matches; ServeHTTP is what fills in
	// r.Pattern and the path values the route's handler reads.
	if _, pattern := s.api.Handler(r); pattern == "" {
		WriteError(w, http.StatusNotFound, "not_found")
		return
	}
	r, ok := s.prepareAPI(w, r)
	if !ok {
		return
	}
	s.api.ServeHTTP(w, r)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if s.deps.DB == nil || s.deps.DB.PingContext(ctx) != nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes {"error":{"code":code}} with the given status.
func WriteError(w http.ResponseWriter, status int, code string) {
	WriteJSON(w, status, map[string]any{"error": map[string]string{"code": code}})
}
