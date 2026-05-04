# HelixCode CLI-Agent Fusion — Live Progress Tracker

> **STOP/RESUME PROTOCOL**: read this file first. The "current focus" pointer
> below identifies the active task. The "evidence trail" links every claim of
> "done" to its commit + Challenge output.
>
> Spec: `docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md`
> Plan: `docs/superpowers/plans/2026-05-04-phase-0-foundation-cleanup.md`

## Current focus
- **Active phase:** P0 — Foundation Cleanup
- **Active task:** P0-04 — verify-llmsverifier-pin-parity.sh
- **Owner:** agent (Claude Opus 4.7)
- **Started:** 2026-05-04
- **Last touched:** 2026-05-04
- **Blocked-on:** none

## Phase status
| Phase | State | Started | Completed | Evidence |
|---|---|---|---|---|
| P0 — Foundation | active | 2026-05-04 | — | docs/improvements/05_phase_0_evidence.md |
| P1 — claude-code | pending | — | — | — |
| P2 — Other CLI agents | pending | — | — | — |
| P3 — Test infra | pending | — | — | — |
| P4 — Anti-bluff audit | pending | — | — | — |
| P5 — End-user materials | pending | — | — | — |

## Active phase task list (Phase 0)
- [x] P0-01 — bootstrap PROGRESS.md  ← commit `2c07f57`
- [~] P0-02 — Agent-Deck nested-worktree recursion error: **DEFERRED** (cosmetic; safe-fix requires modifying third-party submodules which is out of scope per spec §2.1; original `.git/info/exclude` approach was based on incorrect understanding of git submodule recursion semantics; see parking lot). Reverts: commits `904c925` + `a82f1a9`.
- [x] P0-03 — add HelixAgent submodule  ← (this commit) — 47/60 cli_agents populated; 13 deferred to Phase 2 sub-specs (see parking lot)
- [-] P0-04 — verify-llmsverifier-pin-parity.sh
- [ ] P0-04 — verify-llmsverifier-pin-parity.sh
- [ ] P0-05 — migrate API keys from ../HelixAgent/.env
- [ ] P0-06 — update .gitignore (root + inner)
- [ ] P0-07 — refresh HelixCode/HelixCode/.env.example
- [ ] P0-08 — scan-secrets.sh + planted-secret test
- [ ] P0-09 — pre-push hook + installer + setup.sh wiring
- [ ] P0-10 — create HelixCode/HelixCode/{CLAUDE,AGENTS,CONSTITUTION}.md
- [ ] P0-11 — add Article XII (CONST-041, CONST-042) to root CONSTITUTION.md
- [ ] P0-12 — cascade CONST-041/042 + anti-bluff anchor to root sister files (CLAUDE, AGENTS, CRUSH, QWEN)
- [ ] P0-13 — fix root CLAUDE.md §3.2 bluff (HelixCode tracked-dir vs. submodule)
- [ ] P0-14 — extend verify-governance-cascade.sh + run propagate-governance.sh + verify cascade
- [ ] P0-15 — Makefile verify-foundation target + extend ci-validate-all
- [ ] P0-16 — regenerate diagrams + DEPRECATED.md pointers + Phase 0 evidence + push close-out

## Decision log
- 2026-05-04 — Approach A (HelixAgent as integration substrate) — user-approved during brainstorming — see synthesis spec §2.1
- 2026-05-04 — Non-force pushes pre-authorised for the duration of this programme — user statement during brainstorming — see synthesis spec §7.3
- 2026-05-04 — claude-code-source is Phase 1 priority #1 — user statement — see synthesis spec §4.1

## Open risks / parking lot
- HelixAgent submodule clone size — may need `--depth=1` shallow if >500 MB; measured at P0-03
- Codex agent disambiguation (closed vs. open variant) — deferred to Phase 2 sub-spec
- Example_Projects/ deletion — deferred to post-Phase-4 decision
- **Submodule recursion cosmetic error (deferred from P0-02):** `git submodule foreach --recursive` errors out on `Example_Projects/{Agent-Deck,Bridle,Claude-Code-Plugins-And-Skills}` because each of those third-party repos has registered nested gitlinks (mode 160000) without corresponding `.gitmodules` entries. The original Task 2 plan proposed `.git/info/exclude` — that does NOT fix recursion (which walks the index, not the working tree). The only safe in-scope fix is to wrap script calls with `|| true` and tolerate the error. Modifying the affected third-party submodules' contents is forbidden by spec §2.1 (third-party not modified). Decision: scripts that use `git submodule foreach --recursive` (none yet in our codebase) must wrap with `|| true`; documentation updates that erroneously claimed Task 2 would resolve this are corrected.
- **HelixAgent stale cli_agents pins (discovered during P0-03):** 13 of 60 cli_agents under `HelixAgent/cli_agents/` cannot be initialized because HelixAgent's recorded submodule SHAs no longer exist on the corresponding upstream remotes. Affected: `aider, conduit, continue, HelixCode, kilo-code, kiro-cli, mobile-agent, ollama-code, opencode-cli, openhands, plandex, roo-code, superset`. Each Phase 2 sub-spec for the affected agent must first bump HelixAgent's pointer (commit IN HelixAgent itself, then bump HelixAgent's pointer in this meta-repo) to a SHA that exists upstream. Phase 1 priority `claude-code` is NOT affected — fully populated. Per spec §1.3 N2, HelixAgent rewrite is out of scope for this programme; the per-agent pin bumps go through HelixAgent's own governance.
