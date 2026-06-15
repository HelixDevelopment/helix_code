package httpclient

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestNewTunedTransport_Tuning is the CONST-050 unit test for P1-T01.
// It asserts the factory returns a transport whose connection-pooling
// knobs actually fix R1 bottleneck B03 — the default http.Transport caps
// MaxIdleConnsPerHost at 2; the task requires >= 32.
func TestNewTunedTransport_Tuning(t *testing.T) {
	tr := NewTunedTransport()
	if tr == nil {
		t.Fatal("NewTunedTransport returned nil")
	}

	if tr.MaxIdleConnsPerHost < 32 {
		t.Errorf("MaxIdleConnsPerHost = %d; want >= 32 (B03 fix — Go default is 2)",
			tr.MaxIdleConnsPerHost)
	}
	if !tr.ForceAttemptHTTP2 {
		t.Error("ForceAttemptHTTP2 = false; want true (HTTP/2 hint required by P1-T01)")
	}
	if tr.MaxIdleConns < tr.MaxIdleConnsPerHost {
		t.Errorf("MaxIdleConns = %d; want >= MaxIdleConnsPerHost (%d)",
			tr.MaxIdleConns, tr.MaxIdleConnsPerHost)
	}
	if tr.IdleConnTimeout <= 0 {
		t.Errorf("IdleConnTimeout = %v; want a positive value", tr.IdleConnTimeout)
	}
	if tr.TLSHandshakeTimeout <= 0 {
		t.Errorf("TLSHandshakeTimeout = %v; want a positive value", tr.TLSHandshakeTimeout)
	}
	if tr.ExpectContinueTimeout <= 0 {
		t.Errorf("ExpectContinueTimeout = %v; want a positive value", tr.ExpectContinueTimeout)
	}
	if tr.DialContext == nil {
		t.Error("DialContext = nil; want a dialer with a bounded timeout")
	}
}

// TestNewTunedTransport_FreshInstances asserts each call returns a
// distinct transport — a transport owns its connection pool, so callers
// that want a shared pool must share the *http.Client.
func TestNewTunedTransport_FreshInstances(t *testing.T) {
	a := NewTunedTransport()
	b := NewTunedTransport()
	if a == b {
		t.Error("NewTunedTransport returned the same pointer twice; want fresh instances")
	}
}

// TestNewHTTPClient asserts the client carries the supplied timeout and a
// tuned (non-nil, *http.Transport) transport.
func TestNewHTTPClient(t *testing.T) {
	const want = 42 * time.Second
	c := NewHTTPClient(want)
	if c == nil {
		t.Fatal("NewHTTPClient returned nil")
	}
	if c.Timeout != want {
		t.Errorf("Timeout = %v; want %v", c.Timeout, want)
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport is %T; want *http.Transport", c.Transport)
	}
	if tr.MaxIdleConnsPerHost < 32 {
		t.Errorf("client transport MaxIdleConnsPerHost = %d; want >= 32",
			tr.MaxIdleConnsPerHost)
	}
}

// connCountingServer wraps an httptest server with a ConnState hook that
// counts every distinct TCP connection the server ever accepts. It is the
// wire-level instrumentation that makes connection reuse PROVABLE rather
// than merely asserted (CONST-035 — positive runtime evidence).
type connCountingServer struct {
	*httptest.Server
	mu       sync.Mutex
	seen     map[string]struct{}
	requests int
}

func newConnCountingServer() *connCountingServer {
	ccs := &connCountingServer{seen: make(map[string]struct{})}
	// Use NewUnstartedServer + Start so Config.ConnState is installed BEFORE the
	// accept loop begins. With httptest.NewServer the accept loop is already
	// running when we mutate srv.Config.ConnState, and net/http reads that field
	// from the serving goroutine concurrently with the test's write — a DATA
	// RACE flagged 100% under `go test -race`. Setting it pre-Start removes the
	// concurrent access while preserving the conn-reuse counting intent.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ccs.mu.Lock()
		ccs.requests++
		ccs.mu.Unlock()
		fmt.Fprintln(w, "ok")
	}))
	srv.Config.ConnState = func(c net.Conn, state http.ConnState) {
		if state == http.StateNew {
			ccs.mu.Lock()
			ccs.seen[c.RemoteAddr().String()] = struct{}{}
			ccs.mu.Unlock()
		}
	}
	srv.Start()
	ccs.Server = srv
	return ccs
}

// distinctConns returns how many separate TCP connections the server has
// accepted so far.
func (ccs *connCountingServer) distinctConns() int {
	ccs.mu.Lock()
	defer ccs.mu.Unlock()
	return len(ccs.seen)
}
