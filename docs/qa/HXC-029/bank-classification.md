# HXC-029 — HelixQA Bank `manual-review-required` Deep-Classification

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-05-29 |
| Last modified | 2026-05-29 |
| Status | active |
| Tracked item | HXC-029 (§11.4.98 full-automation anti-bluff forward sweep) |
| Pass type | **Static source-read + JSON action-type analysis. NO tests executed. NO bank/test source modified.** |
| Predecessor | `docs/qa/HXC-029/compliance-ledger.md` (flagged these 19 banks NEEDS-MANUAL-REVIEW) |

## Method

§11.4.98(C) requires live tests to be self-driving (no human action during
execution). §11.4.98(F): manual-dependency tests not rewritten within 30 days
graduate to §11.4.90 Obsolete. The prior ledger flagged 19 HelixQA banks under
`helix_qa/banks/` carrying `_conversion_note: "manual-review-required"` steps as
NEEDS-MANUAL-REVIEW (not confirmed violations).

This pass classifies each bank by (1) the **action content** of every
`manual-review-required` step — what action type it carries (`playwright:`,
`http:`, `adb_shell:`, prose-with-no-prefix) and what the prose describes (API
call / CLI run / UI tap / browser action), to decide whether it COULD be driven
programmatically; and (2) the **executor's runtime behaviour** for those steps,
to decide whether any false PASS occurs today.

### Executor semantics established (anti-bluff, cited)

- `helix_qa/pkg/testbank/schema.go:285-320` — `ParseAction` recognises a step as
  executable only when it carries a known `type:` prefix (`adb_shell:`, `shell:`,
  `http:`, `playwright:`, `tap:`, etc.); everything else falls through to
  `ActionTypeDescription` (`schema.go:319-320`).
- `helix_qa/pkg/autonomous/structured_executor.go:575-587` — an
  `ActionTypeDescription` step that begins `# CONVERT: Convert to executable` is
  SKIPPED; **any other prose returns `Success:false` → FAILS** (lines 586-587:
  `"Text-only action - not executable!"`). So prose-with-no-prefix does NOT
  silently SKIP in the structured executor — it FAILS. (Correction to the prior
  ledger's "executor SKIPs unrecognised prose" for the structured-executor path.)
- `structured_executor.go:551-573` — `ActionTypePlaywright` steps SKIP with
  `SKIP-OK: #PLAYWRIGHT-RUNTIME-PENDING` when no CDP URL is configured (honest skip).
- **Across all 19 banks: `# CONVERT:`-prefixed actions = 0** (grep, this pass).
  So none of the prose steps hit the SKIP branch — they would FAIL if the
  structured executor ran them.

### Why no false PASS occurs today (load-path analysis, cited)

- Default executor banks dir is `challenges/helixqa-banks`
  (`helix_qa/cmd/helixqa/main.go:655-657`), and the structured-executor fallback
  alt-dir is the SAME path (`structured_executor.go:99-102`).
  **`challenges/helixqa-banks` does not exist / is empty** (verified this pass) —
  so the autonomous structured executor loads ZERO of these 19 banks by default.
- The 19 banks under `helix_qa/banks/` are loaded ONLY on an explicit `-banks`
  flag (`main.go:112,258`) / `config.Banks` (`pkg/config/config.go:69`), or by the
  one Go test that targets a single bank file directly.
- That test — `helix_qa/pkg/autonomous/bank_realbinary_test.go:74` — loads ONLY
  `full-qa-api.json`, evaluates ONLY `ActionTypeHTTP` steps (`continue`s past all
  prose/playwright at lines 99-101), runs against a REAL server, and SKIPs
  honestly when the server is unreachable
  (`bank_realbinary_test.go:62,67` → `SKIP-OK: #BLUFF-HELIXQA-BANKS-REWRITE-001`).
  The prose `manual-review-required` steps are simply not executed by it.

**Net:** today these prose steps produce NO false PASS — they are either not
loaded (default path) or skipped-by-action-type (realbinary test). They are a
"looks-covered-but-doesn't-run" surface (a §11.4 coverage gap), not a live
§11.4.98(C) manual-intervention-during-execution violation. The existing tracker
ticket for the rewrite is `#BLUFF-HELIXQA-BANKS-REWRITE-001`.

### Classification rubric

- **AUTO-EXECUTABLE-WITH-WORK** — the steps describe actions drivable
  programmatically (server API call, shell/CLI run, headless browser, adb/uiauto)
  with reasonable effort. Candidate for rewrite to `http:`/`assert:`/`shell:`/
  `playwright:`/`adb_shell:` actions.
- **GENUINELY-MANUAL** — a step intrinsically needs a human decision/observation
  no automation can supply. Candidate for §11.4.90 Obsolete citing §11.4.98(F).
- **ALREADY-SKIPPED-HONESTLY** — executor skips with a clear reason, no false
  PASS, low priority.

## Classification table

| Bank (path) | #manual-review steps | Action-type mix | Classification | Evidence (file:line) | Recommended action |
|---|---|---|---|---|---|
| `helix_qa/banks/full-qa-web.json` | 572 | prose=413, playwright=159 | AUTO-EXECUTABLE-WITH-WORK | `full-qa-web.json:24` `playwright: navigate http://localhost:3000/login`; prose like "Check for Catalogizer logo" is a browser-DOM assertion | Wire Playwright runtime (`schema.go:551-573` CDP URL); convert prose to `playwright:`/`assert:`. Highest volume. |
| `helix_qa/banks/atmosphere.json` | 423 | prose=366, playwright=42, adb_shell=15 | AUTO-EXECUTABLE-WITH-WORK | `atmosphere.json` carries `adb_shell:`+`playwright:` already; prose are UI/device steps | Convert prose to `adb_shell:`/`tap:`/`playwright:`; real device or emulator topology per §11.4.3. |
| `helix_qa/banks/full-qa-android.json` | 318 | prose=289, playwright=24, adb_shell=3, keypress=2 | AUTO-EXECUTABLE-WITH-WORK | `full-qa-android.json:526` `adb_shell:`; prose are Android UI taps | Convert prose to `adb_shell:`/`tap:`/`keypress:`; uiautomator dispatch per §11.4.48. |
| `helix_qa/banks/performance-validation.json` | 116 | prose=110, playwright=6 | AUTO-EXECUTABLE-WITH-WORK | `performance-validation.json:27` "Send 3 GET /api/v1/health requests…"; "Ensure at least 500 entities in database" | Convert to `http:` loops + `assert:` on latency; seed DB via API. Pure server-API. |
| `helix_qa/banks/storage-configuration.json` | 88 | prose=88 | AUTO-EXECUTABLE-WITH-WORK | `storage-configuration.json:31` "Tap/click 'Add WebDAV server'"; "Enter username 'testuser'…" | Desktop/mobile GUI taps → `tap:`/`text:` (adb/uiauto) OR drive backend WebDAV config via `http:`. |
| `helix_qa/banks/cli-agents-comprehensive.json` | 79 | prose=79 | AUTO-EXECUTABLE-WITH-WORK | `cli-agents-comprehensive.json:100` "Call registry.GetStats()", "Iterate through AllAgentTypes()" | Go-API exercises → fold into Go integration tests OR `shell:` driving the CLI; no human needed. |
| `helix_qa/banks/edge-cases-stress.json` | 78 | prose=78 | AUTO-EXECUTABLE-WITH-WORK | `edge-cases-stress.json:26` "Open 10 different text files in separate tabs", "Click/tap tabs 1,10,20" | GUI stress → `adb_shell:`/`tap:` loops; aligns with §11.4.85 stress mandate. |
| `helix_qa/banks/aichat-bash-tools-comprehensive.json` | 72 | prose=72 | AUTO-EXECUTABLE-WITH-WORK | `aichat-bash-tools-comprehensive.json:31` "./bin/helixagent --generate-agent-config=aichat" | Shell/CLI runs → `shell:` actions; deterministic, no human. |
| `helix_qa/banks/cli-agents-test-helixagent.json` | 71 | prose=71 | AUTO-EXECUTABLE-WITH-WORK | `cli-agents-test-helixagent.json:158` "aider --model helixagent/ensemble --architect" | CLI invocations → `shell:`; needs a running helixagent endpoint (config bootstrap = permitted §11.4.98(B)). |
| `helix_qa/banks/editor-operations.json` | 63 | prose=63 | AUTO-EXECUTABLE-WITH-WORK | `editor-operations.json:26` "Create a new .txt file", "Type 'Hello, Yole!…'" | Yole editor GUI → `tap:`/`text:` (adb/uiauto). |
| `helix_qa/banks/cloud-storage-operations.json` | 63 | prose=63 | AUTO-EXECUTABLE-WITH-WORK | `cloud-storage-operations.json:4` "Open storage selector, choose WebDAV server" | GUI taps → `tap:` OR backend `http:`. |
| `helix_qa/banks/file-browser.json` | 61 | prose=61 | AUTO-EXECUTABLE-WITH-WORK | `file-browser.json:25` "Tap/click file browser icon or Files tab" | Yole GUI navigation → `tap:` (adb/uiauto). |
| `helix_qa/banks/app-navigation.json` | 54 | prose=54 | AUTO-EXECUTABLE-WITH-WORK | `app-navigation.json:26` "Open Yole application", "Tap/click 'Files' in bottom navigation" | GUI navigation → `adb_shell: am start` + `tap:`. |
| `helix_qa/banks/security-validation.json` | 41 | prose=41 | AUTO-EXECUTABLE-WITH-WORK | `security-validation.json:38` "Use a JWT token with exp claim in the past"; "Sign a JWT with a different secret key" | Server-API security probes → `http:`+`assert:` (mint tokens in-test). Already has 14 honest `_skip` entries. |
| `helix_qa/banks/all-formats.json` | 40 | prose=40 | AUTO-EXECUTABLE-WITH-WORK | `all-formats.json:31` "Create new .md file", "Type '# Title **bold**…'" | Yole editor GUI → `tap:`/`text:` + content assert. |
| `helix_qa/banks/entity-management.json` | 38 | prose=34, playwright=4 | AUTO-EXECUTABLE-WITH-WORK | `entity-management.json:26` "POST 5 episodes with parent_id = season ID" | Pure server-API CRUD → `http:`+`assert:`. Already has 38 honest `_skip` entries. |
| `helix_qa/banks/full-qa-cross-platform.json` | 37 | prose=37 | AUTO-EXECUTABLE-WITH-WORK | `full-qa-cross-platform.json:30` "Navigate to login page, enter admin/admin123, click Login" | Browser/desktop login flow → `playwright:`/`adb_shell:` per platform (§11.4.3 topology dispatch). |
| `helix_qa/banks/admin-operations.json` | 29 | prose=29 | AUTO-EXECUTABLE-WITH-WORK | `admin-operations.json:464` "Poll scan status until complete", "Check that backup file exists and has non-zero size" | Server-API + poll loop → `http:`+`assert:`. Already has 28 honest `_skip` entries. |
| `helix_qa/banks/full-qa-api.json` | 27 | prose=26, http=1 | ALREADY-SKIPPED-HONESTLY (+ AUTO-EXECUTABLE-WITH-WORK remainder) | 71 `_skip:true` w/ reasons (`full-qa-api.json` e.g. `#BLUFF-HELIXQA-LOGOUT-CASCADE-001`); realbinary test runs HTTP steps only + skips honestly (`bank_realbinary_test.go:62,67`) | Lowest priority: HTTP steps already verified by realbinary test; remaining prose convert to `http:`/`assert:`. |

## Counts by classification

| Classification | #banks | Banks |
|---|---|---|
| AUTO-EXECUTABLE-WITH-WORK | 18 | full-qa-web, atmosphere, full-qa-android, performance-validation, storage-configuration, cli-agents-comprehensive, edge-cases-stress, aichat-bash-tools-comprehensive, cli-agents-test-helixagent, editor-operations, cloud-storage-operations, file-browser, app-navigation, security-validation, all-formats, entity-management, full-qa-cross-platform, admin-operations |
| GENUINELY-MANUAL | 0 | — (no sampled step intrinsically requires a human decision; all are API/CLI/UI/browser actions drivable programmatically) |
| ALREADY-SKIPPED-HONESTLY | 1 (primary) | full-qa-api (HTTP path verified + skipped honestly; also has the most explicit `_skip` markers). Note: ALL 19 banks are currently NOT in the default executor load path, so none produce a false PASS today. |

**Sub-grouping of the 18 AUTO-EXECUTABLE banks by drive mechanism (rewrite cost):**

- **Server-API (`http:`+`assert:`) — lowest cost, no GUI/device needed (5):**
  full-qa-api, performance-validation, security-validation, entity-management,
  admin-operations.
- **CLI/shell (`shell:`) — low cost, needs a running endpoint (3):**
  cli-agents-comprehensive, aichat-bash-tools-comprehensive, cli-agents-test-helixagent.
- **Web browser (`playwright:`) — needs Playwright CDP runtime (3):**
  full-qa-web, full-qa-cross-platform, (web portion of) atmosphere/entity-management.
- **Desktop/Android GUI (`adb_shell:`/`tap:`/`text:` + uiauto) — highest cost (7):**
  atmosphere, full-qa-android, storage-configuration, edge-cases-stress,
  editor-operations, cloud-storage-operations, file-browser, app-navigation,
  all-formats (overlap across topologies).

## Anti-bluff caveat (§11.4.6)

- **No GENUINELY-MANUAL bank was found**, but this is a static action-content
  judgement. It is **UNCONFIRMED** whether every GUI step has a reachable
  uiautomator node (empty hierarchies force the §11.4.48/§11.4.52 APK/intent
  fallback) — provable only by running against a real device/emulator, which this
  pass did not do. None is classified GENUINELY-MANUAL on that uncertainty; the
  GUI banks are AUTO-EXECUTABLE-WITH-WORK with this caveat recorded.
- The "no false PASS today" finding is grounded in the load-path analysis above
  (default dir empty + realbinary test skips honestly). If a future runner is
  pointed at `helix_qa/banks/` directly via `-banks`, the structured executor
  would FAIL the prose steps (`structured_executor.go:586-587`), not silently
  PASS them — still anti-bluff-safe, but noisy. Wiring them to a real runner
  before converting prose would manufacture failures; convert first.

## Prioritized worklist (rewrite, not obsolete)

Because **0 banks are GENUINELY-MANUAL**, the §11.4.90 Obsolete path does NOT
apply to any of the 19 — every bank is a rewrite candidate, not an obsolete
candidate. Rewrite order favours lowest cost + highest coverage value first:

1. **`full-qa-api.json` (27) + `entity-management.json` (38 skips) +
   `admin-operations.json` (29) + `security-validation.json` (41) +
   `performance-validation.json` (116)** — pure server-API. Convert prose to
   `http:`+`assert:` (no GUI/device). `full-qa-api` already half-verified by
   `bank_realbinary_test.go`; extend that pattern. Closes `#BLUFF-HELIXQA-BANKS-REWRITE-001`.
2. **`cli-agents-comprehensive.json` (79) + `aichat-bash-tools-comprehensive.json`
   (72) + `cli-agents-test-helixagent.json` (71)** — CLI/Go-API. Convert to
   `shell:` (or fold Go-API steps into `helix_code` integration tests). High
   step-count, deterministic, no GUI.
3. **`full-qa-web.json` (572) + `full-qa-cross-platform.json` (37)** — wire the
   Playwright CDP runtime (`structured_executor.go:551-573`), then convert the 413
   prose web steps to `playwright:`/`assert:`. Largest single coverage block.
4. **`full-qa-android.json` (318) + `atmosphere.json` (423)** — Android GUI.
   Convert prose to `adb_shell:`/`tap:`/`keypress:` with §11.4.48 uiautomator
   dispatch + §11.4.3 topology gating; real device/emulator.
5. **Desktop/Yole-GUI banks: `storage-configuration` (88), `edge-cases-stress`
   (78), `editor-operations` (63), `cloud-storage-operations` (63), `file-browser`
   (61), `app-navigation` (54), `all-formats` (40)** — convert GUI prose to
   `tap:`/`text:`/`adb_shell:` (or backend `http:` where a UI step has a server
   equivalent). Highest per-step cost; batch by shared Yole UI surface.

## Sources verified

Internal source-of-truth files read during this audit (static code audit — no
external web sources):
- `helix_qa/pkg/testbank/schema.go` (ActionType taxonomy, `ParseAction`, `_skip`/`_skip_reason`)
- `helix_qa/pkg/autonomous/structured_executor.go` (step dispatch: Playwright SKIP, prose FAIL, placeholder-skip)
- `helix_qa/pkg/autonomous/pipeline.go` (BanksDir wiring)
- `helix_qa/cmd/helixqa/main.go` (default BanksDir = `challenges/helixqa-banks`)
- `helix_qa/pkg/autonomous/bank_realbinary_test.go` (only HTTP steps run; honest skip)
- all 19 `helix_qa/banks/*.json` (JSON action-type distribution of `manual-review-required` steps)
- `docs/qa/HXC-029/compliance-ledger.md` (predecessor ledger)
