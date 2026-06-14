---
title: "QA Evidence — All ~/api_keys.sh Providers + Positive-Codebase-Answer + Real LLMsVerifier Consumption"
date: 2026-06-14
revision: 1
status: evidence-captured (anti-bluff, real facts only)
commit: a99a5e9d
---

# QA Evidence: All `~/api_keys.sh` Providers, Positive-Codebase-Answer Guarantee, and Real LLMsVerifier Consumption

**Date:** 2026-06-14
**Scope:** HelixCode (`helix_code/`) + HelixAgent + LLMsVerifier submodules
**Commit under test:** `a99a5e9d` — *feat(tui,llm,verifier): use all ~/api_keys.sh providers + guarantee positive codebase answer + real LLMsVerifier consumption*
**Anti-bluff posture (CONST-035 / Article XI §11.9):** This document states only what was observed in this session. Where a thing was NOT proven, it is explicitly flagged as such. **No API key value is printed anywhere in this document.**

---

## 1. Verdict

**PASS (with honest limitations).** Three independently-verified outcomes:

1. **Provider breadth** — HelixCode's TUI live provider count went **5 → 18**, with 8 newly-added OpenAI-compatible providers registering live models at startup and the remaining unreachable ones gracefully skipping (no crash, no fake entries).
2. **Positive-codebase-answer guarantee** — proven live: the agent issues a real tool call before any codebase claim and answered affirmatively with the real `git_status` file list (captured frame).
3. **Real LLMsVerifier consumption (CONST-036)** — proven: HelixCode in remote mode received **all 78 real models** populated in the verifier DB, not the 7 embedded fallback entries.

The following are explicitly **NOT** claimed as working and are documented in §7 (Honest Limitations): several free-tier keys are invalid/suspended; one provider hit a rate limit on a heavy prompt; one HelixAgent provider has no chat-entitled model yet; one key was excluded entirely.

---

## 2. What Changed

### 2.1 HelixCode (`helix_code/`)

- **`internal/llm/openai_compatible_catalogue.go`** (new, +254 lines) — a **data-driven catalogue** of hosted OpenAI-compatible providers. Base URLs are sourced from HelixAgent's verified provider table (not hand-invented). Each entry is keyed off an env var; a provider only activates when its key is present.
- **`GetType()` collision fix** — `GetType()` now derives a distinct type per catalogue provider, fixing a collision that previously conflated multiple OpenAI-compatible providers.
- **`applications/terminal_ui/env_providers.go`** (new, +20 lines) — a **registration loop** that registers every present-key catalogue provider into the TUI provider manager at startup.
- **`applications/terminal_ui/main.go`** (+48 lines) — the positive-codebase-answer changes (see §4): system-prompt instruction requiring a tool call before any codebase claim, `resolveRepoRoot` pinning `git_status` to the git root, and ensemble-first picker behaviour.
- Tests added alongside: `openai_compatible_catalogue_test.go` (+224), `env_providers_test.go` (+11).

### 2.2 HelixAgent (submodule)

- Builds and boots.
- From the same keyset, discovered a **25-provider / 993-model catalogue**.
- **6 providers live-verified**: cerebras, codestral, deepseek, groq, mistral, openrouter.
- A new **INFERENCE** provider was added but its account currently has no chat-entitled model (see §7).

### 2.3 LLMsVerifier (submodule)

- Builds and serves on **`:8095`**.
- DB populated with **78 real models**: groq 16, deepseek 4, mistral 58.
- The HelixCode verifier client was **reconciled** to the verifier's actual contract: envelope shape, `/api/providers` endpoint, Unix timestamp handling, and numeric `id` + `model_id` fields.

---

## 3. Provider-Support Matrix

Live model counts below are from the HelixCode TUI startup log captured this session. "Wired in HelixCode" = registered via the catalogue/registration loop at `a99a5e9d`.

| Provider     | Key present | Wired in HelixCode | Live models (startup) | Status |
|--------------|:-----------:|:------------------:|:---------------------:|--------|
| cerebras     | yes | yes | 2   | Registered live. Hit 429 RPM on heavy multi-turn prompt (see §7). |
| hyperbolic   | yes | yes | 5   | Registered live. |
| novita       | yes | yes | 138 | Registered live. |
| sambanova    | yes | yes | 6   | Registered live. |
| nvidia       | yes | yes | 121 | Registered live. |
| zai          | yes | yes | 7   | Registered live. |
| venice       | yes | yes | 90  | Registered live. |
| chutes       | yes | yes | 13  | Registered live. |
| siliconflow  | yes | yes | 69  | **Fixed** this session: base URL `.cn` → `.com`. Now registers 69 models. |
| fireworks    | yes | yes | 0   | Gracefully skipped (account suspended — 401/403/404/412 class). Documented key-side. |
| kimi         | yes | yes | 0   | Gracefully skipped (invalid key). Documented key-side. |
| upstage      | yes | excluded | — | Excluded: no `/models` endpoint to enumerate. |
| codestral    | yes | excluded | — | Excluded: no `/models` endpoint to enumerate. (Live-verified separately in HelixAgent.) |
| tencent      | invalid | no | — | `TENCENT_CLOUD` key invalid — **not added**. |

**Notes.**
- The 8 providers cerebras / hyperbolic / novita / sambanova / nvidia / zai / venice / chutes are the ones whose live registration drove the 5 → 18 count.
- fireworks and kimi are *wired* but return 0 live models because their keys are unusable; they skip gracefully rather than emitting placeholder entries (anti-bluff: no fabricated models).
- upstage and codestral were excluded from the catalogue because they expose no `/models` listing endpoint; this is a deliberate exclusion, not a silent failure.

---

## 4. Positive-Codebase-Answer Proof

**What was built.** Three coordinated changes in `applications/terminal_ui/main.go`:
1. **System prompt** (`buildToolLoopSystemPrompt`) instructs the agent it operates INSIDE the repository and must **make a tool call before any codebase claim**, then confirm concretely.
2. **`resolveRepoRoot`** pins the `git_status` tool to the actual git root, so the agent reads real repository state.
3. **Picker is ensemble-first**, so a capable model handles the codebase question.

**Live proof (captured frame).** Using cerebras `gpt-oss-120b`, the agent answered:

> "I can see your repository... So yes—I have direct read access to the codebase and its git state"

and rendered the **real `git_status` file list** in the same response. The affirmative claim was backed by an actual tool call returning real repository state — not a hardcoded or simulated answer.

**Honest scope of this proof.** This was verified for the cerebras `gpt-oss-120b` path in a single captured frame. It demonstrates the mechanism works end-to-end; it is not a claim that every one of the 18 providers was individually exercised through the same flow.

---

## 5. Real-Verifier-Consumption Proof (CONST-036)

**Claim under test.** HelixCode consumes LLMsVerifier as the single source of truth, not embedded fallback data.

**Setup.** LLMsVerifier built, served on `:8095`, DB populated with **78 real models** (groq 16, deepseek 4, mistral 58). HelixCode run in **remote mode** against the verifier.

**Result.** HelixCode received **all 78 models** from the verifier — **not** the **7 embedded fallback** entries it would use when the verifier is unreachable. Receiving 78 (the live DB count) rather than 7 (the static fallback count) is the discriminating evidence that real verifier consumption occurred.

**Reconciliation work that made this real.** The HelixCode verifier client was aligned to the verifier's actual API contract: response envelope shape, the `/api/providers` endpoint, Unix-timestamp parsing, and numeric `id` + `model_id` fields. Before reconciliation the client did not correctly parse the verifier's real responses.

---

## 6. Commit Reference

All of the above landed at commit **`a99a5e9d`**:

> `feat(tui,llm,verifier): use all ~/api_keys.sh providers + guarantee positive codebase answer + real LLMsVerifier consumption`

Touched (per `git show --stat`): `internal/llm/openai_compatible_catalogue.go` (+254), `internal/llm/openai_compatible_catalogue_test.go` (+224), `applications/terminal_ui/env_providers.go` (+20), `applications/terminal_ui/env_providers_test.go` (+11), `applications/terminal_ui/main.go` (+48).

---

## 7. Honest Limitations (anti-bluff)

These are real, in-session findings. None is hidden behind a green summary line.

1. **Invalid / suspended free-tier keys.** `fireworks` (account suspended), `kimi` (invalid key), `upstage`, and `tencent` keys are not usable. fireworks and kimi are wired but contribute 0 live models and skip gracefully; tencent (`TENCENT_CLOUD`) was invalid and **not added**.
2. **Rate limit on heavy prompt.** The **cerebras free tier hit a 429 RPM limit** on the heavy multi-turn AGENTS.md prompt. The provider is registered and works for normal prompts; the limit is a free-tier throughput ceiling, not a wiring defect.
3. **HelixAgent INFERENCE provider has no chat-entitled model yet.** The INFERENCE provider was added to HelixAgent, but its account currently exposes no chat-entitled model, so it cannot yet serve chat completions.
4. **upstage / codestral excluded from the HelixCode catalogue.** Reason: neither exposes a `/models` enumeration endpoint. codestral itself IS live-verified inside HelixAgent (one of the 6), so this is a HelixCode-catalogue listing limitation, not a provider-dead finding.
5. **Coverage scope of the positive-answer proof.** Proven for the cerebras `gpt-oss-120b` path (§4). Not every one of the 18 wired providers was individually driven through the codebase-answer flow this session.

---

## 8. Reproduction Pointers

- Catalogue definition: `helix_code/internal/llm/openai_compatible_catalogue.go`
- TUI registration loop: `helix_code/applications/terminal_ui/env_providers.go`
- Positive-answer prompt + repo-root pin: `helix_code/applications/terminal_ui/main.go` (`buildToolLoopSystemPrompt`, `resolveRepoRoot`)
- Catalogue unit tests: `helix_code/internal/llm/openai_compatible_catalogue_test.go`
- Verifier endpoint observed: `http://localhost:8095/api/providers`
