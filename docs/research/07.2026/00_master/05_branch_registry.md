# Canonical Branch-Name Registry (§11.4.181) — HelixLLM Full-Extension Programme

| | |
|---|---|
| **Purpose** | Single source of truth for the ONE canonical feature-branch name (§11.4.181). Every repo touched by this programme uses this EXACT name — never a per-repo variant. Look up here, never re-invent (§11.4.6). |
| **Revision** | 1 · **Created** 2026-07-06 · Track `(T1/main)` |

## The one canonical name

| Axis | Value |
|------|-------|
| **Feature slug** (§11.4.29 lowercase kebab) | `helixllm-full-extension` |
| **Branch name** (§11.4.167-D / §11.4.181) — identical on EVERY touched repo | `feature/helixllm-full-extension` |
| **Release tag axis** (§11.4.151 prefix = `HELIX_RELEASE_PREFIX` else `helix_code`) | `<prefix>-<base>-feat-helixllm-full-extension` |
| **Logic group** | `helixllm-full-extension` (Phase-R/P complete; Phase 0→9 pending) |

## Repos in scope (each uses `feature/helixllm-full-extension` when branched)

- **main meta-repo** — `helix_code` (this repo)
- **submodules/helix_llm** — the primary extension target
- **submodules/helix_agent** — provider gateway + adapters + A2A/MCP
- **submodules/llms_verifier** — capability probes + local verification (+ the vendored `submodules/LLMsVerifier` inside claude_toolkit)
- **submodules/containers** — GPU/compose primitives (upstream PR for the GPU-field gap)
- **submodules/helix_qa** — new capability test banks
- **SIBLING repo** `../claude_toolkit` — HelixAgent provider recognition (separate repo, **same** canonical branch name)

## Rules (§11.4.181)

1. **One name, everywhere.** Any submodule branched for this programme MUST use `feature/helixllm-full-extension` — no per-repo drift. An untouched submodule stays on its base branch (not force-branched).
2. **Look up, never re-invent.** Every branch creation reads this registry; a second stream working this group reads the same name.
3. **Risk-free reconciliation.** A mis-named/duplicate branch is reconciled by merge-onto-canonical preserving every commit (§11.4.113 / §9.2), never delete-before-merge, never force-push.

## Physical branch creation — DEFERRED (with reason, §11.4.6)

Branches are NOT yet created. This meta-repo runs an **auto-commit daemon** that commits to `main`
(observed: `Auto-commit` commits advancing HEAD + submodule pointers). Before creating
`feature/helixllm-full-extension` in any repo, INVESTIGATE how that daemon selects its target branch,
so a feature branch is not silently committed-over or reset by the daemon (a §11.4.6 unknown — do not
guess). Until then:
- Research/planning docs (this workspace) remain on `main` (additive documentation, correctly daemon-committed).
- The canonical name is REGISTERED (this file) satisfying §11.4.181's single-source-of-truth requirement.
- Physical branch creation happens per-repo at first CODE modification, after the daemon-interaction
  investigation (tracked as a Phase-0 prerequisite).
