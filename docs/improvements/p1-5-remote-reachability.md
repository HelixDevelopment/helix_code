# Phase 1.5 — Submodule Remote Reachability

**Captured:** 2026-05-06
**Source:** `git ls-remote $url HEAD` for each unique submodule URL across both
the meta-repo and HelixAgent `.gitmodules`.
**Plan task:** P1.5-WP1-T01.03

---

## Summary

| Metric | Count |
|---|---|
| Unique URLs tested | 181 |
| OK | 178 (after retry) |
| UNREACHABLE (truly) | 3 |
| Initial false-positives | 2 (15s timeout flake — see §False positives) |

## UNREACHABLE — blocks WP2 for these submodules

| URL | Submodule path(s) | Status | Action required |
|---|---|---|---|
| `git@github.com:vasic-digital/DebateOrchestrator.git` | `helix_agent/DebateOrchestrator` | UNREACHABLE | Repo does not exist under vasic-digital. Either create the repo, point gitlink at correct upstream, or remove the entry from helix_agent/.gitmodules. |
| `git@github.com:stark1tty/kiro-cli.git` | `helix_agent/cli_agents/kiro-cli` (will be moved to root in WP2) | UNREACHABLE | Upstream third-party fork was deleted. Already in Phase 0 §3.3 "13 deferred to Phase 2 sub-specs" parking lot. WP2 should drop the entry. |
| `git@github.com:tcsenpai/ollama-code.git` | `helix_agent/cli_agents/ollama-code` (will be moved to root in WP2) | UNREACHABLE | Upstream third-party repo was deleted. Same parking-lot disposition. |

## Reachability impact on WP1 close-out

`git submodule update --init --recursive` aborted on DebateOrchestrator (the
double-failure-aborts behaviour); the recursive pull-loop ran against the
already-initialised tree and was killed at helix_agent/cli_agents/HelixCode
because that submodule has a placeholder `refs/heads/.invalid` HEAD which is
not pull-able (see plan T02.01 — circular reference, scheduled for explicit
removal).

WP1 close-out is therefore PARTIAL pending user decision on the 3 unreachable
remotes. Per user's STOP-protocol mandate, the agent halted at this point.

## False positives (15s timeout flake)

The first ls-remote pass with a 15-second timeout misclassified two URLs as
UNREACHABLE. Re-test with 30s confirmed both are reachable:

| URL | First-pass | Retry | Likely cause |
|---|---|---|---|
| `git@github.com:glittercowboy/get-shit-done.git` | UNREACHABLE | OK | small private/cold repo, slow first SSH negotiate |
| `git@github.com:google-gemini/gemini-cli.git` | UNREACHABLE | OK | large repo, slow `ls-remote` over SSH first time |

Lesson: the `git ls-remote $url HEAD` timeout MUST be ≥30s for the next round
of P1.5 tooling. Recorded for WP9 / WP10 reference-update sweeps.

## Full per-URL log

See `docs/improvements/p1-5-remotes.log` for the verbatim pass output (181
lines, fixed-width formatted as `<url> <status>`).
