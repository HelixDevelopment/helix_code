package main

import (
	"bytes"
	"encoding/json"
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

// scratchTreeWithViolation creates a scratch dir with a single Go file
// containing one known CONST-046 violation. Returns the scratch root
// (suitable for --roots) and the literal text used (for hash recompute
// in baseline-priming helpers).
func scratchTreeWithViolation(t *testing.T) (root string, literal string) {
	t.Helper()
	tmp := t.TempDir()
	scratch := filepath.Join(tmp, "scratch_tree", "pkg")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		t.Fatalf("mkdir scratch: %v", err)
	}
	literal = "Which file has the bug today, dear user?"
	src := []byte("package pkg\n\nvar Question = \"" + literal + "\"\n")
	if err := os.WriteFile(filepath.Join(scratch, "v.go"), src, 0o644); err != nil {
		t.Fatalf("write violation: %v", err)
	}
	return filepath.Join(tmp, "scratch_tree"), literal
}

// runInProcess invokes the testable run() entry point and returns
// (exitCode, stdout, stderr) — no subprocess required.
func runInProcess(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestBaseline_FailOnNew_NewViolationFails(t *testing.T) {
	root, _ := scratchTreeWithViolation(t)
	baselinePath := filepath.Join(t.TempDir(), "empty_baseline.json")
	// Empty baseline (no violations recorded).
	empty := Baseline{SchemaVersion: 1, GeneratedAt: "2026-05-19T00:00:00Z", ToolVersion: toolVersion, Violations: nil}
	data, _ := json.MarshalIndent(&empty, "", "  ")
	if err := os.WriteFile(baselinePath, data, 0o644); err != nil {
		t.Fatalf("seed empty baseline: %v", err)
	}

	code, stdout, stderr := runInProcess(
		"--roots", root,
		"--baseline", baselinePath,
		"--fail-on-new",
	)
	if code != 1 {
		t.Fatalf("expected exit 1 for new violation, got %d; stdout=%s; stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "NEW: 1") {
		t.Fatalf("expected 'NEW: 1' in stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Which file has the bug") {
		t.Fatalf("expected violation excerpt in stdout, got: %s", stdout)
	}
}

func TestBaseline_FailOnNew_PreExistingPasses(t *testing.T) {
	root, _ := scratchTreeWithViolation(t)
	baselinePath := filepath.Join(t.TempDir(), "seeded_baseline.json")

	// Seed the baseline by running --update-baseline first.
	code, stdout, stderr := runInProcess(
		"--roots", root,
		"--baseline", baselinePath,
		"--update-baseline",
	)
	if code != 0 {
		t.Fatalf("update-baseline expected exit 0, got %d; stdout=%s; stderr=%s", code, stdout, stderr)
	}

	// Re-run with --fail-on-new — the violation is now in baseline.
	code, stdout, stderr = runInProcess(
		"--roots", root,
		"--baseline", baselinePath,
		"--fail-on-new",
	)
	if code != 0 {
		t.Fatalf("expected exit 0 for pre-existing violation, got %d; stdout=%s; stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "NEW: 0") {
		t.Fatalf("expected 'NEW: 0' in stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, "PRE-EXISTING: 1") {
		t.Fatalf("expected 'PRE-EXISTING: 1' in stdout, got: %s", stdout)
	}
}

func TestBaseline_UpdateBaseline_WritesFile(t *testing.T) {
	root, _ := scratchTreeWithViolation(t)
	baselinePath := filepath.Join(t.TempDir(), "fresh.json")

	code, stdout, stderr := runInProcess(
		"--roots", root,
		"--baseline", baselinePath,
		"--update-baseline",
	)
	if code != 0 {
		t.Fatalf("expected exit 0 from update-baseline, got %d; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "baseline updated") {
		t.Fatalf("expected 'baseline updated' message, got: %s", stdout)
	}

	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("read written baseline: %v", err)
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("baseline is not valid JSON: %v; raw=%s", err, string(data))
	}
	if b.SchemaVersion != 1 {
		t.Fatalf("expected schema_version=1, got %d", b.SchemaVersion)
	}
	if b.ToolVersion != toolVersion {
		t.Fatalf("expected tool_version=%q, got %q", toolVersion, b.ToolVersion)
	}
	if len(b.Violations) != 1 {
		t.Fatalf("expected 1 violation in baseline, got %d", len(b.Violations))
	}
	if b.Violations[0].LiteralHash == "" {
		t.Fatalf("expected literal_hash populated, got empty")
	}
	if len(b.Violations[0].LiteralHash) != 16 {
		t.Fatalf("expected 16-hex-char hash, got %q", b.Violations[0].LiteralHash)
	}
}

func TestBaseline_MissingFile_TreatsAsEmpty(t *testing.T) {
	root, _ := scratchTreeWithViolation(t)
	bogus := filepath.Join(t.TempDir(), "does_not_exist.json")

	code, stdout, stderr := runInProcess(
		"--roots", root,
		"--baseline", bogus,
		"--fail-on-new",
	)
	if code != 1 {
		t.Fatalf("expected exit 1 (missing baseline ⇒ all NEW), got %d; stdout=%s; stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "WARNING: baseline file not found") {
		t.Fatalf("expected warning about missing baseline in stderr, got: %s", stderr)
	}
	if !strings.Contains(stdout, "NEW: 1") {
		t.Fatalf("expected 'NEW: 1' in stdout, got: %s", stdout)
	}
}

// TestBaseline_RepoRootRelative_PortableAcrossDifferentAbsolutePaths
// is the regression guard for the CONST-046 gate portability defect
// (§11.4.108/§11.4.177): a baseline generated under one absolute
// checkout path MUST still classify the SAME violation as
// PRE-EXISTING when re-scanned under a DIFFERENT absolute checkout
// path (simulating a different host/clone/developer machine), as long
// as --repo-root is supplied for both runs and the repo-relative path
// is identical. Without the fix (i.e. running without --repo-root),
// this exact scenario is what produced 19098/19098 false "NEW"
// findings on a checkout other than the one the baseline was
// generated on.
func TestBaseline_RepoRootRelative_PortableAcrossDifferentAbsolutePaths(t *testing.T) {
	literal := "Which file has the bug today, dear user?"
	src := []byte("package pkg\n\nvar Question = \"" + literal + "\"\n")

	// "Checkout A" — where the baseline is generated.
	tmpA := t.TempDir()
	rootA := filepath.Join(tmpA, "repo_a")
	pkgDirA := filepath.Join(rootA, "pkg")
	if err := os.MkdirAll(pkgDirA, 0o755); err != nil {
		t.Fatalf("mkdir checkout A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDirA, "v.go"), src, 0o644); err != nil {
		t.Fatalf("write checkout A file: %v", err)
	}
	baselinePath := filepath.Join(tmpA, "baseline.json")

	code, stdout, stderr := runInProcess(
		"--roots", pkgDirA,
		"--repo-root", rootA,
		"--baseline", baselinePath,
		"--update-baseline",
	)
	if code != 0 {
		t.Fatalf("seed baseline on checkout A failed: %d; stdout=%s; stderr=%s", code, stdout, stderr)
	}

	// Assert the baseline stored a REPO-ROOT-RELATIVE path, not an
	// absolute one — this is the load-bearing assertion that the fix
	// actually changed on-disk identity, not just in-memory behavior.
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("baseline not valid JSON: %v", err)
	}
	if len(b.Violations) != 1 {
		t.Fatalf("expected 1 baseline violation, got %d: %+v", len(b.Violations), b.Violations)
	}
	wantRelPath := "pkg/v.go"
	if b.Violations[0].Path != wantRelPath {
		t.Fatalf("expected baseline Path to be repo-root-relative %q, got %q (absolute path leaked into baseline identity — the exact portability defect this test guards against)", wantRelPath, b.Violations[0].Path)
	}

	// "Checkout B" — a DIFFERENT absolute directory (simulating a
	// different host / clone / developer machine) containing the
	// IDENTICAL relative-path file with the IDENTICAL literal.
	tmpB := t.TempDir()
	rootB := filepath.Join(tmpB, "repo_b_completely_different_absolute_prefix")
	pkgDirB := filepath.Join(rootB, "pkg")
	if err := os.MkdirAll(pkgDirB, 0o755); err != nil {
		t.Fatalf("mkdir checkout B: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDirB, "v.go"), src, 0o644); err != nil {
		t.Fatalf("write checkout B file: %v", err)
	}

	code, stdout, stderr = runInProcess(
		"--roots", pkgDirB,
		"--repo-root", rootB,
		"--baseline", baselinePath,
		"--fail-on-new",
	)
	if code != 0 {
		t.Fatalf("expected exit 0 (same relative path+literal ⇒ PRE-EXISTING even under a different absolute checkout root), got %d; stdout=%s; stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "NEW: 0") {
		t.Fatalf("expected 'NEW: 0' when re-scanned under a different absolute checkout path, got: %s", stdout)
	}
	if !strings.Contains(stdout, "PRE-EXISTING: 1") {
		t.Fatalf("expected 'PRE-EXISTING: 1', got: %s", stdout)
	}
}

// TestBaseline_WithoutRepoRoot_AbsolutePathBehaviorUnchanged proves the
// fix is backward compatible: callers that omit --repo-root keep the
// legacy absolute-path identity behavior byte-for-byte (all prior
// tests in this file rely on this and must keep passing unmodified).
func TestBaseline_WithoutRepoRoot_AbsolutePathBehaviorUnchanged(t *testing.T) {
	root, _ := scratchTreeWithViolation(t)
	baselinePath := filepath.Join(t.TempDir(), "legacy_absolute.json")

	code, _, stderr := runInProcess(
		"--roots", root,
		"--baseline", baselinePath,
		"--update-baseline",
	)
	if code != 0 {
		t.Fatalf("update-baseline (no --repo-root) failed: %d; stderr=%s", code, stderr)
	}

	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("baseline not valid JSON: %v", err)
	}
	if len(b.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(b.Violations))
	}
	if !filepath.IsAbs(b.Violations[0].Path) {
		t.Fatalf("expected legacy absolute Path when --repo-root omitted, got relative-looking path %q", b.Violations[0].Path)
	}
}

func TestBaseline_HashStability_LineShiftIgnored(t *testing.T) {
	root, literal := scratchTreeWithViolation(t)
	baselinePath := filepath.Join(t.TempDir(), "seed.json")

	// Seed baseline from initial tree.
	code, _, stderr := runInProcess(
		"--roots", root,
		"--baseline", baselinePath,
		"--update-baseline",
	)
	if code != 0 {
		t.Fatalf("seed baseline failed: %d / %s", code, stderr)
	}

	// Re-write the violating file with the SAME literal but shifted
	// down by inserting a blank-line comment block above it. The line
	// number changes; literal hash must stay stable.
	violationFile := filepath.Join(root, "pkg", "v.go")
	shifted := []byte("package pkg\n\n// Unrelated comment block inserted to push the\n// violation down a few lines and prove the baseline\n// matches by literal-hash, not by line number.\n//\n//\n\nvar Question = \"" + literal + "\"\n")
	if err := os.WriteFile(violationFile, shifted, 0o644); err != nil {
		t.Fatalf("rewrite file with shifted line: %v", err)
	}

	code, stdout, stderr := runInProcess(
		"--roots", root,
		"--baseline", baselinePath,
		"--fail-on-new",
	)
	if code != 0 {
		t.Fatalf("expected exit 0 (line shift, same literal → PRE-EXISTING), got %d; stdout=%s; stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "NEW: 0") {
		t.Fatalf("expected 'NEW: 0' after line shift, got: %s", stdout)
	}
	if !strings.Contains(stdout, "PRE-EXISTING: 1") {
		t.Fatalf("expected 'PRE-EXISTING: 1' after line shift, got: %s", stdout)
	}
}
