//go:build integration

// Real-infrastructure verification for infraboot (CONST-050(A): integration
// tests exercise the real system — no fakes). Boots actual Postgres + Redis
// containers through the live containers adapter, then proves the booted
// endpoints accept a REAL pgx ping and a REAL go-redis PING.
//
// §11.4.115 RED-polarity: with RED_MODE=1 (default) the test first asserts the
// alt-port endpoints are DOWN on a clean baseline (reproducing the pre-fix
// "infra not booted" state that caused `lookup redis: no such host`), then
// boots and asserts they are UP — RED→GREEN on the real artifact. With
// RED_MODE=0 it is the standing GREEN regression guard (post-boot-up only).
//
// Run:
//   go test -tags=integration -run TestInfraBoot_RealContainers -v ./internal/infraboot/
package infraboot

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	containersadapter "dev.helix.code/internal/adapters/containers"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/redis"
)

func tcpDown(port int) bool {
	c, err := net.DialTimeout("tcp", net.JoinHostPort(bootHost, strconv.Itoa(port)), 750*time.Millisecond)
	if err != nil {
		return true
	}
	_ = c.Close()
	return false
}

func TestInfraBoot_RealContainers(t *testing.T) {
	ctx := context.Background()

	composeFile, err := filepath.Abs("../../docker/autoboot/docker-compose.autoboot.yml")
	if err != nil || func() bool { _, e := os.Stat(composeFile); return e != nil }() {
		t.Fatalf("autoboot compose file missing at %s: %v", composeFile, err)
	}

	adapter := containersadapter.NewAdapter()
	if !adapter.RuntimeAvailable(ctx) {
		// SKIP-OK: no container runtime on this host (docker/podman). CONST-050(A)
		// integration tests need a real runtime; honest skip per §11.4.3.
		t.Skip("SKIP-OK: no container runtime (docker/podman) available")
	}

	// Clean baseline so the RED precondition is meaningful and the test is
	// re-runnable (§11.4.98): tear down any prior autoboot project.
	_ = adapter.ComposeDown(ctx, composeFile, true)
	t.Cleanup(func() { _ = adapter.ComposeDown(ctx, composeFile, true) })

	pgPort, redisPort := defaultPgPort, defaultRedisPort

	// RED precondition (RED_MODE=1, default): the alt-port endpoints are DOWN
	// before boot — captured evidence the defect (infra not up) is real.
	redMode := os.Getenv("RED_MODE") != "0"
	if redMode {
		if !tcpDown(pgPort) || !tcpDown(redisPort) {
			t.Fatalf("RED precondition failed: expected pg:%d and redis:%d DOWN on clean baseline", pgPort, redisPort)
		}
		t.Logf("RED ok: pg:%d and redis:%d are DOWN pre-boot (defect reproduced)", pgPort, redisPort)
	}

	// THE FIX under test: boot infra and rewrite cfg.
	cfg := &config.Config{}
	cfg.Database.Host = "postgres" // original broken DNS name
	cfg.Redis.Host = "redis"       // original broken DNS name
	res, err := EnsureInfra(ctx, cfg, &Options{ComposeFile: composeFile, HealthBudget: 120 * time.Second})
	if err != nil {
		t.Fatalf("EnsureInfra failed: %v", err)
	}
	if !res.Booted {
		t.Fatalf("expected Booted, got %+v", res)
	}
	t.Logf("GREEN: %s booted postgres:%d redis:%d (alreadyRunning=%v)", res.RuntimeName, res.PgPort, res.RedisPort, res.AlreadyRunning)

	// GREEN proof #1: cfg was rewritten to the booted endpoints.
	if cfg.Database.Host != bootHost || cfg.Database.Port != pgPort {
		t.Fatalf("cfg.Database not rewritten: %s:%d", cfg.Database.Host, cfg.Database.Port)
	}
	if !cfg.Redis.Enabled || cfg.Redis.Host != bootHost || cfg.Redis.Port != redisPort {
		t.Fatalf("cfg.Redis not rewritten: %+v", cfg.Redis)
	}

	// GREEN proof #2: a REAL pgx pool connects + pings the booted Postgres.
	db, err := database.New(cfg.Database)
	if err != nil {
		t.Fatalf("real Postgres ping FAILED against booted infra: %v", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		t.Fatalf("real Postgres re-ping FAILED: %v", err)
	}
	t.Logf("GREEN proof: real pgx ping OK on %s:%d", cfg.Database.Host, cfg.Database.Port)

	// GREEN proof #3: a REAL go-redis client connects + PINGs the booted Redis.
	rds, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		t.Fatalf("real Redis connect FAILED against booted infra: %v", err)
	}
	defer rds.Close()
	t.Logf("GREEN proof: real go-redis connect OK on %s:%d", cfg.Redis.Host, cfg.Redis.Port)
}
