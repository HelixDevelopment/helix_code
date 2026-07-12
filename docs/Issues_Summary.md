# Issues_Summary

Open workable items (current_location = Issues), regenerated from the SQLite single-source-of-truth (§11.4.12).

## Counts by Type × Status

| Type | Status | Count |
|---|---|---|
| Bug | Fixed (→ Fixed.md) | 19 |
| Feature | Implemented (→ Fixed.md) | 3 |
| Feature | Queued | 2 |
| Task | Completed (→ Fixed.md) | 18 |
| Task | Fixed (→ Fixed.md) | 1 |
| Task | Queued | 4 |
| **TOTAL** | | **47** |

## Items

| ATM ID | Type | Status | Severity | Description |
|---|---|---|---|---|
| HXA-001 | Bug | Fixed (→ Fixed.md) | — | helix_agent handler tests surfaced after round-109 fix |
| HXA-002 | Bug | Fixed (→ Fixed.md) | — | helix_agent debate/llmprovider sibling-submodule API drift |
| HXA-003 | Bug | Fixed (→ Fixed.md) | — | venice `TestGetCapabilities` model-list drift (CONST-037) |
| HXC-001 | Task | Completed (→ Fixed.md) | — | CONST-052 rename programme: meta-repo directories still PascalCase — CLOSED (→ Fixed.md) |
| HXC-002 | Bug | Fixed (→ Fixed.md) | — | Round-74 residual LOGIC-class FAILs (CLOSED) |
| HXC-003 | Feature | Implemented (→ Fixed.md) | High | CONST-046 i18n migration backlog — CLOSED (migrated to docs/Fixed.md) |
| HXC-004 | Bug | Fixed (→ Fixed.md) | — | Recovery-batch under-verification (40% FAIL rate per round-193 audit) |
| HXC-005 | Bug | Fixed (→ Fixed.md) | — | `cmd/performance_optimization_standalone/main.go` is a CONST-035 simulation bluff |
| HXC-006 | Feature | Implemented (→ Fixed.md) | High | HelixCode Speed Programme — CLOSED (migrated to docs/Fixed.md) |
| HXC-007 | Task | Completed (→ Fixed.md) | — | Constitution §11.4.68/70-74 cascade + meta-pointer bump — CLOSED (migrated to docs/Fixed.md) |
| HXC-008 | Bug | Fixed (→ Fixed.md) | — | CONST-055 G1 governance gaps surfaced by post-constitution-pull validation sweep — CLOSED (migrated to docs/Fixed.md) |
| HXC-009 | Task | Completed (→ Fixed.md) | — | Owned-submodule GitHub ↔ GitLab mirror-divergence reconciliation — CLOSED (migrated to docs/Fixed.md) |
| HXC-010 | Task | Completed (→ Fixed.md) | — | End-to-end Kimi CLI + Qwen Code CodeGraph verification — CLOSED (migrated to docs/Fixed.md) |
| HXC-013 | Feature | Implemented (→ Fixed.md) | — | Adopt SQLite-backed single-source-of-truth for workable items (§11.4.93/95) |
| HXC-014 | Task | Completed (→ Fixed.md) | — | Stress + chaos test coverage (§11.4.85) |
| HXC-014b | Bug | Fixed (→ Fixed.md) | Medium (latent: requires SetTranslator() concurrent with tr(); existing -race tests pass because SetTranslator is boot-only in practice) | Systemic unguarded i18n translator.go data-race + panic-crash (cross-package) |
| HXC-015 | Task | Completed (→ Fixed.md) | — | Cross-platform parity (§11.4.81) — **Closure (2026-05-28, subagent-driven §11. |
| HXC-016 | Task | Completed (→ Fixed.md) | — | §11.4.69–97 governance cascade into owned submodules (CONST-047/§3) — CLOSED (→ Fixed.md) |
| HXC-017 | Task | Completed (→ Fixed.md) | — | CodeGraph own-org submodule indexing + update automation (§11.4.79/80) — CLOSED (→ Fixed.md) |
| HXC-018 | Task | Completed (→ Fixed.md) | — | Obsolete status (§11.4.90) + summary-doc clarity (§11.4.91) tracker tooling |
| HXC-019 | Task | Completed (→ Fixed.md) | — | docs/qa/ end-user evidence tree (§11.4.83) |
| HXC-022 | Bug | Fixed (→ Fixed.md) | — | test_bank platform + integration packages do not compile (pre-existing) — CLOSED (→ Fixed.md) |
| HXC-023 | Bug | Fixed (→ Fixed.md) | — | `Assert(true,…)` / `AssertTrue(true,…)` literal-true bluffs across test_bank — CLOSED (→ Fixed.md) |
| HXC-024 | Bug | Fixed (→ Fixed.md) | Medium (CONST-050(B): the llm integration suite cannot compile/run; masks integration regressions + blocks the new ollama integration stress/chaos tests from running via the normal path) | internal/llm `-tags=integration` build broken (stale tests reference deleted providers) |
| HXC-025 | Task | Completed (→ Fixed.md) | — | Constitution §11.4.98/99/101 cascade (CONST-047/§3/§11.4.26) |
| HXC-026 | Task | Completed (→ Fixed.md) | — | workable-items md↔db sync gate (§11.4.93/95 follow-up) |
| HXC-027 | Task | Completed (→ Fixed.md) | — | §11.4.98 live-test full-automation compliance audit |
| HXC-028 | Task | Completed (→ Fixed.md) | — | §11.4.99 latest-source documentation cross-reference (README) |
| HXC-029 | Task | Completed (→ Fixed.md) | — | §11.4.98 full-automation compliance sweep of every live/integration/e2e/Challenge test (no human-in-the-loop) — CLOSED (→ Fixed.md) |
| HXC-030 | Task | Completed (→ Fixed.md) | — | §11.4.99 forward: latest-source documentation cross-reference sweep across all operator-facing docs — CLOSED (→ Fixed.md) |
| HXC-032 | Bug | Fixed (→ Fixed.md) | High (breaks `helix_agent` `go build ./...`; a §11.4 PASS-bluff at the build layer — tracked source does not compile) | LLMOrchestrator submodule: committed merge-conflict markers break `helix_agent` build — CLOSED (→ Fixed.md) |
| HXC-033 | Bug | Fixed (→ Fixed.md) | High (§11.4.79 release-blocker — AI agents querying the code-graph get NO own-org submodule symbols; index also unbuildable) | codegraph 0.9.7 update: full index/sync crashes + own-org submodules dropped from the index (§11.4.79 regression) — CLOSED (→ Fixed.md) |
| HXC-034 | Task | Completed (→ Fixed.md) | — | Cascade constitution §11.4.102 into owned submodules + implement CM-COVENANT-114-102-PROPAGATION gate — CLOSED (→ Fixed.md) |
| HXC-035 | Bug | Fixed (→ Fixed.md) | High (no user can register → no JWT mintable → every authenticated API path is undrivable; blocks the positive-path coverage of all 4 HXC-029 API banks) | `POST /api/v1/auth/register` returns 400 `internal_auth_failed_create_user` on the live server (blocks all authenticated flows) — CLOSED (→ Fixed.md) |
| HXC-036 | Task | Fixed (→ Fixed.md) | — | Systemic CONST-046 i18n defect: 74 packages emitted raw message-ID keys because boot-time translator wiring was never implemented — CLOSED (→ Fixed.md) |
| HXC-118 | Feature | Queued | High | A dedicated Retrieval-Augmented-Generation component is maintained as its own reusable module, but the main application does not import or use it anywhere. A capability the product is expected to offer (answering using retrieved documents) is therefore effectively unavailable to end users despite the code existing. The work integrates the existing RAG module into the application, wires it into the request flow, and exposes its capability flag. Users gain working document-grounded answers instead of an orphaned, unused component. |
| HXC-119 | Feature | Queued | High | Governance rule CONST-040 lists the Agent Client Protocol among required capabilities, but there is no implementation of it anywhere in the codebase. Any user or integration expecting ACP connectivity currently cannot use it. The work is to design and implement real ACP support, or, if it proves structurally infeasible, to document that with cited evidence. The platform will then either genuinely support ACP or hold an honest, evidenced position instead of an unmet claim. |
| HXC-122 | Task | Queued | Medium | Two categories of tests, memory-usage and end-to-end automation, skip most of their cases by default because they require a live server or special environment flags not set in normal runs. In practice these areas are largely unverified even though the tests appear to exist. The work provides a documented, repeatable way to run them against real infrastructure so they actually execute and prove the behavior. Memory and automation behavior then becomes genuinely tested rather than merely scaffolded. |
| HXC-126 | Task | Queued | Medium | Eleven work items marked finished still appear in the open-issues document and are missing from the resolved-items document, so the two views disagree about their state and become untrustworthy. The work regenerates the tracker documents from the authoritative database so finished items appear only in the resolved view. The human-readable trackers then accurately reflect the true state. |
| HXC-136 | Task | Queued | Medium | Several mandated automated test categories — load/denial-of-service, scaling, stress and chaos, and user-interface/experience — were not exercised in the latest real-infrastructure run, so their current health is unconfirmed. The work is to run each of these test types against real infrastructure and capture proof of the results. This completes the promised full test-type coverage and confirms the product holds up under load and adverse conditions. |
| HXC-138 | Task | Queued | Low | The end-to-end challenge runner can now launch all its scenarios (a missing option was just fixed), but the scenarios still need to be executed against a live server with a real model to confirm the complete user journeys work. The work is to stand up a server and run the challenges, capturing the results. This provides real proof that the headline user workflows function end to end. |
| HXL-001 | Bug | Fixed (→ Fixed.md) | — | HelixLLM `internal/agents/tools/analysis_test.go` hardcoded absolute path |
| HXL-002 | Bug | Fixed (→ Fixed.md) | — | HelixLLM `internal/gateway/middleware` TOON `WriteTOON` returns 500 |
| HXQ-001 | Bug | Fixed (→ Fixed.md) | — | helix_qa intermittent TestPerformance flake (host-load-sensitive) |
| HXV-002 | Bug | Fixed (→ Fixed.md) | — | LLMsVerifier `verification/` package 10 pre-existing test failures |
| PAN-001 | Bug | Fixed (→ Fixed.md) | — | panoptic `appendJSONString` truncates multi-byte UTF-8 runes to bytes (`TestResult.MarshalJSON` corrupts non-ASCII) |
| VEN-001 | Task | Completed (→ Fixed.md) | — | VisionEngine `helix-gitlab` remote repo missing (404) — CLOSED (→ Fixed.md) |
