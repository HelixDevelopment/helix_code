# HXC-107 — Feature-Status Ledger Enumerated-Completeness Audit (§11.4.153 / §11.4.118)

| | |
|---|---|
| Audit date | 2026-06-23 |
| Target | `docs/features/Status.md` (rev 8, 123 KB, 1029 lines) |
| Method | Enumerate live codebase → reconcile against ledger rows; spot-check evidence paths + backing code |
| Verdict | **GAPS-FOUND** — enumeration is genuinely complete, but the §11.4.153 video-confirmation layer is **stale/bluffed**: every `📹 yes` evidence path cited has been rotated away and no longer exists on disk |

---

## Verdict summary

- **Enumeration completeness (§11.4.118 (a)/(b)/(c) axis): PASS.** Every component/client/package/submodule in the live tree maps to ≥1 ledger row or a documented exclusion. No code-present-but-ledger-missing feature found. No fabricated (ledger-row-without-code) row found in spot-check.
- **§11.4.153 video-confirmation integrity: FAIL.** 23 rows are marked `Overall=confirmed` and cite `📹 yes(<recording>)` evidence, but **all 34 concretely-checkable cited recording files (out of 58 distinct cited names) are MISSING** from `/Volumes/T7/Downloads/Recordings`. The §11.4.154 fresh-corpus rotation has replaced the `-20260615/-20260616` recordings the ledger cites with a newer `-20260622/-20260623` corpus. Per §11.4.153 ("a confirmed row whose evidence path is missing is a bluff"), these 23 `confirmed` rows are currently un-backed.

---

## (a) Structural reconciliation — code AND ledger (PASS)

All five structural counts the ledger claims match the live tree EXACTLY:

| Scope | Live-tree count | Ledger claim | Match |
|---|---|---|---|
| `helix_code/internal/*` | 72 dirs | 72 (71 prod + `i18n_wiring` test-only) | ✓ |
| `helix_code/cmd/*` | 11 dirs | 11 | ✓ |
| `helix_code/applications/*` | 6 dirs (android, aurora_os, desktop, harmony_os, ios, terminal_ui) | 6 | ✓ |
| `submodules/*` | 67 dirs (all `.gitmodules`-registered) | 67 → 65 rowed + 2 excluded | ✓ |
| `cli_agents/*` | 50 dirs (= 50 `.gitmodules` entries) | 50 (scoped to landed ports) | ✓ |

- **All 72 internal dirs** have ≥1 row (verified each name against the ledger's "Internal services + infrastructure" + "Deepened inventory" sections). Both `i18nwiring` and `i18n_wiring` exist on disk as distinct dirs and both are rowed.
- **All 11 cmd dirs** rowed (cli, config_test, helix_config, i18n, infrastructure, performance_optimization, security_fix, security_fix_standalone, security_scan, security_test, server).
- **All 67 submodule dirs** accounted: 65 carry ≥1 `| submodule |` row; the 2 exclusions (`docs_chain`, `challenges`) are documented infra/tooling. `claude-toolkit` (added rev8) confirmed present on disk.
- `helix_code/web/frontend` confirmed present (ledger cites `web/frontend` rows).

## (b) Code present but MISSING from ledger — NONE FOUND

Every live dir maps to a row. The only intentional non-1:1 mapping is `cli_agents/*` (50 vendored agents), where the ledger explicitly scopes the "Ported cli_agents capabilities" section to *landed ports only*, not every vendored agent's full feature set — a documented §11.4.6 scoping decision, not a silent gap.

## (c) Ledger rows with NO backing code — NONE FOUND

Spot-checked backing `.go` files for sampled internal rows (all real code present):

| Row component | prod .go | test .go | Note |
|---|---|---|---|
| internal/voice | 6 | 3 | real |
| internal/kilocode | 8 | 3 | real |
| internal/continua | 7 | 4 | real |
| internal/clarification | 4 | 3 | real |
| internal/approvalwire | 2 | 1 | real |
| internal/infraboot | 1 | 3 | real |
| internal/substrate | 1 | 4 | real |
| internal/i18n_wiring | 0 | 1 | **honestly** flagged test-only — not a false claim |

All 16 spot-checked "ported" rows (approval, autocommit, tools/browser, projectmemory, plantree, workspace, voice, repomap, kilocode, roocode, continua, agent/profiles, checkpoint, workflow/autonomy, workflow/planmode, tools/askuser) resolve to real packages on disk.

---

## Spot-check of "confirmed" evidence paths (§11.4.153) — **FAIL**

Recordings dir `/Volumes/T7/Downloads/Recordings` exists and holds **43 `helixcode-` artifacts**, but they are a **newer corpus** (`-20260622T*` / `-20260623T*`). The ledger's `📹 yes` cells cite the **older `-20260615`/`-20260616` corpus**, which the §11.4.154 fresh-corpus rotation has since deleted.

- Rows marked `Overall=confirmed`: **23**
- Distinct cited recording filenames referencing `-20260615/16`: **58**
- Concretely-checkable cited files → on-disk: **HIT=0 / MISS=34** (zero survive)
- Specifically MISSING (sample): `helixcode-cli-stream-20260616.mp4`, `helixcode-cli-generate-20260616.mp4`, `helixcode-cli-list-models-20260616.mp4`, `helixcode-api-generate-20260616.mp4`, `helixcode-tui-llm-deepseek-20260616.mp4`, `helixcode-web-llm-console-deepseek-20260616.mp4`, `helixcode-desktop-chat-themed-20260615.mp4`, `helixcode-android-themed-20260615.mp4`, `helixcode-ios-launch-20260615.mp4`, `helixcode-web-04-deepseek-stream-20260616.png`.

**Root cause:** the ledger cites the **raw, rotatable, git-ignored corpus** (§11.4.128 / §11.4.154) in the `📹 Video` column instead of **durable curated `docs/qa/<run-id>/` evidence** (§11.4.83). When a later recording run rotated the corpus, every `confirmed` claim was orphaned. (The `V&V` column for the web rows DOES cite a durable path — `docs/qa/web-llm-e2e-20260615/` — which still exists and is valid.)

**Stale in both directions:** disk now holds NEW evidence the ledger does not reflect — `helixcode-cli-{generate,list_models,health,command_exec,list_workers,version}-20260622T*`, `helixcode-server-api-20260622T*`, `helixcode-tui-{dashboard,projects,qa,sessions,tasks,workers}-2026062*`, `helixcode-web-llmchat-20260623T*`, `helixcode-desktopgui-llmchat-20260623T*`, `helixcode-ios-launch-render-20260623-*` (each with `.evidence_frame.png` + several with `.pane.txt`). So the ledger simultaneously cites dead paths AND omits the live ones.

## 4-format exports + sources (PASS)

All present: `Status.{md,html,pdf,docx}` and `Status_Summary.{md,html,pdf,docx}`. All 5 inventory sources present (`cmd_and_clients`, `deepened_round2`, `internal_services`, `ported_cli_agents`, `submodules` — each .md/.html/.pdf). `docs/qa/web-llm-e2e-20260615/` (cited V&V path) exists.

---

## Recommended ledger fixes

1. **Re-anchor the `📹 Video` column to durable §11.4.83 evidence.** Curate the current `-20260622/-20260623` recordings into `docs/qa/<run-id>/` and cite those durable paths, OR downgrade the 23 `confirmed` rows to `working-untaped` until a durable recording is cited. Citing the rotatable raw corpus is the defect that orphaned every confirmation.
2. **Refresh the ledger to the live corpus.** Re-point CLI/API/TUI/Web/Desktop/iOS rows to the new `-2026062*` recordings (which exist with `.evidence_frame.png` + OCR `.pane.txt` evidence), instead of the deleted `-2026061*` names.
3. **Close the ledger's own flagged gap (a):** add the §11.4.86 drift-proof fingerprint sidecar over BOTH the feature-key roster AND the video-artefact roster, so a rotated/missing evidence file mechanically fails a freshness gate instead of silently bluffing green.
4. **Back-port rev2–5 hand-edits into the generator sources** before any docs_chain regen (the ledger's flagged gap (b)) so regen does not regress annotations.

## Honest boundary (§11.4.6)

This audit proves enumerated completeness (every code dir is rowed; no fabricated rows in the sampled set) and proves the §11.4.153 evidence-path bluff (cited recordings gone). It does NOT re-verify per-row Dev/Wired/Real-use correctness beyond file presence, and the backing-code spot-check sampled ~24 of 564+ rows — a representative, not exhaustive, sample.
