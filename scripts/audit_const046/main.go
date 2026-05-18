// Package main implements the CONST-046 hardcoded-content audit walker.
// It scans Go source trees for string literals that LOOK like user-facing
// natural-language English content (clarification questions, prompt
// templates, helper text, UI labels) that should instead be sourced from
// the i18n bundle / LLM / configuration per CONST-046.
//
// Round 92 introduced the auditor in soft-warn mode (always exits 0).
// Round 99b (this revision) adds a BASELINE mechanism so the gate can
// fail-on-NEW-violations while tolerating the pre-existing ~57k findings
// that cannot all be migrated in one round. Operating modes:
//
//	(default)         soft-warn — reports all findings, exits 0
//	--fail-on-new     diff vs baseline; exit 1 iff any violation is NOT
//	                  in the baseline (matched by path + literal hash)
//	--update-baseline regenerate the baseline snapshot, exit 0
//
// The auditor's OWN strings are developer-facing infrastructure and are
// not themselves CONST-046 violations.
package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// toolVersion bumps when the heuristic or baseline schema changes.
const toolVersion = "0.2"

// baselineSchemaVersion bumps when the JSON shape changes.
const baselineSchemaVersion = 1

// Finding records one suspected CONST-046 hardcoded-content violation.
type Finding struct {
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Col        int    `json:"col"`
	Excerpt    string `json:"excerpt"`
	ByteOffset int    `json:"byte_offset"`
	// LiteralHash is the first 16 hex chars of sha256 of the literal
	// text (UNQUOTED). Used for identity matching against the baseline
	// because line numbers shift over time but the literal text is
	// stable until migrated.
	LiteralHash string `json:"literal_hash,omitempty"`
}

// Report is the JSON-serialisable summary emitted under --json.
type Report struct {
	ScannedFiles   int       `json:"scanned_files"`
	SkippedFiles   int       `json:"skipped_files"`
	AllowlistHits  int       `json:"allowlist_hits"`
	Violations     []Finding `json:"violations"`
	HeuristicNotes string    `json:"heuristic_notes"`
}

// BaselineEntry is the minimal violation identity persisted on disk.
type BaselineEntry struct {
	Path        string `json:"path"`
	Line        int    `json:"line"`
	ByteOffset  int    `json:"byte_offset"`
	LiteralHash string `json:"literal_hash"`
}

// Baseline is the on-disk JSON snapshot.
type Baseline struct {
	SchemaVersion int             `json:"schema_version"`
	GeneratedAt   string          `json:"generated_at"`
	ToolVersion   string          `json:"tool_version"`
	Violations    []BaselineEntry `json:"violations"`
}

var (
	twoWordRE              = regexp.MustCompile(`[A-Za-z]+\s+[A-Za-z]+`)
	punctuationIndicatorRE = regexp.MustCompile(`[?!.:](\s|$)`)
	startsWithCapitalRE    = regexp.MustCompile(`^[A-Z]`)
)

// AllowEntry: path-substring + literal-prefix. Both must match (or one
// be empty) for a finding to be suppressed.
type AllowEntry struct {
	PathContains  string
	LiteralPrefix string
}

func main() {
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run is the testable entry point. Returns the desired exit code.
// All output is routed through stdout/stderr writers so in-process
// tests can capture and assert without spawning subprocesses.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("audit_const046", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		rootsCSV     string
		allowlistPth string
		jsonOut      bool
		quiet        bool
		baselinePth  string
		failOnNew    bool
		updateBase   bool
	)
	fs.StringVar(&rootsCSV, "roots", "", "comma-separated list of directories to scan (required)")
	fs.StringVar(&allowlistPth, "allowlist", "", "path to allowlist file (optional)")
	fs.BoolVar(&jsonOut, "json", false, "emit findings as JSON to stdout")
	fs.BoolVar(&quiet, "quiet", false, "suppress per-violation lines (summary only)")
	fs.StringVar(&baselinePth, "baseline", "", "path to baseline JSON file (default: <script_dir>/.baseline.json)")
	fs.BoolVar(&failOnNew, "fail-on-new", false, "exit 1 if any violation is not present in baseline")
	fs.BoolVar(&updateBase, "update-baseline", false, "regenerate baseline JSON snapshot and exit 0")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if rootsCSV == "" {
		fmt.Fprintln(stderr, "audit_const046: --roots is required")
		return 2
	}

	// Resolve default baseline location if not supplied.
	if baselinePth == "" {
		exe, err := os.Executable()
		if err == nil {
			baselinePth = filepath.Join(filepath.Dir(exe), ".baseline.json")
		}
		// Override with source-side path when the binary lives in a
		// temp dir (typical for tests + shell wrapper builds).
		if src := defaultBaselineFromSource(); src != "" {
			baselinePth = src
		}
	}

	allow, err := loadAllowlist(allowlistPth)
	if err != nil {
		fmt.Fprintf(stderr, "audit_const046: allowlist load failed: %v\n", err)
		return 2
	}

	report := Report{HeuristicNotes: "round-99b baseline-aware; length>=16, 2+words, cap-or-punct, errors/log/test/comment exempt"}
	for _, root := range strings.Split(rootsCSV, ",") {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		walk(root, allow, &report)
	}
	// Compute literal hash for every violation (re-read the source
	// region for stability; cheaper to recompute from the excerpt
	// since the excerpt is already truncated — instead we read the
	// raw bytes via byte offset for full fidelity).
	hashViolations(&report)

	sort.Slice(report.Violations, func(i, j int) bool {
		if report.Violations[i].Path != report.Violations[j].Path {
			return report.Violations[i].Path < report.Violations[j].Path
		}
		return report.Violations[i].Line < report.Violations[j].Line
	})

	// --update-baseline path: write snapshot, exit 0 unconditionally.
	if updateBase {
		if baselinePth == "" {
			fmt.Fprintln(stderr, "audit_const046: --update-baseline requires --baseline path (or default resolution)")
			return 2
		}
		if err := writeBaseline(baselinePth, &report); err != nil {
			fmt.Fprintf(stderr, "audit_const046: failed to write baseline: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "baseline updated: %d violations recorded → %s\n", len(report.Violations), baselinePth)
		return 0
	}

	// --fail-on-new path: load baseline, classify NEW vs PRE-EXISTING.
	if failOnNew {
		baseSet, baseLoaded, err := loadBaselineSet(baselinePth)
		if err != nil {
			fmt.Fprintf(stderr, "audit_const046: failed to load baseline %s: %v\n", baselinePth, err)
			return 2
		}
		if !baseLoaded {
			fmt.Fprintf(stderr, "audit_const046: WARNING: baseline file not found at %s — treating as empty (all violations will be classified NEW)\n", baselinePth)
		}
		newCount, preCount := 0, 0
		var newViolations []Finding
		for _, v := range report.Violations {
			key := violationKey(v.Path, v.LiteralHash)
			if _, ok := baseSet[key]; ok {
				preCount++
			} else {
				newCount++
				newViolations = append(newViolations, v)
			}
		}
		fmt.Fprintf(stdout, "CONST-046 audit (fail-on-new, round 99b):\n")
		fmt.Fprintf(stdout, "  scanned files : %d\n", report.ScannedFiles)
		fmt.Fprintf(stdout, "  skipped files : %d\n", report.SkippedFiles)
		fmt.Fprintf(stdout, "  allowlist hits: %d\n", report.AllowlistHits)
		fmt.Fprintf(stdout, "  baseline file : %s (loaded=%v, entries=%d)\n", baselinePth, baseLoaded, len(baseSet))
		fmt.Fprintf(stdout, "  Total: %d (NEW: %d, PRE-EXISTING: %d)\n\n", len(report.Violations), newCount, preCount)
		if !quiet && newCount > 0 {
			fmt.Fprintln(stdout, "NEW violations not present in baseline:")
			for _, v := range newViolations {
				fmt.Fprintf(stdout, "  %s:%d:%d: %s\n", v.Path, v.Line, v.Col, v.Excerpt)
			}
		}
		if newCount > 0 {
			return 1
		}
		return 0
	}

	// Default soft-warn path (unchanged from round 92 surface).
	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
		return 0
	}
	fmt.Fprintf(stdout, "CONST-046 audit (soft-warn, round 92/99b):\n")
	fmt.Fprintf(stdout, "  scanned files : %d\n", report.ScannedFiles)
	fmt.Fprintf(stdout, "  skipped files : %d\n", report.SkippedFiles)
	fmt.Fprintf(stdout, "  allowlist hits: %d\n", report.AllowlistHits)
	fmt.Fprintf(stdout, "  violations    : %d\n\n", len(report.Violations))
	if !quiet {
		for _, v := range report.Violations {
			fmt.Fprintf(stdout, "%s:%d:%d: %s\n", v.Path, v.Line, v.Col, v.Excerpt)
		}
	}
	return 0
}

// hashViolations enriches each finding with the sha256 short-hash of
// its literal text. Re-reading the file is acceptable here because the
// fail-on-new path is invoked far less often than the soft-warn one.
func hashViolations(r *Report) {
	// Cache by path so each file is read at most once.
	cache := map[string][]byte{}
	for i := range r.Violations {
		v := &r.Violations[i]
		src, ok := cache[v.Path]
		if !ok {
			b, err := os.ReadFile(v.Path)
			if err != nil {
				continue
			}
			src = b
			cache[v.Path] = b
		}
		lit := extractLiteralAt(src, v.ByteOffset)
		if lit == "" {
			// Fall back to the excerpt — less stable but better
			// than no hash at all.
			lit = v.Excerpt
		}
		v.LiteralHash = shortHash(lit)
	}
}

// extractLiteralAt extracts the quoted string literal that starts at
// `offset` in `src`. Handles "..." and `...` (raw) literals.
func extractLiteralAt(src []byte, offset int) string {
	if offset < 0 || offset >= len(src) {
		return ""
	}
	first := src[offset]
	if first != '"' && first != '`' {
		return ""
	}
	end := -1
	if first == '`' {
		for i := offset + 1; i < len(src); i++ {
			if src[i] == '`' {
				end = i
				break
			}
		}
	} else {
		// "..." with backslash escapes.
		for i := offset + 1; i < len(src); i++ {
			if src[i] == '\\' && i+1 < len(src) {
				i++
				continue
			}
			if src[i] == '"' {
				end = i
				break
			}
		}
	}
	if end < 0 {
		return ""
	}
	return string(src[offset : end+1])
}

// shortHash returns the first 16 hex chars of sha256(s).
func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:16]
}

// violationKey is the identity used for baseline comparison.
// Same (path, literal_hash) ⇒ same violation, even if line shifted.
func violationKey(path, hash string) string {
	return path + "\x00" + hash
}

// defaultBaselineFromSource attempts to locate the canonical baseline
// path relative to THIS source file (works for `go run`, tests, and
// the shell wrapper that builds into a tmp dir). Returns "" if not
// discoverable.
func defaultBaselineFromSource() string {
	// Best-effort: walk up from CWD looking for scripts/audit_const046/
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		cand := filepath.Join(dir, "scripts", "audit_const046", ".baseline.json")
		if _, err := os.Stat(filepath.Dir(cand)); err == nil {
			return cand
		}
	}
	return ""
}

// loadBaselineSet reads the baseline JSON and returns a (path, hash)
// keyed set. Returns (empty, false, nil) when the file does not exist.
// Transparently supports gzip-compressed baselines (the real-tree
// snapshot is ~16 MB JSON / ~500 KB gzip; we ship gzip in the repo).
// Resolution order: explicit `path`, then `path + ".gz"`, then "not loaded".
func loadBaselineSet(path string) (map[string]struct{}, bool, error) {
	if path == "" {
		return map[string]struct{}{}, false, nil
	}
	data, err := readBaselineBytes(path)
	if err != nil {
		return nil, false, err
	}
	if data == nil {
		return map[string]struct{}{}, false, nil
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, false, fmt.Errorf("parse baseline: %w", err)
	}
	out := make(map[string]struct{}, len(b.Violations))
	for _, e := range b.Violations {
		out[violationKey(e.Path, e.LiteralHash)] = struct{}{}
	}
	return out, true, nil
}

// readBaselineBytes tries `path` first, then `path + ".gz"` (gunzipping).
// Returns (nil, nil) when neither file exists.
func readBaselineBytes(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	gz := path + ".gz"
	f, err := os.Open(gz)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("open gzip baseline: %w", err)
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

// writeBaseline serializes report violations to the baseline path
// using atomic rename (write tmp, rename). Path ending in ".gz" causes
// gzip compression (recommended — 16 MB → 500 KB on the real tree).
func writeBaseline(path string, r *Report) error {
	entries := make([]BaselineEntry, 0, len(r.Violations))
	for _, v := range r.Violations {
		entries = append(entries, BaselineEntry{
			Path:        v.Path,
			Line:        v.Line,
			ByteOffset:  v.ByteOffset,
			LiteralHash: v.LiteralHash,
		})
	}
	b := Baseline{
		SchemaVersion: baselineSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		ToolVersion:   toolVersion,
		Violations:    entries,
	}
	data, err := json.MarshalIndent(&b, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if strings.HasSuffix(path, ".gz") {
		f, err := os.Create(tmp)
		if err != nil {
			return err
		}
		zw := gzip.NewWriter(f)
		if _, err := zw.Write(data); err != nil {
			zw.Close()
			f.Close()
			return err
		}
		if err := zw.Close(); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	} else {
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return err
		}
	}
	return os.Rename(tmp, path)
}

// loadAllowlist parses "<path-substr>\t<literal-prefix>" lines (with
// " :: " separator as fallback). Empty/missing file => nil.
func loadAllowlist(path string) ([]AllowEntry, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []AllowEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var pathPart, litPart string
		if i := strings.Index(line, "\t"); i >= 0 {
			pathPart = strings.TrimSpace(line[:i])
			litPart = strings.TrimSpace(line[i+1:])
		} else if i := strings.Index(line, " :: "); i >= 0 {
			pathPart = strings.TrimSpace(line[:i])
			litPart = strings.TrimSpace(line[i+4:])
		} else {
			pathPart = line
		}
		out = append(out, AllowEntry{PathContains: pathPart, LiteralPrefix: litPart})
	}
	return out, nil
}

var skipDirNames = map[string]struct{}{
	".git": {}, "node_modules": {}, "target": {}, "bin": {}, "build": {},
	"dist": {}, "vendor": {}, "testdata": {},
	"cli_agents": {}, "cli_agents_resources": {},
}

func walk(root string, allow []AllowEntry, report *Report) {
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if _, skip := skipDirNames[filepath.Base(p)]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		if strings.HasSuffix(p, "_test.go") {
			report.SkippedFiles++
			return nil
		}
		if strings.Contains(p, "/scripts/audit_const046/") {
			report.SkippedFiles++
			return nil
		}
		report.ScannedFiles++
		scanFile(p, allow, report)
		return nil
	})
}

func scanFile(path string, allow []AllowEntry, report *Report) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		report.SkippedFiles++
		return
	}
	scanASTFile(fset, file, allow, report)
}

// scanASTFile is the shared AST-level scan logic. Keeping it in one
// place ensures the mutation test (delete the append) proves the
// production detector path.
func scanASTFile(fset *token.FileSet, file *ast.File, allow []AllowEntry, report *Report) {
	var exempt, exemptEnd []token.Pos
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && isExemptCall(call) {
			exempt = append(exempt, call.Pos())
			exemptEnd = append(exemptEnd, call.End())
		}
		return true
	})
	posExempt := func(p token.Pos) bool {
		for i := range exempt {
			if p >= exempt[i] && p < exemptEnd[i] {
				return true
			}
		}
		return false
	}
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || posExempt(lit.Pos()) {
			return true
		}
		text := unquoteLiteral(lit.Value)
		if text == "" || !looksUserFacing(text) {
			return true
		}
		pos := fset.Position(lit.Pos())
		excerpt := text
		if len(excerpt) > 60 {
			excerpt = excerpt[:60] + "..."
		}
		f := Finding{Path: pos.Filename, Line: pos.Line, Col: pos.Column, Excerpt: excerpt, ByteOffset: pos.Offset}
		if isAllowlisted(f, text, allow) {
			report.AllowlistHits++
			return true
		}
		// For in-test paths we also compute the hash here from the raw
		// literal text so the test fixtures (which never go through
		// hashViolations because they bypass run()) still get hashes.
		f.LiteralHash = shortHash(lit.Value)
		report.Violations = append(report.Violations, f)
		return true
	})
}

// isExemptCall returns true for call expressions whose string-literal
// arguments are developer-facing (errors, logging, CLI help) and so are
// not CONST-046 user-facing content.
func isExemptCall(call *ast.CallExpr) bool {
	switch callName(call.Fun) {
	case "errors.New", "fmt.Errorf",
		"errors.Wrap", "errors.Wrapf",
		"pkgerrors.New", "pkgerrors.Wrap",
		"xerrors.New", "xerrors.Errorf",
		"log.Print", "log.Printf", "log.Println",
		"log.Fatal", "log.Fatalf", "log.Fatalln",
		"log.Panic", "log.Panicf", "log.Panicln",
		"log.Debug", "log.Debugf", "log.Info", "log.Infof",
		"log.Warn", "log.Warnf", "log.Error", "log.Errorf",
		"slog.Debug", "slog.Info", "slog.Warn", "slog.Error",
		"slog.DebugContext", "slog.InfoContext",
		"slog.WarnContext", "slog.ErrorContext",
		"glog.Info", "glog.Warning", "glog.Error", "glog.Fatal",
		"logrus.Info", "logrus.Warn", "logrus.Error", "logrus.Debug",
		"flag.String", "flag.Bool", "flag.Int", "flag.Int64",
		"flag.Float64", "flag.Duration",
		"pflag.String", "pflag.StringP",
		"pflag.Bool", "pflag.BoolP",
		"pflag.Int", "pflag.IntP":
		return true
	}
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		switch sel.Sel.Name {
		case "Debug", "Debugf", "Debugln",
			"Info", "Infof", "Infoln",
			"Warn", "Warnf", "Warnln", "Warning", "Warningf",
			"Error", "Errorf", "Errorln",
			"Fatal", "Fatalf", "Fatalln",
			"Trace", "Tracef":
			return true
		}
	}
	return false
}

func callName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			return id.Name + "." + x.Sel.Name
		}
		return x.Sel.Name
	}
	return ""
}

// unquoteLiteral strips surrounding quotes / backticks (keeps escape
// sequences visible for the heuristic).
func unquoteLiteral(raw string) string {
	if len(raw) < 2 {
		return ""
	}
	first, last := raw[0], raw[len(raw)-1]
	if (first == '"' && last == '"') || (first == '`' && last == '`') {
		return raw[1 : len(raw)-1]
	}
	return raw
}

// looksUserFacing applies the round-92 heuristic: length >= 16,
// 2+ ASCII words, capitalised or sentence-punctuated, and NOT obviously
// a URL / import path / struct tag / SQL keyword stream.
func looksUserFacing(s string) bool {
	if len(s) < 16 {
		return false
	}
	t := strings.TrimSpace(s)
	if len(t) < 16 || !twoWordRE.MatchString(t) {
		return false
	}
	if strings.Contains(t, "://") {
		return false
	}
	if strings.HasPrefix(t, "github.com/") || strings.HasPrefix(t, "golang.org/") {
		return false
	}
	if strings.HasPrefix(t, "json:") || strings.HasPrefix(t, "yaml:") ||
		strings.HasPrefix(t, "xml:") || strings.HasPrefix(t, "db:") {
		return false
	}
	upperWords, totalWords := 0, 0
	for _, w := range strings.Fields(t) {
		totalWords++
		if w == strings.ToUpper(w) && len(w) >= 3 {
			upperWords++
		}
	}
	if totalWords > 0 && upperWords*2 >= totalWords {
		return false
	}
	return startsWithCapitalRE.MatchString(t) || punctuationIndicatorRE.MatchString(t)
}

func isAllowlisted(f Finding, full string, allow []AllowEntry) bool {
	for _, e := range allow {
		if e.PathContains != "" && !strings.Contains(f.Path, e.PathContains) {
			continue
		}
		if e.LiteralPrefix != "" && !strings.HasPrefix(full, e.LiteralPrefix) {
			continue
		}
		return true
	}
	return false
}
