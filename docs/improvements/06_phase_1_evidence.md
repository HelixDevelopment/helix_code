# Phase 1 Evidence Log — claude-code feature porting

Each feature's acceptance check output is pasted below with a timestamp.
This file is the rolled-up forensic record per Article XI §11.9.

Spec: `docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md` §4.1
Plan (Feature 1): `docs/superpowers/plans/2026-05-05-p1-f01-auto-compaction.md`

## F01 — Auto-Compaction System (claude-code)

**Timestamp:** 2026-05-05T01:10:35+03:00
**Status:** CLOSED

### Approach

Extended the existing `internal/llm/compression/` infrastructure (CompressionCoordinator + 3 strategies: SlidingWindow / SemanticSummarization / Hybrid) rather than building the parallel `internal/context/compaction.go` system the porting doc proposed. The porting doc was written without awareness of the existing compression layer.

### Commits in order

| Task | Commit | Subject |
|---|---|---|
| P1-F01-T01 | `f0b9b15` | bootstrap Phase 1 evidence + PROGRESS init |
| P1-F01-T02 | `5b153e6` | extend Provider interface (GetContextWindow + CountTokens) |
| P1-F01-T03 | `827971f` | implement Provider methods on 17 providers + 4 mocks |
| P1-F01-T04 | `59f7daa` | ThrashingGuard with TDD (4/4 PASS) |
| P1-F01-T05 | `b9eae7f` | CompactionMetadata with TDD (3/3 PASS) |
| P1-F01-T06 | `4330341` | AutoCompactor 80%-trigger orchestrator (3/3 PASS) |
| P1-F01-T07 | `cace643` | wire AutoCompactor into BaseAgent.executeTaskWithLLM |
| P1-F01-T08 | `b913ce2` | wire ThrashingGuard.NoteUserMessage into session.Manager |
| P1-F01-T09 | `4734f35` | integration test (real Anthropic, SKIP-OK without creds) |
| P1-F01-T10 | `9284392` | Challenge with expected.json + runtime-evidence driver |

### Acceptance

- **Unit tests:** 10/10 PASS (4 ThrashingGuard + 3 CompactionMetadata + 3 AutoCompactor) on top of the existing 33 compression-package tests, all green.
- **Integration test:** SKIP-OK path verified live (without `HELIX_LLM_ANTHROPIC_KEY`); real-credential run is the operator's call.
- **Challenge:** SKIP-OK path verified live; runtime-evidence driver compiles cleanly inside the module (cmd/-mounted to access internal/ packages).
- **Build:** `go build ./...` exits 0 across the inner module after every task.
- **Secret hygiene:** `scripts/scan-secrets.sh` exits 0.
- **`make verify-foundation`:** exit code unchanged from Phase 0 close-out (still =2 due to documented LLMsVerifier carry-forward; F01 work did not affect it).

### Adjustments made vs. the planned design

| Plan said | Reality |
|---|---|
| `internal/context/compaction.go` + parallel system | Extended `internal/llm/compression/` instead — existing CompressionCoordinator already provides the heavy lifting |
| `TokenCounter` interface name | Renamed `ProviderTokenCounter` to avoid collision with existing concrete type `compression.TokenCounter` |
| `compressioniface.CompressionResult.TokensAfter` | Field doesn't exist — used `cr.TokensSaved` to derive `TokensAfter = TokensBefore - TokensSaved` |
| `compressioniface.Message.Metadata` as `map[string]interface{}` | Existing field was a typed `MessageMetadata`; added `Extra map[string]interface{}` to it as the metadata-storage slot |
| `llm.AnthropicConfig` in tests/Challenge | Real constructor takes `llm.ProviderConfigEntry{Type, APIKey, Enabled, Models}` — adjusted |
| `compressioniface.MessageRole("user")` | Replaced with typed constants `RoleUser` / `RoleAssistant` where they exist |
| Challenge driver in `/tmp` | Moved to `mktemp -d -p cmd` so internal/ imports resolve under Go visibility rules |
| Session manager's `AddUserMessage` hook | No such method exists; added a thin `NoteUserMessage(sessionID)` wired the same way |

### Open carry-forward (F01 → Phase 3)

- **Per-provider native tokenizers.** Currently every provider falls back to `CharBasedTokenCount` (1 token ≈ 3.5 chars). Phase 3 sub-spec adds: tiktoken for OpenAI/Azure/Bedrock, anthropic-tokenizer for Anthropic, native counters for Gemini/Qwen/etc. The 80%-threshold trigger is correct under the fallback (conservative under-estimation), so this is a precision improvement, not a correctness fix.
