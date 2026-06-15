package builtin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReportBug_SessionIDPathTraversal_Guard is the STANDING regression guard
// (§11.4.135) for the path-traversal defect in collectRecentLogs: a malicious
// CommandContext.SessionID interpolated raw into
// fmt.Sprintf("session_%s.log", sessionID) + filepath.Join could escape the
// intended "logs/" base and read an arbitrary ".log" file anywhere on the host
// (information disclosure, severity HIGH).
//
// §11.4.115 polarity switch + RED-on-the-broken-artifact discipline:
//
//   - RED_MODE=1 reproduces the defect on a faithful inline stand-in of the
//     PRE-FIX path construction (raw interpolation, no sanitisation) that
//     mirrors EXACTLY how collectRecentLogs builds its relative "logs/" per-
//     session path, and asserts the traversal SUCCEEDS — the off-base secret
//     planted at <root>/secret/x.log is read THROUGH the "logs/" base. This
//     proves the guard is real: it fails on the broken algorithm.
//
//   - RED_MODE=0 (default) is the adversarial GREEN guard. It (a) first proves,
//     via the same inline unguarded construction, that the off-base secret IS
//     readable on disk (so the defect is genuinely live without the guard —
//     collectRecentLogs cannot "pass by accident" because the file is missing);
//     then (b) drives the REAL sanitizeSessionID + collectRecentLogs with that
//     same malicious id and asserts the secret is NOT in the output AND that
//     sanitizeSessionID(maliciousID) == "" (the blocking mechanism). Because
//     the off-base file IS readable, reverting sanitizeSessionID to return the
//     raw id makes collectRecentLogs build+read the now-resolvable off-base
//     path and the secret leaks into the output → this guard FAILS. That is the
//     §1.1 paired-mutation property: the test catches a reverted sanitizer.
//
// Path arithmetic (verified empirically): with CWD == <root>, the real
// collectRecentLogs relative location builds
//
//	filepath.Join("logs", "session_../../../secret/x.log")
//	  -> "logs/session_../../../secret/x.log"
//	  -> Clean -> "secret/x.log"  (escapes the "logs/" base entirely)
//	  -> <root>/secret/x.log      (the planted secret)
//
// so the appended ".log" suffix lands exactly on the planted file. Everything
// stays inside t.TempDir(); no real host secret is ever read.
//
// Run RED:   RED_MODE=1 go test -run TestReportBug_SessionIDPathTraversal_Guard ./internal/commands/builtin/
// Run GREEN: go test -run TestReportBug_SessionIDPathTraversal_Guard ./internal/commands/builtin/
func TestReportBug_SessionIDPathTraversal_Guard(t *testing.T) {
	red := os.Getenv("RED_MODE") == "1"

	// Build a realistic on-disk layout: a "logs" base and a sibling "secret"
	// directory holding a .log file the attacker should NOT be able to read
	// through the logs base.
	root := t.TempDir()
	base := filepath.Join(root, "logs")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir base: %v", err)
	}
	secretDir := filepath.Join(root, "secret")
	if err := os.MkdirAll(secretDir, 0o755); err != nil {
		t.Fatalf("mkdir secret: %v", err)
	}
	const secret = "SECRET-CREDS-DO-NOT-LEAK"
	secretFile := filepath.Join(secretDir, "x.log")
	if err := os.WriteFile(secretFile, []byte(secret), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	// Run from <root> so collectRecentLogs's relative "logs/" location is rooted
	// here and its per-session path resolution is deterministic. Restore the CWD
	// afterwards so we never perturb a sibling test (§11.4.119).
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	// RELATIVE-traversal payload (no leading slash). With the relative "logs"
	// base, "logs/session_../../../secret/x.log" cleans to "secret/x.log" — the
	// off-base planted file under <root>. A leading-slash id would instead be
	// treated as absolute and miss the temp-dir secret entirely (the bug the
	// security review caught), so we deliberately use the relative form that
	// reproduces the REAL relative-traversal leak.
	maliciousID := "../../../secret/x"

	// faithfulUnguardedRead reproduces, byte-for-byte, how the PRE-FIX
	// collectRecentLogs built and read its relative per-session log path when
	// the session id was interpolated raw (no sanitisation). It returns the file
	// content it managed to read (or "" + err on miss).
	faithfulUnguardedRead := func(id string) (string, error) {
		logPath := filepath.Join("logs", fmt.Sprintf("session_%s.log", id))
		data, err := os.ReadFile(logPath)
		return string(data), err
	}

	if red {
		// PRE-FIX stand-in: raw interpolation, no sanitisation. The traversal
		// MUST succeed — reading the off-base secret through the "logs/" base.
		got, err := faithfulUnguardedRead(maliciousID)
		if err != nil {
			t.Fatalf("RED_MODE=1 expected the relative traversal to SUCCEED on the "+
				"unguarded algorithm, but the read failed: id=%q err=%v", maliciousID, err)
		}
		if got != secret {
			t.Fatalf("RED_MODE=1 expected to read the off-base secret %q, got %q", secret, got)
		}
		t.Logf("RED_MODE=1 reproduced the defect: unguarded relative path "+
			"logs/session_%s.log read off-base secret at %s", maliciousID, secretFile)
		return
	}

	// GREEN, step (a): prove the defect is genuinely LIVE on this disk layout —
	// the off-base secret is reachable via the unguarded construction. If this
	// did not hold, the guard below could "pass" only because the file was
	// missing, which would be a blind test.
	if got, err := faithfulUnguardedRead(maliciousID); err != nil || got != secret {
		t.Fatalf("precondition: unguarded construction must read the off-base secret "+
			"(else the guard is blind); got=%q err=%v", got, err)
	}

	// GREEN, step (b): the real fix must reject the malicious id outright — this
	// is the exact mechanism a regression would revert.
	if got := sanitizeSessionID(maliciousID); got != "" {
		t.Fatalf("sanitizeSessionID(%q) = %q, want \"\" (malicious id must be rejected). "+
			"A reverted sanitizer that returns the raw id is what this guard catches.",
			maliciousID, got)
	}

	// Drive the REAL collectRecentLogs with the malicious id. With the off-base
	// secret readable (proven in step (a)), the ONLY thing preventing a leak is
	// sanitizeSessionID rejecting the id so no per-session path is built. Revert
	// the sanitizer and collectRecentLogs builds logs/session_<id>.log, reads
	// <root>/secret/x.log, and the secret appears in `out` → this assertion
	// FAILS. That is the paired-mutation guarantee.
	out := collectRecentLogs(maliciousID, 50)
	if strings.Contains(out, secret) {
		t.Fatalf("collectRecentLogs leaked off-base secret content for malicious "+
			"session id %q (sanitizer not blocking the relative traversal)", maliciousID)
	}

	// Belt-and-braces: a battery of traversal payloads — including the new
	// relative form — are all rejected to "".
	for _, bad := range []string{
		"../../../secret/x", // the relative traversal exercised above
		"/../../secret/x",
		"x/../../secret/x",
		"../etc/hosts",
		"a/b",
		"..",
		".",
		"foo/bar/baz",
		string([]byte{'a', 0, 'b'}),
	} {
		if got := sanitizeSessionID(bad); got != "" {
			t.Fatalf("sanitizeSessionID(%q) = %q, want \"\" (must reject path-traversal payload)", bad, got)
		}
	}

	// And confirm legitimate flat session ids are NOT rejected (no over-blocking).
	for _, ok := range []string{
		"abc123",
		"sess-2026-06-15",
		"7f3a9b2c-1d4e",
		"user_42",
	} {
		if got := sanitizeSessionID(ok); got != ok {
			t.Fatalf("sanitizeSessionID(%q) = %q, want it unchanged (legit id must pass)", ok, got)
		}
	}
}
