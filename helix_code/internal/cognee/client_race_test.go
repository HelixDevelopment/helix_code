package cognee

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/config"
)

// TestClient_MutableConfig_NoRace is the STANDING regression guard (§11.4.135)
// for the guarded-write / unguarded-read DATA RACE on (*Client).baseURL and
// (*Client).apiKey.
//
// Defect (pre-fix, internal/cognee/client.go):
//   - SetBaseURL / SetAPIKey wrote c.baseURL / c.apiKey UNDER c.mu.Lock().
//   - GetBaseURL read c.baseURL with NO lock, and every request builder
//     (`fmt.Sprintf("%s/...", c.baseURL)`) plus setHeaders (`c.apiKey`) read
//     those same fields UNGUARDED.
//   A request in flight concurrent with SetBaseURL/SetAPIKey is therefore a
//   data race — `go test -race` reports "WARNING: DATA RACE" on c.baseURL /
//   c.apiKey. The fix routes every read through the lock-guarded getBaseURL()
//   / getAPIKey() accessors (GetBaseURL also now takes the RLock).
//
// §11.4.115 RED_MODE polarity switch:
//   - RED_MODE=1 drives a FAITHFUL pre-fix stand-in (racyClient: guarded writes,
//     unguarded reads) under the SAME concurrent workload. Built with -race, the
//     stand-in trips the detector and the test process aborts with a non-zero
//     exit — i.e. RED reproduces the defect on the broken shape.
//   - RED_MODE=0 (DEFAULT, no env) drives the REAL fixed *Client concurrently
//     and asserts NO race (clean -race run = GREEN guard).
//
// Run the guard:        go test -race -run TestClient_MutableConfig_NoRace ./internal/cognee/
// Reproduce the defect: RED_MODE=1 go test -race -run TestClient_MutableConfig_NoRace ./internal/cognee/
func TestClient_MutableConfig_NoRace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	deadline := time.Now().Add(400 * time.Millisecond)

	if os.Getenv("RED_MODE") == "1" {
		// RED: faithful pre-fix stand-in. Writes are guarded, reads are NOT —
		// exactly the broken ordering the fix removed. Under -race this races.
		rc := &racyClient{baseURL: srv.URL, apiKey: "key-initial", http: srv.Client()}
		drive(deadline,
			func(i int) { rc.SetAPIKey("k-" + string(rune('A'+i%26))); rc.SetBaseURL(srv.URL) },
			func() { _ = rc.GetBaseURL(); _ = rc.doRequest(context.Background()) },
		)
		return
	}

	// GREEN: the REAL fixed client must be race-free under the same workload.
	c := NewClient(&config.CogneeConfig{Host: "127.0.0.1", Port: 1})
	c.SetBaseURL(srv.URL)
	c.SetAPIKey("key-initial")
	drive(deadline,
		func(i int) { c.SetAPIKey("k-" + string(rune('A'+i%26))); c.SetBaseURL(srv.URL) },
		func() {
			_ = c.GetBaseURL()
			_, _ = c.GetHealth(context.Background())
		},
	)
}

// drive runs one writer goroutine and four reader goroutines until deadline.
func drive(deadline time.Time, write func(int), read func()) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; time.Now().Before(deadline); i++ {
			write(i)
		}
	}()
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				read()
			}
		}()
	}
	wg.Wait()
}

// racyClient is a faithful stand-in for the PRE-FIX Client: it reproduces the
// exact bug shape (writes under the lock, reads with no lock) so RED_MODE=1
// trips the race detector. It is test-only and never used by production code.
type racyClient struct {
	mu      sync.RWMutex
	baseURL string
	apiKey  string
	http    *http.Client
}

func (r *racyClient) SetBaseURL(u string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.baseURL = u
}

func (r *racyClient) SetAPIKey(k string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.apiKey = k
}

// GetBaseURL reads baseURL WITHOUT the lock — the original defect.
func (r *racyClient) GetBaseURL() string { return r.baseURL }

// doRequest reads baseURL + apiKey WITHOUT the lock while building the request —
// the original setHeaders / URL-builder defect.
func (r *racyClient) doRequest(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", r.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	if r.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.apiKey)
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}
