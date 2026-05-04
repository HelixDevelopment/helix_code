# HelixCode CLI-Agent Fusion — Live Progress Tracker

> **STOP/RESUME PROTOCOL**: read this file first. The "current focus" pointer
> below identifies the active task. The "evidence trail" links every claim of
> "done" to its commit + Challenge output.
>
> Spec: `docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md`
> Plan: `docs/superpowers/plans/2026-05-04-phase-0-foundation-cleanup.md`

## Current focus
- **Active phase:** P0 — Foundation Cleanup
- **Active task:** P0-03 — add HelixAgent submodule
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
- [x] P0-01 — bootstrap PROGRESS.md  ← commit 2c07f57
- [x] P0-02 — resolve Agent-Deck nested-worktree recursion error  ← commit (this commit)
- [ ] P0-03 — add HelixAgent submodule
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
