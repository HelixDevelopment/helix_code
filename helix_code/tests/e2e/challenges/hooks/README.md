# Challenge: P1-F05 — Hook-Based Extensibility

**Feature:** P1-F05 Hook-Based Extensibility
**Scenarios:** 3
**Runtime evidence required:** yes (CONST-035 / Article XI §11.9)

---

## Overview

This Challenge drives `hooks.Manager` end-to-end through a Go binary compiled
from inside the module tree (so `internal/` package imports are permitted).
No mocks are used.

---

## Scenarios

### S1 — block-bash-rm

A `before_bash` shell hook whose script exits 1 is registered.
`TriggerEventAndWait` is called with a `Bash` tool event carrying `rm -rf <marker>`.
The Challenge asserts:

- `Blockers(results)` returns exactly 1 entry (the hook's error).
- The marker file still exists — the Go driver (standing in for the agent) never
  executed the `rm` command because the hook blocked it.

### S2 — audit-after-tool

An `after_tool_call` shell hook whose script appends `fired\n` to a log file is
registered. The event is triggered 3 times via `TriggerEventAndWait`.
The Challenge asserts that the log file contains exactly 3 newlines (3 calls
→ 3 lines).

### S3 — yaml-validate-malformed

`FileLoader.Load` is called with a `UserPath` pointing to a file containing
syntactically invalid YAML (`not: valid: yaml: [`).
The Challenge asserts that `Load` returns a non-nil error, confirming the loader
enforces parse validation and does not silently swallow malformed config.

---

## Running

```bash
cd /path/to/HelixCode
tests/e2e/challenges/hooks/run.sh
```

Expected final line: `PASS: all three scenarios produced expected outcomes`

Exit code 0 on success, non-zero on any scenario failure.

---

## Mutation-test recipe

These mutations should each cause the Challenge to **FAIL**, confirming the
assertions are load-bearing and not vacuous:

| Mutation | Expected failure |
|---|---|
| Change `exit 1` to `exit 0` in the S1 block script | `FAIL S1: expected exactly 1 blocker` |
| Return `nil` from `Blockers` unconditionally | `FAIL S1: expected exactly 1 blocker` |
| Comment out `mgr.TriggerEventAndWait(event)` in the S2 loop | `FAIL S2: expected log to have exactly 3 lines` |
| Replace malformed YAML with valid YAML in S3 | `FAIL S3: malformed YAML did not produce a load error` |
| Make `FileLoader.Load` ignore parse errors | `FAIL S3: malformed YAML did not produce a load error` |

To apply a mutation manually:

```bash
# Example: patch Blockers to always return nil, run the Challenge, confirm FAIL:
# 1. Edit internal/hooks/blockers.go — return nil unconditionally
# 2. Run: tests/e2e/challenges/hooks/run.sh
# 3. Observe: FAIL S1: expected exactly 1 blocker
# 4. Revert the edit
```

---

## Anti-bluff compliance

Per Article XI §11.9 and CONST-035, this Challenge produces positive runtime
evidence for each scenario: actual process exits, file-system state, and log
contents are inspected — not merely the absence of errors.
