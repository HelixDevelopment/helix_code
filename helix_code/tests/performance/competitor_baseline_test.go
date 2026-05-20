package performance

// competitor_baseline_test.go — integration coverage for the speed-programme
// P0-T03 competitor wall-clock baseline harness (CONST-050: every script ships
// a test that proves it actually runs end-to-end).
//
// This is an INTEGRATION check, not a unit test: it invokes the real
// scripts/testing/competitor_speed_baseline.sh against the real host and
// verifies it produces a stable, non-empty results table. No mocks — the
// script is exercised exactly as an operator would run it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRootFromTest walks up from this test file to the meta-repo root
// (identified by the presence of CONSTITUTION.md + the scripts/ dir).
func repoRootFromTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file path")
	}
	dir := filepath.Dir(thisFile)
	for i := 0; i < 12; i++ {
		if fileExists(filepath.Join(dir, "CONSTITUTION.md")) &&
			dirExists(filepath.Join(dir, "scripts", "testing")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate meta-repo root from test path")
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// TestCompetitorBaselineScript_RunsEndToEnd runs the real baseline harness and
// asserts it produces a stable table: at least one row per known competitor
// agent, every row either a real measurement or an explicit SKIP-OK.
func TestCompetitorBaselineScript_RunsEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("SKIP-OK: competitor-baseline integration skipped in -short mode (#P0-T03)")
	}
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: baseline harness is a POSIX sh script, not run on windows (#P0-T03)")
	}

	root := repoRootFromTest(t)
	script := filepath.Join(root, "scripts", "testing", "competitor_speed_baseline.sh")
	if !fileExists(script) {
		t.Fatalf("baseline script missing: %s", script)
	}

	// Write the capture file into a temp dir so the test never clobbers the
	// committed baseline artefact.
	outFile := filepath.Join(t.TempDir(), "competitor-baseline-test.md")

	// 2 runs keeps the test fast while still exercising the multi-run path.
	cmd := exec.Command("bash", script, "--runs", "2", "--out", outFile, "--quiet")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("baseline script failed: %v\noutput:\n%s", err, string(out))
	}

	table := string(out)
	if strings.TrimSpace(table) == "" {
		t.Fatal("baseline script produced an empty results table — bluff (CONST-035)")
	}

	// The script must have considered every catalogue agent.
	catalogue := []string{"Claude Code", "Gemini CLI", "Aider", "Cline", "Crush"}
	for _, agent := range catalogue {
		if !strings.Contains(table, agent) {
			t.Errorf("results table missing catalogue agent %q\ntable:\n%s", agent, table)
		}
	}

	// Every catalogue row must be either a real "ms" measurement or SKIP-OK —
	// never an empty / fabricated cell.
	measured := 0
	skipped := 0
	for _, line := range strings.Split(table, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || !strings.Contains(line, "S1 cold-start") {
			continue
		}
		switch {
		case strings.Contains(line, "SKIP-OK"):
			skipped++
		case strings.Contains(line, " ms "):
			measured++
		default:
			t.Errorf("row is neither a measurement nor SKIP-OK (possible bluff): %q", line)
		}
	}
	if measured+skipped == 0 {
		t.Fatal("no S1 cold-start rows found in the results table")
	}
	t.Logf("competitor baseline: %d agent(s) measured, %d SKIP-OK", measured, skipped)

	// The capture file must exist and contain the anti-bluff raw-evidence block.
	captured, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("capture file not written: %v", err)
	}
	capStr := string(captured)
	for _, marker := range []string{
		"Competitor Wall-Clock Baseline",
		"Raw `time` evidence",
		"Measured wall-clock",
	} {
		if !strings.Contains(capStr, marker) {
			t.Errorf("capture file missing required section %q", marker)
		}
	}
}

// TestCompetitorBaselineScript_SelfVerifiesMeasuredAgents asserts that any
// agent the script reports as MEASURED actually executed (non-empty output).
// This guards the CONST-050 "script self-verifies each measured agent" rule.
func TestCompetitorBaselineScript_SelfVerifiesMeasuredAgents(t *testing.T) {
	if testing.Short() {
		t.Skip("SKIP-OK: competitor-baseline self-verify skipped in -short mode (#P0-T03)")
	}
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: baseline harness is a POSIX sh script, not run on windows (#P0-T03)")
	}

	root := repoRootFromTest(t)
	script := filepath.Join(root, "scripts", "testing", "competitor_speed_baseline.sh")
	outFile := filepath.Join(t.TempDir(), "competitor-baseline-selfverify.md")

	cmd := exec.Command("bash", script, "--runs", "1", "--out", outFile, "--quiet")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("baseline script failed: %v\noutput:\n%s", err, string(out))
	}

	captured, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("capture file not written: %v", err)
	}
	capStr := string(captured)

	// For every measured agent the raw evidence block records its resolved
	// binary path and per-run wall figures; a measured agent that produced
	// nothing would carry the explicit "WARNING empty output" marker.
	if strings.Contains(capStr, "WARNING empty output") {
		t.Errorf("a measured agent produced no output — bluff risk (CONST-035):\n%s", capStr)
	}

	// At least the raw-evidence section header must be present.
	if !strings.Contains(capStr, "Raw `time` evidence") {
		t.Fatal("capture file missing raw evidence section")
	}
}
