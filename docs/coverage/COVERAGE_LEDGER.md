# HelixCode CONST-048 Coverage Ledger — Submodule × Feature × Platform × Invariant

**Schema:** `SCHEMA_VERSION = 1.0.0` (see `SCHEMA.md`)
**Last regenerated:** 2026-05-18 (round 68 initial publication)
**Source roster:** `docs/improvements/submodule_owned.txt` (68 owned submodules)
**Companion ledger:** `docs/coverage/ledger.md` (feature × invariant rollup, round 41+)
**Generator:** `scripts/generate-coverage-ledger.sh`
**Mandate:** CONST-048 (Full-Automation-Coverage) / constitution §11.4.25; CONST-035 anti-bluff posture; Article XI §11.9

## How to read this ledger

Each row = `(Submodule, Feature, Platform)`. Six invariant cells per row (I1..I6) hold one of: `PASS`, `PENDING_FORENSICS:`, `OPERATOR-BLOCKED:`, `UNCONFIRMED:`, `SKIP-OK: #<ticket>`, `PARTIAL`, `N/A`. Overall column is mechanical rollup. Notes column carries the evidence reference for any PASS marker.

Round 68 deliverable = **structure + honest conservative initial population**. Default cell is `UNCONFIRMED:`. PASS markers only where CONTINUATION.md round narrative documents captured evidence. Subsequent rounds promote `UNCONFIRMED:` → `PASS` by adding the round reference in Notes.

Auto-marking everything PASS would itself be a CONST-035 PASS-bluff at the governance layer — explicitly forbidden by the round 68 brief.

## CONST-048 invariants

| # | Invariant |
|---|-----------|
| I1 | Anti-bluff posture with captured runtime evidence (CONST-035 / Article XI §11.9) |
| I2 | Proof of working capability end-to-end on target topology (CONST-050(A) — no mocks beyond unit tests) |
| I3 | Implementation matches the documented promise (§11.4.12) |
| I4 | No open issues/bugs surfaced (Issues.md cross-check + source-tree `TODO(round-N)` / `BUG #NN:` grep) |
| I5 | Full documentation in sync (§11.4.12) |
| I6 | Four-layer test floor (pre-build + post-build + runtime + paired mutation) |

## Coverage rollup (round 68 baseline)

| Metric | Count |
|--------|-------|
| Owned submodules (roster) | 68 |
| Rows in ledger | 68 (one per submodule, `whole-module` feature row each) |
| PASS Overall | 0 |
| PARTIAL Overall | 8 |
| PENDING_FORENSICS Overall | 0 |
| OPERATOR-BLOCKED Overall | 0 |
| UNCONFIRMED Overall | 60 |

Conservative posture per CONST-035: no full-row PASS until every invariant for every platform has captured evidence. PARTIAL rows hold the documented-PASS work from CONTINUATION rounds 23/37/41/53/60/62/63/64/65 (the round-specific evidence appears in the cell + Notes).

## Ledger rows

### Submodules with documented PASS-on-some-invariants (PARTIAL Overall)

| Submodule | Feature | Platform | I1 anti-bluff | I2 e2e-working | I3 doc-match | I4 no-issues | I5 doc-sync | I6 4-layer-tests | Overall | Notes |
|-----------|---------|----------|---------------|----------------|--------------|--------------|-------------|------------------|---------|-------|
| dependencies/vasic-digital/auto_temp | grid-search temperature tuning | linux,macos,windows | PASS (round 23 fix 127be62) | PASS (round 37 wired Real backends) | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | PARTIAL | PARTIAL | round 23 + round 37 per CONTINUATION; needs I4/I5/I6 audit |
| dependencies/vasic-digital/storage | recording S3 sync | linux,macos | PASS (round 37 acecf73) | PARTIAL (round 37 env-gated SKIP-OK on real S3) | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | PARTIAL | PARTIAL | round 37 per CONTINUATION; real-S3 integration env-gated |
| dependencies/HelixDevelopment/llm_orchestrator | OpenCode CLI adapter | linux,macos | PASS (round 64 a9493fa) | PASS (round 64 real opencode v1.14.41 invocation) | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | PARTIAL (16 tests + 1 SKIP-OK windows) | PARTIAL | round 64 per CONTINUATION: 106 PASS / 0 FAIL / 0 SKIP unit; real binary integration test passed; Windows job-object deferred to round 65+ |
| dependencies/HelixDevelopment/llm_orchestrator | SimpleAgentPool + 5 builder factories | all-platforms | PASS (round 50+ per CONTINUATION) | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | PARTIAL | PARTIAL | per CONTINUATION pool wiring; E2E topology evidence pending |
| dependencies/HelixDevelopment/helix_specifier | real DebateFunc | all-platforms | PASS (round 65 f2cb17a) | PASS (round 65 LLMResponder interface + LLMBackedDebateFunc) | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | PARTIAL (paired-mutation gate landed) | PARTIAL | round 65 per CONTINUATION; closes ErrDebateFuncNotConfigured (round 28); Option B chosen per CONST-051(B) |
| dependencies/HelixDevelopment/llm_provider | provider error sentinel coverage (17 providers) | all-platforms | PASS (round 63 5171edf) | PASS (round 63 full llm pkg PASS 52.868s) | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | PARTIAL (17 new tests + 21 subtests) | PARTIAL | round 63 100% coverage milestone per CONTINUATION; closes round-46 deferred providers |
| dependencies/HelixDevelopment/llms_verifier | provider/model verification (CONST-036) | all-platforms | PASS (CONST-036/037 wiring per ledger.md) | PARTIAL (groq+openrouter+mistral+deepseek live-verified per ledger.md F12) | PARTIAL | UNCONFIRMED: | UNCONFIRMED: | PARTIAL | PARTIAL | cross-ref ledger.md F12; 10 providers unprobed (no keys / Gemini invalid / Ollama+LlamaCPP local not running) |
| dependencies/HelixDevelopment/models | model metadata source | all-platforms | PASS (per CONST-036 ledger) | PARTIAL (per LLMsVerifier integration above) | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | PARTIAL | wired to LLMsVerifier per CONST-036; depth audit pending |

### All other owned submodules — UNCONFIRMED baseline (round 68)

Each of the rows below is a single `whole-module` placeholder. Future rounds split into per-feature rows when audit work captures evidence. Until then, the rows hold UNCONFIRMED across every invariant — which is the honest position per CONST-035 ("never claim PASS without captured evidence").

| Submodule | Feature | Platform | I1 anti-bluff | I2 e2e-working | I3 doc-match | I4 no-issues | I5 doc-sync | I6 4-layer-tests | Overall | Notes |
|-----------|---------|----------|---------------|----------------|--------------|--------------|-------------|------------------|---------|-------|
| challenges | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; needs per-Challenge-script row split |
| containers | whole-module | linux,containers | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; CONST-045 host-enrolment lives here |
| dependencies/HelixDevelopment/doc_processor | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/HelixDevelopment/helix_llm | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/HelixDevelopment/helix_memory | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/HelixDevelopment/vision_engine | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/agentic | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/auth | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; JWT/bcrypt/argon2 stack |
| dependencies/vasic-digital/background_tasks | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; cross-ref ledger.md F07 |
| dependencies/vasic-digital/benchmark | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/cache | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; Redis backend |
| dependencies/vasic-digital/claritas | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/concurrency | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/config | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; Viper config stack |
| dependencies/vasic-digital/conversation | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/database | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; PostgreSQL via pgx/v5 |
| dependencies/vasic-digital/doc_processor | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline (vasic-digital fork) |
| dependencies/vasic-digital/document | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/embeddings | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/event_bus | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/filesystem | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/formatters | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/gandalf_solutions | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/hyper_tune | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/i18n | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; CONST-046 hardcoded-content sink |
| dependencies/vasic-digital/i_llm | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/lazy | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/leak_hub | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/llm_ops | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/llm_orchestrator | whole-module (vasic-digital fork) | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; HelixDevelopment fork has documented work above |
| dependencies/vasic-digital/llm_provider | whole-module (vasic-digital fork) | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; HelixDevelopment fork has documented work above |
| dependencies/vasic-digital/mcp_module | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; cross-ref ledger.md F06 |
| dependencies/vasic-digital/memory | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; cross-ref ledger.md F24 |
| dependencies/vasic-digital/messaging | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/middleware | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/models | whole-module (vasic-digital fork) | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; HelixDevelopment fork above has documented work |
| dependencies/vasic-digital/normalize | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/observability | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; OpenTelemetry exporters |
| dependencies/vasic-digital/optimization | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/ouroborous | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/planning | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; cross-ref ledger.md F08/F25 |
| dependencies/vasic-digital/plinius_common | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/plugins | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; CONST-040 capability flag |
| dependencies/vasic-digital/rag | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; CONST-040 capability flag |
| dependencies/vasic-digital/rate_limiter | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/recovery | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/red_team | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/self_improve | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/skill_registry | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; cross-ref ledger.md F10 |
| dependencies/vasic-digital/streaming | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/tool_schema | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/toon | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/vector_db | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/veritas | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/vision_engine | whole-module (vasic-digital fork) | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| dependencies/vasic-digital/watcher | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; fsnotify wrapper |
| github_pages_website | whole-module | headless | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; static marketing site |
| helix_agent | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |
| helix_qa | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; CONST-050(B) test-bank harness |
| panoptic | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline; cross-cutting Challenge bank |
| security | whole-module | all-platforms | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | UNCONFIRMED: | round 68 baseline |

## Cross-reference points

- **CONTINUATION.md** — canonical source of round-by-round PASS justifications. Cite `round NN` in Notes for any PASS marker.
- **ledger.md** — feature × invariant rollup (F01..F30 catalogue). Cross-check that PASS rows here align with F-cell PASS in ledger.md.
- **`docs/improvements/submodule_owned.txt`** — canonical owned-submodule roster; every entry MUST have ≥1 row here.
- **`scripts/generate-coverage-ledger.sh --check`** — enforces submodule coverage + schema invariants; CI gate.
- **Issues.md** (per-submodule, future) — feeds I4 once introduced per CONST-057 / §11.4.16.

## How PASS markers get promoted (round N+ workflow)

1. Round agent completes work on a submodule.
2. Round agent captures evidence (test logs, integration runs, real-binary invocations).
3. Round agent updates CONTINUATION.md with round narrative + commit SHA.
4. Round agent (or operator) edits this ledger:
   - Find the row for `(submodule, feature, platform)` — or split a `whole-module` row into per-feature rows.
   - Promote `UNCONFIRMED:` → `PASS` for the specific invariant the work addresses.
   - Add to Notes: `round NN <SHA> + brief evidence pointer`.
5. Commit ledger update in the same commit as the work (per CONST-044 CONTINUATION-sync discipline).
6. `bash scripts/generate-coverage-ledger.sh --check` MUST pass.

## Anti-bluff guarantees (round 68)

1. Default cell = `UNCONFIRMED:` — never `PASS`. (60 of 68 rows are UNCONFIRMED Overall at round 68 publication — honest baseline.)
2. PASS marker requires either CONTINUATION round reference or pasted-evidence path in Notes (enforced by `--check` mode of generator).
3. Every owned submodule has ≥1 row (enforced by `--check`).
4. Status vocabulary is closed (enforced by `--check`).
5. Schema version bump on column changes (enforced by `SCHEMA.md` discipline).
6. PASS-by-default is forbidden — it would be a CONST-035 PASS-bluff at the governance layer, equivalent severity to a false-success test result per Article XI §11.9.

## Operator mandate (verbatim, 2026-05-19, preserved per CONST-049 §11.4.17)

> "all existing tests and Challenges do work in anti-bluff manner - they MUST confirm that all tested codebase really works as expected! We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completition and full usability by end users of the product!"

This ledger is the structural artefact that makes the above mandate auditable per-submodule, per-feature, per-platform, per-invariant.

## Audit trail

| Date | Author | Round | Schema | Notes |
|------|--------|-------|--------|-------|
| 2026-05-18 | Claude Opus 4.7 | round 68 | 1.0.0 | Initial publication. 68 rows (one per owned submodule). 8 PARTIAL Overall (AutoTemp, Storage, LLMOrchestrator×2, HelixSpecifier, LLMProvider, LLMsVerifier, Models — all backed by CONTINUATION rounds 23/37/50/63/64/65). 60 UNCONFIRMED Overall (honest baseline). Closes 50+ round deferred governance debt for CONST-048 ledger structure. Conservative population per CONST-035 anti-bluff posture. |

## Sources verified 2026-05-29: internal-governance doc — no third-party-service operator instructions to cross-reference. This ledger is HelixCode's own CONST-048 submodule × feature × platform × invariant tracker; it references internal constitutional anchors plus a few third-party identifiers used as captured-evidence records, not as instructions: `opencode v1.14.41` (round-64 evidence of a real binary invocation — a historical evidence pin, deliberately not re-bumped), `pgx/v5` / `Redis` / `OpenTelemetry` (stack-description notes). Per CONST-036, the LLM provider/model identifiers in these rows are LLMsVerifier-sourced at runtime (`helixcode llm models list`) — they are NOT pinned here and are deliberately left unmodified. Version authority per CLAUDE.md §3.1 (confirmed in-tree): inner module `go 1.26`, root `go 1.25.2`; PostgreSQL 15+; Redis 7+ — no stale build/Docker version pins in this doc to correct. Reviewed against the live tree on this date; no corrections required.
