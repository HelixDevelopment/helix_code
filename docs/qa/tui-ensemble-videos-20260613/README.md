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
- File: `/tmp/helix_recordings/video2-helix-agent-ensemble.mp4`.
- **Status: PENDING verifier population** (`video2-ensemble-validation-pending-verifier.json`).
- The ensemble is fully implemented + code-reviewed GO + unit/live-tested: it fans each prompt to all env-key cloud providers (DeepSeek/Mistral/Groq/OpenRouter) and votes (confidence/quality). In ISOLATION it returns real voted answers with 3/4 members succeeding (captured: "2 plus 2 equals 4.", winner Groq). Zero hardcoded model names; member-model resolution is verifier-driven (CONST-036/040) with a capability-filtered catalogue fallback; a warm-cache pre-resolves working models.
- HONEST live finding (§11.4.6/§11.4.123): the LIVE 3-rapid-prompt TUI recording all-fails because, with the verifier UNPOPULATED in the TUI, the ensemble must trial-discover each member's working model, and that burst of calls trips free-tier provider rate-limits (Groq/OpenRouter-402). The operator-chosen fix is the CONST-036 path — populate LLMsVerifier so each member resolves its verified working model first-try (1 call/member, no burst). V2 will be re-recorded clean once verifier-driven.

## Tooling (reused + extended per operator ask)
- **Panoptic** `recvalidate` (`submodules/panoptic`): OCR video-validator; de-hardcoded so chrome patterns + reply markers are consumer-supplied (`--chrome-pattern`/`--reply-marker`), keeping Panoptic project-agnostic (CONST-051(B)); golden-good/golden-bad self-validation (§11.4.107(10)).
- **HelixQA** `pkg/recordingqa` + `banks/tui-recording-validation.yaml` (`submodules/helix_qa`): recording-validation bank threading the new options through + a `TRV-ENSEMBLE-001` entry; paired-mutation tests.

## Sources / commands
The exact `panoptic recvalidate` invocations (with chrome patterns + reply marker) and per-check JSON are in the two `*-validation.json` files in this directory.
