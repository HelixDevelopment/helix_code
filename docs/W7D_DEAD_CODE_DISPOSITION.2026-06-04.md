# W7D — Root-Level Dead-Code Disposition: Forensic Verdict + Deletion Plan

**Date:** 2026-06-04
**Status:** INVESTIGATED + OPERATOR-APPROVED FOR DELETION — execution deferred to next session (`continue`).
**Operator decision (2026-06-04):** "Delete all 5, HOWEVER double-check via git history how we got to these being unused — if we deliberately applied an alternative and left these, OK; but we MUST be careful it is NOT the other way around (that these are the real ones and the alternative is the mistake)."

## 1. Targets (root meta-repo Go helpers, ~2978 LOC)
- `internal/security/` (1526 LOC — manager.go + scanners.go)
- `internal/fix/` (885 LOC — security_fixer.go)
- `internal/testing/` (567 LOC — security_integration.go)
- `internal/theme/` (9 LOC — 5 hex color consts)
- `cmd/security-test/` (23 LOC — runtime-disabled logging stub)

## 2. Supersession-direction forensic (the operator's "huge mistake" check) — CONCLUSIVE: SAFE TO DELETE ROOT

**(A) Live-wiring proves the INNER copies are the real/running code, root has zero live consumers.**
Both the root meta-repo and inner `helix_code/` declare `module dev.helix.code` (two modules, same path), so inner `dev.helix.code/internal/...` imports resolve to the INNER packages. Evidence (`grep` of `helix_code/cmd/`):
- `helix_code/cmd/security_test/main.go:10` imports `dev.helix.code/internal/security` → INNER.
- `helix_code/cmd/security_fix/main.go:9` imports `dev.helix.code/internal/fix` → INNER.
- `helix_code/cmd/cli/main.go:44` (the **main CLI**) imports `dev.helix.code/internal/theme` → INNER.
- ROOT `cmd/security-test/main.go` imports NOTHING (`go list -deps ./cmd/security-test` = only itself; it is the disabled stub).
The root `internal/{security,fix,testing}` form a closed `testing→security←fix` cluster with **no root `cmd/` entry point** reaching them.

**(B) Recency confirms inner is forward, root is abandoned.**
- ROOT `internal/security`: last touched `062e11f8` 2026-05-12 (a compile-fix, NOT a deprecation).
- INNER `helix_code/internal/security`: last touched `31c57a2a` 2026-05-29.
- ROOT `internal/fix` + `cmd/security-test`: stuck at `5fcc5a49` 2025-12-11 ("Auto-commit") — ~6 months abandoned.
- INNER `helix_code/internal/fix` + `cmd/security_test`: 2026-05-29.
- `submodules/security` (`digital.vasic.security`, canonical own-org): 2026-06-03.

**(C) The 062e11f8 commit message itself distinguishes them.**
It repaired root `internal/security` to make `go build ./...` pass, explicitly noting it is *"separate from Zero-Bluff P2-T01's `HelixCode/internal/security/`"* — i.e. the INNER one is the planned/canonical implementation; the root one is the older (2025-11-08-created) separate artifact merely kept compiling.

**Conclusion:** We are NOT at risk of deleting the real implementation. The real, live, actively-maintained code is the inner (`helix_code/internal/*`, wired to live entry points incl. the main CLI) + the own-org `submodules/security`. The root copies are the genuinely-superseded, zero-consumer remnant. Deleting root is the correct direction and removes a CONST-051(B)/(C) own-org-code-duplicated-at-root violation. It is git-reversible (§9.2).

**Honest residual (§11.4.6):** No `go run ./cmd/security-test`-by-path invocation found in Makefile/scripts. Execution step below re-runs that sweep once more immediately before `git rm`.

## 3. Deletion plan (for next-session `continue` — execute in order)
1. Final ref-sweep: `grep -rn "internal/security\|internal/fix\|internal/testing\|cmd/security-test\|cmd/security_test" Makefile scripts/ .github 2>/dev/null` (root-scope only; ignore `helix_code/**` matches which are the inner module) + `grep -rn "go run ./cmd/security-test"`. Any hit → STOP + re-evaluate.
2. `git rm -r internal/security internal/fix internal/testing internal/theme cmd/security-test` (root paths only).
3. `go build ./... ` (root module) must still exit 0; `go vet ./...` clean.
4. Commit + push (github + gitlab). Message cites this doc + the forensic verdict.
5. Update CONTINUATION close-out + this doc's Status → `Completed (deleted)`.

## Sources verified 2026-06-04
git show 062e11f8; git log --reverse/-1 on internal/{security,fix} + cmd/security-test + helix_code/internal/{security,fix} + helix_code/cmd/security_test; grep of helix_code/cmd/ imports; W7D agent `go list -deps ./cmd/security-test`.
