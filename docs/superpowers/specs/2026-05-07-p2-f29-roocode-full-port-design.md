# P2-F29 — Roo-code Full Port

**Date:** 2026-05-07 | **Status:** Approved
**Scope:** Full Roo-code port — task delegation (F15), code generation (template+LLM), code review, memory, conversations.

## Architecture
`internal/roocode/` (NEW) — 4 sub-components:
- **TaskDelegator**: F15 subagent dispatch with roo-code task templates
- **CodeGenerator**: Template-based scaffolding with LLM content
- **CodeReviewer**: Diff analysis + improvement suggestions via LLM
- **ConversationStore**: Memory-backed conversation tracking (reuse F24/F11)

## Tools: roo_delegate, roo_generate, roo_bootstrap. Slash: /roocode.

## Tasks (8): T01 bootstrap → T08 challenge + close-out
Zero new deps — F15, F24, F11 all shipped.
