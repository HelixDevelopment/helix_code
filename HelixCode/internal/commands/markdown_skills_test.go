package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSkillFile_Valid(t *testing.T) {
	body := `---
description: Refactor a React component
triggers:
  - "(?i)^refactor (.+) component$"
  - "(?i)^extract hook from (.+)"
variables:
  default_style: functional
requires_isolation: false
---

You are refactoring {{ARG1}}.`
	skill, err := parseSkillFile("refactor-button", body, "/tmp/skill.md")
	require.NoError(t, err)
	assert.Equal(t, "refactor-button", skill.Name())
	assert.Equal(t, "Refactor a React component", skill.Description())
	assert.Len(t, skill.triggers, 2)
	assert.False(t, skill.RequiresIsolation())
	assert.Contains(t, skill.body, "You are refactoring")
}

func TestParseSkillFile_BadRegexSkipped(t *testing.T) {
	body := `---
description: bad regex skill
triggers:
  - "[unclosed"
---

body`
	skill, err := parseSkillFile("bad", body, "/tmp/bad.md")
	require.NoError(t, err)
	// Bad regex is skipped; skill loads with zero compiled triggers.
	assert.Empty(t, skill.triggers)
}

func TestSkill_Render_PositionalArgs(t *testing.T) {
	body := `---
description: x
triggers:
  - "^x$"
---

Got: {{ARG1}}`
	skill, err := parseSkillFile("x", body, "/tmp/x.md")
	require.NoError(t, err)
	out, err := skill.Render([]string{"LoginButton"}, "", "")
	require.NoError(t, err)
	assert.Equal(t, "Got: LoginButton", out)
}

func TestSkill_Render_NamedCapturesViaVariables(t *testing.T) {
	body := `---
description: x
triggers:
  - "(?P<component>[A-Z][A-Za-z0-9]+) refactor"
variables:
  default_style: functional
---

Component: {{ARG.component}}, Style: {{ARG.default_style}}`
	skill, err := parseSkillFile("x", body, "/tmp/x.md")
	require.NoError(t, err)
	captures := map[string]string{"component": "MyButton"}
	out, err := skill.RenderWithCaptures(nil, captures, "", "")
	require.NoError(t, err)
	assert.Contains(t, out, "Component: MyButton")
	assert.Contains(t, out, "Style: functional")
}

func TestSkillRegistry_FindMatching_FirstWins(t *testing.T) {
	reg := NewSkillRegistry()
	s1, _ := parseSkillFile("a", "---\ndescription: a\ntriggers:\n  - \"^foo\"\n---\nA", "")
	s2, _ := parseSkillFile("b", "---\ndescription: b\ntriggers:\n  - \"^foo\"\n---\nB", "")
	reg.Add(s1)
	reg.Add(s2)
	matched, _, ok := reg.FindMatching("foobar")
	require.True(t, ok)
	assert.Equal(t, "a", matched.Name())
}

func TestSkillRegistry_FindMatching_NamedCaptures(t *testing.T) {
	reg := NewSkillRegistry()
	s, _ := parseSkillFile("rc",
		"---\ndescription: rc\ntriggers:\n  - \"refactor (?P<comp>[A-Z][A-Za-z]+) component\"\n---\nbody {{ARG.comp}}", "")
	reg.Add(s)
	matched, captures, ok := reg.FindMatching("please refactor LoginButton component now")
	require.True(t, ok)
	assert.Equal(t, "rc", matched.Name())
	assert.Equal(t, "LoginButton", captures["comp"])
}

func TestSkillRegistry_AddRemove(t *testing.T) {
	reg := NewSkillRegistry()
	s, _ := parseSkillFile("x", "---\ndescription: x\ntriggers:\n  - \"^x\"\n---\nbody", "")
	reg.Add(s)
	_, ok := reg.Get("x")
	require.True(t, ok)
	reg.Remove("x")
	_, ok = reg.Get("x")
	assert.False(t, ok)
}
