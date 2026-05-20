# helix_code/internal CONST-046 hardcoded-content classification

| Field             | Value                                              |
|-------------------|----------------------------------------------------|
| Revision          | 2                                                  |
| Created           | 2026-05-20                                         |
| Last modified     | 2026-05-20                                         |
| Status            | active                                             |
| Status summary    | —                                                  |
| Issues            | docs/Issues.md                                     |
| Issues summary    | docs/Issues_Summary.md                             |
| Fixed             | docs/Fixed.md                                      |
| Fixed summary     | docs/Fixed_Summary.md                              |
| Continuation      | docs/CONTINUATION.md                               |

## Table of contents

- [Purpose](#purpose)
- [Audit scope](#audit-scope)
- [Classification key](#classification-key)
- [Per-file classification](#per-file-classification)
- [Round-350 migration](#round-350-migration)
- [Round-462 cmd+internal final sweep](#round-462-cmdinternal-final-sweep)
- [Precedent](#precedent)

## Purpose

The CONST-046 audit (`scripts/audit-const046-hardcoded-content.sh`)
reports ~1484 hardcoded-content hits under `helix_code/internal`
(non-test). Round-346 confirmed these are genuine production
literals, not test noise. This document classifies the top files so
future rounds and the audit-gate baseline reflect reality and do not
re-examine out-of-scope hits indefinitely.

CONST-046 governs **user-facing** content (UI text, prompts shown to
the operator, error messages surfaced to end users, labels, helper
text). It does NOT govern strings addressed to an LLM, nor
developer-facing wrapped-error / log strings.

## Audit scope

Total `helix_code/internal` non-test hits at round-350: **1484**.
Top packages by hit count:

| Package / file                              | Hits | Class |
|---------------------------------------------|------|-------|
| `server/handlers.go`                        | 112  | C (migrated round-350) |
| `workflow/planmode/*`                       | 70   | B |
| `performance/optimizer.go`                  | 58   | B |
| `memory/providers/*`                        | 52   | B |
| `workflow/autonomy/*`                       | 48   | B |
| `commands/builtin/*`                        | 43   | C (deferred — see below) |
| `workflow/executor.go`                      | 39   | B |
| `editor/formats/*`                          | 30   | B |
| `deployment/production_deployer.go`         | 30   | B |
| `providers/ai_integration.go`               | 27   | B |
| `repomap/tree_sitter.go`                    | 26   | B |
| `agent/base_agent.go`                       | 22   | A |
| `agent/profiles/profile.go`                 | ~12  | A |
| `agent/types/*`                             | 19   | A |

## Classification key

- **(A) LLM prompt templates** — text addressed to the *model*, not
  the operator. OUT of CONST-046 scope per round-321 (HelixLLM
  `openai.go`) and round-326 (LLMsVerifier `verifier.go`) precedent.
  An LLM prompt is consumed by an English-trained model regardless of
  the operator's locale; translating it would degrade output quality.
- **(B) Wrapped-error / log / developer-facing tech strings** — strings
  that appear in `fmt.Errorf` wrapping, structured logs, or
  developer-diagnostic output. OUT of CONST-046 scope: these are read
  by developers/operators inspecting logs, not surfaced as localized
  product UI. CONST-046's i18n mandate targets the product surface.
- **(C) Genuine user-facing UI text** — error messages returned in
  API responses, CLI command descriptions/help, labels. IN scope —
  migrate through the package's i18n seam.

## Per-file classification

### (A) LLM prompt templates — OUT of scope

- **`agent/base_agent.go`** — `lines 471-668`: "Analyze the following
  requirements and create a detailed technical plan", "Generate code
  according to these requirements:", "You are a %s agent named %s...",
  etc. These are prompt strings passed to `provider.Generate`. The
  no-LLM fallback strings at `lines 326-347` ("No-LLM fallback plan:
  requirements echoed verbatim", "Basic analysis without LLM") are
  borderline — they are diagnostic status text, classified (B).
- **`agent/profiles/profile.go`** — `lines 171-180+`: "You are in
  VERIFIER mode. Your task is to review...", "Fabricate findings:
  every issue you raise MUST be grounded...". These compose the
  system prompt sent to the model. OUT of scope.
- **`agent/types/coding_agent.go`, `debugging_agent.go`** — "You are a
  code generation agent...", "You are a debugging agent...". Prompt
  preambles addressed to the model. OUT of scope.

### (B) Wrapped-error / log / developer tech strings — OUT of scope

- **`workflow/planmode/executor.go`** — "Starting execution",
  "Skipped: %s (dependencies not met)", "Successfully wrote %d bytes
  to %s". Step-execution log/status output for developer diagnostics.
- **`workflow/autonomy/controller.go`** — "Mode changed: %s -> %s",
  "Escalation approved: %s", "De-escalated to mode: %s". Autonomy
  controller state-transition log lines.
- **`workflow/executor.go`** — "Project Architecture Planning",
  "Build and compile project", "Run comprehensive test suite". These
  are internal workflow-step *identifiers/labels* composed into
  workflow definitions, not localized end-user UI.
- **`performance/optimizer.go`** — "CPU Goroutine Pool", "Implement
  goroutine pool for CPU-intensive operations". Optimization-strategy
  recommendation records consumed by performance tooling / reports.
- **`memory/providers/anima_provider.go` (+ siblings)** — "Provider is
  operating normally", "Provider has not been initialized", "Anima AI
  Memory Provider". Health-check status strings + provider
  display-name metadata in developer-facing diagnostics.
- **`agent/subagent/*`** — "subagent subprocess: marshal task: %v",
  "failed to construct LLM provider: %v", "InProcessSpawner channel
  closed without result". Wrapped-error tech strings.
- **`adapters/speckit_debate_adapter/adapter.go`** — "## Rounds
  conducted: %d", "- FOR: agent %s (provider=%s ...)". Markdown
  report-assembly fragments for developer-facing debate transcripts.

### (C) Genuine user-facing UI text — IN scope

- **`server/handlers.go`** — JSON API error-response `"message"`
  fields ("Authentication required", "Invalid request", "Task manager
  not available", etc.). Observable by every API consumer (CLI,
  desktop, mobile, third-party). **Migrated in round-350.**
- **`commands/builtin/*`** — slash-command `Description()` / `Usage()`
  return values ("Summarize and condense conversation history to save
  tokens", "/condense [options]", etc.). Genuinely user-facing CLI
  help text. **Deferred**: the `internal/commands/builtin` package is
  a sub-package of `internal/commands`; the `commands.tr()` seam is
  unexported and not reachable from `builtin`, and `Description()` /
  `Usage()` are context-free synchronous methods. Migrating this
  surface requires a dedicated seam refactor (own `builtin/i18n`
  package + context plumbing) — scoped to a future round to keep
  round-350 focused and low-risk.

## Round-350 migration

`server/handlers.go`: 16 distinct user-facing JSON `"message"`
literals migrated to the existing `internal/server` i18n seam
(`internal/server/i18n_seam.go` + `i18n/bundles/active.en.yaml`),
covering ~73 call sites. Six bundle IDs from round-177 that were
declared but never wired into handler code are now wired; ten new IDs
added. A nil-safe `reqCtx(c)` helper was added to the seam, which
also repairs a pre-existing latent nil-`Request` panic in
`qa_handlers.go` (round-70 wiring assumed a non-nil `c.Request`;
`gin.CreateTestContext` leaves it nil). Paired-mutation tests added
in `i18n_seam_test.go`.

## Round-462 cmd+internal final sweep

Final confirm-or-NOOP sweep of `helix_code/cmd/` + `helix_code/internal/`
(2026-05-20). One genuine (C) residual was located and migrated; the
remainder of the audit surface in both trees is confirmed out-of-scope.

**Migrated — `cmd/local_llm.go`:** the five *advanced* discovery/analytics
cobra subcommands (`discoverCmd`, `recommendCmd`, `analyticsCmd`,
`reportCmd`, `insightsCmd`) carried plain `Short:` / `Long:` English
literals — 10 literals total. These are `--help` text genuinely surfaced
to operators and were the only `Short:`/`Long:` literals in all of
`helix_code/cmd/*.go` not already routed through `trc()`. Migrated to the
`cmd` i18n seam (`cmd/i18n_seam.go`): 10 new bundle IDs in
`cmd/i18n/bundles/active.en.yaml` (`cmd_local_llm_{discover,recommend,
analytics,report,insights}_{short,long}`). Paired-mutation tests added to
`cmd/local_llm_i18n_test.go` (`round462CobraMetadataIDs` +
`TestLocalLLMI18n_Round462CobraMetadata{RoutesThroughSeam,NoopEcho}`).

**Confirmed exhausted — out-of-scope residual:**

- `cmd/local_llm.go:580` — `"watch: fsnotify error: %v\n"`: class (B)
  wrapped-error / log diagnostic.
- `cmd/local_llm_advanced.go` — `"Llama 3 8B Instruct"`,
  `"Mistral 7B Instruct"`, etc.: model-identifier metadata tokens, not
  localizable UI; `"Total Models\t%d\n"` etc.: tab-separated report
  table fragments (class B).
- `cmd/performance_optimization/main.go`,
  `cmd/security_fix_standalone/main.go`,
  `cmd/security_scan/main.go` — recommendation-record / scanner-finding
  struct fields + report-template strings (round-460 documented these
  as out-of-scope: scanner-diagnostic struct fields + report templates).
- `cmd/cli/main.go` — pprof diagnostic strings (class B).
- `internal/*` — the top packages (`tools`, `llm`, `workflow`, `agent`,
  `memory`, `performance`, `server`, etc.) were classified A/B in
  revision 1; `agent/base_agent.go` lines 525-722 are class (A) LLM
  prompt templates; `adapters/speckit_debate_adapter/adapter.go` markdown
  report fragments are class (B). The genuine (C) surface
  (`server/handlers.go` JSON `message` fields, `commands/builtin/*`
  `Description()`/`Usage()`) was migrated in earlier rounds — the
  `internal/commands/builtin` package now has its own `builtin/i18n` +
  `builtin/translator.go` seam and every `Description()`/`Usage()`
  returns `trc("builtin_*_...", nil)`.

**Verdict:** `helix_code/cmd/` and `helix_code/internal/` genuine
user-facing (C) hardcoded-content surface is now exhausted. Remaining
audit hits are all class (A) LLM prompts, class (B) wrapped-error / log /
report-template strings, or identifier/format-spec tokens.

## Precedent

- Round-321 — HelixLLM `openai.go`: LLM prompt templates ruled OUT of
  CONST-046 scope.
- Round-326 — LLMsVerifier `verifier.go`: same ruling reaffirmed.
- Round-177 — `internal/server` i18n seam established (round-70 of
  the CONST-046 Phase-4 numbering).
- Round-462 — `cmd/local_llm.go` advanced-subcommand cobra Short/Long
  migrated; cmd+internal genuine (C) surface confirmed exhausted.
