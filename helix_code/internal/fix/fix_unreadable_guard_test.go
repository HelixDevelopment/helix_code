package fix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGuard_UnreadableSecretFile_FailsSafe is the standing regression guard
// (§11.4.135) for the §11.4 / Article XI §11.9 anti-bluff defect where the
// security-fix detectors silently skipped (`continue`) any .go file that
// os.ReadFile could not read, then reported the whole project clean (true) —
// a PASS-bluff: a real hardcoded secret inside an unreadable file was missed
// and certified clean.
//
// §11.4.115 polarity switch:
//   - RED_MODE=1 inlines the PRE-FIX behaviour (skip-on-read-error) on the
//     SAME planted unreadable secret file and asserts the WRONG outcome
//     (reported clean == true). This reproduces the defect and PASSES,
//     proving the guard is real and the counterexample genuinely triggers it.
//   - RED_MODE=0 (DEFAULT, no env) drives the REAL fixed attemptFix and
//     asserts the CORRECT outcome: an unreadable target file is NOT certified
//     clean (false), because a scanner that cannot read a file cannot certify
//     it.
//
// Counterexample: a .go file containing `var password = "hunter2"` made
// mode 0000. Pre-fix: reported clean (secret missed). Post-fix: fail-safe false.
func TestGuard_UnreadableSecretFile_FailsSafe(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("SKIP-OK: running as root — file mode 0000 does not block reads; the unreadable-file path is not reachable as root")
	}

	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.go")
	require.NoError(t, os.WriteFile(secretFile,
		[]byte("package p\nvar password = \"hunter2\"\n"), 0644))
	// Make the planted secret file unreadable to trigger a ReadFile error.
	require.NoError(t, os.Chmod(secretFile, 0000))
	t.Cleanup(func() { _ = os.Chmod(secretFile, 0644) })

	if os.Getenv("RED_MODE") == "1" {
		// PRE-FIX reproduction: replicate the old skip-on-read-error scan
		// inline over the same unreadable-secret fixture and assert it
		// WRONGLY reports the project clean (true).
		reportedClean := redModeScanSkipOnError(t, tmpDir)
		require.True(t, reportedClean,
			"RED_MODE: pre-fix skip-on-read-error scan must (wrongly) report clean, reproducing the PASS-bluff")
		return
	}

	// POST-FIX (default): the real, fixed code must fail safe — an unreadable
	// target file is NOT certified clean.
	got := attemptFix(tmpDir, "hardcoded secret detected")
	require.False(t, got,
		"fixed code must NOT certify a project clean when a target .go file could not be read (fail-safe)")
}

// redModeScanSkipOnError replicates the PRE-FIX detector loop body: silently
// skip any unreadable file (continue) and report clean (true) when no readable
// file matched a pattern. Used only by RED_MODE=1 to reproduce the historical
// defect on the broken-artifact fixture. It is a faithful copy of the old
// fixHardcodedSecret scanning logic with the buggy `continue`.
func redModeScanSkipOnError(t *testing.T, projectPath string) bool {
	t.Helper()
	files, err := findGoFiles(projectPath)
	require.NoError(t, err)
	for _, file := range files {
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			// PRE-FIX BUG: silently skip unreadable files.
			continue
		}
		cs := string(content)
		if containsAny(cs, "password", "secret", "api_key", "token") {
			if contains(cs, "= \"") && !contains(cs, "os.Getenv") {
				return false // detected
			}
		}
	}
	return true // "no patterns found" — the PASS-bluff when files were skipped
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || indexOf(haystack, needle) >= 0
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if contains(haystack, n) {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
