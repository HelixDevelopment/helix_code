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
