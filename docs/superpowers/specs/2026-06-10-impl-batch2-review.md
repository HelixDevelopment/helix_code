# Wave-2 Batch — Independent Code-Review Report (§11.4.125 / §11.4.142)

- **Date:** 2026-06-10
- **Reviewer:** Independent code-review subagent (READ-ONLY on code; no edits/commits/push)
- **Repo root:** `/Volumes/T7/Projects/HelixCode`
- **Scope:** UNCOMMITTED wave-2 batch — SP4 (helix_agent real os/exec), SP1-cont
  (helix_code/internal/llm dynamic GetModels + keyrecognition + honest-empty),
  F1 (git hooks), docs-SQL (architecture docs).

---

## VERDICT: GO-WITH-FIXES

The batch is functionally sound, anti-bluff-genuine, and free of regressions. All
required verifications PASS. Two non-blocking correctness findings and one
scope-hygiene finding warrant attention but do NOT block the commit. There are
**zero BLOCKERs**.

- **Finding counts:** 0 BLOCKER · 2 SHOULD-FIX (non-blocking) · 2 NICE-TO-HAVE · 1 SCOPE-NOTE

---

## 1. Builds + vet — PASS

```
helix_agent: go build ./internal/clis/...   BUILD_EXIT=0
helix_agent: go vet   ./internal/clis/...    VET_EXIT=0
helix_code : go build ./internal/llm/...     BUILD_EXIT=0
helix_code : go vet   ./internal/llm/...     VET_EXIT=0
```

## 2. RED→GREEN real — PASS

SP4 standing pin guards (default GREEN polarity):
```
--- PASS: TestD6_QwenCode_GenerateExecsRealBinary (0.45s)   [fake-binary marker present, template absent, prompt forwarded]
--- PASS: TestD6_QwenCode_AbsentBinaryIsHonestError (0.00s)
--- PASS: TestD9_InstanceManager_ExecuteExecsRealBinary (aider/claude_code/codex/cline/openhands all PASS)
--- PASS: TestD9_InstanceManager_AbsentBinaryIsHonestError (all 5 PASS)
```
SP4 under `PIN_STUB_BLUFF=1` (§11.4.115 RED reproduction; honest-error path on fixed artifact) — **PASS**:
```
--- PASS: TestD6_QwenCode_GenerateIsStubBluff (1.08s)
--- PASS: TestD9_InstanceManager_ExecuteStubsAreBluffs (9.48s)   [claude_code subtest exec'd the real `claude` on PATH → non-literal output → bluff absent]
```
SP1-cont new tests (RED_MODE=0 default GREEN):
```
--- PASS: TestKeyRecognition_MultiAliasTable_T112
--- PASS: TestKeyRecognition_PresentProviders_AliasMatch_T112
--- PASS: TestKeyRecognition_PresentProviders_AbsenceAndPlaceholder_T112
--- PASS: TestKeyRecognition_IsPlaceholder_T112
--- PASS: TestFetchExternalModels_NoVerifier_HonestEmpty_D5
--- PASS: TestOpenAI_DynamicModels_D5 / DeepSeek / Mistral / Anthropic _DynamicModels_D5
```
F1 hook suite:
```
RESULT: 27 PASS / 0 FAIL   (includes the embedded paired §1.1 mutation test)
```

## 3. Anti-bluff soundness — PASS (guards are genuine, not tautologies)

- **SP4 §1.1 live mutation (performed + reverted):** reverted `qwencode.go` `generate()`
  to its templated literal → `TestD6_QwenCode_GenerateExecsRealBinary` **FAILED** with
  "marker absent … BLUFF-001 reintroduced". Source restored + rebuilt clean. Guard
  is real.
- **SP1-cont polarity (verified):** `RED_MODE=1` on the FIXED tree **FAILS**
  `TestFetchExternalModels_NoVerifier_HonestEmpty_D5` + all four `*_DynamicModels_D5`
  (the fix is present, so the RED-reproduction assertion correctly inverts) — proving
  the polarity switch is genuinely wired, not a constant-true test.
- **F1 mutation:** suite's own paired mutation ("broken sibling-assertion no longer
  blocks" → restored "blocks again") PASSED inside the 27/0 run.
- **No INTRODUCED bluffs:** scan of `+` lines across all production source (helix_code
  llm + helix_agent clis, new + modified, excluding `_test.go`) for
  `simulated|for now|TODO implement|in production this would` → **clean**. The only
  `+` match anywhere is a doc-comment in `instance_manager.go:913` that QUOTES the
  removed bluff (`"<Agent> execution completed"`), which the constitutional
  anti-bluff smoke-check explicitly excludes.
- **Pre-existing `instance_manager.go:389` "For now"** confirmed NOT newly introduced
  (it appears only as unchanged diff context; the diff touches lines ~906–1030 only).

## 4. No regression — PASS

- Full `dev.helix.code/internal/llm` suite: **ok (63.07s)**, 0 fail.
- Full `dev.helix.agent/internal/clis/...` suite: **0 FAIL** across all packages
  (qwencode 12.2s incl. new guards; aider/claude/codex/cline/kiro/openhands all ok).
- **no-network-on-construction contract preserved:** `NewOpenAIProvider` /
  `NewDeepSeekProvider` / `NewMistralProvider` call only `initializeModels()` (seed,
  no network); `NewAnthropicProvider` seeds via `getAnthropicModels()`. The live
  `/models` fetch is deferred to the first `GetModels()` call via `sync.Once`
  (`refreshCatalogOnce`). No constructor calls `refreshCatalogOnce` / `fetchModelCatalog`.
- **model_discovery honest-empty reconciled, not deleted:** the removed `verifier`
  import is no longer referenced in `model_discovery.go`; `ConvertVerifiedToModelInfo`
  remains used by `verifier_integration.go:32` (not orphaned); the sibling behaviour
  is covered by the NEW `model_discovery_fallback_test.go` (reconciliation test), so
  no test was silently deleted (§11.4.120 satisfied).
- **F1 hooks NOT installed into `.git/hooks`:** `.git/hooks/{pre-commit,commit-msg,post-commit}`
  all ABSENT; `submodules/helix_agent/.git/hooks` clean. `install_git_hooks.sh` is
  operator-gated ("NOT run automatically"); `test_hooks.sh` exercises hooks in
  isolated temp dirs only.

## 5. Scope + CONST-042 — PASS

- **No `go.mod` / `go.sum` edits** in root, helix_code, or helix_agent.
- **Lane discipline:** SP4 → helix_agent only; SP1-cont → helix_code/internal/llm
  only; F1 → scripts/git_hooks + installer; docs-SQL → docs/architecture. No
  cross-lane bleed.
- **CONST-042 no-secret-leak:** `keyrecognition.go` does NO logging of any kind
  (presence-only; reads env, never prints a value). Provider refresh `log.Printf`
  lines log only model COUNTS — none reference key/token/secret/apiKey. In
  `openai_compatible_catalog.go` the `apiKey` flows ONLY into the
  `Authorization: Bearer` header, never logged. No secret literals in the new docs.
- **docs-SQL:** schema self-declares "Target engine: PostgreSQL 15+ … DESIGN — NOT a
  deployed migration" with a §11.4.44 revision header. sqlite3 parse "errors" are
  expected Postgres-dialect mismatch (`TIMESTAMPTZ`, `COMMENT ON TABLE`, `now()`),
  not real defects.

---

## Findings

### SHOULD-FIX (non-blocking)

1. **`keyrecognition.go:90` — over-broad placeholder token `"xxx"`.**
   `placeholderTokens` contains the bare substring `"xxx"`, matched case-insensitively
   via `strings.Contains`. Any REAL provider key that happens to contain `xxx` as a
   substring would be wrongly classified as a placeholder and the provider reported
   absent (false-negative key presence). `sk-xxx` in the same list is already
   subsumed by `"xxx"` (redundant). Errs toward honest-empty (safe direction, not a
   false-positive), hence non-blocking — but tighten to an anchored/standalone match
   (e.g. exact `"xxx"`/`"sk-xxx"` token or a regex) so legitimate keys are not dropped.
   File:line — `helix_code/internal/llm/keyrecognition.go:90`.

2. **`provider_dynamic_models_test.go` — does not assert the construction contract.**
   The four `*_DynamicModels_D5` tests build providers via struct literal + direct
   `initializeModels()`, never via `New*Provider`, so they prove the live-fetch path
   but do NOT regression-guard "no network at construction." The contract is correct
   by design (verified manually in §4), but a future refactor that moved the fetch
   into the constructor would not be caught by these tests. Add one assertion that
   `New*Provider` performs no `/models` request (e.g. an httptest server with a
   request counter that must read 0 immediately after construction).
   File — `helix_code/internal/llm/provider_dynamic_models_test.go`.

### NICE-TO-HAVE

3. **Partial executor conversion in `instance_manager.go` (pre-existing, in-scope-adjacent).**
   This batch converts 5 executors to real exec (Aider, ClaudeCode, Codex, Cline,
   OpenHands) plus Qwen via `qwencode.go`. The remaining ~34 `execute*` methods
   (`executeKiro` … `executeLlamaCode`, lines ~1033–1270+) STILL return hardcoded
   `"<Agent> execution completed"` templates — pre-existing stub bluffs, NOT newly
   introduced by this batch (the diff only removes 5 such templates). Out of the
   stated D-6/D-9 scope, but these remain live BLUFF-003-class stubs and should be
   tracked for a follow-up wave so the conversion is not left half-applied.

4. **`openai_compatible_catalog.go` default token limits are heuristic seeds.**
   `defaultContext`/`defaultMaxTokens` (e.g. OpenAI 128000/4096) are applied uniformly
   to every live-fetched model id. Acceptable as a metadata seed (the model NAMES are
   live per CONST-036), but per-model context/limits remain approximate until the
   verifier funnel enriches them. Documented honestly in-comment; no action required
   beyond awareness.

### SCOPE-NOTE

5. **Untracked `docs/caf/` (`Status.md`, `map.tsv`) is NOT part of the 4 stated streams.**
   It is a repo-fork PLAN log (`gh repo fork … --org vasic-digital …`) from unrelated
   work present in the working tree. Harmless and untracked, but it must NOT be swept
   into this batch's commit (§11.4.84 quiescence / §11.4.30 staging discipline) — stage
   the batch files explicitly, never `git add -A`.

---

## Captured build/test PASS-FAIL summary

```
BUILD  helix_agent  go build ./internal/clis/...      PASS (exit 0)
VET    helix_agent  go vet   ./internal/clis/...       PASS (exit 0)
BUILD  helix_code   go build ./internal/llm/...        PASS (exit 0)
VET    helix_code   go vet   ./internal/llm/...         PASS (exit 0)
TEST   SP4 pin guards (default GREEN)                  PASS (D6 + D9, all subtests)
TEST   SP4 pin guards PIN_STUB_BLUFF=1                 PASS (D6 + D9)
TEST   SP4 §1.1 live mutation (revert generate→stub)   FAIL-as-required → restored → PASS (guard genuine)
TEST   SP1-cont new tests (RED_MODE=0)                 PASS (KeyRecognition×4, HonestEmpty_D5, DynamicModels×4)
TEST   SP1-cont polarity RED_MODE=1 (fixed tree)       FAIL-as-required (×5 → polarity wired, not tautology)
TEST   F1 scripts/git_hooks/test_hooks.sh              PASS (27 PASS / 0 FAIL, incl. paired mutation)
TEST   FULL internal/llm regression                    PASS (ok, 63.07s, 0 fail)
TEST   FULL internal/clis regression                   PASS (0 FAIL across all packages)
CHECK  .git/hooks NOT polluted                         PASS (pre-commit/commit-msg/post-commit absent)
CHECK  go.mod/go.sum untouched                         PASS (none)
CHECK  anti-bluff INTRODUCED-line scan                 PASS (clean; only a quoted-bluff doc-comment)
CHECK  CONST-042 no-secret-leak                        PASS (presence-only; apiKey header-only, never logged)
```

**VERDICT: GO-WITH-FIXES** — 0 BLOCKER, 2 SHOULD-FIX, 2 NICE-TO-HAVE, 1 SCOPE-NOTE.
