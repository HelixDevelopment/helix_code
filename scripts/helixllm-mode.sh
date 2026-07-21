#!/usr/bin/env bash
# scripts/helixllm-mode.sh — switch the helixllm-coder container between:
#
#   coder  : -c 24576 --parallel 8   (8 x 3072-tok slots; HelixCode / HelixAgent)
#   claude : -c 229376 --parallel 1   (one 229376-tok slot; Claude Toolkit helixagent)
#
# In llama.cpp `-c` is the TOTAL KV context split evenly across `--parallel N`
# slots, so a slot sees c/N tokens. Claude Code's system prompt + tool schemas
# (~67k) cannot fit a 3072-tok coder slot; it needs the whole window in one slot.
#
# Recreate = podman stop && rm && run, reusing boot_coder_cdi.sh's CDI regen +
# readiness wait. The run command is DERIVED FROM THE EXISTING CONTAINER'S
# .Config.CreateCommand (swap ONLY -c and --parallel) so --metrics, flag order
# and resolved paths are carried verbatim — a lost flag breaks HelixLLM.
# VRAM-safe: the old container is stopped BEFORE the new one starts (each mode
# fits alone with headroom; both together do not). Idempotent: a no-op when
# already in the requested mode (unless --force). Fail-closed on unknown state.
#
# Usage:
#   helixllm-mode.sh coder                 # switch to coder mode
#   helixllm-mode.sh claude                # switch to claude mode
#   helixllm-mode.sh status                # print detected mode + live /props
#   helixllm-mode.sh claude --print-cmd    # print the recreate cmd, run NOTHING
#   helixllm-mode.sh coder  --force        # recreate even if already in mode
#   helixllm-mode.sh claude --container N   # operate on container N
#
# Targets Linux; `bash -n` + `shellcheck -S error` clean; no GNU-only constructs.
set -euo pipefail

CONTAINER="${HELIXLLM_CONTAINER:-helixllm-coder}"
CDI_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/cdi"
HEALTH_URL="${HELIXLLM_HEALTH_URL:-http://localhost:18434/v1/models}"
PROPS_URL="${HELIXLLM_PROPS_URL:-http://localhost:18434/props}"
IMAGE_FALLBACK="${HELIXLLM_IMAGE:-localhost/helixllm/llamacpp-router:cuda12.8-sm120}"
MODEL_FALLBACK="${HELIXLLM_MODEL:-/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf}"
READY_TIMEOUT_STEPS="${HELIXLLM_READY_STEPS:-60}"   # 60 x 10s = ~600s

USAGE="usage: helixllm-mode.sh {coder|claude|status} [--print-cmd] [--container NAME] [--force]"

log() { printf '[helixllm-mode] %s\n' "$*" >&2; }
die() { printf '[helixllm-mode] ERROR: %s\n' "$*" >&2; exit 1; }

# mode -> (ctx, parallel)
mode_ctx()      { case "$1" in coder) echo 24576;; claude) echo 229376;; *) return 1;; esac; }
mode_parallel() { case "$1" in coder) echo 8;;     claude) echo 1;;     *) return 1;; esac; }
# parallel count (or /props total_slots) -> mode name
parallel_to_mode() { case "$1" in 8) echo coder;; 1) echo claude;; *) echo unknown;; esac; }

# ---- inspection (works even when the container is STOPPED) ------------------
# Emit the container's stored CreateCommand, one argv token per line. Empty (and
# a clean exit) when the container is absent or podman is unavailable, so callers
# under `set -e` never abort on the absent path.
container_create_argv() {
  podman inspect "$CONTAINER" \
    --format '{{range .Config.CreateCommand}}{{println .}}{{end}}' 2>/dev/null || true
}

# Print the argv token immediately following $1 (a flag) in the CreateCommand.
detect_token_after() {
  container_create_argv | awk -v flag="$1" '
    take==1 { print; exit }
    $0==flag { take=1 }'
}
detect_parallel_inspect() { detect_token_after "--parallel"; }
detect_ctx_inspect()      { detect_token_after "-c"; }

# Cross-check from a live server (total_slots). Empty when server down / no jq /
# no curl — an unreadable signal must never abort or block a caller.
detect_slots_props() {
  command -v jq   >/dev/null 2>&1 || return 0
  command -v curl >/dev/null 2>&1 || return 0
  curl -s --max-time 6 "$PROPS_URL" 2>/dev/null \
    | jq -r '.total_slots // empty' 2>/dev/null || true
}

# absent | coder | claude | unknown
current_mode() {
  local p; p="$(detect_parallel_inspect)"
  if [ -z "$p" ]; then echo "absent"; return 0; fi
  parallel_to_mode "$p"
}

# ---- build the recreate command (swap ONLY -c / --parallel) ----------------
# Emit the argv (one token per line) for the target mode, derived from the
# existing container's CreateCommand. Falls back to the canonical command +
# --metrics when no container exists (first-ever create).
build_run_argv() {
  local mode="$1" ctx par create
  ctx="$(mode_ctx "$mode")"      || die "internal: no ctx for mode '$mode'"
  par="$(mode_parallel "$mode")" || die "internal: no parallel for mode '$mode'"
  create="$(container_create_argv)"
  if [ -n "$create" ]; then
    printf '%s\n' "$create" | awk -v ctx="$ctx" -v par="$par" '
      skipc==1 { skipc=0; print ctx; next }
      skipp==1 { skipp=0; print par; next }
      $0=="-c"         { print; skipc=1; next }
      $0=="--parallel" { print; skipp=1; next }
      { print }'
  else
    printf '%s\n' podman run -d --name "$CONTAINER" \
      --network=host --device nvidia.com/gpu=all --security-opt=label=disable \
      -v "$HOME/models:/models:ro" "$IMAGE_FALLBACK" \
      -m "$MODEL_FALLBACK" -ngl 99 -c "$ctx" --parallel "$par" \
      --cont-batching -fa on --cache-type-k q8_0 --cache-type-v q8_0 \
      --host 0.0.0.0 --port 18434 --jinja --metrics
  fi
}

# ---- CDI regen (mirrors boot_coder_cdi.sh) ---------------------------------
regen_cdi() {
  log "regenerating NVIDIA CDI spec (rootless) -> $CDI_DIR/nvidia.yaml"
  mkdir -p "$CDI_DIR"
  nvidia-ctk cdi generate --output="$CDI_DIR/nvidia.yaml"
  if grep -q "/dev/dri/card0" "$CDI_DIR/nvidia.yaml" 2>/dev/null && ! [ -e /dev/dri/card0 ]; then
    log "WARN: fresh CDI spec still names /dev/dri/card0 which is absent — check /dev/dri"
  fi
}

# ---- readiness wait (mirrors boot_coder_cdi.sh) ----------------------------
wait_ready() {
  log "waiting for readiness at $HEALTH_URL (up to $((READY_TIMEOUT_STEPS * 10))s)..."
  local i
  for i in $(seq 1 "$READY_TIMEOUT_STEPS"); do
    if curl -sf "$HEALTH_URL" >/dev/null 2>&1; then
      log "OK: '$CONTAINER' UP and serving (after ~$((i * 10))s)"
      return 0
    fi
    sleep 10
  done
  die "not healthy within $((READY_TIMEOUT_STEPS * 10))s. Inspect: podman logs --tail 40 $CONTAINER"
}

# ---- recreate --------------------------------------------------------------
recreate() {
  local mode="$1"
  local -a argv
  mapfile -t argv < <(build_run_argv "$mode")
  [ "${#argv[@]}" -gt 0 ] || die "internal: empty recreate argv for mode '$mode'"

  regen_cdi

  if podman container exists "$CONTAINER" 2>/dev/null; then
    log "stopping old container (frees VRAM before the new mode starts)"
    podman stop "$CONTAINER" >/dev/null 2>&1 || true
    podman rm   "$CONTAINER" >/dev/null 2>&1 || true
  fi

  log "podman run ($mode: -c $(mode_ctx "$mode") --parallel $(mode_parallel "$mode"))"
  CDI_SPEC_DIRS="$CDI_DIR" "${argv[@]}"

  wait_ready

  # Post-switch cross-check against the live server (best-effort; empty if down).
  local slots got
  slots="$(detect_slots_props)"
  if [ -n "$slots" ]; then
    got="$(parallel_to_mode "$slots")"
    [ "$got" = "$mode" ] || die "post-switch /props total_slots=$slots resolves to '$got', expected '$mode'"
    log "verified via /props: total_slots=$slots ($got)"
  else
    log "note: /props cross-check skipped (server not readable or no jq/curl)"
  fi
}

cmd_status() {
  local m ctx par slots
  m="$(current_mode)"
  ctx="$(detect_ctx_inspect)"
  par="$(detect_parallel_inspect)"
  slots="$(detect_slots_props)"
  printf 'container : %s\n' "$CONTAINER"
  printf 'mode      : %s\n' "$m"
  printf 'stored    : -c %s --parallel %s\n' "${ctx:-?}" "${par:-?}"
  printf 'live/props: total_slots=%s\n' "${slots:-<server-down-or-no-jq>}"
  # Consistency note: stored -c should match the mode's expected total context.
  if [ "$m" = coder ] || [ "$m" = claude ]; then
    local want; want="$(mode_ctx "$m")"
    if [ -n "$ctx" ] && [ "$ctx" != "$want" ]; then
      printf 'WARNING   : stored -c %s does not match %s-mode context %s (drift)\n' "$ctx" "$m" "$want"
    fi
  fi
}

main() {
  local action="" print_mode="" force=0

  while [ $# -gt 0 ]; do
    case "$1" in
      --print-cmd)
        if [ "${2:-}" = coder ] || [ "${2:-}" = claude ]; then
          print_mode="$2"; shift 2
        else
          print_mode="__action__"; shift
        fi
        ;;
      --container)
        [ $# -ge 2 ] || die "--container needs a NAME"
        CONTAINER="$2"; shift 2
        ;;
      --force) force=1; shift;;
      coder|claude|status) action="$1"; shift;;
      -h|--help) printf '%s\n' "$USAGE"; exit 0;;
      *) die "$USAGE";;
    esac
  done

  # --print-cmd is pure string construction and must not require podman.
  if [ -n "$print_mode" ]; then
    [ "$print_mode" = "__action__" ] && print_mode="$action"
    [ "$print_mode" = coder ] || [ "$print_mode" = claude ] \
      || die "--print-cmd needs a coder|claude mode (got '${print_mode:-<none>}')"
    build_run_argv "$print_mode"   # prints argv; runs NOTHING
    exit 0
  fi

  command -v podman >/dev/null 2>&1 || die "podman not on PATH"

  case "$action" in
    status) cmd_status; exit 0;;
    coder|claude)
      local cur; cur="$(current_mode)"
      if [ "$cur" = "$action" ] && [ "$force" != 1 ]; then
        log "already in '$action' mode — no-op (use --force to recreate anyway)"
        exit 0
      fi
      if [ "$cur" = unknown ] && [ "$force" != 1 ]; then
        die "current mode is UNKNOWN (--parallel is neither 1 nor 8); refusing to guess. Use --force to recreate to '$action'."
      fi
      recreate "$action"
      ;;
    *) die "$USAGE";;
  esac
}

# Source-guard: allow the hermetic test suite to source this file and exercise
# the pure functions directly, while `bash helixllm-mode.sh ...` still runs main.
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  main "$@"
fi
