# HelixCode — Fixed Items Summary

> Generated from `docs/Fixed.md` per Constitution §11.4.19. Counts only.
>
> **Round 189 prefix convention:** Open- and closed-item IDs are now scope-prefixed (`HXC`, `HXA`, `HXL`, `HXQ`, `VEN`, …). See `docs/Issues.md` "Prefix convention" section for the full table + legacy `ISSUE-NNN` mapping. The aggregate counts below are unaffected by the rename — only labels change.

## Aggregate counts (post round-system rebaseline 2026-05-19)

| Type | Count | Closure vocabulary (CONST-057) |
|---|---|---|
| Bug | 13 | `Fixed (→ Fixed.md)` |
| Feature | 66 | `Implemented (→ Fixed.md)` |
| Task | 7 | `Completed (→ Fixed.md)` |

**Total closed items**: 84 (in the round-system tracker; pre-round closures tracked separately in `docs/improvements/PROGRESS.md`). Round 340 added VEN-002 (VisionEngine `vasic-digital-github` divergent fork lineage resolved via CONST-061 §11.4.41 merge-first — real 2-parent merge commit `70c9e0c`, NO force-push, 16-file conflict surface resolved preserving the anti-bluff truth per CONST-035; 7 packages build+test PASS; 4 remotes fast-forwarded). Round 325 added HXQ-001 (helix_qa `TestPerformance` flake resolved — 3 `pkg/vision/` perf tests gated behind `HOST_LOAD_DEDICATED` env var; resolution path (b), timing tolerance NOT loosened, anti-bluff strictness preserved per CONST-035).

## Coverage by round-system phase

| Phase | Closed items | Status |
|---|---|---|
| Round 37-89 (governance + stabilization + LLM wiring) | 6 batched closures | Done |
| CONST-046 Phase 1 (rounds 91-93) | 3 (pkg/i18n core + audit script + injection wiring) | Done |
| CONST-046 Phase 2 (rounds 94-96) | 3 (SelfImprove + HelixLLM + harmony_os, 15 strings) | Done |
| CONST-046 Phase 3 (rounds 97-99) | 4 closed (DocProcessor + Planning + VisionEngine + panoptic + audit-gate; 20 strings + audit-gate) | DONE |
| CONST-046 Phase 4 (rounds 100+) | 5 closed (evaluators + recorded_ai_testgen + desktop + ai_testgen + recorded_mobile, ~40 strings) | Active |

## Open issues snapshot (cross-ref `docs/Issues_Summary.md`)

3 open issues: 1 Bug / 1 Task / 1 Feature; 2 BLOCKED (VEN-002 operator decision + HXA-002 cross-submodule coord).

*Last regenerated: 2026-05-20 (round 325 — HXQ-001 closed). See `docs/Fixed.md` for full closure entries with commit SHAs.*
