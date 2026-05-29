# CodeGraph — Status

**Revision:** 1
**Last modified:** 2026-05-28T12:03:15Z

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-05-28 |
| Last modified | 2026-05-28T12:03:15Z |
| Status | active |
| Status summary | Append-only ledger of every CodeGraph-related event for HelixCode (config changes, index regenerations, sync runs, validation probes). Per §11.4.78 (CodeGraph parent), §11.4.79 (own-org submodule inclusion), §11.4.80 (regular-update + sync automation), §11.4.45 / §11.4.56 (Status-doc shape). The weekly update + sync automation is INHERITED BY REFERENCE from the constitution submodule — invoke `constitution/scripts/codegraph_update.sh` and `constitution/scripts/codegraph_sync.sh` (never copied). |
| Issues | none |
| Issues summary | — |
| Fixed | HXC-017 (own-org submodule inclusion in index) |
| Continuation | sibling `Status_Summary.md` carries the operator-readable digest per §11.4.56. |

## Table of contents

- [Cadence + automation (§11.4.80)](#cadence--automation-1148)
- [Index configuration (§11.4.79)](#index-configuration-11479)
- [Event ledger](#event-ledger)

## Cadence + automation (§11.4.80)

Per §11.4.80, HelixCode MUST run the CodeGraph update + sync automation at
least weekly (cadence floor per §11.4.45 status-digest cadence). The two
canonical scripts are **inherited by reference** from the constitution
submodule (per §3 submodule inheritance) and MUST be invoked at their
constitution-submodule paths — never copied into HelixCode:

- `constitution/scripts/codegraph_update.sh` — npm-installs the latest
  `@colbymchenry/codegraph`, verifies `codegraph --version` reflects the
  new version (anti-bluff: npm exit 0 is not proof of a working binary),
  and appends old/new version to this ledger.
- `constitution/scripts/codegraph_sync.sh` — after a successful update runs
  `codegraph status` → `codegraph sync .` → `codegraph status` →
  validation, appending every step's output to this ledger.

Regeneration mechanism (per §11.4.77): `.codegraph/codegraph.db` is
gitignored; `codegraph index .` (full) or `codegraph sync .` (incremental)
regenerates it from `.codegraph/config.json` (tracked).

## Index configuration (§11.4.79)

`.codegraph/config.json` (tracked) controls which paths enter the index.
Per §11.4.79 the `dependencies/` tree is split by submodule ownership:

**INCLUDED — own-org submodules (full CLI access via vasic-digital + HelixDevelopment):**

- `dependencies/vasic-digital/**` — ~55 own-org submodules (EventBus,
  Concurrency, Observability, Auth, Storage, VectorDB, Embeddings,
  Database, Cache, Messaging, Formatters, MCP_Module, RAG, Memory,
  Optimization, Plugins, Agentic, LLMOps, SelfImprove, Planning,
  Benchmark, ToolSchema, SkillRegistry, Models, LLMProvider,
  BackgroundTasks, DocProcessor, conversation, LLMOrchestrator,
  VisionEngine, Normalize, RedTeam, PliniusCommon, GandalfSolutions,
  AutoTemp, HyperTune, I-LLM, Streaming, Veritas, LeakHub, Claritas,
  Ouroborous, Config, Lazy, Watcher, Middleware, RateLimiter, I18n,
  Recovery, Document, Filesystem, TOON, …).
- `dependencies/HelixDevelopment/**` — own-org submodules (DocProcessor,
  LLMOrchestrator, LLMProvider, VisionEngine, LLMsVerifier, Models,
  HelixMemory, HelixSpecifier, HelixLLM, DebateOrchestrator, …).

**EXCLUDED — third-party vendored submodules (per §11.4.74 `no-match → vendor`):**

- `dependencies/LLama_CPP/**` — `git@github.com:ggml-org/llama.cpp.git`
- `dependencies/Ollama/**` — `git@github.com:ollama/ollama.git`
- `dependencies/HuggingFace_Hub/**` — `git@github.com:huggingface/huggingface_hub.git`

**Credential/secret exclusions (per §11.4.10, belt-and-suspenders):** the
`include` list is code-extensions only (no `.env` / `.key` / `.pem`), and
the exclude list additionally pins `**/.env`, `**/.env.*`, `**/*.key`,
`**/*.pem`, `**/secrets/**` so no credential path can ever be indexed.

## Event ledger

(events appended below by the automation; newest at the bottom)

## 2026-05-28T12:03:15Z — HXC-017 config fix: include own-org submodules, exclude only third-party (§11.4.79)

- **Defect**: `.codegraph/config.json` carried a blanket `dependencies/**`
  exclude that wrongly removed ALL own-org submodules
  (`dependencies/vasic-digital/**` + `dependencies/HelixDevelopment/**`)
  from the index — a §11.4.79 violation (own-org submodules MUST be
  INCLUDED; only third-party submodules excluded).
- **Fix**: replaced the blanket `dependencies/**` exclude with three
  specific third-party excludes (`dependencies/LLama_CPP/**`,
  `dependencies/Ollama/**`, `dependencies/HuggingFace_Hub/**`); added
  explicit credential excludes (`**/.env`, `**/.env.*`, `**/*.key`,
  `**/*.pem`, `**/secrets/**`) per §11.4.10.
- **Config JSON validity**: confirmed via
  `python3 -c "import json;json.load(open('.codegraph/config.json'))"` → VALID.
- **Index status BEFORE re-index**: Files 39,024 / Nodes 624,103 / Edges 1,643,200 / DB 1609.00 MB.
- **Index status AFTER re-index** (`codegraph index .`, exit 0): Files **76,044** / Nodes **1,255,974** / Edges **3,955,444** / DB 2617.24 MB. The +37,020 files / +631,871 nodes delta is the newly-included own-org submodule trees.
- **§11.4.79 anti-bluff probe (own-org symbol now resolves)**:
  - `codegraph query EventBus` → `dependencies/vasic-digital/event_bus/pkg/bus/bus.go:85` ✅ (would NOT have resolved under the old blanket `dependencies/**` exclude).
  - `codegraph query helix_memory` → `dependencies/HelixDevelopment/helix_memory/pkg/config/config.go` (+more) ✅.
  - `codegraph query llama` filtered to `dependencies/LLama_CPP` → **empty** ✅ (third-party correctly excluded).
- **HXC-017 status**: fully done — config fixed, re-index complete, own-org inclusion proven, third-party exclusion proven.

## 2026-05-29T06:37:00Z — codegraph_sync.sh @ /run/media/milosvasic/DATA4TB/Projects/HelixCode

**FAIL** — codegraph sync exited non-zero. Tail of log:\n\n```\n┌  Syncing CodeGraph
[2m│[0m
  _(raw progress-spinner log line removed — was 25387 chars of ANSI noise from codegraph_sync.sh; see qa-results/ logs)_
  _(raw progress-spinner log line removed — was 3007584 chars of ANSI noise from codegraph_sync.sh; see qa-results/ logs)_

## 2026-05-29 — codegraph 0.9.7 update: index/sync FAIL + §11.4.79 own-org regression (HXC-033)

**Event**: operator installed codegraph **0.9.7** on the host (`codegraph --version` → `0.9.7`).

**§11.4.80 post-update sync — FAILED (honest ledger, no bluff):**
- The 0.9.7 install reset the gitignored index DB (was 76,044 files / 1,255,974 nodes at HXC-017; dropped to 39,203).
- `constitution/scripts/codegraph_sync.sh` ran `codegraph sync .` → exited before completing its 4 steps (index reached 43,073 files).
- Full re-index `codegraph index .` → process **KILLED mid-run, no diagnostic / no exit code** (terminated by signal); index left at 54,207 files. Reproduced.
- `codegraph index . --force --quiet` → **KILLED again, no diagnostic**; `--force` wiped + left only 4,630 files.
- `codegraph sync . --quiet` → **exit 1** at 8,461 files.
- Host memory ample (51 GiB free) — not an obvious §12.6 OOM.

**§11.4.79 anti-bluff probe — FAILS:** `codegraph_search BundleTranslator` (MCP) returns ONLY `helix_code/internal/tools/askuser/...` — the own-org `dependencies/HelixDevelopment/llm_orchestrator/pkg/i18n/bundle.go` symbol does NOT resolve. Own-org submodules are NOT reachable in the 0.9.7 index. **This is a §11.4.79 regression introduced by the 0.9.7 update.**

**Not a config regression:** tracked `.codegraph/config.json` is intact (git-clean) — own-org includes + §11.4.10 credential excludes (`**/.env`, `**/*.key`, `**/secrets/**`) all present.

**Root cause: UNCONFIRMED** (§11.4.6) — codegraph 0.9.7 `index`/`sync` terminate without an actionable diagnostic on this 76k-file repo. Not determinable from captured evidence whether it is a 0.9.7 stability bug, a submodule-traversal change, or a config-schema change. Filed as **HXC-033**; needs operator decision (downgrade to prior working version / upstream bug report / accept degraded index). Evidence: `qa-results/codegraph_index_*.log`, `codegraph_recover_*.log`.

## 2026-05-29 — codegraph 0.9.7 RESOLVED: wipe + init + re-index restored own-org index (HXC-033 → Fixed)

**Resolution (operator-directed: "clear all indexed data fully and re-index — it MUST be a data-compatibility problem; ALWAYS index main + HelixDevelopment + vasic-digital").** Confirmed correct.

**Root cause (now CONFIRMED, was UNCONFIRMED):** codegraph **0.9.7 requires an explicit `codegraph init`** before `index` (behavioral change vs the prior version). The pre-0.9.7 index DB was incompatible; operating on it produced the failures. After a full wipe of the gitignored DB (`codegraph.db`/`-wal`/`-shm`; the 1.7 GB stale WAL was a tell) + `codegraph init` (tracked `config.json` preserved — own-org includes + §11.4.10 credential excludes intact) + `codegraph index .`, the index rebuilt cleanly.

**Two earlier mis-diagnoses corrected (per §11.4.6 — stated here as the record):** (1) "index crashes/killed mid-run" was a FAULTY `pgrep` pattern (`codegraph index` vs the real `node … codegraph.js index .`) giving false "ENDED" reads — the process was simply slow (76k files); real `ps` showed it alive (~600 MB RSS, healthy, climbing). (2) "own-org symbols do not resolve" used the WRONG CLI verb (`codegraph search` — removed in 0.9.7; the verb is `codegraph query`) AND queried a stale MCP-server DB handle from before the re-index.

**Result (CLI `codegraph status` on fresh 0.9.7 DB):** Files **75,663** / Nodes **1,272,492** / Edges finalizing (1.7M→ toward ~3.95M; edge-enrichment phase runs async after node indexing).

**§11.4.79 anti-bluff probe — PASS (`codegraph query`, fresh DB):**
- `NewBundleTranslator` → `dependencies/HelixDevelopment/llm_orchestrator/pkg/i18n/bundle.go:34` (+ `dependencies/vasic-digital/...`, `doc_processor`) — 10 own-org hits. ✅ HelixDevelopment + vasic-digital both reachable.
- `EventBus` → resolves. ✅
- `llama_model_load` under `dependencies/LLama_CPP` → empty. ✅ third-party correctly excluded.

**Status.md hygiene fix:** this ledger had bloated to 3.66 MB — a single 3,007,584-char line of raw ANSI progress-spinner output that `constitution/scripts/codegraph_sync.sh` dumped verbatim on its earlier FAIL. Stripped to 8 KB (all 7 real ledger entries preserved). FOLLOW-UP: `codegraph_sync.sh` should strip ANSI / not dump raw spinner logs into Status.md (constitution-submodule fix per §11.4.26).

**Operational follow-up:** the agent-facing codegraph **MCP server** (`tools/codegraph/...serve --mcp`, a separate install) holds the pre-wipe DB inode — it must be **restarted** to serve the fresh index to AI agents; the CLI `query` already reflects it.
