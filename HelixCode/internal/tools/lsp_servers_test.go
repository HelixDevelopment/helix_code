package tools_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools"
)

// TestCuratedServerSpecs_HasFiveEntries asserts the curated list contains
// exactly the five v1 entries with the documented names. If anyone adds or
// removes a server in CuratedServerSpecs, this test forces them to update
// the documentation and v1 contract first.
func TestCuratedServerSpecs_HasFiveEntries(t *testing.T) {
	specs := tools.CuratedServerSpecs()
	require.Len(t, specs, 5, "curated allowlist must contain exactly 5 entries in v1")

	got := make([]string, 0, len(specs))
	for _, s := range specs {
		got = append(got, s.Name)
	}
	want := []string{
		"gopls",
		"rust-analyzer",
		"pyright",
		"typescript-language-server",
		"clangd",
	}
	assert.Equal(t, want, got, "curated server names + order must be stable")
}

// TestCuratedServerSpecs_NoEmptyFields ensures every spec has the minimum
// fields a manager needs to spawn it: Name, Binary, LanguageID, plus at
// least one file extension that begins with a dot.
func TestCuratedServerSpecs_NoEmptyFields(t *testing.T) {
	for _, s := range tools.CuratedServerSpecs() {
		s := s
		t.Run(s.Name, func(t *testing.T) {
			assert.NotEmpty(t, s.Name, "Name must be non-empty")
			assert.NotEmpty(t, s.Binary, "Binary must be non-empty")
			assert.NotEmpty(t, s.LanguageID, "LanguageID must be non-empty")
			require.NotEmpty(t, s.FileExtensions, "FileExtensions must have at least one entry")
			for _, ext := range s.FileExtensions {
				assert.True(t, strings.HasPrefix(ext, "."),
					"extension %q on %s must start with a dot", ext, s.Name)
			}
		})
	}
}

// TestCuratedServerSpecs_NoExtensionCollisions asserts that no extension is
// claimed by two different servers in v1. Within-spec listing is fine
// (e.g. pyright owns both .py and .pyi), but across specs the routing must
// be unambiguous.
func TestCuratedServerSpecs_NoExtensionCollisions(t *testing.T) {
	owner := map[string]string{}
	for _, s := range tools.CuratedServerSpecs() {
		for _, ext := range s.FileExtensions {
			if prev, exists := owner[ext]; exists {
				t.Errorf("extension %q claimed by both %q and %q", ext, prev, s.Name)
				continue
			}
			owner[ext] = s.Name
		}
	}
}

// TestDetectAvailableServers_FiltersByLookPath stages a fake gopls
// executable in a tempdir, prepends it to PATH, and asserts only gopls
// survives the filter. The other curated servers are NOT faked, so they
// must be excluded from the result. PATH is restored on test exit.
func TestDetectAvailableServers_FiltersByLookPath(t *testing.T) {
	tempDir := t.TempDir()
	fakePath := filepath.Join(tempDir, "gopls")
	require.NoError(t, os.WriteFile(fakePath, []byte("#!/bin/sh\necho fake\n"), 0o755))

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	require.NoError(t, os.Setenv("PATH", tempDir))

	available := tools.DetectAvailableServers(tools.CuratedServerSpecs())

	names := make([]string, 0, len(available))
	for _, s := range available {
		names = append(names, s.Name)
	}
	assert.Contains(t, names, "gopls", "gopls fake on PATH must be detected")
	for _, other := range []string{
		"rust-analyzer",
		"pyright",
		"typescript-language-server",
		"clangd",
	} {
		assert.NotContains(t, names, other,
			"%s must not be detected when only gopls is faked", other)
	}
}

// TestDetectAvailableServers_EmptyWhenNothingOnPath confirms that with a
// fully bogus PATH the detector returns an empty slice, not a panic and
// not the input list.
func TestDetectAvailableServers_EmptyWhenNothingOnPath(t *testing.T) {
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	require.NoError(t, os.Setenv("PATH", "/nonexistent-helixcode-lsp-test-path"))

	available := tools.DetectAvailableServers(tools.CuratedServerSpecs())
	assert.Empty(t, available, "no binaries on PATH ⇒ no specs returned")
}

// TestFindSpecForExtension_KnownExtensions runs every documented v1
// extension through the resolver and asserts the right server name is
// returned.
func TestFindSpecForExtension_KnownExtensions(t *testing.T) {
	specs := tools.CuratedServerSpecs()
	cases := []struct {
		ext  string
		want string
	}{
		{".go", "gopls"},
		{".rs", "rust-analyzer"},
		{".py", "pyright"},
		{".pyi", "pyright"},
		{".ts", "typescript-language-server"},
		{".tsx", "typescript-language-server"},
		{".js", "typescript-language-server"},
		{".jsx", "typescript-language-server"},
		{".c", "clangd"},
		{".cc", "clangd"},
		{".cpp", "clangd"},
		{".cxx", "clangd"},
		{".h", "clangd"},
		{".hh", "clangd"},
		{".hpp", "clangd"},
		{".hxx", "clangd"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.ext, func(t *testing.T) {
			got, ok := tools.FindSpecForExtension(specs, tc.ext)
			require.True(t, ok, "extension %q should resolve", tc.ext)
			assert.Equal(t, tc.want, got.Name)
		})
	}
}

// TestFindSpecForExtension_UnknownExtension verifies the (zero, false)
// contract for an extension nobody owns.
func TestFindSpecForExtension_UnknownExtension(t *testing.T) {
	got, ok := tools.FindSpecForExtension(tools.CuratedServerSpecs(), ".unknown")
	assert.False(t, ok)
	assert.Empty(t, got.Name)
}

// TestFindSpecForExtension_DotIsRequired guards the documented contract
// that callers must pass a leading dot. Passing "go" instead of ".go"
// must miss, since FindSpecForExtension does an exact-string match, not
// a suffix match.
func TestFindSpecForExtension_DotIsRequired(t *testing.T) {
	got, ok := tools.FindSpecForExtension(tools.CuratedServerSpecs(), "go")
	assert.False(t, ok, "extension lookup without leading dot must fail")
	assert.Empty(t, got.Name)
}
