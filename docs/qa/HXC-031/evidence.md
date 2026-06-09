# HXC-031 — Deferred long-tail: CONST-052 renames + Codex/Cline reference-agent ports
**Captured:** 2026-06-09T15:23:48Z · Task · Completed (→ Fixed.md)
## Sub-part 1: CONST-052 renames — RESOLVED
D-6 discovery sweep enumerated all 63 Go submodules for capitalised go.mod replace paths.
All remaining CONST-052 build-breaks were fixed + closed this session:
HXC-052 background_tasks, HXC-053 conversation, HXC-051 helix_llm+helix_memory,
HXC-056 (auto_temp/claritas/gandalf_solutions/hyper_tune/leak_hub/ouroborous/veritas → ../plinius_common),
HXC-057 recovery. No capitalised-replace go.mod breaks remain (enumerated coverage §11.4.118).
## Sub-part 2: Codex + Cline reference-agent ports — PRESENT
Both incorporated as reference CLI-agent submodules under cli_agents/ (alongside aider/plandex/etc):
- cli_agents/codex  → submodule openai/codex (.gitmodules), 4803 files / 55M, tracked
- cli_agents/cline  → submodule cline/cline (.gitmodules), 3486 files / 72M, tracked
`git config -f .gitmodules` confirms both path+url entries; `git ls-files` confirms both tracked.
## Determination
Both sub-parts complete with captured evidence; Task closeable.
