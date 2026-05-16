// Package kilocode provides AST-aware multi-file refactoring for HelixCode.
// Cross-file rename via tree-sitter queries + atomic F17 smart-edits,
// impact analysis with call graph + blast radius, and refactoring
// utilities (extract method, move symbol, inline call).
// Reuses repomap/tree-sitter for AST parsing and F17 for atomic edits.
//
// Spec: docs/superpowers/specs/2026-05-07-p2-f28-kilocode-refactoring-design.md
// Plan: docs/superpowers/plans/2026-05-07-p2-f28-kilocode-refactoring.md
package kilocode
