// Package planner provides a task planner and step executor that bridges
// F25 plan trees with Openhands-style sequential execution pipelines.
// TaskSteps (shell commands or LLM prompts) are executed sequentially
// with retry, and status is propagated back to the PlanTree.
//
// Spec: docs/superpowers/specs/2026-05-07-p2-f26-openhands-workspace-design.md
// Plan: docs/superpowers/plans/2026-05-07-p2-f26-openhands-workspace.md
package planner
