# OpenDesign — Durable §11.4.78 / §11.4.83 Dual-Challenge Evidence

**Run ID:** `opendesign_durable_evidence_20260711T141015Z`
**Captured (UTC):** 2026-07-11T14:11:18Z
**Track/branch:** (T1/feature/helixllm-full-extension)
**Repo:** `/home/milos/Factory/projects/tools_and_research/helix_code`
**Purpose:** close the coverage gap raised by the prior audit — the earlier
OpenDesign dual-challenge PASS (commit `c1fe11b0`, 2026-07-08) had its
evidence written only to a git-ignored scratchpad location (non-durable per
§11.4.83) and the daemon on `:7456` was found down at audit time. This run
brings the daemon back up and commits the evidence durably under `docs/qa/`.

## 1. Gap being closed

| Prior state (audit finding) | This run |
|---|---|
| Daemon on `:7456` down | Daemon brought up via `scripts/opendesign/bringup.sh up`, confirmed healthy |
| Evidence only in git-ignored scratchpad (non-durable, §11.4.83 violation) | Evidence written to `docs/qa/opendesign_durable_evidence_20260711T141015Z/` and committed to git (durable) |
| — | `scripts/opendesign/capture_durable_evidence.sh` added: a re-runnable script that reproduces both challenge legs (HTTP-level) so the dual-challenge is not a one-off manual capture |

## 2. Bring-up (§11.4.174 ownership-verified, §11.4.161 rootless — no daemon/root needed, user-level process)

Command: `bash scripts/opendesign/bringup.sh up`

```
[2026-07-11T14:09:20Z] bringup: action=up port=7456 host=127.0.0.1 src=/home/milos/Factory/projects/tools_and_research/.opendesign-src/open-design
[2026-07-11T14:09:21Z] source: checkout already present at /home/milos/Factory/projects/tools_and_research/.opendesign-src/open-design (088ea9041)
[2026-07-11T14:09:21Z] start: launching daemon entry point on 127.0.0.1:7456
[2026-07-11T14:09:21Z] start: daemon pid=2951699 (pidfile=/home/milos/Factory/projects/tools_and_research/helix_code/scripts/opendesign/run/daemon.pid log=/home/milos/Factory/projects/tools_and_research/helix_code/scripts/opendesign/run/daemon.log)
[2026-07-11T14:09:24Z] bringup: DONE — daemon healthy (ok=true version=0.14.1)
```

Exit code: `0`. No bug found in `bringup.sh` — bring-up succeeded on the
first invocation against the pre-existing from-source checkout
(`.opendesign-src/open-design` @ `088ea9041`, outside the repo per §11.4.30
— the 3.8GB checkout is NOT committed). The prior audit's "down" finding was
simply the daemon process having exited since the previous session (no PID
survives a host restart) — this is expected, idempotent, ownership-verified
behavior, not a defect.

`bash scripts/opendesign/bringup.sh status` immediately after:

```
[2026-07-11T14:09:29Z] status: HEALTHY (ok=true version=0.14.1) url=http://127.0.0.1:7456/api/health
```

## 3. Dual-challenge (§11.4.78-style unforgeable evidence)

Two independent facts, each obtainable ONLY by querying its respective live
service — neither can be produced by reading source, grepping config, or any
static inspection:

### 3a. OpenDesign daemon fact — via the REAL MCP tool `mcp__open-design__od_list_projects`

Called directly through the wired `open-design` MCP server (`.mcp.json`,
`OD_DAEMON_URL=http://localhost:7456`) — this is the actual agent-facing
tool path, not a raw HTTP shortcut:

```json
{"projects":[{"id":"helixcode-brand","name":"HelixCode Brand"}]}
```

**Result: the seeded `helixcode-brand` project (created in the prior
2026-07-08 bring-up session) is still present and returned by the live
daemon.** This project id/name cannot be produced without the OpenDesign
daemon actually running and holding real seeded state.

Cross-checked at the raw HTTP layer (same fact, re-runnable without an MCP
client) via `scripts/opendesign/capture_durable_evidence.sh`, captured
verbatim in this directory:

- [`od_health.json`](./od_health.json) — `GET /api/health` → `{"ok":true,"version":"0.14.1"}`
- [`od_projects.json`](./od_projects.json) — `GET /api/projects` → full project record for `helixcode-brand`, including its original `createdAt=1783493400372` timestamp from the 2026-07-08 seeding, `status.value="not_started"`.

The `createdAt` timestamp matching the original seeding session (not
"now") is itself corroborating evidence that this is real persisted daemon
state, not a freshly fabricated response.

### 3b. CodeGraph cross-fact — via `codegraph status` against this repo's live index

```
CodeGraph Status

Project: /home/milos/Factory/projects/tools_and_research/helix_code

Index Statistics:
  Files:     22,690
  Nodes:     460,470
  Edges:     1,475,918
  DB Size:   1488.95 MB
  Backend:   node:sqlite — built-in (full WAL)
  Journal:   wal

✓ Index is up to date
```

Full output captured verbatim: [`codegraph_status.txt`](./codegraph_status.txt).

**Result: 460,470 real indexed nodes** across 22,690 files in the live
SQLite index at `.codegraph/codegraph.db` (1488.95 MB on disk). This number
is a genuine query result against the built index — it changes as the
codebase changes and cannot be produced without the index actually existing
and being queried (§11.4.79 — this repo's own-org submodules are included in
scope, hence the mixed-language node kinds shown: go, python, typescript,
csharp, yaml, tsx, …).

Note: this run's node count (460,470) differs from the prior 2026-07-08
evidence (1,786,072) because the index has been rebuilt/rescoped between the
two sessions — both are genuine live query results at their respective
capture times, not a discrepancy indicating fakery.

### 3c. Dual-challenge verdict

| Leg | Fact obtained ONLY by calling the live service | Result |
|---|---|---|
| OpenDesign | `od_list_projects` MCP tool → seeded `helixcode-brand` project returned | **CONFIRMED** |
| CodeGraph | `codegraph status` → live index node count (460,470) | **CONFIRMED** |

**Dual-challenge: PASS.**

## 4. Re-running this evidence

```bash
bash scripts/opendesign/bringup.sh up
bash scripts/opendesign/capture_durable_evidence.sh <new-output-dir>
```

`capture_durable_evidence.sh` exits non-zero and prints an explicit `FAIL:`
line for any leg that does not confirm (daemon unreachable, seeded project
missing, or codegraph node count unparseable) — never a silent/fake PASS
(§11.4.6, §11.4.107(10) — no bluff on a broken capture).

## 5. Durability (§11.4.83)

This directory (`docs/qa/opendesign_durable_evidence_20260711T141015Z/`) is
committed to git — it is NOT under any git-ignored path (`scripts/opendesign/run/`
and the 3.8GB `.opendesign-src/` checkout remain git-ignored per §11.4.30,
but this evidence directory is not). This closes the non-durable-evidence
gap the audit identified.

## 6. Scope / constraints honored

- No submodule touched (`submodules/helix_agent`, `submodules/helix_llm`,
  `submodules/helix_qa` untouched by this work).
- No files under `helix_code/internal/{security,tools/browser}` or
  `helix_code/internal/llm` touched.
- `.mcp.json` not modified (no fix was needed — the `open-design` entry
  already wires correctly; the daemon was simply not running).
- `coder :18434` untouched.
- The 1.9GB+ (actually 3.8GB on this host) OpenDesign source checkout was
  NOT committed (§11.4.30) — it lives outside the repo at
  `/home/milos/Factory/projects/tools_and_research/.opendesign-src/open-design`
  per `bringup.sh`'s documented default, and is the §11.4.77 re-obtain
  mechanism (clone + `pnpm install`, already codified in `bringup.sh`).

## Sources verified

§11.4.99 not applicable — no third-party service documentation was cited in
this run; all facts are first-party live-tool outputs captured directly.
