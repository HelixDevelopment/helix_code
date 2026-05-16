#!/usr/bin/env bash
# scripts/verify-all-constitution-rules.sh
#
# Post-Constitution-Pull validation sweep — the enforcement engine
# CONST-055 / §11.4.32 names canonical. Without this script, every
# new constitution rule cascades as a decorative anchor; with it,
# every implementable rule gate runs against the post-pull tree
# and produces a directed FAIL on violation.
#
# Gates included (each scoped to what's mechanically checkable):
#   G1  Governance cascade        — scripts/verify-governance-cascade.sh
#                                    (§11.9 + CONST-047..059, 14 anchors × 36 files)
#   G2  CONST-035 anti-bluff smoke — grep for production bluff markers
#                                    in HelixCode/internal + HelixCode/cmd
#   G3  CONST-050(A) mock-from-prod — grep for internal/mocks imports
#                                    in non-test production code
#   G4  CONST-051(C) nested-own-org — each owned submodule's .gitmodules
#                                    must contain zero own-org references
#   G5  CONST-053 .gitignore audit  — every owned submodule MUST have
#                                    a .gitignore at its root; no `.env`,
#                                    `*.pem`, `*.key`, `id_rsa*` tracked
#   G6  CONST-052 case-conformance  — soft warning for directories at
#                                    HelixCode root using PascalCase /
#                                    kebab-case (renames are phased per
#                                    CONST-052; this gate just surfaces
#                                    candidates, never fails for layout)
#
# Per CONST-055 anti-bluff: this sweep MUST be paired with a meta-test
# that plants a known violation per gate and asserts the sweep reports
# FAIL. That meta-test is scripts/test-verify-all-constitution-rules.sh
# (paired-mutation per §1.1).
#
# Exit codes:
#   0  — all gates green
#   1  — at least one gate FAIL
#   2  — script setup error (missing dependency)
#
# Usage:
#   bash scripts/verify-all-constitution-rules.sh
#   bash scripts/verify-all-constitution-rules.sh --quiet     # only print failures
#   bash scripts/verify-all-constitution-rules.sh --gate=G2   # run only one gate
#   bash scripts/verify-all-constitution-rules.sh --explain   # print gate descriptions then exit

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

QUIET=0
ONLY_GATE=""
for arg in "$@"; do
    case "$arg" in
        --quiet)         QUIET=1 ;;
        --gate=*)        ONLY_GATE="${arg#--gate=}" ;;
        --explain)       grep -E '^#   G[0-9]' "$0" | sed 's/^#   //'; exit 0 ;;
        *)               echo "unknown arg: $arg" >&2; exit 2 ;;
    esac
done

OWNED_FILE="$ROOT/docs/improvements/submodule_owned.txt"
ORG_PATTERN='vasic-digital|HelixDevelopment|red-elf|ATMOSphere1234321|Bear-Suite|BoatOS123456|Helix-Flow|Helix-Track|Server-Factory'
FAILURES=0
GATES_RUN=0
declare -a GATE_RESULTS=()

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
gate_header() {
    [[ "$QUIET" -eq 1 ]] && return 0
    echo
    echo "=== $1 ==="
}

gate_pass() {
    local id="$1" desc="$2"
    GATE_RESULTS+=("$id|PASS|$desc")
    [[ "$QUIET" -eq 1 ]] && return 0
    echo "  PASS: $desc"
}

gate_fail() {
    local id="$1" desc="$2" fix="$3"
    GATE_RESULTS+=("$id|FAIL|$desc")
    FAILURES=$((FAILURES + 1))
    echo "  FAIL ($id): $desc"
    echo "    Canonical fix: $fix"
}

want_gate() {
    [[ -z "$ONLY_GATE" || "$ONLY_GATE" == "$1" ]]
}

# ---------------------------------------------------------------------------
# G1 — Governance cascade
# ---------------------------------------------------------------------------
if want_gate G1; then
    GATES_RUN=$((GATES_RUN + 1))
    gate_header "G1 — Governance cascade (§11.9 + CONST-047..059)"
    if bash scripts/verify-governance-cascade.sh > /tmp/g1-cascade.out 2>&1; then
        gate_pass G1 "all 14 anchors present across owned submodules + root"
    else
        gate_fail G1 "cascade verifier reported failures (see /tmp/g1-cascade.out)" \
            "inspect output; add missing anchor(s) and re-run"
    fi
fi

# ---------------------------------------------------------------------------
# G2 — CONST-035 anti-bluff smoke (production code only)
# ---------------------------------------------------------------------------
if want_gate G2; then
    GATES_RUN=$((GATES_RUN + 1))
    gate_header "G2 — CONST-035 anti-bluff smoke (production code)"
    # Production = HelixCode/{internal,cmd}/**/*.go excluding *_test.go.
    bluff_hits=$(
        find HelixCode/internal HelixCode/cmd -type f -name "*.go" \
            ! -name "*_test.go" 2>/dev/null | \
        xargs grep -lE "(simulated|TODO implement|fake response|in production this would)" 2>/dev/null
    )
    if [[ -z "$bluff_hits" ]]; then
        gate_pass G2 "zero production bluff markers in HelixCode/{internal,cmd}"
    else
        gate_fail G2 "production code contains bluff markers" \
            "files: $bluff_hits — replace simulation with real implementation"
    fi
fi

# ---------------------------------------------------------------------------
# G3 — CONST-050(A) mock-from-production
# ---------------------------------------------------------------------------
if want_gate G3; then
    GATES_RUN=$((GATES_RUN + 1))
    gate_header "G3 — CONST-050(A) mock-from-production audit"
    mock_hits=$(
        find HelixCode/cmd HelixCode/applications -type f -name "*.go" \
            ! -name "*_test.go" 2>/dev/null | \
        xargs grep -lE 'dev\.helix\.code/internal/mocks' 2>/dev/null
    )
    # internal/* production code (non _test.go) must also not import internal/mocks
    internal_mock_hits=$(
        find HelixCode/internal -type f -name "*.go" \
            ! -path "*/mocks/*" ! -name "*_test.go" 2>/dev/null | \
        xargs grep -lE 'dev\.helix\.code/internal/mocks' 2>/dev/null
    )
    all_hits="$mock_hits"$'\n'"$internal_mock_hits"
    all_hits=$(printf '%s\n' "$all_hits" | grep -v '^$' || true)
    if [[ -z "$all_hits" ]]; then
        gate_pass G3 "no production code imports internal/mocks"
    else
        gate_fail G3 "production code imports mocks: $all_hits" \
            "refactor to constructor-injected real implementation; mocks ONLY in *_test.go"
    fi
fi

# ---------------------------------------------------------------------------
# G4 — CONST-051(C) nested-own-org submodule chains
# ---------------------------------------------------------------------------
if want_gate G4; then
    GATES_RUN=$((GATES_RUN + 1))
    gate_header "G4 — CONST-051(C) nested-own-org submodule chains"
    if [[ ! -f "$OWNED_FILE" ]]; then
        gate_fail G4 "owned-submodule list missing at $OWNED_FILE" \
            "create docs/improvements/submodule_owned.txt"
    else
        total_nested=0
        offenders=""
        while IFS=' |' read -r sm rest; do
            [[ -z "$sm" ]] && continue
            gm="$sm/.gitmodules"
            [[ ! -f "$gm" ]] && continue
            cnt=$(grep -cE "$ORG_PATTERN" "$gm" 2>/dev/null) || cnt=0
            cnt=$(printf '%s' "$cnt" | tr -d ' \n\r')
            [[ -z "$cnt" ]] && cnt=0
            if [[ "$cnt" -gt 0 ]] 2>/dev/null; then
                total_nested=$((total_nested + cnt))
                offenders="$offenders $sm($cnt)"
            fi
        done < "$OWNED_FILE"
        if [[ "$total_nested" -eq 0 ]]; then
            gate_pass G4 "no nested own-org submodule chains in any owned submodule"
        else
            gate_fail G4 "nested own-org submodule chains found:$offenders" \
                "move each to parent root per CONST-051(C); track as Task #254-style remediation"
        fi
    fi
fi

# ---------------------------------------------------------------------------
# G5 — CONST-053 .gitignore audit + sensitive-file presence
# ---------------------------------------------------------------------------
if want_gate G5; then
    GATES_RUN=$((GATES_RUN + 1))
    gate_header "G5 — CONST-053 .gitignore + sensitive-file audit"
    missing_gitignore=""
    tracked_sensitive=""
    # 1. Every owned submodule MUST have a .gitignore.
    if [[ -f "$OWNED_FILE" ]]; then
        while IFS=' |' read -r sm rest; do
            [[ -z "$sm" ]] && continue
            [[ ! -d "$sm" ]] && continue
            if [[ ! -f "$sm/.gitignore" ]]; then
                missing_gitignore="$missing_gitignore $sm"
            fi
        done < "$OWNED_FILE"
    fi
    # 2. Tracked sensitive files at meta-repo level (not in third-party trees).
    # We scan ls-files for canonical patterns; allowlist .env.example / .env.sample.
    tracked_sensitive=$(
        git ls-files 2>/dev/null | \
        grep -E '(^|/)\.env$|(^|/)\.env\.[^/]+$|\.pem$|\.key$|id_rsa(\.|$)|id_ed25519(\.|$)' | \
        grep -vE '\.env\.example$|\.env\.sample$|\.env\.template$' | \
        grep -vE '\.env\.full-test$|\.env\.test$|\.env\.ci$' | \
        grep -vE '^(HelixAgent|cli_agents|cli_agents_resources|Dependencies/LLama_CPP|Dependencies/Ollama|Dependencies/HuggingFace_Hub|helix_qa/tools|panoptic/tests|panoptic/docs)' || true
    )
    if [[ -z "$missing_gitignore" && -z "$tracked_sensitive" ]]; then
        gate_pass G5 "every owned submodule has .gitignore + no sensitive files tracked at owned-paths"
    else
        if [[ -n "$missing_gitignore" ]]; then
            gate_fail G5 "owned submodules missing .gitignore:$missing_gitignore" \
                "create .gitignore in each per CONST-053; minimum: build/, *.log, .env, .DS_Store"
        fi
        if [[ -n "$tracked_sensitive" ]]; then
            gate_fail G5 "tracked sensitive files found: $tracked_sensitive" \
                "git rm --cached + add to .gitignore + rotate the secret per CONST-042"
        fi
    fi
fi

# ---------------------------------------------------------------------------
# G6 — CONST-052 case-conformance (soft, surfaces candidates only)
# ---------------------------------------------------------------------------
#
# Operator safety mandate (2026-05-15): "double check that snake_case
# renaming is not applied to codebase which is by convention non-snake-case
# — we MUST NOT break the System and working building process".
#
# Per CONST-052's own "common-sense exceptions (technology-preserving)"
# clause, every directory whose name is mandated by language / tool /
# framework convention is exempt from the rename. G6 enforces those
# exemptions: it ONLY enumerates dirs that aren't already exempt, and
# it NEVER fails the build (soft surface — the actual rename is phased
# per Task #252 with full test-execution before each batch).
#
# NEVER_RENAME_PATTERNS — directories that MUST NOT be renamed even if
# their name violates snake_case, because renaming them would break the
# tooling that depends on the exact filename:
NEVER_RENAME_PATTERNS=(
    # Language / framework dirs that mandate specific case
    'gradlew*'              # Gradle wrapper (case-sensitive)
    'gradle'                # Gradle config dir
    'Cargo.toml' 'Cargo.lock'   # Rust
    'Gemfile' 'Gemfile.lock'    # Ruby
    'Makefile' 'GNUmakefile'    # GNU make
    'Dockerfile' 'Containerfile'
    'CMakeLists.txt'
    'build.gradle' 'build.gradle.kts'
    'pom.xml'
    'package.json' 'package-lock.json' 'pnpm-lock.yaml'
    'tsconfig.json' 'jsconfig.json'
    'pyproject.toml' 'setup.py' 'setup.cfg' 'requirements.txt'
    'go.mod' 'go.sum'
    # Android AOSP-mandated names (renaming = build break)
    'AndroidManifest.xml'
    'Android.bp' 'Android.mk'
    'AndroidTest.xml'
    # Apple framework / Xcode-mandated names
    'Info.plist'
    'Podfile' 'Podfile.lock'
    '*.xcodeproj' '*.xcworkspace'
    # AOSP top-level directories (the Android build system depends on these)
    'art' 'bionic' 'bootable' 'bootloader' 'build' 'cts' 'dalvik'
    'developers' 'development' 'device' 'docs' 'external' 'frameworks'
    'hardware' 'kernel' 'kernel-5.10' 'libcore' 'libnativehelper'
    'ndk' 'out' 'packages' 'pdk' 'platform_testing' 'prebuilts'
    'sdk' 'system' 'test' 'toolchain' 'tools' 'vendor'
    # Build / cache / generated artefacts (kept by tooling convention)
    'node_modules' '__pycache__' '.gradle' '.idea' '.vscode'
    'target' 'dist' 'out' 'build'
    # VCS + governance
    '.git' '.github' '.gitlab' '.svn'
    # Names that fail snake_case but ARE the canonical owned-project names
    # — these will be renamed in a deliberate phased batch per Task #252,
    # but the rename includes coordinated upstream-repo renames + every
    # consumer's .gitmodules update, so the local-dir-only check should
    # NOT flag them as "easy fixes". They're explicitly tracked.
    'HelixCode' 'Challenges' 'Containers' 'Dependencies'
    'Github-Pages-Website' 'HelixAgent' 'HelixQA' 'Security' 'Panoptic'
    'Upstreams' 'Assets' 'MCP-Servers'
)

is_protected_name() {
    local name="$1"
    for pattern in "${NEVER_RENAME_PATTERNS[@]}"; do
        # shellcheck disable=SC2053
        [[ "$name" == $pattern ]] && return 0
    done
    return 1
}

if want_gate G6; then
    GATES_RUN=$((GATES_RUN + 1))
    gate_header "G6 — CONST-052 case-conformance (soft — phased per task #252)"
    # Soft gate: list top-level dirs that aren't snake_case AND aren't
    # protected by the never-rename patterns above. Operator safety
    # mandate honoured: AOSP / framework / build-system dirs are never
    # flagged because renaming them would break the build.
    candidates=""
    while IFS= read -r name; do
        [[ -z "$name" ]] && continue
        # Already snake_case (lowercase + underscores + digits)?
        if [[ "$name" =~ ^[a-z0-9_]+$ ]]; then continue; fi
        if is_protected_name "$name"; then continue; fi
        candidates="$candidates $name"
    done < <(find . -maxdepth 1 -mindepth 1 -type d \
                 ! -path "./.git" ! -path "./.github" \
                 -printf "%f\n" 2>/dev/null)
    candidate_count=$(printf '%s\n' "$candidates" | tr ' ' '\n' | grep -v '^$' | wc -l | tr -d ' ')
    # Also enumerate protected (for forensic visibility — show what we
    # deliberately did NOT flag, so the operator can confirm the
    # exemption coverage matches their mental model).
    protected_count=$(
        while IFS= read -r name; do
            [[ -z "$name" ]] && continue
            if [[ "$name" =~ ^[a-z0-9_]+$ ]]; then continue; fi
            if is_protected_name "$name"; then echo "$name"; fi
        done < <(find . -maxdepth 1 -mindepth 1 -type d \
                     ! -path "./.git" ! -path "./.github" \
                     -printf "%f\n" 2>/dev/null) | wc -l | tr -d ' '
    )
    if [[ "$candidate_count" -eq 0 ]]; then
        gate_pass G6 "all top-level directories snake_case OR protected ($protected_count protected)"
    else
        # NOT a failure — phased rename per CONST-052 + Task #252.
        gate_pass G6 "$candidate_count unprotected rename candidates ($protected_count protected from rename)"
    fi
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
echo "=== verify-all-constitution-rules.sh summary ==="
echo "Gates run: $GATES_RUN"
echo "Failures:  $FAILURES"
if [[ "$QUIET" -eq 0 ]]; then
    for line in "${GATE_RESULTS[@]}"; do
        IFS='|' read -r id status desc <<< "$line"
        printf "  %s  %-4s  %s\n" "$id" "$status" "$desc"
    done
fi
echo
if [[ "$FAILURES" -gt 0 ]]; then
    echo "FAIL: $FAILURES gate(s) violate the constitution"
    exit 1
fi
echo "PASS: every implementable gate green — anti-bluff covenant honoured"
exit 0
