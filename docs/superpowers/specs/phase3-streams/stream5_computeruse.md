# Stream HXC031-COMPUTERUSE — Capability B design (read + design only; NO Edit — output text)

Repo: /Users/milosvasic/Projects/HelixCode, app `helix_code/`. HXC-031 Capability B = COMPUTER-USE LOOP, BROWSER-ONLY (FORK-1: browser_action screenshot-feedback loop, NOT OS-level). Depends on Capability A (multimodal message carriage, designed by another stream — assume A gives a Message that can carry an image block).

Known (verify): a chromedp-based browser suite already exists (referred to as "P2-F23"). REUSE+EXTEND, do not build fresh.

Tasks:
1. Find the chromedp browser suite in `helix_code/internal/` (chromedp usage, screenshot capture, click/type/navigate primitives). List files/types/exported funcs, cite file:line, quote the screenshot-capture fn + the action primitives.
2. Determine how browser actions are exposed to the agent/LLM (tool registry?). Quote the registration.
3. DESIGN Capability B: the iterative loop — capture screenshot → send to model as a multimodal message (via Capability A) → receive next browser_action → execute via the chromedp suite → repeat until goal/stop. Specify: loop control (max iterations, stop condition, error handling), the exact integration seam calling Capability A, and what is reused vs new (REUSE+EXTEND). Present a concrete skeleton sketch (new file(s) + functions + the call into A).
4. TEST PLAN per CONST-050: a real-browser headless (chromedp) integration test driving ≥1 screenshot→action cycle; list test files + assertions + the captured evidence that proves real page-state change (not just "no error").

Output: compact markdown (existing-code map with file:line, design + skeleton, dependency-on-A note, test plan). Do not implement.
