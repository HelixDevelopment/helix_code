#!/usr/bin/env bash
#
# install_helix_path.sh
#
# Builds (or locates already-built) the HelixCode power sub-systems --
# HelixCode server + CLI, HelixAgent, HelixLLM, LLMsVerifier, HelixQA -- and
# installs each into a user-writable PATH directory (no sudo, no root),
# idempotently exporting that directory onto the user's shell rc.
#
# Design: see docs/design/setup_path_install.md (same authoring pass).
#
# Intended placement: the root of the HelixCode meta-repo checkout, i.e.
# alongside this repo's own setup.sh. The script resolves the repo root from
# its own location -- it is decoupled and carries no hardcoded absolute host
# path (constitution §11.4.28 / §11.4.177).
#
# Usage:
#   ./install_helix_path.sh                 # build missing components, install, verify
#   ./install_helix_path.sh --skip-build    # only (re)install + verify existing bin/*
#   HELIX_BIN_DIR=/custom/bin ./install_helix_path.sh
#   HELIX_REPO_ROOT=/path/to/helix_code ./install_helix_path.sh
#
set -euo pipefail

# ---------------------------------------------------------------------------
# 0. Resolve paths (decoupled -- never hardcode an absolute host path)
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd -P)"
REPO_ROOT="${HELIX_REPO_ROOT:-$SCRIPT_DIR}"
HELIX_BIN_DIR="${HELIX_BIN_DIR:-$HOME/.local/bin}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
EVIDENCE_DIR="$REPO_ROOT/qa-results/path_install/$TS"

SKIP_BUILD=0
for arg in "$@"; do
  case "$arg" in
    --skip-build) SKIP_BUILD=1 ;;
    -h|--help)
      grep '^#' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "unknown option: $arg" >&2
      exit 2
      ;;
  esac
done

mkdir -p "$HELIX_BIN_DIR" "$EVIDENCE_DIR"

log()  { printf '%s\n' "$*"; }
hr()   { printf -- '-------------------------------------------------------------\n'; }

# ---------------------------------------------------------------------------
# 1. Component table
#
# Fields (pipe-separated, no spaces around |):
#   name | build_dir (relative to REPO_ROOT) | make_target | expected_bin_relpath | final_bin_name | mandatory(1/0)
#
# Real build targets, cited file:line (investigated 2026-07-07, §11.4.6 --
# never guessed):
#   HelixCode server : helix_code/Makefile:4 (BINARY_NAME=helixcode),
#                       helix_code/Makefile:98-101 (`build:` target)
#   HelixAgent       : submodules/helix_agent/Makefile:23-25 (`build:` target)
#   HelixLLM         : submodules/helix_llm/Makefile:4 (BINARY := helixllm),
#                       submodules/helix_llm/Makefile:11-12 (`build:` target)
#   LLMsVerifier     : the outer submodules/llms_verifier/Makefile `build:`
#                       target (line 35-36, `go build -o bin/llm-verifier ./cmd`)
#                       is STALE -- there is NO `cmd/` at the outer level. The
#                       real entry point is the NESTED Go module
#                       submodules/llms_verifier/llm-verifier (module
#                       digital.vasic.llmsverifier), main at
#                       submodules/llms_verifier/llm-verifier/cmd/main.go
#                       (verified 2026-07-08 against the tree; the same
#                       Makefile's `build-acp` target already cd's into
#                       llm-verifier/, confirming `build` is a leftover). We
#                       therefore direct-`go build` ./cmd inside the nested
#                       module rather than call the broken make target.
#   HelixCode CLI    : NO Makefile target exists for bin/cli (open question,
#                       see design doc) -- treated as best-effort direct
#                       `go build`, non-mandatory.
#   HelixQA          : submodules/helix_qa/Makefile:17-18 (`build:` ->
#                       `go build -o bin/helixqa ./cmd/helixqa`) -- HelixQA is
#                       the constitutionally-mandated (CONST-050) autonomous
#                       QA power sub-system; added 2026-07-08 as part of the
#                       "and all others" scope of this installer (§11.4.74
#                       extend-the-scan-not-reimplement -- discovered by
#                       grepping every submodules/*/Makefile for a real
#                       `build:` target producing a binary).
#
# make_target field: a literal make target name, OR `go:<pkg>` to run
# `go build -o <bin_relpath> <pkg>` inside build_dir (used where no correct
# make target exists).
# ---------------------------------------------------------------------------
COMPONENTS=(
  "helixcode|helix_code|build|bin/helixcode|helixcode|1"
  "helixagent|submodules/helix_agent|build|bin/helixagent|helixagent|1"
  "helixllm|submodules/helix_llm|build|bin/helixllm|helixllm|1"
  "llms-verifier|submodules/llms_verifier/llm-verifier|go:./cmd|bin/llm-verifier|llm-verifier|1"
  "helixcode-cli|helix_code|go:./cmd/cli|bin/cli|helixcli|0"
  "helixqa|submodules/helix_qa|build|bin/helixqa|helixqa|1"
)

declare -A RESULT_STATUS
declare -A RESULT_NOTE

# ---------------------------------------------------------------------------
# 2. Build phase
# ---------------------------------------------------------------------------
build_component() {
  local name="$1" build_dir="$2" make_target="$3" bin_relpath="$4"
  local abs_build_dir="$REPO_ROOT/$build_dir"
  local abs_bin="$abs_build_dir/$bin_relpath"
  local log_file="$EVIDENCE_DIR/${name}.build.log"

  if [ ! -d "$abs_build_dir" ]; then
    RESULT_STATUS[$name]="MISSING"
    RESULT_NOTE[$name]="source dir not found: $abs_build_dir (submodule not checked out?)"
    return
  fi

  # --skip-build contract: NEVER build. Use an existing binary if present,
  # otherwise honestly mark MISSING (do not silently build anyway).
  if [ "$SKIP_BUILD" -eq 1 ]; then
    if [ -x "$abs_bin" ]; then
      RESULT_NOTE[$name]="skip-build: using existing $abs_bin"
    else
      RESULT_STATUS[$name]="MISSING"
      RESULT_NOTE[$name]="--skip-build set but no prebuilt binary at $abs_bin"
    fi
    return
  fi

  log "==> building $name in $abs_build_dir (target: $make_target)"
  (
    cd "$abs_build_dir"
    case "$make_target" in
      go:*)
        # Direct `go build` where no correct make target exists (see table).
        mkdir -p "$(dirname "$bin_relpath")"
        go build -o "$bin_relpath" "${make_target#go:}"
        ;;
      *)
        make "$make_target"
        ;;
    esac
  ) >"$log_file" 2>&1 || {
    RESULT_STATUS[$name]="BUILD-FAILED"
    RESULT_NOTE[$name]="see $log_file"
    return
  }

  if [ ! -x "$abs_bin" ]; then
    RESULT_STATUS[$name]="BUILD-FAILED"
    RESULT_NOTE[$name]="build reported success but $abs_bin not found/executable (see $log_file)"
  fi
}

hr
log "HelixCode power sub-systems -- build phase"
hr
for entry in "${COMPONENTS[@]}"; do
  IFS='|' read -r name build_dir make_target bin_relpath final_bin_name mandatory <<<"$entry"
  build_component "$name" "$build_dir" "$make_target" "$bin_relpath"
done

# ---------------------------------------------------------------------------
# 3. Install phase (symlink into user-writable PATH dir, no sudo)
# ---------------------------------------------------------------------------
hr
log "Install phase -> $HELIX_BIN_DIR"
hr
for entry in "${COMPONENTS[@]}"; do
  IFS='|' read -r name build_dir make_target bin_relpath final_bin_name mandatory <<<"$entry"
  [ -n "${RESULT_STATUS[$name]:-}" ] && continue  # already MISSING/BUILD-FAILED

  abs_bin="$REPO_ROOT/$build_dir/$bin_relpath"
  if [ ! -x "$abs_bin" ]; then
    RESULT_STATUS[$name]="MISSING"
    RESULT_NOTE[$name]="expected binary not found after build phase: $abs_bin"
    continue
  fi

  ln -sf "$abs_bin" "$HELIX_BIN_DIR/$final_bin_name"
  log "linked $HELIX_BIN_DIR/$final_bin_name -> $abs_bin"
done

# ---------------------------------------------------------------------------
# 4. Idempotent PATH export into shell rc files
# ---------------------------------------------------------------------------
BEGIN_MARK="# >>> HelixCode PATH (managed by install_helix_path.sh) >>>"
END_MARK="# <<< HelixCode PATH (managed by install_helix_path.sh) <<<"
PATH_LINE="export PATH=\"$HELIX_BIN_DIR:\$PATH\""

update_rc() {
  local rc_file="$1"
  [ -f "$rc_file" ] || return 0

  if grep -qF "$BEGIN_MARK" "$rc_file" 2>/dev/null; then
    # §9.2 data-safety: a BEGIN marker with NO matching END marker means a
    # prior run was interrupted mid-write. The in-place awk below would skip
    # from BEGIN to EOF and TRUNCATE every real line after it. Refuse to
    # rewrite such an rc -- back it up and leave it untouched (the final
    # report still prints the manual `export PATH` line for the user).
    if ! grep -qF "$END_MARK" "$rc_file" 2>/dev/null; then
      cp -p "$rc_file" "$rc_file.helix-bak.$TS"
      log "WARNING: $rc_file has a HelixCode BEGIN marker but no END marker"
      log "         (corrupted/interrupted prior run). Backed up to"
      log "         $rc_file.helix-bak.$TS and left the rc UNCHANGED to avoid"
      log "         truncation. Add the PATH export manually, or repair the"
      log "         managed block, then re-run."
      return 0
    fi
    # §9.2: back up before any in-place mutation.
    cp -p "$rc_file" "$rc_file.helix-bak.$TS"
    # Replace existing managed block in place (idempotent re-run).
    local tmp
    tmp="$(mktemp -p "$(dirname "$rc_file")")"  # same fs -> mv is atomic
    awk -v begin="$BEGIN_MARK" -v end="$END_MARK" -v line="$PATH_LINE" '
      $0 == begin { print; print line; skip=1; next }
      $0 == end   { print; skip=0; next }
      skip == 1   { next }
      { print }
    ' "$rc_file" >"$tmp"
    mv "$tmp" "$rc_file"
    log "updated existing HelixCode PATH block in $rc_file (backup: $rc_file.helix-bak.$TS)"
  else
    {
      printf '\n%s\n%s\n%s\n' "$BEGIN_MARK" "$PATH_LINE" "$END_MARK"
    } >>"$rc_file"
    log "appended HelixCode PATH block to $rc_file"
  fi
}

hr
log "Shell rc PATH export phase"
hr
rc_updated=0
for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
  if [ -f "$rc" ]; then
    update_rc "$rc"
    rc_updated=1
  fi
done
if [ "$rc_updated" -eq 0 ]; then
  update_rc "$HOME/.profile"
fi

# ---------------------------------------------------------------------------
# 5. Anti-bluff verification: command -v + health probe (install exit 0 !=
#    working binary -- constitution §11.4.80 lesson).
# ---------------------------------------------------------------------------
verify_component() {
  local name="$1" final_bin_name="$2"
  [ -n "${RESULT_STATUS[$name]:-}" ] && return  # already MISSING/BUILD-FAILED

  local found_path
  if ! found_path="$(PATH="$HELIX_BIN_DIR:$PATH" command -v "$final_bin_name" 2>/dev/null)"; then
    RESULT_STATUS[$name]="MISSING"
    RESULT_NOTE[$name]="symlink created but command -v could not resolve $final_bin_name"
    return
  fi

  # §11.4.146 finding (real binary, not the fixture): HelixLLM's CLI has no
  # subcommand dispatch -- it only defines `flag` package flags. A bare
  # positional `version` arg (no leading `-`) is NOT rejected by `flag.Parse`;
  # it is silently ignored and the binary falls straight through to its
  # default behavior, which is *launching the full server* (binds a port,
  # attempts outbound HuggingFace/Redis/Qdrant network calls). Reproduced
  # against the real built binary 2026-07-08 (real_install_run.log): the
  # probe would have hung the installer on a server process instead of a
  # cheap health check. Fix: (1) try the flag-style probes first (an
  # unrecognized `-flag` is rejected fast by `flag.Parse`, exit != 0) and
  # leave the risky bare `version` last; (2) ALWAYS bound every probe with a
  # timeout and a closed stdin so a mis-probed binary can never hang this
  # installer or leak a long-lived process, regardless of probe order.
  local probe_timeout_s="${HELIX_PROBE_TIMEOUT_S:-5}"
  local timeout_cmd=()
  if command -v timeout >/dev/null 2>&1; then
    timeout_cmd=(timeout "${probe_timeout_s}s")
  fi

  local probe_out=""
  local probe_ok=0
  for flag in --help --version version; do
    if probe_out="$("${timeout_cmd[@]}" "$found_path" "$flag" </dev/null 2>&1 | head -n1)"; then
      probe_ok=1
      break
    fi
  done

  if [ "$probe_ok" -eq 1 ]; then
    RESULT_STATUS[$name]="INSTALLED"
    RESULT_NOTE[$name]="$found_path :: ${probe_out:-<no output>}"
  else
    RESULT_STATUS[$name]="BUILT-BUT-NOT-VERIFIED"
    RESULT_NOTE[$name]="$found_path exists but --version/version/--help all failed"
  fi
}

hr
log "Verification phase"
hr
for entry in "${COMPONENTS[@]}"; do
  IFS='|' read -r name build_dir make_target bin_relpath final_bin_name mandatory <<<"$entry"
  verify_component "$name" "$final_bin_name"
done

# ---------------------------------------------------------------------------
# 6. Final report
# ---------------------------------------------------------------------------
hr
log "HelixCode PATH install -- final report"
hr
printf '%-16s %-24s %s\n' "COMPONENT" "STATUS" "NOTE"
overall_rc=0
for entry in "${COMPONENTS[@]}"; do
  IFS='|' read -r name build_dir make_target bin_relpath final_bin_name mandatory <<<"$entry"
  status="${RESULT_STATUS[$name]:-INSTALLED}"
  note="${RESULT_NOTE[$name]:-}"
  printf '%-16s %-24s %s\n' "$name" "$status" "$note"
  if [ "$mandatory" -eq 1 ] && [ "$status" != "INSTALLED" ]; then
    overall_rc=1
  fi
done
hr
log "Evidence + build logs: $EVIDENCE_DIR"
log "Add $HELIX_BIN_DIR to your PATH now with: export PATH=\"$HELIX_BIN_DIR:\$PATH\""
log "(already appended idempotently to your shell rc for future sessions)"

exit "$overall_rc"
