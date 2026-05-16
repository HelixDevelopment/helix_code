// Package worktree implements claude-code-style git worktree agent isolation.
//
// Agents (and humans) enter a named, validated worktree at
// <repoRoot>/.helix-worktrees/<name>/ via Manager.EnterWorktree, work in a
// parallel branch without polluting main, and exit via Manager.ExitWorktree.
// All worktree operations shell out to the git binary, consistent with
// internal/tools/git/. Submodules are NOT initialised — the meta-repo and
// the inner Go module at HelixCode/ are present, but submodule directories
// under helix_agent/, Dependencies/, etc. are uninitialised (empty directories).
//
// See: docs/superpowers/specs/2026-05-05-p1-f04-git-worktree-agent-isolation-design.md
package worktree
