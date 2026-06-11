package infraboot

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/config"
)

// fakeBooter is a unit-test-only stand-in for *containers.Adapter (CONST-050:
// fakes live exclusively in unit tests; the real wiring uses the live Adapter).
type fakeBooter struct {
	mu             sync.Mutex
	runtimeAvail   bool
	runtimeName    string
	composeUpErr   error
	composeUpCount int
	healthFn       func(host, port string) error
}

func (f *fakeBooter) RuntimeAvailable(ctx context.Context) bool { return f.runtimeAvail }
func (f *fakeBooter) RuntimeName() string                       { return f.runtimeName }

func (f *fakeBooter) ComposeUp(ctx context.Context, composeFile string, services []string) error {
	f.mu.Lock()
	f.composeUpCount++
	f.mu.Unlock()
	return f.composeUpErr
}

func (f *fakeBooter) HealthCheckTCP(host, port string) error {
	if f.healthFn == nil {
		return errors.New("unhealthy")
	}
	return f.healthFn(host, port)
}

func (f *fakeBooter) upCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.composeUpCount
}

func bptr(b bool) *bool { return &b }

func freshCfg() *config.Config {
	cfg := &config.Config{}
	// Original (broken) values: compose-DNS hostnames the dev host can't reach.
	cfg.Database.Host = "postgres"
	cfg.Database.Port = 5432
	cfg.Database.User = "originaluser"
	cfg.Redis.Host = "redis"
	cfg.Redis.Port = 6379
	cfg.Redis.Enabled = true
	return cfg
}

// TestEnsureInfra_DisabledLeavesConfigUntouched proves the opt-out
// (HELIX_AUTOBOOT_INFRA=false) honours externally-provisioned infra: the
// booter is never touched and cfg is preserved verbatim.
func TestEnsureInfra_DisabledLeavesConfigUntouched(t *testing.T) {
	cfg := freshCfg()
	b := &fakeBooter{runtimeAvail: true}
	res, err := EnsureInfra(context.Background(), cfg, &Options{Booter: b, Enabled: bptr(false)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Skipped || res.Booted {
		t.Fatalf("expected Skipped (not Booted), got %+v", res)
	}
	if b.upCalls() != 0 {
		t.Fatalf("compose up must NOT be called when disabled, got %d calls", b.upCalls())
	}
	if cfg.Database.Host != "postgres" || cfg.Redis.Host != "redis" {
		t.Fatalf("config must be untouched when disabled, got db=%s redis=%s", cfg.Database.Host, cfg.Redis.Host)
	}
}

// TestEnsureInfra_BootsAndRewritesConfig is the core guarantee: when infra is
// down, EnsureInfra boots it (compose up) and rewrites cfg to the booted
// endpoints. This is the exact step whose absence caused the original
// `lookup redis: no such host` fatal.
func TestEnsureInfra_BootsAndRewritesConfig(t *testing.T) {
	cfg := freshCfg()
	b := &fakeBooter{runtimeAvail: true, runtimeName: "podman"}
	// Unhealthy until compose up has been issued, then healthy.
	b.healthFn = func(host, port string) error {
		if b.upCalls() == 0 {
			return errors.New("down")
		}
		return nil
	}
	res, err := EnsureInfra(context.Background(), cfg, &Options{
		Booter:       b,
		ComposeFile:  composeFixture(t),
		HealthBudget: 5 * time.Second,
		pollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Booted || res.AlreadyRunning {
		t.Fatalf("expected Booted (started fresh), got %+v", res)
	}
	if b.upCalls() != 1 {
		t.Fatalf("expected exactly 1 compose up, got %d", b.upCalls())
	}
	if cfg.Database.Host != bootHost || cfg.Database.Port != defaultPgPort {
		t.Fatalf("db not rewritten: %s:%d", cfg.Database.Host, cfg.Database.Port)
	}
	if cfg.Database.User != defaultPgUser || cfg.Database.DBName != defaultPgDBName {
		t.Fatalf("db creds not rewritten: user=%s db=%s", cfg.Database.User, cfg.Database.DBName)
	}
	if !cfg.Redis.Enabled || cfg.Redis.Host != bootHost || cfg.Redis.Port != defaultRedisPort {
		t.Fatalf("redis not rewritten: enabled=%v %s:%d", cfg.Redis.Enabled, cfg.Redis.Host, cfg.Redis.Port)
	}
}

// TestEnsureInfra_AlreadyHealthySkipsComposeUp proves idempotency: if both
// endpoints are already healthy, no compose up is issued but cfg is still
// pointed at them.
func TestEnsureInfra_AlreadyHealthySkipsComposeUp(t *testing.T) {
	cfg := freshCfg()
	b := &fakeBooter{runtimeAvail: true, runtimeName: "podman",
		healthFn: func(host, port string) error { return nil }}
	res, err := EnsureInfra(context.Background(), cfg, &Options{Booter: b})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Booted || !res.AlreadyRunning {
		t.Fatalf("expected Booted+AlreadyRunning, got %+v", res)
	}
	if b.upCalls() != 0 {
		t.Fatalf("compose up must be skipped when already healthy, got %d", b.upCalls())
	}
	if cfg.Database.Host != bootHost || cfg.Redis.Host != bootHost {
		t.Fatalf("config must still be rewritten when already healthy")
	}
}

// TestEnsureInfra_NoRuntimeIsAnError proves a missing container runtime is
// surfaced as an error (not a silent skip / not a fake success).
func TestEnsureInfra_NoRuntimeIsAnError(t *testing.T) {
	cfg := freshCfg()
	b := &fakeBooter{runtimeAvail: false,
		healthFn: func(host, port string) error { return errors.New("down") }}
	_, err := EnsureInfra(context.Background(), cfg, &Options{Booter: b})
	if err == nil {
		t.Fatalf("expected error when no runtime available")
	}
	if cfg.Database.Host != "postgres" {
		t.Fatalf("config must NOT be rewritten when boot fails")
	}
}

// TestEnsureInfra_HealthTimeout proves a never-healthy boot times out rather
// than hanging or falsely succeeding.
func TestEnsureInfra_HealthTimeout(t *testing.T) {
	cfg := freshCfg()
	b := &fakeBooter{runtimeAvail: true, runtimeName: "podman",
		healthFn: func(host, port string) error { return errors.New("never ready") }}
	_, err := EnsureInfra(context.Background(), cfg, &Options{
		Booter:       b,
		ComposeFile:  composeFixture(t),
		HealthBudget: 80 * time.Millisecond,
		pollInterval: 10 * time.Millisecond,
	})
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if cfg.Redis.Host != "redis" {
		t.Fatalf("config must NOT be rewritten when health never passes")
	}
}

// TestEnsureInfra_ComposeUpErrorPropagates proves a failed compose up is a hard
// error, not swallowed.
func TestEnsureInfra_ComposeUpErrorPropagates(t *testing.T) {
	cfg := freshCfg()
	b := &fakeBooter{runtimeAvail: true, runtimeName: "podman",
		composeUpErr: errors.New("boom"),
		healthFn:     func(host, port string) error { return errors.New("down") }}
	_, err := EnsureInfra(context.Background(), cfg, &Options{
		Booter:      b,
		ComposeFile: composeFixture(t),
	})
	if err == nil {
		t.Fatalf("expected compose up error to propagate")
	}
}

// TestEnsureInfra_PortOverrides proves the dedicated ports are configurable and
// land in the rewritten config (alt-port host-safety story).
func TestEnsureInfra_PortOverrides(t *testing.T) {
	cfg := freshCfg()
	b := &fakeBooter{runtimeAvail: true, runtimeName: "podman",
		healthFn: func(host, port string) error { return nil }}
	res, err := EnsureInfra(context.Background(), cfg, &Options{
		Booter: b, PgPort: 59999, RedisPort: 59998,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PgPort != 59999 || cfg.Database.Port != 59999 {
		t.Fatalf("pg port override not applied: res=%d cfg=%d", res.PgPort, cfg.Database.Port)
	}
	if res.RedisPort != 59998 || cfg.Redis.Port != 59998 {
		t.Fatalf("redis port override not applied: res=%d cfg=%d", res.RedisPort, cfg.Redis.Port)
	}
}

// TestEnsureInfra_WaitsForReadiness proves EnsureInfra blocks on a
// protocol-level readiness probe even after TCP health passes — the exact gap
// (TCP-up but Postgres-not-ready) that produced a false-green boot.
func TestEnsureInfra_WaitsForReadiness(t *testing.T) {
	cfg := freshCfg()
	b := &fakeBooter{runtimeAvail: true, runtimeName: "podman",
		healthFn: func(host, port string) error { return nil }} // TCP up immediately
	calls := 0
	probe := func(ctx context.Context, c *config.Config) error {
		calls++
		if calls < 3 { // not ready on first two polls (e.g. initdb in progress)
			return errors.New("postgres initialising")
		}
		return nil
	}
	res, err := EnsureInfra(context.Background(), cfg, &Options{
		Booter:       b,
		Readiness:    probe,
		HealthBudget: 5 * time.Second,
		pollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Booted {
		t.Fatalf("expected Booted after readiness, got %+v", res)
	}
	if calls < 3 {
		t.Fatalf("readiness probe must be polled until ready, got %d calls", calls)
	}
}

// TestEnsureInfra_ReadinessTimeout proves a never-ready probe fails the boot
// (no false-green when the service's port is up but it never accepts traffic).
func TestEnsureInfra_ReadinessTimeout(t *testing.T) {
	cfg := freshCfg()
	b := &fakeBooter{runtimeAvail: true, runtimeName: "podman",
		healthFn: func(host, port string) error { return nil }}
	probe := func(ctx context.Context, c *config.Config) error { return errors.New("never ready") }
	_, err := EnsureInfra(context.Background(), cfg, &Options{
		Booter:       b,
		Readiness:    probe,
		HealthBudget: 60 * time.Millisecond,
		pollInterval: 10 * time.Millisecond,
	})
	if err == nil {
		t.Fatalf("expected readiness timeout error")
	}
}

// TestEnsureInfra_FakeBooterSkipsProductionReadiness pins the intended
// coupling: when a Booter is injected (unit-test path) and no Readiness probe
// is given, the PRODUCTION defaultReadiness (which opens a REAL pgx pool) is
// NOT invoked — otherwise this test would fail trying to reach a non-existent
// Postgres. If a refactor breaks the `opts.Booter == nil` guard so
// defaultReadiness runs under a fake booter, EnsureInfra would error here.
func TestEnsureInfra_FakeBooterSkipsProductionReadiness(t *testing.T) {
	cfg := freshCfg()
	b := &fakeBooter{runtimeAvail: true, runtimeName: "podman",
		healthFn: func(host, port string) error { return nil }}
	res, err := EnsureInfra(context.Background(), cfg, &Options{Booter: b}) // nil Readiness
	if err != nil {
		t.Fatalf("production readiness must be skipped under an injected Booter, got error: %v", err)
	}
	if !res.Booted {
		t.Fatalf("expected Booted, got %+v", res)
	}
}

func TestEnabledEnvGate(t *testing.T) {
	cases := map[string]bool{"": true, "true": true, "1": true, "yes": true,
		"false": false, "0": false, "no": false, "off": false, "FALSE": false}
	for v, want := range cases {
		t.Setenv("HELIX_AUTOBOOT_INFRA", v)
		if got := enabled(nil); got != want {
			t.Fatalf("HELIX_AUTOBOOT_INFRA=%q: enabled()=%v want %v", v, got, want)
		}
	}
}

// composeFixture returns a throwaway compose file path so resolveComposeFile
// succeeds in unit tests without depending on the repo layout. The fake Booter
// never parses the file's contents (its ComposeUp ignores them); only the
// path's existence matters here.
func composeFixture(t *testing.T) string {
	t.Helper()
	f := t.TempDir() + "/docker-compose.autoboot.yml"
	if err := os.WriteFile(f, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return f
}
