# HXC-029 — CLI-class HelixQA banks self-driving conversion (§11.4.98)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-05-29 |
| Last modified | 2026-05-29 |
| Status | active |
| Tracked item | HXC-029 (§11.4.98 full-automation anti-bluff forward sweep) |
| Batch | CLI-class banks: `cli-agents-comprehensive`, `aichat-bash-tools-comprehensive`, `cli-agents-test-helixagent` |
| Predecessor | `docs/qa/HXC-029/bank-classification.md` (classified these AUTO-EXECUTABLE-WITH-WORK) |

## What changed

The three CLI-class banks under `helix_qa/banks/` carried `manual-review-required`
prose steps (Go-API internal calls, external-CLI invocations, service calls).
They are now SELF-DRIVING per §11.4.98: every step is either

- a `shell:` action run via real `os/exec` (the genuine `./bin/cli` HelixCode
  binary + coreutils temp-file roundtrips), asserting real exit code + output, OR
- a `http:` action against the live server (none survived — see below), OR
- an **honest `_skip`** with a precise reason, NEVER a fabricated PASS
  (§11.4 / §11.4.98 / §11.9).

`grep -c manual-review-required` on all three banks = 0 after conversion.

### Step distribution (post-conversion)

| Bank | shell (real PASS) | http | honest skip | total |
|---|---|---|---|---|
| cli-agents-comprehensive | 6 | 0 | 81 | 87 |
| aichat-bash-tools-comprehensive | 9 | 0 | 71 | 80 |
| cli-agents-test-helixagent | 7 | 0 | 71 | 78 |

Each bank gains an injected `HXC029-CLI-PROBE` test case driving the REAL
`./bin/cli` binary (`-health`, `-list-models`, `-list-workers`, `-command`
real-exec, `--help` token grep) + a self-cleaning coreutils roundtrip — so
every bank carries positive runtime evidence, not only skips.

## Honest-skip reasons (real probes captured 2026-05-29 — see `endpoint-probes-cli.txt`)

- **Go-API internal symbols** (`registry.GetStats()`, `agent.Info()`, `.New()`,
  `Initialize(ctx,…)`): no public CLI/shell surface; covered by `helix_code` Go
  unit tests, not a CLI step. These were never genuinely manual.
- **`aider`**: installed (`/home/milosvasic/.local/bin/aider`) but steps drive it
  with `--model helixagent/ensemble` + interactive slash-commands requiring the
  helixagent endpoint on `:7061` (connection refused) and an interactive REPL
  (non-self-driving per §11.4.98(C)).
- **`aichat` / `openhands`**: not on PATH.
- **`./bin/helixagent` binary**: absent (only `./bin/cli` + `./bin/helixcode` built).
- **helixagent service `:7061`**: not running; the live server is `helixcode` on `:8080`.
- **`/v1/mcp/*` HTTP**: helixagent MCP surface — 404 on `:8080`. An assertion-free
  pass on a 404 would be a §11.4.98 bluff, so these are skipped, not passed.
- **`fs_*.sh` bash providers**: `internal/tools/bash_providers/` absent in this checkout.

## Verification (this session, against live `:8080` + real `./bin/cli`)

**3 deterministic runs each** (identical results, exit 0):
- `run_1.txt` / `run_2.txt` / `run_3.txt` per bank dir.
- cli-agents-comprehensive: `6 PASS, 0 FAIL, 81 SKIP` ×3
- aichat-bash-tools-comprehensive: `9 PASS, 0 FAIL, 71 SKIP` ×3
- cli-agents-test-helixagent: `7 PASS, 0 FAIL, 71 SKIP` ×3

**Mutation proof** (`mutation-proof.txt` per bank): flipping a real shell step's
`expect_output_contains` to a value that cannot appear forces the runner to FAIL
(exit 1) — proving the assertions are real, not bluff-proof:
- each bank → `… 1 FAIL …`, exit 1.

## Runner

`docs/qa/HXC-029/cli-runner/main.go` — standalone Go-stdlib runner implementing
the documented HelixQA `shell:`/`http:` + `_skip` executor contract
(`helix_qa/pkg/testbank/schema.go`). Standalone for the same reason as the HTTP
runner: helix_qa's `replace` directives point at uncheckedout owned submodules.
`convert.py` is the deterministic prose→self-driving converter.

Reproduce:

```bash
go build -o /tmp/r docs/qa/HXC-029/cli-runner/main.go
HELIXQA_HTTP_BASE_URL=http://localhost:8080 \
HXC_CLI_WORKDIR=$PWD/helix_code \
  /tmp/r -bank helix_qa/banks/cli-agents-comprehensive.json
```

## Sources verified

Internal source-of-truth read this session (no external web sources — this is a
test-conversion task, not an operator-facing service guide):
- `helix_qa/pkg/testbank/schema.go` (`ActionTypeShell`/`ActionTypeHTTP`, `_skip`)
- `docs/qa/HXC-029/full-qa-api/runner/main.go` (HTTP-runner sibling pattern)
- `docs/qa/HXC-029/bank-classification.md` (predecessor classification)
- live `./bin/cli` probes + `:8080`/`:7061` endpoint probes (`endpoint-probes-cli.txt`)
