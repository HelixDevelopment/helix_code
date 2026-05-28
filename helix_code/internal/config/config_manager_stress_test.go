package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(A) stress coverage for internal/config's ConfigManager.
//
// Exercises the REAL *ConfigManager (no fakes — real on-disk JSON files via
// t.TempDir, real os.ReadFile/WriteFile + json.Unmarshal):
//
//   - sustained load: GetConfig + Load() invoked N>=100 times, p50/p95/p99
//     latency captured (latency.json).
//   - concurrent contention: GetConfig / UpdateConfig / loadConfig hammered
//     from >=10 goroutines (concurrency_report.json); run under -race to catch
//     the get-while-reload data race.
//   - boundary conditions: empty config path, validation boundaries
//     (off-by-one on Server.Port via the validator), and a freshly-created
//     manager with no pre-existing file.

// TestConfigManager_Stress_SustainedGet drives GetConfig under sustained load.
// GetConfig is the hot per-request read path; it must stay fast and never error.
func TestConfigManager_Stress_SustainedGet(t *testing.T) {
	dir := t.TempDir()
	primary := writeValidConfigFile(t, dir, "config.json")
	mgr, err := NewHelixConfigManager(primary)
	if err != nil {
		t.Fatalf("NewHelixConfigManager: %v", err)
	}

	rep := stresschaos.RunSustainedLoad(t, "config_sustained_get",
		stresschaos.SustainedConfig{N: 2000},
		func(i int) error {
			cfg := mgr.GetConfig()
			if cfg == nil {
				return fmt.Errorf("GetConfig returned nil at i=%d", i)
			}
			if cfg.Application.Name == "" {
				return fmt.Errorf("config has empty application name at i=%d", i)
			}
			return nil
		})
	t.Logf("config sustained GetConfig p50=%.3fms p95=%.3fms p99=%.3fms", rep.P50Ms, rep.P95Ms, rep.P99Ms)
}

// TestConfigManager_Stress_SustainedLoad drives the package-level Load() under
// sustained load. Load() builds a fresh per-call viper instance (P2-T07 race
// fix); sustained invocation must stay race-free and error-free with a valid
// config + JWT secret on disk.
func TestConfigManager_Stress_SustainedLoad(t *testing.T) {
	dir := t.TempDir()
	// Load() reads YAML and validates (requires a non-default JWT secret).
	yamlCfg := `version: "1.0.0"
application:
  name: "HelixCode"
server:
  port: 8080
database:
  host: "localhost"
  dbname: "helixcode"
auth:
  jwt_secret: "a-real-secret-that-is-long-enough-for-validation"
workers:
  health_check_interval: 30
  max_concurrent_tasks: 10
llm:
  default_provider: "local"
  max_tokens: 4096
  temperature: 0.7
`
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlCfg), 0o644); err != nil {
		t.Fatalf("write yaml config: %v", err)
	}
	t.Setenv("HELIX_CONFIG", cfgPath)

	rep := stresschaos.RunSustainedLoad(t, "config_sustained_load",
		stresschaos.SustainedConfig{N: 200},
		func(i int) error {
			cfg, err := Load()
			if err != nil {
				return fmt.Errorf("Load failed at i=%d: %w", i, err)
			}
			if cfg.Server.Port != 8080 {
				return fmt.Errorf("Load returned wrong port %d at i=%d", cfg.Server.Port, i)
			}
			return nil
		})
	t.Logf("config sustained Load p50=%.3fms p95=%.3fms p99=%.3fms", rep.P50Ms, rep.P95Ms, rep.P99Ms)
}

// TestConfigManager_Stress_ConcurrentGetUpdate hammers a single REAL
// ConfigManager with concurrent GetConfig / UpdateConfig / loadConfig from
// >=10 goroutines. Under -race this catches the unguarded m.config data race;
// the harness also asserts no deadlock and no goroutine leak.
func TestConfigManager_Stress_ConcurrentGetUpdate(t *testing.T) {
	dir := t.TempDir()
	primary := writeValidConfigFile(t, dir, "config.json")
	mgr, err := NewHelixConfigManager(primary)
	if err != nil {
		t.Fatalf("NewHelixConfigManager: %v", err)
	}

	stresschaos.RunConcurrent(t, "config_concurrent_get_update",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 200},
		func(goroutine, iter int) error {
			switch (goroutine + iter) % 4 {
			case 0, 1:
				cfg := mgr.GetConfig()
				if cfg == nil {
					return fmt.Errorf("nil config")
				}
				_ = cfg.Server.Port
				_ = cfg.Application.Name
			case 2:
				return mgr.UpdateConfig(func(c *Config) { c.Server.Port = 8000 + iter%1000 })
			default:
				return mgr.loadConfig()
			}
			return nil
		})
}

// TestConfigManager_Stress_Boundaries exercises boundary conditions against the
// REAL manager + validator: empty path rejection, no-pre-existing-file creation,
// and off-by-one port validation (0, 1, 65535, 65536).
func TestConfigManager_Stress_Boundaries(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "config_boundaries", "input-corruption")

	// empty config path -> watcher constructor must reject cleanly.
	if _, err := NewConfigWatcher(""); err == nil {
		rec.Record(stresschaos.Fatal, "NewConfigWatcher accepted empty path")
	} else {
		rec.Record(stresschaos.Degraded, "empty watcher path rejected: "+err.Error())
	}

	// No pre-existing file -> NewHelixConfigManager creates a default config.
	dir := t.TempDir()
	freshPath := filepath.Join(dir, "fresh.json")
	mgr, err := NewHelixConfigManager(freshPath)
	if err != nil {
		rec.Record(stresschaos.Fatal, "fresh manager creation failed: "+err.Error())
	} else if !mgr.IsConfigPresent() {
		rec.Record(stresschaos.Fatal, "fresh manager did not persist a default config file")
	} else {
		rec.Record(stresschaos.Recovered, "fresh manager created + persisted default config")
	}

	// Off-by-one port boundaries via the strict validator.
	validator := NewConfigurationValidator(true)
	for _, tc := range []struct {
		port  int
		valid bool
	}{
		{0, false}, {1, true}, {8080, true}, {65535, true}, {65536, false}, {-1, false},
	} {
		cfg := getDefaultConfig()
		cfg.Server.Port = tc.port
		res := validator.ValidateField(cfg, "server.port")
		if res.Valid != tc.valid {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("port %d: validator says valid=%v, want %v", tc.port, res.Valid, tc.valid))
		} else {
			rec.Record(stresschaos.Recovered, fmt.Sprintf("port %d boundary validated correctly (valid=%v)", tc.port, tc.valid))
		}
	}

	// Round-trip boundary: marshal a max-port config and reload it.
	cfg := getDefaultConfig()
	cfg.Server.Port = 65535
	data, _ := json.MarshalIndent(cfg, "", "  ")
	maxPath := filepath.Join(dir, "maxport.json")
	if err := os.WriteFile(maxPath, data, 0o644); err != nil {
		t.Fatalf("write maxport config: %v", err)
	}
	if err := mgr.ImportConfig(maxPath); err != nil {
		rec.Record(stresschaos.Degraded, "import maxport config errored: "+err.Error())
	} else if got := mgr.GetConfig().Server.Port; got != 65535 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("maxport round-trip lost value: got %d", got))
	} else {
		rec.Record(stresschaos.Recovered, "maxport (65535) round-tripped through import correctly")
	}

	rec.AssertNoFatal()
	t.Log("config manager survived boundary conditions")
}
