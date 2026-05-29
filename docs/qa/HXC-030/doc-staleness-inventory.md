# HXC-030 — §11.4.99 Documentation Staleness Inventory (Scoping Pass)

**Item:** HXC-030 — §11.4.99 latest-source documentation cross-reference forward sweep
**Date:** 2026-05-29
**Author method:** Read-only inventory. Enumerated every operator-facing doc under
`docs/`, `README.md`, and `helix_code/README.md`; for each, extracted referenced
external services/libraries via pattern scan (`grep -oiE` over version/service tokens),
checked for an existing `## Sources verified` footer (`grep -rl "Sources verified"`),
captured git last-commit dates for the high-risk setup docs, and flagged stale version
signals (notably `Go 1.24`, which contradicts `CLAUDE.md` §3.1's mandated Go 1.26 inner /
Go 1.25.2 root).

> **SCOPE NOTE — THIS IS A SCOPING INVENTORY, NOT THE FULL §11.4.99 VERIFICATION.**
> No external official documentation was fetched in this pass. NO doc below is claimed
> verified. The actual §11.4.99 pass (separate deliverable) MUST WebFetch the latest
> official online docs for each referenced service/library, cross-reference every
> instruction step, document negative findings, and add the `## Sources verified <date>:
> <urls>` footer + commit-message footer. This document only scopes WHAT needs verifying
> and in WHAT ORDER.

**Exclusions (per task scope):** tracker/governance docs (`Issues.md`, `Fixed.md`,
`CONTINUATION.md`, `*_Summary.md`), pure-internal audit/research/evidence trees
(`docs/audits/`, `docs/qa_evidence/`, `docs/research/`, `docs/improvements/`,
`docs/superpowers/`, `docs/coverage/`, `docs/adr/`, `docs/codegraph/`, `docs/architecture/`,
`docs/exports/`), and the large internal integration-plan blobs
(`docs/llms_verifier/**`, `docs/helix_qa/**`, `docs/bluff_proofing/full_plan/**`) which
are internal planning material rather than operator-facing instructions.

## Risk-classified service categories (per §11.4.99(D), 90-day max staleness)

AI/LLM providers (OpenAI, Anthropic, Gemini, DeepSeek, Groq, Mistral, xAI, OpenRouter,
Ollama, Llama.cpp), cloud APIs (AWS Bedrock, Azure), code-hosting (GitHub/GitLab),
package managers / language toolchains (Go), and container runtimes (podman/docker) all
fall in or adjacent to risk-classified categories. PostgreSQL/Redis are infrastructure
(standard 6-month staleness).

## Inventory table

| Doc (path) | Referenced services/libs | "Sources verified" footer? | Risk-classified? | Priority | Notes |
|---|---|---|---|---|---|
| `README.md` | Go 1.24/1.25/1.26, Ollama, pgx, PostgreSQL 15/17, Redis 7 | **Y** (2026-05-29, go.dev release page + repo cross-ref) | Yes (Go, LLM, DB) | **Done** | Already §11.4.99-footered this date; verify footer URLs remain current at next sweep. |
| `helix_code/README.md` | docker, podman, Kubernetes, Ollama | N | Yes (containers, LLM) | High | Build/run instructions; container runtime guidance (Rule 4 / podman). No footer. |
| `docs/COMPLETE_DEPLOYMENT_GUIDE.md` | Azure, docker, Kubernetes | N | Yes (cloud, containers) | High | Setup/deploy doc; git last-commit **2025-12-11** (>5 mo — near 6-mo stale floor). |
| `docs/DEPLOYMENT_GUIDE.md` | Azure, docker, Kubernetes, **postgresql-15** | N | Yes (cloud, containers, DB) | High | 1782-line deploy doc; git last-commit **2025-12-11**; contains a stale `Go 1.24` ref. |
| `docs/COMPLETE_CONFIGURATION_DOCUMENTATION.md` | Azure, Ollama | N | Yes (cloud, LLM) | High | Config reference incl. provider config; version-bearing. |
| `docs/COMPLETE_SECURITY_GUIDE.md` | Azure, docker, Kubernetes | N | Yes (cloud, containers) | Med | Security/deploy guidance. |
| `docs/COMPLETE_API_REFERENCE.md` | docker, Kubernetes | N | Yes (containers) | Med | API ref; light external-version surface. |
| `docs/COMPLETE_CLI_REFERENCE.md` | docker | N | Yes (containers) | Med | CLI ref. |
| `docs/COMPLETE_EXAMPLES_TUTORIALS.md` | docker, Kubernetes | N | Yes (containers) | Med | Examples/tutorials. |
| `docs/COMPLETE_PERFORMANCE_TUNING_GUIDE.md` | docker, Kubernetes | N | Yes (containers) | Med | Perf tuning. |
| `docs/COMPLETE_TROUBLESHOOTING_GUIDE.md` | docker | N | Yes (containers) | Med | Troubleshooting. |
| `docs/troubleshooting/guide.md` | Anthropic, docker, Gemini, gin, **Go 1.24**, llama, mistral, Ollama, OpenAI | N | Yes (LLM, containers, Go) | High | 1422 lines; broad LLM-provider + stale `Go 1.24`; last-commit 2026-05-16. |
| `docs/general/DEVELOPER_GUIDE.md` | Anthropic, docker, Gemini, Ollama, OpenAI, Podman | N | Yes (LLM, containers) | Med | Dev guide incl. stale `Go 1.24`; last-commit 2026-05-16. |
| `docs/general/PROVIDER_FEATURES.md` | OpenAI, Anthropic, Azure, Bedrock, DeepSeek, Gemini, Groq, llama, Ollama | N | Yes (LLM, cloud) | High | Per-provider feature claims — high §11.4.99 + CONST-036/039 surface. |
| `docs/general/MENTIONS_USER_GUIDE.md` | (none detected) | N | No | Low | Feature usage doc; no external-version surface. |
| `docs/general/SLASH_COMMANDS_USER_GUIDE.md` | (contains a stale `Go 1.24` ref) | N | Borderline (Go) | Low | Mostly internal commands; one stray Go version. |
| `docs/general/PROVIDER_UPDATE_SUMMARY.md` | (not deep-scanned) | N | Yes (LLM) | Med | Provider summary; scan in verify pass. |
| `docs/general/FEATURE_IMPLEMENTATION_COMPLETE.md` | (not deep-scanned) | N | No | Low | Status-flavored; confirm operator-facing in verify pass. |
| `docs/user_manual/README.md` | all 7 LLM providers, Azure, Bedrock, docker, Kubernetes, postgresql-14, **Go 1.24** | N | Yes (LLM, cloud, containers, DB, Go) | High | 3240 lines; broadest external surface; `postgresql-14` + stale `Go 1.24`. |
| `docs/user_manual/INDEX.md` | all 7 LLM providers, Azure, Bedrock, docker, Kubernetes | N | Yes (LLM, cloud, containers) | Med | Index/landing for user manual. |
| `docs/user_manual/ZERO_BLUFF_USER_MANUAL.md` | all 7 LLM providers, Azure, Bedrock, Go 1.26, PostgreSQL 15, Redis 7 | N | Yes (LLM, cloud, DB, Go) | High | Flagship user manual; Go 1.26 is current — good signal but no footer. |
| `docs/user_manual/SUMMARY.md` | (not deep-scanned) | N | Borderline | Low | TOC/summary. |
| `docs/api_reference/README.md` | all 7 LLM providers, Bedrock, gin, **Go 1.25 + go 1.26** | N | Yes (LLM, cloud, Go) | Med | API ref landing; Go versions current. |
| `docs/deployment_guide/README.md` | Anthropic, docker, gin, LLama, Ollama, podman | N | Yes (LLM, containers) | High | Deploy guide landing; container runtime guidance. |
| `docs/developer_guide/README.md` | Docker, gin, Ollama | N | Yes (LLM, containers) | Med | Dev guide landing. |
| `docs/materials/LLMs_Optimization.md` | Anthropic, DeepSeek, gin, groq, llama, Ollama, OpenAI | N | Yes (LLM) | Med | LLM optimization material; provider-version sensitive. |
| `docs/materials/Encryption_Algs.md` | (none detected) | N | No | Low | Crypto-algorithm reference; conceptual, not service instructions. |
| `docs/platforms/README.md` | (none detected) | N | No | Low | 53-line platform index. |
| `docs/LLMsVerifier_User_Guide.md` | Ollama | N | Yes (LLM) | Med | LLMsVerifier user guide. |
| `docs/HOST_POWER_MANAGEMENT.md` | (none detected) | N | No | Low | CONST-033 policy doc; no external service instructions. |
| `docs/bluff_proofing/README.md` | (not deep-scanned) | N | No | Low | Bluff-proofing overview. |
| `docs/bluff_proofing/STEP_BY_STEP_GUIDE.md` | Anthropic, docker, **Go 1.24**, llama, Ollama, OpenAI, PostgreSQL 15, Redis 7 | N | Yes (LLM, containers, DB, Go) | Med | Step-by-step setup-flavored guide; stale `Go 1.24`; last-commit 2026-05-16. |
| `docs/bluff_proofing/MASTER_ACTION_PLAN.md` | (contains stale `Go 1.24`) | N | Borderline | Low | Plan doc; mostly internal. |
| `docs/troubleshooting/guide.md` | (listed above) | — | — | — | (same as troubleshooting row above) |

## Counts

- **Total operator-facing docs inventoried:** 30 (excluding the duplicate troubleshooting row).
- **Already carrying a `## Sources verified` footer:** **1** — `README.md` only.
  (Other `Sources verified` matches found by grep — `docs/Issues.md`,
  `docs/Issues_Summary.md`, and their HTML exports — are governance/tracker docs and are
  OUT OF SCOPE.)
- **Risk-classified (touch LLM/cloud/code-hosting/package-manager/container categories):**
  **22** of 30.
- **Stale-version signal (`Go 1.24`, contradicting CLAUDE.md §3.1 Go 1.26/1.25.2):** present in
  operator docs `docs/DEPLOYMENT_GUIDE.md`, `docs/troubleshooting/guide.md`,
  `docs/general/DEVELOPER_GUIDE.md`, `docs/general/SLASH_COMMANDS_USER_GUIDE.md`,
  `docs/user_manual/README.md`, `docs/user_manual/tutorials/Tutorial_1_Building_Web_App.md`,
  `docs/user_manual/tutorials/Tutorial_9_Custom_Provider.md`,
  `docs/user_manual/tutorials/Tutorial_10_Adding_a_Tool.md`,
  `docs/bluff_proofing/STEP_BY_STEP_GUIDE.md`, `docs/bluff_proofing/MASTER_ACTION_PLAN.md`.
- **Date-stale signal (git last-commit > 5 months, near the 6-month staleness floor):**
  `docs/COMPLETE_DEPLOYMENT_GUIDE.md` and `docs/DEPLOYMENT_GUIDE.md` (both 2025-12-11).

## Prioritized verification worklist (verify these first)

Priority rule (§11.4.99(D)): risk-classified + version-bearing setup/deploy docs first;
docs with both a stale signal AND a missing footer rank highest.

1. **`docs/DEPLOYMENT_GUIDE.md`** — deploy doc, Azure + docker + Kubernetes + postgresql-15,
   stale `Go 1.24`, git-stale (2025-12-11), no footer. Highest blast radius (operators run it).
2. **`docs/COMPLETE_DEPLOYMENT_GUIDE.md`** — companion deploy doc, Azure + containers,
   git-stale (2025-12-11), no footer.
3. **`docs/user_manual/README.md`** — flagship 3240-line user manual; all 7 LLM providers +
   Azure + Bedrock + containers + postgresql-14 + stale `Go 1.24`; broadest external surface.
4. **`docs/general/PROVIDER_FEATURES.md`** — per-provider feature/capability claims across all
   LLM providers + Bedrock + Azure; intersects CONST-036/037/039 (verifier-sourced metadata).
5. **`docs/troubleshooting/guide.md`** — 1422-line troubleshooting doc; broad LLM-provider +
   docker + stale `Go 1.24`; operators consult under failure conditions where bad advice is costly.

Next tier (verify after top 5): `helix_code/README.md`, `docs/deployment_guide/README.md`,
`docs/COMPLETE_CONFIGURATION_DOCUMENTATION.md`, `docs/user_manual/ZERO_BLUFF_USER_MANUAL.md`,
then the remaining COMPLETE_*, user_manual, and materials docs.

## What the actual §11.4.99 verification pass still requires (live work, NOT done here)

For each doc above (starting with the top-5 worklist) the verification pass MUST:
1. WebFetch the LATEST official online docs for each referenced service/library
   (go.dev release page; PostgreSQL/Redis release notes; Ollama docs; podman/Docker docs;
   AWS Bedrock + Azure docs; each LLM provider's official API docs; gin/pgx upstreams).
2. Cross-reference every instruction step against that source; correct the stale `Go 1.24`
   references to the mandated Go 1.26 (inner) / 1.25.2 (root) and reconcile `postgresql-14`
   vs `postgresql-15`/`17`.
3. Document negative findings (gaps/silences/contradictions) explicitly.
4. Add a `## Sources verified <date>: <urls>` footer to each doc + a matching
   `Sources verified <date>: <urls>` commit-message footer.

## Sources verified

N/A — this is a read-only scoping inventory, not an operator-facing instruction doc. No
external sources were fetched; no instruction in this file directs an operator to act on a
third-party service. The §11.4.99 footer obligation applies to the docs listed above when
they are verified, not to this inventory.
