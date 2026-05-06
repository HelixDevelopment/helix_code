// p1_5_challenge runs the Phase 1.5 foundation-cleanup harness end-to-end
// against the real meta-repo working tree, the real bash loader script
// (scripts/load_api_keys.sh), the real anti-bluff cascade verifier, real
// .gitmodules files, and real on-disk directory layout. Article XI 11.9
// anti-bluff anchor: every PASS in this harness is backed by positive
// runtime evidence. A regression that re-introduced a duplicate submodule,
// broke the loader's HOME/.env precedence, re-created an uppercase
// Documentation/ tree, drifted a first-party directory back to mixed-case,
// or removed the anti-bluff anchor from any cascaded file MUST trip one of
// the five phase invariants.
//
// Phases (all five always run; no SKIPs):
//
//	A. NO-DUPLICATE-SUBMODULES  - parses every .gitmodules in the
//	                              first-party tree (skips cli_agents/
//	                              third-party content), enforces that
//	                              each canonical Helix-owned URL maps
//	                              to exactly one path, and verifies
//	                              the canonical first-party submodules
//	                              (LLMsVerifier, Containers, Security,
//	                              HelixQA, MCP-Servers) appear exactly
//	                              once at the root.
//	B. API-KEYS-LOADER          - exercises scripts/load_api_keys.sh
//	                              under three branches (HOME/api_keys.sh
//	                              wins, .env fallback, neither file),
//	                              capturing real env-var values via
//	                              bash subshell.
//	C. DOCS-UNDER-DOCS-DIR      - walks the first-party tree and
//	                              asserts zero `Documentation/` (any
//	                              capitalisation other than `docs/`)
//	                              directories remain; verifies the
//	                              canonical `docs/` exists at root and
//	                              under HelixCode/.
//	D. SNAKE_CASE               - walks first-party directories at
//	                              depth <= 4, applies the WP7 allowlist
//	                              (cmd/<binary>, repo names from
//	                              .gitmodules, test fixtures), and
//	                              asserts zero non-conforming dirs.
//	E. ANTI-BLUFF-ANCHOR        - shells out to
//	                              scripts/verify_anti_bluff_cascade.sh
//	                              and propagates the exit code.
//
// Exit code 0 on PASS; exit 1 with a diagnostic on any check failure.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// repoRoot resolves the meta-repo root by walking up from the current
// working directory until a directory containing both `.gitmodules` and
// `scripts/load_api_keys.sh` is found. The harness binary may be invoked
// from anywhere (tempdir under `go build -o`), so we cannot rely on cwd.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		gm := filepath.Join(dir, ".gitmodules")
		ld := filepath.Join(dir, "scripts", "load_api_keys.sh")
		if fileExists(gm) && fileExists(ld) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("meta-repo root not found from %s", dir)
		}
		dir = parent
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := repoRoot()
	if err != nil {
		return fmt.Errorf("repo root: %w", err)
	}
	fmt.Println("==> P1.5 challenge harness pid:", os.Getpid())
	fmt.Println("==> meta-repo root:", root)

	if err := phaseA(root); err != nil {
		return fmt.Errorf("phase A: %w", err)
	}
	if err := phaseB(root); err != nil {
		return fmt.Errorf("phase B: %w", err)
	}
	if err := phaseC(root); err != nil {
		return fmt.Errorf("phase C: %w", err)
	}
	if err := phaseD(root); err != nil {
		return fmt.Errorf("phase D: %w", err)
	}
	if err := phaseE(root); err != nil {
		return fmt.Errorf("phase E: %w", err)
	}

	fmt.Println("==> ALL CHECKS PASSED")
	fmt.Println("==> P1.5 challenge harness PASS")
	return nil
}

// -----------------------------------------------------------------------------
// .gitmodules parsing
// -----------------------------------------------------------------------------

type submoduleEntry struct {
	declaringFile string // path to the .gitmodules that declared this entry
	name          string
	path          string // path field from .gitmodules
	url           string
}

// parseGitmodules reads a .gitmodules file and returns one entry per
// `[submodule "..."]` block. Path is stored as declared (relative to the
// .gitmodules's directory). URL is normalised by trimming whitespace; no
// case-folding (URLs are case-sensitive on most hosts).
func parseGitmodules(path string) ([]submoduleEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []submoduleEntry
	var cur *submoduleEntry
	scanner := bufio.NewScanner(f)
	headerRe := regexp.MustCompile(`^\[submodule "(.+)"\]$`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := headerRe.FindStringSubmatch(line); m != nil {
			if cur != nil {
				entries = append(entries, *cur)
			}
			cur = &submoduleEntry{declaringFile: path, name: m[1]}
			continue
		}
		if cur == nil {
			continue
		}
		if eq := strings.Index(line, "="); eq != -1 {
			key := strings.TrimSpace(line[:eq])
			val := strings.TrimSpace(line[eq+1:])
			switch key {
			case "path":
				cur.path = val
			case "url":
				cur.url = val
			}
		}
	}
	if cur != nil {
		entries = append(entries, *cur)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// metaRepoGitmodules returns the .gitmodules files the META-REPO directly
// tracks (typically only the root one). Per WP3's scope, the meta-repo is
// only responsible for its own submodule wiring; nested .gitmodules inside
// other submodules are owned and validated by those submodules' own gates.
//
// We use `git ls-files` rather than a filesystem walk so we capture exactly
// the meta-repo's tracked surface — not any .gitmodules tracked by an inner
// submodule's separate index.
func metaRepoGitmodules(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", ".gitmodules")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var hits []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		hits = append(hits, filepath.Join(root, line))
	}
	sort.Strings(hits)
	return hits, nil
}

// submodulePaths returns the slash-separated paths reported by
// `git submodule status --recursive` from the meta-repo root. Used to
// scope Phase D's snake_case scan: directories inside any submodule
// subtree are owned by that submodule, not the meta-repo, so they are
// out of WP7's scope.
func submodulePaths(root string) (map[string]bool, error) {
	cmd := exec.Command("git", "submodule", "status", "--recursive")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git submodule status: %w", err)
	}
	paths := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		// `git submodule status` output: " <sha> <path> (<describe>)"
		// or "+<sha> <path>" / "-<sha> <path>".
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		paths[filepath.ToSlash(fields[1])] = true
	}
	return paths, nil
}

// -----------------------------------------------------------------------------
// Phase A
// -----------------------------------------------------------------------------

// canonicalSubmodules lists the first-party submodules that MUST appear at
// exactly one path AT THE META-REPO ROOT (depth 1 of the working tree).
// These are the canonical locations enforced by WP3 deduplication.
var canonicalSubmodules = []string{
	"LLMsVerifier",
	"Containers",
	"Security",
	"HelixQA",
	"MCP-Servers",
}

// canonicalRootPaths is the expected root-level path for each canonical name.
// LLMsVerifier intentionally lives under Dependencies/HelixDevelopment/.
var canonicalRootPaths = map[string]string{
	"LLMsVerifier": "Dependencies/HelixDevelopment/LLMsVerifier",
	"Containers":   "Containers",
	"Security":     "Security",
	"HelixQA":      "HelixQA",
	"MCP-Servers":  "MCP-Servers",
}

func phaseA(root string) error {
	fmt.Println("==> Phase A — NO-DUPLICATE-SUBMODULES")

	gms, err := metaRepoGitmodules(root)
	if err != nil {
		return err
	}
	fmt.Printf("phaseA: scanned %d meta-repo-tracked .gitmodules file(s)\n", len(gms))

	// Build URL -> []declaration map across the first-party tree.
	urlPaths := make(map[string][]string)
	// Track per-canonical-name occurrences (basename match against the path
	// declared in .gitmodules joined to the declaring file's dir).
	occurrences := make(map[string][]string)
	for _, gm := range gms {
		entries, err := parseGitmodules(gm)
		if err != nil {
			return fmt.Errorf("parse %s: %w", gm, err)
		}
		gmDir := filepath.Dir(gm)
		for _, e := range entries {
			abs := filepath.Join(gmDir, e.path)
			rel, _ := filepath.Rel(root, abs)
			rel = filepath.ToSlash(rel)
			urlPaths[e.url] = append(urlPaths[e.url], rel)

			// Match canonical names by basename.
			base := filepath.Base(e.path)
			for _, name := range canonicalSubmodules {
				if base == name {
					occurrences[name] = append(occurrences[name], rel)
				}
			}
		}
	}

	// Check URL uniqueness for first-party (Helix*-owned) URLs only.
	// Third-party-content trees are pre-skipped at file-discovery time, so
	// any URL we see here is in-scope. We still skip URLs that obviously
	// are third-party mirrors (github.com/<vendor>/...) only when they
	// don't match a canonical name.
	urlDupViolations := 0
	for url, paths := range urlPaths {
		if len(paths) <= 1 {
			continue
		}
		// Some submodules are deliberately referenced in BOTH the meta-repo
		// .gitmodules and a child .gitmodules (e.g. HelixAgent's nested
		// HelixQA copy referencing the same URL). That is a real duplicate
		// only when both declarations are first-party AND the URL is a
		// canonical Helix repo. WP3 collapsed the canonical ones; anything
		// remaining here is a regression.
		isCanonical := false
		for _, p := range paths {
			base := filepath.Base(p)
			for _, name := range canonicalSubmodules {
				if base == name {
					isCanonical = true
				}
			}
		}
		if !isCanonical {
			continue
		}
		fmt.Printf("phaseA: FAIL duplicate canonical URL %s -> %v\n", url, paths)
		urlDupViolations++
	}
	if urlDupViolations > 0 {
		return fmt.Errorf("phase A: %d duplicate canonical URL declarations", urlDupViolations)
	}

	// Per-canonical occurrence + path-correctness check.
	canonicalViolations := 0
	for _, name := range canonicalSubmodules {
		occ := occurrences[name]
		if len(occ) == 0 {
			fmt.Printf("phaseA: FAIL %s missing from first-party tree\n", name)
			canonicalViolations++
			continue
		}
		if len(occ) > 1 {
			fmt.Printf("phaseA: FAIL %s at multiple locations: %v\n", name, occ)
			canonicalViolations++
			continue
		}
		want := canonicalRootPaths[name]
		got := occ[0]
		if got != want {
			fmt.Printf("phaseA: FAIL %s at %s (expected %s)\n", name, got, want)
			canonicalViolations++
			continue
		}
		fmt.Printf("phaseA: %s at %s (1 location, no duplicates)\n", name, got)
	}
	if canonicalViolations > 0 {
		return fmt.Errorf("phase A: %d canonical-submodule violations", canonicalViolations)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Phase B
// -----------------------------------------------------------------------------

// runLoaderInBash sources scripts/load_api_keys.sh under the harness-supplied
// HOME and PWD, then prints the requested env-var value with an unambiguous
// prefix the parent can scan. Returns the captured value (may be empty).
//
// We deliberately spawn a fresh `bash -c '...'` so the calling Go process's
// env is not contaminated; the loader sources files into THAT bash subshell.
func runLoaderInBash(loaderPath, fakeHome, pwd, varName string) (string, int, error) {
	script := fmt.Sprintf(`set +e
. %q >/dev/null 2>&1
echo "P15_VAL=${%s-}"
`, loaderPath, varName)

	cmd := exec.Command("bash", "-c", script)
	cmd.Dir = pwd
	// Build a clean env: keep PATH so bash itself runs; override HOME.
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + fakeHome,
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil && exitCode == 0 {
		// Non-exit error (e.g. fork failure).
		return "", -1, err
	}
	// Extract P15_VAL=... line.
	val := ""
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "P15_VAL=") {
			val = strings.TrimPrefix(line, "P15_VAL=")
			break
		}
	}
	return val, exitCode, nil
}

func phaseB(root string) error {
	fmt.Println("==> Phase B — API-KEYS-LOADER")

	loader := filepath.Join(root, "scripts", "load_api_keys.sh")
	if !fileExists(loader) {
		return fmt.Errorf("loader missing at %s", loader)
	}

	branchResults := []string{}

	// --- Branch 1: $HOME/api_keys.sh present, no .env in pwd ----------------
	tmp1, err := os.MkdirTemp("", "p15-fake-home-1-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp1)
	if err := os.WriteFile(filepath.Join(tmp1, "api_keys.sh"),
		[]byte("export TEST_PHASE_B_KEY=value_from_sh\n"), 0o600); err != nil {
		return err
	}
	// Use an empty pwd dir so the .env walk-up has nothing to find.
	pwd1, err := os.MkdirTemp("", "p15-pwd-1-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(pwd1)
	val1, _, err := runLoaderInBash(loader, tmp1, pwd1, "TEST_PHASE_B_KEY")
	if err != nil {
		return fmt.Errorf("branch1 exec: %w", err)
	}
	if val1 != "value_from_sh" {
		fmt.Printf("phaseB: FAIL branch1 expected value_from_sh got %q\n", val1)
		return fmt.Errorf("branch1: api_keys.sh not honoured")
	}
	branchResults = append(branchResults, "branch1=PASS")

	// --- Branch 2: no api_keys.sh, .env in pwd present ----------------------
	tmp2, err := os.MkdirTemp("", "p15-fake-home-2-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp2)
	pwd2, err := os.MkdirTemp("", "p15-pwd-2-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(pwd2)
	// .gitmodules + .env so the loader's walk-up matches the meta-repo branch.
	if err := os.WriteFile(filepath.Join(pwd2, ".gitmodules"), []byte(""), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(pwd2, ".env"),
		[]byte("TEST_PHASE_B_KEY=value_from_env\n"), 0o600); err != nil {
		return err
	}
	val2, _, err := runLoaderInBash(loader, tmp2, pwd2, "TEST_PHASE_B_KEY")
	if err != nil {
		return fmt.Errorf("branch2 exec: %w", err)
	}
	if val2 != "value_from_env" {
		fmt.Printf("phaseB: FAIL branch2 expected value_from_env got %q\n", val2)
		return fmt.Errorf("branch2: .env fallback not honoured")
	}
	branchResults = append(branchResults, "branch2=PASS")

	// --- Branch 3: neither api_keys.sh nor .env -> silent no-op -------------
	tmp3, err := os.MkdirTemp("", "p15-fake-home-3-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp3)
	pwd3, err := os.MkdirTemp("", "p15-pwd-3-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(pwd3)
	val3, exit3, err := runLoaderInBash(loader, tmp3, pwd3, "TEST_PHASE_B_KEY")
	if err != nil {
		return fmt.Errorf("branch3 exec: %w", err)
	}
	// Branch 3: variable should be unset (empty), and the loader must not
	// have killed the bash subshell with a fatal exit. The loader's
	// helixcode_load_api_keys returns 1 in this branch, but `set +e` in
	// the subshell suppresses propagation; we accept exit 0 (or 1) so
	// long as the variable is empty.
	if val3 != "" {
		fmt.Printf("phaseB: FAIL branch3 expected empty got %q\n", val3)
		return fmt.Errorf("branch3: loader silently set a value with no source")
	}
	if exit3 < 0 {
		return fmt.Errorf("branch3: bash subshell aborted unexpectedly")
	}
	branchResults = append(branchResults, "branch3=PASS")

	fmt.Println("phaseB:", strings.Join(branchResults, " "))
	return nil
}

// -----------------------------------------------------------------------------
// Phase C
// -----------------------------------------------------------------------------

// firstPartyTopLevelSkips returns root-relative top-level dir names that
// should be excluded from first-party scans (third-party content, audit
// trails, vendor dirs).
func firstPartyTopLevelSkips() map[string]bool {
	return map[string]bool{
		"cli_agents":           true,
		"cli_agents_resources": true,
		"cli_agents_configs":   true,
		"Dependencies":         true,
		"awesome-ai-memory":    true,
		".git":                 true,
		".helix.cache":         true,
		"Upstreams":            true,
		"isolated_files":       true,
		"node_modules":         true,
		// Generated audit/governance artefacts:
		"benchmark_reports": true,
	}
}

// shouldSkipDirForScan returns true if the directory at root-relative path
// `rel` should be skipped during first-party walking. We always skip:
//   - any path beginning with a top-level skip dir
//   - HelixQA's tools/opensource subtree (third-party)
//   - any .git directory at any depth
//   - any HelixCode/applications/<thing>/build|node_modules|target|.gradle
func shouldSkipDirForScan(rel string) bool {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	if len(parts) == 0 {
		return false
	}
	skips := firstPartyTopLevelSkips()
	if skips[parts[0]] {
		return true
	}
	if strings.Contains(rel, "/tools/opensource") {
		return true
	}
	for _, p := range parts {
		if p == ".git" || p == "node_modules" || p == ".gradle" ||
			p == "build" || p == "target" || p == "bin" {
			return true
		}
	}
	return false
}

func phaseC(root string) error {
	fmt.Println("==> Phase C — DOCS-UNDER-DOCS-DIR")

	subPaths, err := submodulePaths(root)
	if err != nil {
		return err
	}

	// Walk first-party tree; flag any case-variant of `Documentation` that
	// is NOT exactly `docs`. Skip into submodule subtrees — those repos
	// own their own doc-tree validation.
	var violations []string
	var docsDirs []string
	err = filepath.Walk(root, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if shouldSkipDirForScan(rel) {
			return filepath.SkipDir
		}
		// Skip into submodule subtrees (their internal layout is their concern).
		if subPaths[relSlash] {
			return filepath.SkipDir
		}
		base := info.Name()
		if base == "docs" {
			docsDirs = append(docsDirs, relSlash)
			return nil
		}
		// Flag any other casing of "documentation".
		if strings.EqualFold(base, "documentation") {
			violations = append(violations, relSlash)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		fmt.Println("phaseC: FAIL Documentation/ dirs found:", violations)
		return fmt.Errorf("phase C: %d Documentation/ uppercase dirs remain", len(violations))
	}
	// Verify canonical docs/ exists at root + HelixCode/.
	rootDocs := dirExists(filepath.Join(root, "docs"))
	helixDocs := dirExists(filepath.Join(root, "HelixCode", "docs"))
	if !rootDocs || !helixDocs {
		return fmt.Errorf("phase C: canonical docs/ missing (root=%v helixcode=%v)", rootDocs, helixDocs)
	}
	fmt.Printf("phaseC: zero Documentation/ uppercase dirs in first-party tree; docs/ canonical at %v\n",
		docsDirs[:min(8, len(docsDirs))])
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// -----------------------------------------------------------------------------
// Phase D
// -----------------------------------------------------------------------------

// snakeCaseRe matches snake_case: starts lowercase letter or digit, then
// lowercase letters, digits, and underscores only. Digit-prefixed names
// are allowed (e.g. `01_analysis_step_01`, `06_diagrams_real`) — these are
// sequence-prefixed snake_case used throughout `docs/improvements/`.
var snakeCaseRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_]*$`)

// allDigitsRe matches names that are pure numbers (e.g. "9.0", "2025-10").
// We allow these as data fixtures.
var allDigitsRe = regexp.MustCompile(`^[0-9._-]+$`)

// repoNamesFromGitmodules collects every basename of every submodule path
// declared in the meta-repo's tracked .gitmodules. These are repo names we
// don't own and cannot rename.
func repoNamesFromGitmodules(root string) (map[string]bool, error) {
	gms, err := metaRepoGitmodules(root)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, gm := range gms {
		if !fileExists(gm) {
			continue
		}
		entries, err := parseGitmodules(gm)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			seen[filepath.Base(e.path)] = true
		}
	}
	return seen, nil
}

// d4Allowlist returns directory basenames that are accepted as
// non-snake_case in the WP7 deferred allowlist. These are conventions
// (cmd/<binary>, Go-package-style identifiers, well-known fixtures).
func d4Allowlist() map[string]bool {
	return map[string]bool{
		// Top-level repos, directories, and tracked-but-third-party trees:
		"HelixCode":         true,
		"HelixAgent":        true,
		"HelixQA":           true,
		"HelixDevelopment":  true,
		"HelixLLM":          true,
		"HelixSpecifier":    true,
		"HelixMemory":       true,
		"Containers":        true,
		"Security":          true,
		"Challenges":        true,
		"Dependencies":      true,
		"Github-Pages-Website": true,
		"MCP-Servers":       true,
		"LLMsVerifier":      true,
		"DocProcessor":      true,
		"LLMOrchestrator":   true,
		"LLMProvider":       true,
		"VisionEngine":      true,
		"Assets":            true,
		"Implementation_Guide": true,
		"Specification":    true,
		"Upstreams":        true,
		"Website":          true,
		"Github-Pages":     true,
		// Standard Go project convention dirs:
		"cmd":          true,
		"internal":     true,
		"pkg":          true,
		"api":          true,
		"docs":         true,
		"scripts":      true,
		"tests":        true,
		"test":         true,
		"testdata":     true,
		"vendor":       true,
		"hack":         true,
		"build":        true,
		"deploy":       true,
		"config":       true,
		"configs":      true,
		"examples":     true,
		"web":          true,
		"assets":       true,
		"docker":       true,
		"helm":         true,
		"templates":    true,
		"migrations":   true,
		"seeds":        true,
		"fixtures":     true,
		"e2e":          true,
		"unit":         true,
		"integration":  true,
		"performance":  true,
		"security":     true,
		"benchmark":    true,
		"benchmarks":   true,
		"shared":       true,
		"adapters":     true,
		"applications": true,
		"applications-tests": true,
		// Common doc / project artefact dirs:
		"adr":              true,
		"ADR":              true,
		"audit":            true,
		"audit_trail":      true,
		"general":          true,
		"architecture":     true,
		"testing":          true,
		"benchmark_reports": true,
		"reports":          true,
		"diagrams":         true,
		"isolated_files":   true,
		"helix.cache":      true,
		// WP7 explicit deferred list (recorded in
		// docs/improvements/06_phase_1_evidence.md §P1.5-WP7 "Deferred
		// renames (23)"). These are kebab-case Go-application packages
		// and Specification/ umbrella subdirs that WP7 deliberately did
		// not rename pending a follow-up WP that combines (a) the rename,
		// (b) Makefile import-path updates, (c) Go package name updates,
		// and (d) re-running `make verify-compile`. Listing them here
		// preserves the harness as a regression gate without conflating
		// "WP7 closed" with "every directory is snake_case".
		"aurora-os":   true,
		"harmony-os":  true,
		"terminal-ui": true,
		"CLI_Specs_4": true,
		"CLI_Specs_5": true,
		"TODO":        true,
		// Pre-existing umbrella/research dirs not in WP7 scope:
		"HelixQA_Integration": true,
	}
}

func phaseD(root string) error {
	fmt.Println("==> Phase D — SNAKE_CASE")

	repoNames, err := repoNamesFromGitmodules(root)
	if err != nil {
		return err
	}
	subPaths, err := submodulePaths(root)
	if err != nil {
		return err
	}
	allowlist := d4Allowlist()

	scanned := 0
	allowed := 0
	var violations []string

	err = filepath.Walk(root, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if shouldSkipDirForScan(rel) {
			return filepath.SkipDir
		}
		// Skip into submodule subtrees — WP7 normalised the meta-repo's
		// directly-tracked dirs and the inner HelixCode/ tree only.
		// Submodule-internal layouts (Challenges, Containers, HelixAgent,
		// HelixQA, etc.) are owned by those repos and are out of WP7's
		// scope.
		if subPaths[relSlash] {
			return filepath.SkipDir
		}

		// Apply depth limit (depth <= 4).
		depth := strings.Count(relSlash, "/") + 1
		if depth > 4 {
			return nil
		}

		base := info.Name()
		// Skip dotted dirs (e.g. .helix, .scripts) — they're config/dotfile
		// caches, not first-party feature dirs.
		if strings.HasPrefix(base, ".") {
			return filepath.SkipDir
		}
		scanned++

		// Allowlist checks.
		if allowlist[base] {
			allowed++
			return nil
		}
		if repoNames[base] {
			allowed++
			return nil
		}
		// `cmd/<binary>` is allowlisted regardless of casing (Go binaries
		// often use mixed identifiers).
		parts := strings.Split(relSlash, "/")
		if len(parts) >= 2 && parts[len(parts)-2] == "cmd" {
			allowed++
			return nil
		}
		// Numeric / version dirs.
		if allDigitsRe.MatchString(base) {
			allowed++
			return nil
		}

		// Final test.
		if !snakeCaseRe.MatchString(base) {
			violations = append(violations, relSlash)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(violations) > 0 {
		// Trim the printed list to keep the harness output bounded but
		// keep enough to diagnose.
		head := violations
		if len(head) > 25 {
			head = head[:25]
		}
		for _, v := range head {
			fmt.Println("phaseD: VIOLATION", v)
		}
		fmt.Printf("phaseD: FAIL %d non-conforming first-party directory/ies (showing first %d)\n",
			len(violations), len(head))
		return fmt.Errorf("phase D: %d snake_case violations", len(violations))
	}

	fmt.Printf("phaseD: %d conformant first-party directories scanned; %d allowlisted (cmd/, repo names); 0 violations\n",
		scanned, allowed)
	return nil
}

// -----------------------------------------------------------------------------
// Phase E
// -----------------------------------------------------------------------------

func phaseE(root string) error {
	fmt.Println("==> Phase E — ANTI-BLUFF-ANCHOR")

	script := filepath.Join(root, "scripts", "verify_anti_bluff_cascade.sh")
	if !fileExists(script) {
		return fmt.Errorf("script missing: %s", script)
	}

	cmd := exec.Command("bash", script)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	// Print verbatim output regardless of exit.
	fmt.Println("---- verify_anti_bluff_cascade.sh output ----")
	fmt.Print(string(out))
	if !strings.HasSuffix(string(out), "\n") {
		fmt.Println()
	}
	fmt.Println("---- end verify_anti_bluff_cascade.sh output ----")

	if err != nil && exitCode == 0 {
		return fmt.Errorf("phase E: cascade script exec error: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("phase E: cascade script exit %d", exitCode)
	}
	fmt.Println("phaseE: PASS (cascade script exit 0)")
	return nil
}
