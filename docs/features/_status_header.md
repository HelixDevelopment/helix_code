# HelixCode Feature Status

| | |
|---|---|
| Revision | 1 |
| Created | 2026-06-15 |
| Last modified | 2026-06-15 |
| Status | active (population in progress) |
| Status summary | docs/features/Status_Summary.md |
| Continuation | docs/CONTINUATION.md |

Authoritative, in-depth inventory of **every** HelixCode feature across all
services, infrastructure, and client applications — including capabilities ported
from the `cli_agents/` reference catalogue — with per-feature status across every
dimension the operator mandate (2026-06-15) requires. Kept in sync via the
`docs_chain` engine (§11.4.106, `submodules/docs_chain`) and the Status-doc
covenant (§11.4.45 / §11.4.53 / §11.4.56 / §11.4.57 / CONST-063 / CONST-064).

> **Anti-bluff (CONST-035 / §11.4.83 / §11.4.107):** a feature is marked
> video-confirmed (`📹 yes`) ONLY when a real recorded scenario in
> `/Volumes/T7/Downloads/Recordings` shows it working end-to-end with a strong
> real LLM and that recording has been analyzed. No false "yes". An un-recorded
> or un-analyzed feature is honestly `📹 no` / `📹 pending`, never bluffed green.

## Table of contents

- [Status dimensions (legend)](#status-dimensions-legend)
- [Population progress](#population-progress)
- [Feature inventory](#feature-inventory)
- [Inventory sources](#inventory-sources)

## Status dimensions (legend)

Each feature row carries:

| Column | Meaning | Values |
|---|---|---|
| **Area** | service / infrastructure / application(client) / submodule | — |
| **Component** | package / tool / app / submodule it lives in | — |
| **Feature** | the discrete user-or-system capability | — |
| **Dev** | implementation status | `done` / `partial` / `stub` / `absent` |
| **Wired** | reachable from a shipped flow (not dead code) | `yes` / `no` / `partial` |
| **Real-use** | genuinely usable by an end user | `yes` / `no` / `unknown` |
| **Tests** | automated coverage | `unit` / `integ` / `e2e` / `none` (combinable) |
| **V&V** | captured runtime evidence (§11.4.5/§11.4.69) | `yes(path)` / `no` |
| **📹 Video** | recorded real scenario + analyzed (§11.4.83/§11.4.107) | `yes(path)` / `pending` / `no` / `n/a` |
| **Analysis** | comprehensive recording analysis performed | `yes` / `no` |
| **Origin** | native / `ported:<cli_agent>` | — |
| **Overall** | rollup | `confirmed` / `working-untaped` / `partial` / `gap` |

## Population progress

This document is populated by background inventory subagents fanning out across
the codebase (§11.4.70 subagent-driven, §11.4.103 parallel-streams). Coverage is
reported honestly — `confirmed` rows require a real analyzed video; everything
else is marked truthfully. Population is an ongoing program, NOT a one-shot claim.

| Slice | Scope | Status |
|---|---|---|
| internal services + infra | `helix_code/internal/*` (72 pkgs) | inventory dispatched |
| cmd tools + client apps | `helix_code/cmd/*` (21) + `applications/*` (cli/tui/web/desktop/mobile) | inventory dispatched |
| owned submodules | `submodules/*` (70+) | inventory dispatched |
| ported cli_agents capabilities | `cli_agents/*` → HelixCode | inventory dispatched |
