# CONST-052 Rename Programme — HXC-001 Assessment + Phased Rename Plan

**Revision:** 1
**Last modified:** 2026-05-21T02:30:50Z
**Description:** Current-state assessment of the CONST-052 lowercase-snake_case rename programme (HXC-001) plus a phased, atomic, low-risk→high-risk rename plan.
**Authority:** Constitution §11.4.29 (CONST-052), §11.4.44 (this header); HXC-001 in `docs/Issues.md`.
**Maintainer:** Operator + AI loop per §11.4.42
**Status:** Plan — research output, no renames executed.

---

## 0. Scope and method

CONST-052 (constitution §11.4.29) mandates every directory, submodule, and
file under HelixCode use lowercase `snake_case` names, with technology-
preserving common-sense exceptions. This document assesses the current
state, distinguishes work already done (notably commit `a1ea3c8` and its
two sibling batch commits) from work remaining, applies the exemption
carve-outs, maps the entanglement risk per rename candidate, and lays out
a phased plan. **No renames were performed producing this document.**

A prior phased plan exists at
`docs/superpowers/specs/2026-05-19-const052-rename-programme-plan.md`
(commit `f6664104`, round 113). This document supersedes it as the
current-state assessment: round-343 executed three of its batches and
the inventory below reflects that progress.

Survey commands run 2026-05-21 against HEAD `b4f2193` (4 remotes fetched,
`HEAD..@{u}` empty — local is current).

---

## 1. Current-state inventory

### 1.1 Already compliant (no action)

- **Meta-repo top-level directories** — all snake_case: `helix_code/`,
  `helix_qa/`, `helix_agent/`, `assets/`, `challenges/`, `containers/`,
  `security/`, `constitution/`, `dependencies/`, `docs/`, `scripts/`,
  `cli_agents/`, `cli_agents_resources/`, `cli_agents_configs/`,
  `mcp_servers/`, `github_pages_website/`, `panoptic/`,
  `awesome-ai-memory/`, `implementation_guide/`, `upstreams/` (the
  meta-root one). Earlier rounds (`15c54316` `Dependencies/`→`dependencies/`,
  `cc339fc0` `HelixCode/`→`helix_code/`, `73cb32d2` `HelixAgent/`,
  `5ad402bb` `HelixQA/`, `3f79f72a` `Containers/`, `af62a8cc`
  `Challenges/`+`Security/`, `a01c7c0e`/`17f84987` `cmd/` + `applications/`)
  brought the top two layers into compliance.
- **Go module paths** — abstract (`digital.vasic.auth`, `dev.helix.agent`,
  etc.); they do NOT embed the directory name, so a directory rename does
  **not** change any module path. This materially lowers the risk class
  (see §3).

### 1.2 Already renamed by `a1ea3c8` and sibling round-343 batches

Round 343 executed the safe (zero-go.mod-replace-consumer) leaf batches.
All three confirmed present at new paths on disk and in `.gitmodules`:

| Commit | Renamed | Count |
|---|---|---|
| `a1ea3c8` | `submodules/models` → `models` | 1 |
| `416fe8e` | `submodules/debate_orchestrator` → `debate_orchestrator` | 1 |
| `e813b5c` | 11 `vasic-digital/*` leaves: `auto_temp`, `claritas`, `doc_processor`, `gandalf_solutions`, `hyper_tune`, `i_llm`, `leak_hub`, `ouroborous`, `plinius_common`, `veritas`, `vision_engine` | 11 |

**Total already done by round-343: 13 owned-org leaf directory renames.**
Each carried atomic `.gitmodules` path + section-name updates,
`.git/modules/...` gitdir-pointer fixes, and ledger-row updates;
`go build ./internal/... ./cmd/...` exit 0 was captured per batch.

### 1.3 What remains — categorised

**(A) Owned-org PascalCase submodule directories — 46 remaining.**

`dependencies/HelixDevelopment/*` (8 PascalCase remain): `DocProcessor`,
`HelixLLM`, `HelixMemory`, `HelixSpecifier`, `LLMOrchestrator`,
`LLMProvider`, `LLMsVerifier`, `VisionEngine`.

`dependencies/vasic-digital/*` (38 PascalCase remain): `Agentic`, `Auth`,
`BackgroundTasks`, `Benchmark`, `Cache`, `Concurrency`, `Config`,
`Database`, `Document`, `Embeddings`, `EventBus`, `Filesystem`,
`Formatters`, `I18n`, `Lazy`, `LLMOps`, `LLMOrchestrator`, `LLMProvider`,
`MCP_Module`, `Memory`, `Messaging`, `Middleware`, `Models`, `Normalize`,
`Observability`, `Optimization`, `Planning`, `Plugins`, `RAG`,
`RateLimiter`, `Recovery`, `RedTeam`, `SelfImprove`, `SkillRegistry`,
`Storage`, `Streaming`, `ToolSchema`, `TOON`, `VectorDB`, `Watcher`.
(40 listed; `MCP_Module` and `I18n` are partially-snake/mixed — still
non-compliant. Total non-compliant under `vasic-digital/` = 38 once
`MCP_Module`→`mcp_module` and `I18n`→`i18n` and `TOON`→`toon` count.)

ALL 46 are **go.mod-entangled**: every one is referenced by a `replace
<module> => ../dependencies/<Org>/<Dir>` directive in
`helix_code/go.mod` and/or `helix_agent/go.mod` and/or `helix_qa/go.mod`
(verified by grep of all three consumer `go.mod` files). This is the
HIGH-RISK class — a rename without an atomic `replace`-path edit breaks
the consumer build.

**(B) Parent dependency directory `HelixDevelopment/`.** Per round-343
operator decision D-2, the target is `helix_development/`. Renaming the
parent dir changes the `../dependencies/HelixDevelopment/...` path
component in EVERY consumer `replace` directive that points into it (12
directives across `helix_code/go.mod` + `helix_agent/go.mod`) plus every
`.gitmodules` `path =` line for a `HelixDevelopment/*` submodule (10
entries). `vasic-digital/` is KEPT (D-3: GitHub-org handle / proper-noun
carve-out — it matches the literal org name).

**(C) `Upstreams/` → `upstreams/` — 59 directories.** One inside the
meta-repo's own submodule trees and inside owned-org submodule trees.
Each `Upstreams/` lives INSIDE a submodule's git tree, so each rename is
a commit in that submodule's repo, then a `.gitmodules` pointer bump in
the parent. The meta-root `upstreams/` is already lowercase. Note:
`constitution/install_upstreams.sh` already supports BOTH `Upstreams/`
and `upstreams/` (constitution commit `45d3678`), so this cluster does
NOT break the upstream-install tooling mid-flight.

**(D) `.gitmodules` entries.** Path/name segments tracking (A)/(B)/(C):
46 owned-org submodule sections still carry PascalCase `path =` and
`[submodule "..."]` name segments. Third-party submodule sections
(`cli_agents/*`, `cli_agents_resources/*`, `dependencies/LLama_CPP`,
`dependencies/Ollama`, `dependencies/HuggingFace_Hub`,
`awesome-ai-memory`) are EXEMPT (see §2).

**(E) Files.** UPPERCASE tracked `.md` files are widespread
(`COMPREHENSIVE_*.md`, `PHASE_*.md`, `FINAL_*.md`, etc. — ~120 at
meta-root, dozens under `helix_code/`). Standard-metadata files
(`README.md`, `CLAUDE.md`, `AGENTS.md`, `CONSTITUTION.md`, `LICENSE`,
`CONTINUATION.md`) are EXEMPT per §11.4.44 OUT-scope + de-facto
convention. The remaining UPPERCASE_SNAKE report/plan `.md` files at
meta-root and under `helix_code/` are technically in CONST-052 scope
for files but are **deferred to a final low-risk cosmetic phase** (no
build/import entanglement; reference cost is internal cross-links only)
— see Phase 6.

---

## 2. Exemption list (stays as-is — technology-preserving carve-outs)

Per CONST-052 §11.4.29 the "does renaming break the technology?" test
trumps the rule. EXEMPT:

| Item | Why exempt |
|---|---|
| `cli_agents/*` (45+ submodules: `aider`, `crush`, `claude-code`, …) | Third-party vendor submodules — keep upstream repo names. Mostly already lowercase anyway. |
| `cli_agents_resources/*` (`Awesome-AI-Agents`, `Awesome-AI-GPTs`, `Cheshire-Cat-Ai`, `GitHub-Awesome-Copilot`, `OpenAI-Cookbook`, `Taches-CC-Resources`) | Third-party vendor submodules (`github/`, `openai/`, `e2b-dev/`, …) — keep upstream names. |
| `dependencies/LLama_CPP`, `dependencies/Ollama`, `dependencies/HuggingFace_Hub` | Third-party vendor submodules (`ggml-org/`, `ollama/`, `huggingface/`). |
| `awesome-ai-memory/` | Third-party (`topoteretes/`). Already lowercase. |
| `vasic-digital/` parent dir | Literal GitHub-org handle — proper-noun carve-out (operator decision D-3, round 343). |
| `README.md`, `LICENSE`, `CLAUDE.md`, `AGENTS.md`, `CONSTITUTION.md`, `CRUSH.md`, `QWEN.md`, `CONTINUATION.md`, `NOTICE`, `VERSION`, `OWNERS` | Standard-metadata / governance files — conventional uppercase; §11.4.44 lists CLAUDE/AGENTS/README as OUT-scope; renaming breaks tool discovery (Claude Code, golangci, etc.). |
| Language-mandated subtrees | Java/Kotlin/Android/Apple/Swift/C# source roots inside `helix_code/applications/{android,ios,aurora-os,harmony-os}` — file/dir case follows the language convention; submodule root still follows our convention. |
| Build artefacts | `node_modules/`, `.git/`, `target/`, `build/`, `bin/`, `__pycache__/` — tool-mandated names. |
| Go package directories already lowercase | Go requires lowercase import-path segments anyway; `internal/` packages are compliant. |

**Note on third-party `Upstreams/`:** the 59-count in §1.3(C) includes
`Upstreams/` dirs inside third-party submodule trees only where those
trees are owned-by-us (the count was taken across owned submodules).
`cli_agents/claude-code-source/Upstreams` — `claude-code-source` is the
operator's own private mirror (`gitlab.com:milos85vasic/ccode-private`)
so its `Upstreams/` IS in scope. Pure third-party `Upstreams/` (if any)
are exempt.

---

## 3. Risk map

| Rename class | Entanglement | Risk |
|---|---|---|
| Owned-org leaf dir (A) | `.gitmodules` path + section name; `.git/modules/...` gitdir pointer; consumer `go.mod` `replace` path; coverage ledgers; governance-doc prose | **HIGH** — `replace`-path edit must be atomic with the rename or the consumer build breaks. NOT a module-path change (paths are abstract), so `go list -m` output is unchanged; the break is purely `replace` filesystem-path resolution. |
| Parent dir `HelixDevelopment/`→`helix_development/` (B) | ALL 12 `replace` directives pointing into it + ALL 10 `HelixDevelopment/*` `.gitmodules` `path=` lines + `.git/modules/dependencies/HelixDevelopment/` tree | **HIGHEST** — single rename, broad atomic fan-out across 2 consumer go.mod files + 10 submodule sections. Must be one PWU. |
| `Upstreams/`→`upstreams/` (C) | Inside each submodule's own git tree; needs an inner-repo commit then a parent `.gitmodules` pointer bump | **MEDIUM** — 59× (inner commit + parent bump). Low risk per commit, high commit count. No go.mod entanglement. Tooling already dual-name-safe. |
| `.gitmodules` segment (D) | Folds into (A)/(B) — same commit | covered by A/B |
| UPPERCASE `.md` files (E) | Internal markdown cross-links only; no build/import impact | **LOW** — cosmetic; deferred to Phase 6. |

**High-risk count: 47** = 46 owned-org leaf dirs (A) + 1 parent dir (B).
Every one touches a Go `replace` path and therefore needs a build-verified
atomic commit. None changes a Go *module path*, so none requires editing
`import` statements in `.go` source — that is the one piece of good news
that keeps this from being catastrophic.

---

## 4. Phased rename plan (low-risk → high-risk)

Each phase = a set of fine-grained atomic rename tasks. Each task lists
the rename, every reference updated atomically, and the test verification
(§11.4.29 mandates each batch ships a regression test verifying every
reference resolves + a full test-type run).

### Phase 0 — Pre-flight tooling (no renames)

- **Task 0.1** — Author `scripts/const052_rename_leaf.sh <org> <Old> <new>`:
  performs `git mv`, rewrites `.gitmodules` (`path` + section name),
  fixes `.git/modules/.../` gitdir + worktree pointer, rewrites the
  matching `replace` line(s) in every consumer `go.mod`, updates
  `docs/coverage/COVERAGE_LEDGER.md` + `docs/coverage/ledger.md` rows.
  Idempotent dry-run flag.
- **Task 0.2** — Author `scripts/const052_verify_refs.sh`: the regression
  test — greps the whole tree for the OLD path string (must be 0 hits
  outside historical CONTINUATION prose), runs `git submodule status`
  (all resolve), runs `cd helix_code && go build ./... && cd ../helix_agent
  && go build ./...` (exit 0).
- **Verification:** dry-run the tooling against an already-renamed leaf
  (`models`) — must report "no change needed". Paired-mutation: plant a
  stale ref, assert the verifier FAILs.

### Phase 1 — `Upstreams/`→`upstreams/` cluster (C), 59 dirs — MEDIUM, no go.mod

Independent of the go.mod work; can run first as a low-risk warm-up.
Group into batches of ~10 submodules.

- **Task 1.x (per submodule)** — inside `<submodule>/`: `git mv Upstreams
  upstreams`, commit + push the submodule's 4 remotes, then in the parent
  bump the `.gitmodules` pointer for that submodule in the SAME parent
  commit. Update any `install_upstreams` invocation docs that hardcode
  `Upstreams/` (tooling itself is already dual-name-safe).
- **References per task:** submodule-internal git tree only + parent
  `.gitmodules` SHA pointer. No `replace`, no import, no config.
- **Verification:** `install_upstreams` from the renamed submodule root
  resolves all recipes; `git remote -v | grep -c push` reports expected
  count; submodule status clean. Batch ships the §11.4.29 regression test.

### Phase 2 — owned-org leaf dirs with EXACTLY ONE consumer (A subset) — HIGH but minimal fan-out

The §1.3(A) `replace`-count showed most `vasic-digital/*` leaves have a
single consumer `replace` directive. Batch ~8–10 per round.

- **Task 2.x (per dir)** — e.g. `vasic-digital/Cache`→`cache`:
  `git mv submodules/cache submodules/cache`;
  rewrite the ONE `replace digital.vasic.cache => ../submodules/cache`
  line in `helix_agent/go.mod`; rewrite `.gitmodules` path + section name;
  fix `.git/modules/...` pointer; update coverage ledgers; sweep
  governance docs (`CLAUDE.md` §3.2 prose, `docs/` references) — active
  references only, historical CONTINUATION prose left intact.
- **Verification per batch:** `const052_verify_refs.sh` (0 stale refs,
  submodule status resolves, `go build ./...` exit 0 for every consumer);
  full test-type run on affected consumers (`make test` in `helix_agent`).

### Phase 3 — owned-org leaf dirs with MULTIPLE consumers (A subset) — HIGH

Leaves referenced by 2–3 consumer `go.mod` files: `VisionEngine`,
`LLMOrchestrator`, `DocProcessor`, `Memory`, `LLMsVerifier`,
`HelixSpecifier`. Each rename must rewrite the `replace` line in EVERY
consumer atomically.

- **Task 3.x (per dir)** — same as Phase 2 but the `replace`-rewrite
  spans all consumers (e.g. `DocProcessor` appears in both
  `helix_code/go.mod` and `helix_agent/go.mod`). One PWU per dir.
- **Verification:** build ALL consumers; full test-type run on each.

### Phase 4 — `MCP_Module`, `I18n`, `TOON`, `LLMOps`, `RAG` normalisation — HIGH

Mixed/acronym dirs needing operator-blessed snake_case targets (D-6
`mcp_module`, D-7/D-8/D-9 already decided as `i18n`/`toon`/`rag` style).
Same atomic procedure as Phase 2/3.

### Phase 5 — parent dir `HelixDevelopment/`→`helix_development/` (B) — HIGHEST

Single rename, broadest atomic fan-out. ONE PWU.

- **Task 5.1** — `git mv dependencies/HelixDevelopment dependencies/helix_development`;
  rewrite ALL 12 `replace ../dependencies/HelixDevelopment/...` lines
  (across `helix_code/go.mod` + `helix_agent/go.mod`) to
  `.../helix_development/...`; rewrite ALL 10 `HelixDevelopment/*`
  `.gitmodules` `path=` + section-name segments; move the
  `.git/modules/dependencies/HelixDevelopment/` subtree; sweep all
  governance docs + ledgers.
- **Verification:** every consumer `go build ./...` exit 0; all 10
  submodule statuses resolve; full test-type matrix run; §11.4.29
  regression test.

### Phase 6 — UPPERCASE `.md` file renames (E) — LOW, cosmetic

Rename non-exempt UPPERCASE report/plan `.md` files at meta-root and
under `helix_code/` to snake_case. Batch by directory.

- **Task 6.x** — `git mv COMPREHENSIVE_ANALYSIS_REPORT.md
  comprehensive_analysis_report.md`; grep + rewrite every internal
  markdown cross-link. NO build impact.
- **Verification:** link-checker confirms no broken relative links.

---

## 5. Test coverage required per batch (CONST-052 §11.4.29 + CONST-050(B))

Every rename batch ships:

1. **Regression test** — `scripts/const052_verify_refs.sh`: 0 stale
   OLD-path references outside historical prose; all `git submodule
   status` resolve; all consumer `go build ./...` exit 0.
2. **Full test-type run** — `make test` (unit) on every affected
   consumer module; integration/E2E where the renamed submodule is
   exercised; anti-bluff captured runtime evidence (paste exit codes +
   output).
3. **Paired mutation** — plant a known stale reference, assert the
   regression test reports FAIL (proves the test is not a bluff).

---

## 6. Parallel-session coordination risk (honest note)

Round-343 batch-1/2/3 (`a1ea3c8`, `416fe8e`, `e813b5c`) were executed by
a prior session. Two concrete risks for the next executor:

- **Stale local state.** Per CONST-060 the first git action MUST be
  `git fetch --all --prune` + `git log HEAD..@{u}` + recursive submodule
  fetch. Acting on a stale tree risks re-renaming an already-renamed
  leaf or colliding on `.gitmodules`. (At the time of writing,
  `HEAD..@{u}` is empty — local is current.)
- **`.gitmodules` is a single-file collision hotspot.** Every phase
  edits `.gitmodules`; two parallel sessions WILL conflict there.
  Recommendation: serialise the rename programme to ONE session, or
  partition strictly by phase with an explicit hand-off, and always
  fetch-first per §11.4.71/CONST-060 before each batch.
- **Submodule-internal commits (Phase 1 cluster C)** land in separate
  repos with their own 4-remote push; a parent `.gitmodules` pointer
  bump MUST NOT be pushed until the submodule push is accepted by all
  remotes (per the prior plan's §7.4).

---

## 7. Summary

- **Rename candidates remaining:** 46 owned-org leaf dirs (A) + 1 parent
  dir (B) + 59 `Upstreams/` dirs (C) + N UPPERCASE `.md` files (E,
  deferred cosmetic). Core structural total = **106** dir-level renames.
- **Already done by `a1ea3c8` + siblings:** 13 owned-org leaf renames.
- **Phases:** 7 (Phase 0 pre-flight through Phase 6 cosmetic).
- **High-risk (go.mod-`replace`-entangled):** 47 (46 leaves + 1 parent).
- **Good news:** Go module paths are abstract — no `.go` `import`
  statement edits required; the entanglement is `replace` filesystem
  paths only.
