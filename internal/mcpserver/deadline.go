package mcpserver

import (
	"net/http"
	"time"
)

// LiftWriteDeadline clears the server's per-request write deadline (the
// global WriteTimeout) for an MCP request: a tool call or a streamed answer
// may legitimately take longer. Other routes keep the server's limit.
func LiftWriteDeadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Not every ResponseWriter supports deadlines (httptest recorders);
		// then there is no deadline to lift either.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
		next.ServeHTTP(w, r)
	})
}
