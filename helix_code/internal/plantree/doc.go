// Package plantree provides a plan tree system for the HelixCode CLI agent.
//
// Plan trees are structured, persistent implementation plans modelled on
// Plandex's branching plan architecture. Each plan is a tree of PlanNode
// values (title, description, status, children) serialized to JSON files
// at .helixcode/plans/<name>.json. Agents construct, branch, merge, prune,
// and verify plans through six tools.Tool implementations. Context
// compaction reuses F01's AutoCompactor infrastructure.
//
// Spec: docs/superpowers/specs/2026-05-07-p2-f25-plandex-plan-trees-design.md
// Plan: docs/superpowers/plans/2026-05-07-p2-f25-plandex-plan-trees.md
package plantree
