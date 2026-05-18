package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scanSource is a test helper: parses Go source from a fixture file
// (.txt extension so the parent module does not try to compile it)
// and delegates to the production scanASTFile function. This means
// any mutation to the production detector logic (e.g. deleting the
// append) is observable in this test — proving the detector is real.
func scanSource(t *testing.T, path string, allow []AllowEntry) (Report, []Finding) {
	t.Helper()
	report := Report{}
	fset := token.NewFileSet()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	scanASTFile(fset, file, allow, &report)
	return report, report.Violations
}

func TestHeuristic_FlagsViolationFixture(t *testing.T) {
	path := filepath.Join("testdata", "fixture_violation.go.txt")
	_, findings := scanSource(t, path, nil)
	if len(findings) < 3 {
		t.Fatalf("expected >=3 findings in violation fixture, got %d: %#v", len(findings), findings)
	}
	want := []string{
		"Which file has the bug?",
		"What is the expected behavior?",
		"You are a helpful AI assistant",
	}
	for _, w := range want {
		found := false
		for _, f := range findings {
			if strings.Contains(f.Excerpt, w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find excerpt containing %q in violations, got %v", w, findings)
		}
	}
}

func TestHeuristic_PassesCleanFixture(t *testing.T) {
	path := filepath.Join("testdata", "fixture_clean.go.txt")
	_, findings := scanSource(t, path, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings in clean fixture, got %d: %#v", len(findings), findings)
	}
}

func TestHeuristic_ExemptsDeveloperFacing(t *testing.T) {
	path := filepath.Join("testdata", "fixture_developer_facing.go.txt")
	_, findings := scanSource(t, path, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings in developer-facing fixture (errors/logs/tags exempt), got %d: %#v", len(findings), findings)
	}
}

func TestAllowlist_SuppressesKnownEntries(t *testing.T) {
	path := filepath.Join("testdata", "fixture_allowlist.go.txt")
	// First: without allowlist, the literal must be flagged.
	r, findings := scanSource(t, path, nil)
	if len(findings) == 0 {
		t.Fatalf("baseline: expected the allowlist-fixture literal to be flagged, got 0 findings (heuristic too narrow); report=%+v", r)
	}
	// Apply allowlist that suppresses by path-substring + prefix.
	allow := []AllowEntry{{PathContains: "fixture_allowlist", LiteralPrefix: "Welcome to the synthetic"}}
	r2, findings2 := scanSource(t, path, allow)
	if len(findings2) != 0 {
		t.Fatalf("expected 0 findings after allowlist applied, got %d: %#v", len(findings2), findings2)
	}
	if r2.AllowlistHits == 0 {
		t.Fatalf("expected AllowlistHits > 0 after suppression, got 0")
	}
}

func TestExitCode_AlwaysZeroInSoftWarn(t *testing.T) {
	// Build the binary in a temp dir, run it against testdata, expect
	// exit 0 with non-empty violation list reported.
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "audit_const046")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build auditor: %v", err)
	}

	// Create a scratch tree that contains a known-violating .go file
	// at a path NOT named testdata (the walker prunes testdata).
	scratch := filepath.Join(tmp, "scratch_tree", "pkg")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		t.Fatalf("mkdir scratch: %v", err)
	}
	violation := []byte(`package pkg

var Question = "Which file has the bug today, dear user?"
`)
	if err := os.WriteFile(filepath.Join(scratch, "v.go"), violation, 0o644); err != nil {
		t.Fatalf("write violation: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, "--roots", filepath.Join(tmp, "scratch_tree"))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("auditor returned non-zero exit (expected soft-warn 0): %v; stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "violations") {
		t.Fatalf("expected 'violations' line in stdout, got: %s", out)
	}
	if !strings.Contains(out, "Which file has the bug") {
		t.Fatalf("expected violation excerpt in stdout, got: %s", out)
	}
}
