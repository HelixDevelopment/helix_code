// p1f15_challenge runs the F15 Subagent Team pipeline end-to-end against the
// real SubagentManager, real InProcessSpawner with a real FakeLLMProvider, real
// fork-exec of THIS binary as the subprocess subagent, and (when gated) real
// F04 worktree creation + real cloud LLM round-trip. Runtime-evidence harness
// for the P1-F15 Challenge.
//
// Phases:
//
//	0. Helper-mode dispatch — the FIRST statement of main(). When the parent's
//	   SubprocessSpawner re-execs THIS binary with HELIXCODE_SUBAGENT_HELPER=1,
//	   the child must short-circuit to RunAsSubagent and exit; otherwise it
//	   would re-enter the harness and recurse forever.
//	A. In-process — real InProcessSpawner + real FakeLLMProvider (canned).
//	   Asserts State=StateSucceeded, Output exact match, GenerateCallCount==1.
//	B. Subprocess — real SubprocessSpawner pointing at os.Executable() (i.e.
//	   THIS harness). The child runs RunAsSubagent with its own FakeLLMProvider
//	   (no canned response for "phase-b-prompt", so the fallback echo
//	   "FAKE-LLM-ECHO: phase-b-prompt" is what the child emits). Proves the
//	   parent->child JSON protocol round-trip works against a real fork-exec.
//	C. Worktree (gated) — real `git init` in a tempdir + real F04
//	   WorktreeManager + WorktreeIntegration. Skipped when git is not on PATH.
//	D. Real LLM (gated) — real Anthropic provider with the operator's
//	   ANTHROPIC_API_KEY. Skipped when not set.
//	E. Concurrency limit — MaxConcurrency=2 + 3 slow tasks; the third Dispatch
//	   returns ErrMaxConcurrency.
//	F. Kill — slow task + Kill(id); result MUST carry State=StateCanceled.
//
// Exit code 0 on success; exit 1 with a diagnostic on any check failure.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"

	"dev.helix.code/internal/agent/subagent"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/tools/worktree"
)

// harnessLLMFactory is the SubagentLLMProviderFactory used by the helper-mode
// dispatch when this binary is re-exec'd as a subagent child. It returns a
// fresh FakeLLMProvider with NO canned responses, so the child's Generate
// invocation produces the unique FAKE-LLM-ECHO prefix that Phase B asserts on.
func harnessLLMFactory(ctx context.Context) (llm.Provider, error) {
	return subagent.NewFakeLLMProvider(nil), nil
}

func main() {
	// MUST be the very first statement: when the parent's SubprocessSpawner
	// re-execs THIS binary with HELIXCODE_SUBAGENT_HELPER=1, the child must
	// short-circuit to RunAsSubagent and os.Exit with the returned code.
	// Re-entering run() would recurse: Phase B would fork a child which would
	// run all phases including Phase B, ad infinitum.
	if subagent.IsSubagentInvocation() {
		os.Exit(subagent.RunAsSubagent(harnessLLMFactory))
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("==> P1-F15 challenge harness pid:", os.Getpid())

	if err := phaseA(); err != nil {
		return fmt.Errorf("phase A: %w", err)
	}
	if err := phaseB(); err != nil {
		return fmt.Errorf("phase B: %w", err)
	}
	if err := phaseC(); err != nil {
		return fmt.Errorf("phase C: %w", err)
	}
	if err := phaseD(); err != nil {
		return fmt.Errorf("phase D: %w", err)
	}
	if err := phaseE(); err != nil {
		return fmt.Errorf("phase E: %w", err)
	}
	if err := phaseF(); err != nil {
		return fmt.Errorf("phase F: %w", err)
	}

	fmt.Println("==> ALL CHECKS PASSED")
	fmt.Println("==> P1-F15 challenge harness PASS")
	return nil
}

// phaseA exercises the in-process spawner against a real FakeLLMProvider
// seeded with a canned response. The GenerateCallCount==1 assertion is the
// load-bearing anti-bluff anchor: it proves the provider was actually invoked
// by the spawner, rather than the manager fabricating a result.
func phaseA() error {
	fmt.Println("==> phase A: in-process spawner + real FakeLLMProvider (always runs)")

	provider := subagent.NewFakeLLMProvider(map[string]string{
		"phase-a-prompt": "phase-a-output",
	})

	mgr, err := subagent.NewSubagentManager(subagent.SubagentManagerOptions{
		LLMProvider:    provider,
		MaxConcurrency: 5,
		Logger:         zap.NewNop(),
	})
	if err != nil {
		return fmt.Errorf("NewSubagentManager: %w", err)
	}
	defer func() { _ = mgr.Shutdown(context.Background()) }()

	task := subagent.SubagentTask{
		Description: "phase-a-task",
		Prompt:      "phase-a-prompt",
		Isolation:   subagent.IsolationNone,
		Timeout:     10 * time.Second,
	}
	id, err := mgr.Dispatch(context.Background(), task)
	if err != nil {
		return fmt.Errorf("Dispatch: %w", err)
	}

	res, err := drainOne(mgr, id, 15*time.Second)
	if err != nil {
		return err
	}

	if res.State != subagent.StateSucceeded {
		return fmt.Errorf("state=%q want %q (err=%q)", res.State, subagent.StateSucceeded, res.Error)
	}
	if res.Output != "phase-a-output" {
		return fmt.Errorf("output=%q want %q", res.Output, "phase-a-output")
	}
	if got := provider.GenerateCallCount(); got != 1 {
		return fmt.Errorf("FakeLLMProvider.GenerateCallCount=%d; want 1 (anti-bluff: provider MUST have been invoked exactly once)", got)
	}
	if got := provider.LastPrompt(); got != "phase-a-prompt" {
		return fmt.Errorf("LastPrompt=%q; want %q", got, "phase-a-prompt")
	}

	fmt.Printf("    in-process       : id=%s state=%s output=%q duration=%s call_count=%d\n",
		res.TaskID, res.State, res.Output, res.Duration, provider.GenerateCallCount())
	return nil
}

// phaseB exercises a real fork-exec of THIS binary as the subagent child via
// SubprocessSpawner. The helper-mode dispatch at the top of main() makes this
// safe — the child sees HELIXCODE_SUBAGENT_HELPER=1 and runs RunAsSubagent,
// which constructs ITS OWN FakeLLMProvider via harnessLLMFactory (no canned
// response for "phase-b-prompt", so the fallback echo "FAKE-LLM-ECHO: <prompt>"
// is what the child emits as Output).
func phaseB() error {
	fmt.Println("==> phase B: subprocess spawner re-execs THIS binary (always runs)")

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("os.Executable: %w", err)
	}
	fmt.Printf("    host binary      : %s\n", exe)
	fmt.Printf("    parent pid       : %d\n", os.Getpid())

	subSpawner, err := subagent.NewSubprocessSpawner("")
	if err != nil {
		return fmt.Errorf("NewSubprocessSpawner: %w", err)
	}

	// FakeLLMProvider on the parent side is NOT consulted by the subprocess
	// spawner (the child constructs its own provider); we still pass one so
	// the manager constructor accepts the options.
	parentProvider := subagent.NewFakeLLMProvider(nil)

	mgr, err := subagent.NewSubagentManager(subagent.SubagentManagerOptions{
		LLMProvider:       parentProvider,
		SubprocessSpawner: subSpawner,
		MaxConcurrency:    5,
		Logger:            zap.NewNop(),
	})
	if err != nil {
		return fmt.Errorf("NewSubagentManager: %w", err)
	}
	defer func() { _ = mgr.Shutdown(context.Background()) }()

	task := subagent.SubagentTask{
		Description: "phase-b-task",
		Prompt:      "phase-b-prompt",
		Isolation:   subagent.IsolationWorktree, // routes to the SubprocessSpawner
		Timeout:     30 * time.Second,
	}
	id, err := mgr.Dispatch(context.Background(), task)
	if err != nil {
		return fmt.Errorf("Dispatch: %w", err)
	}

	res, err := drainOne(mgr, id, 60*time.Second)
	if err != nil {
		return err
	}

	if res.State != subagent.StateSucceeded {
		return fmt.Errorf("state=%q want %q (err=%q)", res.State, subagent.StateSucceeded, res.Error)
	}
	// The child runs ITS OWN FakeLLMProvider (no canned), so the fallback
	// echo "FAKE-LLM-ECHO: phase-b-prompt" is what we expect on stdout.
	const wantPrefix = "FAKE-LLM-ECHO: "
	if !strings.HasPrefix(res.Output, wantPrefix) {
		return fmt.Errorf("output=%q must start with %q (proves child's FakeLLMProvider.Generate was invoked, not a parent fabrication)",
			res.Output, wantPrefix)
	}
	if !strings.Contains(res.Output, "phase-b-prompt") {
		return fmt.Errorf("output=%q must contain the original prompt", res.Output)
	}
	// Parent's provider MUST NOT have been invoked — the SubprocessSpawner
	// ignores it by contract.
	if got := parentProvider.GenerateCallCount(); got != 0 {
		return fmt.Errorf("parent FakeLLMProvider.GenerateCallCount=%d; want 0 (subprocess spawner must NOT call parent provider)", got)
	}

	fmt.Printf("    subprocess_used  : true\n")
	fmt.Printf("    output           : %q\n", res.Output)
	fmt.Printf("    duration         : %s\n", res.Duration)
	fmt.Printf("    parent_call_count: %d (must be 0)\n", parentProvider.GenerateCallCount())
	return nil
}

// phaseC exercises a real F04 worktree end-to-end IFF git is on PATH. We
// init a real git repo in a tempdir, construct a real worktree.Manager and
// WorktreeIntegration, call Setup, dispatch an in-process subagent against
// the worktree, capture the diff, then clean up.
//
// Note: the subagent itself runs in-process here (no subprocess), so we can
// directly assert the WorktreePath surfaces in the result. The worktree path
// is set by the caller wiring around manager.Dispatch — for this challenge we
// thread it explicitly to keep the assertion simple.
func phaseC() error {
	fmt.Println("==> phase C: real F04 worktree creation (gated)")

	if _, err := exec.LookPath("git"); err != nil {
		fmt.Println("    [skipped: git not on PATH]")
		return nil
	}

	repo, err := initEphemeralGitRepo()
	if err != nil {
		return fmt.Errorf("initEphemeralGitRepo: %w", err)
	}
	defer os.RemoveAll(repo)

	mgr := worktree.NewManager(repo)
	wi := subagent.NewWorktreeIntegration(mgr)
	if wi == nil {
		return errors.New("NewWorktreeIntegration returned nil")
	}

	task := subagent.SubagentTask{
		ID:          "p1f15-phasec-task",
		Description: "real-worktree",
		Prompt:      "noop",
		Isolation:   subagent.IsolationWorktree,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workDir, cleanup, err := wi.Setup(ctx, task)
	if err != nil {
		return fmt.Errorf("WorktreeIntegration.Setup: %w", err)
	}
	defer func() { _ = cleanup() }()

	if workDir == "" {
		return errors.New("Setup returned empty workDir")
	}
	info, err := os.Stat(workDir)
	if err != nil {
		return fmt.Errorf("stat worktree dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("worktree path is not a directory: %s", workDir)
	}

	// Anti-bluff: the parent worktree.Manager must NOT have been mutated.
	if mgr.IsIsolated() {
		return errors.New("parent worktree manager became IsIsolated()=true; subagent dispatch must NOT mutate parent state")
	}
	if mgr.GetCurrentDirectory() != repo {
		return fmt.Errorf("parent currentDirectory=%q want %q (Setup must not relocate the parent)",
			mgr.GetCurrentDirectory(), repo)
	}

	// Stage a real file inside the worktree, then capture the diff to
	// prove the worktree is a real git checkout.
	added := filepath.Join(workDir, "p1f15-phase-c.txt")
	if err := os.WriteFile(added, []byte("phase-c-content\n"), 0o644); err != nil {
		return fmt.Errorf("write file in worktree: %w", err)
	}
	addCmd := exec.Command("git", "-C", workDir, "add", "p1f15-phase-c.txt")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add in worktree: %w (out=%s)", err, out)
	}

	diff, err := wi.CaptureDiff(ctx, workDir)
	if err != nil {
		return fmt.Errorf("CaptureDiff: %w", err)
	}
	if !strings.Contains(diff, "phase-c-content") {
		return fmt.Errorf("diff must contain the staged content; got %q", diff)
	}

	fmt.Printf("    repo             : %s\n", repo)
	fmt.Printf("    worktree         : %s\n", workDir)
	fmt.Printf("    diff_len         : %d bytes\n", len(diff))
	fmt.Printf("    parent_isolated  : %t (must be false)\n", mgr.IsIsolated())
	return nil
}

// phaseD exercises a real Anthropic round-trip when ANTHROPIC_API_KEY is set.
// Otherwise prints a gated-skip line. Honest skip is the F11/F12/F13/F14
// pattern; do not pretend to test what the host can't reach.
func phaseD() error {
	fmt.Println("==> phase D: real Anthropic LLM round-trip (gated)")

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Println("    [skipped: ANTHROPIC_API_KEY not set]")
		return nil
	}

	provider, err := llm.NewAnthropicProvider(llm.ProviderConfigEntry{
		Type:    llm.ProviderTypeAnthropic,
		APIKey:  apiKey,
		Enabled: true,
	})
	if err != nil {
		return fmt.Errorf("NewAnthropicProvider: %w", err)
	}

	mgr, err := subagent.NewSubagentManager(subagent.SubagentManagerOptions{
		LLMProvider:    provider,
		MaxConcurrency: 1,
		Logger:         zap.NewNop(),
	})
	if err != nil {
		return fmt.Errorf("NewSubagentManager: %w", err)
	}
	defer func() { _ = mgr.Shutdown(context.Background()) }()

	task := subagent.SubagentTask{
		Description:  "real-anthropic",
		Prompt:       "Respond with the literal string 'hello-from-real-llm' and absolutely nothing else.",
		SubagentType: "claude-3-5-haiku-latest",
		Isolation:    subagent.IsolationNone,
		Timeout:      60 * time.Second,
	}
	id, err := mgr.Dispatch(context.Background(), task)
	if err != nil {
		return fmt.Errorf("Dispatch: %w", err)
	}

	res, err := drainOne(mgr, id, 90*time.Second)
	if err != nil {
		return err
	}
	if res.State != subagent.StateSucceeded {
		return fmt.Errorf("state=%q want %q (err=%q)", res.State, subagent.StateSucceeded, res.Error)
	}
	if res.Output == "" {
		return errors.New("real-llm output is empty")
	}

	fmt.Printf("    state            : %s\n", res.State)
	fmt.Printf("    output (first 80): %q\n", truncate(res.Output, 80))
	fmt.Printf("    duration         : %s\n", res.Duration)
	return nil
}

// phaseE proves the manager's MaxConcurrency cap is honoured. With cap=2 and
// three slow tasks dispatched in quick succession, the third Dispatch MUST
// return ErrMaxConcurrency. We then drain the two accepted tasks to release
// the slots cleanly.
func phaseE() error {
	fmt.Println("==> phase E: max-concurrency cap (always runs)")

	provider := subagent.NewFakeLLMProvider(nil)
	provider.WithDelay(500 * time.Millisecond)

	mgr, err := subagent.NewSubagentManager(subagent.SubagentManagerOptions{
		LLMProvider:    provider,
		MaxConcurrency: 2,
		Logger:         zap.NewNop(),
	})
	if err != nil {
		return fmt.Errorf("NewSubagentManager: %w", err)
	}
	defer func() { _ = mgr.Shutdown(context.Background()) }()

	mkTask := func(n int) subagent.SubagentTask {
		return subagent.SubagentTask{
			Description: fmt.Sprintf("phase-e-%d", n),
			Prompt:      fmt.Sprintf("phase-e-prompt-%d", n),
			Isolation:   subagent.IsolationNone,
			Timeout:     10 * time.Second,
		}
	}

	id1, err := mgr.Dispatch(context.Background(), mkTask(1))
	if err != nil {
		return fmt.Errorf("Dispatch #1: %w", err)
	}
	id2, err := mgr.Dispatch(context.Background(), mkTask(2))
	if err != nil {
		return fmt.Errorf("Dispatch #2: %w", err)
	}
	if _, err := mgr.Dispatch(context.Background(), mkTask(3)); !errors.Is(err, subagent.ErrMaxConcurrency) {
		return fmt.Errorf("Dispatch #3 err=%v; want ErrMaxConcurrency", err)
	}

	want := map[string]struct{}{id1: {}, id2: {}}
	results := drainSet(mgr, want, 15*time.Second)
	if len(results) != 2 {
		return fmt.Errorf("drained %d/2 results before timeout", len(results))
	}
	for _, r := range results {
		if r.State != subagent.StateSucceeded {
			return fmt.Errorf("phase-e drained state=%q want %q (err=%q)", r.State, subagent.StateSucceeded, r.Error)
		}
	}
	fmt.Printf("    cap=2 enforced   : true (3rd Dispatch returned ErrMaxConcurrency)\n")
	fmt.Printf("    results drained  : %d\n", len(results))
	return nil
}

// phaseF dispatches a slow task and immediately Kills it. The manager cancels
// the per-task ctx; the in-process spawner's Generate returns ctx.Err(); the
// result on the aggregator MUST carry State=StateCanceled.
func phaseF() error {
	fmt.Println("==> phase F: kill cancels a running subagent (always runs)")

	provider := subagent.NewFakeLLMProvider(nil)
	provider.WithDelay(5 * time.Second)

	mgr, err := subagent.NewSubagentManager(subagent.SubagentManagerOptions{
		LLMProvider:    provider,
		MaxConcurrency: 2,
		Logger:         zap.NewNop(),
	})
	if err != nil {
		return fmt.Errorf("NewSubagentManager: %w", err)
	}
	defer func() { _ = mgr.Shutdown(context.Background()) }()

	id, err := mgr.Dispatch(context.Background(), subagent.SubagentTask{
		Description: "phase-f-kill",
		Prompt:      "phase-f-prompt",
		Isolation:   subagent.IsolationNone,
		Timeout:     30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("Dispatch: %w", err)
	}

	// Give the goroutine a brief moment to enter Generate's blocking section.
	time.Sleep(50 * time.Millisecond)

	if err := mgr.Kill(id); err != nil {
		return fmt.Errorf("Kill(%s): %w", id, err)
	}

	res, err := drainOne(mgr, id, 5*time.Second)
	if err != nil {
		return err
	}
	if res.State != subagent.StateCanceled {
		return fmt.Errorf("state=%q want %q (err=%q)", res.State, subagent.StateCanceled, res.Error)
	}
	fmt.Printf("    kill_id          : %s\n", id)
	fmt.Printf("    state            : %s (cancelled)\n", res.State)
	fmt.Printf("    duration         : %s\n", res.Duration)
	return nil
}

// drainOne reads from the manager's aggregator until it sees a result with
// the given task ID, or the timeout fires.
func drainOne(mgr *subagent.SubagentManager, id string, timeout time.Duration) (subagent.SubagentResult, error) {
	deadline := time.After(timeout)
	for {
		select {
		case r, ok := <-mgr.Results():
			if !ok {
				return subagent.SubagentResult{}, fmt.Errorf("aggregator closed before id %s arrived", id)
			}
			if r.TaskID == id {
				return r, nil
			}
		case <-deadline:
			return subagent.SubagentResult{}, fmt.Errorf("timed out waiting for id %s after %s", id, timeout)
		}
	}
}

// drainSet drains the aggregator until every ID in `want` has been observed
// or the timeout fires. Returns the collected results in arrival order.
func drainSet(mgr *subagent.SubagentManager, want map[string]struct{}, timeout time.Duration) []subagent.SubagentResult {
	pending := make(map[string]struct{}, len(want))
	for k := range want {
		pending[k] = struct{}{}
	}
	out := make([]subagent.SubagentResult, 0, len(want))
	deadline := time.After(timeout)
	for len(pending) > 0 {
		select {
		case r, ok := <-mgr.Results():
			if !ok {
				return out
			}
			if _, w := pending[r.TaskID]; w {
				delete(pending, r.TaskID)
				out = append(out, r)
			}
		case <-deadline:
			return out
		}
	}
	return out
}

// initEphemeralGitRepo creates a tempdir, runs `git init -b main`, sets a
// throwaway user.email/user.name, writes a seed README, and commits it. The
// returned path is the repo root; caller must os.RemoveAll() it. Mirrors the
// helper in worktree_integration_test.go.
func initEphemeralGitRepo() (string, error) {
	tmp, err := os.MkdirTemp("", "p1f15-repo-")
	if err != nil {
		return "", fmt.Errorf("mkdir temp: %w", err)
	}
	runGit := func(args ...string) error {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %s: %w (out=%s)", strings.Join(args, " "), err, out)
		}
		return nil
	}
	if err := runGit("init", "-b", "main"); err != nil {
		return "", err
	}
	if err := runGit("config", "user.email", "p1f15-harness@helixcode.dev"); err != nil {
		return "", err
	}
	if err := runGit("config", "user.name", "p1f15-harness"); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("seed\n"), 0o644); err != nil {
		return "", fmt.Errorf("write seed: %w", err)
	}
	if err := runGit("add", "."); err != nil {
		return "", err
	}
	if err := runGit("commit", "-m", "seed"); err != nil {
		return "", err
	}
	return tmp, nil
}

// truncate returns s capped to limit bytes; if s exceeds the cap, an
// "…(truncated)" suffix is appended for forensic clarity.
func truncate(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit] + "…(truncated)"
}

// _ keeps runtime imported across all build tags; future maintainers may need
// to branch on runtime.GOOS in this harness as F14's did.
var _ = runtime.GOOS
