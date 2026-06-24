# Submodule Remote-Ahead Gitlink-Integration Plan

**Date:** 2026-06-24 · **Scope:** consolidated REMOTE-AHEAD gitlink bump for owned submodules
whose upstream `main` is ahead of the recorded gitlink (challenges, helix_qa, security).
**Constitution:** advanced merge-onto-latest-main, NO force-push (§11.4.113); fetch+investigate
before integrate (§11.4.71); reuse-not-reimplement (§11.4.74); evidence-not-bluff (§11.4.6).
Conductor executes post-cycle. This doc is READ-ONLY planning — no HEAD/working-tree changed.
**Constitution submodule:** covered separately in `docs/qa/constitution_advance_investigation.md`.

Method: per submodule `git fetch <remote> main` (no checkout) + `git log <recorded-gitlink>..<remote>/main`
+ `git show <remote>/main:<file>` content compare. Recorded gitlinks from `git submodule status`.

---

## Per-submodule summary

| Submodule | Recorded gitlink | Remote main HEAD | Ahead | Safe to bump | go.mod impact | Action |
|-----------|------------------|------------------|-------|--------------|---------------|--------|
| **challenges** | `04e6c6ee` | `47691983` | 7 | YES (gitlink-only) | `// indirect` + `replace ../submodules/challenges`; NOT imported by helix_code source → **gitlink-only, no build-test needed** | Integrate my 3 still-needed fix files onto advanced main, discard 2 redundant, then bump |
| **helix_qa** | `26f686a3` | `d45bd3d` | 13 | YES, but **REQUIRES BUILD-TEST** | DIRECT `require digital.vasic.helixqa` + `replace ../submodules/helix_qa`; **imported by 3 helix_code prod files (5 pkgs: config/evidence/orchestrator/reporter/screenshot)** → a bump IS a dependency upgrade | Bump + `cd helix_code && go build ./... && go test ./internal/helixqa/... ./internal/server/...` before commit |
| **security** | `08f3e10` | `b69c7d9` | 5 | YES (gitlink-only) | `replace ../submodules/security` only, no `require`, NOT imported by helix_code source → **gitlink-only** | Plain gitlink bump |

### challenges — what landed (7 commits, recorded→remote)
- `4769198` feat: control-plane Challenge coverage (helixd/etcd/nodes, D17)
- `08e1331` fix(D8): no_suspend challenge reaches scanner + PASSes; allowlist HTML/PDF exports
- `1306ce3` test(anti-bluff): scanner-triage floor — body asserts + state-delta + skip annotations
- `03bdd3f` test(anti-bluff): scanner-triage — real observable assertions + honored skip annotations
- `d7f4656` chore(constitution): inherit Helix Constitution; submodules/ layout
- `402d176`,`69420b0` merge/constitution-layout
- **Beneficial:** new Challenge coverage + anti-bluff hardening. Bump is beneficial AND clears
  the redundant part of my uncommitted fix (see verdict below).

### helix_qa — what landed (13 commits, recorded→remote)
- `74b9e76` fix(banks): helix-ota-ab `requires_env` map→[]string schema match (§11.4.27)
- `0431935` D18: Dockerfile non-root USER hardening (DS-0002)
- `5c632ed` fix(D7): **add SKIP-OK exempt markers to 9 legit t.Skip sites** (BLUFF-G-001)
- `8c15fbd` anti-bluff capture-then-assert markers in orchestrator challenge
- `22822c9` docs(governance) genuine HelixQA manual rewrite
- `78398d8` fix(build): **repoint llmsverifier replace → ../llms_verifier (relocated under submodules/)**
- `8f35209`,`fae0795` decouple foreign-project refs / `findHelixPlayRoot`→`findProjectRoot`
- `b2836d5` fix(build): correct llmsverifier replace path (earlier)
- `6466146`,`44a1983`,`0f7c629` constitution-layout/merges
- **Beneficial:** build-wiring fixes (the relocation repoint), anti-bluff SKIP-OK markers,
  Docker hardening. **But build-graph-sensitive** — see risk note.

### security — what landed (5 commits, recorded→remote)
- `b69c7d9` test(anti-bluff): scanner-triage — real observable assertions + honored skip annotations
- `1a15069` decouple: generalize residual foreign-project refs in comments/docs/scripts
- `04ccf93`,`5c73698`,`05c4527` constitution-layout/merges
- **Beneficial:** anti-bluff + decoupling. Low risk (not in helix_code build graph).

---

## CHALLENGES REDUNDANCY VERDICT — **PARTIAL**

My uncommitted working-tree fix added 15 `SKIP-OK` markers across 5 files (still on the OLD
recorded gitlink `04e6c6ee`). Compared against the advanced remote main (`47691983`):

| File (my fix touched) | remote-main SKIP-OK | Upstream commit that did it | My fix verdict |
|-----------------------|---------------------|------------------------------|----------------|
| `pkg/container/verifier_test.go` | **3 (all sites)** | `03bdd3f` | **REDUNDANT — discard** |
| `pkg/userflow/container_infra_test.go` | **3 (all sites)** | `1306ce3`/`03bdd3f` | **REDUNDANT — discard** |
| `pkg/challenge/stress_test.go` | **0** (bare `t.Skip` short-mode ×3) | — | **STILL NEEDED — integrate** |
| `tests/chaos/chaos_test.go` | **0** (bare `t.Skip` short-mode ×5) | — | **STILL NEEDED — integrate** |
| `tests/stress/stress_test.go` | **0** (bare `t.Skip` short-mode ×3) | — | **STILL NEEDED — integrate** |

**Conclusion:** the challenges upstream-ahead (`03bdd3f`/`1306ce3` scanner-triage commits)
already added SKIP-OK to the **2 container/scanner files** → those edits are redundant and MUST
be discarded (re-applying = merge noise / conflict on identical sites). The **3 short-mode
stress/chaos files** were NOT addressed upstream (0 SKIP-OK, bare `t.Skip`) → my fix for those
11 markers must be re-applied ON TOP of the advanced main `47691983`, not the stale base.

---

## BUILD-GRAPH RISK NOTE (helix_qa only)

helix_qa remote-main `go.mod` has `replace digital.vasic.llmsverifier => ../llms_verifier`,
resolving from `submodules/helix_qa/` to `submodules/llms_verifier/` (exists). BUT that module
declares `module llmsverifier`, not `digital.vasic.llmsverifier` — a potential module-path
mismatch. Commit `78398d8` explicitly repointed this, so it is the intended post-relocation
state, but it is UNVERIFIED at the helix_code build level. This is exactly why a helix_qa bump
needs a real `go build`/`go test` (§11.4.6 — verify, don't assume), not a blind gitlink update.
challenges + security carry no such build-graph dependency.

---

## ORDERED CONDUCTOR INTEGRATION PLAN

Run from repo root, post-cycle, on a quiet host. Per §11.4.113 merge-onto-latest-main, NO force-push.

1. **security (lowest risk, gitlink-only):**
   `git -C submodules/security fetch origin main` → `git -C submodules/security checkout b69c7d9`
   → stage gitlink in meta-repo. No build-test. Commit gitlink bump.

2. **challenges (gitlink-only + partial-fix integration):**
   a. `git -C submodules/challenges fetch origin main`.
   b. **Discard the 2 redundant files** (restore to remote-main versions):
      `git -C submodules/challenges checkout origin/main -- pkg/container/verifier_test.go pkg/userflow/container_infra_test.go`.
   c. **Carry the 3 still-needed files** onto advanced main: stash/re-apply the 11 short-mode
      SKIP-OK markers (`pkg/challenge/stress_test.go` ×3, `tests/chaos/chaos_test.go` ×5,
      `tests/stress/stress_test.go` ×3) on top of `47691983` (merge-onto-latest-main; trivial —
      they touch only bare `t.Skip` lines that are unchanged upstream).
   d. Commit on a challenges branch off `47691983`, push to all upstreams (NO force).
   e. Stage the new challenges gitlink in meta-repo. (Indirect dep, not imported — no build-test.)

3. **helix_qa (LAST — build-test gated):**
   a. `git -C submodules/helix_qa fetch origin main` → checkout `d45bd3d`.
   b. **MANDATORY build-test** (it IS a dependency upgrade):
      `cd helix_code && go build ./... && go test ./internal/helixqa/... ./internal/server/...`
      — confirm the llmsverifier replace-path / module-name resolves. If it fails, STOP and
      investigate the module-path mismatch before bumping (do not commit a broken build graph).
   c. On green, stage the helix_qa gitlink in meta-repo.

4. **Meta-repo commit:** one commit bumping all 3 gitlinks (+ challenges fix carry-over reflected
   in the challenges branch), update `docs/CONTINUATION.md` (§13.1), push to all upstreams (NO force).

**Leave untouched until conductor runs:** all submodule HEADs (detached, normal) and the
challenges working-tree edits (the 5 modified files) — they remain as-is; this plan only records
the discard/carry decision for the conductor.
