# helix_code/internal CONST-046 hardcoded-content classification

| Field             | Value                                              |
|-------------------|----------------------------------------------------|
| Revision          | 1                                                  |
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

## Precedent

- Round-321 — HelixLLM `openai.go`: LLM prompt templates ruled OUT of
  CONST-046 scope.
- Round-326 — LLMsVerifier `verifier.go`: same ruling reaffirmed.
- Round-177 — `internal/server` i18n seam established (round-70 of
  the CONST-046 Phase-4 numbering).
