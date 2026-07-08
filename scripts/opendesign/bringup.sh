#!/usr/bin/env bash
# OpenDesign supervised bring-up (§11.4.162 core dependency).
#
# Starts, health-checks, and (if unhealthy) restarts the OpenDesign daemon
# (nexu-io/open-design) on its documented port, idempotently. This is a
# USER-LEVEL supervised script — it does NOT install a systemd unit and
# does NOT require root (§11.4.133 host safety, §11.4.161 rootless).
#
# Re-obtain mechanism (§11.4.77): the OpenDesign daemon source is NOT
# vendored into this repo (§11.4.30 — it is a ~1.9GB checkout). This
# script IS the documented, automated mechanism that reconstructs it on
# any fresh host: clone (SSH, Rule 3) -> pnpm install -> start the daemon
# entry point directly (apps/daemon/dist/cli.js), never the ephemeral-port
# dev launcher (`pnpm tools-dev run web` probes a free port and ignores
# --port/OD_PORT — unsuitable for a stable, committed integration).
#
# Usage:
#   scripts/opendesign/bringup.sh [up|status|stop|restart]
#     up (default)  - ensure a healthy daemon is running; no-op if already
#                      healthy; restarts if a process on the port is ours
#                      but unhealthy; clones+installs+starts if nothing is
#                      running.
#     status         - report current state (healthy / unhealthy / down)
#                      without changing anything. Exit 0 healthy, 1 else.
#     stop            - stop the daemon IF it was started by this script
#                      (verified via pidfile + cmdline match). Never kills
#                      an unverified process (§11.4.174 / §11.4.122).
#     restart         - stop (if ours) then up.
#
# Env overrides (project-agnostic, §11.4.28 — no hardcoded operator paths
# beyond the documented sibling-directory default):
#   OD_PORT            daemon port                         (default 7456)
#   OD_HOST             daemon bind host                     (default 127.0.0.1)
#   OD_SRC_DIR          from-source checkout directory
#                        (default: <parent-of-repo-root>/.opendesign-src/open-design)
#   OD_REPO_SSH          upstream SSH clone URL
#                        (default: git@github.com:nexu-io/open-design.git)
#   OD_HEALTH_TIMEOUT_S  seconds to poll after a (re)start           (default 180)
#   OD_INSTALL_TIMEOUT_S seconds bound on `pnpm install`             (default 600)
#
# Evidence: every run appends a timestamped line to the run log
# (scripts/opendesign/run/daemon.log) and to its own supervisor log
# (scripts/opendesign/run/bringup.log). Both are gitignored (§11.4.30 /
# already covered by the repo-wide *.log / *.pid patterns, plus an
# explicit scripts/opendesign/run/ entry for defense-in-depth).
set -uo pipefail

# --- Resolve paths (no hardcoded operator home paths) -----------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RUN_DIR="$SCRIPT_DIR/run"
PIDFILE="$RUN_DIR/daemon.pid"
DAEMON_LOG="$RUN_DIR/daemon.log"
SUP_LOG="$RUN_DIR/bringup.log"
mkdir -p "$RUN_DIR"

# --- Config (env-overridable) ------------------------------------------
OD_PORT="${OD_PORT:-7456}"
OD_HOST="${OD_HOST:-127.0.0.1}"
OD_SRC_DIR="${OD_SRC_DIR:-$(dirname "$REPO_ROOT")/.opendesign-src/open-design}"
OD_REPO_SSH="${OD_REPO_SSH:-git@github.com:nexu-io/open-design.git}"
OD_HEALTH_TIMEOUT_S="${OD_HEALTH_TIMEOUT_S:-180}"
OD_INSTALL_TIMEOUT_S="${OD_INSTALL_TIMEOUT_S:-600}"
HEALTH_URL="http://${OD_HOST}:${OD_PORT}/api/health"

ACTION="${1:-up}"

log() {
  local line
  line="[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"
  echo "$line" | tee -a "$SUP_LOG"
}

# --- Health probe --------------------------------------------------------
# Returns 0 + prints "ok=<bool> version=<str>" on a reachable HTTP response,
# returns 1 (unreachable / non-JSON / ok!=true) otherwise. Never trusts a
# bare HTTP 200 without parsing the body (§11.4.6 — no assuming).
health_probe() {
  local body
  body="$(curl -s -m 5 "$HEALTH_URL" 2>/dev/null)" || return 1
  [ -n "$body" ] || return 1
  local ok version
  ok="$(printf '%s' "$body" | jq -r '.ok // empty' 2>/dev/null)"
  version="$(printf '%s' "$body" | jq -r '.version // empty' 2>/dev/null)"
  if [ "$ok" = "true" ]; then
    printf 'ok=true version=%s\n' "${version:-unknown}"
    return 0
  fi
  printf 'ok=false raw=%s\n' "$body"
  return 1
}

is_healthy() { health_probe >/dev/null 2>&1; }

# --- Ownership-verified process discovery (§11.4.174) --------------------
# A PID is "ours" only if it is alive AND its cmdline matches the exact
# daemon entry point + configured port we launch. A loose name match
# (e.g. bare `node`) is never sufficient.
pid_is_our_daemon() {
  local pid="$1"
  [ -n "$pid" ] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  local cmdline
  cmdline="$(tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null)" || return 1
  case "$cmdline" in
    *apps/daemon/dist/cli.js*"--port ${OD_PORT}"*) return 0 ;;
    *apps/daemon/dist/cli.js*"--port=${OD_PORT}"*) return 0 ;;
    *apps/daemon/dist/cli.js*) return 0 ;;  # port arg formatting may vary; entry-point match still required
    *) return 1 ;;
  esac
}

recorded_pid() {
  [ -f "$PIDFILE" ] && cat "$PIDFILE" 2>/dev/null
}

# Find any PID listening on OD_HOST:OD_PORT via ss (used only to detect a
# FOREIGN occupant of the port; we never act on it unless it also passes
# pid_is_our_daemon).
port_listener_pid() {
  ss -ltnp 2>/dev/null | awk -v host="$OD_HOST" -v port="$OD_PORT" \
    '$0 ~ (host":"port"[[:space:]]") { print }' \
    | grep -oP 'pid=\K[0-9]+' | head -1
}

stop_daemon() {
  local pid
  pid="$(recorded_pid)"
  # Fall back to the port occupant (e.g. a daemon started outside this
  # script's pidfile tracking, such as a prior manual bring-up) — still
  # requires positive cmdline-based ownership verification (§11.4.174)
  # before we ever act on it.
  if { [ -z "${pid:-}" ] || ! pid_is_our_daemon "$pid"; }; then
    local occupant
    occupant="$(port_listener_pid)"
    if [ -n "${occupant:-}" ] && pid_is_our_daemon "$occupant"; then
      pid="$occupant"
    fi
  fi
  if [ -n "${pid:-}" ] && pid_is_our_daemon "$pid"; then
    log "stop: sending SIGTERM to our daemon pid=$pid"
    kill -TERM "$pid" 2>/dev/null
    for _ in $(seq 1 20); do
      kill -0 "$pid" 2>/dev/null || { log "stop: pid=$pid exited cleanly"; rm -f "$PIDFILE"; return 0; }
      sleep 0.5
    done
    log "stop: pid=$pid did not exit after SIGTERM, sending SIGKILL"
    kill -KILL "$pid" 2>/dev/null
    rm -f "$PIDFILE"
    return 0
  fi
  log "stop: no verified-ours daemon pid recorded (pidfile=${pid:-<none>}) — nothing to stop"
  return 1
}

ensure_source() {
  if [ ! -d "$OD_SRC_DIR/.git" ]; then
    log "source: cloning $OD_REPO_SSH -> $OD_SRC_DIR (SSH, Rule 3)"
    mkdir -p "$(dirname "$OD_SRC_DIR")"
    if ! git clone "$OD_REPO_SSH" "$OD_SRC_DIR" >>"$SUP_LOG" 2>&1; then
      log "source: FAILED to clone $OD_REPO_SSH"
      return 1
    fi
  else
    log "source: checkout already present at $OD_SRC_DIR ($(cd "$OD_SRC_DIR" && git rev-parse --short HEAD 2>/dev/null))"
  fi
  if [ ! -d "$OD_SRC_DIR/apps/daemon/dist" ] || [ ! -d "$OD_SRC_DIR/node_modules" ]; then
    log "source: installing deps (corepack + pnpm install, bound ${OD_INSTALL_TIMEOUT_S}s)"
    (
      cd "$OD_SRC_DIR" || exit 1
      corepack enable >>"$SUP_LOG" 2>&1 || true
      timeout "$OD_INSTALL_TIMEOUT_S" pnpm install >>"$SUP_LOG" 2>&1
    )
    local rc=$?
    if [ "$rc" -ne 0 ]; then
      log "source: pnpm install FAILED rc=$rc"
      return 1
    fi
  fi
  if [ ! -f "$OD_SRC_DIR/apps/daemon/dist/cli.js" ]; then
    log "source: apps/daemon/dist/cli.js still missing after install — cannot start"
    return 1
  fi
  return 0
}

start_daemon() {
  log "start: launching daemon entry point on ${OD_HOST}:${OD_PORT}"
  (
    cd "$OD_SRC_DIR" || exit 1
    exec node apps/daemon/dist/cli.js --port "$OD_PORT" --host "$OD_HOST" --no-open
  ) >>"$DAEMON_LOG" 2>&1 &
  local pid=$!
  disown
  echo "$pid" > "$PIDFILE"
  log "start: daemon pid=$pid (pidfile=$PIDFILE log=$DAEMON_LOG)"
}

wait_for_health() {
  local waited=0 step=3
  while [ "$waited" -lt "$OD_HEALTH_TIMEOUT_S" ]; do
    if is_healthy; then
      return 0
    fi
    sleep "$step"
    waited=$((waited + step))
  done
  return 1
}

do_status() {
  local probe
  if probe="$(health_probe)"; then
    log "status: HEALTHY ($probe) url=$HEALTH_URL"
    return 0
  fi
  log "status: UNHEALTHY/DOWN url=$HEALTH_URL"
  return 1
}

do_up() {
  log "bringup: action=up port=$OD_PORT host=$OD_HOST src=$OD_SRC_DIR"

  if is_healthy; then
    local probe
    probe="$(health_probe)"
    log "bringup: already healthy ($probe) — idempotent no-op"
    return 0
  fi

  # Something may be listening but unhealthy, or nothing at all.
  local occupant
  occupant="$(port_listener_pid)"
  if [ -n "${occupant:-}" ]; then
    if pid_is_our_daemon "$occupant"; then
      log "bringup: port occupied by our own (unhealthy) daemon pid=$occupant — restarting"
      echo "$occupant" > "$PIDFILE"
      stop_daemon
    else
      log "bringup: port ${OD_HOST}:${OD_PORT} occupied by pid=$occupant which is NOT positively ours (§11.4.174) — refusing to touch it. ABORT."
      return 2
    fi
  fi

  ensure_source || { log "bringup: source provisioning failed — ABORT"; return 3; }
  start_daemon

  if wait_for_health; then
    local probe
    probe="$(health_probe)"
    log "bringup: DONE — daemon healthy ($probe)"
    return 0
  fi

  log "bringup: daemon did NOT become healthy within ${OD_HEALTH_TIMEOUT_S}s — tail of $DAEMON_LOG:"
  tail -30 "$DAEMON_LOG" 2>/dev/null | sed 's/^/  /' | tee -a "$SUP_LOG"
  return 4
}

case "$ACTION" in
  up) do_up; exit $? ;;
  status) do_status; exit $? ;;
  stop) stop_daemon; exit $? ;;
  restart) stop_daemon; do_up; exit $? ;;
  *)
    echo "usage: $0 [up|status|stop|restart]" >&2
    exit 64
    ;;
esac
