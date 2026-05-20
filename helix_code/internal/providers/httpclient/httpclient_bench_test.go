package httpclient

import (
	"io"
	"net/http"
	"sync"
	"testing"
)

// benchBurst issues `burst` sequential GETs through the supplied client
// against a fresh connection-counting server and reports two custom
// metrics: requests-per-TCP-connection (higher = better reuse) and the
// absolute distinct-connection count for the run.
func benchBurst(b *testing.B, client *http.Client, burst int) {
	b.Helper()
	for i := 0; i < b.N; i++ {
		srv := newConnCountingServer()
		for j := 0; j < burst; j++ {
			resp, err := client.Get(srv.URL)
			if err != nil {
				srv.Close()
				b.Fatalf("request failed: %v", err)
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		conns := srv.distinctConns()
		if i == b.N-1 {
			b.ReportMetric(float64(burst)/float64(conns), "reqs/conn")
			b.ReportMetric(float64(conns), "conns/op")
		}
		client.CloseIdleConnections()
		srv.Close()
	}
}

// BenchmarkTunedClient_Burst measures a burst through the P1-T01 shared
// tuned client. The "reqs/conn" custom metric is the speedup signal —
// for a strictly-sequential burst it should be burst:1 (all requests on
// one warm connection).
func BenchmarkTunedClient_Burst(b *testing.B) {
	benchBurst(b, NewHTTPClient(0), 32)
}

// BenchmarkDefaultClient_Burst is the before-state control: the same burst
// through Go's default transport. Comparing the two benchmarks' "reqs/conn"
// and "conns/op" metrics is the anti-bluff before/after evidence for B03.
func BenchmarkDefaultClient_Burst(b *testing.B) {
	benchBurst(b, &http.Client{}, 32)
}

// concurrentWave fires `width` requests concurrently and returns once all
// have completed and their bodies are drained (so connections can return
// to the idle pool).
func concurrentWave(b *testing.B, client *http.Client, url string, width int) {
	var wg sync.WaitGroup
	for j := 0; j < width; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(url)
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}()
	}
	wg.Wait()
}

// benchConcurrentWaves is the burst that exposes R1 B03. It drives
// `waves` successive CONCURRENT waves of `width` requests against one
// server — the real provider pattern under rapid / parallel LLM calls.
//
// Go's default transport caps idle conns per host at 2: each wave leaves
// only 2 connections pooled, so every subsequent wave must re-open
// (width-2) connections and pay (width-2) fresh TCP+TLS handshakes. The
// tuned transport pools up to MaxIdleConnsPerHost (64), so after wave 1
// all `width` connections stay warm and waves 2..N reuse them entirely.
//
// The reported "total-conns/op" is the anti-bluff before/after signal:
// default ≈ width*waves-ish churn, tuned ≈ width (one pool, reused).
func benchConcurrentWaves(b *testing.B, client *http.Client, width, waves int) {
	b.Helper()
	for i := 0; i < b.N; i++ {
		srv := newConnCountingServer()
		for w := 0; w < waves; w++ {
			concurrentWave(b, client, srv.URL, width)
		}
		conns := srv.distinctConns()
		if i == b.N-1 {
			b.ReportMetric(float64(conns), "total-conns/op")
			b.ReportMetric(float64(srv.requests)/float64(conns), "reqs/conn")
		}
		client.CloseIdleConnections()
		srv.Close()
	}
}

// BenchmarkTunedClient_ConcurrentWaves — the after-state. With the tuned
// 64-idle cap, repeated concurrent waves reuse one warm pool.
func BenchmarkTunedClient_ConcurrentWaves(b *testing.B) {
	benchConcurrentWaves(b, NewHTTPClient(0), 32, 8)
}

// BenchmarkDefaultClient_ConcurrentWaves — the before-state. The default
// 2-idle cap discards all-but-2 connections between waves, so each wave
// re-handshakes. The total-conns/op delta vs the tuned benchmark is the
// concrete B03 fix evidence (CONST-035 Rule 9 — pasted numbers).
func BenchmarkDefaultClient_ConcurrentWaves(b *testing.B) {
	benchConcurrentWaves(b, &http.Client{}, 32, 8)
}
