# Phase 1.5 Pre-State Snapshot — Foundation Cleanup

**Captured:** 2026-05-06T16:29 +03:00
**Purpose:** Reversibility anchor per Article XI §11.9. Any later WP that breaks the
submodule tree must be diagnosable + rollback-able from this file.

**WP1 status at capture:** Pre-WP1 cleanup complete (HelixAgent gitlinks committed,
Challenges gitlinks committed, root tracked changes committed, Example_Projects/
fully removed, .gitignore updated). Recursive `git submodule update --init` ran
but ABORTED with three unreachable remotes (see §Reachability). T01.01 evidence
is partial (160/196 submodules accounted for; init aborted by HelixAgent
DebateOrchestrator double-failure). Halted at user STOP-protocol checkpoint.

---

## Initial commit-set (root meta-repo) — pre-WP1 state

| Step | Commit SHA | Message |
|---|---|---|
| 1a (HelixAgent) | `aad6a67d` | chore(governance-cascade): preserve in-flight nested gitlink updates pre-P1.5 |
| 1b (Challenges) | `47dc905a` | chore(P1.5): preserve nested gitlink updates pre-foundation-cleanup |
| 1c (root)       | `d0ad6fd3` | chore(P1.5): preserve in-flight tracked changes + submodule gitlink advances pre-foundation-cleanup |
| 2  (Examples)   | `ad5e108c` | feat(P1.5-pre): remove Example_Projects/ entirely |
| 3  (gitignore)  | `cff2d90f` | chore(P1.5-pre): gitignore phase-1 development artefacts |

Root HEAD at capture: `cff2d90fe550fe069eb3632b7074c412e69c6ddb` (`main`).

## Submodule counts (post-init, pre-fetch-loop)

| Scope | Initialised | Uninitialised | Total |
|---|---|---|---|
| Whole tree (`git submodule status --recursive`) | ~160 | 36 | ~196 |
| Root meta-repo `.gitmodules`  | 21 |  | 21 |
| HelixAgent `.gitmodules`      | ~134 | 36 | 170 |

Top-level uninitialised: HelixAgent/DebateOrchestrator (unreachable),
HelixAgent/HelixLLM/submodules/* (~33 entries deferred since Phase 0),
HelixAgent/cli_agents/{kiro-cli,ollama-code} (unreachable upstream).

Captured listing: `docs/improvements/p1-5-fetch.log` (init log) and
`docs/improvements/p1-5-remote-reachability.md` (per-URL ls-remote result).

## Reachability — the WP1 STOP

Comprehensive ls-remote sweep across 181 unique URLs (root + HelixAgent
combined) found **3 truly unreachable remotes** (after a 30s-timeout retry to
filter 2 false-positives caused by 15s-timeout flake on cold SSH sessions):

| Submodule (HelixAgent path)        | URL                                                         | Status        | Disposition |
|---|---|---|---|
| HelixAgent/DebateOrchestrator      | `git@github.com:vasic-digital/DebateOrchestrator.git`        | UNREACHABLE   | vasic-digital owned — repo missing/never-created. Needs user decision: create vs delete. |
| HelixAgent/cli_agents/kiro-cli     | `git@github.com:stark1tty/kiro-cli.git`                     | UNREACHABLE   | Third-party fork deleted upstream. Already in Phase 0 §3.3 parking lot. |
| HelixAgent/cli_agents/ollama-code  | `git@github.com:tcsenpai/ollama-code.git`                   | UNREACHABLE   | Third-party repo deleted upstream. Same parking-lot disposition. |

`git submodule update --init --recursive` aborted on DebateOrchestrator
(double-failure-aborts behaviour). The pull-loop (`foreach --recursive ... git
fetch + git pull --ff-only`) ran against ~155 initialised submodules in
sequence, then was killed at `HelixAgent/cli_agents/HelixCode` (a circular
self-reference whose HEAD is `refs/heads/.invalid` and therefore non-fetchable
— scheduled for removal in plan T02.01).

Pull-loop log: `docs/improvements/p1-5-pull.log` (548 lines, ~155 submodules
processed before kill).

Per user STOP protocol, WP1 is halted here pending user decision on the 3
unreachable remotes. Reachability detail in
`docs/improvements/p1-5-remote-reachability.md`.

## .gitmodules content (root meta-repo, post-Example_Projects removal)

Root `.gitmodules` is now 63 lines (was 264 lines pre-removal). All 67
`Example_Projects/...` blocks removed; remaining entries:
- Challenges, HelixAgent, HelixQA, Containers, Security, Assets,
  Github-Pages-Website, Dependencies/{LLama_CPP, Ollama, HuggingFace_Hub}.

HelixAgent `.gitmodules` unchanged: 170 submodules across cli_agents/, MCP/,
HelixLLM/submodules/, Toolkit/, Veritas, VisionEngine, etc.

## Dirty-state per submodule (post-init)

`git status` at root reports `M HelixAgent` only — the bare M reflects the
`-dirty` suffix on HelixAgent's gitlink because HelixAgent has nested
unstageable submodule modifications cascade. No tracked-modified files at
root after the 5-commit pre-WP1 cleanup.

Inside HelixAgent: 94 nested gitlinks still show `-dirty` because their own
inner submodules carry uncommitted work (the intentional "preserve in-flight
state" from Step 1a above). These dirty-trees are committed at the HelixAgent
gitlink-pin-bumping level (`aad6a67d`) but the inner submodule WIP was not
walked deeper — by user mandate.

## Decided canonical paths for WP3 dedup (placeholder — populated by T01.04)

See `docs/improvements/p1-5-dedup-canonical.md`.

## Rollback recipe

If WP2/WP3 breaks the submodule tree:

```bash
cd /run/media/milosvasic/DATA4TB/Projects/HelixCode
# 1. Restore root meta to pre-WP1
git reset --hard cff2d90f
git submodule sync --recursive
git submodule update --init --recursive --force

# 2. Restore HelixAgent to its pre-WP1 commit
cd HelixAgent
git reset --hard aad6a67d
git submodule sync --recursive
git submodule update --init --recursive --force
cd ..

# 3. Restore Challenges to its pre-WP1 commit
cd Challenges
git reset --hard 47dc905a
cd ..

# 4. Re-add the four root remotes if any push partially landed
git remote -v   # confirm origin/github/gitlab/upstream all present
```

The destructive `--hard` is intentional for rollback. Do NOT run unless WP2/WP3
left the tree unrecoverable.

## Open issues at this snapshot

1. Three unreachable remotes (DebateOrchestrator, kiro-cli, ollama-code).
   See §Reachability. **WP1 STOP — user decision required.**
2. HelixAgent has 94 nested `-dirty` gitlinks. Captured in commit `aad6a67d`
   but not walked deeper.
3. HelixAgent/HelixLLM/submodules/* (~33 entries) remain uninitialised
   from Phase 0 — out of scope for P1.5.
4. Recursive fetch+pull-loop (T01.01 second half) was started but is still
   running at capture; partial output in `/tmp/p1-5-pull.log`.
