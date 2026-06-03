# Phase 0 — Governance Baseline & Safety Net (Design / Spec)

**Date:** 2026-06-03
**Programme:** Own-org submodule sync → flatten → rebuild → test → fix (HelixCode CLI-Agent Fusion)
**Status:** Approved (operator: 2026-06-03) — execution authorized, subagent-driven, autonomous loop.

## Context

The operator requested a full programme: sync every own-org submodule to latest
main/master + merge local work, flatten all `vasic-digital` / `HelixDevelopment`
submodules to a flat `HelixCode/submodules/<snake_case>` layout, rewrite all
references, run `install_upstreams` everywhere, clean git configs, rebuild
everything via the `containers` submodule, boot infra, run all tests +
Challenges in anti-bluff mode, and fix every issue found. The work is decomposed
into sequenced phases; this spec covers **Phase 0** only.

### Discovered state (read-only probe, 2026-06-03)
- `.gitmodules` declares **70 own-org submodules** (of 131 top-level); ~64 are
  nested under `dependencies/{vasic-digital,HelixDevelopment}/…`.
- HelixConstitution submodule **is wired & checked out** at `d90ab87` on `main`,
  up to date with its tracking upstream; working tree clean.
- The anti-bluff mandate ("…most of the features does not work… MUST guarantee
  quality, completion, full usability…") is **already present** in root
  `CONSTITUTION.md`, `CLAUDE.md`, `AGENTS.md`, `QWEN.md` **and** in the
  HelixConstitution submodule's own files. No insertion required — verify only.
- Four submodules carry forbidden GitFlic/GitVerse push remotes:
  `constitution`, `dependencies/HelixDevelopment/models`,
  `dependencies/vasic-digital/models` (own-org), and `cli_agents/claude-code-source`
  (third-party, not pushed).
- Operator decision: **push to ALL upstreams including GitFlic + GitVerse.** This
  contradicts CONST-038 as written → reconcile the rule (item 2).

## Goal

A verified, self-consistent governance source-of-truth and a reversible safety
baseline, established **before** any 70-repo mutation phase.

## Scope — IN

1. **Governance verification (no insertion).** Run
   `scripts/verify-governance-cascade.sh`; capture output as evidence under
   `docs/migration/`. Confirm the anti-bluff mandate is present across root +
   constitution submodule + owned submodules. If an owned submodule is missing
   it, add it there (and cascade per CONST-047).
2. **Reconcile CONST-038 to operator remote policy.** Edit HelixCode root
   `CONSTITUTION.md`, `CLAUDE.md`, `AGENTS.md`, `QWEN.md`, `CRUSH.md` so the
   CONST-038 / §6.W note permits GitHub + GitLab + GitFlic + GitVerse for the
   affected own-org repos, removing the doc-vs-practice contradiction. Keep all
   five peer governance docs in sync (root "keep in sync" rule).
3. **Safety backup (reversibility).** Generate a restore manifest capturing every
   submodule's current commit SHA + full remote set, plus a snapshot of
   `.gitmodules`, under
   `docs/migration/phase0-restore-manifest-2026-06-03.txt`. Validate it
   round-trips (re-reading reproduces current SHAs).

## Scope — OUT (later phases)
- Submodule sync-to-latest + merge (Phase 1)
- Flatten layout + rewrite references (Phase 2)
- Rebuild everything (Phase 3)
- Boot infra + run all tests/Challenges (Phase 4)
- Fix every issue, anti-bluff (Phase 5)

## Execution

Subagent-driven (§11.4.70). Parallelizable read-only units (cascade audit,
backup manifest) dispatched as concurrent subagents; the CONST-038 reconciliation
edit + all git commit/push operations run serially in the conductor thread to
preserve working-tree quiescence (§11.4.84). Pushes background-detached after
commit (§11.4.88), to **all** upstreams per operator policy.

## Done criteria (anti-bluff, evidence-backed)
- [ ] `verify-governance-cascade.sh` output captured showing mandate present
      everywhere (path recorded).
- [ ] CONST-038 reconciled across all 5 root governance docs; committed; pushed
      to all upstreams; cascade re-verified (no contradiction remains).
- [ ] Restore manifest written + validated (round-trip check passes).
- [ ] Main repo + any touched owned submodule committed & pushed to all upstreams.

## Risk / rollback
- Phase 0 mutations are limited to documentation + a manifest file → low blast
  radius, reversible via git revert. The restore manifest itself is the rollback
  anchor for the higher-risk Phases 1–2.
