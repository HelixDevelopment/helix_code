# Phase 2 / Feature 28 — Kilo-code AST-Aware Refactoring

**Date:** 2026-05-07 | **Status:** Approved
**Programme:** CLI-Agent Fusion — Phase 2 port (kilo-code)

## 1. Goal

Ship AST-aware multi-file refactoring: cross-file rename, semantic multi-edit blocks, codebase impact analysis with call-graph + blast radius, and refactoring utilities (extract method, move, inline). Reuses F17 smart-edit for atomic file edits, repomap/tree-sitter for AST parsing and call graph construction.

## 2. Architecture

`internal/kilocode/` (NEW) — 3 sub-components:
- **RenameEngine**: tree-sitter query all references → atomic F17 edits → verify
- **ImpactAnalyzer**: call graph from tree-sitter → affected files/functions → blast radius
- **Refactorer**: extract method, move symbol, inline call (AST transforms)

Tools: kilocode_rename, kilocode_impact, kilocode_multi_edit. Slash: /kilocode.

## 3. Task Breakdown (8 tasks)

| # | Task | Description |
|---|------|------------|
| T01 | Bootstrap | F28 evidence + PROGRESS + CONTINUATION |
| T02 | types.go + callgraph.go | CallGraph + SymbolRef + sentinels (TDD) |
| T03 | rename.go | Tree-sitter query + atomic F17 edits (TDD) |
| T04 | impact.go | ImpactAnalyzer with call graph + blast radius (TDD) |
| T05 | refactor.go | Extract method, move, inline (TDD) |
| T06 | kilocode_tools.go | 3 tools + /kilocode slash (TDD) |
| T07 | main.go wiring | Register tools + slash |
| T08 | Challenge harness + close-out | 5 phases + push 4 remotes |

**Zero new deps** — tree-sitter already indirect, F17/F25/repomap already shipped.
