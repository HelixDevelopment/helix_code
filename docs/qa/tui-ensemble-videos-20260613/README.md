# QA Evidence — TUI model-interaction video recordings

**Run-id:** tui-ensemble-videos-20260613
**Date:** 2026-06-13
**Feature:** `helix-tui` end-to-end model interaction — agent-driven recording + OCR validation
**Operator request:** record two TUI videos (Video 1 = strongest LLMsVerifier model; Video 2 = "Helix Agent ensemble"), run 3 prompts each, validate both for real replies + no errors/warnings; reuse HelixQA + Panoptic for recording/validation; everything dynamic, no hardcoding, no bluff.
**Anti-bluff basis:** §11.4.5 / §11.4.107 (liveness/reply oracle) / §11.4.69 / §11.4.83 / §11.4.123. The agent (not the operator) drove the TUI and produced the recordings; replies and error-state are captured from real OCR of the recorded frames.

## Prompts (both videos)
1. "Do you see my codebase?"
2. "Do you need an AGENTS.md, and what is name of your agents file exactly?"
3. "Check git status of all work done."

## Video 1 — strongest single model (Groq `llama-3.3-70b-versatile`, digit 7)
- File: `~/Downloads/video1-strongest-model.mp4` (also `/tmp/helix_recordings/`).
- **Validation: 5/5 PASS** (`video1-strongest-model-validation.json`, 68 frames):
  - `no_error_tokens` PASS — clean, no errors/warnings on screen.
  - `prompt_{1,2,3}_has_reply` PASS — real `llama-3.3-70b` prose replies rendered (e.g. honest "I don't have a specific agents file… common names include AGENTS.md", "git status…"), "Response received (tokens: 457)".
  - `intended_model_selected` PASS — model "Groq" visible.
- The model honestly states it has no filesystem/git access (raw LLM via API, no tools) — an honest real reply, not a bluff.

## Video 2 — "Helix Agent ensemble" (digit 9)
- File: `~/Downloads/video2-helix-agent-ensemble.mp4` (also `/tmp/helix_recordings/`).
- **Validation: 5/5 PASS** (`video2-ensemble-validation.json`, 95 frames): `no_error_tokens` PASS, `prompt_{1,2,3}_has_reply` PASS (real ensemble prose replies, e.g. "I don't have the ability to see or access your codebase…"), `intended_model_selected` PASS (ensemble visible). Defect-token grep on the aggregated OCR (`member(s) failed` / `[Error:` / `first error`) → ABSENT.
- The ensemble fans each prompt to all env-key cloud providers (DeepSeek/Mistral/Groq/OpenRouter) and votes (confidence/quality). Zero hardcoded model names; member-model resolution preserves each provider's live working-model-first catalogue order, is verifier-driven when real verified data exists (CONST-036/040), and persists a dead-model set so a decommissioned model (e.g. Groq `gemma-7b-it`) is recorded + never re-tried.
- ROOT-CAUSE of the earlier all-fail (§11.4.102, captured RED→GREEN): NOT rate-limits — the ensemble forwarded `request.Stream=true` into each member's NON-streaming `Generate`, so providers returned SSE that was JSON-decoded → `invalid character 'd' looking for beginning of value` → every member failed. Fix: `Stream=false` forced per member (votes on complete responses; `GenerateStream` emits the voted result as one chunk). A second contributor — an alphabetical sort destroying each provider's working-model-first catalogue order — was also removed. Paired §1.1 regression guards: removing `Stream=false` reproduces the exact live `invalid character 'd' … all member(s) failed` and FAILs the guard.

## Tooling (reused + extended per operator ask)
- **Panoptic** `recvalidate` (`submodules/panoptic`): OCR video-validator; de-hardcoded so chrome patterns + reply markers are consumer-supplied (`--chrome-pattern`/`--reply-marker`), keeping Panoptic project-agnostic (CONST-051(B)); golden-good/golden-bad self-validation (§11.4.107(10)).
- **HelixQA** `pkg/recordingqa` + `banks/tui-recording-validation.yaml` (`submodules/helix_qa`): recording-validation bank threading the new options through + a `TRV-ENSEMBLE-001` entry; paired-mutation tests.

## Sources / commands
The exact `panoptic recvalidate` invocations (with chrome patterns + reply marker) and per-check JSON are in the two `*-validation.json` files in this directory.
