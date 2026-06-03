# Phase 2 — Flatten own-org submodules to `submodules/<leaf>` + rewrite refs (Design / Migration Map)

**Date:** 2026-06-03
**Status:** Approved (operator decisions 2026-06-03) — execution authorized, subagent-driven, incremental with build verification.
**Risk:** VERY HIGH / hard-to-reverse. Restore anchor: `docs/migration/phase0-restore-manifest-2026-06-03.txt`.

## Operator decisions (2026-06-03)
1. **Drop** the 4 unused vasic-digital forks (`doc_processor`, `llm_orchestrator`, `llm_provider`, `vision_engine`) — build consumes the HelixDevelopment forks. Also drop the duplicate **HD `models`** mount (true dup of vd/models). Upstream repos remain intact; only the submodule mounts are removed.
2. **Keep `constitution` at `./constitution/`** — exempt from flattening (governance root reached by path-relative `@import` + name-hardcoded resolver). No constitution reference rewrite needed.

## Target layout
Every own-org submodule EXCEPT `constitution` (and except third-party, which stay nested) lives at `submodules/<snake_case_leaf>`. Inner Go app `helix_code/` stays put (it is a tracked subdir, NOT a submodule).

### DROP (5 mounts — `git submodule deinit -f` + `git rm` + delete `.git/modules/<path>` + remove `.gitmodules` stanza)
- `dependencies/HelixDevelopment/models`   (dup of vd/models)
- `dependencies/vasic-digital/doc_processor`
- `dependencies/vasic-digital/llm_orchestrator`
- `dependencies/vasic-digital/llm_provider`
- `dependencies/vasic-digital/vision_engine`

### MOVE → `submodules/<leaf>` (65 total)
- **Top-level (8):** challenges, containers, security, panoptic, github_pages_website, helix_agent, helix_qa, mcp_servers
- **HelixDevelopment kept (9):** doc_processor, llm_orchestrator, llm_provider, vision_engine, llms_verifier, helix_memory, helix_specifier, helix_llm, debate_orchestrator
- **vasic-digital kept (48):** models + the 47 others (agentic, auth, auto_temp, background_tasks, benchmark, cache, claritas, concurrency, config, conversation, database, document, embeddings, event_bus, filesystem, formatters, gandalf_solutions, hyper_tune, i18n, i_llm, lazy, leak_hub, llm_ops, mcp_module, memory, messaging, middleware, normalize, observability, optimization, ouroborous, planning, plinius_common, plugins, rag, rate_limiter, recovery, red_team, self_improve, skill_registry, storage, streaming, tool_schema, toon, vector_db, veritas, watcher)

Canonical winners for the 5 collision leaves: `submodules/models` ← vd; `submodules/{doc_processor,llm_orchestrator,llm_provider,vision_engine}` ← HD.

## go.mod replace rewrites (62 lines / build-critical) — rule by consumer location
Relative `replace` targets recompute by where the CONSUMING go.mod sits after the move:
- **`helix_code/go.mod`** (stays at `helix_code/`, depth-1): `../dependencies/{org}/X` → `../submodules/X`. (6 lines: docprocessor, llmorchestrator, visionengine, debate→debate_orchestrator, helixspecifier, lazy.)
- **`helix_agent/go.mod`** (moves to `submodules/helix_agent/`): siblings under `submodules/` → `../dependencies/{org}/X` → `../X`; `../dependencies/HelixDevelopment/llms_verifier/llm-verifier` → `../llms_verifier/llm-verifier`. (37 lines.)
- **`helix_qa/go.mod`** (moves to `submodules/helix_qa/`): its existing sibling-form `../X` block already resolves correctly once both it and X are siblings under `submodules/`. Normalize the dual capital/lowercase blocks to ONE lowercase block; fix the `LLMsVerifier`→`llms_verifier` casing mismatch (line 130). Net: `../Challenges`→`../challenges`, `../DocProcessor`→`../doc_processor`, etc.; `../dependencies/HelixDevelopment/LLMsVerifier/llm-verifier`→`../llms_verifier/llm-verifier`.
- **`dependencies/HelixDevelopment/llms_verifier/go.mod`** (moves to `submodules/llms_verifier/`): `../challenges` → `../challenges` (still sibling — OK; verify).
- **Archived copies under `docs/improvements/**/helixcode_sources/go.mod`** — not part of active build; leave (or update for tidiness, L-risk).

## Other reference rewrites
- **`.gitmodules`**: 70 own-org `path=`/`url` stanzas → `git mv` updates them; the 5 drops removed; constitution stanza UNCHANGED.
- **`.codegraph/config.json`**: own-org paths now under `submodules/` must stay in-index per §11.4.79 (they're in-scope by absence of exclusion; verify globs still cover `submodules/`). Re-index after.
- **Shell scripts (541 files / 1217 lines)** + **configs (122/313)** referencing `dependencies/{org}/X` or top-level own-org dirs → rewrite to `submodules/X`. Highest-volume but mechanical; do via scripted sed with an explicit old→new map, then grep-verify zero stale refs.
- **Markdown (1144/12296)** → rewrite for reference integrity (CONST-052 treats post-rename drift as a violation); regenerate `.html`/`.pdf` siblings if exporter present.

## Execution order (incremental, build-verified BEFORE commit)
1. `mkdir submodules/`. Relocate the 65 via `git mv <old> submodules/<leaf>` (git updates `.gitmodules` + `.git/config`). Drop the 5 mounts.
2. Rewrite the 62 go.mod replaces per rules above; fix helix_qa casing/dup-block.
3. Rewrite scripts/configs/.codegraph via scripted old→new map; grep-verify no stale `dependencies/vasic-digital|dependencies/HelixDevelopment` own-org refs remain (excluding third-party + archived).
4. Rewrite markdown refs; regenerate exports.
5. **VERIFY (gate before any commit):** `cd helix_code && go build ./... && go vet ./...`; same for helix_agent, helix_qa where buildable. Reference-integrity grep = 0 stale. Only on green → commit.
6. Commit meta-repo (path moves + .gitmodules + .codegraph + scripts/docs) + each changed submodule (helix_agent/helix_qa go.mod) → push all to all upstreams.
7. CONTINUATION close-out; update `docs/improvements/submodule_owned.txt` paths; HXC-001 (CONST-052 rename) closure.

## Rollback
Restore manifest pins every pre-move SHA + remote. A broken build at step 5 → `git restore`/`git checkout -- .` + `git submodule update` before any commit; nothing is pushed until build is green.
