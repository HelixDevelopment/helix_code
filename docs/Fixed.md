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
| 2026-05-19 | Round 74-87 release-gate stabilization | Task | Completed (→ Fixed.md) | 82-87 | various | 19 of 26 round-74 FAILs closed (helix_qa+panoptic+LLMsVerifier+Observability+Optimization+challenges) |
| 2026-05-19 | release-gate-test.sh --skip-env-failures filter | Feature | Implemented (→ Fixed.md) | 89 | d3b0b92 | 13 regex catalogue + 6 fixtures + HelixLLM smoke validation |
| 2026-05-19 | CONST-052 reference-drift sweep (73 submodules) | Task | Completed (→ Fixed.md) | 88 | a1d3de8 | 3 with drift fixed (helix_agent + challenges + LLMsVerifier) |
| 2026-05-19 | challenges go.mod path fix `../Containers`→`../containers` | Bug | Fixed (→ Fixed.md) | 87 | a1348d9 | CONST-052 drift; 17/17 PASS post-fix |
| 2026-05-19 | LLMOrchestrator builders × 5 wired | Feature | Implemented (→ Fixed.md) | 64-76 | various | gemini/junie/opencode/claudecode/qwencode CLI binaries |
| 2026-05-19 | 4-vendor GPU telemetry chain (NVIDIA+AMD+Apple+Intel) | Feature | Implemented (→ Fixed.md) | 43-51 | various | cognee/performance_optimizer.go probe chain |
| 2026-05-19 | LLM Err coverage 100% across 17 providers | Feature | Implemented (→ Fixed.md) | 46-63 | various | missing_types.go Err field + wiring |

*Last regenerated: 2026-05-19. Earlier closures (P0-P5 phases) tracked via `docs/improvements/PROGRESS.md` + `docs/improvements/*evidence*.md`.*
