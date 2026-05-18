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

## ISSUE-003 — helix_agent `internal/agents/tools/analysis_test.go` hardcoded absolute path

**Status:** Queued
**Type:** Bug
**Discovered:** 2026-05-19 (round 95 — HelixLLM migration; surfaced as pre-existing failure)
**Discovered-By:** AI subagent during HelixLLM standalone test run
**Evidence:** Test references path `/run/media/milosvasic/DATA4TB/Projects/helix_agent/HelixLLM/...` that does not exist; introduced commit `0a84310`.
**Resolution path:** Replace hardcoded path with `t.TempDir()` or `testing.TB.Helper()` parametrization. Test bluff per CONST-035 (passes locally only on a host with that exact directory tree).

---

## ISSUE-004 — helix_agent `internal/gateway/middleware` TOON negotiation_test.go returns 500

**Status:** Queued
**Type:** Bug
**Discovered:** 2026-05-19 (round 95)
**Discovered-By:** AI subagent
**Evidence:** Negotiation handler returns HTTP 500 instead of expected status; introduced commit `6f11c56`. Test currently asserts incorrect expected status OR handler is broken.
**Resolution path:** Investigate whether bug is in test expectation or handler logic. Tighten test per CONST-035.

---

## ISSUE-005 — CONST-052 rename programme: meta-repo directories still PascalCase

**Status:** Queued
**Type:** Task
**Discovered:** 2026-05-15 (CONST-052 cascade landed)
**Discovered-By:** Constitution
**Evidence:** Meta-repo directories like `helix_code/`, `challenges/`, `helix_qa/`, `helix_agent/` still PascalCase despite CONST-052 mandating snake_case. Renames deferred because they break path-encoded references throughout governance docs, CI scripts, and tracker URLs.
**Resolution path:** Phased migration per CONST-052 §11.4.29: comprehensive brainstorming → phase-divided plan → fine-grained tasks → every change covered by every applicable test type. Round 88 made partial progress (3 submodules with drift fixed) but root directories remain.

---

## ISSUE-006 — Round-74 environmental-class FAILs not yet classified by audit-gate filter

**Status:** Queued
**Type:** Bug
**Discovered:** 2026-05-19 (round 74 — release-gate-test.sh creation; classified by round 89)
**Discovered-By:** AI release-gate sweep
**Evidence:** Round 74 surfaced 26 FAILs across submodules; rounds 82-87 closed 19; remaining ~7 are HelixMemory + vasic-digital/Planning + helix_agent inner test bugs. Round 89's `--skip-env-failures` filter classifies env-vs-logic FAILs but the LOGIC-class residual still needs per-submodule fix-up.
**Resolution path:** Per-submodule investigation; likely small focused fixes similar to rounds 82-87 pattern.

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
