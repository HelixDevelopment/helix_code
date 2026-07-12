# Fixed_Summary

Closed workable items (current_location = Fixed), regenerated from the SQLite single-source-of-truth (§11.4.53).

## Counts by Type × Status

| Type | Status | Count |
|---|---|---|
| Bug | Fixed (→ Fixed.md) | 137 |
| Bug | Obsolete (→ Fixed.md) | 3 |
| Feature | Implemented (→ Fixed.md) | 90 |
| Task | Completed (→ Fixed.md) | 58 |
| **TOTAL** | | **288** |

## Items

| # | Level | Status | Type | Fixed-In Tag(s) | One-line description |
|---|---|---|---|---|---|
| 1 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19 — CONST-046 i18n architecture design doc — 368 LOC design; Option D (nicksnyder/go-i18n/v2) selected |
| 2 | High | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#1 — pkg/i18n core foundation — 11 tests + mutation; Bundle/Localizer + sentinel errors |
| 3 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#10 — This task added translation support to five short help texts shown in the command-line tool for the panoptic component, so those brief descriptions can eventually be displayed in different languages instead of being locked to English. |
| 4 | Medium | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#11 — challenges/pkg/i18n/ Phase 4 infrastructure + evaluators.go migration |
| 5 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#12 — challenges/pkg/userflow/challenge_recorded_ai_testgen.go × 10 of 25 migration |
| 6 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#13 — challenges/pkg/userflow/challenge_desktop.go migration |
| 7 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#14 — challenges/pkg/userflow/challenge_ai_testgen.go × 10 user-facing migration |
| 8 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#15 — challenges/pkg/userflow/challenge_recorded_mobile.go × 7 unique × 14 call sites |
| 9 | — | Completed (→ Fixed.md) | Task | — | FIX-2026-05-19#16 — CONST-046 i18n implemented-architecture overview doc |
| 10 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#17 — This task added the ability to automatically export the project's tracking document into easy-to-read HTML web pages and PDF files, using standard document-conversion tools. This makes it easier for team members and stakeholders to review project status without needing to open raw text files. |
| 11 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#18 — helix_code/cmd/helix_config/main.go × 10 migration |
| 12 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#19 — This task began the work of adding multi-language (translation) support to the helix_qa testing and quality-assurance component, so any text it shows to users can eventually be translated into other languages instead of only English. |
| 13 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#2 — CONST-046 audit script (soft-warn) — 5 tests; real-tree scan 57,345 violations across 21,937 files |
| 14 | — | Completed (→ Fixed.md) | Task | — | FIX-2026-05-19#20 — CONST-052 rename programme phased plan (HXC-001 plan, ex-ISSUE-005) |
| 15 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#21 — This task started adding multi-language support to the LLMOrchestrator component, which coordinates requests across several different AI language models. A handful of its hardcoded English error messages were converted into a translatable format as a first step. |
| 16 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#22 — This task began converting some of the command-line text shown by the LLMsVerifier tool (which checks and validates AI model providers) into a translatable format, so it can eventually support multiple languages. Only a small number of the many text strings in this tool were converted in this first pass. |
| 17 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#23 — This task started adding translation support to the HelixSpecifier tool so that its user-facing text can eventually work in languages other than English. |
| 18 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#24 — This task started adding translation support to the Storage component of the system, which is responsible for saving and retrieving data, so its messages can eventually be shown in multiple languages. |
| 19 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#25 — This task started adding translation support to the LLMOps component, which manages the day-to-day operation of the AI language models the system relies on, so its messages can eventually be shown in multiple languages. |
| 20 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#26 — This task started adding translation support to the VectorDB component, which is the database used to store and search specialized AI-related data, so its messages can eventually be shown in multiple languages. |
| 21 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#27 — This task started adding translation support to the Observability component, the part of the system that monitors overall health and performance, so its messages can eventually be shown in multiple languages. |
| 22 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#28 — This task added translation support to the MCP module, the part of the system that connects to external tool servers. Five hardcoded English error and status messages were converted into a translatable format, fully completing this module's conversion with none left over. |
| 23 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#29 — This task added translation support to the Messaging component, converting five hardcoded English messages into a translatable format so the messaging system's text can eventually work in different languages for users. |
| 24 | Medium | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#3 — This task built the underlying plumbing that lets each individual sub-component of the system plug into the shared translation system, including a working example already supporting both English and Serbian. This lays the groundwork so future work can translate all remaining user-facing text across the whole project. |
| 25 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#30 — This task added translation support to the Middleware component, converting three hardcoded English error messages about authentication and rate-limiting problems into a translatable format. |
| 26 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#31 — This task added translation support to the Plugins component, the part of the system that loads optional add-on extensions, converting five hardcoded messages about plugin checks and safety sandboxing into a translatable format. |
| 27 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#32 — This task added translation support to the Streaming component, which handles live, real-time data flowing to and from users, converting five hardcoded connection-related messages into a translatable format. |
| 28 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#33 — This task added the underlying translation groundwork to the Watcher component, which monitors the system for file and event changes. This was a behind-the-scenes setup step, since this component's outward labels are internal codes rather than text users directly read. |
| 29 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#34 — This task started adding translation support to the conversation component, which manages chat and dialogue interactions with users, so its messages can eventually be shown in multiple languages. |
| 30 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#35 — This task added translation support to the containers component, the part of the system that manages application containers, converting nine hardcoded status and error messages into a translatable format and reducing the number of remaining untranslated messages in that area from 73 to 64. |
| 31 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#36 — This task added translation support to the security component's privilege-check feature, converting twenty-seven hardcoded warning and description messages into a translatable format, a 92 percent reduction in the untranslated messages that remained in that area. |
| 32 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#37 — helix_code/cmd/cli/main.go × 10 migration (Phase 4 round 24) |
| 33 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#38 — This task started adding translation support to the AutoTemp component so its messages can eventually be shown in multiple languages. |
| 34 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#39 — This task started adding translation support to the Auth (login and authentication) component so its messages can eventually be shown in multiple languages. |
| 35 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#4 — This task converted eight hardcoded English text templates used by the SelfImprove component, which builds prompts asking the AI model to review and improve its own code, into a translatable format. |
| 36 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#40 — helix_code/cmd/server/main.go × 10 migration (Phase 4 round 27) |
| 37 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#41 — This task added the underlying translation groundwork, including 36 translation entries, to the shared PliniusCommon library, so that any text it produces can later be translated and can safely support many people using the system at the same time. |
| 38 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#42 — helix_code/applications/terminal_ui × up to 10 migration (Phase 4 round 30) |
| 39 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#43 — helix_code/applications/desktop i18n (Phase 4 round 29) |
| 40 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#44 — helix_code/applications/ios infrastructure-only (Phase 4 round 31) |
| 41 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#45 — helix_code/applications/android infrastructure-only (Phase 4 round 32) |
| 42 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#46 — helix_code/applications/aurora_os × up to 10 migration (Phase 4 round 33) |
| 43 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#47 — helix_code/cmd/config_test × 12 migration (Phase 4 round 34) |
| 44 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#48 — helix_code/cmd/security_test × 10 migration (Phase 4 round 35) |
| 45 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#49 — helix_code/cmd/security_fix × 10 migration (Phase 4 round 36) |
| 46 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#5 — This task converted two hardcoded command-line messages in the HelixLLM tool into a translatable format, and added a small new capability that other parts of the system can reuse to do the same kind of translation work. |
| 47 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#50 — helix_code/cmd/performance_optimization × 10 migration (Phase 4 round 37) |
| 48 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#51 — helix_code/cmd/security_fix_standalone × 10 of 27 migration (Phase 4 round 38) |
| 49 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#52 — helix_code/internal/auth × up to 10 migration (Phase 4 round 39) |
| 50 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#53 — helix_code/internal/agent × 10 of 64 migration (Phase 4 round 40) |
| 51 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#54 — helix_code/internal/cognee × migration (Phase 4 round 41) |
| 52 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#55 — helix_code/internal/commands × 10 migration (Phase 4 round 42) |
| 53 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#56 — helix_code/internal/config × 10 migration (Phase 4 round 43) |
| 54 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#57 — helix_code/internal/context × 8 sites / 5 IDs (Phase 4 round 44) |
| 55 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#58 — helix_code/internal/database × 8 migration (Phase 4 round 45) |
| 56 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#59 — helix_code/internal/discovery migration (Phase 4 round 47) |
| 57 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#6 — This task converted five hardcoded header messages shown in the command-line interface of the HarmonyOS platform build into a translatable format, so users on that platform see properly localized headers. |
| 58 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#60 — helix_code/internal/deployment × 10 migration (Phase 4 round 46) |
| 59 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#61 — helix_code/internal/editor migration (Phase 4 round 48) |
| 60 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#62 — helix_code/internal/event migration (Phase 4 round 49) |
| 61 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#63 — helix_code/internal/focus migration (Phase 4 round 50) |
| 62 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#64 — helix_code/internal/hardware migration (Phase 4 round 51) |
| 63 | — | Completed (→ Fixed.md) | Task | — | FIX-2026-05-19#65 — Round 74-87 release-gate stabilization — 19 of 26 round-74 FAILs closed (helix_qa+panoptic+LLMsVerifier+Observability+Optimization+challenges) |
| 64 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#66 — This task added a new option to the project's pre-release testing script that lets it skip test failures caused only by a missing local setup, such as a missing tool or network access, rather than a genuine problem in the code. This keeps release checks from being blocked by environment issues outside the code itself. |
| 65 | — | Completed (→ Fixed.md) | Task | — | FIX-2026-05-19#67 — This task checked all 73 sub-components of the project to confirm their names followed the project's lowercase naming rules and that every reference to them elsewhere was still accurate, finding and correcting three sub-components where the references had drifted out of sync. |
| 66 | — | Fixed (→ Fixed.md) | Bug | — | FIX-2026-05-19#68 — challenges go.mod path fix `../Containers`→`../containers` |
| 67 | High | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#69 — LLMOrchestrator builders × 5 wired — gemini/junie/opencode/claudecode/qwencode CLI binaries |
| 68 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#7 — DocProcessor CLI × 8 migration — Refactored to runCLI(); 6 tests + mutation; Upstreams recipe fix bonus |
| 69 | High | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#70 — 4-vendor GPU telemetry chain (NVIDIA+AMD+Apple+Intel) |
| 70 | High | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#71 — This task made sure that every one of the 17 different AI-model providers the system can connect to, such as major cloud AI services, correctly reports detailed error information when something goes wrong, instead of failing silently or giving an incomplete error message. |
| 71 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#8 — This task converted seven hardcoded text messages across the Planning component, which breaks tasks into smaller steps, and the VisionEngine component, which processes images, into a translatable format. Two additional issues were discovered and logged along the way for later attention. |
| 72 | Low | Implemented (→ Fixed.md) | Feature | — | FIX-2026-05-19#9 — This task built an automated check that scans the codebase for any newly added hardcoded English text that bypasses the translation system, and blocks the change if any is found. It starts from a recorded baseline of roughly 55,000 existing text strings still to be migrated over time. |
| 73 | — | Fixed (→ Fixed.md) | Bug | — | HXA-001 — HXA-001 (ex-ISSUE-009): helix_agent 4 handler tests |
| 74 | — | Fixed (→ Fixed.md) | Bug | — | HXA-002 — HXA-002 (ex-ISSUE-010): helix_agent debate/llmprovider sibling-submodule API drift |
| 75 | — | Fixed (→ Fixed.md) | Bug | — | HXA-003 — HXA-003 (ex-ISSUE-011): venice `TestGetCapabilities` CONST-037 model-list drift |
| 76 | — | Completed (→ Fixed.md) | Task | — | HXC-001 — HXC-001 (ex-ISSUE-005): CONST-052 lowercase-snake_case rename programme — all owned-org submodule leaf dirs + 57 `Upstreams/` dirs renamed |
| 77 | — | Fixed (→ Fixed.md) | Bug | — | HXC-002 — HXC-002 (ex-ISSUE-006) (partial): HelixMemory LOGIC-class FAIL cleanup |
| 78 | — | Completed (→ Fixed.md) | Task | — | HXC-002#1 — HXC-002 (ex-ISSUE-006) (partial): Planning LOGIC FAIL audit confirms clean |
| 79 | — | Fixed (→ Fixed.md) | Bug | — | HXC-002#2 — HXC-002 (ex-ISSUE-006) (final): helix_agent inner LOGIC FAIL cleanup |
| 80 | High | Implemented (→ Fixed.md) | Feature | — | HXC-003 — HXC-003: CONST-046 i18n migration backlog (no user-facing text hardcoded as static string literals) |
| 81 | — | Fixed (→ Fixed.md) | Bug | — | HXC-004 — HXC-004: recovery-batch 4-package under-verification (llm + logo + notification test-assertion drift + performance translator.go build break) |
| 82 | — | Fixed (→ Fixed.md) | Bug | — | HXC-005 — HXC-005: `cmd/performance_optimization_standalone/main.go` is a CONST-035 simulation bluff |
| 83 | High | Implemented (→ Fixed.md) | Feature | — | HXC-006 — HXC-006: HelixCode Speed Programme — 3-5× faster than competitor AI CLI agents (6-phase / 31-task) |
| 84 | — | Completed (→ Fixed.md) | Task | — | HXC-007 — HXC-007: Constitution §11.4.68/70-74 cascade + meta-pointer bump |
| 85 | — | Fixed (→ Fixed.md) | Bug | — | HXC-008 — HXC-008: CONST-055 G1 governance gaps surfaced by post-constitution-pull validation sweep |
| 86 | — | Completed (→ Fixed.md) | Task | — | HXC-009 — HXC-009: Owned-submodule GitHub ↔ GitLab mirror-divergence reconciliation |
| 87 | — | Completed (→ Fixed.md) | Task | — | HXC-010 — HXC-010: End-to-end Kimi CLI + Qwen Code CodeGraph verification (operator-blocked on LLM backend quota/credentials) |
| 88 | — | Fixed (→ Fixed.md) | Bug | — | HXC-011 — HXC-011: helix_qa runner emits hollow sub-microsecond "PASSED" rows for desktop-platform bank cases instead of executing the case's `action:` |
| 89 | — | Fixed (→ Fixed.md) | Bug | — | HXC-012 — HXC-012: data race in `helix_code/internal/llm/load_balancer.go` background stat-collector goroutine |
| 90 | — | Completed (→ Fixed.md) | Task | — | HXC-016 — HXC-016: §11.4.69–97 governance cascade into all owned submodules + mechanical propagation gate (CONST-047/§3, §11.4.32) |
| 91 | — | Completed (→ Fixed.md) | Task | — | HXC-017 — HXC-017: CodeGraph index excluded all own-org submodules (blanket `dependencies/**`) + config.json was untracked — §11.4.79/§11.4.78/§11.4.80 |
| 92 | — | Fixed (→ Fixed.md) | Bug | — | HXC-021 — HXC-021 + HXC-014a + HXC-015a: fake-skip `Assert(true,"...skipped")` bluffs (11) + empty `TestProviderStress` stubs report green while exercising nothing |
| 93 | — | Fixed (→ Fixed.md) | Bug | — | HXC-022 — HXC-022: test_bank platform+integration packages did not compile — half-written stubs + root package-name collision (anti-bluff: an uncompilable test package never runs) |
| 94 | — | Fixed (→ Fixed.md) | Bug | — | HXC-023 — HXC-023: literal-true `Assert(true,…)`/`AssertTrue(true,…)` PASS-bluffs across the e2e test banks (report PASS without exercising behaviour) |
| 95 | — | Completed (→ Fixed.md) | Task | — | HXC-029 — HXC-029: §11.4.98 full-automation compliance sweep — verify every live/integration/e2e/Challenge test is self-driving with no human-in-the-loop |
| 96 | — | Completed (→ Fixed.md) | Task | — | HXC-030 — HXC-030: §11.4.99 latest-source documentation cross-reference sweep across all operator-facing docs |
| 97 | — | Completed (→ Fixed.md) | Task | — | HXC-031 — Deferred long-tail: CONST-052 renames (RESOLVED — none remain) + Codex/Cline reference-agent ports |
| 98 | — | Completed (→ Fixed.md) | Task | — | HXC-031 — HXC-031: Deferred long-tail: CONST-052 renames (RESOLVED — none remain) + Codex/Cline reference-agent ports |
| 99 | — | Fixed (→ Fixed.md) | Bug | — | HXC-032 — HXC-032: LLMOrchestrator submodule had committed git conflict markers in 5 Go files (26 hunks) breaking `helix_agent` build — a §11.4 build-layer PASS-bluff already on origin/master |
| 100 | — | Fixed (→ Fixed.md) | Bug | — | HXC-033 — HXC-033: codegraph 0.9.7 update dropped own-org submodules from the index + full index/sync appeared to crash (§11.4.79 regression) |
| 101 | — | Completed (→ Fixed.md) | Task | — | HXC-034 — HXC-034: Cascade constitution §11.4.102 (mandatory systematic-debugging + always-loaded using-superpowers + plugin-dependency availability) into every owned submodule + wire the CM-COVENANT-114-102-PROPAGATION enforcement gate |
| 102 | — | Fixed (→ Fixed.md) | Bug | — | HXC-035 — HXC-035: `POST /api/v1/auth/register` returned 400 `internal_auth_failed_create_user` on a fresh DB — blocked all authenticated flows |
| 103 | — | Fixed (→ Fixed.md) | Bug | — | HXC-036 — HXC-036: Systemic CONST-046 i18n defect — 74 packages emitted raw message-ID keys to users because the boot-time translator wiring was never implemented (a §11.9 tests-green-feature-broken defect) |
| 104 | High | Completed (→ Fixed.md) | Task | — | HXC-037 — §11.4.103-141 + CONST-048..060 anchor-cascade backfill into 7 owned submodules (verify-governance-cascade.sh 30→0) |
| 105 | — | Completed (→ Fixed.md) | Task | — | HXC-037 — HXC-037: §11.4.103-141 + CONST-048..060 anchor-cascade backfill into 7 owned submodules (verify-governance-cascade.sh 30→0) |
| 106 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-038 — docs_chain G14: fixed.yaml fixed_summary transform-contract mismatch + stale state.json baseline false-CONFLICT on issues context |
| 107 | — | Fixed (→ Fixed.md) | Bug | — | HXC-038 — HXC-038: docs_chain G14: fixed.yaml fixed_summary transform-contract mismatch + stale state.json baseline false-CONFLICT on issues context |
| 108 | Medium | Completed (→ Fixed.md) | Task | — | HXC-039 — G7 §11.4.83 docs/qa evidence gap: 8 past feature/fix commits lack docs/qa/<run-id>/ directories |
| 109 | — | Completed (→ Fixed.md) | Task | — | HXC-039 — HXC-039: G7 §11.4.83 docs/qa evidence gap: 8 past feature/fix commits lack docs/qa/<run-id>/ directories |
| 110 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-040 — CLAUDE.md §9/§3.4 anti-bluff smoke command false-alarm (527 i18n/test hits) + case-sensitivity miss of BLUFF-001 |
| 111 | — | Fixed (→ Fixed.md) | Bug | — | HXC-040 — HXC-040: CLAUDE.md §9/§3.4 anti-bluff smoke command false-alarm (527 i18n/test hits) + case-sensitivity miss of BLUFF-001 |
| 112 | Medium | Implemented (→ Fixed.md) | Feature | — | HXC-041 — helixqa standalone HTTP bank-runner subcommand (helixqa http) drives http: banks vs live server without Playwright or LLM |
| 113 | — | Implemented (→ Fixed.md) | Feature | — | HXC-041 — HXC-041: helixqa standalone HTTP bank-runner subcommand (helixqa http) drives http: banks vs live server without Playwright or LLM |
| 114 | Medium | Completed (→ Fixed.md) | Task | — | HXC-042 — CONST-050(B) challenge-coverage gap: 12 missing challenge scripts in debate_orchestrator + helix_agent (ddos/stress/chaos/scaling/ui/ux) |
| 115 | — | Completed (→ Fixed.md) | Task | — | HXC-042 — HXC-042: CONST-050(B) challenge-coverage gap: 12 missing challenge scripts in debate_orchestrator + helix_agent (ddos/stress/chaos/scaling/ui/ux) |
| 116 | High | Fixed (→ Fixed.md) | Bug | — | HXC-043 — auth Login nil-DB panic causes HTTP 500: server advertises graceful no-DB operation but first /api/v1/auth/login dereferences nil s.db |
| 117 | — | Fixed (→ Fixed.md) | Bug | — | HXC-043 — HXC-043: auth Login nil-DB panic causes HTTP 500: server advertises graceful no-DB operation but first /api/v1/auth/login dereferences nil s.db |
| 118 | Medium | Obsolete (→ Fixed.md) | Bug | — | HXC-044 — internal/cognee: AMD-GPU rocm-smi JSON parser returns -1 sentinel instead of parsed GPU-use value |
| 119 | — | Obsolete (→ Fixed.md) | Bug | — | HXC-044 — HXC-044: internal/cognee — AMD-GPU rocm-smi JSON parser returns -1 sentinel instead of parsed GPU-use value |
| 120 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-045 — internal/hooks: cancelled hook ExecutionResult leaves duration unset (should always be populated) |
| 121 | — | Fixed (→ Fixed.md) | Bug | — | HXC-045 — HXC-045: internal/hooks: cancelled hook ExecutionResult leaves duration unset (should always be populated) |
| 122 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-046 — internal/memory/providers: generateThreadID() non-unique under fast back-to-back calls (timestamp-only, same-nanosecond collision) |
| 123 | — | Fixed (→ Fixed.md) | Bug | — | HXC-046 — HXC-046: internal/memory/providers: generateThreadID() non-unique under fast back-to-back calls (timestamp-only, same-nanosecond collision) |
| 124 | Low | Completed (→ Fixed.md) | Task | — | HXC-047 — internal/redis TestNewClient_WithDatabase needs-live-Redis with no SKIP-OK guard (§11.4.98) + i18n error no longer contains literal Redis |
| 125 | — | Completed (→ Fixed.md) | Task | — | HXC-047 — HXC-047: internal/redis TestNewClient_WithDatabase needs-live-Redis with no SKIP-OK guard (§11.4.98) + i18n error no longer contains literal Redis |
| 126 | Medium | Implemented (→ Fixed.md) | Feature | — | HXC-048 — helixcode-system.yaml HelixQA bank: 11 self-driving http cases for the non-auth server surface (health/server-info/system-status/llm-providers + negatives) |
| 127 | — | Implemented (→ Fixed.md) | Feature | — | HXC-048 — HXC-048: helixcode-system.yaml HelixQA bank: 11 self-driving http cases for the non-auth server surface (health/server-info/system-status/llm-providers + negatives) |
| 128 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-049 — doc_processor TestAutomation_UpstreamsExist reads capital 'Upstreams' but canonical dir is lowercase 'upstreams' (CONST-052 case drift, deterministic FAIL) |
| 129 | — | Fixed (→ Fixed.md) | Bug | — | HXC-049 — HXC-049: doc_processor TestAutomation_UpstreamsExist reads capital 'Upstreams' but canonical dir is lowercase 'upstreams' (CONST-052 case drift, deterministic FAIL) |
| 130 | Low | Completed (→ Fixed.md) | Task | — | HXC-050 — event_bus NATS env-gated integration skips lack SKIP-OK markers required by the no-silent-skips gate (§11.4.98) |
| 131 | — | Completed (→ Fixed.md) | Task | — | HXC-050 — HXC-050: event_bus NATS env-gated integration skips lack SKIP-OK markers required by the no-silent-skips gate (§11.4.98) |
| 132 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-051 — helix_llm + helix_memory go.mod replace directives point to non-existent ../../vasic-digital/* sibling layout (CONST-051(C) dependency-layout) |
| 133 | — | Fixed (→ Fixed.md) | Bug | — | HXC-051 — HXC-051: helix_llm + helix_memory go.mod replace directives point to non-existent ../../vasic-digital/* sibling layout (CONST-051(C) dependency-layout) |
| 134 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-052 — background_tasks go.mod build break — capitalised replace paths |
| 135 | — | Fixed (→ Fixed.md) | Bug | — | HXC-052 — HXC-052: background_tasks go.mod build break — capitalised replace paths |
| 136 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-053 — conversation go.mod build break — capitalised replace path ../Messaging |
| 137 | — | Fixed (→ Fixed.md) | Bug | — | HXC-053 — HXC-053: conversation go.mod build break — capitalised replace path ../Messaging |
| 138 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-054 — leak_detector parallel test flake — §11.4.50 determinism |
| 139 | — | Fixed (→ Fixed.md) | Bug | — | HXC-054 — HXC-054: leak_detector parallel test flake — §11.4.50 determinism |
| 140 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-055 — formatters brittle cat --version probe + go_hello fixtures break go build |
| 141 | — | Fixed (→ Fixed.md) | Bug | — | HXC-055 — HXC-055: formatters brittle cat --version probe + go_hello fixtures break go build |
| 142 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-056 — 7 submodules: CONST-052 capitalised replace => ../PliniusCommon (dir is plinius_common) |
| 143 | — | Fixed (→ Fixed.md) | Bug | — | HXC-056 — HXC-056: 7 submodules: CONST-052 capitalised replace => ../PliniusCommon (dir is plinius_common) |
| 144 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-057 — recovery go.mod missing require+replace for digital.vasic.concurrency (pkg/breaker import unwired) |
| 145 | — | Fixed (→ Fixed.md) | Bug | — | HXC-057 — HXC-057: recovery go.mod missing require+replace for digital.vasic.concurrency (pkg/breaker import unwired) |
| 146 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-058 — helix_agent go build fails on vendored third-party cli_agents/continue test fixture (quarantine) |
| 147 | — | Fixed (→ Fixed.md) | Bug | — | HXC-058 — HXC-058: helix_agent go build fails on vendored third-party cli_agents/continue test fixture (quarantine) |
| 148 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-059 — debate_orchestrator sandbox: ctx-cancel/timeout fails to kill child process tree on non-Linux (§11.4.81) |
| 149 | — | Fixed (→ Fixed.md) | Bug | — | HXC-059 — HXC-059: debate_orchestrator sandbox: ctx-cancel/timeout fails to kill child process tree on non-Linux (§11.4.81) |
| 150 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-060 — debate_orchestrator challenges/runner/main.go:516 context cancel not called on all return paths (vet leak) |
| 151 | — | Fixed (→ Fixed.md) | Bug | — | HXC-060 — HXC-060: debate_orchestrator challenges/runner/main.go:516 context cancel not called on all return paths (vet leak) |
| 152 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-061 — helix_agent legacy unit-test calls memory.GetRelevant with stale 2-arg signature (won't compile) |
| 153 | — | Fixed (→ Fixed.md) | Bug | — | HXC-061 — HXC-061: helix_agent legacy unit-test calls memory.GetRelevant with stale 2-arg signature (won't compile) |
| 154 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-062 — helix_specifier pkg/metrics copies sync.RWMutex by value (vet lock-copy, concurrency hazard) |
| 155 | — | Fixed (→ Fixed.md) | Bug | — | HXC-062 — HXC-062: helix_specifier pkg/metrics copies sync.RWMutex by value (vet lock-copy, concurrency hazard) |
| 156 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-063 — panoptic StartRecording: unreachable recording-bootstrap after early return nil — recorder never starts |
| 157 | — | Fixed (→ Fixed.md) | Bug | — | HXC-063 — HXC-063: panoptic StartRecording: unreachable recording-bootstrap after early return nil — recorder never starts |
| 158 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-064 — cognee AMD-GPU parser tests flake under parallel load (rocm-smi fake subprocess signal-killed before 2s timeout, §11.4.50) |
| 159 | — | Fixed (→ Fixed.md) | Bug | — | HXC-064 — HXC-064: cognee AMD-GPU parser tests flake under parallel load (rocm-smi fake subprocess signal-killed before 2s timeout, §11.4.50) |
| 160 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-065 — cache/pkg/postgres: finite-TTL Set invisible to immediate Get (expires_at clock/timezone skew vs real PG) |
| 161 | — | Fixed (→ Fixed.md) | Bug | — | HXC-065 — HXC-065: cache/pkg/postgres: finite-TTL Set invisible to immediate Get (expires_at clock/timezone skew vs real PG) |
| 162 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-066 — inner internal/database integration tests hardcode localhost:5433/helix_test, never read HELIX_DATABASE_* env |
| 163 | — | Fixed (→ Fixed.md) | Bug | — | HXC-066 — HXC-066: inner internal/database integration tests hardcode localhost:5433/helix_test, never read HELIX_DATABASE_* env |
| 164 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-067 — inner internal/redis stress suite reads TEST_REDIS_HOST/PORT (default :6379) not HELIX_REDIS_HOST/PORT |
| 165 | — | Fixed (→ Fixed.md) | Bug | — | HXC-067 — HXC-067: inner internal/redis stress suite reads TEST_REDIS_HOST/PORT (default :6379) not HELIX_REDIS_HOST/PORT |
| 166 | Medium | Implemented (→ Fixed.md) | Feature | — | HXC-068 — speckit debate adapter wireable into agentic debate flow |
| 167 | — | Implemented (→ Fixed.md) | Feature | — | HXC-068 — HXC-068: speckit debate adapter wireable into agentic debate flow |
| 168 | High | Implemented (→ Fixed.md) | Feature | — | HXC-069 — HelixMemory default-on durable persistence with graceful fallback |
| 169 | — | Implemented (→ Fixed.md) | Feature | — | HXC-069 — HXC-069: HelixMemory default-on durable persistence with graceful fallback |
| 170 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-070 — HelixMemory persist log no longer misreports success on failure |
| 171 | — | Fixed (→ Fixed.md) | Bug | — | HXC-070 — HXC-070: HelixMemory persist log no longer misreports success on failure |
| 172 | Medium | Completed (→ Fixed.md) | Task | — | HXC-071 — Web LLM handler httptest coverage for generate and stream |
| 173 | — | Completed (→ Fixed.md) | Task | — | HXC-071 — HXC-071: Web LLM handler httptest coverage for generate and stream |
| 174 | Medium | Implemented (→ Fixed.md) | Feature | — | HXC-072 — CLI /undo and /diff slash commands over autocommit substrate |
| 175 | — | Implemented (→ Fixed.md) | Feature | — | HXC-072 — HXC-072: CLI /undo and /diff slash commands over autocommit substrate |
| 176 | Medium | Implemented (→ Fixed.md) | Feature | — | HXC-073 — The CLI keeps an automatic git-backed history of every edit it makes so users can review and safely roll back changes; this item established that autocommit substrate underneath the edit history. |
| 177 | — | Implemented (→ Fixed.md) | Feature | — | HXC-073 — HXC-073: Autocommit git substrate backing CLI edit history |
| 178 | Medium | Implemented (→ Fixed.md) | Feature | — | HXC-074 — Mobile gomobile Generate binding for on-device LLM calls |
| 179 | — | Implemented (→ Fixed.md) | Feature | — | HXC-074 — HXC-074: Mobile gomobile Generate binding for on-device LLM calls |
| 180 | Low | Completed (→ Fixed.md) | Task | — | HXC-075 — Phase-1 CLI-Agent Fusion plan reconciliation with delivered state |
| 181 | — | Completed (→ Fixed.md) | Task | — | HXC-075 — HXC-075: Phase-1 CLI-Agent Fusion plan reconciliation with delivered state |
| 182 | High | Implemented (→ Fixed.md) | Feature | — | HXC-076 — Web /llm/generate + /llm/stream endpoints with frontend (partial — e2e pending) |
| 183 | — | Implemented (→ Fixed.md) | Feature | — | HXC-076 — HXC-076: Web /llm/generate + /llm/stream endpoints with frontend (partial — e2e pending) |
| 184 | Low | Implemented (→ Fixed.md) | Feature | — | HXC-077 — T1.5 context-window percentage indicator (partial) |
| 185 | — | Implemented (→ Fixed.md) | Feature | — | HXC-077 — HXC-077: T1.5 context-window percentage indicator (partial) |
| 186 | Medium | Completed (→ Fixed.md) | Task | — | HXC-078 — T1.6 SKILL.md precedence resolution (partial) |
| 187 | — | Completed (→ Fixed.md) | Task | — | HXC-078 — HXC-078: T1.6 SKILL.md precedence resolution (partial) |
| 188 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-079 — debate_orchestrator consensus emits unresolved i18n key |
| 189 | — | Fixed (→ Fixed.md) | Bug | — | HXC-079 — HXC-079: debate_orchestrator consensus emits unresolved i18n key |
| 190 | High | Fixed (→ Fixed.md) | Bug | — | HXC-080 — /debate and /specify broken at runtime — single agent vs 2-min |
| 191 | — | Fixed (→ Fixed.md) | Bug | — | HXC-080 — HXC-080: /debate and /specify broken at runtime — single agent vs 2-min |
| 192 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-081 — helix_specifier speckit topic i18n key unresolved plus format-verb mismatch |
| 193 | — | Fixed (→ Fixed.md) | Bug | — | HXC-081 — HXC-081: helix_specifier speckit topic i18n key unresolved plus format-verb mismatch |
| 194 | High | Fixed (→ Fixed.md) | Bug | — | HXC-082 — performance optimizer fabricates success — 8 apply methods sleep and return Success true |
| 195 | — | Fixed (→ Fixed.md) | Bug | — | HXC-082 — HXC-082: performance optimizer fabricates success — 8 apply methods sleep and return Success true |
| 196 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-083 — production_deployer fabricates rollback env-prep server-validation and strategy differentiation |
| 197 | — | Fixed (→ Fixed.md) | Bug | — | HXC-083 — HXC-083: production_deployer fabricates rollback env-prep server-validation and strategy differentiation |
| 198 | High | Fixed (→ Fixed.md) | Bug | — | HXC-084 — challenge scripts use GNU-only grep -P backslash-K — breaks on macOS BSD grep |
| 199 | — | Fixed (→ Fixed.md) | Bug | — | HXC-084 — HXC-084: challenge scripts use GNU-only grep -P backslash-K — breaks on macOS BSD grep |
| 200 | High | Fixed (→ Fixed.md) | Bug | — | HXC-085 — 14 LLM providers HealthCheck hardcodes production URL ignoring injected baseURL |
| 201 | — | Fixed (→ Fixed.md) | Bug | — | HXC-085 — HXC-085: 14 LLM providers HealthCheck hardcodes production URL ignoring injected baseURL |
| 202 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-086 — SSE broker client-ID UnixNano collision under concurrent connect |
| 203 | — | Fixed (→ Fixed.md) | Bug | — | HXC-086 — HXC-086: SSE broker client-ID UnixNano collision under concurrent connect |
| 204 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-087 — skill_registry randomString UnixNano same-tick produces identical chars and colliding IDs |
| 205 | — | Fixed (→ Fixed.md) | Bug | — | HXC-087 — HXC-087: skill_registry randomString UnixNano same-tick produces identical chars and colliding IDs |
| 206 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-088 — llm_orchestrator opencode cancel path hangs 30s — cmd.WaitDelay unset |
| 207 | — | Fixed (→ Fixed.md) | Bug | — | HXC-088 — HXC-088: llm_orchestrator opencode cancel path hangs 30s — cmd.WaitDelay unset |
| 208 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-089 — panoptic web Element infinite-retry hang plus recorder zero-frames |
| 209 | — | Fixed (→ Fixed.md) | Bug | — | HXC-089 — HXC-089: panoptic web Element infinite-retry hang plus recorder zero-frames |
| 210 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-090 — panoptic tracks test-generated audit.json users.json (CONST-053 hygiene) |
| 211 | — | Fixed (→ Fixed.md) | Bug | — | HXC-090 — HXC-090: panoptic tracks test-generated audit.json users.json (CONST-053 hygiene) |
| 212 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-091 — containers custom health-check duration can be 0 (timer-resolution flake) |
| 213 | — | Fixed (→ Fixed.md) | Bug | — | HXC-091 — HXC-091: containers custom health-check duration can be 0 (timer-resolution flake) |
| 214 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-092 — debate_orchestrator 30s DefaultTimeout too short for capable models on multi-round /specify |
| 215 | — | Fixed (→ Fixed.md) | Bug | — | HXC-092 — HXC-092: debate_orchestrator 30s DefaultTimeout too short for capable models on multi-round /specify |
| 216 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-093 — helix_code module graph has phantom digital.vasic.* requires + private transitive blocking go list -m all / gomobile bind |
| 217 | — | Fixed (→ Fixed.md) | Bug | — | HXC-093 — HXC-093: helix_code module graph has phantom digital.vasic.* requires + private transitive blocking go list -m all / gomobile bind |
| 218 | High | Implemented (→ Fixed.md) | Feature | — | HXC-094 — F12 workspace checkpoints — file snapshot + restore/undo safety net |
| 219 | — | Implemented (→ Fixed.md) | Feature | — | HXC-094 — HXC-094: F12 workspace checkpoints — file snapshot + restore/undo safety net |
| 220 | High | Fixed (→ Fixed.md) | Bug | — | HXC-095 — CLI binary generate/debate/specify return 404 against live local ollama |
| 221 | — | Fixed (→ Fixed.md) | Bug | — | HXC-095 — HXC-095: CLI binary generate/debate/specify return 404 against live local ollama |
| 222 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-096 — desktop nogui prints raw i18n keys + %!(EXTRA) format mismatch in status/help |
| 223 | — | Fixed (→ Fixed.md) | Bug | — | HXC-096 — HXC-096: desktop nogui prints raw i18n keys + %!(EXTRA) format mismatch in status/help |
| 224 | High | Fixed (→ Fixed.md) | Bug | — | HXC-097 — SYSTEMIC: standalone binaries + internal/config + internal/database never wire i18n Translator -> raw keys at runtime |
| 225 | — | Fixed (→ Fixed.md) | Bug | — | HXC-097 — HXC-097: SYSTEMIC: standalone binaries + internal/config + internal/database never wire i18n Translator -> raw keys at runtime |
| 226 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-098 — out-of-box config fails 'version required' validation — blocks client status/system commands |
| 227 | — | Fixed (→ Fixed.md) | Bug | — | HXC-098 — HXC-098: out-of-box config fails 'version required' validation — blocks client status/system commands |
| 228 | — | Completed (→ Fixed.md) | Task | — | HXC-099 — Systemic i18n raw-key sweep redo (CONST-046) — corrected, regression-free, with default-translator contract decision |
| 229 | — | Completed (→ Fixed.md) | Task | — | HXC-099 — HXC-099: Systemic i18n raw-key sweep redo (CONST-046) — corrected, regression-free, with default-translator contract decision |
| 230 | — | Completed (→ Fixed.md) | Task | — | HXC-100 — Resync docs/CONTINUATION.md to current HEAD + de-bloat the 32k-token line-1 header (CONST-044/§12.10 + CONST-064 hygiene) |
| 231 | — | Completed (→ Fixed.md) | Task | — | HXC-100 — HXC-100: Resync docs/CONTINUATION.md to current HEAD + de-bloat the 32k-token line-1 header (CONST-044/§12.10 + CONST-064 hygiene) |
| 232 | — | Fixed (→ Fixed.md) | Bug | — | HXC-101 — security/security_test.go TestTLSConfiguration — external-network dependency + nil-deref panic crashes the whole security test binary |
| 233 | — | Fixed (→ Fixed.md) | Bug | — | HXC-101 — HXC-101: security/security_test.go TestTLSConfiguration — external-network dependency + nil-deref panic crashes the whole security test binary |
| 234 | — | Fixed (→ Fixed.md) | Bug | — | HXC-102 — harmony_os main_nogui.go — 2 user-facing strings ('Goodbye!', 'Error: %v') bypass i18n (CONST-046, low severity) |
| 235 | — | Fixed (→ Fixed.md) | Bug | — | HXC-102 — HXC-102: harmony_os main_nogui.go — 2 user-facing strings ('Goodbye!', 'Error: %v') bypass i18n (CONST-046, low severity) |
| 236 | — | Completed (→ Fixed.md) | Task | — | HXC-103 — Web-client runtime e2e proof — live browser/HTTP -> server -> LLM provider round-trip for /api/v1/llm/generate + /llm/stream (CONTINUATION honest gap) |
| 237 | — | Completed (→ Fixed.md) | Task | — | HXC-103 — HXC-103: Web-client runtime e2e proof — live browser/HTTP -> server -> LLM provider round-trip for /api/v1/llm/generate + /llm/stream (CONTINUATION honest gap) |
| 238 | — | Fixed (→ Fixed.md) | Bug | — | HXC-104 — streamLLM /api/v1/llm/stream hangs forever — chunkChan never closed, [DONE] never emitted (production defect found by web e2e) |
| 239 | — | Fixed (→ Fixed.md) | Bug | — | HXC-104 — HXC-104: streamLLM /api/v1/llm/stream hangs forever — chunkChan never closed, [DONE] never emitted (production defect found by web e2e) |
| 240 | — | Completed (→ Fixed.md) | Task | — | HXC-105 — Runtime e2e for server POST /api/v1/specify — boot server -> real spec output via live provider (speckit HTTP-endpoint gap) |
| 241 | — | Completed (→ Fixed.md) | Task | — | HXC-105 — HXC-105: Runtime e2e for server POST /api/v1/specify — boot server -> real spec output via live provider (speckit HTTP-endpoint gap) |
| 242 | — | Completed (→ Fixed.md) | Task | — | HXC-106 — helix_agent durable memory: process-lifetime in-memory fallback is NOT disk-durable — recall lost on restart (CONTINUATION honest gap) |
| 243 | — | Completed (→ Fixed.md) | Task | — | HXC-106 — HXC-106: helix_agent durable memory: process-lifetime in-memory fallback is NOT disk-durable — recall lost on restart (CONTINUATION honest gap) |
| 244 | — | Completed (→ Fixed.md) | Task | — | HXC-107 — Feature Status docs program (docs/features) — comprehensive per-feature inventory across all components/clients/submodules/ported-cli_agents, docs_chain-synced |
| 245 | — | Completed (→ Fixed.md) | Task | — | HXC-108 — Video-QA program: record all clients x all features with strongest models + ensemble -> /Volumes/T7/Downloads/Recordings, analyze + fix |
| 246 | — | Fixed (→ Fixed.md) | Bug | — | HXC-109 — Mobile apps are scaffolds — Android has no build.gradle/AndroidManifest, iOS has no Xcode project (not buildable -> not recordable) |
| 247 | — | Fixed (→ Fixed.md) | Bug | — | HXC-109 — HXC-109: Mobile apps are scaffolds — Android has no build.gradle/AndroidManifest, iOS has no Xcode project (not buildable -> not recordable) |
| 248 | — | Completed (→ Fixed.md) | Task | — | HXC-110 — Extend containers submodule to launch iOS simulators (operator-directed Apple-support mechanism) |
| 249 | — | Completed (→ Fixed.md) | Task | — | HXC-110 — HXC-110: Extend containers submodule to launch iOS simulators (operator-directed Apple-support mechanism) |
| 250 | — | Fixed (→ Fixed.md) | Bug | — | HXC-111 — Desktop GUI shows raw i18n keys (desktop_dashboard_header/_activity_title) — CONST-046 gap |
| 251 | — | Fixed (→ Fixed.md) | Bug | — | HXC-111 — HXC-111: Desktop GUI shows raw i18n keys (desktop_dashboard_header/_activity_title) — CONST-046 gap |
| 252 | — | Completed (→ Fixed.md) | Task | — | HXC-112 — Desktop GUI feature-recording: Fyne OpenGL canvas ignores osascript synthetic clicks — need cliclick/real-event automation to record LLM-chat in-GUI |
| 253 | — | Fixed (→ Fixed.md) | Bug | — | HXC-113 — MCP tool names use 'server:name' (colon) — OpenAI-compatible providers (DeepSeek/etc.) reject function names, breaking LLM chat with MCP enabled |
| 254 | — | Fixed (→ Fixed.md) | Bug | — | HXC-113 — HXC-113: MCP tool names use 'server:name' (colon) — OpenAI-compatible providers (DeepSeek/etc.) reject function names, breaking LLM chat with MCP enabled |
| 255 | Critical (production/release blocker — unauthenticated real, paid LLM-provider generation, reachable on every interface per every shipped config's server.address: "0.0.0.0") | Fixed (→ Fixed.md) | Bug | — | HXC-114 — Wire facade (/v1/chat/completions, /v1/messages) had NO authentication — unauthenticated, paid-provider-consuming surface reachable on 0.0.0.0 |
| 256 | High | Fixed (→ Fixed.md) | Bug | — | HXC-115 — The automated check that scans the codebase for forbidden hardcoded user-facing text stores its list of known-good files using full disk paths tied to one specific machine. On any other computer or checkout location those paths no longer match, so the check wrongly flags every file as a brand-new violation and exits with an error. This silently disables a governance safety-net everywhere except its original machine. The fix stores the known-good list using paths relative to the project folder so the check works anywhere. This restores real enforcement for every developer and automated run. |
| 257 | Medium | Completed (→ Fixed.md) | Task | — | HXC-116 — The multi-track development system's configuration file and its command-line tool both direct readers to an operating guide that does not exist in the repository. Anyone trying to bring up or operate the parallel development tracks has no authoritative instructions. This risks mistakes with the shared storage drives. The task is to write the missing guide covering the track layout, how to start tracks safely, and the storage-drive precautions. Operators can then run the multi-track system correctly from documented steps. |
| 258 | High | Fixed (→ Fixed.md) | Bug | — | HXC-117 — Governance rule CONST-040 requires that every advanced capability a model supports be reported by the central verifier component rather than hardcoded. Today the verifier only records whether a model supports embeddings; the other six capabilities are documented as verifier-sourced but are not implemented there. As a result the product cannot truthfully tell users which models support which capabilities. The work adds these capability fields to the verifier's results and has the product read them from there. Users then receive accurate, single-source-of-truth capability information. |
| 259 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-120 — A safety test for the web server checked that cross-origin browser requests are answered with a permissive wildcard allowing any website to call the API. The server was correctly hardened to allow only an explicit list of trusted sites, so the old test failed and encoded an insecure expectation. The fix updates the test to verify the secure behavior (only listed sites allowed, others refused) without reintroducing the wildcard. The test suite now protects the secure behavior instead of demanding an insecure one. |
| 260 | Medium | Completed (→ Fixed.md) | Task | — | HXC-121 — The code connecting the platform to the HuggingFace and Together AI model services shipped with no automated tests. A future change could silently break how requests are built or responses parsed and nothing would catch it. The work adds real tests exercising the actual client code against a simulated provider endpoint, checking the outgoing request, the parsed reply, and error handling. These two integrations become protected against silent regressions. |
| 261 | Low | Completed (→ Fixed.md) | Task | — | HXC-123 — The command-line security-scan helper ships without automated tests covering its behavior, so bugs could go unnoticed until a real scan produces wrong results. The work adds tests verifying the tool's core scanning and reporting logic. The security-scan tool then becomes protected against silent breakage. |
| 262 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-124 — One automated quality-assurance test bank that drives real server workflows has a documented gap: it cannot mint the authentication token needed to finish the workflow end-to-end without manual help, so it does not meet the fully-automated no-human-intervention standard. The work provides an automated way to obtain a valid token during the test so the workflow runs unattended. The QA bank then becomes fully automated and re-runnable. |
| 263 | Low | Completed (→ Fixed.md) | Task | — | HXC-125 — A large set of integration tests is hidden behind a build tag, so an ordinary test run never compiles or executes them and their pass/fail signal is absent from routine checks. They pass when the tag is supplied, so this is a visibility gap, not a broken feature. The work makes the standard test command or a documented target include them so their status is always visible. Everyday test results then reflect integration coverage. |
| 264 | Low | Completed (→ Fixed.md) | Task | — | HXC-127 — When an item is retired as obsolete, governance requires a recorded reason, date, and superseding reference, but the table holding these details is completely empty, including for the one currently obsolete item. This leaves retirements unexplained and non-compliant. The work populates the required justification details for obsolete items so every retired item carries an auditable reason. |
| 265 | Low | Completed (→ Fixed.md) | Task | — | HXC-128 — Thirty-six tracked items have descriptions shorter than the minimum length needed to convey what they are about, so a reader cannot understand them without reading code. The work rewrites these into clear plain-language descriptions explaining what each item is and why it matters. Every item then becomes understandable to non-developers and stakeholders. |
| 266 | Low | Completed (→ Fixed.md) | Task | — | HXC-129 — Seventy-nine finished feature items have no severity recorded, so risk-ordered validation and reporting cannot rank them by importance. The work backfills an appropriate severity for each item based on its content and impact. Risk-based ordering and reporting then work correctly across the full item set. |
| 267 | Medium | Completed (→ Fixed.md) | Task | — | HXC-130 — A full build fails on a clean machine because the desktop and mobile graphical apps need system graphics libraries (X11 and OpenGL) that are neither installed nor documented, so newcomers hit a confusing build error with no guidance. The work documents the exact system packages required, the command to install them, and the headless no-graphics build path used for development and continuous integration. Anyone can then build the project or knowingly choose the headless path without surprise failures. |
| 268 | Medium | Obsolete (→ Fixed.md) | Bug | — | HXC-131 — The client that talks to the Cognee memory service stopped completing its login and caching the access token — its tests show the login endpoint is never called and no bearer token is stored, so authenticated calls would fail. This means memory features that rely on Cognee cannot authenticate reliably. The work is to restore the login-then-cache-token flow and prove it with the existing auth tests. Users regain dependable access to Cognee-backed memory. |
| 269 | Medium | Completed (→ Fixed.md) | Task | — | HXC-132 — The full-infrastructure test stack fails to build the HelixCode server container because its build recipe points at a Dockerfile path that does not exist, and many memory and security tests skip themselves because they look for a server on a fixed port 8080 with no way to override it. As a result a whole class of tests never run against a real server. The work is to fix the container build path and make the server URL configurable so those tests execute. This unlocks real end-to-end coverage of server-dependent features. |
| 270 | Low | Fixed (→ Fixed.md) | Bug | — | HXC-133 — One Azure-provider test quietly depends on an endpoint value being present in the environment; when that value is injected by the full-test setup the test's assumptions no longer hold. It does not crash, but it is fragile and can give misleading results depending on the environment. The work is to make the test hermetic (control its own environment) like the sibling test already fixed. This makes the Azure test suite reliable regardless of how it is run. |
| 271 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-134 — The central model-verifier service reports each model's id as a numeric value, while HelixCode expects the id as text — a type mismatch that can break how verified models are matched and displayed. The work is to align the two so the id is consistently text end to end. Correct model identity keeps verification, listing, and status accurate for users. |
| 272 | Medium | Implemented (→ Fixed.md) | Feature | — | HXC-135 — HelixCode is now wired to read six advanced capability indicators (tool protocols, code intelligence, retrieval, skills, plugins) from the central verifier, but the verifier's live responses do not yet include those fields, so the flags always read as unsupported. The work is to have the verifier publish these capability values it already computes. Then users see accurate per-model capability information across the product. |
| 273 | Medium | Completed (→ Fixed.md) | Task | — | HXC-137 — The project depends on many owned code modules, and a full health check of all of them (does each build, pass static checks, and pass its tests) did not finish in the latest session. The work is to run that complete health sweep and record the result for every module. This assures that the whole codebase, not just the main application, is in good shape. |
| 274 | High | Fixed (→ Fixed.md) | Bug | — | HXC-139 — A vendored copy of a third-party reference coding-agent (the Continue project) includes a Go source file that imports a path that does not exist, and because that file has no separate module marker it gets swept into the helix_agent module's build — breaking the build and static checks for the whole module. This blocks reliable building and testing of the agent module. The work is to isolate those vendored reference files so they are not compiled as part of our module (a build-ignore or nested module marker). Developers regain a clean, buildable agent module. |
| 275 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-140 — The quality-assurance module has code that copies a value containing a lock (a mutex) instead of sharing it, which the Go checker flags as unsafe and can cause subtle concurrency bugs; separately, one test that loads real test banks is failing. The work is to pass the lock-bearing value by reference (pointer) instead of copying it, and to fix or reconcile the failing test-bank test. This makes the QA module concurrency-safe and its tests green. |
| 276 | Medium | Fixed (→ Fixed.md) | Bug | — | HXC-141 — The MCP module's Docker adapter crashes with a null-pointer error when asked to stop a container that was never started or does not exist, instead of returning cleanly. This can bring down callers that expect a safe no-op. The work is to guard the stop path so a not-started or missing container is handled gracefully. The adapter becomes robust against stop-before-start and missing-container situations. |
| 277 | — | Fixed (→ Fixed.md) | Bug | — | HXL-001 — HXL-001 (ex-ISSUE-003): HelixLLM analysis_test.go hardcoded path |
| 278 | — | Fixed (→ Fixed.md) | Bug | — | HXL-002 — HXL-002 (ex-ISSUE-004): HelixLLM TOON WriteTOON 500 |
| 279 | — | Fixed (→ Fixed.md) | Bug | — | HXQ-001 — HXQ-001 (ex-ISSUE-008): helix_qa intermittent `TestPerformance` flake (host-load-sensitive) |
| 280 | — | Fixed (→ Fixed.md) | Bug | — | HXQ-002 — HXQ-002: helix_qa `pkg/autonomous` ↔ VisionEngine `remote` API drift blocks helix_agent `tests/integration` compile |
| 281 | — | Fixed (→ Fixed.md) | Bug | — | HXV-001 — HXV-001: LLMsVerifier 18 pre-existing `tests/` failures (CLI-integration + verification/scoring) |
| 282 | — | Fixed (→ Fixed.md) | Bug | — | HXV-002 — HXV-002: LLMsVerifier `verification/` package 10 pre-existing test failures |
| 283 | — | Fixed (→ Fixed.md) | Bug | — | HXV-003 — HXV-003: LLMsVerifier `ProviderAdapterForBenchmark.Complete` is a CONST-050(A) production mock-bluff |
| 284 | — | Fixed (→ Fixed.md) | Bug | — | OPS-001 — OPS-001: LLMOps 2 pre-existing `CreatePromptExperiment` test failures |
| 285 | — | Fixed (→ Fixed.md) | Bug | — | PAN-001 — PAN-001: panoptic `appendJSONString` truncates multi-byte UTF-8 runes to bytes (`TestResult.MarshalJSON` corrupts non-ASCII) |
| 286 | — | Completed (→ Fixed.md) | Task | — | VEN-001 — VEN-001 (ex-ISSUE-001): VisionEngine `helix-gitlab` URL fix (was misconfigured, not missing) |
| 287 | — | Fixed (→ Fixed.md) | Bug | — | VEN-002 — VEN-002 (ex-ISSUE-002): VisionEngine `vasic-digital-github` fork lineage divergent at SHA 93c830a |
| 288 | — | Fixed (→ Fixed.md) | Bug | — | VEN-002#1 — VEN-002 (ex-ISSUE-002): VisionEngine `vasic-digital-github` fork lineage divergent at SHA 93c830a |
