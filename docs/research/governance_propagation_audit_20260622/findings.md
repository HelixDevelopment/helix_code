# Governance Anchor Propagation Audit — Findings

| Field | Value |
|-------|-------|
| Date | 2026-06-22 |
| Scope | All owned `vasic-digital`/`HelixDevelopment` carriers + `constitution/` + `helix_code/` + `assets/` |
| Method | Read-only `grep -l` literal-anchor presence test (§11.4.6/§11.4.123) |
| Constitution ceiling | `11.4.165` (root `CLAUDE.md` in sync) |

## Headline

The **entire owned-submodule fleet is frozen at `11.4.141`** — the
`11.4.142`→`11.4.165` band (24 anchors, incl. §11.4.162 OpenDesign, §11.4.157
GEMINI-lockstep, §11.4.163/164/165) **never propagated**. CodeGraph anchors
78/79/80 ARE present fleet-wide, so the last successful cascade predates 141.

## Worst-lagging carrier sets (ceiling = 165)

| Carrier | Highest anchor | Lag |
|---------|----------------|-----|
| `submodules/claude-toolkit` | none (0 anchors; only CLAUDE.md exists) | **165** |
| `submodules/pipeline_runtime` | 11.4.5 | 160 |
| `submodules/dag_orchestrator` | 11.4.28 | 137 |
| `helix_code/` (inner app) | 11.4.74 | 91 |
| `submodules/doc_processor` | 11.4.97 | 68 |
| `assets/` | 11.4.102 | 63 |
| **62 other owned submodules** | 11.4.141 | **24** |
| `docs_chain` | 11.4.141 (also missing 11.4.80) | 24 + |

## §11.4.162 (OpenDesign) gaps — CONFIRMED

Missing from: `assets/CLAUDE.md`, `assets/AGENTS.md`, `assets/CONSTITUTION.md`,
`submodules/helix_agent/CLAUDE.md`, `submodules/helix_agent/AGENTS.md`.
Fleet-wide, `11.4.162` is absent from every owned submodule's CLAUDE.md except
`constitution/`.

## §11.4.157 (GEMINI.md lockstep) — systemic violation

- GEMINI.md exists in **only 1 of 69** carrier sets (`constitution/` alone). All
  68 submodules with a CLAUDE.md lack their GEMINI.md sibling.
- Even `constitution/GEMINI.md` (otherwise at 165) is **missing the literal
  `11.4.80`** while its CLAUDE.md carries it 6× → internal lockstep gap failing
  `CM-COVENANT-114-80-PROPAGATION` on GEMINI specifically.
- QWEN.md is also absent from ~48 submodules.

## Structural finding

`assets/` is listed as a wired submodule in root `CLAUDE.md` §3.2 but is NOT in
the current `.gitmodules` — a wiring gap to confirm with the operator.

## Prioritized back-fill plan

- **P0 (quick):** add `11.4.80` to `constitution/GEMINI.md`.
- **P1:** cascade `11.4.142`→`11.4.165` across all 62+ submodules' CLAUDE/AGENTS/QWEN
  via `scripts/propagate-governance.sh` + `scripts/verify-governance-cascade.sh`
  (covers 162/157/163/164/165); plus `11.4.80` for `docs_chain`.
- **P2:** create GEMINI.md (and missing QWEN.md) fleet-wide per §11.4.157.
- **P3:** full cascade-from-scratch for `claude-toolkit`, `pipeline_runtime`,
  `dag_orchestrator`, `doc_processor`, `assets`, `helix_code`; explicitly add
  `11.4.162` to `assets/` + `helix_agent/` carriers.
- **Structural:** confirm whether `assets/` should be `.gitmodules`-wired.

## Honest boundary (§11.4.6)

This audit proves anchor PRESENCE/ABSENCE, not that the cascaded text is
byte-correct — the P1/P2/P3 back-fill must run `verify-governance-cascade.sh`
afterward to confirm each anchor landed correctly fleet-wide.
