# Phase 1.5 — Submodule Deduplication: Canonical Paths

**Captured:** 2026-05-06
**Source:** Plan `docs/superpowers/plans/2026-05-06-p1-5-foundation-cleanup.md` §WP1.T01.04
**Purpose:** WP3 reads this file to know which copy stays and which copies are removed.

---

## The 5 dedup sets

| # | Submodule | Canonical path | Removed paths | Consumers to update |
|---|---|---|---|---|
| 1 | LLMsVerifier | `Dependencies/HelixDevelopment/LLMsVerifier/` (root) | `HelixAgent/LLMsVerifier/` | `HelixAgent/Makefile`, `HelixAgent/scripts/*`, any `internal/...` refs to old path |
| 2 | Containers   | `Containers/` (root) | `Challenges/Containers/`, `HelixAgent/Containers/`, `HelixAgent/HelixLLM/submodules/Containers/` | Each removed parent's Makefile + nested verifier scripts (per-WP2 audit) |
| 3 | Security     | `Security/` (root) | `HelixAgent/Security/`, `HelixAgent/HelixLLM/submodules/Security/` (if present) | HelixAgent/scripts/security-*.sh, root cmd/security-test wiring |
| 4 | HelixQA      | `HelixQA/` (root) | `HelixAgent/HelixQA/` | `HelixAgent/Makefile` test wiring; root `scripts/run-all-tests.sh` |
| 5 | MCP-Servers  | TBD at WP3.T03.05 — current candidates: `MCP-Servers/` (root, may not exist), `HelixAgent/MCP-Servers/`, `HelixAgent/MCP/submodules/...` (per-server) | TBD per resolution | TBD per resolution |

**Note on set #5 (MCP-Servers):** The HelixAgent submodule tree has both
`MCP-Servers/` (potentially a single aggregator) and `MCP/submodules/<NAME>/`
(per-server child entries: airtable-mcp, all-in-one-mcp, atlassian-mcp, etc.).
Whether these are duplicates or whether `MCP-Servers/` is *the* aggregator that
links the per-server entries is not resolvable from `.gitmodules` alone — the
plan defers the canonical decision to WP3.T03.05 with audit-at-execution.

## Dependency edges (read these BEFORE WP3)

- LLMsVerifier dedup blocks WP3 until `make verify-llmsverifier-pin-parity` is
  reproduced from the new canonical path.
- Containers dedup affects all three Helix* submodules; per-removal rebuild
  required (T03.0X completion gate).
- HelixQA dedup affects HelixAgent's test wiring; integration test must still
  pass post-dedup.
- MCP-Servers TBD blocks any consumer code that imports MCP servers (CONST-040
  capability flags) until canonical chosen.

## Rollback per dedup

For each canonical decision, the rollback is:

```bash
cd <parent-of-removed-path>
git submodule add -f <url-from-captured-snapshot> <removed-path>
git submodule update --init --recursive <removed-path>
```

URLs are captured in `docs/improvements/p1-5-snapshot-pre.md` and the
verbatim `.gitmodules` content snapshots referenced from there.

## What this list does NOT decide

- Per-removal commit ordering (left to WP3 task graph).
- Whether to also dedup any `MCP/submodules/<NAME>` entries where a root-level
  copy now exists (audit at WP3.T03.05).
- Whether `Toolkit/Toolkit/Chutes` (a dup-path-typo seen in HelixAgent
  `.gitmodules`) is fixed in WP3 or in WP2 restructuring — owned by WP2.
