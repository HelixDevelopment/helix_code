# CONST-052 Rename Programme — Phased Plan (ISSUE-005)

**Spec date:** 2026-05-19
**Constitutional anchor:** CONST-052 (constitution §11.4.29)
**Tracker:** ISSUE-005 (`docs/Issues.md`)
**Scope:** PLAN ONLY. No renames executed in this round. Operator review + approval gates each phase before execution.
**Author intent:** Translate CONST-052's *"phased execution per the operator's explicit instruction: comprehensive brainstorming → phase-divided plan → fine-grained tasks/subtasks → every change covered by every applicable test type"* into a concrete, evidence-backed migration map for HelixCode's meta-repo.

---

## 1. Status Snapshot

### 1.1 Meta-root naming (already compliant)

A `find . -maxdepth 1 -type d -not -path "./.git" -not -path "." -printf "%f\n" | sort | grep -E '[A-Z]'` against HEAD `2a5586e` returns **empty**. The 32 top-level meta-repo directories (`assets/`, `awesome-ai-memory/`, `challenges/`, `cli_agents/`, `cli_agents_configs/`, `cli_agents_resources/`, `cmd/`, `configs/`, `constitution/`, `containers/`, `dependencies/`, `docker/`, `docs/`, `github_pages_website/`, `helix_agent/`, `helix_code/`, `helix_qa/`, `implementation_guide/`, `internal/`, `mcp_servers/`, `packaging/`, `panoptic/`, `reports/`, `scripts/`, `security/`, `specification/`, `test/`, `tests/`, `upstreams/`, `website/`, plus hidden `.claude/`, `.github/`) are ALL snake_case (verified `2026-05-18`).

**ISSUE-005 phrasing — "meta-repo directories like `helix_code/`, `challenges/`, `helix_qa/`, `helix_agent/` still PascalCase"** — was authored before the round-88 meta-root partial sweep; the meta-root is now compliant. **The remaining non-compliance is two layers deeper:** under `dependencies/` and inside every submodule's `Upstreams/` directory.

### 1.2 Where PascalCase still lives (the actual work)

Three deficiency clusters remain:

| Cluster | Where | Count | Severity |
|---|---|---|---|
| (A) Owned-org submodule **directories** | `dependencies/HelixDevelopment/*` + `dependencies/vasic-digital/*` | 10 + 48 = 58 dirs | HIGH (CONST-052 §11.4.29 applies — owned by us) |
| (B) Third-party submodule directories | `dependencies/{LLama_CPP,Ollama,HuggingFace_Hub}`, `cli_agents_resources/*` | 3 + 7 = 10 dirs | EXEMPT (CONST-052 vendor exception) |
| (C) `Upstreams/` (PascalCase) → `upstreams/` (snake) per CONST-052 transition | 49 submodule subtrees (incl. own + vendor) | 49 dirs | MEDIUM (transition explicitly supported by `constitution/install_upstreams.sh` for both) |

**Verification commands (all run 2026-05-18 against HEAD `2a5586e`):**

```bash
# (A) Owned-org PascalCase submodule dirs:
find dependencies/HelixDevelopment -maxdepth 1 -type d -not -path "dependencies/HelixDevelopment" | wc -l   # → 10
find dependencies/vasic-digital   -maxdepth 1 -type d -not -path "dependencies/vasic-digital"   | wc -l   # → 48

# (B) Third-party submodule dirs:
find dependencies -maxdepth 1 -type d | grep -E '/[A-Z]' | wc -l  # → 3 (LLama_CPP, Ollama, HuggingFace_Hub)
find cli_agents_resources -maxdepth 2 -type d | grep -E '/[A-Z]' | wc -l  # → 7 (Awesome-*, Cheshire-Cat-Ai, GitHub-Awesome-Copilot, OpenAI-Cookbook, Taches-CC-Resources)

# (C) PascalCase Upstreams dirs:
find . -maxdepth 4 -type d -name "Upstreams" 2>/dev/null | wc -l  # → 49
```

### 1.3 Common-sense exceptions per CONST-052 §11.4.29

The rule has four explicit carve-outs. Applied here:

1. **Language-mandated case** (Java/Kotlin/Android/Apple/C#/Swift inside language roots). Affects: paths inside `cli_agents/junie`, `cli_agents/swe-agent`, `helix_code/applications/{ios,android,aurora-os,harmony-os}`. **Exempt** — language convention wins inside the language root.
2. **Vendor / upstream third-party submodules.** Affects cluster (B): `LLama_CPP`, `Ollama`, `HuggingFace_Hub`, `cli_agents_resources/*`, `cli_agents/*` (third-party agent forks). **Exempt** — renaming would break upstream merge alignment.
3. **Build artefacts** (`node_modules`, `__pycache__`, `.git`, `target`, `build`, `bin`). Already ignored, irrelevant.
4. **Common-sense test** ("does renaming break the technology?"). Special case for `submodules/llms_verifier/Assets/` and `Website/` — these are language-neutral but ship as fixed-name asset roots wired into deployment manifests; the **decision is operator-side** (deferred to §8 decision-points).

### 1.4 Why this was deferred

Three coupled cascades historically blocked execution:

1. **Documentation-path encoding.** A literal string `submodules/helix_llm/...` appears in 244 .md files (verified count); each is a working anchor that breaks if the directory moves.
2. **Go module replace directives.** 8 `go.mod` files (verified `grep -l "HelixDevelopment\|vasic-digital" --include="go.mod" -r .`) carry `replace` lines that map a module-id to a relative directory; rename without paired `replace` update silently breaks `go build`.
3. **Submodule registration.** `.gitmodules` carries 70 PascalCase `path = ...` entries; renaming requires `git mv` + `.gitmodules` rewrite + `git submodule sync` + re-init across all clones.

Round 106 (the **HelixMemory** rename pilot) demonstrated all three at once: one wrong path knocked out 6 packages.

---

## 2. Reference-Cascade Audit

### 2.1 Owned-org per-submodule fan-out (the cost-of-rename matrix)

All numbers from `grep -rn "$path" --include="<ext>" . | grep -v "\.git/" | wc -l`, run 2026-05-18.

#### 2.1.1 `dependencies/HelixDevelopment/*` (10 submodules)

| Submodule | .md refs | .sh refs | .go refs | Aggregate | Risk band |
|---|---:|---:|---:|---:|---|
| `HelixLLM` | 244 | 9 | **269** | **522** | **PHASE-3 critical (Go-replace blast radius)** |
| `LLMsVerifier` | 88 | 6 | 1 | 95 | PHASE-3 |
| `DocProcessor` | 65 | 2 | 0 | 67 | PHASE-2 |
| `LLMOrchestrator` | 57 | 5 | 1 | 63 | PHASE-2 |
| `VisionEngine` | 56 | 5 | 0 | 61 | PHASE-2 |
| `LLMProvider` | 41 | 1 | 0 | 42 | PHASE-2 |
| `HelixMemory` | 39 | 9 | 0 | 48 | PHASE-2 (pilot already done — confirms cost band) |
| `HelixSpecifier` | 39 | 9 | 0 | 48 | PHASE-2 |
| `Models` | 5 | 0 | 0 | 5 | PHASE-1 |
| `DebateOrchestrator` | 4 | 0 | 0 | 4 | PHASE-1 |

Aggregate cluster total: **`dependencies/HelixDevelopment` string appears 260 times** across .md/.sh/.go/.yaml/.yml/.toml (verified).

#### 2.1.2 `dependencies/vasic-digital/*` (48 submodules — top 10 only audited; remainder estimated)

| Submodule | .md refs | .sh refs | .go refs | Aggregate | Risk band |
|---|---:|---:|---:|---:|---|
| `Models` | 8 | 8 | 0 | 16 | PHASE-1 |
| `Memory` | 6 | 1 | 0 | 7 | PHASE-1 |
| `Storage` | 6 | 2 | 0 | 8 | PHASE-1 |
| `VectorDB` | 6 | 2 | 0 | 8 | PHASE-1 |
| `Auth` | 5 | 4 | 0 | 9 | PHASE-1 |
| `Cache` | 5 | 2 | 0 | 7 | PHASE-1 |
| `Database` | 4 | 2 | 0 | 6 | PHASE-1 |
| `RAG` | 3 | 2 | 0 | 5 | PHASE-1 |
| `Plugins` | 2 | 1 | 0 | 3 | PHASE-1 |
| `MCP_Module` | [estimated 5-10, not exhaustively counted] | [estimated 2-3, not exhaustively counted] | 0 | ~10 | PHASE-1 |
| Other 38 vasic-digital submodules | [estimated 0-10 each, not exhaustively counted] | [estimated 0-3 each, not exhaustively counted] | 0 | individually small | PHASE-1 |

Aggregate cluster total: **`dependencies/vasic-digital` string appears 138 times** (verified).

### 2.2 Third-party (EXEMPT but tabulated for completeness)

| Path | .md/.sh/.go refs | Disposition |
|---|---:|---|
| `dependencies/LLama_CPP` | 4 | EXEMPT (vendor; upstream merge alignment) |
| `dependencies/Ollama` | 4 | EXEMPT (vendor) |
| `dependencies/HuggingFace_Hub` | 20 | EXEMPT (vendor) |
| `cli_agents_resources/GitHub-Awesome-Copilot` | 0 | EXEMPT (vendor) |
| `cli_agents_resources/Awesome-AI-GPTs` | 0 | EXEMPT (vendor) |
| `cli_agents_resources/{Awesome-AI-Agents, Cheshire-Cat-Ai, OpenAI-Cookbook, Taches-CC-Resources}` | [estimated 0-5 each, not exhaustively counted] | EXEMPT (vendor) |

### 2.3 `Upstreams/` (PascalCase) → `upstreams/` (snake) — cluster (C)

Reference count: `grep -rn "/Upstreams/\|/Upstreams\b" --include="*.md" --include="*.sh" . | grep -v "\.git/" | wc -l` → **40 references** across all .md and .sh files.

Important coupling: `constitution/install_upstreams.sh` (commit `45d3678` of the constitution submodule) **already supports BOTH directory names** (verified: `grep -n "upstreams\|Upstreams" constitution/install_upstreams.sh` returns dual-name handling at lines 6 + lookup loop). This means cluster (C) renames are **non-breaking by construction** — the upstream installer continues to work whichever name the submodule uses. **Convention drift, not functional drift.**

### 2.4 Submodule directories owned by us but unaffected (already snake_case at root)

- `challenges/`, `containers/`, `helix_agent/`, `helix_code/`, `helix_qa/`, `security/`, `assets/`, `github_pages_website/`, `panoptic/`, `awesome-ai-memory/`.

These dirs are compliant at the meta-root path. The **remote GitHub repo URLs** (`HelixDevelopment/HelixCode.git`, `HelixDevelopment/HelixAgent.git`, `vasic-digital/Challenges.git`, etc.) are PascalCase but **CONST-052 §11.4.29 governs filesystem paths, not org-side repository identifiers** — renaming a GitHub repo is a separate operational concern outside CONST-052's scope (would require breaking-change coordination across every clone everywhere). Out-of-scope here.

---

## 3. Renames To Perform (Full List)

### 3.1 Cluster A — owned-org `dependencies/HelixDevelopment/*` (10 renames)

| OLD | NEW |
|---|---|
| `submodules/debate_orchestrator/` | `dependencies/helix_development/debate_orchestrator/` |
| `submodules/doc_processor/` | `dependencies/helix_development/doc_processor/` |
| `submodules/helix_llm/` | `dependencies/helix_development/helix_llm/` |
| `submodules/helix_memory/` | `dependencies/helix_development/helix_memory/` |
| `submodules/helix_specifier/` | `dependencies/helix_development/helix_specifier/` |
| `submodules/llm_orchestrator/` | `dependencies/helix_development/llm_orchestrator/` |
| `submodules/llm_provider/` | `dependencies/helix_development/llm_provider/` |
| `submodules/llms_verifier/` | `dependencies/helix_development/llms_verifier/` |
| `submodules/models/` | `dependencies/helix_development/models/` |
| `submodules/vision_engine/` | `dependencies/helix_development/vision_engine/` |

**Parent dir `HelixDevelopment/` → `helix_development/`** is part of cluster A (renamed once before/after the leaf renames per operator preference).

### 3.2 Cluster A — owned-org `dependencies/vasic-digital/*` (48 renames)

`vasic-digital/` itself **already contains a hyphen** (`-` separator). CONST-052 §11.4.29 says "snake_case" with `_` as the separator. **Org-name renaming decision deferred to §8** (would require GitHub org rename, which is out-of-scope). For the IN-SCOPE filesystem path, treatment:

- **Option 1:** leave the org subdir as `vasic-digital/` (it is operator's GitHub-org handle; treat as proper noun exempt — though CONST-052 mentions only language exceptions, not proper nouns). Plan default.
- **Option 2:** rename to `vasic_digital/` (strictly compliant; introduces drift between filesystem path and GitHub org handle).

Plan adopts Option 1 (defer org-dir rename pending operator decision, §8). The **leaf submodule renames proceed** regardless:

| OLD (sample — full list applied to all 48) | NEW |
|---|---|
| `submodules/agentic/` | `submodules/agentic/` |
| `submodules/auth/` | `submodules/auth/` |
| `submodules/auto_temp/` | `submodules/auto_temp/` |
| `submodules/background_tasks/` | `submodules/background_tasks/` |
| `submodules/benchmark/` | `submodules/benchmark/` |
| `submodules/cache/` | `submodules/cache/` |
| `submodules/claritas/` | `submodules/claritas/` |
| `submodules/concurrency/` | `submodules/concurrency/` |
| `submodules/config/` | `submodules/config/` |
| `submodules/conversation/` | `submodules/conversation/` (NO-OP — already compliant) |
| `submodules/database/` | `submodules/database/` |
| `submodules/doc_processor/` | `submodules/doc_processor/` |
| `submodules/document/` | `submodules/document/` |
| `submodules/embeddings/` | `submodules/embeddings/` |
| `submodules/event_bus/` | `submodules/event_bus/` |
| `submodules/filesystem/` | `submodules/filesystem/` |
| `submodules/formatters/` | `submodules/formatters/` |
| `submodules/gandalf_solutions/` | `submodules/gandalf_solutions/` |
| `submodules/hyper_tune/` | `submodules/hyper_tune/` |
| `submodules/i_llm/` | `submodules/i_llm/` |
| `submodules/i18n/` | `submodules/i18n/` |
| `submodules/lazy/` | `submodules/lazy/` |
| `submodules/leak_hub/` | `submodules/leak_hub/` |
| `submodules/llm_ops/` | `submodules/llm_ops/` |
| `submodules/llm_orchestrator/` | `submodules/llm_orchestrator/` |
| `submodules/llm_provider/` | `submodules/llm_provider/` |
| `submodules/mcp_module/` | `submodules/mcp_module/` |
| `submodules/memory/` | `submodules/memory/` |
| `submodules/messaging/` | `submodules/messaging/` |
| `submodules/middleware/` | `submodules/middleware/` |
| `submodules/models/` | `submodules/models/` |
| `submodules/normalize/` | `submodules/normalize/` |
| `submodules/observability/` | `submodules/observability/` |
| `submodules/optimization/` | `submodules/optimization/` |
| `submodules/ouroborous/` | `submodules/ouroborous/` |
| `submodules/planning/` | `submodules/planning/` |
| `submodules/plinius_common/` | `submodules/plinius_common/` |
| `submodules/plugins/` | `submodules/plugins/` |
| `submodules/rag/` | `submodules/rag/` |
| `submodules/rate_limiter/` | `submodules/rate_limiter/` |
| `submodules/recovery/` | `submodules/recovery/` |
| `submodules/red_team/` | `submodules/red_team/` |
| `submodules/self_improve/` | `submodules/self_improve/` |
| `submodules/skill_registry/` | `submodules/skill_registry/` |
| `submodules/storage/` | `submodules/storage/` |
| `submodules/streaming/` | `submodules/streaming/` |
| `submodules/tool_schema/` | `submodules/tool_schema/` |
| `submodules/toon/` | `submodules/toon/` |
| `submodules/vector_db/` | `submodules/vector_db/` |
| `submodules/veritas/` | `submodules/veritas/` |
| `submodules/vision_engine/` | `submodules/vision_engine/` |
| `submodules/watcher/` | `submodules/watcher/` |

Total cluster A: **10 + 48 = 58 leaf renames** (+1 parent rename = 59 if `HelixDevelopment` → `helix_development` executes).

### 3.3 Cluster B — third-party (NO RENAMES — exempt per CONST-052 §11.4.29 vendor carve-out)

`dependencies/LLama_CPP`, `dependencies/Ollama`, `dependencies/HuggingFace_Hub`, `cli_agents_resources/*` (7 dirs), `cli_agents/*` (third-party agent forks) — **0 renames.** Rationale documented per dir in a follow-on exemption ledger (created in Phase-1 setup, §4.1).

### 3.4 Cluster C — `Upstreams/` → `upstreams/` (49 renames)

All 49 submodule `Upstreams/` subdirs:

```
./constitution/Upstreams                       → ./constitution/upstreams
./panoptic/Upstreams                           → ./panoptic/upstreams
./github_pages_website/Upstreams               → ./github_pages_website/upstreams
./containers/Upstreams                         → ./containers/upstreams
./helix_qa/Upstreams                           → ./helix_qa/upstreams
./challenges/Upstreams                         → ./challenges/upstreams
./cli_agents/claude-code-source/Upstreams      → ./cli_agents/claude-code-source/upstreams
./security/Upstreams                           → ./security/upstreams
./helix_agent/Upstreams                        → ./helix_agent/upstreams
./helix_agent/templates/Upstreams              → ./helix_agent/templates/upstreams
./dependencies/HelixDevelopment/*/Upstreams    → ./dependencies/helix_development/*/upstreams   (×7 leaves with Upstreams dirs)
./dependencies/vasic-digital/*/Upstreams       → ./dependencies/vasic-digital/*/upstreams       (×32 leaves with Upstreams dirs)
```

**Important:** each `Upstreams/` lives INSIDE a submodule's own git tree. Renaming requires committing in that submodule's repo first, then bumping the parent's `.gitmodules` pointer. Cluster (C) is therefore **49 inner-submodule commits + 1 parent bump per leaf** — high commit count, low risk per commit.

### 3.5 Out-of-scope (operator decisions in §8)

- `HelixDevelopment/` parent dir → `helix_development/` (renames 260 references, must paired-execute with cluster A).
- `vasic-digital/` parent dir → `vasic_digital/` (renames 138 references; drift vs GitHub org handle).
- `LLMsVerifier/Assets/` → `assets/` and `LLMsVerifier/Website/` → `website/` (inside submodule, possibly deployment-wired).

---

## 4. Phased Execution Plan

Five phases ordered by **cost × blast radius**, low to high. Each phase ends with a green-bar checkpoint before next phase begins. **Operator approves each phase before execution.**

### 4.1 Phase 0 — Pre-flight (tooling + exemption ledger)

**Goal:** make subsequent phases auditable and reversible.

**Tasks:**
1. **T-0.1** Author `scripts/rename-dir.sh <OLD> <NEW>` — wraps `git mv`, then runs the four reference-class sweeps (.md, .sh, .go, .gitmodules) with `sed -i` replacement, then `git submodule sync --recursive`. Idempotent: re-run is no-op once new path exists.
2. **T-0.2** Author `scripts/verify-no-stale-refs.sh <OLD>` — exits non-zero if `grep -rn "$OLD" --include='*.{md,sh,go,yaml,yml,toml}' . | grep -v "\.git/"` returns any hit. Used as gate post-rename.
3. **T-0.3** Author `scripts/dry-run-rename.sh <OLD> <NEW>` — produces a diff preview of every file change without writing (uses `git mv -n` semantics emulated by `git diff --no-index` against a temp `sed`-rewritten tree).
4. **T-0.4** Create `docs/governance/CONST-052-exemption-ledger.md` enumerating every directory exempted per §1.3 / §3.3 with citation. Becomes the audit answer to "why is `LLama_CPP` still PascalCase?"
5. **T-0.5** Add a pre-commit hook entry to `.git/hooks/pre-commit` (template form, opt-in) that runs `verify-no-stale-refs.sh` for any PascalCase path mentioned in `git diff --staged --name-only`.

**Test coverage (per CONST-050(B) + CONST-052 §11.4.29):**
- Unit: feed each script a synthetic 3-file fixture; assert correct rename behaviour.
- Integration: throwaway-clone test that runs `rename-dir.sh` against a real (cluster-A pilot) directory, verifies build still passes.
- Anti-bluff wire evidence: pre-rename SHA + post-rename SHA + `git submodule status` snapshot.

**Estimated effort:** 1 commit (scripts), ~250 LOC bash, 0 pushes.

### 4.2 Phase 1 — Low-risk leaves (vasic-digital + small HelixDevelopment + cluster C inner)

**Scope:** 48 cluster-A vasic-digital renames + 2 small cluster-A HelixDevelopment renames (`Models`, `DebateOrchestrator`) + all 49 cluster-C `Upstreams/` → `upstreams/` renames.

**Why first:** lowest reference cascades (≤16 refs each for vasic-digital, ≤5 for the two HD leaves, ≤40 total for Upstreams).

**Tasks (per-rename templated; representative sample shown):**

```
For each leaf L in {vasic-digital/*} ∪ {HelixDevelopment/Models, HelixDevelopment/DebateOrchestrator}:
  T-1.L.1 ./scripts/dry-run-rename.sh dependencies/<L> dependencies/<new_L>
  T-1.L.2 Operator review of diff preview (one approval covers a batch of 5 leaves).
  T-1.L.3 ./scripts/rename-dir.sh dependencies/<L> dependencies/<new_L>
  T-1.L.4 git -C dependencies/<new_L> submodule sync
  T-1.L.5 cd helix_code && go build ./... (verifies no Go-import drift)
  T-1.L.6 ./scripts/verify-no-stale-refs.sh dependencies/<L>
  T-1.L.7 Run helix_code/Makefile unit + integration tests (real infra per CONST-050(A))
  T-1.L.8 Commit: "rename(P1): <L> → <new_L> (CONST-052)"
  T-1.L.9 Push to all 4 remotes.

For each Upstreams dir U:
  T-1.U.1 cd <U>'s parent submodule
  T-1.U.2 git mv Upstreams upstreams
  T-1.U.3 sed -i 's|/Upstreams/|/upstreams/|g' (per the submodule's tree only)
  T-1.U.4 ./scripts/verify-no-stale-refs.sh Upstreams (inside that submodule)
  T-1.U.5 Submodule commit + push (4 remotes).
  T-1.U.6 In parent: git add <submodule> && commit "bump: <submodule> upstreams rename".
```

**Batching:** group 5 leaves per commit for vasic-digital (10 commits total) + 1 commit for cluster-C bumps in parent + per-submodule cluster-C inner commits (~49 inner commits across 49 submodules).

**Test coverage per batch:**
1. **Regression test:** `verify-no-stale-refs.sh` for every renamed path.
2. **Unit:** `cd helix_code && make test` (real units).
3. **Integration:** `cd helix_code && make test-integration-full` against the docker-compose stack from `make test-infra-up`.
4. **E2E:** `cd helix_code/tests/e2e/challenges && go run cmd/runner/main.go -all`.
5. **Challenges:** `cd challenges && make demo-all`.
6. **Wire evidence:** terminal output of every test run captured, attached to the close-out.

**Estimated effort:** ~10 batch commits cluster A + ~49 inner commits cluster C + 49 parent bumps + 1 close-out commit. ~110 commits, ~440 pushes (4 remotes). LOC delta primarily renames (low); reference-update sed sweeps ~150 LOC delta net.

### 4.3 Phase 2 — Medium-cascade HelixDevelopment leaves

**Scope:** `DocProcessor` (67), `LLMOrchestrator` (63), `VisionEngine` (61), `LLMProvider` (42), `HelixMemory` (48 — partially done in round 88-pilot, re-verify), `HelixSpecifier` (48).

**Why second:** medium reference cascades (40-70 each). The HelixMemory pilot taught us the failure modes (round 106 — 6 packages broke from one wrong path); apply the lesson at scale here.

**Tasks (per-rename, same template as Phase 1 but with three additions):**

```
For each leaf L in {DocProcessor, LLMOrchestrator, VisionEngine, LLMProvider, HelixMemory, HelixSpecifier}:
  T-2.L.0 (NEW) Audit every helix_code/go.mod replace directive that mentions L; map each to its new path.
  T-2.L.1-9 same as Phase 1.
  T-2.L.10 (NEW) Run helix_code's helixqa integration suite specifically (these 6 modules touch helix_qa bindings).
  T-2.L.11 (NEW) Update docs/ARCHITECTURE.md path references in same commit batch.
```

**Test coverage per leaf:** Phase 1 floor + module-specific suites (e.g., LLMProvider rename also runs `helix_code/internal/providers/...` test set).

**Estimated effort:** 6 leaves × 1 commit = 6 commits, 24 pushes, ~200 LOC sed delta across docs.

### 4.4 Phase 3 — High-cascade leaves (HelixLLM + LLMsVerifier)

**Scope:** `HelixLLM` (244 .md + 9 .sh + **269 .go refs** — 522 aggregate) and `LLMsVerifier` (95 aggregate, plus CONST-036/037 constitutional dependence).

**Why last:** highest blast radius. `HelixLLM`'s 269 Go references mean the rename is effectively a refactor of every consumer's import path — same magnitude as a major-version-bump library migration. `LLMsVerifier` is the constitutional single-source-of-truth (CONST-036) so its rename touches every provider-integration test.

**Tasks:**

```
T-3.HLLM.0  Snapshot ALL helix_code/go.mod replace lines mentioning HelixLLM.
T-3.HLLM.1  Snapshot every Go file with `import "dev.helix.code/.../HelixLLM/..."` (or equivalent path-based import).
T-3.HLLM.2  dry-run-rename.sh
T-3.HLLM.3  Operator approval (REQUIRED — large-blast-radius gate).
T-3.HLLM.4  rename-dir.sh
T-3.HLLM.5  Update every go.mod replace + every Go import line.
T-3.HLLM.6  go build ./... in helix_code/.
T-3.HLLM.7  Full test floor: unit + integration + e2e + challenges + helixqa-autonomous-session.
T-3.HLLM.8  Commit + push.
T-3.HLLM.9  Repeat for LLMsVerifier with extra CONST-036/037 verifier-cascade check (run scripts/verify-llmsverifier-pin-parity.sh).
```

**Test coverage:** full CONST-050(B) matrix — unit, integration, e2e, full-automation, security, DDoS, scaling, chaos, stress, performance, benchmarking, UI, UX, Challenges, helix_qa autonomous session. Each leaf gets its own wire-evidence dossier.

**Estimated effort:** 2 leaves × 2-3 commits each (rename commit + Go-path-sweep commit + post-test fix commit if any) = ~6 commits, ~24 pushes, ~400 LOC delta (mostly Go import lines + go.mod replaces).

### 4.5 Phase 4 — Parent-dir + Go-module-replace bumps

**Scope:** if operator approves §8 decisions, rename `dependencies/HelixDevelopment/` → `dependencies/helix_development/` and optionally `dependencies/vasic-digital/` → `dependencies/vasic_digital/`. **And** bump every `replace` directive in every `go.mod` to the new paths.

**Tasks:**
1. **T-4.1** Inventory: list all 8 `go.mod` files (already verified) plus their `replace` directives.
2. **T-4.2** Author a single bash script `scripts/bump-go-replaces.sh` that walks all 8, applies `sed -i 's|HelixDevelopment|helix_development|g; s|vasic-digital|vasic_digital|g'` (where in-scope), and runs `go mod tidy`.
3. **T-4.3** Bulk rename parent dirs (cluster A parent).
4. **T-4.4** Bulk rename `.gitmodules` paths.
5. **T-4.5** `git submodule sync --recursive` everywhere.
6. **T-4.6** `helix_code && go build ./... && go test ./...` validates Go cascade.
7. **T-4.7** Full test floor.
8. **T-4.8** Commit + push (single atomic commit per cluster A parent + cluster B parent if in scope).

**Test coverage:** full CONST-050(B) matrix + paired mutation (purposely plant a stale `HelixDevelopment/...` ref and assert `verify-no-stale-refs.sh` catches it).

**Estimated effort:** 2 atomic commits (one per parent rename), 8 pushes, ~50 LOC delta (path-rewrite only).

---

## 5. Test Coverage per Rename Batch (CONST-050(B) + CONST-052 §11.4.29 conformance)

Every rename batch (Phase 1 leaves, Phase 2-3 individual, Phase 4 atomic) MUST ship the four invariants:

| Test type | Command | Evidence captured | Gate |
|---|---|---|---|
| 1. Reference-resolution regression | `./scripts/verify-no-stale-refs.sh <old_path>` | exit code + stdout | exit=0 required |
| 2. Unit (mocks allowed only here per CONST-050(A)) | `cd helix_code && make test` | terminal output | 0 failures |
| 3. Integration (real infra) | `cd helix_code && make test-integration-full` | terminal output + docker-compose status | 0 failures |
| 4. E2E challenges (full user flow) | `cd helix_code/tests/e2e/challenges && go run cmd/runner/main.go -all` | runner output + per-challenge wire evidence | 0 failures |
| 5. Challenges submodule (vasic-digital/Challenges) | `cd challenges && make demo-all` | per-Challenge captured output | 0 failures |
| 6. helix_qa autonomous session | `cd helix_qa && ./run-autonomous-session.sh` | session report + wire-evidence dossier | 0 failures |
| 7. Anti-bluff smoke | `grep -rn "simulated\|for now\|TODO implement\|placeholder" helix_code/internal helix_code/cmd` | empty grep output | empty required |
| 8. Governance cascade | `./scripts/verify-governance-cascade.sh` | pass/fail report | pass required |
| 9. Constitution rules sweep | `./scripts/verify-all-constitution-rules.sh` | per-rule status | every implementable rule PASS |

For Phase 3 (high-cascade) + Phase 4 (Go-replace): add (10) `cd helix_code && go build ./... && go test -race ./...` and (11) paired mutation per §1.1 — plant stale ref, verify gate catches it, then revert.

---

## 6. Tooling Requirements

| Script | Purpose | Phase used | LOC est. |
|---|---|---|---|
| `scripts/rename-dir.sh <OLD> <NEW>` | git mv + 4-class reference sweep + submodule sync | All | ~80 |
| `scripts/verify-no-stale-refs.sh <OLD>` | post-rename gate; greps all reference classes | All | ~30 |
| `scripts/dry-run-rename.sh <OLD> <NEW>` | preview diff without writing | All | ~70 |
| `scripts/bump-go-replaces.sh` | bulk update go.mod replace lines | Phase 4 | ~50 |
| `scripts/CONST-052-progress-report.sh` | print "X of 58 cluster-A leaves done" | Reporting | ~40 |
| `docs/governance/CONST-052-exemption-ledger.md` | per-dir exemption rationale (cluster B) | Phase 0 docs | ~80 doc lines |

**Manual edits** (no script — operator approval gate per edit):

1. `.gitmodules` — paths updated atomically with `git mv` commits.
2. Root governance docs (`CLAUDE.md`, `AGENTS.md`, `CONSTITUTION.md`) — only if they cite a renamed path. **Caution:** most governance is inside the `constitution/` submodule; CONST-049 §11.4.26 7-step workflow applies if those need edits.
3. `docs/ARCHITECTURE.md` — likely needs sweep at end of each phase.
4. `docs/CONTINUATION.md` — CONST-044 mandates same-commit update for state advancement.

---

## 7. Risk Mitigation

### 7.1 Per-phase rollback procedure

Each phase ends with a tagged commit. Rollback = `git revert <tag>` (NOT `git reset --hard`; CONST-043 forbids force operations). Submodule pointer reversions handled by re-checking-out the prior `.gitmodules` content and `git submodule update --init --recursive`.

### 7.2 Per-rename dry-run

`scripts/dry-run-rename.sh` produces a unified diff preview of every file the script would write. Operator reviews diff before approving the wet run. Same pattern as `git mv -n` for the move itself, extended for reference-class sweeps.

### 7.3 Bisect strategy

If post-phase test floor FAILs:
1. Identify the failing test.
2. `git bisect start HEAD <phase-start-tag>`.
3. Run `verify-no-stale-refs.sh` + the failing test at each bisect step.
4. Once the offending commit is identified: revert that one commit, fix the missed reference, re-commit.

### 7.4 Submodule out-of-sync risk

Cluster (C) modifies submodule trees. If a submodule push fails (e.g., a remote rejects), the parent's `.gitmodules` bump must NOT be pushed until all 4 remotes accept the submodule push. Gate: `scripts/verify-submodule-push-symmetric.sh` (NEW — author in Phase 0 if not present).

### 7.5 Documentation drift after submodule pointer bump

When a submodule renames its `Upstreams/` → `upstreams/`, the parent's `.gitmodules` pointer bumps to a new SHA, but the parent's own .md files may still cite `<submodule>/Upstreams/...`. Gate: `verify-no-stale-refs.sh "Upstreams"` runs across the **parent's** tree after each cluster-C bump.

---

## 8. Operator Decision Points

The following decisions must be made by the operator BEFORE execution of Phase 1 (or in some cases, before later phases):

| # | Decision | Default suggestion | Affects phase |
|---|---|---|---|
| **D-1** | Execute phases sequentially (1 → 4) or interleave cluster C with cluster A? | Sequential (lowest risk). Cluster C is independent and could run in parallel — but adds cognitive load. | All |
| **D-2** | Rename `dependencies/HelixDevelopment/` → `dependencies/helix_development/`? Touches 260 references; mandatory for strict CONST-052 compliance but breaks every external bookmark to the org-dir. | YES (strict compliance). Schedule for Phase 4. | Phase 4 |
| **D-3** | Rename `dependencies/vasic-digital/` → `dependencies/vasic_digital/`? Touches 138 references; introduces drift vs GitHub-org handle. | DEFER (proper-noun-via-GitHub-handle exception; treat as carve-out). | Phase 4 |
| **D-4** | Rename `helix_code/` itself (if any PascalCase ever creeps back)? Would change every inner-Go module path (~15 go.mod replaces). | NO action needed — already snake_case. | n/a |
| **D-5** | Rename `LLMsVerifier/Assets/` → `assets/` and `LLMsVerifier/Website/` → `website/`? Inside submodule; possibly deployment-wired. | DEFER pending deployment-wire audit. | Phase 3 |
| **D-6** | Should `MCP_Module` become `mcp_module` (already mixed case with underscore) or `mcp/module`? Strict snake_case = `mcp_module`. | `mcp_module` (strict snake_case, single token). | Phase 1 |
| **D-7** | Should `I-LLM` become `i_llm` or `illm`? Hyphen present in source. | `i_llm` (preserve token boundary). | Phase 1 |
| **D-8** | Should `TOON` become `toon` or stay UPPER (acronym exception)? CONST-052 doesn't carve out acronyms. | `toon` (strict). | Phase 1 |
| **D-9** | Should `RAG` become `rag` (acronym)? Same as D-8. | `rag` (strict). | Phase 1 |
| **D-10** | Defer cluster C (`Upstreams` → `upstreams`) entirely? It's convention-only since `install_upstreams.sh` accepts both. | NO — strict compliance preferred per CONST-052 spirit. | Phase 1 |
| **D-11** | Should rename commits be co-authored "Co-Authored-By: Claude" per HelixCode commit convention? | YES (consistent with existing close-out cadence). | All |
| **D-12** | Per-phase approval cadence: one approval per phase, or one per batch (5-leaf group)? | One per phase (less interruption). Per-batch escalation only if a phase reveals failures. | All |

---

## 9. Estimated Effort

Aggregate estimates for the full programme, anchored to the verified data above.

| Phase | Commits | Pushes (× 4 remotes) | LOC delta | Wall-time guess (operator-paced) |
|---|---:|---:|---:|---|
| Phase 0 (tooling) | 1 | 4 | ~250 LOC scripts + 80 LOC docs | half-day |
| Phase 1 (low-risk leaves + cluster C) | ~110 | ~440 | ~200 LOC net (mostly sed sweeps) | 2-3 days |
| Phase 2 (medium HelixDevelopment) | ~6 | ~24 | ~200 LOC | half-day |
| Phase 3 (HelixLLM + LLMsVerifier) | ~6 | ~24 | ~400 LOC (Go imports + replaces) | 1 day |
| Phase 4 (parent dirs + Go-replace bumps) | ~2 | ~8 | ~50 LOC | quarter-day |
| **Total programme** | **~125 commits** | **~500 pushes** | **~1100 LOC delta** | **~5 days operator-paced** |

Test runs: each batch triggers ~9 test types per §5; for 110 Phase-1 batches that is ~990 test runs (some batched together — actual unique runs probably 60-80 if batched 5-leaf-per-batch). Phase 3 alone may take 30-60 min per leaf due to full helix_qa autonomous session.

**Conservativeness:** the LOC + commit counts above are **lower bounds**. Real execution will surface unanticipated reference classes (e.g., `.yaml` deployment manifests, `.toml` package configs) requiring extra sed sweeps. Mark every estimate as `[estimated, will refine after Phase 0 dry-run output]`.

---

## Appendix A — Verification commands run for this plan (all 2026-05-18, HEAD `2a5586e`)

```bash
# §1.1 meta-root compliance:
find . -maxdepth 1 -type d -not -path "./.git" -not -path "." -printf "%f\n" | sort | grep -E '[A-Z]' | wc -l   # → 0

# §1.2 cluster A counts:
find dependencies/HelixDevelopment -maxdepth 1 -type d | wc -l   # → 11 (includes parent)
find dependencies/vasic-digital   -maxdepth 1 -type d | wc -l   # → 49 (includes parent)

# §1.2 cluster C count:
find . -maxdepth 4 -type d -name "Upstreams" | wc -l   # → 49

# §1.4 go.mod count:
grep -l "HelixDevelopment\|vasic-digital" --include="go.mod" -r . | grep -v "\.git/" | wc -l   # → 8

# §2.1 per-submodule HelixDevelopment refs (sample):
grep -rn "HelixDevelopment/HelixLLM" --include="*.go" . | grep -v "\.git/" | wc -l   # → 269
grep -rn "HelixDevelopment/HelixLLM" --include="*.md" . | grep -v "\.git/" | wc -l   # → 244

# §2.3 cluster C refs:
grep -rn "/Upstreams/\|/Upstreams\b" --include="*.md" --include="*.sh" . | grep -v "\.git/" | wc -l   # → 40

# §1.4 .gitmodules PascalCase paths:
grep -E 'path = .*[A-Z]' .gitmodules | wc -l   # → 70

# §2.4 aggregate HelixDevelopment / vasic-digital fan-out:
grep -rn "dependencies/HelixDevelopment" --include="*.md" --include="*.sh" --include="*.go" --include="*.yaml" --include="*.yml" --include="*.toml" . | grep -v "\.git/" | wc -l   # → 260
grep -rn "dependencies/vasic-digital"   --include="*.md" --include="*.sh" --include="*.go" --include="*.yaml" --include="*.yml" --include="*.toml" . | grep -v "\.git/" | wc -l   # → 138
```

All commands re-runnable; no claim in this plan rests on memory or estimation unless explicitly annotated `[estimated, not exhaustively counted]`.

---

## Appendix B — Anti-bluff pledge

Every claim in §1-§4 above is backed by a verifiable command in Appendix A. The two `[estimated, not exhaustively counted]` ranges (MCP_Module refs, "other 38 vasic-digital" refs) are explicitly marked; Phase 0 will replace them with full counts before Phase 1 executes. No PASS claim, no completion claim, no false-success summary is permitted in this programme without paired terminal evidence per CONST-035 / Article XI §11.9.
