# Stream HXC031-MULTIMODAL — Capability A design (read + design only; NO Edit — output text/diff sketch)

Repo: /Users/milosvasic/Projects/HelixCode, app `helix_code/`. HXC-031 Capability A = MULTIMODAL REQUEST CARRIAGE.
Adopted forks: FORK-2 = Anthropic/Claude provider FIRST only (not all providers); FORK-3 = reuse vasic-digital/vision_engine as encode-helpers only (no deep integration). Consumer is a browser screenshot loop.

Known (verify): the LLM `Message` type in `helix_code/internal/llm` is TEXT-ONLY today; the screenshot tool returns a FILE PATH, not bytes. The one genuinely-new piece is carrying image content in a request.

Tasks:
1. Read `helix_code/internal/llm/`: quote the current `Message` struct, the request/response types, and the Anthropic/Claude provider's Generate / GenerateStream request-building code. Cite file:line.
2. Locate the screenshot tool (likely `helix_code/internal/tools/` or similar); confirm it returns a path; quote the signature/return.
3. DESIGN Capability A: per-message image content blocks carried as base64 data-URLs, mapped to Anthropic's multimodal content-block request format (text|image blocks per message), Claude-first, backward-compatible with existing text-only callers. Present a concrete diff SKETCH (file + before/after) covering: the `Message` type change, screenshot bytes exposure (or caller path→base64), and the Anthropic request-mapping change.
4. TEST PLAN per CONST-050 (no fakes beyond unit tests): unit tests for type/encoding; ONE integration test making a REAL Anthropic call with an image, gated on the API key in `.env` (SKIP-OK marker if absent). List test files + assertions.

Output: compact markdown (current-state quotes with file:line, diff sketch, test plan). Do not implement.
