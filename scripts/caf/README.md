# `scripts/caf/` — Fork-ALL cli_agents + auto-merge mechanism (SP3)

Reusable, fully-parameterized bash scripts that fork every third-party
`cli_agents/*` submodule under an org with a prefix, keep each fork current by
merging upstream `main`/`master`, and recursively classify nested deps.

**Nothing is hardcoded** — org / prefix / src-dir / providers / branch / depth
are all flags or `CAF_*` env vars (see `caf_lib.sh`).

## Files

| File | Purpose | External mutation? |
|------|---------|--------------------|
| `caf_lib.sh` | shared helpers (URL normalize, own-org test, fork-name, branch-detect, `caf_safe_push` no-force guard, arg parsing) | none (sourced) |
| `fork_third_party_submodule.sh` | gh+glab fork-or-create under `--org`/`--prefix`, wire `origin`=fork + `upstream`=original, record `--map-file` | **YES — OPERATOR-GATED (G-1)** |
| `update_fork_from_upstream.sh` | fetch + merge upstream `main`/`master` INTO each fork, fast-forward/merge only, push fan-out | **YES (push) — OPERATOR-GATED (G-1-push)** |
| `resolve_recursive_fork_deps.sh` | recursive nested-dep classifier: third-party→`FORK:`, own-org→`PULL_TO_ROOT` (CONST-051) | report-only by default; delegated fork is G-1 |
| `challenge_caf.sh` | anti-bluff Challenge — proves all of the above against **throwaway LOCAL git repos** (no real remote) + paired §1.1 mutation | none (mktemp scratch only) |

## Binding constraints (cited, never weakened)

- **§11.4.113** — absolute no-force-push. Every push goes through `caf_safe_push`,
  which **refuses** `--force` / `--force-with-lease` / `--mirror` / `+ref`
  overwrites. Merge is always upstream→fork on top of the fork tip, so every
  push is a fast-forward — force is never needed.
- **§11.4.28(B) / CONST-051** — forks stay decoupled / project-not-aware. The
  ONLY edits to a fork are non-source: remote config, `.gitignore`, governance
  pointers, nested-`.gitmodules` SSH rewrites. Never a blanket `git add -A`.
- **Rule 1 (No CI/CD)** — auto-merge is host-local (launchd/cron) only; no
  `.github/workflows`, `.gitlab-ci.yml`, etc.
- **Rule 3 (SSH only)** — every fork URL and rewritten nested URL is `git@…`.
- **§11.4.101** — every irreversible external mutation (fork creation, push) is
  OPERATOR-GATED behind **G-1**; the agent makes only reversible local
  decisions autonomously. The **default is `--dry-run`**.
- **CONST-033** — never touches host power state.

## Usage

```bash
# Plan only (DEFAULT — mutates nothing, no remote touched):
bash scripts/caf/fork_third_party_submodule.sh --dry-run

# OPERATOR-GATED (G-1) — actually create the forks (requires gh; glab for gitlab):
bash scripts/caf/fork_third_party_submodule.sh --execute \
     --org vasic-digital --prefix caf- --src-dir cli_agents --providers github,gitlab

# Auto-merge unit of work (G-1-push for the push step):
bash scripts/caf/update_fork_from_upstream.sh --map-file docs/caf/map.tsv --execute

# Recursive nested-dep classification (report-only):
bash scripts/caf/resolve_recursive_fork_deps.sh --src-dir cli_agents --depth 3

# Anti-bluff Challenge (local-only, no remote) + paired mutation:
bash scripts/caf/challenge_caf.sh           # → 28 PASS, 0 FAIL, exit 0
bash scripts/caf/challenge_caf.sh --mutate  # → an assertion FAILs (guard is real), exit 1
```

### Flag / env surface (all defaults overridable)

```
--org / CAF_ORG                 default vasic-digital
--prefix / CAF_PREFIX           default caf-
--src-dir / CAF_SRC_DIR         default cli_agents
--gitmodules / CAF_GITMODULES   default <repo>/.gitmodules
--providers / CAF_PROVIDERS     default github,gitlab
--gitlab-group / CAF_GITLAB_GROUP default vasic-digital
--branch / CAF_BRANCH           default "" → auto-detect remote HEAD (main|master)
--only / CAF_ONLY               restrict to a subset (resumable retry)
--exclude / CAF_EXCLUDE         default claude-code-source
--depth / CAF_DEPTH             default 3 (recursion bound)
--workdir / CAF_WORKDIR         default mktemp -d
--map-file / CAF_MAP_FILE       default docs/caf/map.tsv
--strategy / CAF_STRATEGY       merge (default) | ff-only
--visibility / CAF_VISIBILITY   private (default) | public
--dry-run (default) | --execute / --no-dry-run
--push (default) | --no-push
```

## Graceful degradation

- `gh`/`glab` absent → the real-remote fork path SKIPs with a logged reason and
  exits `2`; `--dry-run` always works.
- `update_fork_from_upstream.sh` and `resolve_recursive_fork_deps.sh` need only
  `git`.
- `challenge_caf.sh` uses **only local git** — it requires no `gh`/`glab` and
  touches no real remote.

## Auto-merge schedule (host-local, NO CI per Rule 1)

`update_fork_from_upstream.sh` is the scheduled unit of work. NOT committed as a
project pipeline.

**macOS (launchd)** — `~/Library/LaunchAgents/digital.vasic.caf-update.plist`,
weekly, runs detached:

```xml
<key>ProgramArguments</key>
<array>
  <string>/opt/homebrew/bin/bash</string>
  <string>/Volumes/T7/Projects/HelixCode/scripts/caf/update_fork_from_upstream.sh</string>
  <string>--map-file</string><string>/Volumes/T7/Projects/HelixCode/docs/caf/map.tsv</string>
  <string>--execute</string>
</array>
<key>StartCalendarInterval</key><dict><key>Weekday</key><integer>1</integer><key>Hour</key><integer>4</integer></dict>
```

**Linux (cron)** — `@weekly`:

```cron
@weekly /usr/bin/env bash -lc 'nohup bash $HOME/Projects/HelixCode/scripts/caf/update_fork_from_upstream.sh --map-file $HOME/Projects/HelixCode/docs/caf/map.tsv --execute > $HOME/Projects/HelixCode/qa-results/caf/update_$(date +%s).log 2>&1 &'
```

## Evidence / ledgers

- Run ledger: appended to `docs/caf/Status.md` (append-only).
- Conflicts: parked to `docs/caf/conflicts/<fork>_<ts>.md` — **never**
  auto-resolved (no `-s ours` / `-X theirs`).
- The Challenge writes only to a `mktemp -d` scratch tree and self-cleans.

## Exit codes

| Script | 0 | 1 | 2 |
|--------|---|---|---|
| `fork_third_party_submodule.sh` | all handled / dry-run planned | partial (some FAIL logged) | env (gh/glab/git missing) |
| `update_fork_from_upstream.sh` | all updated, no conflict | partial: a CONFLICT parked / PUSHFAIL | env (git / map-file missing) |
| `resolve_recursive_fork_deps.sh` | no CONST-051 violation | own-org nested chain flagged | env (git / scan-root missing) |
| `challenge_caf.sh` | all assertions PASS | ≥1 FAIL (incl. expected under `--mutate`) | — |
