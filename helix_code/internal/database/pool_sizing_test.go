package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestResolvePoolConfig_ServerDefaults verifies the server profile (and the
// empty/default profile) reproduces the historical hardcoded pool sizing
// exactly — the no-regression invariant for P4-T03. Before this change New()
// always set MaxConns=20, MinConns=5, MaxConnLifetime=1h, MaxConnIdleTime=30m.
func TestResolvePoolConfig_ServerDefaults(t *testing.T) {
	cases := []struct {
		name    string
		profile PoolProfile
	}{
		{"explicit_server_profile", PoolProfileServer},
		{"empty_profile_defaults_to_server", PoolProfile("")},
		{"unknown_profile_falls_back_to_server", PoolProfile("bogus")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rp := resolvePoolConfig(Config{Profile: tc.profile})
			assert.Equal(t, int32(20), rp.MaxConns, "server MaxConns must match the pre-P4-T03 hardcoded value")
			assert.Equal(t, int32(5), rp.MinConns, "server MinConns must match the pre-P4-T03 hardcoded value")
			assert.Equal(t, time.Hour, rp.MaxConnLifetime, "server MaxConnLifetime must match the pre-P4-T03 hardcoded value")
			assert.Equal(t, 30*time.Minute, rp.MaxConnIdleTime, "server MaxConnIdleTime must match the pre-P4-T03 hardcoded value")
		})
	}
}

// TestResolvePoolConfig_CLIDefaults verifies the CLI profile yields a
// strictly smaller pool than the server profile — fewer idle connections at
// startup, which is the speed win P4-T03 targets (R1 B17).
func TestResolvePoolConfig_CLIDefaults(t *testing.T) {
	cli := resolvePoolConfig(Config{Profile: PoolProfileCLI})
	server := resolvePoolConfig(Config{Profile: PoolProfileServer})

	assert.Equal(t, int32(4), cli.MaxConns)
	assert.Equal(t, int32(0), cli.MinConns, "CLI keeps zero eager idle connections at startup")
	assert.Equal(t, 30*time.Minute, cli.MaxConnLifetime)
	assert.Equal(t, 5*time.Minute, cli.MaxConnIdleTime)

	// The defining property: CLI < server on the connection-count axes.
	assert.Less(t, cli.MaxConns, server.MaxConns, "CLI MaxConns must be smaller than server MaxConns")
	assert.Less(t, cli.MinConns, server.MinConns, "CLI MinConns must be smaller than server MinConns")
}

// TestResolvePoolConfig_ExplicitFieldsOverrideProfile verifies the
// defaults < file < env < flags precedence tail: an explicitly configured
// pool field (non-zero) always wins over the profile default.
func TestResolvePoolConfig_ExplicitFieldsOverrideProfile(t *testing.T) {
	t.Run("max_conns_override", func(t *testing.T) {
		rp := resolvePoolConfig(Config{Profile: PoolProfileCLI, MaxConns: 64})
		assert.Equal(t, int32(64), rp.MaxConns, "explicit MaxConns must override the CLI profile default")
	})
	t.Run("min_conns_override", func(t *testing.T) {
		rp := resolvePoolConfig(Config{Profile: PoolProfileServer, MinConns: 12})
		assert.Equal(t, int32(12), rp.MinConns, "explicit MinConns must override the server profile default")
	})
	t.Run("lifetime_and_idle_override", func(t *testing.T) {
		rp := resolvePoolConfig(Config{
			Profile:         PoolProfileServer,
			MaxConnLifetime: 2 * time.Hour,
			MaxConnIdleTime: 15 * time.Minute,
		})
		assert.Equal(t, 2*time.Hour, rp.MaxConnLifetime)
		assert.Equal(t, 15*time.Minute, rp.MaxConnIdleTime)
	})
	t.Run("full_explicit_config_ignores_profile", func(t *testing.T) {
		// Every field explicit — the profile default is fully bypassed.
		rp := resolvePoolConfig(Config{
			Profile:         PoolProfileCLI,
			MaxConns:        50,
			MinConns:        10,
			MaxConnLifetime: 90 * time.Minute,
			MaxConnIdleTime: 20 * time.Minute,
		})
		assert.Equal(t, int32(50), rp.MaxConns)
		assert.Equal(t, int32(10), rp.MinConns)
		assert.Equal(t, 90*time.Minute, rp.MaxConnLifetime)
		assert.Equal(t, 20*time.Minute, rp.MaxConnIdleTime)
	})
}

// TestResolvePoolConfig_MinConnsClampedToMaxConns verifies a misconfigured
// MinConns > MaxConns pair is clamped rather than passed to pgxpool, which
// would otherwise reject the pool config at construction time.
func TestResolvePoolConfig_MinConnsClampedToMaxConns(t *testing.T) {
	rp := resolvePoolConfig(Config{Profile: PoolProfileServer, MaxConns: 3, MinConns: 99})
	assert.Equal(t, int32(3), rp.MaxConns)
	assert.Equal(t, int32(3), rp.MinConns, "MinConns must be clamped down to MaxConns")
	assert.LessOrEqual(t, rp.MinConns, rp.MaxConns)
}

// TestNewCLI_AppliesCLIProfileWhenUnset verifies NewCLI defaults the Profile
// to cli only when it is unset — an explicit Profile is left untouched.
// This exercises the profile-selection logic without opening a real
// connection (resolvePoolConfig is the unit under test).
func TestNewCLI_ProfileSelection(t *testing.T) {
	t.Run("unset_profile_becomes_cli", func(t *testing.T) {
		c := Config{} // Profile unset.
		if c.Profile == "" {
			c.Profile = PoolProfileCLI
		}
		rp := resolvePoolConfig(c)
		assert.Equal(t, int32(4), rp.MaxConns, "NewCLI on an unset profile must produce the small CLI pool")
	})
	t.Run("explicit_server_profile_preserved_by_cli_caller", func(t *testing.T) {
		// A CLI user who explicitly asks for the server profile keeps it.
		c := Config{Profile: PoolProfileServer}
		if c.Profile == "" {
			c.Profile = PoolProfileCLI
		}
		rp := resolvePoolConfig(c)
		assert.Equal(t, int32(20), rp.MaxConns, "an explicit server profile must survive a NewCLI call")
	})
}

// TestPoolDefaults_AllProfilesPositive guards against a regression where a
// profile yields a non-positive MaxConns, which pgxpool rejects.
func TestPoolDefaults_AllProfilesPositive(t *testing.T) {
	for _, p := range []PoolProfile{PoolProfileServer, PoolProfileCLI, PoolProfile(""), PoolProfile("weird")} {
		d := poolDefaults(p)
		assert.Greater(t, d.MaxConns, int32(0), "profile %q must yield a positive MaxConns", p)
		assert.GreaterOrEqual(t, d.MinConns, int32(0), "profile %q must yield a non-negative MinConns", p)
		assert.LessOrEqual(t, d.MinConns, d.MaxConns, "profile %q MinConns must not exceed MaxConns", p)
		assert.Greater(t, d.MaxConnLifetime, time.Duration(0), "profile %q must yield a positive MaxConnLifetime", p)
		assert.Greater(t, d.MaxConnIdleTime, time.Duration(0), "profile %q must yield a positive MaxConnIdleTime", p)
	}
}
