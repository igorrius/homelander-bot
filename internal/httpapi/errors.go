package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/igorrius/homelander-bot/internal/domain"
)

const maxJSONBody = 64 << 10

// errBadRequest marks a body that is not the expected JSON.
var errBadRequest = errors.New("bad_request")

// decode reads a JSON body of at most 64 KiB, refusing unknown fields.
func decode(r *http.Request, v any) error { return decodeLimit(r, v, maxJSONBody) }

// decodeLimit is decode with a body cap of n bytes.
func decodeLimit(r *http.Request, v any, n int64) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, n))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errBadRequest
	}
	return nil
}

// writeDomainError maps service errors to API error responses.
func writeDomainError(w http.ResponseWriter, log *slog.Logger, err error) {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		body := map[string]any{"code": "validation", "fields": ve.Fields}
		if len(ve.Details) > 0 {
			body["details"] = ve.Details
		}
		WriteJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": body})
	case errors.Is(err, errBadRequest):
		WriteError(w, http.StatusBadRequest, "bad_request")
	case errors.Is(err, domain.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, domain.ErrLastAdmin):
		WriteError(w, http.StatusConflict, "last_admin")
	case errors.Is(err, domain.ErrFirstAdmin):
		WriteError(w, http.StatusConflict, "first_admin")
	case errors.Is(err, domain.ErrConflict):
		WriteError(w, http.StatusConflict, "conflict")
	case errors.Is(err, domain.ErrForbidden):
		WriteError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, domain.ErrRateLimited):
		WriteError(w, http.StatusTooManyRequests, "rate_limited")
	case errors.Is(err, domain.ErrTryLater):
		WriteError(w, http.StatusServiceUnavailable, "try_later")
	case errors.Is(err, domain.ErrInvalidCode):
		WriteError(w, http.StatusUnauthorized, "invalid_code")
	case errors.Is(err, domain.ErrTokenInvalid):
		WriteError(w, http.StatusUnprocessableEntity, "token_invalid")
	case errors.Is(err, domain.ErrChatNotAccepted):
		WriteError(w, http.StatusConflict, "chat_not_accepted")
	case errors.Is(err, domain.ErrModuleOff):
		WriteError(w, http.StatusNotFound, "module_disabled")
	case errors.Is(err, domain.ErrOwned):
		WriteError(w, http.StatusConflict, "module_owned")
	case errors.Is(err, domain.ErrUnauthorized):
		WriteError(w, http.StatusUnauthorized, "unauthorized")
	default:
		log.Error("api error", "err", err)
		WriteError(w, http.StatusInternalServerError, "internal")
	}
}
