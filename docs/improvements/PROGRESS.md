# HelixCode CLI-Agent Fusion — Live Progress Tracker

> **STOP/RESUME PROTOCOL**: read this file first. The "current focus" pointer
> below identifies the active task. The "evidence trail" links every claim of
> "done" to its commit + Challenge output.
>
> Spec: `docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md`
> Plan: `docs/superpowers/plans/2026-05-04-phase-0-foundation-cleanup.md`

## Current focus
- **Active phase:** P0 — Foundation Cleanup
- **Active task:** P0-11 — add Article XII (CONST-042 + CONST-043) to root CONSTITUTION.md
- **Last completed:** P0-10 — create HelixCode/{CLAUDE,AGENTS,CONSTITUTION}.md (inner Go-app governance triplet)
- **Owner:** agent (Claude Opus 4.7)
- **Started:** 2026-05-04
- **Last touched:** 2026-05-04 (P0-08)
- **Blocked-on:** none

## Phase status
| Phase | State | Started | Completed | Evidence |
|---|---|---|---|---|
| P0 — Foundation | active | 2026-05-04 | — | docs/improvements/05_phase_0_evidence.md |
| P1 — claude-code | pending | — | — | — |
| P2 — Other CLI agents | pending | — | — | — |
| P3 — Test infra | pending | — | — | — |
| P4 — Anti-bluff audit | pending | — | — | — |
| P5 — End-user materials | pending | — | — | — |

## Active phase task list (Phase 0)
- [x] P0-01 — bootstrap PROGRESS.md  ← commit `2c07f57`
- [~] P0-02 — Agent-Deck nested-worktree recursion error: **DEFERRED** (cosmetic; safe-fix requires modifying third-party submodules which is out of scope per spec §2.1; original `.git/info/exclude` approach was based on incorrect understanding of git submodule recursion semantics; see parking lot). Reverts: commits `904c925` + `a82f1a9`.
- [x] P0-03 — add HelixAgent submodule  ← (this commit) — 47/60 cli_agents populated; 13 deferred to Phase 2 sub-specs (see parking lot)
- [x] P0-04 — verify-llmsverifier-pin-parity.sh  ← (this commit)
- [x] P0-05 — migrate API keys from ../HelixAgent/.env  ← (this commit)
- [x] P0-06 — update .gitignore (root + inner)  ← (this commit)
- [x] P0-07 — refresh HelixCode/HelixCode/.env.example  ← (this commit)
- [x] P0-08 — scan-secrets.sh + planted-secret test  ← (this commit)
- [x] P0-08.5 — remediate 3 pre-existing tracked credentials  ← commits `8d30add` + `15cca9b` + `45bd4d4`
- [x] P0-08.7 — Port SonarQube + Snyk security scan integration through Containers ← commits `1d728de` + `2494bc8` + `e29e2f6` + `16a4490` + sub5; Challenges: 33/33 + 26/26 PASS; Go BootManager wiring landed (go build exits 0); live scans deferred pending credential rotation (see evidence §P0-08.7)
- [x] P0-08.7-fix — T08.7 code-quality review findings (Critical 1+2, Important 3-7) ← commits `b21b051`; evidence: §P0-T08.7 (fix-it) in 05_phase_0_evidence.md
- [x] P0-09 — pre-push hook + installer + setup.sh wiring ← (this commit)
- [x] P0-10 — create HelixCode/{CLAUDE,AGENTS,CONSTITUTION}.md (inner Go-app governance triplet) ← (this commit)
- [ ] P0-11 — add Article XII (CONST-042, CONST-043) to root CONSTITUTION.md
- [ ] P0-12 — cascade CONST-042/042 + anti-bluff anchor to root sister files (CLAUDE, AGENTS, CRUSH, QWEN)
- [ ] P0-13 — fix root CLAUDE.md §3.2 bluff (HelixCode tracked-dir vs. submodule)
- [ ] P0-14 — extend verify-governance-cascade.sh + run propagate-governance.sh + verify cascade
- [ ] P0-15 — Makefile verify-foundation target + extend ci-validate-all
- [ ] P0-16 — regenerate diagrams + DEPRECATED.md pointers + Phase 0 evidence + push close-out

## Decision log
- 2026-05-04 — Approach A (HelixAgent as integration substrate) — user-approved during brainstorming — see synthesis spec §2.1
- 2026-05-04 — Non-force pushes pre-authorised for the duration of this programme — user statement during brainstorming — see synthesis spec §7.3
- 2026-05-04 — claude-code-source is Phase 1 priority #1 — user statement — see synthesis spec §4.1

## Open risks / parking lot
- **Historical SSH key leak (remediated in P0-T08.5):** `id_rsa` + `id_rsa.pub` at `HelixCode/test/workers/ssh-keys/` were committed as test fixtures before this programme. Their material lives in git history forever and is considered compromised. Mitigation: keys were ephemerally test-only (no production trust), replaced with auto-generated ed25519 ephemeral keys via `HelixCode/test/workers/ssh-keys/generate-test-keys.sh`, removed from the index via `git rm --cached`. Any future production system that erroneously trusts the leaked public key must reject it.
- **Historical helix.security.json credential leak (remediated in P0-T08.5):** `helix.security.json` at repo root was committed with real SonarQube and Snyk credentials (token, project_key, organization, url). Material lives in git history and is considered compromised. Mitigation: removed from index via `git rm --cached`; replaced with `helix.security.json.example` containing `<REDACTED>` placeholders. Rotate: SonarQube token, Snyk token, organization, and project_key immediately.
- HelixAgent submodule clone size — may need `--depth=1` shallow if >500 MB; measured at P0-03
- Codex agent disambiguation (closed vs. open variant) — deferred to Phase 2 sub-spec
- Example_Projects/ deletion — deferred to post-Phase-4 decision
- **Submodule recursion cosmetic error (deferred from P0-02):** `git submodule foreach --recursive` errors out on `Example_Projects/{Agent-Deck,Bridle,Claude-Code-Plugins-And-Skills}` because each of those third-party repos has registered nested gitlinks (mode 160000) without corresponding `.gitmodules` entries. The original Task 2 plan proposed `.git/info/exclude` — that does NOT fix recursion (which walks the index, not the working tree). The only safe in-scope fix is to wrap script calls with `|| true` and tolerate the error. Modifying the affected third-party submodules' contents is forbidden by spec §2.1 (third-party not modified). Decision: scripts that use `git submodule foreach --recursive` (none yet in our codebase) must wrap with `|| true`; documentation updates that erroneously claimed Task 2 would resolve this are corrected.
- **HelixAgent stale cli_agents pins (discovered during P0-03):** 13 of 60 cli_agents under `HelixAgent/cli_agents/` cannot be initialized because HelixAgent's recorded submodule SHAs no longer exist on the corresponding upstream remotes. Affected: `aider, conduit, continue, HelixCode, kilo-code, kiro-cli, mobile-agent, ollama-code, opencode-cli, openhands, plandex, roo-code, superset`. Each Phase 2 sub-spec for the affected agent must first bump HelixAgent's pointer (commit IN HelixAgent itself, then bump HelixAgent's pointer in this meta-repo) to a SHA that exists upstream. Phase 1 priority `claude-code` is NOT affected — fully populated. Per spec §1.3 N2, HelixAgent rewrite is out of scope for this programme; the per-agent pin bumps go through HelixAgent's own governance.
- **SonarQube + Snyk live-scan deferral (P0-T08.7):** The scan infrastructure (compose files, scripts, BootManager binary, Challenges) is fully wired and configuration-validated. Live scans CANNOT run until the user rotates the leaked credentials from `helix.security.json` (remediated in P0-T08.5 but historical values are compromised). Action required: (1) generate new SonarQube API token, (2) set `SONAR_TOKEN` + `SONARQUBE_PROJECT_KEY` + `SONARQUBE_PROJECT_NAME` in `HelixCode/.env`, (3) generate new Snyk token, (4) set `SNYK_TOKEN` in `HelixCode/.env`, (5) run `make scan-sonarqube` / `make scan-snyk`. This is NOT a code defect — it is a security-rotation dependency on the operator.
- **LLMsVerifier dual-pin divergence (discovered during P0-04):** `Dependencies/HelixDevelopment/LLMsVerifier` at `629c5bd5d141351270e72b6fb7359fa4b7881d7c`; `HelixAgent/LLMsVerifier` at `1d53ae3b72c77c1f27171c0677431c48d2d02bdd`. Per spec §2.2 the canonical pin is the one in `Dependencies/HelixDevelopment/LLMsVerifier` (direct Go import path). The canonical is exactly one commit ahead of the transitive (HelixAgent) view. Resolving the divergence requires either (a) bumping HelixAgent's recorded LLMsVerifier pointer to the canonical SHA — out of scope per spec §1.3 N2 (HelixAgent rewrite forbidden), or (b) updating `Dependencies/HelixDevelopment/LLMsVerifier` to match HelixAgent's view if HelixAgent's view is more current. Decision deferred; the parity verifier (`scripts/verify-llmsverifier-pin-parity.sh`) will continue to gate any future change that introduces NEW divergence beyond this baseline.
