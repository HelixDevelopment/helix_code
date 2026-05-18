# HelixCode — Open Issues Tracker

> Per Constitution §11.4.15 (Item-status tracking) + §11.4.16 (Item-type tracking) + §11.4.19 (Fixed-document column-alignment) + CONST-057 (Type-aware closure vocabulary) + CONST-058 (Reopened-source attribution).
>
> **Authoritative resumption ledger**: `docs/CONTINUATION.md` (CONST-044). This file complements it with item-level granularity for currently-open work.
>
> **Status vocabulary** (closed set): `Queued` | `In progress` | `Ready for testing` | `In testing` | `Reopened` | `Fixed/Implemented/Completed (→ Fixed.md)`
>
> **Type vocabulary** (closed set): `Bug` | `Feature` | `Task`

---

## ISSUE-001 — VisionEngine `helix-gitlab` remote repo missing (404)

**Status:** Queued — BLOCKED on operator (gitlab repo creation)
**Type:** Task
**Discovered:** 2026-05-19 (round 98 — Planning + VisionEngine i18n migration)
**Discovered-By:** AI subagent during 4-remote push attempt
**Evidence:** Push attempt against `git@gitlab.com:HelixDevelopment/visionengine.git` returned `404 not found`. CONST-043 honoured (no force-push attempted).
**Resolution path:** Operator creates the gitlab.com repository OR removes the `helix-gitlab` named remote from VisionEngine's local config. Pre-existing infra gap, not introduced by round-98 work.

---

## ISSUE-002 — VisionEngine `vasic-digital-github` fork lineage divergent at SHA 93c830a

**Status:** Queued — BLOCKED on operator (CONST-061 merge-first investigation)
**Type:** Bug
**Discovered:** 2026-05-19 (round 98)
**Discovered-By:** AI subagent during 4-remote push attempt
**Evidence:** vasic-digital-github HEAD `93c830a` carries round-48/52/57 commits absent from HelixDevelopment local main. Non-FF push rejected. NO force-push attempted (CONST-043).
**Resolution path:** Operator-led CONST-061 merge-first pipeline — fetch divergent commits, audit conflict surface, integrate or document divergence as intentional fork, then either FF-push or designate one lineage as canonical.

---

## ISSUE-003 — HelixLLM `internal/agents/tools/analysis_test.go` hardcoded absolute path

**Status:** Fixed (→ Fixed.md)
**Type:** Bug
**Discovered:** 2026-05-19 (round 95 — HelixLLM migration; surfaced as pre-existing failure)
**Discovered-By:** AI subagent during HelixLLM standalone test run
**Closed-By:** Round 105 (commit `a5e56d4` in HelixLLM; meta pointer `fedd152`)
**Attribution correction:** Originally documented as helix_agent; actual location is HelixLLM submodule (`dependencies/HelixDevelopment/HelixLLM/internal/agents/tools/`). Commit SHAs `0a84310` resolved there.
**Resolution:** Replaced hardcoded path with `t.TempDir()` + 2 synthesised fixture files. Bonus: same bug-pattern discovered in `git_test.go` (constant `helixLLMRoot` + 7 tests) — refactored `gitSandbox()` signature. 6 tests now PASS on any host. Mutation verified.

---

## ISSUE-004 — HelixLLM `internal/gateway/middleware` TOON `WriteTOON` returns 500

**Status:** Fixed (→ Fixed.md)
**Type:** Bug
**Discovered:** 2026-05-19 (round 95)
**Discovered-By:** AI subagent
**Closed-By:** Round 105 (commit `a5e56d4`)
**Attribution correction:** Originally documented as helix_agent; actual location is HelixLLM submodule. Commit `6f11c56` resolved there.
**Resolution:** Root cause was vasic-digital/TOON's round-27 anti-bluff change (Marshal returns `ErrTOONEncodingNotImplemented` unconditionally) combined with `WriteTOON` treating ANY Marshal error as 500. Fix: fall back to `json.Marshal` while preserving `application/toon` Content-Type (matches ContentNegotiation middleware). 500 still returned for genuinely unmarshallable values (channels). 19 middleware tests now PASS. Mutation verified.

---

## ISSUE-005 — CONST-052 rename programme: meta-repo directories still PascalCase

**Status:** Queued
**Type:** Task
**Discovered:** 2026-05-15 (CONST-052 cascade landed)
**Discovered-By:** Constitution
**Evidence:** Meta-repo directories like `helix_code/`, `challenges/`, `helix_qa/`, `helix_agent/` still PascalCase despite CONST-052 mandating snake_case. Renames deferred because they break path-encoded references throughout governance docs, CI scripts, and tracker URLs.
**Resolution path:** Phased migration per CONST-052 §11.4.29: comprehensive brainstorming → phase-divided plan → fine-grained tasks → every change covered by every applicable test type. Round 88 made partial progress (3 submodules with drift fixed) but root directories remain.

---

## ISSUE-006 — Round-74 residual LOGIC-class FAILs (CLOSED)

**Status:** Fixed (→ Fixed.md)
**Type:** Bug
**Discovered:** 2026-05-19 (round 74 — release-gate-test.sh creation; classified by round 89)
**Discovered-By:** AI release-gate sweep
**Closure progress:**
- ✓ HelixMemory: closed round 106 (commit `69016df` — single-line `go.mod` fix; 6 FAIL → 0 FAIL)
- ✓ vasic-digital/Planning: round 107 NO-OP — 275 PASS / 0 FAIL / 20 SKIP-OK; likely incidentally fixed by round 98 i18n migration
- ✓ helix_agent inner: closed round 109 (commit `0f492e98` — 5 test-side bluff fixes, zero production changes)
**Evidence:** Round 74 surfaced 26 FAILs across submodules; rounds 82-87 closed 19; this Issue tracked the residual 7 across 3 submodules. All 3 components closed by rounds 106 + 107 + 109.
**Follow-ups surfaced (NEW issues to file)**: 4 helix_agent handler tests previously masked by mid-run panic (now visible) + 3 build-failed packages depending on sibling submodule API drift (`digital.vasic.debate`) + 2 LOGIC FAILs reclassified as cross-cutting work (venice CONST-037 model-wiring + compliance CONST-051 architectural reconciliation). See ISSUE-009 through ISSUE-013 (TBD next sweep).

---

## ISSUE-009 — helix_agent handler tests surfaced after round-109 fix

**Status:** Queued
**Type:** Bug
**Discovered:** 2026-05-19 (round 109)
**Discovered-By:** AI subagent (helix_agent LOGIC audit)
**Evidence:** Mid-run panic in `TestIsProviderAvailable_NotAvailable` aborted test binary; round 109's fix unblocked execution, surfacing 4 pre-existing FAILs: `TestFormattersHandler_FormatCode_UnsupportedLanguage`, `TestEmbeddingHandler_WithRealManager`, `TestGetTaskResources`, `TestGetTaskLogs`. Out of round-109's 5-fix cap.
**Resolution path:** Per-handler investigation, similar to round 109's test-side bluff pattern.

---

## ISSUE-010 — helix_agent 3 build-failed packages (sibling submodule API drift)

**Status:** Queued — BLOCKED on cross-submodule coordination
**Type:** Bug
**Discovered:** 2026-05-19 (round 109)
**Discovered-By:** AI subagent
**Evidence:** 3 packages in helix_agent depend on `digital.vasic.debate` API surface that changed; build fails with type/method mismatches. Pre-existing.
**Resolution path:** Either rebuild the consuming code to new debate API OR pin older debate version in helix_agent go.mod. Cross-submodule coordination required.

---

## ISSUE-011 — venice `TestGetCapabilities` model-list drift (CONST-037)

**Status:** Queued — needs CONST-037 canonical-model wiring
**Type:** Bug
**Discovered:** 2026-05-19 (round 109)
**Discovered-By:** AI subagent
**Evidence:** Test hardcodes `venice-uncensored` as expected model; Venice API no longer returns it. CONST-037 mandates LLMsVerifier as single source of truth for model metadata.
**Resolution path:** Replace hardcoded expectation with LLMsVerifier dynamic lookup OR pin against a stable model that's not subject to vendor list changes.

---

## ISSUE-007 — CONST-046 migration backlog (57,329 violations baselined; shrinking)

**Status:** In progress
**Type:** Feature
**Discovered:** 2026-05-19 (round 92 — audit script)
**Discovered-By:** AI subagent ground-truth scan
**Evidence:** Round-92 scan reported 57,345 violations across 21,937 files. Round 99b baseline collapsed to 54,803 unique `(path, literal_hash)` keys. Phase 4 (rounds 100+) systematically migrating top-concentration files: round 100 (evaluators.go), 101 (challenge_recorded_ai_testgen.go), 102 (challenge_desktop.go) — see CONTINUATION.md close-outs.
**Resolution path:** Continued Phase 4 cadence; audit-gate `--fail-on-new` already enforced; each migration round MUST re-run `--update-baseline` so snapshot shrinks toward zero.

---

## ISSUE-008 — helix_qa intermittent TestPerformance flake (host-load-sensitive)

**Status:** Queued — BLOCKED on operator (host topology decision)
**Type:** Bug
**Discovered:** 2026-05-19 (round 82)
**Discovered-By:** AI subagent
**Evidence:** helix_qa TestPerformance fails intermittently under high host load (concurrent containers + builds). Not a code bug per se; a sensitivity issue.
**Resolution path:** Either (a) loosen timing tolerance with explicit comment + reference to host topology, or (b) gate the test behind a `HOST_LOAD_DEDICATED=1` env var to run only on quiescent hosts. Operator decision needed.

---

*Last regenerated: 2026-05-19. To update Issues_Summary.md mechanically, run `scripts/generate_issues_summary.sh` (TODO: create — currently this Issues.md is the source of truth and Summary is hand-maintained).*
