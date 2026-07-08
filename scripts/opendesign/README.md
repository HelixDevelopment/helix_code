# OpenDesign — Supervised Bring-Up

**Constitution:** §11.4.162 (OpenDesign is a mandatory core dependency for
every user-facing UI surface) · §11.4.161 (rootless — no root, no system
service) · §11.4.133 (host safety) · §11.4.77 (gitignored-artifact
re-obtain mechanism) · §11.4.174 (process-ownership verification before
inspecting/acting on any process).

## What this replaces

An earlier stream (`scratchpad/phase1_opendesign_install.md`) provisioned
the OpenDesign daemon (`nexu-io/open-design`) as a bare **detached
process**: clone → `pnpm install` → `nohup ... &` with no health
supervision, no restart-on-failure, and its pidfile/log living *outside*
the tracked tree (in the sibling `.opendesign-src/` clone). That was
sufficient to prove the integration works end-to-end but is not a
managed service — a crashed or hung daemon just stays down/hung until a
human notices.

`bringup.sh` is the supervised replacement: **idempotent**, **health-
checked**, **restarts only a positively-verified-ours process**, and its
own pidfile/log live inside this repo (gitignored) so they are
discoverable without knowing the external clone path.

## Usage

```bash
scripts/opendesign/bringup.sh            # same as `up`
scripts/opendesign/bringup.sh up         # ensure a healthy daemon is running
scripts/opendesign/bringup.sh status     # report state only, no side effects
scripts/opendesign/bringup.sh stop       # stop it, only if verified ours
scripts/opendesign/bringup.sh restart    # stop (if ours) + up
```

`up` (the default) is safe to run any number of times:

- **Already healthy** → prints the health probe result and exits 0
  (no-op — proven by re-running it twice in a row, see evidence below).
- **Port occupied by our own daemon but failing health checks** →
  restarts it (SIGTERM, wait, SIGKILL fallback, then a fresh start).
- **Port occupied by something that does NOT match our daemon's cmdline
  signature** (`apps/daemon/dist/cli.js`) → refuses to touch it and exits
  non-zero (§11.4.174 — never kill an unverified process; §11.4.122 —
  never silently remove something we didn't launch).
- **Nothing running** → provisions the source if missing, installs deps
  if missing, starts the daemon, and polls its health endpoint for up to
  `OD_HEALTH_TIMEOUT_S` (default 180s).

## Health check

`GET http://<OD_HOST>:<OD_PORT>/api/health` → `{"ok":true,"version":"X.Y.Z"}`.
The script parses this JSON with `jq` — a bare HTTP 200 is not trusted on
its own (§11.4.6).

## Re-obtain mechanism (§11.4.77)

The daemon's source is **not vendored into this repo** — it is a ~1.9 GB
checkout of `nexu-io/open-design`, excluded per §11.4.30. This script
*is* the documented, automated regeneration mechanism: on any fresh host
with no prior checkout, `bringup.sh up` will:

1. `git clone git@github.com:nexu-io/open-design.git` (SSH, Rule 3) into
   `OD_SRC_DIR` (default: a sibling directory of the repo root,
   `<parent-of-repo-root>/.opendesign-src/open-design` — kept OUTSIDE the
   tracked tree, matching the prior stream's layout).
2. `corepack enable && pnpm install` (bounded by `OD_INSTALL_TIMEOUT_S`,
   default 600s) if `node_modules/` or the built daemon
   (`apps/daemon/dist/`) is missing.
3. Start the daemon directly via its stable entry point,
   `node apps/daemon/dist/cli.js --port <OD_PORT> --host <OD_HOST> --no-open`
   — **not** the upstream dev launcher `pnpm tools-dev run web`, which
   deliberately probes for a free port and ignores `--port`/`OD_PORT`,
   making it unsuitable for a stable, committed `:7456` integration (the
   `.mcp.json` `open-design` MCP entry is pinned to
   `OD_DAEMON_URL=http://localhost:7456`).

Toolchain requirement: Node.js ~24, pnpm 10.33.x (matches upstream's
pinned quickstart). `corepack enable` activates the pinned pnpm version
from the checkout's `package.json`.

## Config (env-overridable, project-agnostic per §11.4.28)

| Var                      | Default                                                    |
|---------------------------|-------------------------------------------------------------|
| `OD_PORT`                 | `7456`                                                       |
| `OD_HOST`                 | `127.0.0.1`                                                  |
| `OD_SRC_DIR`               | `<parent-of-repo-root>/.opendesign-src/open-design`          |
| `OD_REPO_SSH`              | `git@github.com:nexu-io/open-design.git`                     |
| `OD_HEALTH_TIMEOUT_S`      | `180`                                                         |
| `OD_INSTALL_TIMEOUT_S`     | `600`                                                         |

No operator home-directory path is hardcoded — the source directory
default is derived from the repo root (`$(dirname "$REPO_ROOT")`), and
every value above may be overridden.

## Runtime artifacts (gitignored)

- `scripts/opendesign/run/daemon.pid` — the PID this script launched.
- `scripts/opendesign/run/daemon.log` — the daemon's stdout/stderr.
- `scripts/opendesign/run/bringup.log` — this script's own supervisor
  log (every action + health-probe result, timestamped).

Already covered by the repo's global `*.pid` / `*.log` `.gitignore`
patterns; `scripts/opendesign/run/` is also listed explicitly for
defense-in-depth (§11.4.53).

## Honest limitations (§11.4.6)

- This is a **user-level supervised script**, not an OS-managed service
  (no systemd unit, no launchd agent) — per the task's explicit
  constraint (§11.4.133 host safety / §11.4.161 rootless: no root, no
  system-wide install). It does **not** survive a host reboot on its
  own; re-run `bringup.sh up` after a reboot (or wire it into whatever
  user-level session-start mechanism the operator already uses — out of
  scope here since that would require assumptions about the host's init
  system).
- Restart only ever targets a process whose `/proc/<pid>/cmdline`
  matches the daemon's own entry point (`apps/daemon/dist/cli.js`). A
  foreign process squatting on the configured port is left alone and the
  script exits non-zero with a clear message (§11.4.174).
- `od_generate_design` (BYOK-gated) is unaffected by this script — it
  still needs operator secrets in `.env` per `docs/OPENDESIGN.md`;
  `bringup.sh` only supervises the daemon process, not BYOK
  configuration.

## Evidence

See `scratchpad/opendesign_evidence/bringup_supervised_*.log` for a
captured run proving: stop (ownership-verified) → confirmed port-free →
`status` reporting UNHEALTHY/DOWN → `up` bringing the daemon back
healthy in ~3s (source already provisioned) → a second `up` call
no-op'ing (idempotency) → the daemon's SQLite-persisted project data
(`helixcode-brand`) surviving the restart.
