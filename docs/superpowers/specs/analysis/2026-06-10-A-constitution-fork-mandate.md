# Workstream A — Constitution submodule + new fork-and-maintain mandate

**Analysis date:** 2026-06-10
**Mode:** READ-ONLY (no code modified, no git operations performed, no network calls)
**Request under analysis:** `docs/requests/helix_code_request_llms_access.txt` (lines 14–15)
**Scope:** Constitution submodule state; existing third-party / fork governance; draft of the NEW fork-and-maintain anchor; reusable-script specs; CONST-049 edit workflow.

Every claim below cites a file path + line range actually read this session. Where something is absent, that is stated as `ABSENT (searched …)`.

---

## Task 1 — Constitution submodule state (local only; no fetch/pull performed)

**Local HEAD (read via `git -C constitution`):**
- HEAD commit: `60e2d667ec08a0729725ed8c4ec40c74c2fa067a`
- `git -C constitution status`: `HEAD detached from 9e96f6b` / `nothing to commit, working tree clean`
- `git -C constitution log --oneline -5` top entry: `60e2d66 1.2.0-dev: §11.4.140 universal action-prefix system … + §11.4.141 token-efficiency mandate … User mandates 2026-06-09. §11.4.113 no-force-push.`
- Recent log also shows merge/cascade commits: `9e96f6b`, `f861df5` (`Merge branch 'main' of gitflic.ru:helixdevelopment/helixconstitution`), `f26e960`, `4ede601`.

**Version / revision headers actually read:**
- `constitution/Constitution.md` top-of-file anchors: the most recent substantive anchor present in the body is **§11.4.141** (token-efficiency mandate, dated 2026-06-09). Highest `11.4.N` token present = `11.4.141` (grep of `constitution/Constitution.md`, sorted numeric).
- `constitution/CLAUDE.md` metadata table (read in full): `| Revision | 23 |`, `| Last modified | 2026-06-07T12:30:00Z |`, `| Status | active |`. NOTE: the CLAUDE.md table's `Last modified` (2026-06-07) lags the Constitution.md body, but the CLAUDE.md body **does** carry the §11.4.140 + §11.4.141 mirror blocks at the end — so the mirror content is present even though the metadata `Last modified` line was not bumped in lockstep. (Minor §11.4.44 metadata-drift observation, not blocking for this workstream.)

**Currency verdict (local-state only, per instruction not to fetch):** The detached HEAD `60e2d66` is the tip of the local `constitution` checkout and carries the newest anchors (§11.4.140/§11.4.141, 2026-06-09). It is `clean`. **Whether it is current vs. the four upstreams is NOT knowable without a fetch** (per §11.4.6 / §11.4.37 remote state is unknowable without `git fetch`); CONST-049 step 1 (fetch+pull first) MUST be executed before any edit — see Task 5. The detached-HEAD state means a future edit MUST first `git checkout main` (or the canonical branch) inside the submodule before committing, or the commit will be unreachable.

---

## Task 2 — Existing anchors governing THIRD-PARTY submodules + forking (what exists vs. what is MISSING)

Searched `constitution/Constitution.md` + `constitution/CLAUDE.md` for: `third-party`, `3rd party`, `fork`, `upstream merge`, `not control`, `§11.4.74`, `§11.4.51`.

### What ALREADY EXISTS (third-party submodules are treated as "vendored, untouched, exempt")

1. **§11.4.74 — Submodule-catalogue-first discovery + extend-don't-reimplement** (`constitution/Constitution.md:6836-6869`; mirror `constitution/CLAUDE.md`). Governs **own-org** repos only (`vasic-digital` + `HelixDevelopment`): survey the 142-repo catalogue (`constitution/submodules-catalogue.md`, confirmed present), reuse ≥80%, or extend-in-place via upstream PR. It introduces the `Catalogue-Check: reuse|extend|no-match` tracker field where `no-match → vendor` is the third-party path. **It does NOT say anything about forking the vendored third-party repo.**

2. **§11.4.28(C) — Submodules-As-Equal-Codebase + dependency-layout** (`constitution/Constitution.md:2413` region; mirror `constitution/CLAUDE.md` §11.4.28 block). "Nested own-org submodule chains are FORBIDDEN … **Third-party submodules exempt.**" (`constitution/Constitution.md:2454-2456`: "Third-party submodules (not under our orgs) are exempt — they MAY appear at any depth as the upstream's structure dictates."). So today nested third-party chains are explicitly *allowed* as-is.

3. **§11.4.29 / §11.4.52 naming** (`constitution/Constitution.md:2276-2278`): "**Vendor / upstream submodules** (third-party orgs not in our owned set) keep their upstream-mandated names — we MUST NOT rename a third party's repo."

4. **§11.4.79 — own-org submodules in the CodeGraph index** (`constitution/CLAUDE.md:2491`; `constitution/Constitution.md:7143-7160`): "**Third-party submodules** (the §11.4.74 `no-match → vendor` path, e.g. `gopkg.in/telebot.v3`) — MUST be **EXCLUDED**." + respect "load-bearing pins on third-party submodules" (`Constitution.md:7149`).

5. **§11.4.65 — universal Markdown export** (`constitution/CLAUDE.md` §11.4.65 block): excludes "third-party submodules NOT in the owned set."

6. **§11.4.51 — Live-ADB-First** (`constitution/Constitution.md:4399`): unrelated to forking (rebuild-class classification). Confirmed it does NOT touch third-party submodule forking.

7. The repo-root `CLAUDE.md` §3.3 / inner architecture note: `cli_agents/` holds "reference CLI agents (aider, cline, plandex, openhands, …)" and Rule 2 of the project manual forbids modifying CLI-agent submodules ("We MUST NOT change any codebase of CLI agents Submodules since we do not control them" — request line 14 restates this).

### What is MISSING (the gap the request opens)

`ABSENT (searched constitution/Constitution.md + constitution/CLAUDE.md for "fork", "upstream merge", "not control"):` there is **NO** anchor, clause, or script that mandates ANY of the request's fork-and-maintain workflow. Specifically missing:

- **(M1)** No rule to **fork every 3rd-party (not-owned) submodule** into an owned org (`vasic-digital`) with a **naming prefix** (request proposes `caf-`), using GitHub + GitLab CLIs.
- **(M2)** No rule to **swap the project's submodule pointer** from the upstream third-party URL to our fork's SSH URL.
- **(M3)** No rule to **recursively fork nested third-party deps** of each forked CLI agent and rewire the fork's own nested `.gitmodules` to point at *our* forks. (Today §11.4.28(C) explicitly *exempts* third-party nesting — the new anchor must carve a refinement, not a contradiction: forked-into-our-org repos become own-org and thus fall back under §11.4.28(C)/§11.4.79 inclusion semantics.)
- **(M4)** No rule for a **regular fetch+merge of upstream `main`/`master` into our fork** (scheduled maintenance) — the request's core automation ask.
- **(M5)** No **reusable, decoupled, generic fork/update scripts** in `constitution/scripts/` (confirmed: `ls constitution/scripts/ | grep -iE "fork|merge|upstream|vendor"` → NONE; see Task 4).
- **(M6)** No statement of how the fork-and-maintain workflow **composes with §11.4.113** (absolute no-force-push) — upstream-merge-into-fork MUST be merge-onto-latest-main, never a history-rewriting overwrite.

**Tension to resolve in the anchor (not a blocker):** the request says we WILL commit small changes (gitignore, minor) into our forks (M1/M2), which appears to conflict with §11.4.28(B) "NEVER inject project-specific context INTO a submodule" and the project-manual "we do not control them." The new anchor resolves this cleanly: **once forked into our org the repo IS owned**, so §11.4.28 equal-codebase semantics apply, BUT the anchor must add a *decoupling guard* — fork edits are restricted to a closed allow-list (`.gitignore`, upstream-tracking config, CI-disable, governance-pointer files) and MUST NOT inject consuming-project context, so the fork stays generically reusable and cleanly mergeable with upstream (§11.4.28(B) preserved).

---

## Task 3 — DRAFT of the new constitutional anchor

**Chosen number: §11.4.142.** Rationale: highest existing anchor in both `constitution/Constitution.md` body and `constitution/CLAUDE.md` mirror is **§11.4.141** (grep, sorted numeric). Highest `CM-COVENANT-114-N` is `CM-COVENANT-114-141`. Next free = **§11.4.142** / `CM-COVENANT-114-142-PROPAGATION`. (§11.4.100 is RETIRED but its slot is not reusable — numbering is append-only per the §-slot-history convention.)

Draft below is in the SAME house style as §11.4.122–§11.4.141 (verbatim forensic anchor → mandate body → composes-with list → classification → propagation gate + recommended gate + paired mutation → canonical authority → no-escape-hatch line). The forensic anchor quotes the request file lines 14–15 verbatim.

---

> ### §11.4.142 — Fork-and-maintain mandate for third-party (not-owned) submodules + recursive nested-dependency forking (User mandate, 2026-06-10)
>
> **Forensic anchor — verbatim user mandate (2026-06-10, `helix_code_request_llms_access.txt` lines 14–15):**
>
> > "We MUST NOT change any codebase of CLI agents Submodules since we do not controll them! Maybe we should fork each in our own repository (with some proper naming prefix) under vasic-digital org using CLIs for GitHub and GitLab both. Then we use these repos as Submodule and we are able to commit and push (updates to git ignore files and other minor changes). On top of this WE MUST have bash scripts which will regulalry fetch and pull main (master) branches of all CLI agents main repos, and perform merging of the latest changes to our forks! We MUST support all recursive Submodules nested inside every single CLI agent we fork and in our forks use only our versions of these Submodules. All forked Submodules CLI agents use MUST BE maintained regularly as well by fethcing and pulling the parent (original) repos and merging all new codebase changes into our repository versions! This MUST be fully automatized and whole idea added as MANDATORY for all future and current work with 3rd party owned Submodules and its 3rd party Submodule (fully recursive!!!) dependencies! … We MUST make reusable bash scripts for forking, updating and other work … as reusable, decoupled, generic tools for any projects MUST BE part of constitution Submodule properly placed under proper directories (scripts dir for example)."
>
> **The mandate.** Any third-party (not under an owned org — `vasic-digital` / `HelixDevelopment` / the other §11.4.28(A) orgs) Git submodule that a consuming project needs to (a) carry minor non-upstreamable edits for (gitignore, remote/CI config, governance pointer), OR (b) keep building/running locally under our own version control, MUST be brought under our control by FORKING rather than by editing the upstream in place or carrying a detached patched copy:
>
> 1. **Fork into an owned org with a stable naming prefix.** Fork the upstream repo into `vasic-digital` (GitHub AND GitLab, both, via `gh` + `glab` CLIs) under a deterministic prefix (project-declared per §11.4.35; the consumer's default is `caf-` for "CLI-Agent Fork", e.g. `vasic-digital/caf-aider`). The prefix MUST be unique, lowercase-snake/kebab consistent with §11.4.29, and MUST encode that the repo is a maintained fork (so the catalogue and CodeGraph classifiers can tell forks from native own-org repos).
> 2. **Swap the consuming project's submodule pointer to the fork.** The project's `.gitmodules` entry MUST point at the fork's SSH URL (§3 SSH-only), never the upstream HTTPS/SSH URL. The fork becomes the project's source of truth for that dependency.
> 3. **Recursively fork nested third-party dependencies.** Every third-party submodule nested inside a forked repo MUST itself be forked into the owned org (same prefix discipline), and the fork's OWN `.gitmodules` rewired to point at *our* nested forks — "in our forks use only our versions of these Submodules." This REFINES §11.4.28(C): a forked-into-our-org repo IS now own-org, so its nested own-org (forked) deps follow §11.4.28(C) root-layout + §11.4.79 CodeGraph-inclusion semantics; the original "third-party nesting exempt" clause now applies only to dependencies NOT yet forked (the migration frontier), never as a permanent excuse to leave a build-essential third-party chain un-forked.
> 4. **Regular upstream-tracking maintenance (fully automated).** Each fork MUST be maintained by a scheduled job that: `git fetch` the original upstream's `main`/`master`, `git merge` (NEVER rebase/reset/`-s ours` that drops commits) the latest upstream changes onto the fork, resolve conflicts preserving our minor edits, and push to ALL owned-org mirrors. Recursive: the maintenance walks every nested fork too. Cadence: project-declared, weekly floor (per §11.4.45 / §11.4.80 status-digest cadence). A fork un-merged from its upstream for > 2 cycles with open dependent work is a release blocker (mirrors §11.4.80's staleness rule).
> 5. **Decoupling guard (preserves §11.4.28(B)).** Edits committed to a fork are restricted to a CLOSED allow-list — `.gitignore`, git remote / upstream-tracking config, CI-disable markers, governance pointer files, and other non-functional minor changes that DO NOT inject consuming-project context (no hardcoded host paths, asset names, project-private logic). The fork MUST stay generically reusable + cleanly mergeable with upstream. Functional capability changes follow the §11.4.74 extend-don't-reimplement / upstream-PR path, not a divergent in-fork patch.
> 6. **Anti-bluff.** A fork claimed "maintained" with no merge commits tracking the upstream tip, OR a `.gitmodules` still pointing at the un-forked upstream after a fork was created, OR a nested third-party chain left un-forked while the parent is forked, is a §11.4 PASS-bluff at the dependency-control layer. Maintenance runs MUST capture evidence (fetch output + merged upstream SHA + push result per mirror) per §11.4.5.
>
> Reusable, decoupled, project-agnostic tooling for (1)–(4) lives in `constitution/scripts/` (per the mandate's explicit "part of constitution Submodule … scripts dir") and is inherited by reference (per §11.4.80 codegraph-script precedent — referenced, NEVER copied). See the script specs in Task 4.
>
> **Composes with** §11.4.28 (own-equal-codebase + decoupling(B) + dependency-layout(C) — forks become own-org; this anchor REFINES the "third-party nesting exempt" clause) / §11.4.29 (fork name follows naming convention) / §11.4.36 (`install_upstreams` on the fork after clone) / §11.4.74 (catalogue-first — a fork is recorded `Catalogue-Check: fork <upstream>@<sha> → vasic-digital/caf-<name>`) / §11.4.79 (forked-into-own-org repos now INCLUDED in CodeGraph) / §11.4.80 (scheduled-update + status-doc automation precedent + > 2-cycle staleness blocker) / §11.4.113 (upstream-merge-into-fork is merge-onto-latest-main, force-push STRICTLY forbidden) / §2.1 (multi-mirror push of the fork) / §3 (SSH-only remotes) / §11.4.5 / §11.4.6 / §1.1.
>
> **Classification:** universal (§11.4.17) — a platform-neutral dependency-control discipline reusable by ANY project that consumes third-party submodules; the consuming project supplies its concrete fork prefix, target org, and dependency list per §11.4.35.
>
> Propagation gate `CM-COVENANT-114-142-PROPAGATION` (literal `11.4.142` across the consumer fleet) + recommended gates `CM-THIRD-PARTY-FORKED-NOT-UPSTREAM` (a `.gitmodules` entry for a build-essential third-party dep whose upstream is forkable resolves to an owned-org fork, not the upstream) / `CM-FORK-UPSTREAM-TRACKING-FRESH` (each fork has a recorded upstream-merge within the cadence window) / `CM-FORK-EDIT-ALLOWLIST` (fork diffs vs. upstream touch only the closed allow-list) + paired §1.1 meta-test mutations (point a `.gitmodules` at the un-forked upstream → first gate FAILs; backdate a fork's last-merge stamp → second gate FAILs; inject a functional change into a fork diff → third gate FAILs).
>
> **Canonical authority:** constitution submodule [`Constitution.md`](Constitution.md) §11.4.142. Non-compliance is a release blocker. No escape hatch — no `--edit-upstream-in-place`, `--skip-fork`, `--no-upstream-tracking`, `--leave-nested-third-party`, `--fork-without-prefix`, `--point-at-upstream-url` flag.

---

## Task 4 — Inventory of `constitution/scripts/` + specs for the NEW reusable scripts

### Current `constitution/scripts/` inventory (actually listed this session)

| Entry | Kind |
|---|---|
| `action_prefix_lib.sh` | action-prefix system (§11.4.140) |
| `codegraph_sync.sh`, `codegraph_update.sh` | CodeGraph automation (§11.4.78–§11.4.80) — **the by-reference precedent for the new scripts** |
| `enable_prompt_caching_check.sh` + `.md/.html/.pdf` | token-efficiency (§11.4.141) |
| `generate_agent_prefix_commands.sh`, `install_action_prefix.sh` | action-prefix wiring |
| `subagent_tier.sh`, `subagent_model_tiering.*` | model-tiering (§11.4.141) |
| `token_accounting.sh` | token-efficiency harness (§11.4.141) |
| `hooks/` | `action_prefix_expand.sh`, `guard-forbidden-commands.sh` (§11.4.109/§11.4.113 PreToolUse guard) |
| `workable-items/` | Go binary for §11.4.93 SQLite tracker |

**Confirmed ABSENT:** any `fork*`, `*merge*`, `*upstream*`, `*vendor*` script (`ls … | grep -iE "fork|merge|upstream|vendor"` → NONE). The three new scripts below are NEW work.

All three MUST be project-decoupled (CONST-051 / §11.4.28(B)): NO hardcoded project paths, org names, prefixes, or dependency lists — everything via env/args. They follow the §11.4.80 "inherited by reference, never copied" pattern and ship a companion `docs/scripts/<name>.md` per §11.4.18.

### Script 1 — `constitution/scripts/fork_third_party_submodule.sh`

- **Purpose:** Fork (or create-from-mirror if the upstream forbids forks) a third-party repo into a target owned org on BOTH GitHub and GitLab, with a naming prefix; configure the fork's upstream remote; optionally swap the consuming project's `.gitmodules` pointer.
- **Inputs (env/args, no hardcoding):**
  - `--upstream-url <ssh-or-https>` (the third-party repo)
  - `--target-org <org>` (default from `$HELIX_FORK_ORG`, e.g. `vasic-digital`)
  - `--prefix <str>` (default `$HELIX_FORK_PREFIX`, e.g. `caf-`)
  - `--hosts <github,gitlab>` (default both)
  - `--swap-gitmodules <path>` (optional; rewrites the entry to the fork SSH URL)
  - `--dry-run`
- **Outputs:** the fork's SSH URL(s) on stdout; an evidence record (fork creation API response + configured `upstream` remote) under `$HELIX_FORK_EVIDENCE_DIR/<prefix><name>/created.json`; exit 0 only when the fork exists on every requested host AND has its `upstream` remote set.
- **Anti-bluff Challenge:** bootstrap a throwaway test org/repo (or a sandbox fixture), run the script, assert via `gh repo view` / `glab repo view` that the fork exists on both hosts with the correct prefixed name AND `git remote -v` in the cloned fork shows the upstream — `gh` exit 0 alone is NOT proof (mirrors §11.4.80 "npm exit 0 ≠ working binary"). Paired §1.1 mutation: strip the prefix → assert the post-create name-check FAILs.

### Script 2 — `constitution/scripts/update_fork_from_upstream.sh`

- **Purpose:** Fetch the upstream `main`/`master`, merge it onto the fork's latest main (merge-onto-latest-main per §11.4.113, NEVER force/rebase/reset), resolve via configured strategy, push to ALL owned mirrors. Records the merged upstream SHA + per-mirror push result.
- **Inputs:** `--fork-dir <path>` OR `--fork-url <ssh>`; `--upstream-branch <main|master>` (auto-detect default); `--conflict-policy <abort|operator>` (default `abort` — never silent `--ours`/`--theirs`, per §11.4.6 + §11.4.113 step 3); `--recurse <true|false>` (walk nested forks). `$HELIX_FORK_MIRRORS` lists push remotes.
- **Outputs:** captured `git fetch` stdout, the merged upstream SHA, the merge commit SHA, per-mirror `git push` results, appended to a `docs/codegraph/Status.md`-style ledger (`docs/forks/Status.md` proposed); a `.regenerated/<fork>.ok` freshness stamp for the freshness gate. Exit non-zero on any conflict (so an operator resolves), on any dropped-commit detection, or any mirror push rejection.
- **Anti-bluff Challenge:** create a fork whose upstream has a NEW commit the fork lacks; run the script; assert `git merge-base --is-ancestor <upstream-tip> <fork-HEAD>` is now true (upstream genuinely integrated) AND no commit was dropped (`git rev-list` count monotonic) AND the freshness stamp is fresh. Paired §1.1 mutation: replace the merge with `git reset --hard upstream/main` (a history-rewrite) → assert the no-dropped-commit guard FAILs (composes §11.4.113 `CM-NO-FORCE-PUSH-ABSOLUTE`).

### Script 3 — `constitution/scripts/resolve_recursive_fork_deps.sh`

- **Purpose:** Recursively walk a forked repo's `.gitmodules`, identify every nested third-party submodule, fork each (delegating to Script 1), and rewrite the fork's own `.gitmodules` so nested entries point at OUR forks ("in our forks use only our versions"). Emits the dependency graph as an audit record (analogue of §11.4.31 `.helix-manifest.yaml`).
- **Inputs:** `--root-fork-dir <path>`; `--target-org`, `--prefix`, `--hosts` (passed through to Script 1); `--max-depth <n>` (cycle guard); `--exclude <glob>` (deps deliberately left upstream, e.g. an unforkable vendor blob — must be justified). `$HELIX_FORK_ORG` etc. inherited.
- **Outputs:** the rewritten `.gitmodules` (staged, not auto-committed — §11.4.84 quiescence), a `forks-manifest.yaml` listing `upstream → fork` per node + depth, and an evidence directory. Exit 0 only when every non-excluded nested third-party dep resolves to an owned-org fork.
- **Anti-bluff Challenge:** fixture repo with a 2-level nested third-party chain; run the script; assert every `.gitmodules` entry at every depth (parse recursively) points at `$HELIX_FORK_ORG/<prefix>*` and that each referenced fork actually exists on both hosts. Paired §1.1 mutation: leave one nested dep pointing at its upstream → assert the recursion-complete gate FAILs (composes `CM-THIRD-PARTY-FORKED-NOT-UPSTREAM`).

**Cross-cutting (all three):** §11.4.18 in-source doc block + `docs/scripts/<name>.md`; §11.4.67 `sh -n` parseable; §11.4.81 cross-platform (`gh`/`glab`/`git` behave on macOS + Linux — `case "$(uname -s)"` only if a platform-specific path appears); §11.4.10 credentials via env, never logged; §11.4.65 doc-export siblings.

---

## Task 5 — CONST-049 / §11.4.26 workflow that MUST be followed when later EDITING the constitution

Source read: `constitution/CLAUDE.md` §11.4.26 block (7-step pipeline) + root `CLAUDE.md` CONST-049 anchor. When the team later lands §11.4.142 (and its CLAUDE.md/AGENTS.md/QWEN.md mirrors), execute IN ORDER:

1. **Fetch + pull FIRST** — inside `constitution/`: `git fetch` every remote, then `git pull --ff-only` (or `--rebase` if non-FF; never `--strategy=ours`/`--allow-unrelated-histories` unauthorised). Submodule MUST be at upstream tip before any edit. **Plus:** the current local checkout is a DETACHED HEAD at `60e2d66` — `git checkout main` (canonical branch) before editing/committing, or the commit will be unreachable.
2. **Apply the change** — classify per §11.4.17 (this is **universal** — goes in the constitution submodule, not the consumer). Cite the verbatim mandate (request lines 14–15) in the commit message. Add the §11.4.142 block to `Constitution.md` + mirror blocks to `constitution/CLAUDE.md` + `constitution/AGENTS.md` + `constitution/QWEN.md`.
3. **Validate before commit** — run `meta_test_inheritance.sh` (or equivalent); `grep -rn '^<<<<<<< \|^=======$\|^>>>>>>> '` returns empty; verify Constitution + CLAUDE + AGENTS + QWEN cross-reference §11.4.142 consistently; the new `CM-COVENANT-114-142-PROPAGATION` literal `11.4.142` is present in every governed file.
4. **Commit + push to ALL upstreams** — stage ONLY the governance files (NEVER `git add -A` inside the submodule, per §11.4.30); push to EVERY configured remote across GitHub + GitLab + GitFlic + GitVerse (a one-upstream landing is a §2.1 + §11.4.26 violation). Push is fast-forward-only (§11.4.113 — force-push STRICTLY forbidden, no exception).
5. **Conflict resolution** — if `pull --ff-only` reports non-FF, merge carefully onto latest main (§11.4.113 step 2–4), preserve union of governance content, no clause dropped, re-classify, re-validate. Force-push to "make conflicts go away" is FORBIDDEN.
6. **Post-merge validation + cascade** — `git submodule update --remote --init` in the consumer, then re-run the cascade verifier (`scripts/verify-governance-cascade.sh` / CONST-047) confirming §11.4.142 reached every owned submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md / QWEN.md. Close any cascade gap in the same change-window.
7. **Bump consuming-project pointer** — advance the `.gitmodules`-tracked `constitution` submodule pointer to the new constitution HEAD in the SAME commit as the cascade work (out-of-sync pointer = §11.4.26 violation). Also bump `constitution/CLAUDE.md` metadata `Revision` (currently 23) + `Last modified` per §11.4.44 — and note the current metadata-drift (table says 2026-06-07 while body carries 2026-06-09 §11.4.140/141) should be corrected in the same pass.

Additional binding constraints for the edit: §11.4.99 (cross-reference latest `gh`/`glab`/`git` fork-API docs before publishing the script instructions, `## Sources verified` footer) and §11.4.125/§11.4.134 (code-review gate iterate-until-GO before the build/landing).

---

## Executive summary (10 lines)

1. Constitution submodule local HEAD = `60e2d66` (DETACHED, clean), carrying the newest anchors **§11.4.140 + §11.4.141** (2026-06-09); currency vs. upstreams is NOT knowable without a fetch (not performed per instruction).
2. `constitution/CLAUDE.md` metadata = Revision 23 / Last-modified 2026-06-07 — lags the body (which DOES contain the §11.4.140/141 mirrors); minor §11.4.44 metadata-drift to fix on next edit.
3. The DETACHED-HEAD state means a future edit MUST `git checkout main` inside the submodule before committing.
4. **Highest existing anchor = §11.4.141; highest gate = `CM-COVENANT-114-141`. Next free = §11.4.142 / `CM-COVENANT-114-142-PROPAGATION`** (chosen).
5. Existing third-party governance treats not-owned submodules as **vendored / untouched / exempt**: §11.4.74 (catalogue, own-org only), §11.4.28(C) ("third-party nesting exempt"), §11.4.79 (excluded from CodeGraph), §11.4.29/§11.4.65 (keep upstream names, excluded from export).
6. **ABSENT (searched "fork", "upstream merge", "not control"):** NO anchor and NO script for forking third-party submodules, swapping pointers to our fork, recursively forking nested deps, or scheduled upstream-merge maintenance.
7. Drafted **§11.4.142 — Fork-and-maintain mandate** (forensic anchor = verbatim request lines 14–15) with a 6-clause body, a decoupling guard preserving §11.4.28(B), explicit §11.4.113 no-force-push composition, propagation + 3 recommended gates + paired mutations.
8. `constitution/scripts/` has NO fork/merge/upstream tooling today (confirmed by grep); specified **3 new reusable, CONST-051-decoupled scripts**: `fork_third_party_submodule.sh`, `update_fork_from_upstream.sh`, `resolve_recursive_fork_deps.sh` — each with env/arg inputs, evidence outputs, and an anti-bluff Challenge + §1.1 mutation; they follow the §11.4.80 "inherited-by-reference" precedent.
9. The fork-edit allow-list (gitignore / remote-config / CI-disable / governance pointers only) reconciles the request's "we commit minor changes to our forks" with §11.4.28(B) "never inject project context."
10. When landing the anchor, follow the **CONST-049 / §11.4.26 7-step pipeline** (fetch+pull first → classify universal → validate no-conflict-markers → commit governance-only + push ALL four upstreams fast-forward-only → careful conflict merge → cascade-verify reaches every owned submodule → bump consumer `.gitmodules` pointer + Revision in the same commit).

**Chosen new anchor number: §11.4.142** (gate `CM-COVENANT-114-142-PROPAGATION`).
