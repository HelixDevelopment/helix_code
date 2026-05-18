// Package main implements the CONST-046 hardcoded-content audit walker.
// It scans Go source trees for string literals that LOOK like user-facing
// natural-language English content (clarification questions, prompt
// templates, helper text, UI labels) that should instead be sourced from
// the i18n bundle / LLM / configuration per CONST-046. Round 92 is
// soft-warn (always exits 0); round 99 of Phase 3 tightens to fail-on-hit.
// The auditor's OWN strings are developer-facing infrastructure and are
// not themselves CONST-046 violations.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Finding records one suspected CONST-046 hardcoded-content violation.
type Finding struct {
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Col        int    `json:"col"`
	Excerpt    string `json:"excerpt"`
	ByteOffset int    `json:"byte_offset"`
}

// Report is the JSON-serialisable summary emitted under --json.
type Report struct {
	ScannedFiles   int       `json:"scanned_files"`
	SkippedFiles   int       `json:"skipped_files"`
	AllowlistHits  int       `json:"allowlist_hits"`
	Violations     []Finding `json:"violations"`
	HeuristicNotes string    `json:"heuristic_notes"`
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
	var (
		rootsCSV     string
		allowlistPth string
		jsonOut      bool
		quiet        bool
	)
	flag.StringVar(&rootsCSV, "roots", "", "comma-separated list of directories to scan (required)")
	flag.StringVar(&allowlistPth, "allowlist", "", "path to allowlist file (optional)")
	flag.BoolVar(&jsonOut, "json", false, "emit findings as JSON to stdout")
	flag.BoolVar(&quiet, "quiet", false, "suppress per-violation lines (summary only)")
	flag.Parse()

	if rootsCSV == "" {
		fmt.Fprintln(os.Stderr, "audit_const046: --roots is required")
		os.Exit(2)
	}
	allow, err := loadAllowlist(allowlistPth)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit_const046: allowlist load failed: %v\n", err)
		os.Exit(2)
	}
	report := Report{HeuristicNotes: "round-92 soft-warn; length>=16, 2+words, cap-or-punct, errors/log/test/comment exempt"}
	for _, root := range strings.Split(rootsCSV, ",") {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		walk(root, allow, &report)
	}
	sort.Slice(report.Violations, func(i, j int) bool {
		if report.Violations[i].Path != report.Violations[j].Path {
			return report.Violations[i].Path < report.Violations[j].Path
		}
		return report.Violations[i].Line < report.Violations[j].Line
	})

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
		os.Exit(0)
	}
	fmt.Printf("CONST-046 audit (soft-warn, round 92):\n")
	fmt.Printf("  scanned files : %d\n", report.ScannedFiles)
	fmt.Printf("  skipped files : %d\n", report.SkippedFiles)
	fmt.Printf("  allowlist hits: %d\n", report.AllowlistHits)
	fmt.Printf("  violations    : %d\n\n", len(report.Violations))
	if !quiet {
		for _, v := range report.Violations {
			fmt.Printf("%s:%d:%d: %s\n", v.Path, v.Line, v.Col, v.Excerpt)
		}
	}
	os.Exit(0)
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
