package ddos

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// §1.1 paired-mutation meta-tests prove the ddos harness cannot bluff. They drive
// the harness at hand-written httptest servers (no real DB needed — meta-tests
// isolate the HARNESS, not the system) that plant a known defect, and assert the
// harness DETECTS it (detection path is t.Fatalf, captured via failTB).

type failTB struct {
	testing.TB
	mu     sync.Mutex
	failed bool
	msg    string
}

func (f *failTB) Helper() {}
func (f *failTB) Fatalf(format string, args ...interface{}) {
	f.mu.Lock()
	f.failed = true
	f.msg = fmt.Sprintf(format, args...)
	f.mu.Unlock()
	panic(sentinelFatal{})
}
func (f *failTB) Errorf(format string, args ...interface{}) {
	f.mu.Lock()
	f.failed = true
	f.msg = fmt.Sprintf(format, args...)
	f.mu.Unlock()
}
func (f *failTB) Logf(format string, args ...interface{}) {}

type sentinelFatal struct{}

func runWithFailTB(body func(tb testing.TB)) (failed bool, msg string) {
	f := &failTB{TB: &testing.T{}}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(sentinelFatal); !ok {
					panic(r)
				}
			}
		}()
		body(f)
	}()
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failed, f.msg
}

func isolatedEvidence(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	old := os.Getenv("DDOS_EVIDENCE_ROOT")
	os.Setenv("DDOS_EVIDENCE_ROOT", tmp)
	t.Cleanup(func() { os.Setenv("DDOS_EVIDENCE_ROOT", old) })
}

// TestMeta_RunFlood_Detects5xxStorm plants a server that returns 500 for EVERY
// request and asserts the harness FLAGS the 5xx-storm. A harness that PASSes a
// 500-storm is a bluff.
func TestMeta_RunFlood_Detects5xxStorm(t *testing.T) {
	isolatedEvidence(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	failed, _ := runWithFailTB(func(tb testing.TB) {
		RunFlood(tb, "meta-5xx-storm", FloodConfig{
			URL: srv.URL, BodyMarker: "", Parallelism: 10, IterationsPerGoroutine: 10,
			Timeout: 20 * time.Second,
		})
	})
	if !failed {
		t.Fatal("meta: RunFlood did NOT detect the 5xx storm — harness is a bluff")
	}
}

// TestMeta_RunFlood_DetectsLatencyBomb plants a server that sleeps a large fixed
// delay and asserts the harness records a p99 blowout that exceeds the bounded
// ceiling.
func TestMeta_RunFlood_DetectsLatencyBomb(t *testing.T) {
	isolatedEvidence(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(60 * time.Millisecond) // every response is slow
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("healthy"))
	}))
	defer srv.Close()

	failed, _ := runWithFailTB(func(tb testing.TB) {
		RunFlood(tb, "meta-latency-bomb", FloodConfig{
			URL: srv.URL, BodyMarker: "healthy", Parallelism: 10, IterationsPerGoroutine: 10,
			MaxP99Ms: 20.0, // 60ms responses blow past a 20ms ceiling
			Timeout:  30 * time.Second,
		})
	})
	if !failed {
		t.Fatal("meta: RunFlood did NOT detect the latency bomb (p99 ceiling breach) — harness is a bluff")
	}
}

// TestMeta_RunFlood_DetectsNoServedResponses plants a server that returns 204 with
// an empty body (never the body marker) and asserts the harness FAILS on zero real
// served responses.
func TestMeta_RunFlood_DetectsNoServedResponses(t *testing.T) {
	isolatedEvidence(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // 200 but body never contains the marker
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()

	failed, _ := runWithFailTB(func(tb testing.TB) {
		RunFlood(tb, "meta-no-served", FloodConfig{
			URL: srv.URL, BodyMarker: "healthy", Parallelism: 10, IterationsPerGoroutine: 10,
			Timeout: 20 * time.Second,
		})
	})
	if !failed {
		t.Fatal("meta: RunFlood did NOT detect zero real served responses (no body-marker match) — harness is a bluff")
	}
}

// TestMeta_RunFlood_LimiterModeDetectsNoRefusals enables DDOS_EXPECT_RATELIMIT=1
// against a server that NEVER returns 429 and asserts the harness FAILS
// "expected refusals, got none" — so the limiter assertion itself cannot bluff
// once enabled.
func TestMeta_RunFlood_LimiterModeDetectsNoRefusals(t *testing.T) {
	isolatedEvidence(t)
	old := os.Getenv("DDOS_EXPECT_RATELIMIT")
	os.Setenv("DDOS_EXPECT_RATELIMIT", "1")
	t.Cleanup(func() { os.Setenv("DDOS_EXPECT_RATELIMIT", old) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // never 429 — no limiter
		_, _ = w.Write([]byte("healthy"))
	}))
	defer srv.Close()

	failed, _ := runWithFailTB(func(tb testing.TB) {
		RunFlood(tb, "meta-limiter-no-refusal", FloodConfig{
			URL: srv.URL, BodyMarker: "healthy", Parallelism: 10, IterationsPerGoroutine: 10,
			Timeout: 20 * time.Second,
		})
	})
	if !failed {
		t.Fatal("meta: limiter-mode RunFlood did NOT detect zero 429 refusals — limiter assertion is a bluff")
	}
}

// TestMeta_PositivePathWritesEvidence proves a healthy 200-OK server makes the
// harness write a non-empty flood_report.json (the artefact really exists).
func TestMeta_PositivePathWritesEvidence(t *testing.T) {
	isolatedEvidence(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer srv.Close()

	rep := RunFlood(t, "meta-positive", FloodConfig{
		URL: srv.URL, BodyMarker: "healthy", Parallelism: 12, IterationsPerGoroutine: 15,
		Timeout: 20 * time.Second,
	})
	if rep.Status5xx != 0 {
		t.Fatalf("meta: positive path recorded %d 5xx unexpectedly", rep.Status5xx)
	}
	if rep.BodyMarkerHits == 0 {
		t.Fatal("meta: positive path saw zero served responses")
	}
	path := filepath.Join(EvidenceRoot(), "meta-positive", "flood_report.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("meta: flood_report.json not written: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("meta: flood_report.json is empty — would be a hollow PASS")
	}
}
