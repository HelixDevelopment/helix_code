package commands

import (
	"os"
	"path/filepath"
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

func TestSkillLoader_LoadProjectAndUser(t *testing.T) {
	projectDir := t.TempDir()
	userDir := t.TempDir()
	projDir := filepath.Join(projectDir, ".helix", "skills")
	usrDir := filepath.Join(userDir, ".config", "helixcode", "skills")
	require.NoError(t, os.MkdirAll(projDir, 0755))
	require.NoError(t, os.MkdirAll(usrDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(usrDir, "shared.md"),
		[]byte("---\ndescription: from user\ntriggers: [\"^x\"]\n---\nuser body"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "shared.md"),
		[]byte("---\ndescription: from project\ntriggers: [\"^x\"]\n---\nproject body"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(usrDir, "user-only.md"),
		[]byte("---\ndescription: u\ntriggers: [\"^u\"]\n---\nuser only body"), 0644))

	reg := NewSkillRegistry()
	loader := NewSkillLoader(reg, projDir, usrDir)
	require.NoError(t, loader.Load())

	// "shared" should reflect project version (project overrides user)
	s, ok := reg.Get("shared")
	require.True(t, ok)
	assert.Equal(t, "from project", s.Description())

	// "user-only" still loaded
	_, ok = reg.Get("user-only")
	assert.True(t, ok)
}

func TestSkillLoader_ReloadDiff_AddsRemovesUpdates(t *testing.T) {
	projectDir := t.TempDir()
	skills := filepath.Join(projectDir, ".helix", "skills")
	require.NoError(t, os.MkdirAll(skills, 0755))

	reg := NewSkillRegistry()
	loader := NewSkillLoader(reg, skills, "")
	require.NoError(t, loader.Load())
	_, ok := reg.Get("a")
	assert.False(t, ok)

	require.NoError(t, os.WriteFile(filepath.Join(skills, "a.md"),
		[]byte("---\ndescription: a\ntriggers: [\"^a\"]\n---\nbody a"), 0644))
	require.NoError(t, loader.Reload())
	_, ok = reg.Get("a")
	assert.True(t, ok)

	require.NoError(t, os.Remove(filepath.Join(skills, "a.md")))
	require.NoError(t, loader.Reload())
	_, ok = reg.Get("a")
	assert.False(t, ok)
}

func TestSkillLoader_BadFrontmatterIsLoggedNotFatal(t *testing.T) {
	projectDir := t.TempDir()
	skills := filepath.Join(projectDir, ".helix", "skills")
	require.NoError(t, os.MkdirAll(skills, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skills, "good.md"),
		[]byte("---\ndescription: g\ntriggers: [\"^g\"]\n---\ngood body"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skills, "bad.md"),
		[]byte("---\ntitle: oops\n(no closing fence)"), 0644))

	reg := NewSkillRegistry()
	loader := NewSkillLoader(reg, skills, "")
	require.NoError(t, loader.Load())
	_, ok := reg.Get("good")
	assert.True(t, ok)
	_, ok = reg.Get("bad")
	assert.False(t, ok, "bad.md must be skipped")
}

func TestSkillLoader_NonExistentDirsAreSkipped(t *testing.T) {
	reg := NewSkillRegistry()
	loader := NewSkillLoader(reg, "/tmp/p1f10-skills-does-not-exist", "/tmp/p1f10-skills-also-not-exist")
	require.NoError(t, loader.Load())
}

// TestSkillLoader_SkillMdManifestRecognised verifies the packaged
// "<name>/SKILL.md" manifest form (T1.6) is loaded with the skill name taken
// from the containing directory, while a stray flat "SKILL.md" in the skills
// dir root is NOT registered under the bogus name "SKILL".
func TestSkillLoader_SkillMdManifestRecognised(t *testing.T) {
	root := t.TempDir()
	skills := filepath.Join(root, ".helix", "skills")
	pkgDir := filepath.Join(skills, "deploy")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))

	// Packaged manifest: <skills>/deploy/SKILL.md -> skill name "deploy".
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "SKILL.md"),
		[]byte("---\ndescription: packaged deploy\ntriggers: [\"^deploy\"]\n---\ndeploy body"), 0644))
	// A flat SKILL.md at the skills-dir root must be ignored (no name of its own).
	require.NoError(t, os.WriteFile(filepath.Join(skills, "SKILL.md"),
		[]byte("---\ndescription: stray\ntriggers: [\"^stray\"]\n---\nstray body"), 0644))

	reg := NewSkillRegistry()
	loader := NewSkillLoader(reg, skills, "")
	require.NoError(t, loader.Load())

	s, ok := reg.Get("deploy")
	require.True(t, ok, "packaged deploy/SKILL.md must register as skill \"deploy\"")
	assert.Equal(t, "packaged deploy", s.Description())
	assert.Contains(t, s.SourcePath(), filepath.Join("deploy", "SKILL.md"))

	_, ok = reg.Get("SKILL")
	assert.False(t, ok, "a flat SKILL.md must not register under the name \"SKILL\"")
}

// TestSkillLoader_ManifestOverridesFlatInSameDir verifies that within a single
// directory the packaged "<name>/SKILL.md" manifest takes precedence over a
// legacy flat "<name>.md" of the same skill name.
func TestSkillLoader_ManifestOverridesFlatInSameDir(t *testing.T) {
	root := t.TempDir()
	skills := filepath.Join(root, ".helix", "skills")
	pkgDir := filepath.Join(skills, "review")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))

	// Flat legacy form.
	require.NoError(t, os.WriteFile(filepath.Join(skills, "review.md"),
		[]byte("---\ndescription: flat review\ntriggers: [\"^review\"]\n---\nflat body"), 0644))
	// Packaged manifest form (should win).
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "SKILL.md"),
		[]byte("---\ndescription: manifest review\ntriggers: [\"^review\"]\n---\nmanifest body"), 0644))

	reg := NewSkillRegistry()
	loader := NewSkillLoader(reg, skills, "")
	require.NoError(t, loader.Load())

	s, ok := reg.Get("review")
	require.True(t, ok)
	assert.Equal(t, "manifest review", s.Description(),
		"packaged SKILL.md manifest must override flat review.md in the same dir")
}

// TestSkillLoader_ThreeWayPrecedence asserts the full T1.6 precedence chain for
// a single skill name across user + project dirs and the two on-disk forms:
// project SKILL.md  >  project flat  >  user SKILL.md  >  user flat.
func TestSkillLoader_ThreeWayPrecedence(t *testing.T) {
	type variant struct {
		dirRoot string // the skills dir passed to the loader
		pkg     bool   // packaged (<name>/SKILL.md) vs flat (<name>.md)
		desc    string
	}
	mkSkill := func(t *testing.T, v variant, name string) {
		t.Helper()
		body := []byte("---\ndescription: " + v.desc + "\ntriggers: [\"^p\"]\n---\nbody")
		if v.pkg {
			d := filepath.Join(v.dirRoot, name)
			require.NoError(t, os.MkdirAll(d, 0755))
			require.NoError(t, os.WriteFile(filepath.Join(d, "SKILL.md"), body, 0644))
			return
		}
		require.NoError(t, os.MkdirAll(v.dirRoot, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(v.dirRoot, name+".md"), body, 0644))
	}

	root := t.TempDir()
	projSkills := filepath.Join(root, "project", ".helix", "skills")
	userSkills := filepath.Join(root, "user", ".config", "helixcode", "skills")

	// Seed all four sources for the same skill name "p".
	mkSkill(t, variant{userSkills, false, "user-flat"}, "p")
	mkSkill(t, variant{userSkills, true, "user-manifest"}, "p")
	mkSkill(t, variant{projSkills, false, "project-flat"}, "p")
	mkSkill(t, variant{projSkills, true, "project-manifest"}, "p")

	reg := NewSkillRegistry()
	loader := NewSkillLoader(reg, projSkills, userSkills)
	require.NoError(t, loader.Load())

	s, ok := reg.Get("p")
	require.True(t, ok)
	// project dir processed last; within it the manifest pass runs last.
	assert.Equal(t, "project-manifest", s.Description(),
		"precedence must be project-SKILL.md > project-flat > user-SKILL.md > user-flat")
	assert.Contains(t, s.SourcePath(), filepath.Join("project", ".helix", "skills", "p", "SKILL.md"))
}

func TestSkillLoader_LoadedReturnsSnapshot(t *testing.T) {
	projectDir := t.TempDir()
	skills := filepath.Join(projectDir, ".helix", "skills")
	require.NoError(t, os.MkdirAll(skills, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skills, "x.md"),
		[]byte("---\ndescription: x\ntriggers: [\"^x\"]\n---\nbody"), 0644))

	reg := NewSkillRegistry()
	loader := NewSkillLoader(reg, skills, "")
	require.NoError(t, loader.Load())

	loaded := loader.Loaded()
	require.Len(t, loaded, 1)
	assert.Contains(t, loaded["x"], "x.md")
}
