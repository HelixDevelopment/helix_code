# D4 — Closed-Item Audit ("closed ≠ working" hypothesis test)

Read-only audit. REPO=/home/milos/Factory/projects/tools_and_research/helix_code, INNER=$REPO/helix_code,
DB=$REPO/docs/workable_items.db. No edits/commits made. One read-only script execution
(`scripts/audit-const046-hardcoded-content.sh`) was run to obtain runtime evidence per
§11.4.6/§11.4.123 (no-guessing) — this is a read-only grep-based auditor, not a build/boot/commit action.

## Sample selection

DB has 308 rows (149 Bug / 92 Feature / 67 Task); representation='table' rows are the
Fixed.md-parsed closure rows (188 of them), representation='section' are Issues.md rows (120).
All 79 distinct Feature items with representation='table' have severity='' (blank) — the
severity column is essentially unused for closed Feature rows, so "high/critical severity"
filtering was not possible; instead sampled for spread across atm_id prefixes and subsystems.

Sampled 25 distinct closed Feature items:
- All 12 `HXC-*` Feature/table items in the DB (full population, not a subsample)
- 13 `FIX-2026-05-19#N` Feature/table items spanning different subsystems (i18n core lib,
  per-package i18n migrations, GPU telemetry, LLM provider Err coverage, tracker exports,
  release-gate tooling)

## Audit table

| atm_id | claim (from title/closure_criteria/body_md) | verdict | evidence |
|---|---|---|---|
| HXC-003 | CONST-046 i18n migration "exhausted"; audit-gate `scripts/audit-const046-hardcoded-content.sh --fail-on-new` "is enforced"; baseline "shrank monotonically" | **PARTIAL** | Infra present: `pkg/i18n/`, `docs/audits/2026-05-20-internal-const046-classification.md`, `scripts/audit_const046/.baseline.json` (52927 entries) all exist. BUT: live-ran the gate (`--fail-on-new`) and got **exit 1**, log line `Total: 19098 (NEW: 19098, PRE-EXISTING: 0)` — i.e. **100% of current hits report as NEW, 0 match the baseline**, even though the baseline has 52927 stored entries. Root cause confirmed in `scripts/audit_const046/main.go:319-320` (`violationKey(path, hash)` uses the **absolute scan path** as half the comparison key) vs `.baseline.json` entries stored with `"path": "/run/media/milosvasic/DATA4TB/Projects/HelixCode/..."` — a **different host's absolute mount path** than the current checkout (`/home/milos/Factory/projects/tools_and_research/helix_code/...`). The gate is portability-broken: on any checkout whose absolute path differs from the exact machine that generated the baseline, `--fail-on-new` ALWAYS reports every hit as new and ALWAYS exits 1. This directly contradicts the HXC-003 closure claim that the gate "is enforced" — see F-D4-01. |
| HXC-006 | Speed Programme docs (6-phase/31-task) with `docs/research/speed/05-coverage-ledger.md` | VERIFIED-PRESENT | `docs/research/speed/00..05-*.md/.html/.pdf` all exist (file listing confirmed) |
| HXC-041 | `helixqa http` standalone subcommand drives `http:` banks vs live server, no browser/LLM | VERIFIED-PRESENT | `submodules/helix_qa/cmd/helixqa/http.go:44` (`flag.NewFlagSet("http", ...)`), `cmd/helixqa/main.go:64` (`case "http":`) |
| HXC-048 | `helixcode-system.yaml` bank: 11 self-driving http cases (health/server-info/system-status/llm-providers) | VERIFIED-PRESENT | `submodules/helix_qa/banks/helixcode-system.yaml` exists, 12 `http:` action lines incl. `GET /health`, server-info, system-status 401 case |
| HXC-068 | speckit debate adapter wireable into agentic debate flow | VERIFIED-PRESENT | `helix_code/applications/terminal_ui/specify_command.go`, `helix_code/tests/integration/debate_e2e_test.go` |
| HXC-069 | HelixMemory default-on durable persistence + graceful fallback | VERIFIED-PRESENT | `helix_code/internal/memory/manager_persistence.go`, `helixmemory_provider.go`, `persistence_red_test.go` |
| HXC-072 | CLI `/undo` and `/diff` slash commands over autocommit substrate | VERIFIED-PRESENT | `helix_code/cmd/cli/main.go:2376` (`case "/undo", "undo":`), `:2384` (`/diff` handling) |
| HXC-073 | Autocommit git substrate backing CLI edit history | VERIFIED-PRESENT | `helix_code/internal/autocommit/git.go`, `secret_filter.go`, `summariser.go` + tests |
| HXC-074 | Mobile gomobile `Generate` binding for on-device LLM calls | VERIFIED-PRESENT | `helix_code/shared/mobile_core/mobile.go:275` `func (mc *MobileCore) Generate(prompt string) (string, error)` |
| HXC-076 | Web `/llm/generate` + `/llm/stream` endpoints with frontend | VERIFIED-PRESENT | `helix_code/internal/server/llm_generate.go:50,232,291` (`generateLLM`/`streamLLM` handlers for both routes), `llm_auth_guard_test.go` |
| HXC-077 | T1.5 context-window percentage indicator | VERIFIED-PRESENT | `helix_code/applications/terminal_ui/context_usage_test.go` present (guard test referenced in closure) |
| HXC-094 | F12 workspace checkpoints — snapshot + restore/undo | VERIFIED-PRESENT | `helix_code/internal/checkpoint/checkpoint.go`, `checkpoint_test.go`; `cmd/cli/main.go:2416` (`/checkpoint` dispatch) |
| FIX-2026-05-19#1 | pkg/i18n core foundation (Bundle/Localizer + sentinel errors) | VERIFIED-PRESENT | `helix_code/pkg/i18n/bundle.go`, `localizer.go`, `errors.go` + tests |
| FIX-2026-05-19#3 | Per-submodule i18n injection wiring + i18nadapter | VERIFIED-PRESENT | `helix_code/pkg/i18nadapter/adapter.go` + test |
| FIX-2026-05-19#37 | `cmd/cli/main.go` i18n migration (Option B cmd-local pkg) | VERIFIED-PRESENT | `helix_code/cmd/cli/i18n/` (bundle.go+bundles+translator.go), 50 `tr(ctx` call sites in `main.go` |
| FIX-2026-05-19#40 | `cmd/server/main.go` i18n migration (Option B) | VERIFIED-PRESENT | `helix_code/cmd/server/i18n/` present, 11 `tr(` sites in `main.go` |
| FIX-2026-05-19#52 | `internal/auth` i18n migration (10 sites) | VERIFIED-PRESENT | `helix_code/internal/auth/auth.go` lines 154,172,201,211,217,237,268,301,336,394 = `tr(ctx, "internal_auth_...")` (>10 sites); `translator.go` defines `tr()`+`SetTranslator` |
| FIX-2026-05-19#53 | `internal/agent` coordinator+base_agent i18n (10 of 64) | VERIFIED-PRESENT | `helix_code/internal/agent/i18n/bundle.go`, `translator.go`, `translator_test.go` |
| FIX-2026-05-19#58 | `internal/database` i18n (8 fmt.Errorf sites) | VERIFIED-PRESENT | `helix_code/internal/database/translator.go` (10 `i18n.`-pattern hits), `translator_test.go` |
| FIX-2026-05-19#61 | `internal/editor` i18n migration | VERIFIED-PRESENT | `helix_code/internal/editor/i18n/bundle.go`, `translator.go`, `translator_test.go` |
| FIX-2026-05-19#69 | LLMOrchestrator builders × 5 (gemini/junie/opencode/claudecode/qwencode) | VERIFIED-PRESENT (outside $INNER, in `submodules/llm_orchestrator`) | `submodules/llm_orchestrator/pkg/agent/builders.go` contains gemini/junie/opencode/claudecode/qwencode references |
| FIX-2026-05-19#70 | 4-vendor GPU telemetry probe chain (NVIDIA+AMD+Apple+Intel) | VERIFIED-PRESENT | `helix_code/internal/cognee/performance_optimizer.go:971-1014` — NVIDIA `nvidia-smi` (round 43), AMD `rocm-smi` (round 45), Apple/Intel referenced in probe-chain comment |
| FIX-2026-05-19#71 | LLM `Err` field coverage across 17 providers | VERIFIED-PRESENT | `helix_code/internal/llm/missing_types.go` — 16 `Err ` field hits |
| FIX-2026-05-19#17 | Tracker HTML+PDF exports (4 HTML + 4 PDF + script + README) | VERIFIED-PRESENT | `docs/Issues.html/.pdf`, `docs/Fixed.html/.pdf`, `docs/Issues_Summary.html/.pdf`, `docs/Fixed_Summary.html/.pdf` all present; mtimes newer than their `.md` sources (no staleness) |
| FIX-2026-05-19#66 | `release-gate-test.sh --skip-env-failures` filter | VERIFIED-PRESENT | `scripts/release-gate-test.sh:99,120,155` implement `MODE_SKIP_ENV` / `--skip-env-failures` |

**Tally: 25 sampled — 24 VERIFIED-PRESENT, 1 PARTIAL, 0 NOT-FOUND.**

The §11.4.118 "closed ≠ working" hypothesis did NOT reproduce as a code-absence problem in this
sample (no "closed-but-no-code" item found) — the codebase-presence side of these 25 closures
is genuinely solid. The one real defect found (HXC-003) is a subtler, arguably more dangerous
class: an anti-bluff **enforcement mechanism itself silently non-functional** on this checkout
(see F-D4-01) — exactly the kind of gap §11.4.108/§11.4.110 warn about (green-looking gate,
broken enforcement).

## DB-integrity findings

### 1. §11.4.171 short descriptions (<50 chars)
Query: `length(description) < 50` on `representation='table'` rows.
**31 rows found** (task expected ~36 — actual count is 31; all from one batch):
```
FIX-2026-05-19#3   (49)   FIX-2026-05-19#4   (43)   FIX-2026-05-19#5   (34)
FIX-2026-05-19#6   (36)   FIX-2026-05-19#8   (41)   FIX-2026-05-19#9   (43)
FIX-2026-05-19#10  (47)   FIX-2026-05-19#17  (39)   FIX-2026-05-19#19  (39)
FIX-2026-05-19#21  (46)   FIX-2026-05-19#22  (43)   FIX-2026-05-19#23  (46)
FIX-2026-05-19#24  (39)   FIX-2026-05-19#25  (38)   FIX-2026-05-19#26  (40)
FIX-2026-05-19#27  (45)   FIX-2026-05-19#28  (42)   FIX-2026-05-19#29  (41)
FIX-2026-05-19#30  (42)   FIX-2026-05-19#31  (39)   FIX-2026-05-19#32  (41)
FIX-2026-05-19#33  (39)   FIX-2026-05-19#34  (44)   FIX-2026-05-19#35  (42)
FIX-2026-05-19#36  (40)   FIX-2026-05-19#38  (40)   FIX-2026-05-19#39  (36)
FIX-2026-05-19#41  (45)   FIX-2026-05-19#66  (47)   FIX-2026-05-19#67  (47)
FIX-2026-05-19#71  (41)
```
All 31 are from the FIX-2026-05-19 i18n-migration-round batch — the `description` column
was populated with terse fragments (e.g. "10 tests + mutation; Bundle/Localizer + sentinel
errors" = the closure_criteria-style summary, not a real §11.4.91-class description) rather
than the mandated ≥50-char human/machine-readable description. Note: FIX-2026-05-19#66 and
#71 are in this list DESPITE being in the sampled-item audit above as VERIFIED-PRESENT closures
— i.e. the underlying work is real, but the DB row itself fails the §11.4.171 description floor.

### 2. §11.4.19 atomic-move drift (terminal status, still in Issues)
Query: `current_location='Issues' AND status LIKE '%→ Fixed.md%'`.
**41 rows matched the raw query**, but 30 of those are the documented dual-representation
pattern (a `current_location='Fixed'` counterpart row also exists — legitimate per the
schema's own comment about composite `(atm_id, current_location)` identity). The remaining
**11 rows have NO matching Fixed-location row at all** — confirmed by direct inspection of
`docs/Issues.md`/`docs/Fixed.md`: these items' full H2 entries are still physically present
in `Issues.md` with a terminal Status line, and are **absent from `Fixed.md`** entirely — the
§11.4.19 atomic move was never executed:

```
HXC-013   Implemented (→ Fixed.md)   "Adopt SQLite-backed single-source-of-truth..." (Issues.md:322)
HXC-014   Completed (→ Fixed.md)     "Stress + chaos test coverage (§11.4.85)" (Issues.md:334)
HXC-014b  Fixed (→ Fixed.md)         "Systemic unguarded i18n translator.go data-race..." (Issues.md:354)
HXC-015   Completed (→ Fixed.md)     "Cross-platform parity (§11.4.81)"
HXC-018   Completed (→ Fixed.md)     "Obsolete status (§11.4.90) + summary-doc clarity..." (Issues.md:447)
HXC-019   Completed (→ Fixed.md)     "docs/qa/ end-user evidence tree (§11.4.83)"
HXC-024   Fixed (→ Fixed.md)         "internal/llm -tags=integration build broken..." (Issues.md:403)
HXC-025   Completed (→ Fixed.md)     "Constitution §11.4.98/99/101 cascade..."
HXC-026   Completed (→ Fixed.md)     "workable-items md<->db sync gate follow-up"
HXC-027   Completed (→ Fixed.md)     "§11.4.98 live-test full-automation compliance audit"
HXC-028   Completed (→ Fixed.md)     "§11.4.99 latest-source documentation cross-reference" (Issues.md:367)
```
Verified directly for HXC-013, HXC-018, HXC-024, HXC-028 with `grep` against both files:
present + full-content in `Issues.md` at the cited line numbers, zero match in `Fixed.md`.

### 3. §11.4.90 Obsolete items missing `obsolete_details`
`obsolete_details` table has **0 rows** (confirmed empty). The DB has exactly 1 distinct
Obsolete atm_id (`HXC-044`, present as 2 rows — `representation='section'` + `'table'`, both
`current_location='Fixed'`) and it has **no** corresponding `obsolete_details` row — a
§11.4.90 mandate violation (100% of Obsolete items lack the required detail row).

### 4. Items missing type or status
**0 rows** — clean. (The schema's `NOT NULL` + `CHECK (... IN (...))` constraints on both
`type` and `status` make this structurally hard to violate via the DB layer itself.)

## TOP FINDINGS

- F-D4-01 | High | HXC-003's CONST-046 anti-bluff gate (`scripts/audit-const046-hardcoded-content.sh --fail-on-new`) is portability-broken: `scripts/audit_const046/main.go:319-320` keys baseline comparison on the full absolute scan path, but `.baseline.json` stores paths from a different host's mount (`/run/media/milosvasic/DATA4TB/...`) — live-run on this checkout got 19098/19098 false "NEW" hits and exit 1, contradicting the closure claim that the gate "is enforced" and "shrinks monotonically."
- F-D4-02 | Med | §11.4.19 atomic-move drift: 11 terminal-status items (HXC-013,014,014b,015,018,019,024,025,026,027,028) have their full entry still physically present in `docs/Issues.md` with no counterpart in `docs/Fixed.md` — the mandated move-on-close never happened for this batch.
- F-D4-03 | Low | §11.4.90 violation: `obsolete_details` table is 100% empty; the sole Obsolete item (HXC-044) has no required detail row.
- F-D4-04 | Low | §11.4.171 violation: 31 Feature/table rows (all `FIX-2026-05-19#N` i18n-migration-round items) have `description` under the 50-char floor — terse closure-style fragments instead of a real description.
- F-D4-05 | Low | Severity column is effectively unused for closed items: all 79 distinct Feature/table rows have `severity=''`, making severity-based triage/risk-ordering (§11.4.132) impossible for this item class as currently populated.
- F-D4-06 | Low | FIX-2026-05-19#69's claimed builders live in `submodules/llm_orchestrator` (a separate submodule), not under `$INNER` (`helix_code/`) — the closure's file-scope claim is technically outside the audited inner app; work confirmed present, just in an adjacent codebase.

No NOT-FOUND ("closed-but-no-code") items surfaced in this 25-item sample.
