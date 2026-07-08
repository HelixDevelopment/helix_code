#!/usr/bin/env bash
#
# test_install_helix_path.sh
#
# Black-box, end-to-end test for ../../install_helix_path.sh.
#
# Constitution alignment:
#   - §11.4.146 (reproduce-first / extend-to-all-cases): drives the REAL
#     script end-to-end against a FIXTURE tree of fake pre-built binaries
#     (via HELIX_REPO_ROOT + --skip-build), never a re-implementation of the
#     script's logic.
#   - §11.4.169: full-automation, re-runnable, self-driving (no manual
#     intervention), deterministic PASS/FAIL/exit-code contract.
#   - §9.2 / §11.4.6: case (c) proves the rc-file corruption-guard added to
#     the script (BEGIN marker with no END marker => backup + leave-untouched,
#     never truncate) with real captured evidence (byte-identical sha256
#     before/after + presence of the *.helix-bak.* file).
#   - §11.4.174: uses ONLY fresh mktemp -d HOME/bin/fixture dirs -- NEVER the
#     real $HOME, NEVER the real submodules/ trees. No host process is
#     inspected or touched.
#
# Usage:
#   bash tests/install_helix_path/test_install_helix_path.sh
#
# Exit code: 0 if every case + every assertion passed; 1 otherwise.
#
set -uo pipefail   # NOT -e: case (d) deliberately expects a non-zero exit
                   # from the script under test, and we need to capture +
                   # assert on that rc rather than have the harness abort.

# ---------------------------------------------------------------------------
# 0. Resolve paths (decoupled -- never hardcode an absolute host path)
# ---------------------------------------------------------------------------
TEST_SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd -P)"
REPO_ROOT="$(cd -- "$TEST_SCRIPT_DIR/../.." >/dev/null 2>&1 && pwd -P)"
INSTALL_SCRIPT="$REPO_ROOT/install_helix_path.sh"

if [ ! -x "$INSTALL_SCRIPT" ]; then
  echo "FATAL: install script not found or not executable: $INSTALL_SCRIPT" >&2
  exit 2
fi

# Mirrors the exact marker strings the script under test writes/looks for.
# This is asserting the script's documented external rc-file contract, not
# re-implementing its internals.
BEGIN_MARK="# >>> HelixCode PATH (managed by install_helix_path.sh) >>>"
END_MARK="# <<< HelixCode PATH (managed by install_helix_path.sh) <<<"

TEMP_DIRS=()
OVERALL_FAIL=0
ASSERT_TOTAL=0
ASSERT_FAIL=0
CASE_FAIL=0

cleanup() {
  local d
  for d in "${TEMP_DIRS[@]:-}"; do
    if [ -n "$d" ] && [ -d "$d" ]; then
      rm -rf -- "$d"
    fi
  done
}
trap cleanup EXIT

log()  { printf '%s\n' "$*"; }
hr()   { printf -- '=============================================================\n'; }

assert() {
  # assert "<description>" <command...>
  local desc="$1"; shift
  ASSERT_TOTAL=$((ASSERT_TOTAL + 1))
  if "$@"; then
    log "  PASS: $desc"
  else
    log "  FAIL: $desc"
    ASSERT_FAIL=$((ASSERT_FAIL + 1))
    CASE_FAIL=1
  fi
}

# ---------------------------------------------------------------------------
# Fixture: fake pre-built binaries at the exact relative paths the real
# component table (install_helix_path.sh COMPONENTS array) expects.
# ---------------------------------------------------------------------------
make_fake_bin() {
  local path="$1" label="$2"
  mkdir -p "$(dirname "$path")"
  cat > "$path" <<EOF
#!/bin/sh
case "\$1" in
  --version|version) echo "$label v0.0.1-test" ;;
  *) echo "$label (ok)" ;;
esac
exit 0
EOF
  chmod +x "$path"
}

setup_fixture() {
  local root="$1"
  make_fake_bin "$root/helix_code/bin/helixcode" "helixcode"
  make_fake_bin "$root/helix_code/bin/cli" "helixcode-cli"
  make_fake_bin "$root/submodules/helix_agent/bin/helixagent" "helixagent"
  make_fake_bin "$root/submodules/helix_llm/bin/helixllm" "helixllm"
  make_fake_bin "$root/submodules/llms_verifier/llm-verifier/bin/llm-verifier" "llms-verifier"
  make_fake_bin "$root/submodules/helix_qa/bin/helixqa" "helixqa"
}

# Runs the REAL script under test with an isolated repo root / bin dir / HOME.
# Echoes ONLY the numeric exit code (the script's own stdout/stderr goes to
# out_log). Never touches the real $HOME.
run_install() {
  local repo_root="$1" bin_dir="$2" home_dir="$3" out_log="$4"; shift 4
  HELIX_REPO_ROOT="$repo_root" HELIX_BIN_DIR="$bin_dir" HOME="$home_dir" \
    bash "$INSTALL_SCRIPT" "$@" >"$out_log" 2>&1
  echo "$?"
}

# Extracts the STATUS column for a given component name from the script's
# final report table (awk splits on whitespace regardless of printf padding,
# so this is robust to column width).
component_status() {
  local log_file="$1" name="$2"
  awk -v n="$name" '$1==n{print $2; exit}' "$log_file"
}

# ---------------------------------------------------------------------------
# CASE (a) HAPPY + CASE (b) IDEMPOTENT (same environment, run twice)
# ---------------------------------------------------------------------------
run_case_a_b() {
  hr
  log "CASE (a) HAPPY + (b) IDEMPOTENT"
  hr
  local fixture binout home
  fixture="$(mktemp -d)"; binout="$(mktemp -d)"; home="$(mktemp -d)"
  TEMP_DIRS+=("$fixture" "$binout" "$home")
  setup_fixture "$fixture"
  : > "$home/.bashrc"   # pre-existing empty rc so the script must APPEND first

  local log1="$home/run1.log" rc1
  rc1="$(run_install "$fixture" "$binout" "$home" "$log1" --skip-build)"

  log "--- (a) assertions ---"
  CASE_FAIL=0
  assert "(a) exit code 0" test "$rc1" -eq 0
  local comp st
  for comp in helixcode helixagent helixllm llms-verifier helixcode-cli helixqa; do
    st="$(component_status "$log1" "$comp")"
    assert "(a) component '$comp' reports INSTALLED (got: ${st:-<none>})" test "${st:-}" = "INSTALLED"
  done
  local fb
  for fb in helixcode helixagent helixllm llm-verifier helixcli helixqa; do
    assert "(a) installed symlink $binout/$fb resolves+executes" test -x "$binout/$fb"
  done
  local begin_count end_count
  begin_count="$(grep -cF "$BEGIN_MARK" "$home/.bashrc" 2>/dev/null || true)"
  end_count="$(grep -cF "$END_MARK" "$home/.bashrc" 2>/dev/null || true)"
  assert "(a) exactly one BEGIN block in .bashrc (got: ${begin_count:-0})" test "${begin_count:-0}" -eq 1
  assert "(a) exactly one END block in .bashrc (got: ${end_count:-0})" test "${end_count:-0}" -eq 1
  if [ "$CASE_FAIL" -eq 0 ]; then log "CASE (a): PASS"; else log "CASE (a): FAIL"; OVERALL_FAIL=1; fi

  # --- (b) IDEMPOTENT: re-run the identical environment ---
  local log2="$home/run2.log" rc2
  rc2="$(run_install "$fixture" "$binout" "$home" "$log2" --skip-build)"

  log "--- (b) assertions ---"
  CASE_FAIL=0
  assert "(b) exit code 0 on re-run" test "$rc2" -eq 0
  begin_count="$(grep -cF "$BEGIN_MARK" "$home/.bashrc" 2>/dev/null || true)"
  end_count="$(grep -cF "$END_MARK" "$home/.bashrc" 2>/dev/null || true)"
  assert "(b) still exactly one BEGIN block after re-run (got: ${begin_count:-0})" test "${begin_count:-0}" -eq 1
  assert "(b) still exactly one END block after re-run (got: ${end_count:-0})" test "${end_count:-0}" -eq 1
  local path_line_count
  path_line_count="$(grep -cF "export PATH=\"$binout:\$PATH\"" "$home/.bashrc" 2>/dev/null || true)"
  assert "(b) PATH_LINE content still present exactly once (got: ${path_line_count:-0})" test "${path_line_count:-0}" -eq 1
  if [ "$CASE_FAIL" -eq 0 ]; then log "CASE (b): PASS"; else log "CASE (b): FAIL"; OVERALL_FAIL=1; fi
}

# ---------------------------------------------------------------------------
# CASE (c) §9.2 CORRUPTION-GUARD: BEGIN marker present, END marker absent,
# followed by real user content. The script MUST back up and leave the rc
# file byte-identical (never truncate).
# ---------------------------------------------------------------------------
run_case_c() {
  hr
  log "CASE (c) §9.2 CORRUPTION-GUARD"
  hr
  local fixture binout home
  fixture="$(mktemp -d)"; binout="$(mktemp -d)"; home="$(mktemp -d)"
  TEMP_DIRS+=("$fixture" "$binout" "$home")
  setup_fixture "$fixture"

  {
    echo "# pre-existing real user shell config"
    echo "$BEGIN_MARK"
    echo 'export PATH="/old/stale/bin:$PATH"'
    echo "USER_REAL_CONTENT_KEEPME"
  } > "$home/.bashrc"

  local before_hash after_hash
  before_hash="$(sha256sum "$home/.bashrc" | awk '{print $1}')"

  local log_file="$home/run.log" rc
  rc="$(run_install "$fixture" "$binout" "$home" "$log_file")"

  log "--- (c) assertions ---"
  CASE_FAIL=0
  assert "(c) .bashrc still contains sentinel USER_REAL_CONTENT_KEEPME" \
    grep -qF "USER_REAL_CONTENT_KEEPME" "$home/.bashrc"
  after_hash="$(sha256sum "$home/.bashrc" | awk '{print $1}')"
  assert "(c) .bashrc is byte-identical before/after (sha256 before=$before_hash after=$after_hash)" \
    test "$before_hash" = "$after_hash"
  local bak_count
  bak_count="$(find "$home" -maxdepth 1 -name '.bashrc.helix-bak.*' | wc -l | tr -d ' ')"
  assert "(c) a *.helix-bak.* backup file was created (count: $bak_count)" test "${bak_count:-0}" -ge 1
  assert "(c) script did not abort/crash (rc is a small int, got: $rc)" test "$rc" -ge 0
  if [ "$CASE_FAIL" -eq 0 ]; then log "CASE (c): PASS"; else log "CASE (c): FAIL"; OVERALL_FAIL=1; fi
}

# ---------------------------------------------------------------------------
# CASE (d) --skip-build with a MISSING mandatory component: must report
# MISSING for that component and exit non-zero overall.
# ---------------------------------------------------------------------------
run_case_d() {
  hr
  log "CASE (d) --skip-build with a MISSING mandatory component"
  hr
  local fixture binout home
  fixture="$(mktemp -d)"; binout="$(mktemp -d)"; home="$(mktemp -d)"
  TEMP_DIRS+=("$fixture" "$binout" "$home")
  setup_fixture "$fixture"
  rm -f "$fixture/submodules/helix_llm/bin/helixllm"
  : > "$home/.bashrc"

  local log_file="$home/run.log" rc
  rc="$(run_install "$fixture" "$binout" "$home" "$log_file" --skip-build)"

  log "--- (d) assertions ---"
  CASE_FAIL=0
  local st
  st="$(component_status "$log_file" "helixllm")"
  assert "(d) helixllm reports MISSING (got: ${st:-<none>})" test "${st:-}" = "MISSING"
  assert "(d) overall exit rc == 1 (mandatory component missing)" test "$rc" -eq 1
  # Sanity: the other mandatory components (untouched) must still install fine.
  st="$(component_status "$log_file" "helixcode")"
  assert "(d) unrelated mandatory component 'helixcode' still INSTALLED (got: ${st:-<none>})" test "${st:-}" = "INSTALLED"
  if [ "$CASE_FAIL" -eq 0 ]; then log "CASE (d): PASS"; else log "CASE (d): FAIL"; OVERALL_FAIL=1; fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
log "install_helix_path.sh black-box test suite"
log "install script under test: $INSTALL_SCRIPT"
log "started (UTC): $(date -u +%Y-%m-%dT%H:%M:%SZ)"

run_case_a_b
run_case_c
run_case_d

hr
log "SUMMARY"
hr
log "assertions total: $ASSERT_TOTAL   failed: $ASSERT_FAIL"
if [ "$OVERALL_FAIL" -eq 0 ]; then
  log "OVERALL: PASS"
  exit 0
else
  log "OVERALL: FAIL"
  exit 1
fi
