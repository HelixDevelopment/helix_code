# Issues_Summary

Open workable items (current_location = Issues), regenerated from the SQLite single-source-of-truth (§11.4.12).

## Counts by Type × Status

| Type | Status | Count |
|---|---|---|
| Feature | Queued | 1 |
| Task | Queued | 1 |
| **TOTAL** | | **2** |

## Items

| ATM ID | Type | Status | Severity | Description |
|---|---|---|---|---|
| HXC-119 | Feature | Queued | High | Governance rule CONST-040 lists the Agent Client Protocol among required capabilities, but there is no implementation of it anywhere in the codebase. Any user or integration expecting ACP connectivity currently cannot use it. The work is to design and implement real ACP support, or, if it proves structurally infeasible, to document that with cited evidence. The platform will then either genuinely support ACP or hold an honest, evidenced position instead of an unmet claim. |
| HXC-146 | Task | Queued | Medium | The e2e challenge runner advertises multiple interface modes (cli, rest, tui, websocket) but none of them actually exercises the HelixCode server's real HTTP API during a run, so the challenges validate the runner's own logic rather than the shipped server endpoints. This is a documentation-versus-implementation gap that weakens the end-to-end proof. The work is to wire the challenge runner's interfaces to genuinely call the running server's HTTP API so the challenges prove the real user-facing endpoints work. Discovered 2026-07-12 real-infra retest. Evidence: docs/qa/infra_retest_20260712_hxc122_138/hxc138_challenge_report.json. |
