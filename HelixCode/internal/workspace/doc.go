// Package workspace provides container-based per-task workspace management
// for the HelixCode CLI agent. Workspaces are Docker/Podman containers with
// mounted project directories, isolated networking, and auto-cleanup TTL.
//
// Spec: docs/superpowers/specs/2026-05-07-p2-f26-openhands-workspace-design.md
// Plan: docs/superpowers/plans/2026-05-07-p2-f26-openhands-workspace.md
package workspace
