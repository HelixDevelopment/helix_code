package config

// Standing regression guard (§11.4.135) for the bcrypt-cost validation gap with
// §11.4.115 RED_MODE polarity.
//
// DEFECT (HXC-config / discovery §11.4.118):
//   validateConfig() and ConfigurationValidator.Validate() validated Server.Port
//   and Redis.Port ranges but performed NO range check on Auth.BcryptCost. A
//   config with bcrypt_cost > 31 was accepted as "valid" by the loader, yet
//   golang.org/x/crypto/bcrypt.GenerateFromPassword (called from
//   internal/auth/auth.go:452 on every password-hash) returns
//   "cost N is outside allowed inclusive range 4..31" for any cost > 31 — so
//   EVERY user registration / password-set flow fails at runtime against a
//   config the System reported healthy. A cost < 4 is silently coerced by the
//   bcrypt package to DefaultCost, quietly weakening the configured work factor.
//
// REPRODUCTION (captured before fix, inside this test via the faithful pre-fix
// stand-in validateConfigPreBcryptFix):
//   bcrypt cost=32 -> validateConfigPreBcryptFix=<nil> (ACCEPTED) but
//   bcrypt.GenerateFromPassword(..., 32) = "cost 32 is outside allowed inclusive
//   range 4..31" — the loader accepted a config that breaks password hashing.
//
// RED_MODE=1: drive the pre-fix stand-in and ASSERT the defect is present
//   (cost=32 accepted while bcrypt rejects it). PASSES on the broken artifact;
//   MUST FAIL if someone re-introduces the check into the stand-in.
// RED_MODE=0 (default): drive the REAL fixed validateConfig + Validate and
//   ASSERT they reject out-of-range costs and accept in-range ones. The standing
//   GREEN regression guard.

import (
	"os"
	"testing"

	"dev.helix.code/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// validBaseConfigForBcrypt returns a config that satisfies every OTHER
// validateConfig branch so that Auth.BcryptCost is the only field under test.
func validBaseConfigForBcrypt(cost int) *Config {
	return &Config{
		Version:     "1.0.0",
		Application: ApplicationConfig{Name: "helix", Environment: "development"},
		Server:      ServerConfig{Port: 8080},
		Database:    database.Config{Host: "localhost", Port: 5432, DBName: "helix"},
		Auth:        AuthConfig{JWTSecret: "a-real-secret-at-least-32-chars-long-xx", BcryptCost: cost},
		Workers:     WorkersConfig{HealthCheckInterval: 30, MaxConcurrentTasks: 10},
		Tasks:       TasksConfig{MaxRetries: 3},
		LLM:         LLMConfig{DefaultProvider: "local", MaxTokens: 1000, Temperature: 0.7},
	}
}

// validateConfigPreBcryptFix is a faithful copy of the auth-validation slice of
// validateConfig() AS IT WAS BEFORE the bcrypt-cost check was added — used only
// to reproduce the defect in RED_MODE. It deliberately omits the bcrypt range
// check. It checks only the fields that gate reaching the (absent) bcrypt check
// for the cost-only fixture above, so a non-nil result here means "the pre-fix
// validator accepted this cost".
func validateConfigPreBcryptFix(cfg *Config) error {
	if cfg.Auth.JWTSecret == "" || cfg.Auth.JWTSecret == "default-secret-change-in-production" {
		return assertNonNilSentinel
	}
	// NOTE: no bcrypt-cost validation here — this is the pre-fix behaviour.
	return nil
}

// assertNonNilSentinel is a stand-in error for the pre-fix validator's failure
// branch (we only care whether it returned nil/non-nil for the cost fixture).
var assertNonNilSentinel = &preFixError{}

type preFixError struct{}

func (*preFixError) Error() string { return "pre-fix validation failed" }

func TestBcryptCostValidationGuard(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	if redMode {
		// RED: reproduce the defect on the faithful pre-fix stand-in.
		const brokenCost = 32
		preFixErr := validateConfigPreBcryptFix(validBaseConfigForBcrypt(brokenCost))
		_, bErr := bcrypt.GenerateFromPassword([]byte("pw"), brokenCost)

		t.Logf("[RED] cost=%d preFixValidate=%v bcrypt=%v", brokenCost, preFixErr, bErr)
		assert.NoError(t, preFixErr,
			"[RED] pre-fix validateConfig MUST accept cost=%d (defect present)", brokenCost)
		assert.Error(t, bErr,
			"[RED] bcrypt MUST reject cost=%d — proving the accepted config is broken at runtime", brokenCost)
		return
	}

	// GREEN (default): the real fixed validators must reject out-of-range costs
	// and accept in-range ones; and an accepted cost must actually work with bcrypt.
	validator := &ConfigurationValidator{}

	t.Run("rejects_cost_above_max", func(t *testing.T) {
		for _, cost := range []int{32, 40, 100} {
			require.Error(t, validateConfig(validBaseConfigForBcrypt(cost)),
				"validateConfig must reject bcrypt cost=%d (> 31)", cost)
			res := validator.Validate(validBaseConfigForBcrypt(cost))
			assert.False(t, res.Valid, "Validate must reject bcrypt cost=%d", cost)
		}
	})

	t.Run("rejects_cost_below_min", func(t *testing.T) {
		for _, cost := range []int{0, -1, 3} {
			require.Error(t, validateConfig(validBaseConfigForBcrypt(cost)),
				"validateConfig must reject bcrypt cost=%d (< 4)", cost)
			res := validator.Validate(validBaseConfigForBcrypt(cost))
			assert.False(t, res.Valid, "Validate must reject bcrypt cost=%d", cost)
		}
	})

	t.Run("accepts_in_range", func(t *testing.T) {
		// 4 = bcrypt.MinCost, 31 = bcrypt.MaxCost — the full inclusive bound the
		// fix permits. The validators must accept every in-range value.
		for _, cost := range []int{4, 12, 31} {
			require.NoError(t, validateConfig(validBaseConfigForBcrypt(cost)),
				"validateConfig must accept in-range bcrypt cost=%d", cost)
			res := validator.Validate(validBaseConfigForBcrypt(cost))
			assert.True(t, res.Valid, "Validate must accept in-range bcrypt cost=%d", cost)
		}
	})

	t.Run("accepted_cost_hashes_without_runtime_error", func(t *testing.T) {
		// Positive end-to-end proof the accepted range produces a WORKING hash
		// (no bcrypt InvalidCostError). Only LOW costs are actually run — a real
		// cost=31 hash is ~2^31 rounds (hours) and infeasible to execute; cost=31
		// being valid is bcrypt.MaxCost by definition and is covered by the
		// validator-acceptance assertion above.
		for _, cost := range []int{4, 10} {
			require.NoError(t, validateConfig(validBaseConfigForBcrypt(cost)),
				"validateConfig must accept bcrypt cost=%d", cost)
			_, bErr := bcrypt.GenerateFromPassword([]byte("pw"), cost)
			require.NoError(t, bErr,
				"an accepted bcrypt cost=%d must hash without a runtime error", cost)
		}
	})
}
