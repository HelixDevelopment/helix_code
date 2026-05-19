# HelixCode — Fixed Items Summary

> Generated from `docs/Fixed.md` per Constitution §11.4.19. Counts only.
>
> **Round 189 prefix convention:** Open- and closed-item IDs are now scope-prefixed (`HXC`, `HXA`, `HXL`, `HXQ`, `VEN`, …). See `docs/Issues.md` "Prefix convention" section for the full table + legacy `ISSUE-NNN` mapping. The aggregate counts below are unaffected by the rename — only labels change.

## Aggregate counts (post round-system rebaseline 2026-05-19)

| Type | Count | Closure vocabulary (CONST-057) |
|---|---|---|
| Bug | 11 | `Fixed (→ Fixed.md)` |
| Feature | 66 | `Implemented (→ Fixed.md)` |
| Task | 7 | `Completed (→ Fixed.md)` |

**Total closed items**: 82 (in the round-system tracker; pre-round closures tracked separately in `docs/improvements/PROGRESS.md`). Round 323 added HXV-001 (LLMsVerifier 18 pre-existing `tests/` failures resolved — CLI test-build drift + scoring test-assertion drift to honest contract + env-gated SKIP-OK).

## Coverage by round-system phase

| Phase | Closed items | Status |
|---|---|---|
| Round 37-89 (governance + stabilization + LLM wiring) | 6 batched closures | Done |
| CONST-046 Phase 1 (rounds 91-93) | 3 (pkg/i18n core + audit script + injection wiring) | Done |
| CONST-046 Phase 2 (rounds 94-96) | 3 (SelfImprove + HelixLLM + harmony_os, 15 strings) | Done |
| CONST-046 Phase 3 (rounds 97-99) | 4 closed (DocProcessor + Planning + VisionEngine + panoptic + audit-gate; 20 strings + audit-gate) | DONE |
| CONST-046 Phase 4 (rounds 100+) | 5 closed (evaluators + recorded_ai_testgen + desktop + ai_testgen + recorded_mobile, ~40 strings) | Active |

## Open issues snapshot (cross-ref `docs/Issues_Summary.md`)

4 open issues: 3 Bugs / 1 Task / 1 Feature; 3 BLOCKED on operator decision.

*Last regenerated: 2026-05-20 (round 323 — HXV-001 closed). See `docs/Fixed.md` for full closure entries with commit SHAs.*
