# P2-F28 — Kilo-code AST-Aware Refactoring Plan

**Goal:** Ship cross-file rename, impact analysis, and refactoring suite. New `internal/kilocode/` package. Reuses F17, repomap, F25.

**Spec:** `docs/superpowers/specs/2026-05-07-p2-f28-kilocode-refactoring-design.md`
**Q1-Q5:** Kilo-code / Core+refactoring / Tree-sitter+atomic edits / Call graph+blast radius / 3 tools+slash

---

## Task list

- [ ] P2-F28-T01 — bootstrap F28 evidence + advance PROGRESS
- [ ] P2-F28-T02 — `internal/kilocode/types.go` + `callgraph.go`: CallGraph, SymbolRef, BuildCallGraph (TDD)
- [ ] P2-F28-T03 — `internal/kilocode/rename.go`: rename engine (tree-sitter query + atomic F17 edits) (TDD)
- [ ] P2-F28-T04 — `internal/kilocode/impact.go`: ImpactAnalyzer (call graph + blast radius) (TDD)
- [ ] P2-F28-T05 — `internal/kilocode/refactor.go`: extract method, move, inline (TDD)
- [ ] P2-F28-T06 — `internal/kilocode/kilocode_tools.go`: 3 tools + /kilocode slash (TDD)
- [ ] P2-F28-T07 — main.go wiring
- [ ] P2-F28-T08 — Challenge harness 5 phases + close-out + push 4 remotes

---

## T02: types.go + callgraph.go

- CallGraph: nodes (functions, methods, classes), edges (calls, references)
- BuildCallGraph via tree-sitter: for each file, parse AST, find function declarations and call sites
- Tests with real tempdirs containing multi-file Go projects

## T03: rename.go

- Query tree-sitter for all occurrences of a symbol across the codebase
- Apply renames via F17 smart-edit (search/replace blocks)
- Validate post-rename: no broken references

## T04: impact.go

- Given a symbol, traverse call graph to find all callers/callees
- Report blast radius: files affected, risk score
- Tests with connected Go files

## T05: refactor.go

- Extract method: move a code block into a new function
- Move symbol: relocate function between files with import updates
- Inline: replace function call with its body

## T06: kilocode_tools.go

- kilocode_rename (category=kilocode, LevelEdit)
- kilocode_impact (category=kilocode, LevelReadOnly)
- kilocode_multi_edit (category=kilocode, LevelEdit)
- /kilocode slash (rename/impact/edit subcommands)

## T07: main.go wiring

Register 3 tools + /kilocode slash in main.go startup.

## T08: Challenge harness

5 phases: rename verification, impact analysis, multi-edit atomicity, extract method, inline refactoring.

---

*Execute via TDD starting with T01.*
