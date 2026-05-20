# HelixCode — Issues Summary

> Generated from `docs/Issues.md` per Constitution §11.4.15 + §11.4.16. Must stay in sync with the source via `CM-DOCS-EXPORT-SYNC` discipline.
>
> **Round 189 prefix convention:** IDs are now scope-prefixed (`HXC` = root project; `HXA` = HelixAgent; `HXL` = HelixLLM; `HXQ` = HelixQA; `VEN` = VisionEngine; etc.). See `docs/Issues.md` "Prefix convention" section for the full table + legacy `ISSUE-NNN` → new mapping.

| ID | Title | Type | Status | Discovered | Notes |
|---|---|---|---|---|---|
| VEN-001 | VisionEngine `helix-gitlab` remote repo missing (404) | Task | Completed (→ Fixed.md) round 188 | 2026-05-19 | NOT missing — URL misconfig (HelixDevelopment vs helixdevelopment1 path); fixed via git remote set-url; 100/100 owned-org URLs reachable |
| VEN-002 | VisionEngine `vasic-digital-github` fork lineage divergent at SHA 93c830a | Bug | Fixed (→ Fixed.md) round 340 | 2026-05-19 | Closed: CONST-061 merge-first — real 2-parent merge `70c9e0c` (NO force); 16-file conflict surface resolved; 7 pkgs build+test PASS; 4 remotes FF-pushed |
| HXL-001 | HelixLLM `analysis_test.go` hardcoded absolute path (was: helix_agent) | Bug | Fixed (→ Fixed.md) round 105 | 2026-05-19 | Closed via t.TempDir + fixtures (a5e56d4) |
| HXL-002 | HelixLLM `gateway/middleware` TOON `WriteTOON` 500 (was: helix_agent) | Bug | Fixed (→ Fixed.md) round 105 | 2026-05-19 | Closed via json.Marshal fallback (a5e56d4) |
| HXC-001 | CONST-052 rename programme: meta-repo dirs still PascalCase | Task | Plan-Ready (→ specs round 113) | 2026-05-15 | Round 113 produced phased plan (f666410). 12 operator decisions needed before execution |
| HXC-002 | Round-74 residual LOGIC FAILs | Bug | Fixed (→ Fixed.md) | 2026-05-19 | All 3 components closed: HelixMemory (106), Planning (107), helix_agent (109) |
| HXA-001 | helix_agent handler tests surfaced after round-109 fix | Bug | Fixed (→ Fixed.md) round 116 | 2026-05-19 | Closed: 4 tests fixed (da782d4) |
| HXA-002 | helix_agent 3 build-failed packages (sibling debate API drift) | Bug | Queued — BLOCKED on cross-submodule coord | 2026-05-19 | digital.vasic.debate API changed; helix_agent consumers broken |
| HXA-003 | venice TestGetCapabilities model-list drift (CONST-037) | Bug | Fixed (→ Fixed.md) round 190 | 2026-05-19 | Closed: structural assertion (NotEmpty + family-substring) + SKIP-OK fallback per CONST-035 (220eff0f) |
| HXC-003 | CONST-046 migration backlog (57,329 violations, baselined, shrinking) | Feature | In progress | 2026-05-19 | Phase 4 (rounds 100+) actively migrating; audit-gate `--fail-on-new` enforced |
| HXC-004 | Recovery-batch under-verification (40% FAIL rate per round 193) | Bug | Fixed (→ Fixed.md) round 200 | 2026-05-19 | Closed: 11 test assertions re-keyed to message-IDs across llm/logo/notification + performance build-break fixed; all 4 packages PASS, mutation-verified |
| HXQ-001 | helix_qa intermittent TestPerformance flake (host-load-sensitive) | Bug | Fixed (→ Fixed.md) round 325 | 2026-05-19 | Closed: path (b) chosen — 3 `pkg/vision/` perf tests gated behind `HOST_LOAD_DEDICATED=1` env var (SKIP-OK on loaded hosts, strict on dedicated); tolerance NOT loosened; helix_qa `649e2dd` |
| PAN-001 | panoptic `appendJSONString` truncates multi-byte UTF-8 runes (TestResult.MarshalJSON) | Bug | Fixed (→ Fixed.md) round 302 | 2026-05-19 | Closed: utf8.AppendRune applied (panoptic 24aa627); detector flipped regression-present → fixed; 39/39 challenge PASS |
| HXC-005 | `cmd/performance_optimization_standalone/main.go` is a CONST-035 simulation bluff | Bug | Fixed (→ Fixed.md) round 318 | 2026-05-20 | Closed: obsolete bluff DELETED (superseded by real cmd/performance_optimization → internal/performance); regression test added |
| HXV-001 | LLMsVerifier 18 pre-existing `tests/` failures (CLI + scoring) | Bug | Fixed (→ Fixed.md) round 323 | 2026-05-20 | Closed: 18 failures classified (test-build drift `go run` single-file → whole-package + test-assertion drift to honest `ErrVerificationNotWired` contract + env-gated SKIP-OK 404 tolerance); all `tests/...` PASS; LLMsVerifier `59f542ba` |

**Counts**: 3 open (VEN-002 closed round 340; HXQ-001 closed round 325; HXV-001 closed round 323; HXC-005 closed round 318; PAN-001 closed round 302; HXC-004 closed round 200; VEN-001/HXA-001/HXA-003/HXC-002/HXL-001/HXL-002 all closed) | 1 Bug open (HXA-002) | 1 Task open (HXC-001 Plan-Ready) | 1 Feature open (HXC-003) | 1 BLOCKED (HXA-002 cross-submodule coord)

*Last regenerated: 2026-05-20 (round 340 — VEN-002 closed → Fixed.md via CONST-061 merge-first). See `docs/Issues.md` for full details + prefix convention table. PDF/HTML exports auto-regenerated via `scripts/regenerate-tracker-exports.sh`.*
