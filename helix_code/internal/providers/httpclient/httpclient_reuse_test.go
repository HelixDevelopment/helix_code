package httpclient

import (
	"io"
	"net/http"
	"sync"
	"testing"
)

// drain reads and closes a response body so the underlying connection can
// be returned to the idle pool — without this, no connection is ever
// reusable regardless of transport tuning.
func drain(t testing.TB, resp *http.Response) {
	t.Helper()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("draining response body: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("closing response body: %v", err)
	}
}

// TestSharedClient_ReusesConnection is the CONST-050 integration test for
// P1-T01. It exercises the REAL factory client against a REAL local HTTP
// server (no mocks) and proves — via the server-side ConnState hook — that
// two sequential requests reuse a SINGLE TCP connection. With Go's default
// transport this would still be one connection for two strictly-sequential
// requests, so the test additionally drives a concurrent burst where the
// tuning (MaxIdleConnsPerHost >= 32) is what keeps the pool warm.
func TestSharedClient_ReusesConnection(t *testing.T) {
	srv := newConnCountingServer()
	defer srv.Close()

	client := NewHTTPClient(0) // shared client => shared connection pool

	// Two strictly-sequential requests must travel over ONE connection.
	for i := 0; i < 2; i++ {
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		drain(t, resp)
	}

	if got := srv.distinctConns(); got != 1 {
		t.Fatalf("sequential reuse: server saw %d distinct TCP connections; want 1", got)
	}
	t.Logf("sequential: %d requests over %d TCP connection(s) — reuse confirmed",
		srv.requests, srv.distinctConns())
}

// TestSharedClient_BurstReuse drives a concurrent burst through the shared
// tuned client and asserts the number of distinct TCP connections never
// exceeds the tuned per-host idle cap — i.e. the pool genuinely absorbs
// the burst instead of churning a fresh handshake per call. It then drains
// the pool and replays the burst, proving the warmed connections are
// reused on the second wave.
func TestSharedClient_BurstReuse(t *testing.T) {
	srv := newConnCountingServer()
	defer srv.Close()

	client := NewHTTPClient(0)

	const burst = 24

	runBurst := func() {
		var wg sync.WaitGroup
		errs := make(chan error, burst)
		for i := 0; i < burst; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := client.Get(srv.URL)
				if err != nil {
					errs <- err
					return
				}
				drain(t, resp)
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatalf("burst request failed: %v", err)
		}
	}

	// Wave 1 — cold pool. Concurrency forces several connections open but
	// the tuned cap keeps it bounded.
	runBurst()
	afterWave1 := srv.distinctConns()
	if afterWave1 > transportMaxIdleConnsPerHost {
		t.Fatalf("wave 1 opened %d connections; tuned cap is %d",
			afterWave1, transportMaxIdleConnsPerHost)
	}

	// Wave 2 — pool is warm. Sequential replay must reuse, opening ZERO
	// new connections. This is the anti-bluff proof of B03's fix.
	for i := 0; i < burst; i++ {
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatalf("wave 2 request %d failed: %v", i, err)
		}
		drain(t, resp)
	}
	afterWave2 := srv.distinctConns()

	if afterWave2 != afterWave1 {
		t.Fatalf("wave 2 opened %d new connection(s); want 0 (warm-pool reuse)",
			afterWave2-afterWave1)
	}
	t.Logf("burst: %d requests total over %d TCP connection(s); wave-2 sequential replay opened 0 new — warm-pool reuse confirmed",
		srv.requests, afterWave2)
}

// TestDefaultTransport_Churns is the contrast control: it runs the same
// sequential-burst-after-drain pattern through Go's DEFAULT transport
// (MaxIdleConnsPerHost = 2) and documents the connection count, so the
// before/after delta in the test log is concrete (CONST-035 Rule 9).
func TestDefaultTransport_Churns(t *testing.T) {
	srv := newConnCountingServer()
	defer srv.Close()

	// A bare client — exactly the pre-P1-T01 provider construction.
	client := &http.Client{}

	const burst = 24
	var wg sync.WaitGroup
	for i := 0; i < burst; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(srv.URL)
			if err != nil {
				return
			}
			drain(t, resp)
		}()
	}
	wg.Wait()

	t.Logf("default transport: %d concurrent requests over %d TCP connection(s) "+
		"(MaxIdleConnsPerHost default = 2 — idle conns above 2 are discarded and re-handshaked)",
		srv.requests, srv.distinctConns())
}
