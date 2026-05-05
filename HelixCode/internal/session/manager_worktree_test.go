package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManager_GetCurrentWorktree_DefaultEmpty(t *testing.T) {
	m := NewManager()
	assert.Equal(t, "", m.GetCurrentWorktree())
}

func TestManager_SetCurrentWorktree_RoundTrip(t *testing.T) {
	m := NewManager()
	m.SetCurrentWorktree("/tmp/repo/.helix-worktrees/feature-x")
	assert.Equal(t, "/tmp/repo/.helix-worktrees/feature-x", m.GetCurrentWorktree())
}

func TestManager_SetCurrentWorktree_Empty(t *testing.T) {
	m := NewManager()
	m.SetCurrentWorktree("/tmp/repo/.helix-worktrees/feature-x")
	m.SetCurrentWorktree("")
	assert.Equal(t, "", m.GetCurrentWorktree())
}
