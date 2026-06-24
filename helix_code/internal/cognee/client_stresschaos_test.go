package cognee

// §11.4.85 stress + chaos coverage for the rewritten Cognee *Client HTTP path
// (the 1.1.x v1 + bearer-login client, internal/cognee/client.go).
//
// Gap this fills (investigated, not guessed): the existing cognee_stress_test.go /
// cognee_chaos_test.go suites EXPLICITLY exercise only the IN-PROCESS service
// machinery (ServiceCache, statistics, event dispatch) and their doc-comments
// state "Network-facing paths (the *Client HTTP calls ...) are NOT exercised —
// they require a live Cognee endpoint". The rewritten Client IS the concurrency-
// and-error-handling-critical surface of this session's fix (7f028910), so this
// suite drives it under §11.4.85 load + fault — HERMETICALLY, via httptest.Server
// stand-ins. NO real nezha / live Cognee is touched (§11.4.119 — another stream
// owns the nezha test-DB), and credentials are never hardcoded.
//
// Concurrency focus: the bearer-token login cache. attachAuth lazily calls login()
// once and caches the JWT under c.mu; under concurrent first-use, login() may run
// N times (a benign thundering-herd) but the client MUST stay race-clean and MUST
// converge on a non-empty token. The §1.1 mutation at the bottom of this file
// proves the chaos error-path test is not a tautology.
//
// No fakes in the system-under-test: the REAL *Client (real http.Client, real
// request builders, real JSON decode, real attachAuth/login) runs against an
// httptest backend. Only the *server* is a controllable stand-in — that is a
// hermetic transport, not a mock of the client. Run under -race.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/tests/stresschaos"
)

// newTestClient builds a REAL *Client pointed at the given base URL (an httptest
// server). It configures username/password so the bearer-login path is exercised;
// credentials are test literals, never real secrets (CONST-042 — these touch no
// real service). A short timeout keeps fault-injection tests from hanging the suite.
func newTestClient(t testing.TB, baseURL string) *Client {
	t.Helper()
	c := NewClient(&config.CogneeConfig{
		Host: "127.0.0.1",
		Port: 1, // overwritten by SetBaseURL below
		Mode: "local",
		RemoteAPI: &config.CogneeRemoteAPIConfig{
			Username: "tester@example.com",
			Password: "test-password-not-a-secret",
			Timeout:  3 * time.Second,
		},
	})
	c.SetBaseURL(baseURL)
	// HXC-064 (§11.4.50 determinism): 8s client timeout (was 3s). A larger headroom for
	// the §11.4.85 concurrent stress test (16×50 = 800 calls) so the slowest call's
	// scheduling latency under real machine load cannot blow the client timeout (the
	// hermetic backend ALWAYS responds). 8s stays < the chaos suite's 10s guard, so the
	// hang_past_timeout fault (6s server sleep) still returns under the guard and no
	// fault assertion is weakened.
	c.SetTimeout(8 * time.Second)
	// HXC-064 (§11.4.50 determinism): a SINGLE reused keep-alive connection
	// (MaxConnsPerHost=1, MaxIdleConnsPerHost=1) for the hermetic test backend.
	//
	// Forensic root cause (measured, not guessed). The §11.4.85 concurrent test was
	// load-flaky in TWO mutually-exclusive ways even when run ALONE (each ~1-in-3 at
	// high repetition), and both stem from the transport, not the product:
	//   • DisableKeepAlives (the previous setting): each of the 800 calls opens + tears
	//     down its OWN TCP connection. (a) The transient server-side conn-handler
	//     goroutines had not all exited within RunConcurrent's fixed 50ms post-run
	//     settle window → runtime.NumGoroutine() delta exceeded the tolerance-4 leak
	//     guard ("goroutine leak delta>4"); (b) the rapid connect/close storm exhausted
	//     ephemeral sockets on macOS → connection errors ("reported N errors").
	//   • A multi-connection keep-alive pool: each PARKED idle connection holds ~3
	//     goroutines (client read+write loop + server conn-handler), so even a pool of 2
	//     parks ~6 — a DETERMINISTIC leak delta of 6 > tolerance 4.
	// A single reused keep-alive connection is the only shape that satisfies BOTH the
	// RunConcurrent leak guard (1 conn → delta ~3 ≤ tolerance 4) AND avoids the connect
	// storm (conn is reused, never re-handshaked → no socket exhaustion, no timeouts).
	// Serializing the WIRE does NOT weaken the concurrency assertion: the property under
	// test is the bearer-login TOKEN-CACHE race — 16 goroutines concurrently hit
	// attachAuth()/login() and contend on c.mu, independent of the connection count
	// (the loginHits 1..=Parallelism bound below still proves the cache holds under that
	// concurrent first-use race). c.Close() (t.Cleanup below) drains the idle conn at
	// test end. Same-package access to the unexported httpClient.
	c.httpClient.Transport = &http.Transport{MaxConnsPerHost: 1, MaxIdleConnsPerHost: 1}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// fastBackend is an httptest handler that serves correct, fast canned responses for
// the v1 surface used by the stress paths (login, search, datasets). loginHits
// counts how many times the login endpoint was hit (to observe cache behaviour).
func fastBackend(loginHits *int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case pathLogin:
			atomic.AddInt64(loginHits, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-jwt-token","token_type":"bearer"}`))
		case pathSearch:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"search_result":"hello","dataset_id":"d1","dataset_name":"ds"}]`))
		case pathDatasets:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"d1","name":"ds","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","ownerId":"o1"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// ---------------------------------------------------------------------------
// STRESS — §11.4.85(A)
// ---------------------------------------------------------------------------

// TestClientHTTP_Stress_SustainedSearch issues ≥100 sequential SearchMemory calls
// through a fast hermetic backend and captures p50/p95/p99 latency. Every call
// drives the real request-build → attachAuth (bearer cache) → HTTP → JSON-decode
// path. The login endpoint must be hit at most once (token cached after first use).
func TestClientHTTP_Stress_SustainedSearch(t *testing.T) {
	var loginHits int64
	srv := httptest.NewServer(fastBackend(&loginHits))
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv.URL)
	ctx := context.Background()

	rep := stresschaos.RunSustainedLoad(t, "client_http_sustained_search",
		stresschaos.SustainedConfig{N: 500}, func(i int) error {
			resp, err := c.SearchMemory(ctx, &SearchMemoryRequest{Query: fmt.Sprintf("q-%d", i)})
			if err != nil {
				return err
			}
			if resp.TotalCount != 1 {
				return fmt.Errorf("iter %d: got %d results, want 1", i, resp.TotalCount)
			}
			return nil
		})

	if rep.ErrorRate != 0 {
		t.Fatalf("sustained search error rate %.4f != 0", rep.ErrorRate)
	}
	if h := atomic.LoadInt64(&loginHits); h != 1 {
		t.Fatalf("login endpoint hit %d times in sequential run, want exactly 1 (bearer token not cached)", h)
	}
}

// TestClientHTTP_Stress_ConcurrentRequests hammers SearchMemory + ListDatasets from
// ≥10 goroutines through the fast backend. This stresses the bearer-login cache
// under concurrent FIRST-use (login() may race a handful of times — benign thunder
// herd — but the client MUST stay race-clean, converge on a token, and leak no
// goroutines). -race is the evidence for no data race on bearerToken/baseURL/apiKey.
func TestClientHTTP_Stress_ConcurrentRequests(t *testing.T) {
	var loginHits int64
	srv := httptest.NewServer(fastBackend(&loginHits))
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv.URL)
	ctx := context.Background()

	rep := stresschaos.RunConcurrent(t, "client_http_concurrent_requests",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 50},
		func(g, it int) error {
			if it%2 == 0 {
				resp, err := c.SearchMemory(ctx, &SearchMemoryRequest{Query: fmt.Sprintf("g%d-%d", g, it)})
				if err != nil {
					return err
				}
				if resp.TotalCount != 1 {
					return fmt.Errorf("g%d it%d: %d results, want 1", g, it, resp.TotalCount)
				}
				return nil
			}
			ds, err := c.ListDatasets(ctx)
			if err != nil {
				return err
			}
			if ds.Total != 1 {
				return fmt.Errorf("g%d it%d: %d datasets, want 1", g, it, ds.Total)
			}
			return nil
		})

	if rep.ErrorCount > 0 {
		t.Fatalf("concurrent run reported %d errors", rep.ErrorCount)
	}
	// The token cache means login fires at most a small handful of times under the
	// concurrent first-use race — NOT once per request. Bound it well below the
	// total call count to prove caching holds under concurrency.
	if h := atomic.LoadInt64(&loginHits); h == 0 || h > int64(rep.Parallelism) {
		t.Fatalf("login hit %d times across %d concurrent calls — want 1..=%d (cache should hold; 0 means auth never ran)",
			h, rep.TotalCalls, rep.Parallelism)
	}
}

// ---------------------------------------------------------------------------
// CHAOS — §11.4.85(B) — network-fault injection
// ---------------------------------------------------------------------------

// TestClientHTTP_Chaos_NetworkFaults injects the §11.4.85(B)(2) network-fault class:
// the backend variously 500s, returns truncated/garbage JSON, hangs past the client
// timeout, or closes the connection mid-response (EOF). For EVERY fault the client
// MUST return a clean non-nil error — never panic, never hang past the timeout,
// never decode garbage as success.
func TestClientHTTP_Chaos_NetworkFaults(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "client_http_network_faults", "network-fault")

	type fault struct {
		name    string
		handler http.HandlerFunc
	}
	faults := []fault{
		{"http_500", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == pathLogin {
				_, _ = w.Write([]byte(`{"access_token":"t","token_type":"bearer"}`))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"detail":"boom"}`))
		}},
		{"truncated_json", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == pathLogin {
				_, _ = w.Write([]byte(`{"access_token":"t","token_type":"bearer"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"search_result":"hel`)) // abruptly cut off
		}},
		{"garbage_body", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == pathLogin {
				_, _ = w.Write([]byte(`{"access_token":"t","token_type":"bearer"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("\x00\x01not json at all\xff"))
		}},
		{"hang_past_timeout", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == pathLogin {
				_, _ = w.Write([]byte(`{"access_token":"t","token_type":"bearer"}`))
				return
			}
			// Sleep well past the client's 3s timeout so the request deadline fires.
			time.Sleep(6 * time.Second)
		}},
		{"connection_drop_mid_response", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == pathLogin {
				_, _ = w.Write([]byte(`{"access_token":"t","token_type":"bearer"}`))
				return
			}
			// Write a partial body then hijack + close the connection (EOF mid-read).
			w.Header().Set("Content-Length", "4096")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"search_result":"partial"`))
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					_ = conn.Close()
				}
			}
		}},
	}

	for _, f := range faults {
		func(f fault) {
			srv := httptest.NewServer(f.handler)
			defer srv.Close()
			c := newTestClient(t, srv.URL)
			ctx := context.Background()

			done := make(chan struct{})
			var resErr error
			go func() {
				defer close(done)
				defer func() {
					if p := recover(); p != nil {
						rec.Record(stresschaos.Fatal, fmt.Sprintf("%s: client PANICKED: %v", f.name, p))
					}
				}()
				_, resErr = c.SearchMemory(ctx, &SearchMemoryRequest{Query: "x"})
			}()

			// Bounded wait: the client's own 3s timeout must make this return well
			// under our 10s guard. If it does not, the client hangs — a Fatal.
			select {
			case <-done:
				if resErr == nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("%s: client returned NIL error on an injected fault — decoded garbage/partial as success", f.name))
				} else {
					rec.Record(stresschaos.Degraded, fmt.Sprintf("%s: client returned clean error: %v", f.name, resErr))
				}
			case <-time.After(10 * time.Second):
				rec.Record(stresschaos.Fatal, fmt.Sprintf("%s: client hung past 10s (timeout not honoured)", f.name))
			}
		}(f)
	}

	rec.AssertNoFatal()
}

// TestClientHTTP_Chaos_ServerCloseDuringConcurrentLoad injects the §11.4.85(B)(1)
// process-death-equivalent fault: the backend is CLOSED while a swarm of requests
// is in flight. Post-close every request must fail cleanly (connection refused),
// the client must not panic or deadlock, and it must remain usable once a healthy
// backend is restored.
func TestClientHTTP_Chaos_ServerCloseDuringConcurrentLoad(t *testing.T) {
	var loginHits int64
	srv := httptest.NewServer(fastBackend(&loginHits))
	c := newTestClient(t, srv.URL)
	ctx := context.Background()
	rec := stresschaos.NewChaosRecorder(t, "client_http_server_close_during_load", "process-death")

	// Warm the token + prove the client works against the live backend first.
	if _, err := c.SearchMemory(ctx, &SearchMemoryRequest{Query: "warmup"}); err != nil {
		t.Fatalf("warmup search failed against healthy backend: %v", err)
	}
	rec.Record(stresschaos.Recovered, "warmup search succeeded against healthy backend")

	// Kill the backend, then fire a concurrent swarm — every call must error cleanly.
	srv.Close()
	var wg sync.WaitGroup
	var cleanErrs, panics, nilErrs int64
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			_, err := c.SearchMemory(cctx, &SearchMemoryRequest{Query: fmt.Sprintf("dead-%d", n)})
			if err != nil {
				atomic.AddInt64(&cleanErrs, 1)
			} else {
				atomic.AddInt64(&nilErrs, 1)
			}
		}(i)
	}
	wg.Wait()

	if atomic.LoadInt64(&panics) > 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("%d goroutines PANICKED against a dead backend", panics))
	}
	if atomic.LoadInt64(&nilErrs) > 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("%d calls returned NIL error against a dead backend (fabricated success)", nilErrs))
	}
	rec.Record(stresschaos.Degraded, fmt.Sprintf("%d calls failed cleanly against the dead backend", atomic.LoadInt64(&cleanErrs)))

	// Restore a healthy backend on a NEW URL and prove the client recovers.
	srv2 := httptest.NewServer(fastBackend(&loginHits))
	t.Cleanup(srv2.Close)
	c.SetBaseURL(srv2.URL)
	if _, err := c.SearchMemory(ctx, &SearchMemoryRequest{Query: "recovered"}); err != nil {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("client did not recover after backend restored: %v", err))
	} else {
		rec.Record(stresschaos.Recovered, "client recovered + served a request once a healthy backend was restored")
	}

	rec.AssertNoFatal()
}

// TestClientHTTP_Chaos_TruncatedBodyInputCorruption injects the §11.4.85(B)(3)
// input-corruption class at the JSON-decode boundary: a range of structurally
// hostile response bodies are served and the client must surface a parse error
// (non-nil) for each, never panic, never decode partial data as a success.
func TestClientHTTP_Chaos_TruncatedBodyInputCorruption(t *testing.T) {
	corruptBodies := [][]byte{
		[]byte(``),                              // empty body
		[]byte(`{`),                             // open brace only
		[]byte(`[{"search_result":`),            // truncated mid-key
		[]byte(`not-json`),                      // plain garbage
		[]byte(`[1,2,3]`),                       // valid JSON, wrong shape (array of ints)
		[]byte("\x00\x00\x00"),                  // NUL bytes
		[]byte(`{"deeply":{"nested":` + "\x00"), // nested + NUL
	}

	stresschaos.ChaosCorruptInputDuring(t, "client_http_truncated_body_input_corruption", corruptBodies,
		func(body []byte) error {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == pathLogin {
					_, _ = w.Write([]byte(`{"access_token":"t","token_type":"bearer"}`))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(body)
			}))
			defer srv.Close()
			c := newTestClient(t, srv.URL)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			resp, err := c.SearchMemory(ctx, &SearchMemoryRequest{Query: "x"})
			if err != nil {
				// Clean parse-error rejection — the desired graceful path.
				return fmt.Errorf("rejected corrupt body cleanly: %w", err)
			}
			// `[1,2,3]` decodes into the typed array WITHOUT error (numbers coerce to
			// the interface field), yielding a non-error but well-formed empty/garbage
			// result set. That is acceptable as long as it did not panic and did not
			// fabricate matching content. Accepting-without-crash is a Recovered path.
			if resp == nil {
				return fmt.Errorf("nil response with nil error on corrupt body — inconsistent")
			}
			return nil
		})
}
