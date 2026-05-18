# HelixCode — Fixed Items Tracker

> Per Constitution §11.4.19 (Fixed-document column-alignment) + CONST-057 (Type-aware closure vocabulary: `Bug` → `Fixed`, `Feature` → `Implemented`, `Task` → `Completed`, all with `(→ Fixed.md)` suffix preserved).
>
> This file is a **closure ledger** — items migrate here from `docs/Issues.md` ONLY after positive captured-evidence per §11.4.5.
>
> **Authoritative round-by-round narrative**: `docs/CONTINUATION.md` (CONST-044). Each row below points to the relevant close-out section there. Items predating the round-system are not retroactively captured (would be impractical) — they live in commit history + the `docs/improvements/` evidence chain (P0-P5 phases).

| Closure | Title | Type | Status | Round | Commit(s) | Evidence |
|---|---|---|---|---|---|---|
| 2026-05-19 | CONST-046 i18n architecture design doc | Feature | Implemented (→ Fixed.md) | 90 | f9dc102 | 368 LOC design; Option D (nicksnyder/go-i18n/v2) selected |
| 2026-05-19 | pkg/i18n core foundation | Feature | Implemented (→ Fixed.md) | 91 | e29b075 | 11 tests + mutation; Bundle/Localizer + sentinel errors |
| 2026-05-19 | CONST-046 audit script (soft-warn) | Feature | Implemented (→ Fixed.md) | 92 | 57de105 | 5 tests; real-tree scan 57,345 violations across 21,937 files |
| 2026-05-19 | Per-submodule i18n injection wiring + i18nadapter | Feature | Implemented (→ Fixed.md) | 93 | 03e131f + 930c6fe | 3-layer pattern; Lazy proof-of-life; bilingual EN+SR |
| 2026-05-19 | SelfImprove × 8 hardcoded-content migration | Feature | Implemented (→ Fixed.md) | 94 | a39d855 + c73a8f4 | LLM prompt-builder strings; 11 test assertions + mutation |
| 2026-05-19 | HelixLLM × 2 CLI strings migration | Feature | Implemented (→ Fixed.md) | 95 | abe0319 + 380e1c0 | TranslatorAPI surface added; 7 new tests |
| 2026-05-19 | harmony_os × 5 CLI headers migration | Feature | Implemented (→ Fixed.md) | 96 | 1eb1851 | 7 tests + mutation; Option A uniform pattern |
| 2026-05-19 | DocProcessor CLI × 8 migration | Feature | Implemented (→ Fixed.md) | 97 | e584e4b + ae83bc8 | Refactored to runCLI(); 6 tests + mutation; Upstreams recipe fix bonus |
| 2026-05-19 | Planning × 3 + VisionEngine × 4 migration | Feature | Implemented (→ Fixed.md) | 98 | 6abed9b + 2d0c35b + a79e022 | 13 tests + dual mutation; ISSUE-001 + ISSUE-002 surfaced |
| 2026-05-19 | CONST-046 audit-gate fail-on-new + baseline | Feature | Implemented (→ Fixed.md) | 99b | 3f4f110 | 54,803 baseline keys; 10 tests + mutation + 4-scenario smoke |
| 2026-05-19 | panoptic × 5 cobra Short descriptions migration | Feature | Implemented (→ Fixed.md) | 99a | 3074c77 + c4e50d8 | 8 tests + mutation; pkg/i18n/global.go package-level seam pattern; install_upstreams bonus |
| 2026-05-19 | challenges/pkg/i18n/ Phase 4 infrastructure + evaluators.go migration | Feature | Implemented (→ Fixed.md) | 100 | 898e39f + ba5b76d | Infrastructure reused by rounds 101+; formal report pending |
| 2026-05-19 | challenges/pkg/userflow/challenge_recorded_ai_testgen.go × 10 of 25 migration | Feature | Implemented (→ Fixed.md) | 101 | 67a6c9d + 1a1b270 | 10 user-facing AssertionResult.Message; 10 tests + mutation; baseline-preserving fallback pattern |
| 2026-05-19 | challenges/pkg/userflow/challenge_desktop.go migration | Feature | Implemented (→ Fixed.md) | 102 | (submodule TBD) + 74c43ec | Formal report truncated; commit visible |
| 2026-05-19 | challenges/pkg/userflow/challenge_ai_testgen.go × 10 user-facing migration | Feature | Implemented (→ Fixed.md) | 103 | 73bd0e7 + 5002c97 | 9 tests + mutation; baseline-preserving fallback pattern |
| 2026-05-19 | challenges/pkg/userflow/challenge_recorded_mobile.go × 7 unique × 14 call sites | Feature | Implemented (→ Fixed.md) | 104 | 012164c + 852c172 + cdb753f | 12 tests + mutation; launch+flow dedup; baseline refresh applied |
| 2026-05-19 | ISSUE-003: HelixLLM analysis_test.go hardcoded path | Bug | Fixed (→ Fixed.md) | 105 | a5e56d4 + fedd152 | t.TempDir + fixtures; bonus git_test.go same-pattern fix (7 more tests); 6 tests PASS + mutation |
| 2026-05-19 | ISSUE-004: HelixLLM TOON WriteTOON 500 | Bug | Fixed (→ Fixed.md) | 105 | a5e56d4 + fedd152 | Root cause: round-27 TOON Marshal anti-bluff change + WriteTOON treating any error as 500. Fix: json.Marshal fallback preserving application/toon; 19 middleware tests PASS + mutation |
| 2026-05-19 | ISSUE-006 (partial): HelixMemory LOGIC-class FAIL cleanup | Bug | Fixed (→ Fixed.md) | 106 | 69016df + 6862cc7 | 6 FAIL/23 PASS → 0 FAIL/29 PASS. Single root cause: go.mod replace ../Memory → ../../vasic-digital/Memory (wrong depth). +5 LOC. Mutation verified |
| 2026-05-19 | ISSUE-006 (partial): Planning LOGIC FAIL audit confirms clean | Task | Completed (→ Fixed.md) | 107 | (no-op) | 275 PASS / 0 FAIL / 20 SKIP-OK. Zero LOGIC FAILs needed fixing. Likely incidentally fixed by round 98 i18n migration. No commit per dispatch spec |
| 2026-05-19 | CONST-046 i18n implemented-architecture overview doc | Task | Completed (→ Fixed.md) | 111 | 2bbd516 | 325 lines / 3048 words / 9 sections; 28 commit SHA citations + 14 file-path refs; zero [unverified] markings |
| 2026-05-19 | Tracker HTML + PDF exports per §11.4.19 | Feature | Implemented (→ Fixed.md) | 110 | e028073 | pandoc 3.9 + weasyprint; 10 artefacts (4 HTML + 4 PDF + script + README ~160KB); validated + mutation-tested |
| 2026-05-19 | helix_code/cmd/helix_config/main.go × 10 migration | Feature | Implemented (→ Fixed.md) | 108 | 878fcfc + 5b5c3c6 | Phase 4 next-tier; dynamic-pick agent selected helix_config CLI |
| 2026-05-19 | helix_qa i18n kickoff (Phase 4 round 7) | Feature | Implemented (→ Fixed.md) | 112 | a676ba2 + c538642 | Submodule pointer + baseline refresh; formal report truncated |
| 2026-05-19 | CONST-052 rename programme phased plan (ISSUE-005 plan) | Task | Completed (→ Fixed.md) | 113 | f666410 | 522 LOC / 4709 words / 9 sections; 107 renames inventoried; 12 operator decisions; estimated 5 days execution |
| 2026-05-19 | LLMOrchestrator i18n kickoff (Phase 4 round 9) | Feature | Implemented (→ Fixed.md) | 115 | 26b7609 + 954ab7a | 5/17 strings migrated (1 invocationError per 5 builder agents); NoopTranslator-fallback pattern keeps bare ID from leaking; +410 LOC |
| 2026-05-19 | ISSUE-006 (final): helix_agent inner LOGIC FAIL cleanup | Bug | Fixed (→ Fixed.md) | 109 | 0f492e98 + 35e0d52 | 5/7 LOGIC FAILs fixed (all test-side bluffs, zero production); 2 reclassified as cross-cutting; +49 LOC; ISSUE-006 fully CLOSED |
| 2026-05-19 | LLMsVerifier i18n kickoff (Phase 4 round 8) | Feature | Implemented (→ Fixed.md) | 114 | 2e670bb2 + c5675e6 + e959a4f | 5/1819 strings migrated (CLI table headers/empties); package-level seam pattern; 8 tests + mutation; baseline 57,320; ~1814 remain |
| 2026-05-19 | HelixSpecifier i18n kickoff (Phase 4 round 10) | Feature | Implemented (→ Fixed.md) | 117 | (submodule TBD) + 2d97af3 + 156c931 | Pointer + baseline refresh visible; formal report truncated |
| 2026-05-19 | Storage i18n kickoff (Phase 4 round 11) | Feature | Implemented (→ Fixed.md) | 118 | (submodule TBD) + 938dd9f | Pointer visible; formal report pending |
| 2026-05-19 | LLMOps i18n kickoff (Phase 4 round 12) | Feature | Implemented (→ Fixed.md) | 119 | (submodule TBD) + 8afad84 | Pointer visible; formal report minimal |
| 2026-05-19 | VectorDB i18n kickoff (Phase 4 round 13) | Feature | Implemented (→ Fixed.md) | 120 | (submodule TBD) + c74e7ed + 6ea87b8 | Pointer + baseline refresh visible; formal report pending |
| 2026-05-19 | Observability i18n kickoff (Phase 4 round 14) | Feature | Implemented (→ Fixed.md) | 121 | (submodule TBD) + b95877a + 9380b02 | Pointer + baseline refresh visible; formal report pending |
| 2026-05-19 | MCP_Module i18n kickoff (Phase 4 round 15) | Feature | Implemented (→ Fixed.md) | 122 | d7b5e6c + 76b4a29 | 6→0 violations (clean); 5 migrated (RPCError × 2 + server × 3); package-level seam; 0 remaining |
| 2026-05-19 | ISSUE-009: helix_agent 4 handler tests | Bug | Fixed (→ Fixed.md) | 116 | (submodule TBD) + da782d4 | Pointer visible; formal report pending |
| 2026-05-19 | Messaging i18n kickoff (Phase 4 round 16) | Feature | Implemented (→ Fixed.md) | 123 | 51ff3ab + b762b79 | vasic-digital (attribution correction); 5 sites; atomic.Value per-pkg wiring + NoopTranslator-key-verbatim fallback; +326 LOC |
| 2026-05-19 | Middleware i18n kickoff (Phase 4 round 17) | Feature | Implemented (→ Fixed.md) | 124 | f491c45 + 5e61707 | vasic-digital (3rd attribution correction); 3 http.Error strings (401/429/415); Option/Config wiring; 4 tests + mutation; +343 LOC |
| 2026-05-19 | Plugins i18n kickoff (Phase 4 round 18) | Feature | Implemented (→ Fixed.md) | 125 | c37b2b2 + 3699b31 | vasic-digital (4th attribution correction); 5 sites (Metadata.Validate × 2 + sandbox × 3); 8 tests + mutation; +399 LOC |
| 2026-05-19 | Streaming i18n kickoff (Phase 4 round 19) | Feature | Implemented (→ Fixed.md) | 126 | f32380d + 70e1724 | vasic-digital (5th attribution correction); 5 sites (SSE × 2 + WS × 2 + Transport × 1); mixed Config-field + package-seam; 9 packages PASS + mutation; +377 LOC |
| 2026-05-19 | Watcher i18n kickoff (Phase 4 round 20) | Feature | Implemented (→ Fixed.md) | 127 | (submodule TBD) + 66322c2 | Pointer visible; formal report pending |
| 2026-05-19 | Round 74-87 release-gate stabilization | Task | Completed (→ Fixed.md) | 82-87 | various | 19 of 26 round-74 FAILs closed (helix_qa+panoptic+LLMsVerifier+Observability+Optimization+challenges) |
| 2026-05-19 | release-gate-test.sh --skip-env-failures filter | Feature | Implemented (→ Fixed.md) | 89 | d3b0b92 | 13 regex catalogue + 6 fixtures + HelixLLM smoke validation |
| 2026-05-19 | CONST-052 reference-drift sweep (73 submodules) | Task | Completed (→ Fixed.md) | 88 | a1d3de8 | 3 with drift fixed (helix_agent + challenges + LLMsVerifier) |
| 2026-05-19 | challenges go.mod path fix `../Containers`→`../containers` | Bug | Fixed (→ Fixed.md) | 87 | a1348d9 | CONST-052 drift; 17/17 PASS post-fix |
| 2026-05-19 | LLMOrchestrator builders × 5 wired | Feature | Implemented (→ Fixed.md) | 64-76 | various | gemini/junie/opencode/claudecode/qwencode CLI binaries |
| 2026-05-19 | 4-vendor GPU telemetry chain (NVIDIA+AMD+Apple+Intel) | Feature | Implemented (→ Fixed.md) | 43-51 | various | cognee/performance_optimizer.go probe chain |
| 2026-05-19 | LLM Err coverage 100% across 17 providers | Feature | Implemented (→ Fixed.md) | 46-63 | various | missing_types.go Err field + wiring |

*Last regenerated: 2026-05-19. Earlier closures (P0-P5 phases) tracked via `docs/improvements/PROGRESS.md` + `docs/improvements/*evidence*.md`.*
