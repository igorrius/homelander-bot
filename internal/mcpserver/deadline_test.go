package mcpserver_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/mcpserver"
)

// slow answers after the server's write timeout has passed.
var slow = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	time.Sleep(300 * time.Millisecond)
	_, _ = w.Write([]byte("ok"))
})

func TestMCPNotCutByWriteTimeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpserver.LiftWriteDeadline(slow))
	mux.Handle("/other", slow)
	srv := httptest.NewUnstartedServer(mux)
	srv.Config.WriteTimeout = 100 * time.Millisecond
	srv.Start()
	defer srv.Close()

	res, err := http.Get(srv.URL + "/mcp")
	if err != nil {
		t.Fatalf("/mcp cut by the write timeout: %v", err)
	}
	body, err := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if err != nil || string(body) != "ok" {
		t.Errorf("/mcp body = %q, %v", body, err)
	}

	if res, err := http.Get(srv.URL + "/other"); err == nil {
		b, rerr := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if rerr == nil && string(b) == "ok" {
			t.Error("other routes should still be cut by the write timeout")
		}
	}
}
