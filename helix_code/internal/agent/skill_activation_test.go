package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoSkillsDir resolves the real `.helix/skills` directory shipped in the repo
// (two levels up from internal/agent). The sample skill explain-arch.md lives
// there so the activation path is demonstrated against a REAL on-disk skill,
// not a synthetic in-memory fixture.
func repoSkillsDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", ".helix", "skills")
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	if _, err := os.Stat(filepath.Join(abs, "explain-arch.md")); err != nil {
		t.Fatalf("sample skill not found at %s: %v", abs, err)
	}
	return abs
}

func TestLoadSkillsAndDispatcher_RealSampleSkillMatchesAndRenders(t *testing.T) {
	reg, disp, err := LoadSkillsAndDispatcher([]string{repoSkillsDir(t)})
	require.NoError(t, err)
	require.NotNil(t, reg)
	require.NotNil(t, disp)

	// The real sample skill is registered under its filename stem.
	s, ok := reg.Get("explain-arch")
	require.True(t, ok, "explain-arch skill loaded from .helix/skills")
	assert.NotEmpty(t, s.Description())

	// DispatchSkill runs the regex trigger and renders the body with the named
	// capture group topic=ensemble substituted.
	rendered, matched := DispatchSkill(disp, "explain ensemble architecture")
	require.True(t, matched, "the sample skill's trigger matched the prompt")
	assert.True(t, strings.Contains(rendered, "Architecture: ensemble"),
		"rendered heading carries the captured topic=ensemble (got: %q)", rendered)
	// The frontmatter default variable audience=engineer is substituted too.
	assert.True(t, strings.Contains(rendered, "engineer audience"),
		"rendered body substituted the default audience variable")
	// No raw {{ARG.topic}} placeholder must remain after rendering.
	assert.False(t, strings.Contains(rendered, "{{ARG.topic}}"),
		"no unrendered {{ARG.topic}} placeholder remains")
}

func TestDispatchSkill_NoMatchFallsThrough(t *testing.T) {
	_, disp, err := LoadSkillsAndDispatcher([]string{repoSkillsDir(t)})
	require.NoError(t, err)

	rendered, matched := DispatchSkill(disp, "what is the weather today")
	assert.False(t, matched, "non-triggering prompt does not match any skill")
	assert.Empty(t, rendered)
}

func TestDispatchSkill_NilDispatcherSafe(t *testing.T) {
	rendered, matched := DispatchSkill(nil, "explain ensemble architecture")
	assert.False(t, matched)
	assert.Empty(t, rendered)
}

func TestLoadSkillsAndDispatcher_EmptyAndMissingDirsAreSafe(t *testing.T) {
	reg, disp, err := LoadSkillsAndDispatcher([]string{"", filepath.Join(t.TempDir(), "does-not-exist")})
	require.NoError(t, err)
	require.NotNil(t, reg)
	require.NotNil(t, disp)
	// Built-in (bundled, //go:embed) skills are always present regardless of the
	// on-disk dirs. Empty/missing dirs must therefore contribute NO on-disk skill:
	// every skill in the registry must be a built-in (SourcePath "builtin:...").
	// This still FAILs if an on-disk skill ever leaked from an empty/missing dir
	// (the original invariant this test guards), so it is not a tautology.
	for _, s := range reg.List() {
		assert.True(t, strings.HasPrefix(s.SourcePath(), "builtin:"),
			"empty/missing dirs must load no on-disk skill; got %q from %q", s.Name(), s.SourcePath())
	}
}
