package domain

import (
	"errors"
	"sort"
	"strconv"
	"strings"
)

// Errors that the HTTP layer maps to status codes. Their texts are the API
// error codes.
var (
	ErrNotFound     = errors.New("not_found")
	ErrConflict     = errors.New("conflict")
	ErrForbidden    = errors.New("forbidden")
	ErrRateLimited  = errors.New("rate_limited")
	ErrTryLater     = errors.New("try_later")
	ErrInvalidCode  = errors.New("invalid_code")
	ErrUnauthorized = errors.New("unauthorized")
	ErrLastAdmin    = errors.New("last_admin")
	ErrFirstAdmin   = errors.New("first_admin")
	ErrTokenInvalid = errors.New("token_invalid")
	// ErrLinkOtherUser: a login link was opened in a browser signed in as
	// someone else; the link is still unspent.
	ErrLinkOtherUser = errors.New("link_other_user")
)

// ValidationError maps field names to reason codes such as "required",
// "invalid", "out_of_range" or "taken".
// Details, when set, says more for one problem, e.g. {"path": "x.ico"} for
// the page file that was refused.
type ValidationError struct {
	Fields  map[string]string
	Details map[string]string
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for f, r := range e.Fields {
		parts = append(parts, f+": "+r)
	}
	sort.Strings(parts)
	msg := "validation: " + strings.Join(parts, ", ")
	if p := e.Details["path"]; p != "" {
		msg += " (path " + strconv.Quote(p) + ")"
	}
	return msg
}

// validation collects field problems; err returns nil when there are none.
type validation map[string]string

func (v validation) add(field, reason string) {
	if _, ok := v[field]; !ok {
		v[field] = reason
	}
}

func (v validation) err() error {
	if len(v) == 0 {
		return nil
	}
	return &ValidationError{Fields: v}
}
