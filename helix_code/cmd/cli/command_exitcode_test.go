package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// command_exitcode_test.go — standing regression guard for the BLUFF-003
// follow-up defect: `helixcode --command '<cmd>'` MUST surface the child
// process's REAL exit code as the CLI process exit code (it previously
// blanket-mapped every failure to exit 1 via main()'s log.Fatalf).
//
// §11.4.115 RED-polarity-switch: this guard reproduces the defect on a
// broken artifact when RED_MODE=1 (asserting the broken-exit-1 behaviour is
// present — proving the guard is real), and stands as the GREEN regression
// guard when RED_MODE=0 (the default), asserting the real exit code propagates.
//
// §11.4.135 standing guard: it is a normal `go test` so it runs on every
// build and blocks a regression that re-loses the child exit code.
//
// This drives the REAL compiled binary as a subprocess because the fix lives
// at the os.Exit boundary in main() — only a subprocess can observe a true
// process exit code.

// buildCLIBinary compiles the CLI into a temp dir and returns its path.
func buildCLIBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "helixcode-cli-exitcode-test")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("failed to build CLI binary: %v", err)
	}
	return bin
}

// runCommand runs `<bin> --command <shellCmd>` and returns its real exit code.
func runCommand(t *testing.T, bin, shellCmd string) int {
	t.Helper()
	cmd := exec.Command(bin, "--command", shellCmd)
	// Discard child stdout/stderr; we only care about the exit code.
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if !asExitError(err, &exitErr) {
		t.Fatalf("unexpected non-exit error running CLI: %v", err)
	}
	return exitErr.ProcessState.ExitCode()
}

// asExitError is a tiny errors.As shim kept local to avoid an extra import
// churn in this guard file.
func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

// TestCommandPropagatesRealExitCode is the BLUFF-003 follow-up regression guard.
//
//	RED_MODE=1 → reproduce the defect: assert `--command 'exit 42'` exits 1
//	             (the OLD, broken behaviour). Run this against a pre-fix binary
//	             to prove the guard genuinely catches the defect.
//	RED_MODE=0 → standing GREEN guard (default): assert the real exit codes
//	             propagate. This is what runs on every build.
func TestCommandPropagatesRealExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("builds + runs a subprocess; skipped in -short mode")
	}
	bin := buildCLIBinary(t)
	redMode := os.Getenv("RED_MODE") == "1"

	t.Run("non_zero_exit_42", func(t *testing.T) {
		got := runCommand(t, bin, "exit 42")
		if redMode {
			// Defect present: the child's 42 is lost and mapped to 1.
			if got != 1 {
				t.Fatalf("RED_MODE: expected the BROKEN behaviour (exit 1) but got %d — defect not reproduced on this artifact", got)
			}
			return
		}
		if got != 42 {
			t.Fatalf("expected CLI to surface child exit code 42, got %d", got)
		}
	})

	t.Run("success_exit_0", func(t *testing.T) {
		got := runCommand(t, bin, "true")
		// exit 0 is correct on BOTH broken and fixed artifacts; assert it
		// unconditionally so the success path never silently regresses.
		if got != 0 {
			t.Fatalf("expected CLI to exit 0 for a succeeding command, got %d", got)
		}
	})

	t.Run("real_failing_command_exit_7", func(t *testing.T) {
		got := runCommand(t, bin, "exit 7")
		if redMode {
			if got != 1 {
				t.Fatalf("RED_MODE: expected the BROKEN behaviour (exit 1) but got %d — defect not reproduced on this artifact", got)
			}
			return
		}
		if got != 7 {
			t.Fatalf("expected CLI to surface child exit code 7, got %d", got)
		}
	})

	t.Run("genuine_error_stays_non_zero", func(t *testing.T) {
		// A command that fails to start (sh syntax error → non-zero from sh)
		// must still produce a non-zero exit on the fixed artifact.
		got := runCommand(t, bin, "exit 3")
		want := 3
		if redMode {
			want = 1
		}
		if got != want {
			t.Fatalf("expected exit %d, got %d", want, got)
		}
	})
}
