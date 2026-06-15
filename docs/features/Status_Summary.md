# HelixCode Feature Status — Summary

| | |
|---|---|
| Revision | 1 |
| Created | 2026-06-15 |
| Last modified | 2026-06-15 |
| Status | active (population round 1 complete) |
| Status detail | docs/features/Status.md |

One-screen rollup of the full feature inventory in `docs/features/Status.md`. Kept
in sync via `docs_chain` (§11.4.106). Anti-bluff: **video-confirmed = 0** until real
analyzed recordings exist in `/Volumes/T7/Downloads/Recordings` (§11.4.83/§11.4.107).

## Table of contents

- [Coverage by area](#coverage-by-area)
- [Rollup by overall status](#rollup-by-overall-status)
- [Video-confirmation progress](#video-confirmation-progress)
- [Honest coverage gaps](#honest-coverage-gaps)

## Coverage by area

Round-1 inventory catalogued **~388 discrete features** across the whole system:

| Area | Source | Features | Notes |
|---|---|---:|---|
| Internal services + infrastructure | `helix_code/internal/*` (72 pkgs) | 233 | domain code: llm, tools, auth, task, session, server, memory, … |
| cmd tools + client apps | `cmd/*` (21) + `applications/*` + web | 67 | CLI (14 REPL cmds + 7 cobra), TUI (7), web (4 + ~18 API groups), desktop (6), mobile (8) |
| Owned-submodule capabilities | `submodules/*` (50 owned) | 55 | only 3 imported directly by HelixCode; ~10 by path; ~25 transitive-via-helix_agent; ~25 not in build graph |
| Ported cli_agents capabilities | `cli_agents/*` → HelixCode | 33 | 20 landed, 3 partial, 10 planned-not-landed |

## Rollup by overall status

| Overall | Meaning | Approx. count |
|---|---|---:|
| `confirmed` | working + real analyzed video | **0** (recordings not yet produced) |
| `working-untaped` | implemented + wired + unit-tested, no video yet | ~290 |
| `partial` | real but thin/unverified coverage or partial wiring | ~190 |
| `gap` | scaffold / untested / not-wired / planned-not-landed | ~52 |

## Video-confirmation progress

The video-QA program (real strong-LLM scenarios → `/Volumes/T7/Downloads/Recordings`,
analyzed, problems fixed) is the path from `working-untaped` → `confirmed`. Verified
foundation: **DeepSeek V3** (strongest) + Mistral Large + Groq Llama-3.3-70B ensemble,
all real through the helix stack. Web LLM trio (`/generate`, `/stream`, `/specify`)
already has real e2e evidence (`docs/qa/web-llm-e2e-20260615/`). Recordings-confirmed
count rises only as real analyzed videos land — never bluffed.

## Honest coverage gaps

- **Mobile apps are scaffolds** (no `build.gradle`/`AndroidManifest.xml` for Android,
  no Xcode project for iOS) — must be made buildable before any device/simulator
  recording. Android Genymotion emulator is live (adb); iOS-simulator launch
  mechanism (via the `containers` submodule, per operator) is a pending build task.
- **Deeper inventory recommended** for: `helix_agent` (1583 test files — only umbrella
  capabilities captured), `security` submodule (per-package capabilities not yet
  enumerated), `helix_specifier`, `helix_qa`, `panoptic`; and internal `clientcore`,
  `agentbridge`, `checkpoint`, `ensembleui`, `substrate`, `workspace`, `voice`,
  `roocode`, `telemetry`, `verifier`, `worker`, `server`.
- `cmd/security_scan` has zero tests (bluff-risk `gap`).
- ~60 HTTP CRUD/auth/workflow endpoints are tested at the manager layer, not
  HTTP-transport — honestly `integ`/`none`, not `e2e`.
