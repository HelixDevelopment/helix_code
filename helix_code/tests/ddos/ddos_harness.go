// Package ddos is a HelixCode-LOCAL DDoS-class flood harness. It drives a
// concurrent request flood at a real HTTP endpoint (the REAL booted
// internal/server.Server in the integration driver; a hand-written httptest.Server
// in the meta-tests that isolate the HARNESS), captures per-status-code counts +
// p50/p95/p99 latency, and asserts graceful degradation — NOT a delegation to a
// HelixQA shell script.
//
// It does NOT re-implement the stresschaos load primitives (§11.4.74 extend-don't-
// reimplement): it reuses stresschaos.RunSustainedLoad / RunConcurrent for the
// latency + deadlock/leak capture, and adds a FloodReport shape recording the
// per-status-code distribution + a refusal ratio on top.
//
// HONEST GROUND TRUTH (§11.4.6): internal/server today wires NO rate-limit
// middleware (server.go: Logger -> Recovery -> CORS -> Security only). So the
// default assertions are graceful-degradation (no goroutine leak / no deadlock,
// zero 5xx, bounded p99, real responses over the wire) — NEVER a 429-refusal ratio
// the codebase cannot produce. The 429-refusal assertion ships behind the
// DDOS_EXPECT_RATELIMIT env switch, OFF by default, so it only fires once a real
// limiter lands. Asserting a 429 today would itself be a §11.4 bluff.
package ddos

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// FloodReport is the closed-set flood_report.json evidence shape.
type FloodReport struct {
	Name            string  `json:"name"`
	Endpoint        string  `json:"endpoint"`
	RequestsSent    int64   `json:"requests_sent"`
	Status2xx       int64   `json:"status_2xx"`
	Status4xx       int64   `json:"status_4xx"`
	Status429       int64   `json:"status_429"`
	Status5xx       int64   `json:"status_5xx"`
	TransportErrors int64   `json:"transport_errors"`
	RefusalRatio    float64 `json:"refusal_ratio"`
	BodyMarkerHits  int64   `json:"body_marker_hits"`
	P99UnderFloodMs float64 `json:"p99_under_flood_ms"`
	Timestamp       string  `json:"timestamp"`
}

var (
	runIDOnce sync.Once
	runIDVal  string
)

func runID() string {
	runIDOnce.Do(func() {
		if v := os.Getenv("DDOS_RUN_ID"); v != "" {
			runIDVal = v
			return
		}
		if v := os.Getenv("STRESSCHAOS_RUN_ID"); v != "" {
			runIDVal = v
			return
		}
		runIDVal = time.Now().UTC().Format("20060102T150405Z")
	})
	return runIDVal
}

// EvidenceRoot resolves qa-results/<run-id>/. Override with DDOS_EVIDENCE_ROOT.
func EvidenceRoot() string {
	if v := os.Getenv("DDOS_EVIDENCE_ROOT"); v != "" {
		return filepath.Join(v, runID())
	}
	return filepath.Join(moduleRoot(), "qa-results", runID())
}

func moduleRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "qa-results-fallback"
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}

func evidenceDir(t testing.TB, name string) string {
	t.Helper()
	dir := filepath.Join(EvidenceRoot(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("ddos: cannot create evidence dir %s: %v", dir, err)
	}
	return dir
}

// writeJSON writes v then RE-READS it, failing on empty (§11.4.5/§11.4.69).
func writeJSON(t testing.TB, path string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("ddos: marshal evidence %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("ddos: write evidence %s: %v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("ddos: evidence artefact missing %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("ddos: evidence artefact empty (not evidence per §11.4.5) %s", path)
	}
}

// envOr reads an env var or returns the default. Shared across all test types
// (unit + integration) so env-dependent helpers work without build tags.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

// envOrHelix reads from the HELIX_* var first (.env.full-test convention),
// then falls back to the legacy TEST_* var, then to the hardcoded default.
// HXC-150: aligns ddos harness with .env.full-test so tests execute out of
// the box under the standard make test-infra-up / make test-load-full workflow.
func envOrHelix(helixKey, legacyKey, def string) string {
	if v := os.Getenv(helixKey); v != "" {
		return v
	}
	return envOr(legacyKey, def)
}

func envOrIntHelix(helixKey, legacyKey string, def int) int {
	if v := os.Getenv(helixKey); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return envOrInt(legacyKey, def)
}

// FloodConfig tunes a flood run.
type FloodConfig struct {
	// URL is the full target endpoint URL (real HTTP listener).
	URL string
	// BodyMarker, if non-empty, MUST appear in every 2xx body for the request to
	// count as a real served response (proves the server actually served, not a
	// no-op). The total body-marker hit count is asserted > 0.
	BodyMarker string
	// Parallelism is the concurrent flooder count (>= stresschaos.MinParallelism).
	Parallelism int
	// IterationsPerGoroutine is requests per flooder goroutine.
	IterationsPerGoroutine int
	// MaxP99Ms is the bounded-latency ceiling under flood. If 0, no p99 ceiling is
	// asserted (the harness still records p99 for the calibration baseline).
	MaxP99Ms float64
	// Timeout is the deadlock guard for the concurrent flood.
	Timeout time.Duration
}

// RunFlood drives a concurrent request flood at cfg.URL via stresschaos.RunConcurrent
// (which captures the goroutine-leak/deadlock evidence) while a sustained sweep
// captures p50/p95/p99 latency, tallies the per-status-code distribution, writes
// flood_report.json (write+re-read), and asserts graceful degradation:
//
//	(1) no goroutine leak / no deadlock under flood (from RunConcurrent),
//	(2) zero 5xx — a server-error storm under load is a defect,
//	(3) at least one real served response (body-marker hits > 0),
//	(4) bounded p99 if cfg.MaxP99Ms > 0.
//
// When DDOS_EXPECT_RATELIMIT=1 (a real limiter has landed) it additionally asserts
// status_429 > 0 — i.e. the limiter REFUSES excess load rather than melting. OFF by
// default so it never asserts a 429 the codebase cannot produce (§11.4.6).
func RunFlood(t testing.TB, name string, cfg FloodConfig) FloodReport {
	t.Helper()

	par := cfg.Parallelism
	if par == 0 {
		par = stresschaos.MinParallelism
	}
	iters := cfg.IterationsPerGoroutine
	if iters == 0 {
		iters = 30
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	var (
		sent       int64
		s2xx       int64
		s4xx       int64
		s429       int64
		s5xx       int64
		transport  int64
		markerHits int64
	)

	// DisableKeepAlives so the transport does not retain pooled idle-connection
	// reader goroutines after the flood — those pooled goroutines would otherwise
	// be miscounted by the RunConcurrent goroutine-leak guard as a false leak (they
	// are http.Transport bookkeeping, not a server/harness leak). Each request gets
	// a fresh connection that is closed when done, so the post-flood goroutine count
	// settles back. CloseIdleConnections is also called after the flood phase.
	httpTransport := &http.Transport{DisableKeepAlives: true}
	client := &http.Client{Timeout: 10 * time.Second, Transport: httpTransport}

	floodOnce := func() error {
		atomic.AddInt64(&sent, 1)
		resp, err := client.Get(cfg.URL)
		if err != nil {
			atomic.AddInt64(&transport, 1)
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			atomic.AddInt64(&s429, 1)
		case resp.StatusCode >= 500:
			atomic.AddInt64(&s5xx, 1)
		case resp.StatusCode >= 400:
			atomic.AddInt64(&s4xx, 1)
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			atomic.AddInt64(&s2xx, 1)
			if cfg.BodyMarker == "" || strings.Contains(string(body), cfg.BodyMarker) {
				atomic.AddInt64(&markerHits, 1)
			}
		}
		// A 5xx is the defect this harness hunts: surface it as an error so the
		// stresschaos error-rate path also records it.
		if resp.StatusCode >= 500 {
			return fmt.Errorf("5xx under flood: %d", resp.StatusCode)
		}
		return nil
	}

	// Concurrent flood: deadlock + goroutine-leak guard from the proven harness.
	// We tolerate the harness's own error-on-5xx (it would Fatalf); to keep the
	// flood report intact we run the concurrency guard with a NON-fatal wrapper and
	// assert the counts ourselves below. RunConcurrent fails on deadlock/leak which
	// is exactly what we want; its ErrorCount>0 path would also fire on a 5xx-storm,
	// so a 5xx storm fails here too — both are real defects.
	stresschaos.RunConcurrent(t, name+"_flood", stresschaos.ConcurrencyConfig{
		Parallelism:            par,
		IterationsPerGoroutine: iters,
		Timeout:                timeout,
	}, func(g, it int) error {
		return floodOnce()
	})
	httpTransport.CloseIdleConnections() // settle pooled conns before the leak snapshot

	// Sustained sweep for the latency percentiles (writes latency.json too).
	lat := stresschaos.RunSustainedLoad(t, name+"_latency", stresschaos.SustainedConfig{
		N:            stresschaos.MinSustainedN,
		MaxErrorRate: 0.0,
	}, func(i int) error {
		return floodOnce()
	})

	totalSent := atomic.LoadInt64(&sent)
	refused := atomic.LoadInt64(&s429) + atomic.LoadInt64(&s4xx)
	ratio := 0.0
	if totalSent > 0 {
		ratio = float64(refused) / float64(totalSent)
	}

	rep := FloodReport{
		Name:            name,
		Endpoint:        cfg.URL,
		RequestsSent:    totalSent,
		Status2xx:       atomic.LoadInt64(&s2xx),
		Status4xx:       atomic.LoadInt64(&s4xx),
		Status429:       atomic.LoadInt64(&s429),
		Status5xx:       atomic.LoadInt64(&s5xx),
		TransportErrors: atomic.LoadInt64(&transport),
		RefusalRatio:    ratio,
		BodyMarkerHits:  atomic.LoadInt64(&markerHits),
		P99UnderFloodMs: lat.P99Ms,
		Timestamp:       time.Now().UTC().Format(time.RFC3339Nano),
	}

	dir := evidenceDir(t, name)
	path := filepath.Join(dir, "flood_report.json")
	writeJSON(t, path, rep)

	// --- Assertions (graceful degradation, today's no-limiter reality) ---------
	if rep.Status5xx > 0 {
		t.Fatalf("ddos: %q recorded %d 5xx responses under flood — server-error storm is a defect (evidence: %s)",
			name, rep.Status5xx, path)
	}
	if rep.BodyMarkerHits == 0 {
		t.Fatalf("ddos: %q saw zero real served responses (body-marker %q never matched) — server may be a no-op (evidence: %s)",
			name, cfg.BodyMarker, path)
	}
	if cfg.MaxP99Ms > 0 && rep.P99UnderFloodMs > cfg.MaxP99Ms {
		t.Fatalf("ddos: %q p99 under flood %.2fms exceeds bounded ceiling %.2fms (evidence: %s)",
			name, rep.P99UnderFloodMs, cfg.MaxP99Ms, path)
	}

	// --- Limiter-mode assertion (gated, OFF by default) ------------------------
	if os.Getenv("DDOS_EXPECT_RATELIMIT") == "1" {
		if rep.Status429 == 0 {
			t.Fatalf("ddos: %q DDOS_EXPECT_RATELIMIT=1 but recorded ZERO 429 refusals — a rate limiter that never refuses is broken (evidence: %s)",
				name, path)
		}
	}

	t.Logf("ddos: %q flood endpoint=%s sent=%d 2xx=%d 4xx=%d 429=%d 5xx=%d markerHits=%d p99=%.2fms -> %s",
		name, rep.Endpoint, rep.RequestsSent, rep.Status2xx, rep.Status4xx, rep.Status429,
		rep.Status5xx, rep.BodyMarkerHits, rep.P99UnderFloodMs, path)
	return rep
}
