# S3 — CONST-046 audit gate portability fix — evidence (§11.4.115 RED→GREEN)

Checkout under test: `/home/milos/Factory/projects/tools_and_research/helix_code`

## Root cause (§11.4.102)

`scripts/audit_const046/.baseline.json` keyed every violation's identity on an
**ABSOLUTE filesystem path** (e.g.
`/run/media/milosvasic/DATA4TB/Projects/HelixCode/challenges/...`) baked in at
the time the baseline was generated on a different host/clone. `main.go`'s
`violationKey(path, hash)` compared the CURRENT scan's absolute path against
that baked-in absolute path. On any OTHER checkout (different host, different
clone directory — this checkout included) every path mismatched, so
`--fail-on-new` classified 100% of findings as "NEW" regardless of whether the
source had actually changed — a §11.4.108 SOURCE→ARTIFACT/RUNTIME portability
defect and a §11.4.177 hardcoded-host-path decoupling violation. The gate was
silently non-functional on this checkout even though it was claimed to enforce
CONST-046 (HXC-003).

## RED — reproduce defect on this checkout (before fix)

```
$ bash scripts/audit-const046-hardcoded-content.sh --fail-on-new --quiet
CONST-046 audit (fail-on-new, round 99b):
  scanned files : 6380
  skipped files : 5559
  allowlist hits: 0
  baseline file : /home/milos/.../scripts/audit_const046/.baseline.json (loaded=true, entries=52927)
  Total: 19098 (NEW: 19098, PRE-EXISTING: 0)
$ echo $?
1
```
19098/19098 false "NEW" violations, exit 1 — defect confirmed present.

Baseline path sample (absolute, host-specific):
```
/run/media/milosvasic/DATA4TB/Projects/HelixCode/challenges/cmd/userflow-runner/main.go
```

## Fix

- `scripts/audit_const046/main.go`:
  - Added optional `--repo-root` flag.
  - Added `normalizeReportPaths()`: after `hashViolations()` (which needs the
    real absolute path to read the file) and before sort/baseline-compare/
    output, rewrites every violation's `Path` to a repo-root-relative,
    slash-normalized (`filepath.ToSlash`) path via `filepath.Rel`. A path that
    resolves outside `repoRoot` is left absolute (safe fallback, never a
    silently dropped finding).
  - Omitting `--repo-root` preserves the legacy absolute-path behavior exactly
    (backward compatible — existing tests pass unmodified).
  - Bumped `toolVersion` "0.2" → "0.3"; banner text updated to "round 100".
- `scripts/audit-const046-hardcoded-content.sh`: now passes
  `--repo-root "${REPO_ROOT}"` (REPO_ROOT already computed dynamically from
  the script's own location — zero hardcoded host path, §11.4.177). Caller
  can still override via `--repo-root` in `"$@"` (same last-flag-wins
  precedent as `--baseline`).
- Regenerated `scripts/audit_const046/.baseline.json` (+ `.baseline.json.gz`
  kept in sync) via `--update-baseline` with the fix active. Verified 0
  absolute paths remain in the regenerated baseline (19098/19098 entries now
  repo-relative, e.g. `helix_code/applications/desktop/main_nogui.go`).

## GREEN — re-run on this checkout (after fix, unmodified tree)

```
$ bash scripts/audit-const046-hardcoded-content.sh --fail-on-new --quiet
CONST-046 audit (fail-on-new, round 100):
  scanned files : 6380
  skipped files : 5561
  allowlist hits: 0
  repo root     : /home/milos/Factory/projects/tools_and_research/helix_code (paths normalized relative+portable)
  baseline file : /home/milos/.../scripts/audit_const046/.baseline.json (loaded=true, entries=18079)
  Total: 19098 (NEW: 0, PRE-EXISTING: 19098)
$ echo $?
0
```
0 false NEW, exit 0. (Entry count 18079 < 19098 total findings because
identical-literal duplicates within the same file collapse to one
(path,hash) key in the baseline set — expected, does not affect matching.)

## Module test suite (go test ./...)

```
$ cd scripts/audit_const046 && gofmt -l . && go vet ./... && go test ./... -count=1 -v
(gofmt: no output — clean)
(go vet: no output — clean)
=== RUN   TestHeuristic_FlagsViolationFixture            --- PASS
=== RUN   TestHeuristic_PassesCleanFixture                --- PASS
=== RUN   TestHeuristic_ExemptsDeveloperFacing            --- PASS
=== RUN   TestAllowlist_SuppressesKnownEntries            --- PASS
=== RUN   TestExitCode_AlwaysZeroInSoftWarn               --- PASS
=== RUN   TestBaseline_FailOnNew_NewViolationFails        --- PASS
=== RUN   TestBaseline_FailOnNew_PreExistingPasses        --- PASS
=== RUN   TestBaseline_UpdateBaseline_WritesFile          --- PASS
=== RUN   TestBaseline_MissingFile_TreatsAsEmpty          --- PASS
=== RUN   TestBaseline_RepoRootRelative_PortableAcrossDifferentAbsolutePaths   --- PASS  (NEW)
=== RUN   TestBaseline_WithoutRepoRoot_AbsolutePathBehaviorUnchanged           --- PASS  (NEW)
=== RUN   TestBaseline_HashStability_LineShiftIgnored     --- PASS
PASS
ok  	dev.helix.code/scripts/audit_const046	1.771s
```
All pre-existing tests pass UNMODIFIED (no test asserted absolute-path
behavior — §11.4.120 reconciliation was not needed). Two new regression
guards added:
- `TestBaseline_RepoRootRelative_PortableAcrossDifferentAbsolutePaths` —
  seeds a baseline under one absolute temp-dir root ("checkout A"), asserts
  the on-disk baseline stores the repo-relative path `pkg/v.go` (not
  absolute), then re-scans the IDENTICAL relative content under a
  COMPLETELY DIFFERENT absolute temp-dir root ("checkout B") and asserts
  `NEW: 0 / PRE-EXISTING: 1` — this is the exact defect scenario, proven
  fixed.
- `TestBaseline_WithoutRepoRoot_AbsolutePathBehaviorUnchanged` — proves
  omitting `--repo-root` keeps legacy absolute-path baseline identity
  (backward compatibility).

## §1.1 — confirm a genuinely-planted violation still trips the gate (enforcement not neutered)

```
$ mkdir -p /tmp/const046_plant_test/pkg
$ cat > /tmp/const046_plant_test/pkg/planted.go <<'EOF'
package pkg

var PlantedViolation = "This is a brand new hardcoded user-facing sentence for CONST-046 planted-violation verification."
EOF
$ ./audit_const046 --roots /tmp/const046_plant_test --repo-root /tmp/const046_plant_test \
    --baseline scripts/audit_const046/.baseline.json --fail-on-new --quiet
CONST-046 audit (fail-on-new, round 100):
  scanned files : 1
  skipped files : 0
  allowlist hits: 0
  repo root     : /tmp/const046_plant_test (paths normalized relative+portable)
  baseline file : scripts/audit_const046/.baseline.json (loaded=true, entries=18079)
  Total: 1 (NEW: 1, PRE-EXISTING: 0)
$ echo $?
1
```
A genuinely new hardcoded-content violation is still caught (NEW:1, exit 1) —
the fix did not weaken enforcement, only fixed path-identity portability.

## Files modified (scope-limited)

- `scripts/audit_const046/main.go`
- `scripts/audit_const046/main_test.go`
- `scripts/audit_const046/.baseline.json` (regenerated, relative paths)
- `scripts/audit_const046/.baseline.json.gz` (regenerated in sync)
- `scripts/audit-const046-hardcoded-content.sh`

No `git add`/commit performed (per instructions). `git status --porcelain`
confirms only these five paths are modified.
