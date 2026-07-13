# Discovery / Hardening Sweep — Conductor Ledger (claude2, §11.4.147)

Branch: feature/helixllm-full-extension | HEAD 6bf1be7a | started 2026-07-12
Mandate (§11.4.126, operator CRITICAL): deep analysis of unfinished/untested/known-issues
→ phased plan (phases/tasks/sub-tasks) → workable-items in DB → multi-track kickoff, endless loop.

## Baseline (first-hand evidence)
- Remotes: main==feature==6bf1be7 on github/gitlab/origin/upstream — merged + pushed.
- Queue: 308 rows = **203 DISTINCT items** (105 doubled by `representation` table+section — BY DESIGN, not dup bug), ALL terminal (0 open). Families: HXC 116, FIX 72, VEN/HXV/HXA 3 ea, HXQ/HXL 2, PAN/OPS 1.
- Core build `go build ./cmd/... ./internal/...` = EXIT 0. Sanctioned `make verify-compile` = `go build -tags=nogui ./...`.
- Full `go build ./...` FAILS (X11/GL) — confined to applications/{desktop,aurora_os,harmony_os}+tests/ui (Fyne/go-gl need X11+gl.pc). Env dep, NOT core-code defect.
- Anti-bluff scan §3.4: 0 hits. Mock-in-production §11.4.50A: 0.
- Test-type dirs all populated w/ real funcs: integ161 e2e150 sec102 perf28 mem15 reg14 ui10 ux9 stresschaos7 automation7 ddos6 scaling6 unit34.

## Findings pre-fanout
- F-BUILD-GUI (Med): full-GUI build needs X11/GL dev libs — undocumented env dep (§11.4.77 regen-mechanism missing).
- F-DB-DESC (Low): 36 items desc <50 chars (§11.4.171).
- F-DB-LOC  (Low): 41 `section` rows current_location=Issues while status terminal (§11.4.19 drift).

## Discovery fan-out — subagents write to <scratch>/discovery/
- D1 test-execution reality       -> DONE — build clean, unit 153/153, 12/13 dirs PASS. Findings: F-D1-01(Med) tests/regression FAIL CORSHeaders expects "*" got "" (critical_paths_test.go:733); F-D1-02(Med-High) providers huggingface+together [no test files]; F-D1-03(Low-Med) cmd/security_scan no tests; F-D1-04(Med) tests/memory 12/15 SKIP no-server; F-D1-05(Med) tests/automation 5/6 opt-in env-gated; F-D1-06(Low) integration tests behind //go:build integration invisible to default run (PASS with tag).
- D2 owned-submodules health      -> PAUSED — launched detached 10-min script over 66 submodules, stopped w/o finalizing; RESUME to collect (agent a56a6e5c21768a1f3, §11.4.147)
- D3 feature wiring/runtime-sig    -> DONE — Findings: F-D3-01(HIGH) CONST-040 caps MCP/LSP/ACP/RAG/Skills/Plugins declared verifier-sourced but only SupportsEmbeddings impl in verifier/types.go; F-D3-02(HIGH) RAG zero impl in internal/ (RECONCILE: submodules/rag exists -> integration gap?); F-D3-03(HIGH) ACP absent everywhere; F-D3-04(Med) MCP/LSP/Skills/Plugins real but not verifier-gated (CONST-040 sourcing unmet). POSITIVES: all 10 CONST-039 providers real HTTP; /api/v1/llm/generate|stream wired e2e; NO BLUFF-001/2/3 regression. F-D3-08 UNCONFIRMED deep e2e of MCP/LSP/Skills/Plugins.
- D4 closed-item spot-audit        -> DONE — 25 sampled, 24 verified-present, 0 not-found (closed!=working REFUTED for sample). Findings: F-D4-01(HIGH) HXC-003 CONST-046 gate portability-broken (abs-path baseline -> 19098 false hits, exit1, §11.4.108/§11.4.177); F-D4-02(Med) §11.4.19 drift 11 items in Issues absent Fixed (HXC-013/014/014b/015/018/019/024/025/026/027/028); F-D4-03(Low) obsolete_details empty §11.4.90; F-D4-04(Low) 31 FIX rows desc<50 §11.4.171; F-D4-05(Low) severity blank 79 Features §11.4.132; F-D4-06(Low) FIX#69 in submodules/llm_orchestrator. LEAD open: HXC-029 JWT-mint (from D5) not in sample.
- D5 challenges + helix_qa reality -> DONE — CLEAN: both suites REAL, wired, actually-executed (real 200-OK qa-results, 41 e2e dirs, embedded server feature). Only F-D5-06(low): scripts not re-run this session. LEAD: helix_qa docs an HXC-029 JWT-mint gap -> chase in synthesis.

## Findings pre-fanout (added)
- F-DOC-RUNBOOK (Med): docs/guides/MULTITRACK_WORKTREE_RUNBOOK.md referenced by config + multitrack.sh help but MISSING (§13.1/§11.4.99 doc drift).

## Tooling interfaces (VERIFIED — for the materialize + kickoff phases)
- workable-items binary: `cd constitution/scripts/workable-items && go run ./cmd/workable-items <sub>`
  - `add <type> <sev> --db <p> --title <T> --description <D> [--id --prefix HXC --created-by --assigned-to]`  -> new Queued item in Issues
  - `group add <group_id> <destination> <priority> --title <T> [--state s]`  -> logic_group (per-track domain, §11.4.176/191)
  - `update|reopen|block|close|obsolete-details|diary add|export|report|validate|version-tags`
  - validate OK on live-DB copy (308 items). ALWAYS operate on a §9.2 COPY when probing; commit DB after real add (§11.4.95).
  - §11.4.171: descriptions MUST be >=50 chars human-readable.
- multitrack: `constitution/scripts/multitrack/multitrack.sh {status|up}` (up never mounts; exit20=operator-blocked).
  Feature tracks 2-4 need /mnt/track2-4 (ATMOSphere-owned) already mounted; check `multitrack.sh status` at kickoff.
  Work dispatch engine: multitrack_alias_orchestrator.sh + multitrack_supervisor.sh + multitrack_claim.sh + multitrack_work_binding.sh.

## Disambiguations (FACT, conductor-verified)
- CORS F-D1-01: STALE TEST (§11.4.120). Server hardened to explicit allowlist (cfg.Auth.CORSAllowedOrigins, "never wildcard+credentials", doc.go:101-104); test asserts old insecure "*". FIX=reconcile test to secure behavior, NOT re-add "*".
- RAG F-D3-02: ORPHANED submodule (HIGH). submodules/rag=digital.vasic.rag, 0 inner imports, no internal/rag. Mandated CONST-040 capability entirely UNWIRED. Item=integrate+wire+verifier-flag.
- ACP F-D3-03: effectively absent (1 doc-mention). Needs real impl/scoping (HIGH).
- HXC-029: Completed §11.4.98 sweep Task; "JWT-mint gap" is a documented helix_qa helixcode-task-workflow bank limitation blocking full automation -> own hardening item (Med).

## Draft phase skeleton (pre-D2)
- Phase A Governance-gate/decoupling: HXC-003 CONST-046 gate portability (F-D4-01 HIGH), multitrack runbook (F-DOC-RUNBOOK).
- Phase B CONST-040 capability completion: verifier capability sourcing (F-D3-01/04 HIGH), RAG integration (F-D3-02 HIGH), ACP (F-D3-03 HIGH).
- Phase C Test-coverage hardening: CORS stale test (F-D1-01), provider tests huggingface/together (F-D1-02), memory/automation infra-gated (F-D1-04/05), security_scan tests (F-D1-03), HXC-029 JWT-mint QA gap.
- Phase D Tracker/DB hygiene: §11.4.19 move drift (F-D4-02), obsolete_details (F-D4-03), descriptions (F-D4-04), severity backfill (F-D4-05).
- Phase E Build-env/docs: GUI X11 regen-mechanism §11.4.77 (F-BUILD-GUI), runbook §13.1 (F-DOC-RUNBOOK).
- Phase F Owned-submodules (§11.4.28): TBD from D2.

## MATERIALIZED BACKLOG (§11.4.93, live DB, §9.2 backup @ scratch/db_backups/) — validate OK 324 items
Finding -> ATM-id map (all Queued/Issues):
- HXC-115 F-D4-01 CONST-046 gate portability (Bug/High)   <- S3 fixing
- HXC-116 F-DOC-RUNBK multitrack runbook (Task/Med)        <- S4 fixing
- HXC-117 F-D3-01 verifier capability sourcing (Bug/High)
- HXC-118 F-D3-02 RAG submodule integration (Feature/High)
- HXC-119 F-D3-03 ACP implementation (Feature/High)
- HXC-120 F-D1-01 CORS stale test (Bug/Med)               <- S1 DONE (evidence fixes/S1_cors_evidence.md)
- HXC-121 F-D1-02 provider tests hf+together (Task/Med)   <- S2 DONE 94.5% cov (fixes/S2_provider_tests_evidence.md)
- HXC-122 F-D1-04 memory/automation infra-run (Task/Med)
- HXC-123 F-D1-03 security_scan tests (Task/Low)
- HXC-124 F-D5-JWT HelixQA JWT-mint gap (Bug/Med)
- HXC-125 F-D1-06 integration-tag visibility (Task/Low)
- HXC-126 F-D4-02 tracker move-drift 11 items (Task/Med)
- HXC-127 F-D4-03 obsolete_details empty (Task/Low)
- HXC-128 F-D4-04 short descriptions 36 (Task/Low)
- HXC-129 F-D4-05 severity backfill 79 (Task/Low)
- HXC-130 F-BUILD-GUI GUI build-env doc (Task/Med)        <- S4 fixing

## FIX STREAMS (wave 1, shared checkout, no self-commit — conductor reviews+commits)
- S1 HXC-120 CORS  -> DONE clean (RED expected"*"got"" -> GREEN ok 1.772s; 1 file; §11.4.120 no wildcard reintroduced)
- S2 HXC-121 tests -> DONE clean (hf 92.6% + together 96.4%, §1.1 mutation-proven; 2 new _test.go)
- S3 HXC-115 gate  -> DONE clean (RED 19098-NEW/exit1 -> GREEN 0-NEW/exit0; --repo-root normalizeReportPaths §11.4.177; 2 new §11.4.135 guards; 12/12 tests; §1.1 planted-violation still caught). Files: scripts/audit_const046/{main.go,main_test.go,.baseline.json,.baseline.json.gz}, scripts/audit-const046-hardcoded-content.sh
- S4 HXC-116+130 docs -> DONE clean (RUNBOOK cross-checked vs 9 real scripts; GUI doc real captured error + verify-compile GREEN + honest UNCONFIRMED on X11-ext pkgs; explicitly did NOT run full GUI build). Files: docs/guides/{MULTITRACK_WORKTREE_RUNBOOK.md,GUI_BUILD_ENV.md}. Sub-finding: MULTITRACK_ACTIVATION.md + docs/scripts/*.md still missing (future low item).
- S5 HXC-123 security_scan tests -> RUNNING (agent ac9f2d9933e0d8a65)
- D2 submodules (Phase F) -> RUNNING

## INDEPENDENT REVIEW (§11.4.142, conductor=opus vs sonnet implementers) — wave-1
- S1 HXC-120 CORS  -> GO (asserts secure allowlist + default-deny; NotEqual "*" guard; no wildcard reintroduced)
- S2 HXC-121 tests -> GO (files present; 94.5% cov; §1.1 mutation-proven non-tautological)
- S3 HXC-115 gate  -> GO (baseline 0 abs-leaks, 19098 entries preserved repo-relative; normalizeReportPaths correct; §1.1 enforcement preserved)
- S4 HXC-116+130 docs -> GO (accurate vs real scripts; honest UNCONFIRMED/boundaries)
- S5 HXC-123 -> review pending completion

## Pending commit set (commit at quiescence, §11.4.19 close + §11.4.135 guard + §11.4.88 bg-push no-force §11.4.113):
- HXC-120 CORS: helix_code/tests/regression/critical_paths_test.go
- HXC-121 provider tests: helix_code/internal/llm/providers/{huggingface,together}/client_test.go (new)
- HXC-115 gate: scripts/audit_const046/* + scripts/audit-const046-hardcoded-content.sh
- backlog: docs/workable_items.db (+16) -> run `export` to sync trackers (also resolves HXC-126 move-drift) then commit
- (S4 docs, S5 tests pending)

## WAVE-1 COMMITTED + PUSHED
- commit 3d3f3326 on feature/helixllm-full-extension (parent 6bf1be7). Pre-commit §11.4.65 gate PASSED.
- pushed ff/no-force to github+gitlab+upstream (all @3d3f332). Main untouched (§11.4.167). §9.2 backup @ scratch/db_backups.
- closed HXC-115/116/120/121/123/130 (Issues->Fixed, evidence docs/qa/discovery_hardening_20260711T212548Z/). Queue: 10 Queued left.

## MULTITRACK: all 4 tracks MOUNTED+ACTIVE. multitrack-up spawns tmux headless claude workers on FREE native aliases only (claude1; claude3/4 held by ATMOSphere §11.4.174); conductor=claude2 never launches. Using conductor-driven Agent-subagents (equivalent, coordinatable, §11.4.101 safe path) + organizing work into logic_groups.

## WAVE-2 (dispatched)
- W2-A CONST-040 capability design (HXC-117/118/119, §11.4.145 design-first, READ-ONLY) -> DISPATCHED
- W2-B HXC-125 integration-tag visibility (helix_code/Makefile) -> DISPATCHED
- Conductor DB-hygiene batch NEXT: HXC-127 obsolete_details, HXC-128 descriptions, HXC-129 severity, HXC-126 move-drift + logic_group creation (single-owner DB).
- D2 submodules (Phase F) -> still RUNNING.

## WAVE-2 status
- W2-A CONST-040 design (HXC-117/118/119) -> RUNNING (agent af9eb42b5663012ed)
- W2-B HXC-125 integration-tag -> DONE clean: test-integration-tag target, 123 PASS/0 FAIL/29 honest-SKIP. helix_code/Makefile. READY to close HXC-125. (bonus: tests/README.md refs nonexistent run_tests.sh -> future low item)
- W2-C HXC-128/129 enrichment -> RUNNING (agent a61055cf877e6bcb5)
- D2 submodules (Phase F) -> RUNNING (agent a56a6e5c21768a1f3)

## Conductor DB-hygiene findings (probed on §9.2 copies — live UNTOUCHED)
- HXC-127 obsolete_details: READY — `wi obsolete-details HXC-044 --since 2026-06-09 --reason not-reproducible --superseding none --evidence docs/qa/HXC-044/evidence.md` (facts from body, validate OK on copy).
- HXC-126 move-drift: DEFERRED. Raw SQL current_location UPDATE => 83 validate violations (doc_segments §11.4.93/ATM-627 not maintained). `wi close` maintains doc_segments but RESETS closure_date (fidelity loss) on 40 items. Low-Med + risky/lossy => keep Queued, document; correct fix = a date-preserving relocate (tool enhancement) or accept close-with-date-reset later. NOT attempted on live.

## !!! CRITICAL FINDING + DECISION (overnight, operator away, ZERO-RISK mandate) !!!
FINDING F-DBTOOL (NEW, untracked in DB because tooling is the problem): the workable-items
edit->sync path is UNRELIABLE. On the untouched clean committed DB, `sync db-to-md` AND `export`
BOTH round-trip cleanly (diff=0). But applying `close` / `obsolete-details` to certain items
(notably HXC-044, whose committed MD body=33796B vs DB body_md=663B — a PRE-EXISTING inconsistency)
then re-syncing yields 176-190 DB<->MD diffs (status/type mismatches, items present-in-DB-absent-in-MD).
Wave-1's 6 closes happened to stay clean; wave-2's hygiene (110 updates + obsolete-details + closes)
did NOT. Root: edit ops desync body_md/doc_segments vs items columns for some items; sync can't reconcile.
DECISION: reverted ALL wave-2 DB+tracker changes to clean HEAD (git checkout HEAD -- docs/workable_items.db docs/Issues* docs/Fixed*).
=> NO workable-items DB changes for the rest of this session. Item completion tracked via commit msgs + this ledger + resumption doc.
=> Deferred (stay Queued in committed DB, honest): HXC-126 (doc_segments), HXC-127/128/129 (hygiene — W2C proposals saved at scratch/discovery/fixes/W2C_hygiene_proposals.jsonl for careful re-apply once F-DBTOOL is fixed).
=> HXC-125 (integration-tag) SOURCE fix is real + will be committed, but the item stays Queued (not DB-closed) — honest.
F-DBTOOL itself is a real BUG to track once the tooling is fixed.

## OVERNIGHT SOURCE-ONLY PLAN (ABSOLUTE PRIORITY = most stable build)
Commit only build-relevant source, each: review -> `make verify-compile` FULL build green -> tests green -> commit -> push. Never commit DB/trackers.
- W2-B Makefile test-integration-tag -> DONE (123 PASS)
- W3-117 verifier capability fields -> DONE clean (RED->GREEN, 4 tests, build green, additive)
- W2-A design doc docs/research/const040_capability_model_20260712/ -> DONE (md-only, hook-exempt new spec)
- W3-119 ACP scaffold -> RUNNING (agent abc36bf29cbefe2bf; owns go.mod/go.sum/internal/acp/cmd-cli/acp_cmd + i18n bundle)
- W3-118 RAG -> HOLD until W3-119 go.mod committed (avoid go.mod collision)
- D2 submodules (Phase F) -> RUNNING (agent a56a6e5c21768a1f3)

## LANDED + PUSHED tonight (all mirrors, build-verified, zero-bluff)
- 3d3f3326 wave-1 (6 fixes + 16-item plan) | ca76e14b wave-2/3 source (HXC-117/119/125 + design) | 52b41448 handoff | 54a76c3c HXC-118 RAG
- STABILITY CONFIRMED @54a76c3c: `make verify-compile` GREEN + full unit sweep = 159 ok pkgs, 0 FAIL, 0 build-failed. No regressions from any commit.
- Morning handoff: docs/MORNING_RESUMPTION_20260712.md (committed+pushed).

## Streams running
- F-DBTOOL gated fix (copies-only, no live DB/commit) -> agent aac605d831b19d12d
- D2 submodules (Phase F) -> agent a56a6e5c21768a1f3 (long-running/unknown)

## OPERATOR RETURNED + answered 4 unblock questions -> executing (BACKGROUND :: subagent-driven)
Answers: F-DBTOOL=land fully; CONST-040=advance all safe-defaults; infra=boot+full suite; HXC-108=Operator-blocked.
- F-DBTOOL-1 LANDED: constitution 3302587 (representation-scoping fix + §11.4.115/135 regression test) pushed ALL 7 mirrors; helix_code 7ce471ee (pointer bump + DB hygiene subset + trackers) pushed 3 mirrors. Closed HXC-125/127/129 + 79 severities. DB diff=0, validate OK.
- F-DBTOOL-2 (NEW finding, deferred): `update --description` bulk path desyncs body_md vs rendered/columns (38 body + 31 absent + 12 status/type = 81 diffs). HXC-128 (descriptions) + HXC-126 (move-drift) stay Queued until fixed. (Severity + close paths are clean.)
- HXC-108 -> mark Operator-blocked (pending, conductor — test block on copy first per F-DBTOOL-2 caution).
STREAMS: W4-117 verifier-caps (ab22d6f3) | W4-118 RAG retriever+wire (a316db2f) | W4-119 ACP turn-gen (ad53e552) | infra full-suite (a605a6b4). Each gated on full make verify-compile before commit.

## UNBLOCK PROGRAM (operator returned) — LANDED
- F-DBTOOL-1: constitution 3302587 (all 7 mirrors) + helix_code 7ce471ee -> closed HXC-125/127/129 + 79 severities, diff=0.
- CONST-040 Phase-2: helix_code edbd5a49 -> HXC-117 verifier caps (real LLMsVerifier path, honest live-not-emitting-yet) + HXC-118 real Ollama RAG retriever + default-OFF wire + HXC-119 ACP->GenerateStream. Full build green. Items stay Queued (Phase-2, not fully user-visible).
- Infra full-suite: booted rootless-podman 17 containers, teardown clean; security 272 PASS, memory 3, integ 189-pkg-ok. Evidence committed docs/qa/infra_fulltest_20260712/.
- NEW infra defects: INFRA-1 Azure nil-ptr SIGSEGV (real bug), INFRA-2 stale integ test, INFRA-3 e2e -all flag, INFRA-4 cognee auth drift, INFRA-5 Dockerfile.test path, HXC-124 token-field mismatch (root-caused, consumer-side fix).
- HXC-108: NO action — it's a Completed Video-QA task, NOT Aurora/Harmony (old prompt wrong).

## STREAMS RUNNING
- F-DBTOOL-2 fix (a3cd75a74828a3901) — unblocks HXC-128/126.
- W5 infra-defects fix (a45ef6c44d83dd361) — INFRA-1 Azure panic + INFRA-2 stale test + INFRA-3 e2e -all.

## STILL DEFERRED / follow-up items
HXC-128 descriptions + HXC-126 (pending F-DBTOOL-2), HXC-124 (token-field consumer fix), INFRA-4 cognee, INFRA-5 Dockerfile.test + server:8080 test-config, HXC-117/118/119 full user-visibility. HXC-108/Aurora needs operator confirm on whether to open a new item.

## FINAL RESTING STATE (overnight, all safe work complete)
- HEAD 19dd018a, pushed to all 3 mirrors. 6 commits tonight: 3d3f3326 / ca76e14b / 52b41448 / 54a76c3c / d0be2462 / 19dd018a.
- BUILD MAXIMALLY STABLE (proven): make verify-compile GREEN + unit sweep 159 ok/0 fail + broad suite (integration+security+regression) 7 ok/0 fail. HXC-120 CORS fix confirmed at HEAD.
- DB clean + in-sync (validate OK 324). Working tree: only `M constitution` (preserved F-DBTOOL fix for operator) + 3 pre-existing dirt.
- F-DBTOOL: root-caused + fixed + regression-tested + copy-validated diff=0 (uncommitted in constitution WT; constitution 1-behind remote c793ba6). Morning steps in docs/MORNING_RESUMPTION_20260712.md.
- D2 (submodules/Phase F): status unknown/likely-timed-out — Phase F unverified this session (owned submodules were §12.10-verified in the prior rev15 session).

## Legitimately BLOCKED remaining work (§11.4.94(A) idle-when-blocked; NOT done overnight per ZERO-RISK):
- F-DBTOOL commit + DB closures (HXC-125/127/128/129): operator, cross-repo canonical-root push (highest blast radius).
- HXC-117/118/119 Phase 2+: need architecture decisions (verifier capability-data source; concrete RAG retriever+embedding; ACP turn-gen + permission-map security) — not safe unattended.
- HXC-122 (infra boot), HXC-124 (submodule+security), HXC-126 (needs F-DBTOOL).
=> Endless loop legitimately idles: every remaining item is externally blocked; forcing them violates the zero-risk/most-stable mandate. Responsive to D2 if it completes.
-> workable-items add (DB, §11.4.93/148/171) + group add per track -> multitrack kickoff (§11.4.187).
