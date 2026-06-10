# SP0b — CONST-055 Post-Constitution-Pull Validation Audit (REPORT-ONLY)

| | |
|---|---|
| Revision | 1 |
| Created | 2026-06-10 |
| Last modified | 2026-06-10 |
| Status | active |
| Mode | READ-ONLY audit — identifies violations, fixes nothing |
| HEAD at audit | `e547d4db` (2026-06-10 13:41:24 +0500) |
| Authority | CONST-055 / §11.4.32 (post-constitution-pull validation sweep) |

## Table of contents

- [1. Scope and method](#1-scope-and-method)
- [2. Gates run (real output captured)](#2-gates-run-real-output-captured)
- [3. Findings table](#3-findings-table)
- [4. Detail notes](#4-detail-notes)
- [5. Summary](#5-summary)

---

## 1. Scope and method

This is the "Before we begin" CONST-055 post-constitution-pull validation sweep,
executed in REPORT-ONLY mode. No code edited, no commit, no push. The only write
is this document.

Tooling located under `scripts/`:
- `scripts/verify-all-constitution-rules.sh` (the CONST-055 runner; 14 gates G1–G14) — **read-only** (greps + delegated gate scripts; the only `git rm` token is inside a fix-suggestion string, line 254, never executed). RAN.
- `scripts/verify-governance-cascade.sh` — read-only cascade verifier. RAN.
- `scripts/no-silent-skips.sh` — read-only `t.Skip()`/`describe.skip` scanner. RAN.
- `scripts/install-git-hooks.sh` + `scripts/git_hooks/` — hook installer (NOT run; would mutate `.git/hooks`). Inspected only.

## 2. Gates run (real output captured)

**G — `verify-governance-cascade.sh`** → `=== Result: 0 failures ===` / `PASS`.
14 governance anchors × consumer fleet; every owned + third-party submodule resolved (third-party correctly listed in `submodule_third_party.txt`).

**CONST-055 sweep — `verify-all-constitution-rules.sh --quiet`** → `Gates run: 14 / Failures: 3`:
- **G8 FAIL** §11.4.90 — `docs/Fixed.md` `## HXC-044` has Obsolete status but no `**Obsolete-Details:**` line within 8 non-blank lines.
- **G12 FAIL** §11.4.12/91 — `docs/Fixed_Summary.md` stale vs trackers (`Last regenerated: 2026-06-09`, expected `2026-06-10`).
- **G14 FAIL** §11.4.106 — docs_chain `verify --all` reports drift: `fixed STALE [fixed_html fixed_summary]`, `issues STALE [issues_html issues_summary]`.
- (G1–G7, G9–G11, G13 PASS.)

**`no-silent-skips.sh`** → 167 bare skips (head showed 15, "+152 more"). ALL hits are under `submodules/helix_agent/cli_agents/continue/**` — i.e. a **third-party vendored** CLI agent tree, not HelixCode-owned code. Treated as low-severity/out-of-scope (third-party exempt), but noted.

**Anti-bluff smoke (CONST-035, CLAUDE.md §9 pattern)** → `bluff-count: 0` over `helix_code/internal` + `helix_code/cmd`. PASS.

## 3. Findings table

Severity: P0 = release blocker per cited mandate; P1 = high; P2 = medium; P3 = low/advisory.
"SP" = suggested fix-owner work-package.

| # | Rule | Status | Severity | Evidence | Suggested fix-owner (SP) |
|---|------|--------|----------|----------|--------------------------|
| F1 | §11.4.75 mechanical git-hook enforcement | **FAIL** | P0 | `.git/hooks/` contains ONLY `*.sample` files; no `pre-commit`/`pre-push`/`post-commit`/`commit-msg` installed (`core.hooksPath` = default). Source dir `scripts/git_hooks/` ships ONLY `pre-push` — the other 3 hooks §11.4.75 mandates are not even present to install. | SP-hooks: author the 4 §11.4.75 hooks into `scripts/git_hooks/`, wire `install-git-hooks.sh`, run installer (mutation — operator-gated). |
| F2 | §11.4.90 Obsolete-Details (G8) | **FAIL** | P1 | `docs/Fixed.md:211` `## HXC-044 …` — no `**Obsolete-Details:**` line within 8 non-blank lines (`Evidence:` + summary present, but not the required Since/Reason/Superseding/Triple-check line). | SP-trackers: add `**Obsolete-Details:**` line OR re-classify HXC-044 status if it is Fixed-not-Obsolete. |
| F3 | §11.4.12/91 summary freshness (G12) | **FAIL** | P1 | `docs/Fixed_Summary.md:15` footer `Last regenerated: 2026-06-09`; trackers advanced to 2026-06-10. Diff: `113` total closed counted but timestamp stale. | SP-docs: re-run `scripts/generate_fixed_summary.sh` (+ issues) and `regenerate-tracker-exports.sh`. |
| F4 | §11.4.106 docs_chain sync (G14) | **FAIL** | P1 | `docs_chain verify --all`: `fixed STALE [fixed_html fixed_summary]`, `issues STALE [issues_html issues_summary]`. `.html`/`.pdf` siblings older than `.md`. | SP-docs: `docs_chain sync --all --root .` (mechanically resolves F3 + F4). |
| F5 | CONST-044 CONTINUATION freshness | **FAIL** | P1 | `docs/CONTINUATION.md` last git-touch `638bd400 2026-06-09 22:08` but HEAD is `e547d4db 2026-06-10 13:41` (LLMs-access roadmap + 6 SP plans landed AFTER CONTINUATION's last update). CONTINUATION did not advance with the state-advancing commit → out-of-sync = CRITICAL DEFECT per Art. XIII §13.1. | SP-continuation: update `docs/CONTINUATION.md` §3 to reflect the SP-planning commits, same-commit discipline. |
| F6 | CONST-036 hardcoded model lists (D-5) | **FAIL** | P1 | `helix_code/internal/llm/model_discovery.go:1200-1209` `getFallbackAlternatives` returns hardcoded model-name slices (`codellama-7b-instruct`, `mistral-7b-instruct`, `llama-3-8b-instruct`, `gemma-7b-instruct`, …). Not verifier-sourced. | SP-llm: source fallback alternatives from LLMsVerifier metadata per CONST-036, or document/guard as last-resort non-displayed fallback. |
| F7 | CONST-036 hardcoded model default (D-1) | **PARTIAL** | P2 | `helix_code/cmd/cli/main.go:206,341` `DefaultModel: "llama3.2"`; `:1167,1503` default flag `"llama-3-8b"`. Mitigated: `:1503-1507` replaces the generic default with `c.llmProvider.GetModels()[0]` at runtime (live query). The literal default remains a hardcoded model name but is a documented fallback, not a displayed model list. | SP-llm: move default to config/verifier; low risk. |
| F8 | §11.4.65/106 third-party doc-sync (advisory) | UNKNOWN | P3 | docs_chain drift limited to issues/fixed; governance `in-sync`. No evidence of broader INCLUDED-scope staleness sampled this run. | — (resolved by F4 sync). |
| F9 | CONST-053 tracked secrets/artifacts | **PASS** | — | `git ls-files \| grep -E '\.(env\|pem\|key)\|id_rsa'` (excl `.example/.sample/.full-test`) = 0; no extensionless `/bin/` binaries tracked. 10 `*.log` tracked but all under `docs/**/evidence/` + `docs/improvements/` = reference/captured-evidence assets (CONST-053 class-5 "unless reference assets" exemption). Root `.gitignore` present (195 lines). | — |
| F10 | CONST-035 production bluff smoke | **PASS** | — | `bluff-count: 0` over `helix_code/internal` + `helix_code/cmd`. | — |
| F11 | CONST-046 hardcoded user-facing strings (sample) | **PASS (sampled)** | P3 | CLI generate path uses `tr(ctx, "cli_…")` i18n (main.go:1510-1511). Only `fmt.Printf("Error: %v\n", err)` (main.go:2077) and ~3 similar dev-facing error prints — error wrapping, not localized user-flow content. No hardcoded question/prompt arrays found. | SP-i18n (low): migrate residual `fmt.Printf("Error:…")` to i18n if user-facing. |
| F12 | no-silent-skips (third-party) | UNKNOWN/exempt | P3 | 167 bare `*.skip()` all under `submodules/helix_agent/cli_agents/continue/**` (third-party vendored). No HelixCode-owned bare skips surfaced. | — (third-party exempt). |

## 4. Detail notes

- **F1 is the highest-severity finding.** §11.4.75 demands FIVE mechanical enforcement layers; the repo has neither the installed hooks (only `.sample`) nor even the source for 3 of the 4 mandated hooks. This is the structural reason the other doc-sync drifts (F2–F5) reached HEAD un-blocked — there is no local pre-commit/pre-push gate stopping them.
- **F3 + F4 + F5 are one mechanical-drift cluster**: a state-advancing commit (`e547d4db`, the LLMs-access SP planning bundle) landed without re-running the doc-sync/summary/CONTINUATION regeneration. A single `docs_chain sync --all` + summary regen + CONTINUATION update would clear F3/F4/F5.
- **F6/F7 (CONST-036)** match the D-1/D-5 patterns referenced in the task. The cli default (D-1/F7) is runtime-overridden by a live `GetModels()` query so it is largely mitigated; the `model_discovery.go` fallback slices (D-5/F6) are genuine static model-name literals with no verifier sourcing.
- Governance cascade (G1) and anti-bluff smoke are clean — the violations are concentrated in doc-sync mechanics, the missing hook layer, and LLM fallback hardcoding.

## 5. Summary

```
CONST-055 POST-PULL AUDIT — 2026-06-10 — REPORT-ONLY (no fixes applied)
Gates run: verify-governance-cascade (PASS, 0 fail) | verify-all-constitution-rules
  (14 gates, 3 FAIL: G8/G12/G14) | no-silent-skips (167 third-party only) |
  CONST-035 bluff smoke (0 hits, PASS).
Violations by severity: P0 = 1 (F1) | P1 = 5 (F2 F3 F4 F5 F6) | P2 = 1 (F7) |
  P3/advisory = 3 (F8 F11 F12). PASS: F9 F10.
Top 5 to fix (priority order):
  1. F1  §11.4.75 — no git hooks installed (only .sample); 3 of 4 mandated hooks
         not even present in scripts/git_hooks/. [P0, SP-hooks]
  2. F5  CONST-044 — docs/CONTINUATION.md out-of-sync with HEAD e547d4db
         (CRITICAL per Art. XIII §13.1). [P1, SP-continuation]
  3. F4  §11.4.106 — docs_chain drift: issues/fixed html+summary STALE; run
         docs_chain sync --all (also clears F3). [P1, SP-docs]
  4. F2  §11.4.90 — Fixed.md HXC-044 missing Obsolete-Details line. [P1, SP-trackers]
  5. F6  CONST-036 — model_discovery.go:1200-1209 hardcoded fallback model lists
         (D-5), not verifier-sourced. [P1, SP-llm]
Note: nothing was fixed; all items are referred to suggested SP work-packages.
```
