# HXC-148 — QA evidence (§11.4.83)

**Item:** HXC-148 (Task/Low) — OpenAI/Anthropic wire-facade endpoints bypass RAG
**Fix commit:** helix_code `6efadd15` (2 files; pushed github+gitlab)
**Date (UTC):** 2026-07-12T17:30:00Z
**Closure vocab:** Fixed (§11.4.33, Task)

## Root cause
HXC-118 wired RAG into the native generate/stream endpoints but the OpenAI/Anthropic
compat endpoints (/v1/chat/completions, /v1/messages) still bypassed RAG entirely.

## Fix
Added applyRAGContext calls to chatCompletions + anthropicMessages in wire_facade.go,
mirroring the native path. Disabled by default; graceful degrade on error.

## Verification
4 guard tests: disabled=byte-identical + enabled=augmented, for both endpoints. All PASS.
go build/vet clean. Anti-bluff smoke clean.
