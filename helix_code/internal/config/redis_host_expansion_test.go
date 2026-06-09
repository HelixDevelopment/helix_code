package config

// TestRedisHostEnvExpansion — RED/GREEN polarity-switch test (§11.4.115).
//
// ROOT CAUSE documented in qa-results/W2-REDISBUG-evidence.txt:
//   config/config.yaml line 20: `host: "${HELIX_REDIS_HOST:redis}"`
//
// Viper does NOT perform shell-style ${VAR:default} expansion on YAML values.
// When HELIX_REDIS_HOST is unset, viper returns the literal string
// "${HELIX_REDIS_HOST:redis}" from the YAML — because:
//   1. v.BindEnv("redis.host", "HELIX_REDIS_HOST") has higher precedence than
//      the YAML file value ONLY when HELIX_REDIS_HOST is set in the environment.
//   2. When HELIX_REDIS_HOST is empty/"", BindEnv falls through to the YAML
//      value, which is the unexpanded literal "${HELIX_REDIS_HOST:redis}".
//   3. No os.ExpandEnv or custom expander is called on YAML string values after
//      ReadInConfig → Unmarshal.
//   4. The literal reaches redis.NewClient → fmt.Sprintf("%s:%d", host, port)
//      → "${HELIX_REDIS_HOST:redis}:6379" → "too many colons in address".
//
// RED_MODE=1 (default): asserts the BUG IS PRESENT on current code.
//   Test MUST FAIL when the bug is fixed (the resolved host will be a plain
//   hostname with no "${" prefix).
//
// RED_MODE=0: asserts the BUG IS ABSENT — the standing GREEN regression guard.
//   Set via env: RED_MODE=0 go test -run TestRedisHostEnvExpansion ./internal/config/...
//
// §11.4.102: NO fix without root cause. §11.4.115: RED on broken artifact first.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisHostEnvExpansion(t *testing.T) {
	// Polarity switch (§11.4.115): default is the standing GREEN regression guard
	// (asserts the bug is ABSENT — fix landed). Set RED_MODE=1 to reproduce the
	// original defect against a pre-fix checkout.
	redMode := os.Getenv("RED_MODE") == "1"

	// Ensure HELIX_REDIS_HOST is unset for this test so we exercise the
	// default-from-YAML path.
	origVal, wasSet := os.LookupEnv("HELIX_REDIS_HOST")
	require.NoError(t, os.Unsetenv("HELIX_REDIS_HOST"))
	t.Cleanup(func() {
		if wasSet {
			os.Setenv("HELIX_REDIS_HOST", origVal)
		} else {
			os.Unsetenv("HELIX_REDIS_HOST")
		}
	})

	// Point Load() at the real config/config.yaml (two levels up from this
	// package dir) by writing HELIX_CONFIG to the project config path.
	// We use the actual project yaml — the one containing the ${...} literal —
	// to exercise the exact real-world code path.
	configYAML := `
server:
  address: "0.0.0.0"
  port: 8080
database:
  host: "localhost"
  port: 5432
  user: "test"
  dbname: "test"
  sslmode: "disable"
redis:
  host: "${HELIX_REDIS_HOST:redis}"
  port: 6379
  enabled: true
auth:
  jwt_secret: "test-secret-minimum-32-chars-long-enough"
  token_expiry: 86400
  session_expiry: 604800
  bcrypt_cost: 12
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(configYAML), 0600))

	origCfgEnv, cfgWasSet := os.LookupEnv("HELIX_CONFIG")
	require.NoError(t, os.Setenv("HELIX_CONFIG", cfgPath))
	t.Cleanup(func() {
		if cfgWasSet {
			os.Setenv("HELIX_CONFIG", origCfgEnv)
		} else {
			os.Unsetenv("HELIX_CONFIG")
		}
		resetForTest() // clear Get() memoisation so other tests are unaffected
	})
	resetForTest()

	cfg, err := Load()
	require.NoError(t, err, "Load() must not return an error")

	resolvedHost := cfg.Redis.Host
	t.Logf("resolved redis.host = %q  (RED_MODE=%v)", resolvedHost, redMode)

	isLiteral := strings.HasPrefix(resolvedHost, "${")

	if redMode {
		// RED assertion: the bug IS present — the host contains the unexpanded literal.
		// This MUST FAIL once the bug is fixed.
		assert.True(t, isLiteral,
			"[RED] expected redis.host to be the unexpanded literal ${...} (bug present), got %q", resolvedHost)
	} else {
		// GREEN assertion: the bug IS absent — the host is a plain hostname.
		// This is the standing regression guard.
		assert.False(t, isLiteral,
			"[GREEN] redis.host must NOT contain the unexpanded ${...} literal, got %q", resolvedHost)
		assert.NotEmpty(t, resolvedHost,
			"[GREEN] redis.host must not be empty after expansion")
	}
}
