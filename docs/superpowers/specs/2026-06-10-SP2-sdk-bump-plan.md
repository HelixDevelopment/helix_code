# SP2 — SDK Bump Plan (provider/infra SDK currency)

**Revision:** 1
**Created:** 2026-06-10
**Last modified:** 2026-06-10
**Maintainer:** CLI-Agent Fusion programme (SP2 P2.3 SDK-currency)
**Status:** active
**Status summary:** Read-only planning artefact — ordered, risk-classified go.mod bump sequence for HelixCode's provider/infra SDKs.

> READ-ONLY PLAN. This document changes nothing. It cites the currency research at
> `docs/research/2026-06-10-sdk-cli-currency.md` (PART A) and the exact import-site
> evidence gathered from the two go.mod files + the `*.go` import statements. Every
> bump step below is to be executed one SDK at a time per §11.4.42 iteration discipline,
> §11.4.92 multi-pass, §11.4.40 full-suite-retest, and §11.4.108 four-layer verification.

---

## Table of contents

- [1. Inventory — current pins vs latest](#1-inventory--current-pins-vs-latest)
- [2. Classification](#2-classification)
- [3. Per-bump detail (edit + importers + verification + rollback)](#3-per-bump-detail-edit--importers--verification--rollback)
- [4. Ordered execution plan](#4-ordered-execution-plan)
- [5. helix_code vs helix_agent coordination](#5-helix_code-vs-helix_agent-coordination)
- [6. Final ordered bump list + code-change verdict](#6-final-ordered-bump-list--code-change-verdict)

---

## 1. Inventory — current pins vs latest

Module IDs are both `dev.helix.code` (inner) / `dev.helix.agent` (helix_agent submodule).
Inner app go.mod = `helix_code/go.mod`; submodule = `submodules/helix_agent/go.mod`.

| SDK module | helix_code pin | helix_agent pin | LATEST (research §A) | Skew | Same-major? |
|---|---|---|---|---|---|
| `github.com/aws/aws-sdk-go-v2` (core) | v1.32.7 (`helix_code/go.mod:16`) | — (not used) | v1.42.0 | 10 minor | yes (v1) |
| `github.com/aws/aws-sdk-go-v2/config` | v1.28.7 (`helix_code/go.mod:17`) | — | siblings of core | — | yes |
| `github.com/aws/aws-sdk-go-v2/credentials` | v1.17.48 (`helix_code/go.mod:18`) | — | siblings of core | — | yes |
| `github.com/aws/aws-sdk-go-v2/service/bedrockruntime` | v1.23.1 (`helix_code/go.mod:19`) | — | **v1.53.5** | **~30 minor** | yes (v1) |
| `github.com/aws/smithy-go` | v1.22.1 (`helix_code/go.mod:20`) | — | (transitive of AWS core) | — | yes |
| `github.com/Azure/azure-sdk-for-go/sdk/azcore` | v1.16.0 (`helix_code/go.mod:13`) | — | v1.22.0 | 6 minor | yes (v1) |
| `github.com/Azure/azure-sdk-for-go/sdk/azidentity` | v1.8.0 (`helix_code/go.mod:14`) | — | **v1.13.1** | 5 minor | yes (v1) — **SECURITY GO-2024-2918** |
| `github.com/getzep/zep-go/v3` | v3.10.0 (`helix_code/go.mod:27`) | — | v3.23.0 | 13 minor | yes (v3) |
| `github.com/gin-gonic/gin` | **v1.12.0** (`helix_code/go.mod:28`) | v1.12.0 (`submodules/helix_agent/go.mod:5`) | v1.12.0 | **none — already aligned** | yes |
| `github.com/smacker/go-tree-sitter` | `v0.0.0-20240827094217-dd81d9e9be82` (`helix_code/go.mod:41`) | same commit (`submodules/helix_agent/go.mod:73`) | smacker at HEAD (dormant); maintained alt = `tree-sitter/go-tree-sitter v0.25.0` | at HEAD | n/a — migration, not bump |

### Important fact-correction vs the research's pin snapshot

The research (`docs/research/2026-06-10-sdk-cli-currency.md:13-16`) recorded `helix_code` gin
as **v1.11.0**. The CURRENT `helix_code/go.mod:28` already reads **`gin v1.12.0`** — the gin
skew described in the research summary item 6 **is already resolved** (per the gitStatus this
session: "gin already fixed to 1.12"). Both modules are on gin v1.12.0. **No gin bump remains.**
This is captured here so the executor does not re-do already-landed work (§11.4.37 fetch-before-edit /
§11.4.114 known-good).

---

## 2. Classification

| Bump | Class | Rationale |
|---|---|---|
| `service/bedrockruntime` v1.23.1 → v1.53.5 | **BREAKING-RISK** (large jump, needs its own careful sub-task) | Same v1 major, but ~30 minor jump. Generated AWS service clients add model/operation/types churn across 30 minors; the import surface in `bedrock_provider.go` includes `.../bedrockruntime/types` + `ResponseStreamReader` + `InvokeModelWithResponseStream` — event-stream + types are the historically churny areas. Treat as code-change-possible, isolate. Core+config+credentials+smithy MUST move together (shared internal versions). |
| `aws-sdk-go-v2` core + `config` + `credentials` + `smithy-go` | **SAFE-COORDINATED** (must move as a set, before/with bedrockruntime) | Same v1 line. v1.42.0's new standard-retry is opt-in behind `AWS_NEW_RETRIES_2026` env (non-breaking per research §A row 1). These four + the ~10 AWS indirect modules (`internal/configsources`, `internal/endpoints/v2`, `service/internal/presigned-url`, `sso`, `ssooidc`, `sts`, `protocol/eventstream`, etc. at `helix_code/go.mod:82-91`) share lock-step internal versions; `go mod tidy` resolves the set. |
| `azidentity` v1.8.0 → v1.13.1 | **SECURITY** (prioritise — GO-2024-2918 EoP) | Same v1 line. Carries the Azure Identity elevation-of-privilege advisory on old releases (research §A row 4). Stay on **stable v1.13.1**; v1.14.0 is beta-only — do NOT adopt beta. |
| `azcore` v1.16.0 → v1.22.0 | **SAFE** | Same v1 line, additive (research §A row 5). azidentity v1.13.1 likely pulls a newer azcore as a minimum anyway, so doing azcore alongside/just-before azidentity avoids a double tidy. |
| `getzep/zep-go/v3` v3.10.0 → v3.23.0 | **SAFE** | Same v3 line, additive (research §A row 2). Single importer file. |
| gin | **DONE / no-op** | Already v1.12.0 in both modules. No action. |
| `smacker/go-tree-sitter` → `tree-sitter/go-tree-sitter v0.25.0` | **DEFER** (separate migration epic, NOT in this bump batch) | Different import path + different API (Close-on-allocation), 30+ language sub-package imports across `internal/repomap/tree_sitter.go`, `internal/repomap/tag_extractor.go`, `internal/tools/mapping/treesitter_parsers.go` (helix_code) + `submodules/helix_agent/internal/clis/aider/repo_map.go` (helix_agent). No vendor supersession statement (research §A migration note). Real migration work — do NOT fold into a currency bump. |

---

## 3. Per-bump detail (edit + importers + verification + rollback)

For every step: pre-flight `git fetch --all --prune` (§11.4.37), branch off `main` (never edit
go.mod on `main` directly), run the bump, then the listed verification. Rollback for ALL = `git
checkout -- go.mod go.sum && go mod tidy` (or `git stash`/`git restore` to the pre-bump SHA).
All commands run from `helix_code/` unless noted.

### 3.1 — getzep/zep-go/v3 (SAFE, do first — smallest blast radius)

- **Edit:** `helix_code/go.mod:27` `v3.10.0` → `v3.23.0`.
- **Importer (only one prod file):** `internal/memory/providers/zep_provider.go` —
  imports `getzep/zep-go/v3` (`zep`), `.../v3/client` (`zepclient`), `.../v3/option`
  (`zep_provider.go:14-16`). 1342 lines; the API surface to re-verify is the zep client
  constructor + memory get/add option chains.
- **Command:**
  ```bash
  go get github.com/getzep/zep-go/v3@v3.23.0 && go mod tidy && go build ./... \
    && go test ./internal/memory/...
  ```
- **§11.4.40 verification:** `go build ./...` clean + `go vet ./internal/memory/...` +
  `go test -count=1 ./internal/memory/...` GREEN. If a real-Zep integration test exists
  (`-tags=integration`), run it against the live endpoint per CONST-050(A) (no mocks beyond unit).
- **Rollback:** `git checkout -- go.mod go.sum && go mod tidy`.

### 3.2 — azcore (SAFE) then azidentity (SECURITY) — do as a pair

- **Edit:** `helix_code/go.mod:13` azcore `v1.16.0` → `v1.22.0`; `helix_code/go.mod:14`
  azidentity `v1.8.0` → `v1.13.1`. Also the indirect `sdk/internal v1.10.0`
  (`helix_code/go.mod:78`) will likely bump via tidy.
- **Importers:** `internal/llm/azure_provider.go` — imports
  `sdk/azcore`, `sdk/azcore/policy`, `sdk/azidentity` (`azure_provider.go:18-20`); uses
  `azcore.TokenCredential`, `azidentity.NewManagedIdentityCredential`,
  `azidentity.ClientID`, `azidentity.NewDefaultAzureCredential`
  (`azure_provider.go:39,150,252,261-270`). Test: `internal/llm/azure_provider_test.go`.
- **Command:**
  ```bash
  go get github.com/Azure/azure-sdk-for-go/sdk/azcore@v1.22.0
  go get github.com/Azure/azure-sdk-for-go/sdk/azidentity@v1.13.1
  go mod tidy && go build ./... && go test ./internal/llm/...
  ```
- **§11.4.40 verification:** build clean + `go test -count=1 ./internal/llm/...` GREEN. The
  credential constructors above are the contract to re-verify (managed-identity + default-cred
  signatures are the most likely churn points). SECURITY note: confirm `go.sum` now records
  azidentity ≥ v1.13.1 (the GO-2024-2918-patched line) — capture `go list -m
  github.com/Azure/azure-sdk-for-go/sdk/azidentity` output as the §11.4.5 evidence the
  advisory is cleared. Optionally run `govulncheck ./internal/llm/...` to prove GO-2024-2918
  no longer reports.
- **Rollback:** `git checkout -- go.mod go.sum && go mod tidy`.

### 3.3 — aws-sdk-go-v2 core set (SAFE-COORDINATED) — gate for bedrockruntime

- **Edit:** `helix_code/go.mod:16` core `v1.32.7` → `v1.42.0`; `:17` config; `:18`
  credentials; `:20` smithy-go — bump together; let `go mod tidy` settle the ~10 AWS indirect
  modules at `helix_code/go.mod:82-91`.
- **Importers:** `internal/llm/bedrock_provider.go` — `aws-sdk-go-v2/aws`,
  `.../config` (`awsconfig`), `.../credentials`, `.../smithy-go`
  (`bedrock_provider.go:13-15,18`); uses `smithy.APIError` (`bedrock_provider.go:1038`),
  `config.LoadDefaultConfig`-style `NewFromConfig` plumbing.
- **Command:**
  ```bash
  go get github.com/aws/aws-sdk-go-v2@v1.42.0 \
         github.com/aws/aws-sdk-go-v2/config@latest \
         github.com/aws/aws-sdk-go-v2/credentials@latest \
         github.com/aws/smithy-go@latest
  go mod tidy && go build ./... && go test ./internal/llm/...
  ```
  (Resolve config/credentials to the versions tidy picks as compatible with core v1.42.0
  rather than pinning a literal — they track core.)
- **§11.4.40 verification:** build clean + `go test -count=1 ./internal/llm/...` GREEN. Do NOT
  set `AWS_NEW_RETRIES_2026` (leave retry behaviour unchanged — research §A row 1). Capture
  `go list -m github.com/aws/aws-sdk-go-v2` as evidence.
- **Rollback:** `git checkout -- go.mod go.sum && go mod tidy`.

### 3.4 — service/bedrockruntime (BREAKING-RISK — own careful sub-task)

> Do this ONLY after 3.3 lands GREEN (the core set is the dependency floor for the service client).

- **Edit:** `helix_code/go.mod:19` `v1.23.1` → `v1.53.5`.
- **Importers:** `internal/llm/bedrock_provider.go` —
  `service/bedrockruntime` + `service/bedrockruntime/types` (`bedrock_provider.go:16-17`).
  Concrete API surface to re-verify (the churn-risk points across 30 minors):
  - `InvokeModel(ctx, *bedrockruntime.InvokeModelInput, ...func(*bedrockruntime.Options))`
    + `InvokeModelWithResponseStream(...)` interface methods (`bedrock_provider.go:24-25`);
  - `bedrockruntime.NewFromConfig(awsCfg)` (`bedrock_provider.go:238`);
  - `InvokeModelInput` / `InvokeModelWithResponseStreamInput` struct fields
    (`bedrock_provider.go:456,493`);
  - `bedrockruntime.ResponseStreamReader` event-stream iface in `processEventStream`
    (`bedrock_provider.go:880`) — **highest churn risk** (event-stream types move most).
  - Tests: `internal/llm/bedrock_provider_test.go`, `internal/llm/response_err_round54_test.go`.
- **Command:**
  ```bash
  go get github.com/aws/aws-sdk-go-v2/service/bedrockruntime@v1.53.5
  go mod tidy && go build ./... && go test ./internal/llm/...
  ```
- **If `go build ./...` FAILS** (the breaking-risk case): the failure is the §11.4.108
  ARTIFACT/SOURCE signal. Per §11.4.102 systematic-debugging FIRST — read the compile error,
  diff the changed `types`/`ResponseStreamReader`/`*Input` symbol against v1.53.5's GoDoc
  (verify against `pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/bedrockruntime` per
  §11.4.99 latest-source), update the call sites in `bedrock_provider.go` minimally, re-build.
  This is the one bump where code edits are expected, so it is split into its own sub-task and
  its own commit.
- **§11.4.40 verification:** build clean + `go test -count=1 ./internal/llm/...` GREEN +, if a
  real-Bedrock integration test gate exists, a live `InvokeModel` round-trip producing a real
  response (anti-bluff per §11.9 — NEVER a simulated response per BLUFF-001). Capture the real
  AWS response as §11.4.69 `network_connectivity` / LLM-output evidence.
- **Rollback:** `git checkout -- go.mod go.sum internal/llm/bedrock_provider.go && go mod tidy`.

### 3.5 — smacker → tree-sitter migration (DEFER — not part of this batch)

- **Why deferred:** different import path (`github.com/tree-sitter/go-tree-sitter`) + different
  API (mandatory `Close()` on allocations). Importers to migrate (count = real work):
  - helix_code: `internal/repomap/tree_sitter.go` (sitter + 9 language subpkgs,
    `tree_sitter.go:9-18`), `internal/repomap/tag_extractor.go` (`:8`),
    `internal/tools/mapping/treesitter_parsers.go` (sitter + **30 language subpkgs**,
    `treesitter_parsers.go:6-33`), `internal/tools/mapping/doc.go`, plus test
    `internal/repomap/incremental_p2t06_test.go`;
  - helix_agent: `submodules/helix_agent/internal/clis/aider/repo_map.go`.
- **Blocker:** the 30+ smacker language sub-packages (`/bash`, `/c`, `/cpp`, `/golang`, …) do
  NOT have a 1:1 mapping in `tree-sitter/go-tree-sitter` — each language is a separate
  `tree-sitter-<lang>/bindings/go` module. This is a multi-file, multi-module migration epic
  that MUST be its own plan with its own §11.4.43 RED tests per language. **Track as a separate
  workable item; do not bump in the currency batch.** smacker is at HEAD, so currency is not
  regressing by leaving it.

---

## 4. Ordered execution plan

One SDK (or coordinated set) per branch/commit, each fully verified before the next. All in
`helix_code` (helix_agent has none of these except the deferred tree-sitter — see §5).

1. **zep-go/v3** (SAFE, smallest) — §3.1. Commit on GREEN.
2. **azcore + azidentity** (SAFE + SECURITY pair) — §3.2. **Prioritised within the safe tier
   because of GO-2024-2918.** Capture the patched-version evidence. Commit on GREEN.
3. **aws core set** (core+config+credentials+smithy, SAFE-COORDINATED) — §3.3. Commit on GREEN.
   This is the dependency floor for step 4.
4. **service/bedrockruntime** (BREAKING-RISK, own sub-task) — §3.4. Expect possible code edits in
   `bedrock_provider.go`; systematic-debugging on any compile break; own commit.
5. **(DEFER)** smacker → tree-sitter migration — §3.5. NOT in this batch; spin a separate plan.

After EACH step: `go build ./...` + the affected-package `go test -count=1` GREEN before
proceeding (§11.4.42). After the WHOLE batch: full `go test -count=1 ./...` + `go vet ./...` +
anti-bluff smoke (§9 of root CLAUDE.md) + §11.4.125 code-review gate before any tag.

Ordering rationale: safe→security→breaking, BUT the AWS core set (3.3) must precede
bedrockruntime (3.4) because the service client depends on the core/internal modules' versions;
within the "safe" tier azidentity is pulled forward to clear the security advisory early.

## 5. helix_code vs helix_agent coordination

- **bedrockruntime, aws core set, azcore, azidentity, zep-go/v3** — **helix_code ONLY.**
  `submodules/helix_agent/go.mod` does NOT require any of these (confirmed: it has no
  `aws-sdk-go-v2`, no `Azure/azure-sdk-for-go`, no `getzep/zep-go`). No cross-module
  coordination needed for steps 1-4.
- **gin** — both modules **already on v1.12.0** (`helix_code/go.mod:28`,
  `submodules/helix_agent/go.mod:5`). The "gin-skew" the research warned of is already closed;
  no action. (Keep them aligned in future bumps — gin-skew style coordination still applies if
  either moves.)
- **smacker/go-tree-sitter** — pinned at the SAME commit in BOTH modules
  (`helix_code/go.mod:41`, `submodules/helix_agent/go.mod:73`). The deferred migration is the
  ONE item needing gin-skew-style cross-module coordination: when it runs, helix_code's 5 files
  + helix_agent's `repo_map.go` must migrate together so the pinned commit / new module stays
  consistent across both go.mod files. Plan that as a coordinated two-module epic, not a bump.

## 6. Final ordered bump list + code-change verdict

| # | SDK | go.mod line(s) | Class | Code changes? |
|---|---|---|---|---|
| 1 | `getzep/zep-go/v3` v3.10.0→v3.23.0 | `helix_code/go.mod:27` | SAFE | **No** (pure go.mod; verify `zep_provider.go` compiles) |
| 2 | `azcore` v1.16.0→v1.22.0 + `azidentity` v1.8.0→v1.13.1 | `helix_code/go.mod:13,14` | SAFE + **SECURITY** | **No** expected (pure go.mod; verify `azure_provider.go` cred ctors) |
| 3 | aws core: `aws-sdk-go-v2` v1.32.7→v1.42.0 + `config` + `credentials` + `smithy-go` | `helix_code/go.mod:16,17,18,20` | SAFE-COORDINATED | **No** expected (pure go.mod; tidy settles ~10 indirects) |
| 4 | `service/bedrockruntime` v1.23.1→v1.53.5 | `helix_code/go.mod:19` | **BREAKING-RISK** | **Possibly YES** — `bedrock_provider.go` `types`/`ResponseStreamReader`/`*Input` may need edits; own sub-task + own commit |
| 5 | `smacker`→`tree-sitter/go-tree-sitter v0.25.0` | `helix_code/go.mod:41` + `submodules/helix_agent/go.mod:73` | **DEFER** | **YES (heavy)** — import-path + API migration across 5 helix_code files + 1 helix_agent file + 30+ language subpkgs; separate epic, NOT this batch |

**Pure-go.mod (no code expected):** steps 1, 2, 3.
**Code-change candidate (its own careful sub-task):** step 4 (bedrockruntime 30-minor jump).
**Out of scope / separate epic (definite code migration, two-module coordination):** step 5
(tree-sitter import-path migration).
