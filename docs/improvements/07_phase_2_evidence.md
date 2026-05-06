# Phase 2 — CLI Agent Porting — Runtime Evidence

**Date opened:** 2026-05-06

Each feature's acceptance check output is pasted below with a timestamp.
This file is the rolled-up forensic record per Article XI §11.9.

Phase 1's evidence lives in `06_phase_1_evidence.md` (with Phase 1.5
Foundation Cleanup inlined as `§P1.5`). Phase 2 deserves its own file
because the scope changes — Phase 1 was a single source agent
(`claude-code`); Phase 2 ports features across multiple non-claude-code
CLI agents (codex, aider, cline, plandex, opencode, kiro, kilo-code,
roo-code, openhands, …) following the order described in the synthesis
design §4.2.

Spec: `docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md` §4.2
Phase status pointer: `docs/improvements/PROGRESS.md`

---

## P2-F21 — Codex Approval Modes

**Date opened:** 2026-05-06
**Spec:** `7128289`
**Plan:** `bbb61de`
**Status:** in progress

### One-line goal

Codex-compatible 4-mode approval system (suggest / auto-edit / full-auto /
dangerously-bypass) with CLI flag > env > config > default precedence;
per-tool `RequiresApproval()` gate; F14 sandbox coupling for full-auto;
F02 final-deny authority retained; `/approval` slash + atomic-pointer
runtime mode swap.

### Commits in order

| Task | Commit | Subject |
|---|---|---|
| P2-F21-T01 |  | bootstrap Phase 2 evidence + advance PROGRESS to F21 |
| P2-F21-T02 |  | approval/types.go: ApprovalMode + ApprovalLevel + Decision + sentinels + ModeDescriptors (TDD) |
| P2-F21-T03 |  | approval/selector.go: flag > env > config > default precedence (TDD) |
| P2-F21-T04 |  | approval/manager.go: ApprovalManager with 4×4 matrix gate + F02/F14/F19 integration (TDD) |
| P2-F21-T05 |  | Extend Tool interface with RequiresApproval() + DefaultLevelEdit + apply to ~30 existing tools (TDD) |
| P2-F21-T06 |  | /approval slash command (status/set/show) (TDD) |
| P2-F21-T07 |  | main.go wiring + --approval pflag + registry hook + integration test (TDD) |
| P2-F21-T08 |  | Challenge harness 5 phases (suggest-deny + auto-edit-prompt + full-auto-sandbox + runtime-change + F02-final-deny) |
| P2-F21-T09 |  | Feature 21 close-out + push 4 remotes non-force |

### Acceptance

_to be filled in as tasks land — every claim of "PASS" must include
pasted runtime evidence per Article XI §11.9._

### P2-F21-T01 — bootstrap Phase 2 evidence + advance PROGRESS

_filled in by the close-out commit of T01 itself._

### P2-F21-T02 — approval/types.go (TDD)

_pending._

### P2-F21-T03 — approval/selector.go (TDD)

_pending._

### P2-F21-T04 — approval/manager.go (TDD)

_pending._

### P2-F21-T05 — Tool interface extension (TDD)

_pending._

### P2-F21-T06 — /approval slash command (TDD)

_pending._

### P2-F21-T07 — main.go wiring + --approval pflag (TDD)

_pending._

### P2-F21-T08 — Challenge harness 5 phases

_pending._

### P2-F21-T09 — Feature 21 close-out + push 4 remotes

_pending._

---
