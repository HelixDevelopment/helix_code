package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// concurrencyTimeout scales a deadlock-guard timeout so the test stays a genuine
// deadlock detector without false-failing on CPU-starved runs. 12×80 = 960 real
// filesystem ops take ~52s isolated when only GOMAXPROCS=2 CPUs are available
// (e.g. the host-safe constrained validation run), yet finish in a few seconds
// with ample cores. The timeout is the deadlock GUARD — a real deadlock never
// completes within even the scaled bound, so the assertion is preserved. We pick
// a generous floor (120s) and additionally widen it when GOMAXPROCS is scarce.
func concurrencyTimeout(base time.Duration) time.Duration {
	const floor = 120 * time.Second
	scaled := base
	if procs := runtime.GOMAXPROCS(0); procs > 0 && procs < 8 {
		// Fewer cores ⇒ more wall-clock for the same op count. Scale inversely
		// against an 8-core reference, capped so it never grows unbounded.
		scaled = base * time.Duration(8) / time.Duration(procs)
	}
	if scaled < floor {
		return floor
	}
	return scaled
}

// §11.4.85 stress coverage for the REAL tool registry + real filesystem/shell
// tools.
//
// The unit under stress is the REAL *ToolRegistry's sync.RWMutex-guarded tools
// + aliases maps and the real Register / Get / List / GetSchema / Execute /
// ExecuteBatch machinery, plus the genuinely-implemented filesystem tools
// (fs_write/fs_read/fs_edit/glob via os.WriteFile/os.ReadFile under a
// t.TempDir() workspace) and the genuinely-implemented shell tool (real
// os/exec of FAST, SAFE, LOCAL commands — `true`, `echo` — only; never any
// destructive, network, or power-management command). No fakes are introduced
// for the concurrency surface: these *_test.go files run without the
// integration build tag (unit-test scope per CONST-050(A)), but every code
// path exercised here is the production registry/tool path.
//
// Coverage: sustained tool invocation (N>=100) + N>=10 concurrent
// registration/lookup/invocation + boundary conditions (empty/max/off-by-one).

// stressRegistry builds a real ToolRegistry whose filesystem + shell tools are
// confined to a per-test temp workspace so every fs op is a real, isolated
// os.WriteFile/os.ReadFile and every shell op runs in the sandbox under the
// temp dir. The registry is Close()d via t.Cleanup.
func stressRegistry(t *testing.T) (*ToolRegistry, string) {
	t.Helper()
	tmp := t.TempDir()
	cfg := DefaultRegistryConfig()
	cfg.FileSystemConfig.WorkspaceRoot = tmp
	cfg.MultiEditConfig.WorkspaceRoot = tmp
	cfg.ShellConfig.WorkDir = tmp
	r, err := NewToolRegistry(cfg)
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r, tmp
}

// TestToolRegistry_Stress_SustainedFileOps drives the REAL fs_write -> fs_read
// -> fs_edit lifecycle through the registry's Execute path under sustained load
// (N>=100), recording per-call latency. Each iteration writes a real file,
// reads it back (asserting the bytes round-trip), and edits it — proving real
// filesystem work happened on every iteration, not a no-op.
func TestToolRegistry_Stress_SustainedFileOps(t *testing.T) {
	r, tmp := stressRegistry(t)
	ctx := context.Background()

	var processed int64
	stresschaos.RunSustainedLoad(t, "tool_registry_sustained_file_ops",
		stresschaos.SustainedConfig{N: 400, MaxErrorRate: 0.0},
		func(i int) error {
			path := filepath.Join(tmp, fmt.Sprintf("stress_%d.txt", i))
			content := fmt.Sprintf("helix stress payload %d marker", i)

			if _, err := r.Execute(ctx, "fs_write", map[string]interface{}{
				"path": path, "content": content,
			}); err != nil {
				return fmt.Errorf("fs_write: %w", err)
			}
			res, err := r.Execute(ctx, "fs_read", map[string]interface{}{"path": path})
			if err != nil {
				return fmt.Errorf("fs_read: %w", err)
			}
			// Assert the real bytes round-tripped — the read must reflect the write.
			if got := readContent(res); got != content {
				return fmt.Errorf("round-trip mismatch: wrote %q read %q", content, got)
			}
			if _, err := r.Execute(ctx, "fs_edit", map[string]interface{}{
				"path": path, "old_string": "marker", "new_string": "EDITED",
			}); err != nil {
				return fmt.Errorf("fs_edit: %w", err)
			}
			atomic.AddInt64(&processed, 1)
			return nil
		})

	if atomic.LoadInt64(&processed) == 0 {
		t.Fatal("registry processed zero file ops under sustained load — not real work")
	}
	t.Logf("tool_registry sustained: %d write+read+edit cycles", atomic.LoadInt64(&processed))
}

// TestToolRegistry_Stress_SustainedShellExec drives the REAL shell tool through
// the registry under sustained load, executing only FAST, SAFE, LOCAL commands
// (`true`). Each invocation is a genuine os/exec via the shell executor; we
// assert a real ExecutionResult with exit code 0 comes back so the run proves
// real process execution (anti-BLUFF-003), not a print-and-sleep.
func TestToolRegistry_Stress_SustainedShellExec(t *testing.T) {
	r, _ := stressRegistry(t)
	ctx := context.Background()

	var ran int64
	stresschaos.RunSustainedLoad(t, "tool_registry_sustained_shell_exec",
		stresschaos.SustainedConfig{N: 150, MaxErrorRate: 0.0},
		func(i int) error {
			res, err := r.Execute(ctx, "shell", map[string]interface{}{
				"command": "true",
				"timeout": 5,
			})
			if err != nil {
				return fmt.Errorf("shell exec: %w", err)
			}
			if !shellExitedZero(res) {
				return fmt.Errorf("shell did not exit 0: %+v", res)
			}
			atomic.AddInt64(&ran, 1)
			return nil
		})

	if atomic.LoadInt64(&ran) == 0 {
		t.Fatal("registry ran zero shell commands under sustained load — not real exec")
	}
	t.Logf("tool_registry sustained: %d real `true` executions", atomic.LoadInt64(&ran))
}

// TestToolRegistry_Stress_ConcurrentRegisterLookup hammers the registry's
// RWMutex-guarded maps from N>=10 goroutines doing genuinely contending work:
// concurrent Register (write lock), Get/List/GetSchema (read lock), and
// RegisterAlias (write lock). Run under -race this proves the registry's shared
// state has no data race, no deadlock, and no goroutine leak under churn.
func TestToolRegistry_Stress_ConcurrentRegisterLookup(t *testing.T) {
	r, _ := stressRegistry(t)

	var ops int64
	stresschaos.RunConcurrent(t, "tool_registry_concurrent_register_lookup",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 150, Timeout: 25 * time.Second},
		func(g, it int) error {
			// Write-lock churn: register a uniquely-named probe tool.
			name := fmt.Sprintf("stress_probe_%d_%d", g, it)
			r.Register(&scProbeTool{name: name})

			// Read-lock churn against a definitely-present built-in tool.
			if _, err := r.Get("fs_read"); err != nil {
				return fmt.Errorf("get fs_read: %w", err)
			}
			if _, err := r.GetSchema("fs_write"); err != nil {
				return fmt.Errorf("get schema fs_write: %w", err)
			}
			// List takes the read lock and copies the whole map under contention.
			if tools := r.List(); len(tools) == 0 {
				return fmt.Errorf("List returned empty under contention")
			}
			// Read-lock churn against the just-written probe (visibility).
			if _, err := r.Get(name); err != nil {
				return fmt.Errorf("get own probe %q: %w", name, err)
			}
			// Alias write-lock churn + lookup-by-alias read path.
			alias := "alias_" + name
			if err := r.RegisterAlias(alias, name); err != nil {
				return fmt.Errorf("register alias: %w", err)
			}
			if _, err := r.Get(alias); err != nil {
				return fmt.Errorf("get by alias %q: %w", alias, err)
			}
			atomic.AddInt64(&ops, 1)
			return nil
		})

	if atomic.LoadInt64(&ops) == 0 {
		t.Fatal("zero registry ops completed under concurrent load")
	}
	t.Logf("tool_registry concurrent: %d register+lookup+alias cycles", atomic.LoadInt64(&ops))
}

// TestToolRegistry_Stress_ConcurrentExecute hammers the registry's Execute path
// from N>=10 goroutines doing real, non-conflicting filesystem work — each
// goroutine writes + reads ITS OWN files so the work is genuinely concurrent
// (disjoint paths) yet shares the single registry's lock + tool instances. Run
// under -race to catch any shared-state race in the Execute pipeline.
func TestToolRegistry_Stress_ConcurrentExecute(t *testing.T) {
	r, tmp := stressRegistry(t)
	ctx := context.Background()

	var done int64
	stresschaos.RunConcurrent(t, "tool_registry_concurrent_execute",
		stresschaos.ConcurrencyConfig{Parallelism: 12, IterationsPerGoroutine: 80, Timeout: concurrencyTimeout(25 * time.Second)},
		func(g, it int) error {
			path := filepath.Join(tmp, fmt.Sprintf("conc_%d_%d.txt", g, it))
			content := fmt.Sprintf("g%d-it%d", g, it)
			if _, err := r.Execute(ctx, "fs_write", map[string]interface{}{
				"path": path, "content": content,
			}); err != nil {
				return fmt.Errorf("fs_write: %w", err)
			}
			res, err := r.Execute(ctx, "fs_read", map[string]interface{}{"path": path})
			if err != nil {
				return fmt.Errorf("fs_read: %w", err)
			}
			if got := readContent(res); got != content {
				return fmt.Errorf("concurrent round-trip mismatch on %s: %q != %q", path, got, content)
			}
			atomic.AddInt64(&done, 1)
			return nil
		})

	if atomic.LoadInt64(&done) == 0 {
		t.Fatal("zero concurrent Execute round-trips completed")
	}
	t.Logf("tool_registry concurrent Execute: %d write+read round-trips", atomic.LoadInt64(&done))
}

// TestToolRegistry_Stress_BoundaryConditions exercises the §11.4.85 boundary
// category (empty / max / off-by-one) against the REAL registry, asserting each
// edge degrades into a clean error or a correct result — never a panic.
func TestToolRegistry_Stress_BoundaryConditions(t *testing.T) {
	r, tmp := stressRegistry(t)
	ctx := context.Background()

	t.Run("empty_batch", func(t *testing.T) {
		// EMPTY: zero-length batch must return an empty result slice, no panic.
		out := r.ExecuteBatch(ctx, nil, 0)
		if len(out) != 0 {
			t.Fatalf("empty batch returned %d results, want 0", len(out))
		}
	})

	t.Run("unknown_tool_errors_cleanly", func(t *testing.T) {
		// OFF-BY-ONE-ish: a name that is one char off a real tool must error,
		// not panic, and not match anything.
		if _, err := r.Execute(ctx, "fs_reads", map[string]interface{}{"path": "x"}); err == nil {
			t.Fatal("unknown tool 'fs_reads' did not error")
		}
		if _, err := r.Get(""); err == nil {
			t.Fatal("empty tool name resolved to a tool")
		}
	})

	t.Run("empty_content_write", func(t *testing.T) {
		// EMPTY: writing a zero-byte file then reading it must yield empty content.
		p := filepath.Join(tmp, "empty.txt")
		if _, err := r.Execute(ctx, "fs_write", map[string]interface{}{
			"path": p, "content": "",
		}); err != nil {
			t.Fatalf("write empty file: %v", err)
		}
		res, err := r.Execute(ctx, "fs_read", map[string]interface{}{"path": p})
		if err != nil {
			t.Fatalf("read empty file: %v", err)
		}
		if got := readContent(res); got != "" {
			t.Fatalf("empty file read back non-empty: %q", got)
		}
	})

	t.Run("max_large_payload", func(t *testing.T) {
		// MAX: a 1 MiB payload must round-trip exactly (real os.WriteFile path).
		p := filepath.Join(tmp, "big.txt")
		big := make([]byte, 1<<20)
		for i := range big {
			big[i] = byte('a' + (i % 26))
		}
		bigStr := string(big)
		if _, err := r.Execute(ctx, "fs_write", map[string]interface{}{
			"path": p, "content": bigStr,
		}); err != nil {
			t.Fatalf("write 1MiB: %v", err)
		}
		res, err := r.Execute(ctx, "fs_read", map[string]interface{}{"path": p})
		if err != nil {
			t.Fatalf("read 1MiB: %v", err)
		}
		if got := readContent(res); len(got) != len(bigStr) {
			t.Fatalf("1MiB round-trip length mismatch: got %d want %d", len(got), len(bigStr))
		}
	})

	t.Run("batch_max_concurrency_floor", func(t *testing.T) {
		// MAX/OFF-BY-ONE: a single read-only batch element with maxConcurrency<=0
		// must degrade to the default bound and still produce one result.
		p := filepath.Join(tmp, "batch.txt")
		if _, err := r.Execute(ctx, "fs_write", map[string]interface{}{
			"path": p, "content": "batch",
		}); err != nil {
			t.Fatalf("seed batch file: %v", err)
		}
		out := r.ExecuteBatch(ctx, []ToolCallRequest{
			{ID: "1", Name: "fs_read", Params: map[string]interface{}{"path": p}},
		}, -5)
		if len(out) != 1 {
			t.Fatalf("single-element batch returned %d results", len(out))
		}
		if out[0].Err != nil {
			t.Fatalf("single-element batch errored: %v", out[0].Err)
		}
	})
}
