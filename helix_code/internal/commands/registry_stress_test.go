package commands

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 stress coverage for the slash-command machinery.
//
// The units under stress are the REAL *Registry (RWMutex-guarded commands +
// aliases maps), the REAL *Parser (regexp slash-command parsing + quote-aware
// arg/flag splitting), and the REAL *Executor (parse -> registry lookup ->
// real Command.Execute dispatch). No fakes of the system: the command type
// used below (countingCommand) is a genuine, fully-implemented Command — it
// does real atomic work in Execute and returns a real *CommandResult — so
// every PASS proves real registration, lookup, parse, and dispatch happened.
//
// Coverage:
//   - sustained parse load (N>=100, p50/p95/p99 captured) against the real Parser
//   - sustained register/lookup/dispatch load against the real Executor
//   - N>=10 concurrent goroutines hammering the shared RWMutex-guarded registry
//     with interleaved Register / Get / List / Count / Unregister + parse +
//     dispatch (run under -race to catch data races on the maps)
//   - boundary conditions: empty command, unknown command, many commands

// countingCommand is a real Command implementation (NOT a mock — it performs
// real work and returns a real result). It counts its own dispatches via an
// atomic so tests can prove the Executor truly routed to it.
type countingCommand struct {
	name     string
	aliases  []string
	dispatch *int64
}

func (c *countingCommand) Name() string        { return c.name }
func (c *countingCommand) Aliases() []string    { return c.aliases }
func (c *countingCommand) Description() string  { return "stress-chaos counting command " + c.name }
func (c *countingCommand) Usage() string        { return "/" + c.name + " [args]" }
func (c *countingCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	if c.dispatch != nil {
		atomic.AddInt64(c.dispatch, 1)
	}
	// Touch the parsed context to prove real data flowed through the dispatch.
	return &CommandResult{
		Success: true,
		Message: c.name,
		Output:  fmt.Sprintf("args=%d flags=%d", len(cc.Args), len(cc.Flags)),
	}, nil
}

// TestCommands_Stress_SustainedParse drives the REAL Parser under sustained
// load (N>=100), recording per-call latency. Each iteration parses a real
// slash command with quoted args and flags and asserts the parse extracted the
// expected command name + a non-empty arg/flag set — so the run proves real
// parsing work, not a no-op.
func TestCommands_Stress_SustainedParse(t *testing.T) {
	p := NewParser()

	var parsed int64
	stresschaos.RunSustainedLoad(t, "commands_sustained_parse",
		stresschaos.SustainedConfig{N: 2000, MaxErrorRate: 0.0},
		func(i int) error {
			// Trailing boolean flag (--force) with no following token, so the
			// parser's look-ahead records it as "true"; --env=prod is an explicit
			// key=value; the quoted token is a real quote-aware argument.
			input := fmt.Sprintf(`/deploy target%d --env=prod "quoted arg %d" --force`, i, i)
			name, args, flags, isCmd := p.Parse(input)
			if !isCmd {
				return fmt.Errorf("parse %q reported not-a-command", input)
			}
			if name != "deploy" {
				return fmt.Errorf("parsed command name %q, want deploy", name)
			}
			// Two args: the bare "targetN" and the quote-merged "quoted arg N".
			if len(args) != 2 {
				return fmt.Errorf("parsed %d args from %q, want 2 (%v)", len(args), input, args)
			}
			if args[1] != fmt.Sprintf("quoted arg %d", i) {
				return fmt.Errorf("quote-aware split failed: arg[1]=%q", args[1])
			}
			if flags["env"] != "prod" || flags["force"] != "true" {
				return fmt.Errorf("flags not parsed: %v", flags)
			}
			atomic.AddInt64(&parsed, 1)
			return nil
		})

	if atomic.LoadInt64(&parsed) == 0 {
		t.Fatal("parser parsed zero commands under sustained load — not real work")
	}
	t.Logf("commands sustained parse: %d commands parsed", atomic.LoadInt64(&parsed))
}

// TestCommands_Stress_SustainedRegisterLookupDispatch drives the REAL
// Registry+Executor under sustained load (N>=100). Each iteration registers a
// fresh real command, looks it up, dispatches to it through the Executor, then
// unregisters it — asserting the dispatch count incremented for THIS iteration
// so the run proves real route-and-execute work.
func TestCommands_Stress_SustainedRegisterLookupDispatch(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)
	ctx := context.Background()

	var dispatched int64
	var iterations int64
	stresschaos.RunSustainedLoad(t, "commands_sustained_register_lookup_dispatch",
		stresschaos.SustainedConfig{N: 1200, MaxErrorRate: 0.0},
		func(i int) error {
			name := "cmd" + strconv.Itoa(i)
			cmd := &countingCommand{name: name, dispatch: &dispatched}
			if err := registry.Register(cmd); err != nil {
				return fmt.Errorf("register: %w", err)
			}
			if got, ok := registry.Get(name); !ok || got.Name() != name {
				return fmt.Errorf("lookup of %q failed (ok=%v)", name, ok)
			}
			before := atomic.LoadInt64(&dispatched)
			res, err := executor.Execute(ctx, "/"+name+" a b --k=v", nil)
			if err != nil {
				return fmt.Errorf("execute: %w", err)
			}
			if res == nil || !res.Success {
				return fmt.Errorf("execute returned non-success result: %+v", res)
			}
			if atomic.LoadInt64(&dispatched)-before != 1 {
				return fmt.Errorf("dispatch count did not advance by 1 for %q", name)
			}
			registry.Unregister(name)
			if _, ok := registry.Get(name); ok {
				return fmt.Errorf("command %q still present after Unregister", name)
			}
			atomic.AddInt64(&iterations, 1)
			return nil
		})

	if atomic.LoadInt64(&iterations) == 0 {
		t.Fatal("executor dispatched zero commands under sustained load")
	}
	t.Logf("commands sustained dispatch: %d full register/lookup/dispatch/unregister cycles, %d total dispatches",
		atomic.LoadInt64(&iterations), atomic.LoadInt64(&dispatched))
}

// TestCommands_Stress_ConcurrentRegistryAccess hammers the shared
// RWMutex-guarded registry from N>=10 concurrent goroutines that interleave
// Register / Get / List / Count / ListNames / GetAllHelp / Unregister plus
// real Parser parses and real Executor dispatches. Asserts no deadlock, no
// goroutine leak, no data race (run under -race), and no error.
//
// Each goroutine owns a disjoint command-name namespace (g-prefixed) so its
// own register/unregister cycle is deterministic, while all goroutines still
// genuinely contend on the SAME maps + the SAME mutex (read-write contention).
func TestCommands_Stress_ConcurrentRegistryAccess(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry)
	parser := NewParser()
	ctx := context.Background()

	var dispatched int64
	var ops int64
	stresschaos.RunConcurrent(t, "commands_concurrent_registry_access",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 150, Timeout: 25 * time.Second},
		func(g, it int) error {
			name := fmt.Sprintf("g%dc%d", g, it)
			cmd := &countingCommand{name: name, dispatch: &dispatched}

			if err := registry.Register(cmd); err != nil {
				return fmt.Errorf("register %q: %w", name, err)
			}

			// Real parse of this goroutine's command string.
			pname, _, _, isCmd := parser.Parse("/" + name + " --flag")
			if !isCmd || pname != name {
				return fmt.Errorf("parse %q -> (%q,%v)", name, pname, isCmd)
			}

			// Real dispatch through the executor (read-locks the registry).
			res, err := executor.Execute(ctx, "/"+name+" arg", nil)
			if err != nil {
				return fmt.Errorf("execute %q: %w", name, err)
			}
			if res == nil || !res.Success {
				return fmt.Errorf("execute %q non-success", name)
			}

			// Read-only accessors widen the RLock contention surface against
			// the concurrent writers (other goroutines registering/unregistering).
			_ = registry.Count()
			_ = registry.ListNames()
			_ = registry.List()
			_ = registry.GetAllHelp()

			// Clean up this goroutine's own command (write-locks again).
			registry.Unregister(name)
			atomic.AddInt64(&ops, 1)
			return nil
		})

	if atomic.LoadInt64(&dispatched) == 0 {
		t.Fatal("zero dispatches under concurrent load — registry/executor did no real work")
	}
	t.Logf("commands concurrent: %d ops, %d dispatches, final registry count=%d",
		atomic.LoadInt64(&ops), atomic.LoadInt64(&dispatched), registry.Count())
}

// TestCommands_Stress_BoundaryConditions exercises §11.4.85(A)(3) boundary
// cases against the real Parser/Registry/Executor: empty / bare-slash input
// (must be rejected cleanly), unknown command (clean error, no panic), and a
// large registry fan-in (many commands all resolvable).
func TestCommands_Stress_BoundaryConditions(t *testing.T) {
	ctx := context.Background()

	t.Run("empty_and_bare_slash", func(t *testing.T) {
		p := NewParser()
		for _, in := range []string{"", "   ", "/", "not a command", "hello /world"} {
			if _, _, _, isCmd := p.Parse(in); isCmd {
				t.Fatalf("parser treated %q as a command — should not", in)
			}
		}
	})

	t.Run("unknown_command_clean_error", func(t *testing.T) {
		registry := NewRegistry()
		executor := NewExecutor(registry)
		// Unknown command: must return an error, never panic, never nil-nil.
		res, err := executor.Execute(ctx, "/doesnotexist arg", nil)
		if err == nil {
			t.Fatalf("expected error for unknown command, got result %+v", res)
		}
		if res != nil {
			t.Fatalf("expected nil result for unknown command, got %+v", res)
		}
	})

	t.Run("many_commands_all_resolvable", func(t *testing.T) {
		registry := NewRegistry()
		executor := NewExecutor(registry)
		const many = 1000
		var dispatched int64
		for i := 0; i < many; i++ {
			name := "bcmd" + strconv.Itoa(i)
			if err := registry.Register(&countingCommand{name: name, dispatch: &dispatched}); err != nil {
				t.Fatalf("register %q: %v", name, err)
			}
		}
		if registry.Count() != many {
			t.Fatalf("registry count %d, want %d", registry.Count(), many)
		}
		// Every one must be resolvable + dispatchable.
		for i := 0; i < many; i++ {
			name := "bcmd" + strconv.Itoa(i)
			res, err := executor.Execute(ctx, "/"+name, nil)
			if err != nil || res == nil || !res.Success {
				t.Fatalf("dispatch %q failed: err=%v res=%+v", name, err, res)
			}
		}
		if atomic.LoadInt64(&dispatched) != many {
			t.Fatalf("dispatched %d/%d commands", atomic.LoadInt64(&dispatched), many)
		}
	})
}
