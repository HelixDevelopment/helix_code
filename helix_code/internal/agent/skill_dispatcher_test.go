package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/commands"
)

func makeSkill(t *testing.T, name, body string) *commands.Skill {
	t.Helper()
	s, err := commands.ParseSkillForTest(name, body, "/tmp/"+name+".md")
	require.NoError(t, err)
	return s
}

func TestSkillDispatcher_RegistryNil_ReturnsNoMatch(t *testing.T) {
	d := NewSkillDispatcher(nil, nil)
	rendered, skill, caps, ok, err := d.Match(context.Background(), "anything", "", "")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, rendered)
	assert.Nil(t, skill)
	assert.Nil(t, caps)
}

func TestSkillDispatcher_NoMatch(t *testing.T) {
	reg := commands.NewSkillRegistry()
	reg.Add(makeSkill(t, "x",
		`---
description: x
triggers:
  - "^xyz$"
---

body`))
	d := NewSkillDispatcher(reg, nil)
	_, _, _, ok, err := d.Match(context.Background(), "totally unrelated", "", "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestSkillDispatcher_Match_Injects(t *testing.T) {
	reg := commands.NewSkillRegistry()
	reg.Add(makeSkill(t, "rc",
		`---
description: refactor
triggers:
  - "refactor (?P<comp>[A-Z][A-Za-z]+) component"
---

Refactoring {{ARG.comp}}.`))
	d := NewSkillDispatcher(reg, nil)
	rendered, skill, caps, ok, err := d.Match(context.Background(), "please refactor LoginButton component", "", "")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "rc", skill.Name())
	assert.Equal(t, "LoginButton", caps["comp"])
	assert.Contains(t, rendered, "Refactoring LoginButton")
}

func TestSkillDispatcher_RequiresIsolation_FlaggedInResult(t *testing.T) {
	reg := commands.NewSkillRegistry()
	reg.Add(makeSkill(t, "iso",
		`---
description: isolated
triggers:
  - "^iso$"
requires_isolation: true
---

isolated body`))
	d := NewSkillDispatcher(reg, nil)
	_, skill, _, ok, err := d.Match(context.Background(), "iso", "", "")
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, skill.RequiresIsolation())
}
