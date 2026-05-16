package agent

import (
	"context"
	"fmt"

	"dev.helix.code/internal/commands"
)

// worktreeManager is the minimal interface SkillDispatcher needs from F04.
// Using an interface decouples this package from internal/tools/worktree's
// concrete type and avoids an import cycle if one ever arises.
type worktreeManager interface{}

// SkillDispatcher routes user input to skills via trigger matching.
// The caller (typically baseAgent) is responsible for using the returned
// (rendered, skill, captures) tuple — including routing to a worktree when
// skill.RequiresIsolation() is true.
type SkillDispatcher struct {
	registry *commands.SkillRegistry
	wtMgr    worktreeManager // optional; nil when isolation is unsupported
}

// NewSkillDispatcher constructs a dispatcher. wtMgr may be nil; the
// dispatcher does NOT route to it directly. Callers inspect
// skill.RequiresIsolation() and the returned rendered body and decide.
func NewSkillDispatcher(reg *commands.SkillRegistry, wtMgr worktreeManager) *SkillDispatcher {
	return &SkillDispatcher{registry: reg, wtMgr: wtMgr}
}

// Match looks up a skill matching the user input. On match it returns the
// rendered body (variables + named captures substituted), the matched skill,
// and the named capture map. On no match it returns empty rendered, nil skill,
// nil captures, ok=false.
func (d *SkillDispatcher) Match(
	_ context.Context,
	input, selection, currentFile string,
) (rendered string, matched *commands.Skill, captures map[string]string, ok bool, err error) {
	if d.registry == nil {
		return "", nil, nil, false, nil
	}
	skill, caps, found := d.registry.FindMatching(input)
	if !found {
		return "", nil, nil, false, nil
	}
	out, rerr := skill.RenderWithCaptures(nil, caps, selection, currentFile)
	if rerr != nil {
		return "", skill, caps, false, fmt.Errorf("skill %s render: %w", skill.Name(), rerr)
	}
	return out, skill, caps, true, nil
}
