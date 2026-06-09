# Documentation Reconciliation Plan — Root `*.md` Sprawl & Rule-9 Over-Claims

**Date:** 2026-06-04
**Author:** Comprehensive-completion programme, round 468 W6E (conductor main-stream, read-only discovery)
**Status:** DISCOVERY + PLAN ONLY — no files deleted/moved by this stream. Execution (mass retirement) is decision-heavy and requires operator sign-off + proper §11.4.90 `Obsolete` classification.
**Method:** read-only filesystem inventory + grep evidence captured this session. Supersedes the stale handoff note "No .mmd/.puml diagram sources" (false — see F5).

---

## 1. Findings (evidence-cited)

### F1 — Root markdown sprawl: 106 `*.md` at repo root
`ls *.md | wc -l` = **106** (vs 18 in `docs/`). Buckets:

| Bucket | ~count | Examples | Disposition |
|---|---|---|---|
| **Governance (canonical — KEEP)** | 6 | `CLAUDE.md`, `AGENTS.md`, `CONSTITUTION.md`, `CRUSH.md`, `QWEN.md`, `README.md` | Keep; peer-sync per §1.1 of CLAUDE.md |
| **Entry/nav (KEEP, reconcile)** | ~6 | `START_HERE.md`, `CONTINUE_HERE.md`, `README_DOCKER.md`, `HELIX_SCRIPT_GUIDE.md`, `DOCKER_SETUP.md`, `QUICK_START_TOMORROW.md` | Keep; point at `docs/CONTINUATION.md` as the single resumption record (CONST-044) |
| **Completion/Success reports (Rule-9 — RETIRE)** | ~22 | see F2 | Archive → `docs/archive/` + §11.4.90 `Obsolete` |
| **Phase/session churn (CONSOLIDATE)** | ~24 | `PHASE_0_*`(5), `PHASE_1_*`(7), `PHASE_2_*`(3), `PHASE_3_*`(3), `Phase{2,4,5}_Implementation_Summary`, `SESSION_*` | Fold into `CONTINUATION.md` history; archive originals |
| **Plans (OVERLAP — consolidate to 1)** | ~13 | `*IMPLEMENTATION_PLAN*`, `*ROADMAP*`, `MASTER_IMPLEMENTATION_PLAN`, `COMPREHENSIVE_COMPLETION_PLAN` | Consolidate into the 2026-06-04 `docs/CONSOLIDATED_*` reports already produced |
| **Analysis/Audit (OVERLAP)** | ~14 | `COMPREHENSIVE_{ANALYSIS,AUDIT}_REPORT`, `*GAP_ANALYSIS*`, `AUDIT_TRACKER_2026`, `SKIPPED_TESTS_ANALYSIS` | Consolidate; keep the newest as reference |
| **Trackers (SUPERSEDED by SQLite SSOT — RETIRE)** | ~6 | `IMPLEMENTATION_TRACKER`, `DETAILED_IMPLEMENTATION_TRACKER`, `COMPLETE_IMPLEMENTATION_TRACKER`, `AUDIT_IMPLEMENTATION_TRACKER`, `APPLICATION_CHALLENGE_STATUS` | See F3 — replaced by `docs/workable_items.db` + `docs/Issues.md`/`Fixed.md` |
| **Reference (cli_agents — KEEP)** | ~10 | `EXAMPLE_PROJECTS_*`, `PLANDEX_*`, `FEATURE_COMPARISON_MATRIX`, `HELIXCODE_QUICK_REFERENCE` | Keep; legit reference material |
| **Design/spec (KEEP, assess accuracy)** | ~5 | `E2E_TEST_SPECIFICATION`, `TESTING_PLAN`, `SECURITY_IMPLEMENTATION`, `PENPOT_INTEGRATION`, `TEAM_DEVELOPMENT_BREAKDOWN` | Keep; verify claims vs reality |

### F2 — Rule-9 / Article XI §11.9 false-success artifacts (HIGH severity)
`grep -icE "100% complete|all tests pass|fully working|production[- ]ready|fully functional|all.*passing|mission accomplished"`:

| File | over-claim phrase hits |
|---|---|
| `FINAL_COMPLETION_SUCCESS_REPORT.md` | **29** |
| `PRODUCTION_DEPLOYMENT_READINESS_REPORT.md` | **24** |
| `IMPLEMENTATION_COMPLETE.md` | **16** |
| `FINAL_COMPLETION_REPORT.md` | **13** |
| `HELIXCODE_COMPLETION_REPORT.md` | 6 |
| `IMPLEMENTATION_SUCCESS.md` | 2 |
| `FINAL_COMPLETION.md`, `PROJECT_COMPLETION_ANALYSIS.md` | 1 each |

These assert "complete / production-ready / all passing" while `docs/CONTINUATION.md` (the CONST-044 authoritative record) shows the comprehensive-completion programme still has W6+ open work, and rounds 468b–468d were fixing **broken-build** modules (doc_processor conflict markers, challenges missing-dep, llms_verifier go.sum) and a committed **secret leak**. They are textbook false-success surface (Rule 9 / §11.4.1).

### F3 — Tracker docs superseded by the SQLite single-source-of-truth
`docs/workable_items.db` **exists** (356 KB) — the §11.4.93/§11.4.95 mandated single source of truth — alongside its generated `docs/Issues.md` (99 KB) + `docs/Fixed.md` (76 KB). The ~6 root `*_TRACKER.md` / `*_STATUS.md` files predate and duplicate this; per §11.4.95 the DB is authoritative. They are retire candidates (risk: drift — a human reads a stale tracker that contradicts the DB).

### F4 — Missing `docs/ARCHITECTURE.md` (gap)
`ls docs/ARCHITECTURE.md ARCHITECTURE.md` → **No such file**. Yet `CLAUDE.md` §11 cites `docs/ARCHITECTURE.md` as the architecture-question authority and §3.4-area docs reference it. `HELIXCODE_ARCHITECTURE_CONSISTENCY_REPORT.md` exists but is a *report*, not the architecture doc. **Action: author `docs/ARCHITECTURE.md`** (the meta-repo + inner `helix_code/` module + submodule topology, sourced from §3.2 of CLAUDE.md and verified against the live tree).

### F5 — Diagram sources DO exist (corrects stale handoff)
`find … -name '*.mmd' -o -name '*.puml' -o -name '*.drawio'` = **128** (excluding cli_agents/resources/deps). Distribution: `helix_agent` 40, `helix_llm` 14, `llms_verifier` 6, `helix_qa` 6, plus 3–4 each across ~30 functional submodules. The handoff's "No .mmd/.puml diagram sources" is **false**. Real gap is narrower: **root + inner `helix_code/` architecture diagrams** appear thin — to be quantified when `docs/ARCHITECTURE.md` is authored.

### F6 — Aspirational/stub docs that over-claim by title
- `SONARQUBE_SNYK_IMPLEMENTATION.md` — aspirational only (no working containerized Snyk/Sonar yet; that's a queued LARGE phase). Title implies implemented.
- `VIDEO_COURSE_CURRICULUM.md` — stub; real course material lives in `github_pages_website/docs/courses`.
Both should be relabeled `…_PLAN.md` / carry a `Status: PLANNED` header (§11.4.73 spec-versioning posture) until backed by reality.

---

## 2. Prioritized reconciliation plan

> Each retirement = move to `docs/archive/<name>` (preserve history, §9 no-loss) + a one-line `docs/archive/INDEX.md` pointer + §11.4.90 `Obsolete (→ Fixed.md)` ledger entry with `Reason: superseded-by-later-mandate` (SSOT/CONTINUATION) or `stale-documentation`. **No `rm`.**

- **P1 (HIGH, anti-bluff) — Neutralize F2 false-success reports.** Archive the ~22 completion/success reports; add a one-line banner redirect to `docs/CONTINUATION.md`. Removes the worst Rule-9 surface. *Decision-gated (operator).*
- **P2 (HIGH) — Author `docs/ARCHITECTURE.md` (F4).** Fills a cited-but-missing authority doc. Net-new content, low risk — can proceed without retirement sign-off. *Good candidate for a dedicated subagent stream.*
- **P3 (MED) — Retire F3 tracker docs** in favor of `workable_items.db`/`Issues.md`/`Fixed.md`. *Decision-gated.*
- **P4 (MED) — Consolidate F1 plan/analysis/phase churn** into the existing `docs/CONSOLIDATED_*` + `CONTINUATION.md`; archive originals. *Decision-gated.*
- **P5 (LOW) — Relabel F6 aspirational docs** with `Status: PLANNED`. Low risk.

## 3. Decision points for operator
1. **Approve mass archive** of P1/P3/P4 (~50 root docs → `docs/archive/`)? Default safe action = archive-not-delete (reversible per §9.2), so this is low-blast-radius and a candidate for autonomous proceed under §11.4.101 — but volume warrants explicit sign-off.
2. Confirm `docs/CONTINUATION.md` + `docs/workable_items.db` as the **two** canonical live records, with everything else either reference or archived.

## 4. Honest boundary (§11.4.6)
This is a read-only inventory + plan. It does **not** verify the *content accuracy* of the KEEP-bucket design/spec docs (F1 row 9) — that is per-doc follow-up work. Submodule-internal doc sprawl (helix_code 84, llms_verifier 159 per handoff) is **not** re-counted here and remains a separate per-module sweep.

## Sources verified
Sources verified 2026-06-09: internal HelixCode consolidation/planning document — no third-party-service instructions; cross-referenced against project state (docs/Issues.md, docs/Fixed.md, .gitmodules, docs/CONTINUATION.md) as of 2026-06-09.
