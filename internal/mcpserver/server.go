// Package mcpserver serves Homelander's MCP endpoint: tools through which an
// admin's AI agent manages the data space and writes home-page drafts. Every
// request carries an OAuth access token from domain.McpAuthService; the tools
// act as that admin, marked "via MCP" in the audit log.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/ratelimit"
)

// Deps are the services the tools call.
type Deps struct {
	Auth      *domain.McpAuthService
	Data      *domain.DataService
	Pages     *domain.PageService
	Catalog   *domain.CatalogService
	Limits    *ratelimit.Limiter
	PublicURL string
	Now       domain.Clock
	Log       *slog.Logger // nil discards
}

const (
	rateLimit   = 300
	rateWindow  = time.Minute
	maxBodySize = 32 << 20 // a page draft of 20 MiB in base64, plus JSON
)

const instructions = `Homelander is the website and bot of one resident community. Through these tools you act as the admin who connected you.
- The data space holds named values and tables (keys like "house.address"); public items are readable by the site pages.
- Before writing or changing the site (index.html is the home page, every other root *.html is a page at /<name>), call page_guide (also the resource homelander://guide/page): the page runs in a sandbox with strict rules.
- You can only write drafts. An admin previews and publishes them in the admin panel; give them the admin_url you get back.
- Everything you change is recorded in the audit log as the admin, via MCP.`

// New returns the /mcp handler: bearer-token auth, a per-grant rate limit,
// and the MCP Streamable HTTP transport (stateless, JSON responses).
func New(d Deps) http.Handler {
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "homelander", Title: "Homelander", Version: "1"},
		&mcp.ServerOptions{Instructions: instructions})
	t := tools{d}
	t.addData(s)
	t.addPages(s)
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: maxBodySize})
	verify := func(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		p, err := d.Auth.Verify(ctx, token)
		if errors.Is(err, domain.ErrUnauthorized) {
			return nil, fmt.Errorf("%w: token is not valid or the admin lost access", auth.ErrInvalidToken)
		}
		if err != nil {
			d.Log.Error("mcp token check failed", "err", err)
			return nil, errors.New("internal error")
		}
		return &auth.TokenInfo{UserID: strconv.FormatInt(p.User.ID, 10), Expiration: p.ExpiresAt,
			Extra: map[string]any{"principal": p}}, nil
	}
	base := strings.TrimRight(d.PublicURL, "/")
	return LiftWriteDeadline(auth.RequireBearerToken(verify, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: base + "/.well-known/oauth-protected-resource/mcp",
	})(limit(d, h)))
}

// limit allows rateLimit requests per minute per grant.
func limit(d Deps, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := principalOf(auth.TokenInfoFromContext(r.Context())); ok && d.Limits != nil &&
			!d.Limits.Allow("mcp:"+strconv.FormatInt(p.Grant.ID, 10), rateLimit, rateWindow) {
			w.Header().Set("Retry-After", "60")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32000,"message":"rate limited: at most 300 requests a minute"}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func principalOf(ti *auth.TokenInfo) (domain.MCPPrincipal, bool) {
	if ti == nil {
		return domain.MCPPrincipal{}, false
	}
	p, ok := ti.Extra["principal"].(domain.MCPPrincipal)
	return p, ok
}

type tools struct{ d Deps }

// actor is who a tool call acts as: the admin behind the token, via MCP.
func actor(req *mcp.CallToolRequest) (domain.Actor, error) {
	var ti *auth.TokenInfo
	if req != nil && req.Extra != nil {
		ti = req.Extra.TokenInfo
	}
	p, ok := principalOf(ti)
	if !ok {
		return domain.Actor{}, errors.New("not authorized")
	}
	id := p.User.ID
	return domain.Actor{UserID: &id, Via: "mcp", Client: p.Grant.ClientName}, nil
}

// toolError turns a domain error into the readable text of a tool error.
func (t tools) toolError(err error) error {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		parts := make([]string, 0, len(ve.Fields))
		for f, reason := range ve.Fields {
			parts = append(parts, f+": "+reason)
		}
		sort.Strings(parts)
		msg := "validation failed: " + strings.Join(parts, ", ")
		if len(ve.Details) > 0 {
			keys := make([]string, 0, len(ve.Details))
			for k := range ve.Details {
				keys = append(keys, k+"="+ve.Details[k])
			}
			sort.Strings(keys)
			msg += " (" + strings.Join(keys, ", ") + ")"
		}
		return errors.New(msg)
	case errors.Is(err, domain.ErrNotFound):
		return errors.New("not found")
	case errors.Is(err, domain.ErrConflict):
		return errors.New("conflict: the item is in a state that does not allow this (e.g. a published page version cannot be edited)")
	case errors.Is(err, domain.ErrRateLimited):
		return errors.New("rate limited")
	case errors.Is(err, errInput):
		return err
	}
	t.d.Log.Error("mcp tool failed", "err", err)
	return errors.New("internal error")
}

// errInput marks a problem with the tool arguments themselves.
var errInput = errors.New("invalid input")

func inputError(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errInput, fmt.Sprintf(format, a...))
}
