package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the slash-command machinery.
//
// Chaos classes exercised against the REAL *Registry / *Parser / *Executor
// (no fakes — real registrations, real parsing, real dispatch, real
// RWMutex-guarded maps):
//
//   - command-panic injection: a registered Command whose Execute() panics
//     MUST NOT take down the dispatcher (and with it the whole host process).
//     The Executor MUST isolate the panic, surface it as a controlled error,
//     and remain usable for the next command. An unrecovered panic propagating
//     to the caller is a §11.4.85(B) Fatal (it crashes the CLI / server).
//   - input-corruption: structurally hostile command strings (control bytes,
//     embedded NULs, unterminated quotes, megabyte-long args, regexp-hostile
//     metacharacters, deeply nested flags) fed to the real Parser + Executor.
//     Parsing/dispatch MUST reject or normalise them, never crash.
//   - state-corruption under contention: the SAME command name is concurrently
//     Registered / Unregistered / Get / dispatched from many goroutines. The
//     registry mutex MUST serialise the map mutations so the registry never
//     panics or races and ends self-consistent.

// panickingCommand is a real Command whose Execute panics — used to prove the
// Executor isolates a misbehaving command. It is a genuine implementation of
// the Command interface (the system under test routes to it for real), not a
// mock of the dispatcher.
type panickingCommand struct {
	name  string
	panics *int64
}

func (c *panickingCommand) Name() string       { return c.name }
func (c *panickingCommand) Aliases() []string   { return nil }
func (c *panickingCommand) Description() string { return "chaos panicking command" }
func (c *panickingCommand) Usage() string       { return "/" + c.name }
func (c *panickingCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	if c.panics != nil {
		atomic.AddInt64(c.panics, 1)
	}
	panic(fmt.Sprintf("chaos: command %q exploded mid-execution", c.name))
}

// TestCommands_Chaos_CommandPanicIsolation registers a command that panics and
// dispatches it through the REAL Executor. The Executor MUST recover the panic
// and return a non-nil error rather than propagating it to the caller — an
// unrecovered panic in a slash-command dispatcher crashes the host CLI/server
// process along with every unrelated goroutine, which is a §11.4.85(B) Fatal.
// After the panic the dispatcher MUST still route a healthy command.
func TestCommands_Chaos_CommandPanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "commands_command_panic_isolation", "process-death")

	registry := NewRegistry()
	executor := NewExecutor(registry)
	ctx := context.Background()

	var panics int64
	if err := registry.Register(&panickingCommand{name: "boom", panics: &panics}); err != nil {
		t.Fatalf("register panicking command: %v", err)
	}

	// Drive the dispatch on a guarded goroutine: if the Executor does NOT
	// isolate the panic, it propagates here and we record Fatal. If it returns
	// an error instead, the dispatcher degraded gracefully (the desired path).
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal,
					fmt.Sprintf("executor propagated command panic to caller: %v — host process would crash", p))
			}
		}()
		res, err := executor.Execute(ctx, "/boom arg", nil)
		switch {
		case err != nil:
			rec.Record(stresschaos.Degraded,
				fmt.Sprintf("executor isolated panic, surfaced controlled error: %v", err))
		case res != nil && !res.Success:
			rec.Record(stresschaos.Degraded, "executor isolated panic, returned non-success result")
		default:
			rec.Record(stresschaos.Fatal,
				fmt.Sprintf("executor swallowed panic and reported success (res=%+v) — masks a crash", res))
		}
	}()

	// The command's Execute body MUST have actually run (proof we exercised the
	// real panic path, not a short-circuit).
	if atomic.LoadInt64(&panics) == 0 {
		rec.Record(stresschaos.Fatal, "panicking command Execute never ran — panic path not exercised")
	}

	// The dispatcher MUST remain usable after the panic — register and route a
	// healthy command successfully.
	var ok int64
	if err := registry.Register(&countingCommand{name: "healthy", dispatch: &ok}); err != nil {
		rec.Record(stresschaos.Fatal, "could not register after panic: "+err.Error())
	} else {
		res, err := executor.Execute(ctx, "/healthy x", nil)
		if err != nil || res == nil || !res.Success || atomic.LoadInt64(&ok) != 1 {
			rec.Record(stresschaos.Fatal,
				fmt.Sprintf("dispatcher unusable after panic: err=%v res=%+v dispatched=%d", err, res, atomic.LoadInt64(&ok)))
		} else {
			rec.Record(stresschaos.Recovered, "dispatcher still routes healthy commands after a command panic")
		}
	}

	rec.AssertNoFatal()
	t.Log("commands dispatcher survived command-panic injection and stayed usable")
}

// TestCommands_Chaos_CorruptCommandInput feeds structurally hostile command
// strings to the REAL Parser + Executor. Neither the regexp parse, the
// quote-aware arg splitter, nor the registry lookup may crash on control bytes,
// embedded NULs, unterminated quotes, oversized args, or regexp metacharacters
// — a crash on malformed input is a §11.4.85(B) failure. A registered command
// touches its parsed args so corrupt input that does parse flows through real
// dispatch.
func TestCommands_Chaos_CorruptCommandInput(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)
	parser := NewParser()
	ctx := context.Background()

	// A real command that genuinely consumes its parsed args/flags.
	var seen int64
	if err := registry.Register(&countingCommand{name: "sink", dispatch: &seen}); err != nil {
		t.Fatalf("register sink: %v", err)
	}

	corrupt := [][]byte{
		[]byte("/sink \x00\x01\x02 control bytes"),       // control bytes / NUL
		[]byte("/sink \"unterminated quote arg"),          // unbalanced quote
		[]byte("/sink 'mismatch\" quotes"),                // mismatched quotes
		[]byte("/sink " + strings.Repeat("A", 1<<20)),      // 1 MiB single arg
		[]byte("/sink --" + strings.Repeat("k", 4096) + "=v"), // huge flag key
		[]byte("/" + strings.Repeat("x", 8192)),            // huge command name
		[]byte("/sink .*+?[](){}|^$\\"),                     // regexp metacharacters
		[]byte("/sink --a --b --c --d --e --f --g --h"),    // many bare flags
		[]byte("/sink\t\t\ttabs\v\fvertical"),              // whitespace variants
		[]byte("/" + string([]rune{0x202E, 0x200B, 0xFEFF}) + " unicode controls"), // bidi/zero-width/BOM (built from code points, no hidden chars in source)
		[]byte("/sink " + strings.Repeat("\"q\" ", 5000)),                          // many quoted tokens
		[]byte(""),                                                                 // empty
	}

	stresschaos.ChaosCorruptInputDuring(t, "commands_corrupt_command_input", corrupt,
		func(input []byte) error {
			s := string(input)

			// 1) Parsing must never panic. (recover handled by the harness.)
			name, args, flags, isCmd := parser.Parse(s)
			_ = name
			_ = args
			_ = flags

			// 2) Dispatching must never panic regardless of how hostile the
			//    string is. A returned error (e.g. unknown / not-a-command) is
			//    the graceful-rejection path; a successful parse+dispatch of the
			//    "sink" command is acceptable normalisation. Either is non-fatal.
			res, err := executor.Execute(ctx, s, nil)
			if err != nil {
				// Graceful rejection — surface it so the harness records Degraded.
				return fmt.Errorf("rejected corrupt input (isCmd=%v): %w", isCmd, err)
			}
			_ = res
			return nil
		})
}

// TestCommands_Chaos_ConcurrentRegisterUnregisterChurn hammers the SAME
// command name with concurrent Register / Unregister / Get / Execute / List /
// Count from many goroutines. The real registry.mutex MUST serialise the map
// mutations so the registry never panics or races, and the registry MUST end
// self-consistent (a fresh register + dispatch still works). Run under -race.
func TestCommands_Chaos_ConcurrentRegisterUnregisterChurn(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "commands_register_unregister_churn", "state-corruption")

	registry := NewRegistry()
	executor := NewExecutor(registry)
	ctx := context.Background()

	const goroutines = 16
	const iters = 400
	const sharedName = "churn"

	var dispatched int64
	var regs, unregs, gets, execs int64
	var wg sync.WaitGroup

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
				switch (id + it) % 4 {
				case 0:
					// Register-all races against concurrent Unregisters; a
					// duplicate "already registered" error is the expected,
					// non-fatal serialised outcome.
					_ = registry.Register(&countingCommand{name: sharedName, dispatch: &dispatched})
					atomic.AddInt64(&regs, 1)
				case 1:
					registry.Unregister(sharedName)
					atomic.AddInt64(&unregs, 1)
				case 2:
					_, _ = registry.Get(sharedName)
					atomic.AddInt64(&gets, 1)
				default:
					// Dispatch reads the (concurrently mutating) registry; may
					// hit "command not found" if a peer just unregistered —
					// that is graceful, never a panic.
					_, _ = executor.Execute(ctx, "/"+sharedName+" a", nil)
					atomic.AddInt64(&execs, 1)
				}
				// Read-only accessors widen the RLock contention surface.
				_ = registry.Count()
				_ = registry.ListNames()
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived churn: regs=%d unregs=%d gets=%d execs=%d, no panic/race",
		atomic.LoadInt64(&regs), atomic.LoadInt64(&unregs),
		atomic.LoadInt64(&gets), atomic.LoadInt64(&execs)))

	// Registry must be self-consistent: count is non-negative and a fresh
	// register + dispatch of a NEW name still works.
	if registry.Count() < 0 {
		rec.Record(stresschaos.Fatal, "registry count went negative after churn")
	}
	registry.Unregister(sharedName) // normalise
	var finalHit int64
	freshName := "post-churn-" + strconv.Itoa(int(atomic.LoadInt64(&regs)))
	if err := registry.Register(&countingCommand{name: freshName, dispatch: &finalHit}); err != nil {
		rec.Record(stresschaos.Fatal, "cannot register fresh command after churn: "+err.Error())
	} else if res, err := executor.Execute(ctx, "/"+freshName, nil); err != nil || res == nil || !res.Success || atomic.LoadInt64(&finalHit) != 1 {
		rec.Record(stresschaos.Fatal,
			fmt.Sprintf("registry not self-consistent after churn: err=%v res=%+v hit=%d", err, res, atomic.LoadInt64(&finalHit)))
	} else {
		rec.Record(stresschaos.Recovered, "registry routes a fresh command correctly after churn — map self-consistent")
	}

	rec.AssertNoFatal()
	t.Logf("commands churn: regs=%d unregs=%d gets=%d execs=%d final-count=%d",
		atomic.LoadInt64(&regs), atomic.LoadInt64(&unregs),
		atomic.LoadInt64(&gets), atomic.LoadInt64(&execs), registry.Count())
}
