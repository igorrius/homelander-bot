package httpapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// SessionCookie holds the raw session token. The __Host- prefix makes the
// browser require Secure, Path=/ and no Domain.
const SessionCookie = "__Host-session"

// csrfHeader must accompany every state-changing API request; a cross-site
// form cannot set it.
const csrfHeader, csrfValue = "X-Requested-With", "homelander"

// Authenticator resolves a session token.
type Authenticator interface {
	Authenticate(ctx context.Context, raw string) (domain.User, domain.Session, error)
}

type userKey struct{}

// WithSession makes API requests carry the signed-in user, if any.
func (s *Server) WithSession(a Authenticator) { s.auth = a }

// ClientIP returns the caller's address; behind a trusted proxy, the rightmost
// X-Forwarded-For entry (the one the proxy appended).
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// CurrentUser returns the signed-in user of the request.
func CurrentUser(r *http.Request) (domain.User, bool) {
	u, ok := r.Context().Value(userKey{}).(domain.User)
	return u, ok
}

// RequireUser answers 401 unless someone is signed in.
func RequireUser(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := CurrentUser(r); !ok {
			WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		h.ServeHTTP(w, r)
	})
}

// RequireAdmin answers 401 when nobody is signed in and 403 for non-admins.
func RequireAdmin(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := CurrentUser(r)
		switch {
		case !ok:
			WriteError(w, http.StatusUnauthorized, "unauthorized")
		case !u.IsAdmin:
			WriteError(w, http.StatusForbidden, "forbidden")
		default:
			h.ServeHTTP(w, r)
		}
	})
}

// SetSessionCookie stores the session token in the browser.
func SetSessionCookie(w http.ResponseWriter, token string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: token, Path: "/", MaxAge: int(maxAge.Seconds()),
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

// sessionToken returns the raw token of the request's session cookie.
func sessionToken(r *http.Request) string {
	if c, err := r.Cookie(SessionCookie); err == nil {
		return c.Value
	}
	return ""
}

func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// prepareAPI checks the CSRF header and attaches the signed-in user. It
// reports false when it already answered.
func (s *Server) prepareAPI(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	if !safeMethod(r.Method) && r.Header.Get(csrfHeader) != csrfValue {
		WriteError(w, http.StatusForbidden, "csrf")
		return r, false
	}
	if s.auth == nil {
		return r, true
	}
	tok := sessionToken(r)
	if tok == "" {
		return r, true
	}
	u, _, err := s.auth.Authenticate(r.Context(), tok)
	switch {
	case err == nil:
		return r.WithContext(context.WithValue(r.Context(), userKey{}, u)), true
	case errors.Is(err, domain.ErrUnauthorized):
		ClearSessionCookie(w)
		return r, true
	default:
		s.deps.Log.Error("session check failed", "err", err)
		WriteError(w, http.StatusInternalServerError, "internal")
		return r, false
	}
}
