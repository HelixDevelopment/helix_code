package llm

// §11.4.135 standing regression guard for the semgrep exec-command triage
// (docs/research/semgrep_exec_triage_20260622/findings.md).
//
// Findings 1 & 2 (auto_llm_manager.go / local_llm_manager.go) feed
// `LocalLLMProvider.BuildScript` (and the AutoProvider copy) into
// `exec.Command("bash", "-c", ...)` — a true shell-string form. The semgrep
// `nosemgrep` suppressions on those two sites are SAFE *only* because the
// `BuildScript` field is sourced exclusively from the compile-time-static
// `providerDefinitions` map literal and is NEVER populated from user input,
// config files, HTTP bodies, or any deserializer.
//
// This guard locks that invariant in source. It scans the package's own
// non-test `.go` files at test time and FAILs (RED) the moment anyone makes
// `BuildScript` config-loadable — e.g. a `json.Unmarshal` / `yaml.Unmarshal`
// / `viper.*` call writing into a struct whose `BuildScript` field would then
// flow to `bash -c`. If that ever happens, findings 1 & 2 flip from SAFE to
// REAL command-injection risk and the suppressions must be removed.
//
// It is a real `go test`: it reads the actual package source from disk and
// asserts the deserialization-into-BuildScript pattern is absent and that
// every write into a `.BuildScript` field is a trusted (static-literal or
// static-map-copy) source.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// packageGoSources returns the absolute paths of every non-test `.go` file in
// the current package directory (the directory holding this test file).
func packageGoSources(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	var sources []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue // production source only — mocks/test helpers excluded
		}
		sources = append(sources, name)
	}
	if len(sources) == 0 {
		t.Fatal("no production .go sources found in package dir; guard would be vacuous")
	}
	return sources
}

// decoderCall matches an actual deserializer *call expression* — the thing
// that populates a struct from external bytes/config: json/yaml/toml
// `Unmarshal(`, a streaming `.Decode(`, or a viper `Unmarshal(` / getter.
// (The struct TAG `json:"build_script"` on a field declaration is NOT a
// decoder call and is intentionally not matched — only an invocation that
// would route external input into BuildScript is the risk.)
var decoderCall = regexp.MustCompile(
	`\b(json|yaml|toml|viper)\.(Unmarshal|NewDecoder|Get\w*)\b|\.Decode\(`,
)

// buildScriptFieldDecl matches the struct field *declaration* line
// (`BuildScript  string \`json:...\``) so it is excluded from decoder-write
// detection — a declaration is not a write from external input.
var buildScriptFieldDecl = regexp.MustCompile(`\bBuildScript\s+string\b`)

// buildScriptStructType is the only struct type that carries the BuildScript
// field (verified at authoring time). A decoder call targeting this type is
// the precise config-loadable risk this guard locks out.
const buildScriptStructType = "LocalLLMProvider"

// structVarDecl matches `var <name> LocalLLMProvider` (optionally pointer).
var structVarDecl = regexp.MustCompile(`\bvar\s+(\w+)\s+\*?` + buildScriptStructType + `\b`)

// structVarShortDecl matches `<name> := [&]LocalLLMProvider{`.
var structVarShortDecl = regexp.MustCompile(`\b(\w+)\s*:=\s*&?` + buildScriptStructType + `\b`)

// structVarRef reports whether a decoder-call line references the given var by
// name as a whole word (so `p` does not match `ptr`). The idiomatic risk form
// is `json.Unmarshal(data, &p)`.
func structVarRef(line, name string) bool {
	re := regexp.MustCompile(`(^|[^.\w])` + regexp.QuoteMeta(name) + `\b`)
	return re.MatchString(line)
}

// Matches any write into a `.BuildScript` field:
//   - struct-literal form:  `BuildScript: <expr>`
//   - assignment form:      `<x>.BuildScript = <expr>`
var buildScriptStructLiteralWrite = regexp.MustCompile(`(^|[^.\w])BuildScript\s*:`)
var buildScriptAssignWrite = regexp.MustCompile(`\.BuildScript\s*=[^=]`)

// trustedBuildScriptSource reports whether the right-hand side of a
// BuildScript write is a trusted (static) source: a Go string literal, or a
// copy from the static providerDefinitions map (`providerDef.BuildScript` /
// `definition.BuildScript`).
func trustedBuildScriptSource(rhs string) bool {
	rhs = strings.TrimSpace(rhs)
	rhs = strings.TrimRight(rhs, ",")
	rhs = strings.TrimSpace(rhs)
	if rhs == "" {
		return false
	}
	// Static string literal (interpreted or raw).
	if strings.HasPrefix(rhs, `"`) || strings.HasPrefix(rhs, "`") {
		return true
	}
	// Copy from the static providerDefinitions map literal.
	if rhs == "providerDef.BuildScript" || rhs == "definition.BuildScript" {
		return true
	}
	return false
}

// TestBuildScriptProvenanceGuard is the §11.4.135 standing GREEN guard.
//
// RED behaviour: if someone introduces a deserialization path into
// `BuildScript`, or assigns it from a non-static source, this test FAILs —
// tripping before the unsafe code can reach a `bash -c` site.
func TestBuildScriptProvenanceGuard(t *testing.T) {
	sources := packageGoSources(t)

	var (
		deserializeViolations []string
		untrustedWrites       []string
		sawAnyWrite           bool
	)

	for _, file := range sources {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		lines := strings.Split(string(data), "\n")

		// Strip line comments once so the `nosemgrep` rationale comments (which
		// legitimately mention BuildScript) are never mistaken for code.
		code := make([]string, len(lines))
		for i, raw := range lines {
			if idx := strings.Index(raw, "//"); idx >= 0 {
				code[i] = raw[:idx]
			} else {
				code[i] = raw
			}
		}

		// PRE-PASS: collect local variable names declared as the
		// BuildScript-bearing struct type (`var p LocalLLMProvider`,
		// `p := LocalLLMProvider{...}`, `p := &LocalLLMProvider{...}`). A
		// decoder call later targeting one of these vars is the idiomatic
		// cross-line config-loadable pattern that a same-line check misses.
		structVarNames := map[string]bool{}
		for _, c := range code {
			if m := structVarDecl.FindStringSubmatch(c); m != nil {
				structVarNames[m[1]] = true // `var p LocalLLMProvider`
			}
			if m := structVarShortDecl.FindStringSubmatch(c); m != nil {
				structVarNames[m[1]] = true // `p := [&]LocalLLMProvider{`
			}
		}

		for i, raw := range lines {
			line := code[i]
			lineNo := i + 1

			// (1) No deserializer call may route external input into the
			// BuildScript field or its enclosing struct. Flag a decoder call
			// that EITHER (a) references BuildScript / the struct type on the
			// same line, OR (b) takes the address of a var declared as the
			// BuildScript-bearing struct type (the `json.Unmarshal(data, &p)`
			// idiom). The struct field declaration line (with its `json:"..."`
			// tag) is NOT a decoder call and is never matched.
			if decoderCall.MatchString(line) {
				flag := strings.Contains(line, "BuildScript") ||
					strings.Contains(line, buildScriptStructType)
				if !flag {
					for name := range structVarNames {
						if structVarRef(line, name) {
							flag = true
							break
						}
					}
				}
				if flag {
					deserializeViolations = append(deserializeViolations,
						fileLinef(file, lineNo, raw))
				}
			}

			if !strings.Contains(line, "BuildScript") {
				continue
			}
			if buildScriptFieldDecl.MatchString(line) {
				continue // field declaration, not a write from a value
			}

			// (2) Every write into a .BuildScript field must be trusted.
			if buildScriptStructLiteralWrite.MatchString(line) {
				sawAnyWrite = true
				rhs := afterFirst(line, ":")
				if !trustedBuildScriptSource(rhs) {
					untrustedWrites = append(untrustedWrites,
						fileLinef(file, lineNo, raw))
				}
			}
			if buildScriptAssignWrite.MatchString(line) {
				sawAnyWrite = true
				rhs := afterFirst(line, "=")
				if !trustedBuildScriptSource(rhs) {
					untrustedWrites = append(untrustedWrites,
						fileLinef(file, lineNo, raw))
				}
			}
		}
	}

	// Sanity: the guard must actually be observing the static writes it
	// protects. If no BuildScript write is seen at all, the field was renamed
	// or removed and this guard is silently vacuous — fail loudly so it is
	// re-pointed rather than passing on nothing.
	if !sawAnyWrite {
		t.Fatal("guard saw zero BuildScript writes in package source; the field was renamed/removed — re-point this §11.4.135 guard")
	}

	if len(deserializeViolations) > 0 {
		t.Errorf("§11.4.135 VIOLATION: BuildScript is being made config-loadable via a deserializer. "+
			"This flips semgrep findings 1 & 2 (bash -c) from SAFE to REAL command-injection risk; "+
			"remove the deserialization OR remove the nosemgrep suppressions and re-triage.\n  %s",
			strings.Join(deserializeViolations, "\n  "))
	}
	if len(untrustedWrites) > 0 {
		t.Errorf("§11.4.135 VIOLATION: BuildScript written from a non-static source. "+
			"BuildScript must only ever be a static string literal or a copy from the static "+
			"providerDefinitions map (providerDef.BuildScript / definition.BuildScript).\n  %s",
			strings.Join(untrustedWrites, "\n  "))
	}
}

func fileLinef(file string, lineNo int, raw string) string {
	return filepath.Base(file) + ":" + itoa(lineNo) + ": " + strings.TrimSpace(raw)
}

// afterFirst returns the substring after the first occurrence of sep.
func afterFirst(s, sep string) string {
	if idx := strings.Index(s, sep); idx >= 0 {
		return s[idx+len(sep):]
	}
	return ""
}

// itoa is a tiny dependency-free int-to-string for the violation messages.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
