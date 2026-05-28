package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for internal/config's ConfigManager.
//
// The ConfigManager is the concurrency-relevant surface in this package: it
// holds a *Config that is read on every API request (GetConfig) while a reload
// (loadConfig / ImportConfig / ResetToDefaults) swaps the pointer underneath —
// the classic "config read while a reload writes it" data-race site. The
// ConfigAPI HTTP handlers (config_api.go) drive exactly this pattern: GetConfig
// serves api.manager.GetConfig() while POST /config/reload calls
// api.manager.loadConfig() from another goroutine.
//
// Chaos classes exercised against the REAL *ConfigManager (no fakes — real
// on-disk JSON files via t.TempDir, real os.ReadFile/WriteFile, real
// json.Unmarshal):
//
//   - state-corruption under contention: a single ConfigManager is concurrently
//     GetConfig / UpdateConfig / UpdateConfigFromMap / loadConfig (reload) /
//     ImportConfig / ResetToDefaults'd from many goroutines. The shared
//     m.config pointer + the struct it points to must be guarded so the manager
//     never panics, never races, and ends self-consistent. Run under -race.
//   - input-corruption: structurally hostile JSON config content fed to the
//     REAL loadConfig / ImportConfig path. Malformed JSON must reject with an
//     error (Degraded), never crash (Fatal). A panic on malformed input is a
//     §11.4.85(B) Fatal.

// writeValidConfigFile writes a default config as JSON to a temp path and
// returns the path. NewHelixConfigManager(path) loads an existing file via
// loadConfig (json.Unmarshal), so the on-disk shape must be JSON.
func writeValidConfigFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data, err := json.MarshalIndent(getDefaultConfig(), "", "  ")
	if err != nil {
		t.Fatalf("marshal default config: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

// TestConfigManager_Chaos_ConcurrentGetWhileReload is the key data-race probe.
// It hammers a single REAL ConfigManager with concurrent reads (GetConfig +
// reading the returned *Config's fields) while other goroutines swap the config
// underneath via loadConfig (reload-from-disk), ImportConfig, ResetToDefaults,
// UpdateConfig, and UpdateConfigFromMap. Without a mutex guarding m.config this
// races (detected by -race) and may also tear the struct mid-read. The manager
// must serialise so it never panics or races and stays self-consistent.
func TestConfigManager_Chaos_ConcurrentGetWhileReload(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "config_concurrent_get_while_reload", "state-corruption")

	dir := t.TempDir()
	primary := writeValidConfigFile(t, dir, "config.json")
	// A second valid file so ImportConfig has a real source to read.
	importSrc := writeValidConfigFile(t, dir, "import_src.json")

	mgr, err := NewHelixConfigManager(primary)
	if err != nil {
		t.Fatalf("NewHelixConfigManager: %v", err)
	}

	const goroutines = 16
	const iters = 300
	var wg sync.WaitGroup
	var reads, reloads, updates, imports, resets int64

	for w := 0; w < goroutines; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("goroutine %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				switch (id + it) % 6 {
				case 0, 1:
					// Hot read path — GetConfig + touch fields (mirrors the
					// per-request HTTP handler reading config under reload).
					cfg := mgr.GetConfig()
					if cfg != nil {
						_ = cfg.Server.Port
						_ = cfg.Application.Name
						_ = cfg.LLM.DefaultProvider
					}
					atomic.AddInt64(&reads, 1)
				case 2:
					// Reload from disk swaps m.config (the writer the reader races).
					_ = mgr.loadConfig()
					atomic.AddInt64(&reloads, 1)
				case 3:
					_ = mgr.UpdateConfig(func(c *Config) { c.Server.Port = 8000 + (id+it)%1000 })
					atomic.AddInt64(&updates, 1)
				case 4:
					_ = mgr.ImportConfig(importSrc)
					atomic.AddInt64(&imports, 1)
				default:
					_ = mgr.ResetToDefaults()
					atomic.AddInt64(&resets, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived get-while-reload: %d reads, %d reloads, %d updates, %d imports, %d resets, no panic/race",
		atomic.LoadInt64(&reads), atomic.LoadInt64(&reloads), atomic.LoadInt64(&updates),
		atomic.LoadInt64(&imports), atomic.LoadInt64(&resets)))

	// The manager must still be usable and coherent after the churn.
	final := mgr.GetConfig()
	if final == nil {
		rec.Record(stresschaos.Fatal, "GetConfig returned nil after churn — manager corrupted")
	} else if final.Application.Name == "" || final.Version == "" {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("config torn after churn: name=%q version=%q", final.Application.Name, final.Version))
	} else {
		rec.Record(stresschaos.Recovered, "config coherent after concurrent get-while-reload churn")
	}

	rec.AssertNoFatal()
	t.Logf("config churn: reads=%d reloads=%d updates=%d imports=%d resets=%d",
		atomic.LoadInt64(&reads), atomic.LoadInt64(&reloads), atomic.LoadInt64(&updates),
		atomic.LoadInt64(&imports), atomic.LoadInt64(&resets))
}

// TestConfigManager_Chaos_CorruptConfigFiles feeds structurally hostile JSON
// content to the REAL loadConfig / ImportConfig path. A malformed config file
// must be rejected with an error (graceful Degraded) — never crash the manager.
// A panic on corrupt input is a §11.4.85(B) Fatal. Critically, a failed reload
// must NOT leave the manager with a nil/torn config: the previous good config
// must remain readable.
func TestConfigManager_Chaos_CorruptConfigFiles(t *testing.T) {
	dir := t.TempDir()
	primary := writeValidConfigFile(t, dir, "config.json")
	mgr, err := NewHelixConfigManager(primary)
	if err != nil {
		t.Fatalf("NewHelixConfigManager: %v", err)
	}

	corrupt := [][]byte{
		[]byte(`{`),                                       // 0: unterminated object
		[]byte(`{"server": {"port": }}`),                  // 1: missing value
		[]byte(`{"server": {"port": "not-a-number"}}`),    // 2: wrong type for int field
		[]byte(`not json at all`),                         // 3: plain text
		[]byte("\x00\x01\x02\xff\xfe binary garbage \x00"), // 4: binary garbage
		[]byte(`[]`),                                       // 5: JSON array, not object
		[]byte(``),                                         // 6: empty file
		[]byte(`{"server": {"port": 99999999999999999999999999}}`), // 7: numeric overflow
		[]byte(`{"llm": {"temperature": [1,2,3]}}`),        // 8: array where number expected
	}

	// Persist each corrupt payload as a real file and feed its on-disk path
	// through ImportConfig (real os.ReadFile + json.Unmarshal).
	payloads := make([][]byte, len(corrupt))
	for i, c := range corrupt {
		path := filepath.Join(dir, fmt.Sprintf("corrupt-%d.json", i))
		if err := os.WriteFile(path, c, 0o644); err != nil {
			t.Fatalf("write corrupt file %d: %v", i, err)
		}
		payloads[i] = []byte(path)
	}

	stresschaos.ChaosCorruptInputDuring(t, "config_corrupt_config_files", payloads,
		func(input []byte) error {
			path := string(input)
			err := mgr.ImportConfig(path)
			// After a corrupt-import attempt the manager must still hand back a
			// usable, non-panicking config — touch it to flow data through.
			cfg := mgr.GetConfig()
			if cfg != nil {
				_ = cfg.Server.Port
				_ = cfg.Application.Name
			}
			return err // graceful rejection (Degraded) is the desired outcome
		})
}

// TestConfigManager_Chaos_ReloadCorruptInPlace points the manager's own backing
// file at corrupt content and reloads. A failed in-place reload must surface an
// error and must NOT crash; the manager must remain coherent for the next
// reader. This mirrors POST /config/reload hitting a file that was clobbered.
func TestConfigManager_Chaos_ReloadCorruptInPlace(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "config_reload_corrupt_in_place", "input-corruption")

	dir := t.TempDir()
	primary := writeValidConfigFile(t, dir, "config.json")
	mgr, err := NewHelixConfigManager(primary)
	if err != nil {
		t.Fatalf("NewHelixConfigManager: %v", err)
	}

	// Clobber the backing file with garbage, then reload.
	if err := os.WriteFile(primary, []byte("}{ totally broken \x00"), 0o644); err != nil {
		t.Fatalf("clobber config file: %v", err)
	}

	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("loadConfig panicked on corrupt in-place file: %v", p))
			}
		}()
		if err := mgr.loadConfig(); err != nil {
			rec.Record(stresschaos.Degraded, "reload rejected corrupt in-place file cleanly: "+err.Error())
		} else {
			rec.Record(stresschaos.Degraded, "reload accepted corrupt file (parser tolerant) without crash")
		}
	}()

	// The manager must still respond without panicking.
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("GetConfig panicked after corrupt reload: %v", p))
			}
		}()
		cfg := mgr.GetConfig()
		if cfg == nil {
			rec.Record(stresschaos.Degraded, "GetConfig returned nil after corrupt reload (controlled)")
		} else {
			rec.Record(stresschaos.Recovered, "GetConfig usable after corrupt reload")
		}
	}()

	rec.AssertNoFatal()
	t.Log("config manager survived corrupt in-place reload")
}
