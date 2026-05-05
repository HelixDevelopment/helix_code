package tools

import (
	"os/exec"
)

// CuratedServerSpecs returns the curated allowlist of LSP servers.
//
// v1 ships exactly five specs, in this fixed order:
//
//  1. gopls (Go)
//  2. rust-analyzer (Rust)
//  3. pyright (Python)
//  4. typescript-language-server (TypeScript / JavaScript)
//  5. clangd (C / C++)
//
// Order is deterministic so that file-extension routing is stable across
// runs and across processes. The list is intentionally small — adding
// servers is a v2 concern, gated on a real user request, not a parking-lot
// item.
func CuratedServerSpecs() []LSPServerSpec {
	return []LSPServerSpec{
		{
			Name:           "gopls",
			Binary:         "gopls",
			Args:           []string{"serve"},
			FileExtensions: []string{".go"},
			LanguageID:     "go",
		},
		{
			Name:           "rust-analyzer",
			Binary:         "rust-analyzer",
			FileExtensions: []string{".rs"},
			LanguageID:     "rust",
		},
		{
			Name:           "pyright",
			Binary:         "pyright-langserver",
			Args:           []string{"--stdio"},
			FileExtensions: []string{".py", ".pyi"},
			LanguageID:     "python",
		},
		{
			Name:           "typescript-language-server",
			Binary:         "typescript-language-server",
			Args:           []string{"--stdio"},
			FileExtensions: []string{".ts", ".tsx", ".js", ".jsx"},
			LanguageID:     "typescript",
		},
		{
			Name:           "clangd",
			Binary:         "clangd",
			FileExtensions: []string{".c", ".cc", ".cpp", ".cxx", ".h", ".hh", ".hpp", ".hxx"},
			LanguageID:     "cpp",
		},
	}
}

// DetectAvailableServers filters a list of specs down to those whose
// Binary resolves on the current PATH via exec.LookPath. The function is
// pure: it never mutates input, never spawns a subprocess, and returns a
// fresh slice.
//
// Callers that want "what can we actually use right now" pass
// CuratedServerSpecs(). Callers that want to test PATH detection in
// isolation pass a tailored slice.
//
// A nil input yields a nil result.
func DetectAvailableServers(specs []LSPServerSpec) []LSPServerSpec {
	var available []LSPServerSpec
	for _, s := range specs {
		if _, err := exec.LookPath(s.Binary); err == nil {
			available = append(available, s)
		}
	}
	return available
}

// FindSpecForExtension returns the first spec in `specs` whose
// FileExtensions slice contains `ext`. Comparison is exact-string, so
// callers must include the leading dot (".go", not "go"). On a miss,
// returns the zero LSPServerSpec and false.
//
// This is the public, pure variant of the manager's internal
// specForExtension helper. The manager's helper does additional work
// (filepath.Ext + ToLower) because it accepts a full path; this helper is
// for callers that already have an extension in hand and want
// deterministic, side-effect-free routing.
func FindSpecForExtension(specs []LSPServerSpec, ext string) (LSPServerSpec, bool) {
	for _, s := range specs {
		for _, e := range s.FileExtensions {
			if e == ext {
				return s, true
			}
		}
	}
	return LSPServerSpec{}, false
}
