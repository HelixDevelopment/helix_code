# Issues_Summary

Open workable items (current_location = Issues), regenerated from the SQLite single-source-of-truth (§11.4.12).

## Counts by Type × Status

| Type | Status | Count |
|---|---|---|
| Bug | Queued | 3 |
| Feature | Queued | 1 |
| Task | Queued | 2 |
| **TOTAL** | | **6** |

## Items

| ATM ID | Type | Status | Severity | Description |
|---|---|---|---|---|
| HXC-119 | Feature | Queued | High | Governance rule CONST-040 lists the Agent Client Protocol among required capabilities, but there is no implementation of it anywhere in the codebase. Any user or integration expecting ACP connectivity currently cannot use it. The work is to design and implement real ACP support, or, if it proves structurally infeasible, to document that with cited evidence. The platform will then either genuinely support ACP or hold an honest, evidenced position instead of an unmet claim. |
| HXC-145 | Bug | Queued | Low | During the real-infra retest the Xiaomi provider chaos tests failed 2 of 5 because the model id configured for Xiaomi (mimo-v2-flash) is rejected by the live Xiaomi API, indicating the configured model name is stale or wrong. Users selecting the Xiaomi provider with that model would get errors. The work is to determine the correct current Xiaomi model id (from the provider or the verifier as single source of truth) and update the configuration so Xiaomi requests succeed. Evidence: docs/qa/infra_retest_20260712_hxc122_138/EVIDENCE.md (Xiaomi 3/5). |
| HXC-146 | Task | Queued | Medium | The e2e challenge runner advertises multiple interface modes (cli, rest, tui, websocket) but none of them actually exercises the HelixCode server's real HTTP API during a run, so the challenges validate the runner's own logic rather than the shipped server endpoints. This is a documentation-versus-implementation gap that weakens the end-to-end proof. The work is to wire the challenge runner's interfaces to genuinely call the running server's HTTP API so the challenges prove the real user-facing endpoints work. Discovered 2026-07-12 real-infra retest. Evidence: docs/qa/infra_retest_20260712_hxc122_138/hxc138_challenge_report.json. |
| HXC-147 | Bug | Queued | Medium | Running the (now-compilable) automation test binary against the live OpenRouter API, TestAllFreeProvidersAutomation Provider_OpenRouter BasicGeneration panics with a nil-pointer dereference: the configured free model id deepseek-r1-free is stale/rejected and the code path is missing a nil-check on the error before using the response. Users of the OpenRouter free provider with that model would hit the same crash. The work is to correct the free-provider model id (sourced from the verifier as single source of truth) and add the missing nil-check so a rejected model degrades gracefully instead of panicking. NOTE this environment has live provider API keys set so provider tests spend real money; guard/skip accordingly. Found 2026-07-12. |
| HXC-148 | Task | Queued | Low | HXC-118 wired Retrieval-Augmented-Generation into the native server generate and stream endpoints and the CLI, but the OpenAI-compatible and Anthropic-compatible wire-facade endpoints (/v1/chat/completions and /v1/messages) still bypass RAG entirely, so clients using those compatibility surfaces do not get retrieval-augmentation even when it is enabled. The work is to apply the same applyRAGContext wiring to those facade handlers so RAG behaves consistently across every generate surface. This is a smaller secondary surface than the native path already fixed. Found during HXC-118 review 2026-07-12. |
| HXC-149 | Bug | Queued | Medium | The main repository git index carries approximately 70 stale submodule gitlinks at pre-rename paths (dependencies/HelixDevelopment/*, dependencies/vasic-digital/*, and top-level helix_agent/helix_qa/panoptic/security) with no .gitmodules mapping, left over from a historical path-rename to the submodules/ layout. As a result git submodule status and git submodule foreach abort mid-walk with no submodule mapping found in .gitmodules for path ..., so any release or maintenance script that walks all submodules unfiltered fails partway. The work is to remove ALL stale cached gitlinks (git rm --cached on each) so the submodule set is consistent with .gitmodules and submodule-walking tooling completes. This is a git-index-only change, reversible via git reset. Found by the 2026-07-12 release-readiness survey. Low runtime risk but blocks release automation. |
