package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dev.helix.code/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDBPoolConfig_DefaultProfileIsServer verifies that, with no config file
// and no env override, the database pool profile defaults to "server" — the
// no-regression invariant for P4-T03 (a server process keeps today's pool).
func TestDBPoolConfig_DefaultProfileIsServer(t *testing.T) {
	cfg := getDefaultConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, database.PoolProfileServer, cfg.Database.Profile,
		"the default database pool profile must be 'server' so existing server callers keep today's pool")

	// The individual sizing keys are unset by default so the profile
	// default applies in database.New().
	assert.Zero(t, cfg.Database.MaxConns, "database.max_conns must be unset by default (profile default applies)")
	assert.Zero(t, cfg.Database.MinConns, "database.min_conns must be unset by default (profile default applies)")
	assert.Zero(t, cfg.Database.MaxConnLifetime, "database.max_conn_lifetime must be unset by default")
	assert.Zero(t, cfg.Database.MaxConnIdleTime, "database.max_conn_idle_time must be unset by default")
}

// writeMinimalConfig writes a config file carrying the fields validateConfig
// requires (version, application name, auth jwt secret, server port,
// database host/dbname) plus the supplied database pool block, and returns
// the path. It lets a pool-precedence test isolate the pool fields without
// tripping unrelated validation.
func writeMinimalConfig(t *testing.T, dbPoolBlock string) string {
	t.Helper()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := `{
  "version": "1.0.0",
  "application": { "name": "HelixCode" },
  "auth": { "jwt_secret": "test-secret-not-a-real-credential" },
  "server": { "address": "0.0.0.0", "port": 8080 },
  "database": {
    "host": "localhost",
    "port": 5432,
    "dbname": "test",
    "user": "test"` + dbPoolBlock + `
  }
}`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))
	return configPath
}

// TestDBPoolConfig_FileOverridesProfileAndSizing verifies a config file can
// drive the pool profile and the individual sizing fields — the "file" layer
// of the defaults < file < env < flags precedence chain.
func TestDBPoolConfig_FileOverridesProfileAndSizing(t *testing.T) {
	configPath := writeMinimalConfig(t, `,
    "profile": "cli",
    "max_conns": 7,
    "min_conns": 2,
    "max_conn_lifetime": 900000000000,
    "max_conn_idle_time": 120000000000`)

	t.Setenv("HELIX_CONFIG", configPath)
	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, database.PoolProfileCLI, cfg.Database.Profile,
		"the config file must be able to set the database pool profile")
	assert.Equal(t, 7, cfg.Database.MaxConns, "the config file must set database.max_conns")
	assert.Equal(t, 2, cfg.Database.MinConns, "the config file must set database.min_conns")
	assert.Equal(t, 15*time.Minute, cfg.Database.MaxConnLifetime,
		"the config file must set database.max_conn_lifetime")
	assert.Equal(t, 2*time.Minute, cfg.Database.MaxConnIdleTime,
		"the config file must set database.max_conn_idle_time")
}

// TestDBPoolConfig_FileDefaultProfileStaysServer verifies that a config file
// which omits the pool block leaves the profile at the "server" default —
// the no-regression invariant for existing config files in the repo.
func TestDBPoolConfig_FileDefaultProfileStaysServer(t *testing.T) {
	configPath := writeMinimalConfig(t, "")
	t.Setenv("HELIX_CONFIG", configPath)
	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, database.PoolProfileServer, cfg.Database.Profile,
		"a config file without a pool block must keep the 'server' profile default")
	assert.Zero(t, cfg.Database.MaxConns, "an unset max_conns must remain zero so the profile default applies")
}

// TestDBPoolConfig_EnvOverridesProfile verifies an environment variable wins
// over a config file value — the "env" beats "file" layer of the precedence
// chain. The file asks for the server profile; the env demands cli.
func TestDBPoolConfig_EnvOverridesProfile(t *testing.T) {
	configPath := writeMinimalConfig(t, `,
    "profile": "server",
    "max_conns": 20`)
	t.Setenv("HELIX_CONFIG", configPath)
	t.Setenv("HELIX_DATABASE_POOL_PROFILE", "cli")
	t.Setenv("HELIX_DATABASE_POOL_MAX_CONNS", "9")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, database.PoolProfileCLI, cfg.Database.Profile,
		"HELIX_DATABASE_POOL_PROFILE must override the config-file 'server' profile")
	assert.Equal(t, 9, cfg.Database.MaxConns,
		"HELIX_DATABASE_POOL_MAX_CONNS must override the config-file max_conns")
}
