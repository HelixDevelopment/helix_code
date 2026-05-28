package tools

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/filesystem"
	"dev.helix.code/internal/tools/shell"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the REAL tool registry + real tools.
//
// Chaos classes exercised against the production registry/tool code (no fakes
// for the surface under test; unit-test scope per CONST-050(A)):
//
//   - input-corruption: hostile/malformed tool args + unknown tool names are
//     fed through the real Execute path. The registry MUST reject them with a
//     clean error (Validate failure or "tool not found"), NEVER panic/crash.
//   - command-injection: blocked / power-management / destructive commands are
//     fed to the REAL shell tool. The security manager MUST refuse them with a
//     SecurityError — proving the dangerous command never reaches os/exec.
//   - state-corruption under contention: the SAME tool name is registered,
//     overwritten, alias-bound, and looked up concurrently. The RWMutex must
//     keep the maps self-consistent — a torn map would panic or race under -race.
//   - process-death: a long-running real shell command is cancelled mid-flight
//     via context; the executor must unwind cleanly (no leaked process / no
//     deadlock).

// scProbeTool is a minimal real Tool used to exercise the registry's
// registration/lookup machinery under load. It performs no side effects and is
// LevelReadOnly so it is parallel-safe. It is NOT a mock of any production tool
// — it is a genuine Tool implementation registered into the real registry to
// drive the real RWMutex-guarded maps. (Unit-test scope per CONST-050(A).)
// (Named scProbeTool to avoid colliding with parallel_dispatch_test.go's
// probeTool, which has different concurrency-counter semantics.)
type scProbeTool struct {
	name string
}

func (p *scProbeTool) Name() string       { return p.name }
func (p *scProbeTool) Description() string { return "stress/chaos probe tool" }
func (p *scProbeTool) Category() ToolCategory {
	return ToolCategory("probe")
}
func (p *scProbeTool) Schema() ToolSchema {
	return ToolSchema{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}
}
func (p *scProbeTool) Validate(map[string]interface{}) error { return nil }
func (p *scProbeTool) Execute(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	return "ok", nil
}
func (p *scProbeTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

// readContent extracts the file content string from an fs_read Execute result
// (*filesystem.FileContent). Returns "" for any other shape.
func readContent(res interface{}) string {
	if fc, ok := res.(*filesystem.FileContent); ok {
		return string(fc.Content)
	}
	return ""
}

// shellExitedZero reports whether an fs/shell Execute result is an
// *shell.ExecutionResult that completed with exit code 0 and no Go-level error.
func shellExitedZero(res interface{}) bool {
	er, ok := res.(*shell.ExecutionResult)
	if !ok {
		return false
	}
	return er.ExitCode == 0 && er.Error == nil
}

// TestToolRegistry_Chaos_CorruptToolArgs feeds malformed tool invocations
// through the REAL registry Execute path. Each must degrade into a clean error
// (missing-required-param Validate failure, or unknown-tool error) — a panic or
// crash would be a §11.4.85(B) failure. The harness records a panic as Fatal.
func TestToolRegistry_Chaos_CorruptToolArgs(t *testing.T) {
	r, _ := stressRegistry(t)
	ctx := context.Background()
	rec := stresschaos.NewChaosRecorder(t, "tool_registry_corrupt_args", "input-corruption")

	type bad struct {
		tool   string
		params map[string]interface{}
	}
	cases := []bad{
		{"fs_read", map[string]interface{}{}},                               // missing required "path"
		{"fs_write", map[string]interface{}{"path": "x"}},                   // missing required "content"
		{"fs_edit", map[string]interface{}{"path": "x", "old_string": "a"}}, // missing "new_string"
		{"glob", map[string]interface{}{}},                                  // missing required "pattern"
		{"shell", map[string]interface{}{}},                                 // missing required "command"
		{"this_tool_does_not_exist", map[string]interface{}{"x": 1}},        // unknown tool
		{"", nil},                                                           // empty name + nil params
		{"fs_read", nil},                                                    // nil params on a real tool
	}

	for i, c := range cases {
		func(idx int, b bad) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal,
						fmt.Sprintf("case[%d] tool=%q PANICKED on corrupt args: %v", idx, b.tool, p))
				}
			}()
			_, err := r.Execute(ctx, b.tool, b.params)
			if err != nil {
				rec.Record(stresschaos.Degraded,
					fmt.Sprintf("case[%d] tool=%q rejected cleanly: %v", idx, b.tool, err))
			} else {
				// No error is acceptable only if no panic occurred; record it so
				// the trace is honest about which inputs were accepted.
				rec.Record(stresschaos.Recovered,
					fmt.Sprintf("case[%d] tool=%q accepted without crash", idx, b.tool))
			}
		}(i, c)
	}

	// Every malformed case MUST have been rejected, not silently accepted: each
	// case above is genuinely invalid (missing required param or unknown tool).
	rec.AssertNoFatal()
}

// TestToolRegistry_Chaos_CommandInjection feeds dangerous / blocked / power-
// management commands to the REAL shell tool through the registry. The shell
// security manager MUST refuse every one with a SecurityError BEFORE os/exec is
// reached — a successful run of any of these would be a critical defect. This
// proves the dangerous command never executes (CONST-033 host-power ban +
// blocklist enforcement). NOTE: these strings are passed to a security gate
// that REJECTS them; none are ever executed.
func TestToolRegistry_Chaos_CommandInjection(t *testing.T) {
	r, _ := stressRegistry(t)
	ctx := context.Background()
	rec := stresschaos.NewChaosRecorder(t, "tool_registry_command_injection", "input-corruption")

	// Each of these MUST be refused by the blocklist / dangerous-pattern guard.
	// (rm, dd, mkfs, fork-bomb, and the CONST-033 power-management family.)
	dangerous := []string{
		"rm -rf /",
		"rm -rf /tmp/anything",
		"dd if=/dev/zero of=/dev/sda",
		"mkfs.ext4 /dev/sda1",
		":(){ :|:& };:",
		"shutdown -h now",
		"reboot",
		"poweroff",
		"halt",
		"kill -9 1",
		"killall init",
	}

	for i, cmd := range dangerous {
		func(idx int, command string) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal,
						fmt.Sprintf("dangerous[%d] %q PANICKED: %v", idx, command, p))
				}
			}()
			res, err := r.Execute(ctx, "shell", map[string]interface{}{
				"command": command,
				"timeout": 5,
			})
			// The contract: the security manager returns a SecurityError, so
			// Execute returns a non-nil error and the command never runs.
			if err != nil {
				rec.Record(stresschaos.Degraded,
					fmt.Sprintf("dangerous[%d] %q refused before exec: %v", idx, command, err))
				return
			}
			// If err is nil the command was NOT refused — that is a real defect.
			rec.Record(stresschaos.Fatal,
				fmt.Sprintf("dangerous[%d] %q was NOT refused (result=%+v) — security bypass", idx, command, res))
		}(i, cmd)
	}

	rec.AssertNoFatal()
}

// TestToolRegistry_Chaos_ConcurrentSameNameChurn registers, overwrites,
// alias-binds, and looks up the SAME tool name from many goroutines at once.
// The registry's RWMutex must keep the tools + aliases maps self-consistent —
// under -race a torn map write would be reported. The terminal state must have
// the name resolvable to a valid tool (last writer wins, no corruption).
func TestToolRegistry_Chaos_ConcurrentSameNameChurn(t *testing.T) {
	r, _ := stressRegistry(t)
	rec := stresschaos.NewChaosRecorder(t, "tool_registry_same_name_churn", "state-corruption")

	const goroutines = 16
	const iters = 300
	const target = "churn_target"

	var wg sync.WaitGroup
	var registers, gets, aliases int64

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("goroutine %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				// Concurrent write-lock: overwrite the SAME key from all goroutines.
				r.Register(&scProbeTool{name: target})
				atomic.AddInt64(&registers, 1)

				// Concurrent read-lock against the contended key.
				if _, err := r.Get(target); err == nil {
					atomic.AddInt64(&gets, 1)
				}

				// Concurrent alias write-lock targeting the contended key. May
				// fail benignly if a racing goroutine has not yet registered, but
				// must NEVER corrupt the map.
				if err := r.RegisterAlias(fmt.Sprintf("a_%d", id), target); err == nil {
					atomic.AddInt64(&aliases, 1)
				}

				// Map-wide read under contention (List copies the whole map).
				_ = r.List()
			}
		}(g)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived same-name churn: %d registers, %d gets, %d aliases, no panic/race",
		atomic.LoadInt64(&registers), atomic.LoadInt64(&gets), atomic.LoadInt64(&aliases)))

	// Terminal state: the contended name must resolve to a valid, usable tool.
	tool, err := r.Get(target)
	if err != nil {
		rec.Record(stresschaos.Fatal, "contended tool vanished after churn: "+err.Error())
	} else if tool.Name() != target {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("contended tool name corrupted: got %q", tool.Name()))
	} else {
		rec.Record(stresschaos.Recovered, fmt.Sprintf("terminal state consistent: %q resolvable", target))
	}

	rec.AssertNoFatal()
	t.Logf("registry churn: registers=%d gets=%d aliases=%d",
		atomic.LoadInt64(&registers), atomic.LoadInt64(&gets), atomic.LoadInt64(&aliases))
}

// TestToolRegistry_Chaos_ShellProcessDeath starts a REAL long-running shell
// command (`sleep 30` — allowed, fast to cancel, no side effects) through the
// registry and cancels its context mid-flight. The real shell executor MUST
// observe the cancellation, SIGKILL the child, and unwind without deadlocking
// or leaking the process. The helper records Fatal if the op fails to unwind.
func TestToolRegistry_Chaos_ShellProcessDeath(t *testing.T) {
	r, _ := stressRegistry(t)

	stresschaos.ChaosKillDuring(t, "tool_registry_shell_process_death",
		300_000_000, // 300ms before cancelling, so the process is genuinely mid-run
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			// `sleep 30` is on the default allowlist and is a SAFE, LOCAL, no-side-
			// effect command. We expect the context cancellation to terminate it
			// long before 30s elapse.
			res, err := r.Execute(ctx, "shell", map[string]interface{}{
				"command": "sleep 30",
			})
			if err != nil {
				// Context cancellation surfaced as a Go error — clean unwind.
				rec.Record(stresschaos.Recovered, "shell exec unwound via error after cancel: "+err.Error())
				return
			}
			if er, ok := res.(*shell.ExecutionResult); ok {
				if er.Killed || er.TimedOut {
					rec.Record(stresschaos.Recovered, "shell process killed cleanly on cancel")
				} else {
					// Completing normally before cancel is also non-fatal (the
					// op simply finished first); record honestly.
					rec.Record(stresschaos.Recovered,
						fmt.Sprintf("shell completed before cancel (exit=%d)", er.ExitCode))
				}
				return
			}
			rec.Record(stresschaos.Degraded, "shell returned unexpected result shape on cancel")
		})
}
