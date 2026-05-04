// Package permissions implements the claude-code-style permission rule system.
//
// It is a thin layer that loads YAML rule files (user + project), composes them
// with one of five built-in mode presets (default, auto, acceptEdits, dontAsk,
// bypassPermissions), and produces a confirmation.Policy that registers with
// the existing confirmation.PolicyEngine. Compound Bash commands are split via
// mvdan.cc/sh/v3/syntax so command substitutions, backticks, heredocs, and
// pipelines all aggregate to most-restrictive (deny > ask > allow).
//
// See: docs/superpowers/specs/2026-05-05-p1-f02-permission-rules-design.md
package permissions
