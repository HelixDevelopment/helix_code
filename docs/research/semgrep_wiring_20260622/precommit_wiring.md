# Pre-commit semgrep wiring — exact change

**Constitution:** §11.4.166(3) — pre-commit `semgrep scan --config auto --error`
blocks the commit on any finding. §11.4.28/§11.4.35 — invoke the constitution
reference hook by reference, never copy it.

**Date:** 2026-06-22
**Target hook:** `/Volumes/T7/Projects/helix_code/.git/hooks/pre-commit`
**Reference script (already shipped, executable):**
`constitution/scripts/hooks/semgrep_precommit.sh`

> NOT applied by the onboarding agent (live-hook edit + meta-git race avoidance).
> The conductor/operator applies this.

---

## Current state (FACT)

The live `.git/hooks/pre-commit` is **NOT** a semgrep hook — it is the
`guard-forbidden-commands.sh` PreToolUse-style guard (§11.4.109). It currently
has **zero** semgrep reference. The semgrep wiring MUST be **ADDITIVE**: it must
not remove or weaken the existing guard. Two options:

### Option A — append a sourcing block to the existing pre-commit (RECOMMENDED)

The existing hook ends with:

```sh
# --------------------------------------------------------------------------
# Nothing matched (or only allow-marked warnings fired) → allow the command.
# --------------------------------------------------------------------------
exit 0
```

The guard hook is `set -euo pipefail` and ends in `exit 0`, so a new block must
run **before** that final `exit 0`. Replace the trailing `exit 0` with a call
into the semgrep reference hook, so its exit code governs the commit:

```sh
# --------------------------------------------------------------------------
# §11.4.166(3) — semgrep static-analysis gate (additive). Runs the constitution
# reference hook by reference (§11.4.28/§11.4.35 — never copied). It is
# graceful-degrading (skips cleanly when semgrep is absent) and blocks the
# commit (exit 1) only when `semgrep scan --config auto --error` finds issues.
# --------------------------------------------------------------------------
_SEMGREP_HOOK="$(git rev-parse --show-toplevel)/constitution/scripts/hooks/semgrep_precommit.sh"
if [ -x "$_SEMGREP_HOOK" ]; then
  "$_SEMGREP_HOOK" || exit 1
fi

exit 0
```

(Place this block immediately before the existing final `exit 0`, replacing that
single trailing `exit 0` line. The guard's own earlier `exit 2`/`exit 0` paths
are untouched — only the fall-through "allow" path now additionally runs
semgrep.)

> Caveat (§11.4.6): the existing guard runs under `set -euo pipefail`. The
> `"$_SEMGREP_HOOK" || exit 1` form is `-e`-safe (the `||` consumes the
> non-zero). Do NOT write a bare `"$_SEMGREP_HOOK"` line — under `-e` a non-zero
> from the reference hook would abort with the hook's own exit code rather than
> the intended `exit 1`, which is acceptable but less explicit.

### Option B — wire via `scripts/install_git_hooks.sh` (if the project uses a hook installer)

If the project installs hooks through a chained installer
(`scripts/install_git_hooks.sh` per §11.4.75), register
`constitution/scripts/hooks/semgrep_precommit.sh` as an additional pre-commit
stage there instead of hand-editing `.git/hooks/pre-commit`, so a hook
re-install does not drop the semgrep stage. Prefer this if the installer exists.

---

## Exact one-liner (if a minimal in-place edit is preferred)

If the operator wants the smallest possible change and accepts running semgrep
inline (rather than via the reference hook), insert before the final `exit 0`:

```sh
git diff --cached --name-only --diff-filter=ACM \
  | grep -E '\.(py|js|ts|go|java|kt|rb|c|cpp|h|hpp|ya?ml|json|sh|bash)$' \
  | { read -r _f && { echo "$_f"; cat; } | xargs -r semgrep scan --config auto --error || exit 1; }
```

**RECOMMENDED is Option A** — it reuses the constitution reference hook
(graceful degradation, staged-file filtering, evidence messaging already built
in) per §11.4.28/§11.4.74 (reuse-don't-reimplement).

---

## Apply checklist (conductor)

1. `cd /Volumes/T7/Projects/helix_code`
2. Confirm reference hook is executable: `test -x constitution/scripts/hooks/semgrep_precommit.sh && echo OK`
3. Edit `.git/hooks/pre-commit`: replace the single trailing `exit 0` with the
   Option-A block above.
4. Verify: stage a file with a known finding and attempt a commit — it MUST be
   blocked with `[semgrep-precommit] SEMGREP FOUND ISSUES — COMMIT BLOCKED`.
   Stage a clean file — commit MUST proceed.
5. Confirm the guard hook still fires (e.g. a `sudo` Bash call is still blocked).

## Sources verified 2026-06-22
- `constitution/scripts/hooks/semgrep_precommit.sh` (read this session)
- `.git/hooks/pre-commit` live content (read this session)
- Constitution §11.4.166(3); §11.4.28/§11.4.35/§11.4.74/§11.4.75
