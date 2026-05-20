//go:build integration
// +build integration

package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// poolIntegrationConfig builds a real-PostgreSQL Config for the P4-T03
// pool-sizing integration tests. Connection parameters are read from the
// environment so the test follows CONST-§11.4.3 — it skips with a SKIP-OK
// marker when no real database is reachable instead of failing.
func poolIntegrationConfig(t *testing.T) (Config, bool) {
	t.Helper()
	host := os.Getenv("HELIX_TEST_DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := 5433 // matches the docker-compose.full-test.yml test instance
	user := os.Getenv("HELIX_TEST_DB_USER")
	if user == "" {
		user = "helix_test"
	}
	password := os.Getenv("HELIX_TEST_DB_PASSWORD")
	if password == "" {
		password = "test_password_secure_123"
	}
	dbname := os.Getenv("HELIX_TEST_DB_NAME")
	if dbname == "" {
		dbname = "helix_test"
	}
	cfg := Config{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DBName:   dbname,
		SSLMode:  "disable",
	}
	// Probe reachability with a fast-ping server-profile pool. If the probe
	// fails the real database is unavailable and the caller skips.
	probe, err := New(cfg)
	if err != nil {
		return cfg, false
	}
	probe.Close()
	return cfg, true
}

// TestPoolSizing_ConfigDrivenMaxConns_Integration is the P4-T03 anti-bluff
// proof: it opens a real pool with an explicit non-default MaxConns and
// asserts the live pgxpool reports exactly that size. A config value that
// did not reach the pool would make this fail.
func TestPoolSizing_ConfigDrivenMaxConns_Integration(t *testing.T) {
	base, ok := poolIntegrationConfig(t)
	if !ok {
		t.Skip("SKIP-OK: #P4-T03 no real PostgreSQL reachable (CONST-§11.4.3)")
	}

	const wantMax = 11 // deliberately not 20 (server default) and not 4 (CLI default)
	cfg := base
	cfg.MaxConns = wantMax
	cfg.MinConns = 3

	db, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	stats := db.Pool.Stat()
	assert.Equal(t, int32(wantMax), stats.MaxConns(),
		"the live pool MaxConns must equal the explicitly configured value — proves config-driven sizing")
	t.Logf("config-driven pool-stat: MaxConns=%d (configured %d), TotalConns=%d, IdleConns=%d",
		stats.MaxConns(), wantMax, stats.TotalConns(), stats.IdleConns())
}

// TestPoolSizing_CLIvsServerProfile_Integration proves the CLI profile
// produces a strictly smaller live pool than the server profile against a
// real database, and that the CLI profile holds zero eager idle connections.
func TestPoolSizing_CLIvsServerProfile_Integration(t *testing.T) {
	base, ok := poolIntegrationConfig(t)
	if !ok {
		t.Skip("SKIP-OK: #P4-T03 no real PostgreSQL reachable (CONST-§11.4.3)")
	}

	serverCfg := base
	serverCfg.Profile = PoolProfileServer
	serverDB, err := New(serverCfg)
	require.NoError(t, err)
	defer serverDB.Close()

	cliCfg := base
	cliDB, err := NewCLI(cliCfg) // Profile unset → NewCLI applies the CLI profile.
	require.NoError(t, err)
	defer cliDB.Close()

	serverStats := serverDB.Pool.Stat()
	cliStats := cliDB.Pool.Stat()

	assert.Equal(t, int32(20), serverStats.MaxConns(), "server profile must keep the historical MaxConns=20")
	assert.Equal(t, int32(4), cliStats.MaxConns(), "CLI profile must yield the smaller MaxConns=4")
	assert.Less(t, cliStats.MaxConns(), serverStats.MaxConns(),
		"the CLI pool must be strictly smaller than the server pool")

	// CLI profile MinConns is 0 → no connections are eagerly opened.
	assert.Equal(t, int32(0), cliStats.IdleConns(),
		"a freshly-opened CLI pool must hold zero eager idle connections")
	t.Logf("server-profile pool-stat: MaxConns=%d IdleConns=%d | cli-profile pool-stat: MaxConns=%d IdleConns=%d",
		serverStats.MaxConns(), serverStats.IdleConns(), cliStats.MaxConns(), cliStats.IdleConns())

	// The CLI pool must still be usable — issue a real query.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var one int
	require.NoError(t, cliDB.Pool.QueryRow(ctx, "SELECT 1").Scan(&one))
	assert.Equal(t, 1, one, "the smaller CLI pool must still serve real queries")
}
