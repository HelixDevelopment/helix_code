#!/usr/bin/env bash
# cross_platform_parity_gate.sh — Constitution §11.4.81 gate (CM-CROSS-PLATFORM-PARITY).
# Tracker item HXC-015.
#
# WHAT §11.4.81 MANDATES
#   For platform-specific primitives, a multi-platform project must ship per-OS
#   equivalents chosen at runtime via `uname -s` (or equivalent), with honest
#   kernel-gap citations where a Linux primitive has no portable equivalent.
#
# GATE POLICY (pragmatic + honest — NOT a rubber stamp)
#   Most repo shell scripts are currently Linux-only; a full per-OS rewrite is a
#   phased effort. So the gate classifies each scanned script as one of:
#
#     PASS-dispatch   The script does `case "$(uname -s)"` (or equivalent
#                     `uname -s` branching) AND covers every host-shell manifest
#                     platform's uname_s value with a non-SKIP branch, OR cites
#                     an honest kernel gap (a `# PARITY-GAP:` comment naming the
#                     uncovered platform) for any platform it omits.
#
#     PASS-linux-only The script declares `# PARITY: linux-only — <reason>` and
#                     uses no `case "$(uname -s)"` dispatch. Accepted as
#                     compliant during the phased migration.
#
#     FAIL            HARD failure — the script DOES `case "$(uname -s)"`
#                     dispatch (claims to be multi-platform) but is MISSING a
#                     host-shell manifest platform's branch with NO honest-gap
#                     citation. This is the bluff §11.4.81 forbids.
#
#     SOFT            Soft finding (reported, non-fatal) — the script uses a
#                     platform-specific primitive (sw_vers, launchctl, systemctl,
#                     /proc, cgroup, GOOS=darwin/windows ...) but has NEITHER a
#                     `# PARITY:` marker NOR a uname-dispatch block. Surfaces a
#                     candidate for the phased per-OS work without blocking.
#
#   Exit 0 when there are zero FAIL classifications (soft findings allowed).
#   Exit 1 on any FAIL. Exit 2 on usage / manifest error.
#
# SCOPE
#   Scans tracked .sh files under scripts/ by default (the gate-owned surface).
#   Pass a different root dir as $1 to widen.
#
# Honest shebang, `bash -n` clean (CONST-068).

set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
MANIFEST="$ROOT/docs/platforms/supported_platforms.yaml"
SCAN_DIR="${1:-$ROOT/scripts}"

if [[ ! -f "$MANIFEST" ]]; then
    echo "ERROR: manifest not found: $MANIFEST" >&2
    exit 2
fi

# --- Parse host-shell platforms' uname_s values from the manifest -----------
# The manifest is a small flat YAML list; we extract, per platform block, the
# uname_s value but ONLY for blocks with host_shell_target: true. No yq
# dependency — a tiny awk state machine keyed on the `- id:` block boundary.
mapfile -t HOST_UNAMES < <(awk '
    function strip(v) {
        sub(/^[^:]*:[[:space:]]*/, "", v)   # drop "key:" prefix
        sub(/[[:space:]]+#.*$/, "", v)        # drop trailing inline comment
        gsub(/["\x27]/, "", v)                 # drop quotes
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", v)
        return v
    }
    /^[[:space:]]*-[[:space:]]*id:/        { in_block=1; uname=""; host="" ; next }
    in_block && /^[[:space:]]*uname_s:/    { uname=strip($0) }
    in_block && /^[[:space:]]*host_shell_target:/ { host=strip($0)
        if (host=="true" && uname!="") print uname
        in_block=0
    }
' "$MANIFEST")

if [[ "${#HOST_UNAMES[@]}" -eq 0 ]]; then
    echo "ERROR: no host-shell platforms parsed from manifest (expected Linux/Darwin/Windows_NT)" >&2
    exit 2
fi

echo "CM-CROSS-PLATFORM-PARITY (§11.4.81) — host-shell platforms from manifest:"
printf '  - %s\n' "${HOST_UNAMES[@]}"
echo "Scanning shell scripts under: ${SCAN_DIR#"$ROOT"/}"
echo

# Platform-specific primitive signatures that signal a script touches OS-bound
# behaviour. Used only to raise SOFT findings on scripts that neither dispatch
# nor declare linux-only.
PRIMITIVE_RE='(\bsw_vers\b|\blaunchctl\b|\blaunchd\b|\bsystemctl\b|\bsystemd\b|/proc/|cgroup|\bsetrlimit\b|RLIMIT_|\bsysctl\b|GOOS=(darwin|windows)|\bplutil\b|\bdiskutil\b|\bwmic\b|\bpowershell\b)'

fail=0
soft=0
pass_dispatch=0
pass_linux=0
scanned=0

# Collect target scripts (tracked or not — gate runs on working tree).
mapfile -t SCRIPTS < <(find "$SCAN_DIR" -type f -name '*.sh' 2>/dev/null | sort)

for f in "${SCRIPTS[@]}"; do
    # Never analyse the gate or its meta-test against itself (they contain the
    # very tokens they police, by necessity).
    case "$f" in
        */cross_platform_parity_gate.sh|*/cross_platform_parity_meta_test.sh) continue ;;
    esac
    scanned=$((scanned + 1))
    rel="${f#"$ROOT"/}"

    has_dispatch=0
    if grep -Eq 'case[[:space:]]+"?\$\(uname[[:space:]]+-s\)"?[[:space:]]+in' "$f" 2>/dev/null; then
        has_dispatch=1
    elif grep -Eq '\$\(uname[[:space:]]+-s\)' "$f" 2>/dev/null && grep -Eq '(if|elif).*uname' "$f" 2>/dev/null; then
        has_dispatch=1
    fi

    has_linux_only_marker=0
    grep -Eq '^[[:space:]]*#[[:space:]]*PARITY:[[:space:]]*linux-only' "$f" 2>/dev/null && has_linux_only_marker=1

    if [[ "$has_dispatch" -eq 1 ]]; then
        # Multi-platform claim: every host-shell platform must be covered by a
        # branch OR carry an honest `# PARITY-GAP: <uname>` citation.
        missing=()
        for u in "${HOST_UNAMES[@]}"; do
            # A branch is "covered" only when the uname_s string appears as an
            # actual `case` branch PATTERN — i.e. the token (optionally inside a
            # |-alternation or with a trailing glob) immediately followed by the
            # `)` that opens the branch body. This deliberately does NOT match
            # the token inside comments or prose (a prior bug). For Windows the
            # MINGW*/MSYS*/CYGWIN* families are accepted as equivalent patterns.
            covered=0
            # token)  |  token|...)  |  ...|token)  |  token*)
            if grep -Eq "(^|[[:space:]|(])${u}[A-Za-z_*]*[[:space:]]*[)|]" "$f" 2>/dev/null; then
                covered=1
            fi
            if [[ "$covered" -eq 0 && "$u" == "Windows_NT" ]]; then
                grep -Eq "(^|[[:space:]|(])(MINGW|MSYS|CYGWIN)[A-Za-z0-9_*]*[[:space:]]*[)|]" "$f" 2>/dev/null && covered=1
            fi
            # Honest kernel-gap citation for this uncovered platform.
            if [[ "$covered" -eq 0 ]]; then
                grep -Eq "^[[:space:]]*#[[:space:]]*PARITY-GAP:.*${u}" "$f" 2>/dev/null && covered=1
            fi
            [[ "$covered" -eq 0 ]] && missing+=("$u")
        done
        if [[ "${#missing[@]}" -gt 0 ]]; then
            echo "FAIL  $rel — uname-dispatch present but missing branch/gap-citation for: ${missing[*]}"
            fail=$((fail + 1))
        else
            echo "PASS  $rel — uname-dispatch covers all host-shell platforms"
            pass_dispatch=$((pass_dispatch + 1))
        fi
        continue
    fi

    if [[ "$has_linux_only_marker" -eq 1 ]]; then
        pass_linux=$((pass_linux + 1))
        continue
    fi

    # No dispatch, no linux-only marker: soft finding only if it touches a
    # platform-specific primitive.
    if grep -Eq "$PRIMITIVE_RE" "$f" 2>/dev/null; then
        echo "SOFT  $rel — uses platform-specific primitive but has no uname-dispatch and no '# PARITY: linux-only' marker"
        soft=$((soft + 1))
    fi
done

echo
echo "=== CM-CROSS-PLATFORM-PARITY summary ==="
echo "Scripts scanned:      $scanned"
echo "PASS (dispatch):      $pass_dispatch"
echo "PASS (linux-only):    $pass_linux"
echo "SOFT findings:        $soft"
echo "HARD failures:        $fail"

if [[ "$fail" -gt 0 ]]; then
    echo "FAIL: $fail script(s) claim uname-dispatch but omit a manifest platform with no honest-gap citation"
    exit 1
fi
echo "PASS: no multi-platform script silently drops a manifest platform"
exit 0
