package agent

import (
	"context"
	"fmt"

	"dev.helix.code/internal/commands"
)

// LoadSkillsAndDispatcher loads every `.md` skill found under the supplied
// directories into a fresh SkillRegistry and returns that registry together
// with a ready-to-use SkillDispatcher built on top of it.
//
// skillsDirs is an ordered list of directories scanned for `*.md` skill files.
// Later directories override earlier ones on name collision (project dir should
// come last). Non-existent directories are skipped silently; per-file parse
// errors are logged at WARN by the underlying SkillLoader and do NOT fail the
// load (a single broken skill file never blocks the front-end).
//
// A front-end (CLI or TUI) calls this ONCE at startup, then on each user prompt
// runs DispatchSkill against the returned dispatcher. The returned registry is
// retained by the caller for `/skills` introspection and hot-reload.
func LoadSkillsAndDispatcher(skillsDirs []string) (*commands.SkillRegistry, *SkillDispatcher, error) {
	reg := commands.NewSkillRegistry()

	// SkillLoader's two-slot (userDir, projectDir) contract overrides user with
	// project on collision. Generalise to an arbitrary ordered list by reloading
	// once per directory with the SAME registry: each Reload reconciles the
	// registry for that directory's files, and because each loader tracks only
	// the files IT loaded, later directories add/override without removing the
	// skills contributed by earlier directories.
	for _, dir := range skillsDirs {
		if dir == "" {
			continue
		}
		loader := commands.NewSkillLoader(reg, dir, "")
		if err := loader.Load(); err != nil {
			return nil, nil, fmt.Errorf("skill activation: load %q: %w", dir, err)
		}
	}

	disp := NewSkillDispatcher(reg, nil)
	return reg, disp, nil
}

// DispatchSkill runs the dispatcher's trigger matching against a raw user
// prompt and returns the rendered skill body when a trigger matches.
//
// On a match it returns (renderedBody, true): the matched skill's body with its
// default variables and the regex's named capture groups substituted. The
// front-end injects this rendered body into the prompt sent to the LLM (or runs
// it as a sub-prompt). On no match — or any render error — it returns ("", false)
// so the caller falls through to the ordinary prompt path unchanged.
//
// selection/currentFile are not part of this convenience signature; the
// dispatcher's Match is called with empty selection/currentFile. Callers needing
// editor context should call dispatcher.Match directly.
func DispatchSkill(dispatcher *SkillDispatcher, prompt string) (rendered string, matched bool) {
	if dispatcher == nil {
		return "", false
	}
	out, _, _, ok, err := dispatcher.Match(context.Background(), prompt, "", "")
	if err != nil || !ok {
		return "", false
	}
	return out, true
}
