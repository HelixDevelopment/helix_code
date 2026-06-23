# Issues_Summary

Open workable items (current_location = Issues), regenerated from the SQLite single-source-of-truth (§11.4.12).

## Counts by Type × Status

| Type | Status | Count |
|---|---|---|
| Bug | Fixed (→ Fixed.md) | 19 |
| Feature | Implemented (→ Fixed.md) | 3 |
| Task | Completed (→ Fixed.md) | 18 |
| Task | Fixed (→ Fixed.md) | 1 |
| Task | Operator-blocked | 3 |
| **TOTAL** | | **44** |

## Items

| ATM ID | Type | Status | Severity | Description |
|---|---|---|---|---|
| HXA-001 | Bug | Fixed (→ Fixed.md) | — | helix_agent handler tests surfaced after round-109 fix |
| HXA-002 | Bug | Fixed (→ Fixed.md) | — | helix_agent debate/llmprovider sibling-submodule API drift |
| HXA-003 | Bug | Fixed (→ Fixed.md) | — | venice `TestGetCapabilities` model-list drift (CONST-037) |
| HXC-001 | Task | Completed (→ Fixed.md) | — | CONST-052 rename programme: meta-repo directories still PascalCase — CLOSED (→ Fixed.md) |
| HXC-002 | Bug | Fixed (→ Fixed.md) | — | Round-74 residual LOGIC-class FAILs (CLOSED) |
| HXC-003 | Feature | Implemented (→ Fixed.md) | — | CONST-046 i18n migration backlog — CLOSED (migrated to docs/Fixed.md) |
| HXC-004 | Bug | Fixed (→ Fixed.md) | — | Recovery-batch under-verification (40% FAIL rate per round-193 audit) |
| HXC-005 | Bug | Fixed (→ Fixed.md) | — | `cmd/performance_optimization_standalone/main.go` is a CONST-035 simulation bluff |
| HXC-006 | Feature | Implemented (→ Fixed.md) | — | HelixCode Speed Programme — CLOSED (migrated to docs/Fixed.md) |
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
| HXC-107 | Task | Operator-blocked | — | Feature Status docs program (docs/features) — comprehensive per-feature inventory across all components/clients/submodules/ported-cli_agents, docs_chain-synced |
| HXC-108 | Task | Operator-blocked | — | Video-QA program: record all clients x all features with strongest models + ensemble -> /Volumes/T7/Downloads/Recordings, analyze + fix |
| HXC-112 | Task | Operator-blocked | — | Desktop GUI feature-recording: Fyne OpenGL canvas ignores osascript synthetic clicks — need cliclick/real-event automation to record LLM-chat in-GUI |
| HXL-001 | Bug | Fixed (→ Fixed.md) | — | HelixLLM `internal/agents/tools/analysis_test.go` hardcoded absolute path |
| HXL-002 | Bug | Fixed (→ Fixed.md) | — | HelixLLM `internal/gateway/middleware` TOON `WriteTOON` returns 500 |
| HXQ-001 | Bug | Fixed (→ Fixed.md) | — | helix_qa intermittent TestPerformance flake (host-load-sensitive) |
| HXV-002 | Bug | Fixed (→ Fixed.md) | — | LLMsVerifier `verification/` package 10 pre-existing test failures |
| PAN-001 | Bug | Fixed (→ Fixed.md) | — | panoptic `appendJSONString` truncates multi-byte UTF-8 runes to bytes (`TestResult.MarshalJSON` corrupts non-ASCII) |
| VEN-001 | Task | Completed (→ Fixed.md) | — | VisionEngine `helix-gitlab` remote repo missing (404) — CLOSED (→ Fixed.md) |
