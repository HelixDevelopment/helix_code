package worktree

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateName_Valid(t *testing.T) {
	m := NewManager("/tmp/repo")
	for _, name := range []string{"feature-x", "_pre-release", "v1.2.3-rc1", "a", "a.b.c"} {
		assert.NoError(t, m.ValidateName(name), "expected %q to be valid", name)
	}
}

func TestValidateName_Empty(t *testing.T) {
	m := NewManager("/tmp/repo")
	err := m.ValidateName("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestValidateName_TooLong(t *testing.T) {
	m := NewManager("/tmp/repo")
	tooLong := strings.Repeat("a", WorktreeNameMaxLength+1)
	err := m.ValidateName(tooLong)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestValidateName_AtLengthLimit(t *testing.T) {
	m := NewManager("/tmp/repo")
	exact := strings.Repeat("a", WorktreeNameMaxLength)
	assert.NoError(t, m.ValidateName(exact),
		"name of length WorktreeNameMaxLength (boundary) must be valid")
}

func TestValidateName_InvalidChars(t *testing.T) {
	m := NewManager("/tmp/repo")
	bad := []string{"/", "..", "../etc", "with space", "feature/x", "with\ttab", "with\nnewline", "tildé"}
	for _, name := range bad {
		err := m.ValidateName(name)
		assert.Error(t, err, "expected %q to be rejected", name)
	}
}

func TestGetCurrentDirectory_DefaultIsRepoRoot(t *testing.T) {
	m := NewManager("/tmp/repo")
	assert.Equal(t, "/tmp/repo", m.GetCurrentDirectory())
}

func TestIsIsolated_DefaultIsFalse(t *testing.T) {
	m := NewManager("/tmp/repo")
	assert.False(t, m.IsIsolated())
}
