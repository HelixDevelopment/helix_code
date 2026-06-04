<!-- AUTHORITATIVE ADDENDUM — generated 2026-06-04 from read-only discovery Wave 2 (wf_e0bd1a2a-792, 12 modules). Pairs with CONSOLIDATED_UNFINISHED_WORK_AND_PLAN.2026-06-04.md. -->

# Consolidated Report — Addendum: previously-un-assessed modules (2026-06-04, Wave 2)

This addendum synthesizes the 12 Wave-2 module assessments that were rate-limited in the prior discovery round. All findings are sourced from the assessments above. "unverified" is preserved verbatim wherever the assessor flagged it. No runtime claims are asserted beyond what an assessor explicitly captured.

## A. Newly-assessed module status

| Module | Builds | Broken/disabled? | Dead code | Top unfinished | Evidence |
|---|---|---|---|---|---|
| challenges | **Partial** — `go build ./...` FAILS | `pkg/userflow/` (118 .go) unbuildable: missing `digital.vasic.containers` dep, 0 `replace` directives | Whole `pkg/userflow/` effectively dead in this checkout | `pkg/userflow/container_infra.go:8-13` unresolved containers import; `cmd/userflow-runner` blocked transitively | `cd submodules/challenges && go build ./...` → missing-module error; 14 non-userflow pkgs build+test PASS (`assertion 5.184s`, `runner 3.599s`) |
| security | **Yes** | None | None | No blocking items; no committed vuln-scan config | `go build ./...` BUILD_EXIT=0; `go vet ./...` exit 0; 343 test funcs compile (`ok ... [no tests to run]`) |
| llms_verifier | **App module: yes / outer+meta: no** | Outer module won't build offline (missing go.sum entries) | `*.go.backup`, `backup/scoring_example.go` orphan main, committed `*.db.backup_*`, `server.pid` | Dead `replace digital.vasic.llmprovider => ../../LLMProvider` (target MISSING, no importer); empty metadata "for now" | inner `go build ./...` exit 0, `go vet` exit 0; outer `go build ./...` → missing go.sum entries |
| helix_agent | **Yes** | None (0 bare skips of 1172) | No `.disabled`/`.bak` in internal/cmd/pkg | `internal/tools/tool_executors.go:292,792` stubs; `internal/optimization/guidance` (22 markers) | `go build -o /tmp/... ./cmd/helixagent/` → 96 MB binary; `go vet` exit 0 |
| doc_processor | **No** — committed merge-conflict markers | `pkg/i18n/translator.go` + `translator_test.go` have unresolved `<<<<<<<`/`>>>>>>>` markers (committed at HEAD `7750140`) | None observed in buildable pkgs | `pkg/i18n/translator.go` (lines 4,14,32,37,58,75,80,89,98); binary blocked (imports i18n) | `go build ./...` → `translator.go:4:1: expected 'package', found '<<'`; 7 non-i18n pkgs test PASS |
| llm_orchestrator | **Yes** | None | None | `pkg/agent/gemini_agent.go:126` stdin-pipe mode follow-up; `go.mod:18` case-mismatch replace | `go build ./...` EXIT=0; `go vet` exit 0; `go test -race ./pkg/agent/` PASS (14.3s) |
| llm_provider | **Yes** | None | `pkg/discovery/discovery.go:162-190` deprecated Tier-3 hardcoded-fallback still consulted at runtime | CONST-036 fallback sweep (~40 providers); zen provider needs external OpenCode CLI | `go build ./...` exit 0; `go vet` exit 0; core pkgs PASS under `-race -short` |
| vision_engine | **Yes (default) / partial (vision tag)** | `-tags vision` build fails ONLY for missing host OpenCV (expected, isolated behind `//go:build vision`) | None — build-tag split intentional | OpenCV vision path untested on this host (toolchain-gated, not code-gated) | `go build ./...` EXIT=0; `go test -race ./pkg/remote/` ok 1.507s; 8 vision providers confirmed real HTTP |
| assets | **n/a** (brand dir) | None | None | `.gitignore` derivative patterns reference a non-existent generator; tautological mutation self-test | Challenge ran: PASS both PNGs + governance files; `MUTATION_TEST=1` rejected sentinel |
| github_pages_website | **Partial** (static site = serve; not run at runtime) | HTML validation soft-skips when html5validator absent (effectively unvalidated) | `courses/player.js:264` quality-switch stub; `certificate.html:241` PDF stub | `package.json:16-18` docker-compose scripts with no compose file; provider claims substantiated | `node --check` all JS OK; `bash test-website.sh html` → "html5validator not installed, skipping" |
| root meta-repo (`dev.helix.code` go 1.25.2) | **Yes** | None (errors only from non-module `cli_agents*/` reference trees) | `.gitmodules.bak.https-to-ssh`; tracked `coverage.out`/README.html/README.pdf | ~150+ aspirational `*_IMPLEMENTATION*.md`/`PHASE_*` doc sprawl | `go build ./...` EXIT 0 (cli_agents errors outside module graph) |
| helix_code (inner, go 1.26) | **Yes (Asmt 5/11) / unverified (Asmt 12 timed out)** | None disabled; cold compile hit 90-180s timeout in Asmt 12 | `internal/tools/lsp_fakeserver` (test fixture) | `internal/memory/providers/faiss_provider.go` self-described "simulation"; `internal/worker/consensus.go:222` | Asmt 11: `go build ./...` clean (benign `-lobjc` linker warnings); Asmt 12: builds **unverified** within time cap |
| helix_qa | **Yes** | None | None | (not deep-assessed this wave) | `go build ./...` EXIT 0 (Asmt 11) |

## B. New broken/failing/disabled items (fix-first candidates) — with evidence

1. **doc_processor — committed broken build (CONFIRMED).** `pkg/i18n/translator.go` and `pkg/i18n/translator_test.go` contain unresolved git merge-conflict markers at HEAD `7750140` (clean working tree → in version control, not local). `go build ./...` → `pkg/i18n/translator.go:4:1: expected 'package', found '<<'`. The binary `cmd/docprocessor` imports i18n so it cannot build; root `automation_test.go`/`e2e_test.go`/`security_test.go` cascade-fail. README claims §11.4 anti-bluff "every claim exercised" while the repo does not build — a direct contradiction. **Highest-severity fix-first.**

2. **challenges — `go build ./...` broken out-of-the-box (CONFIRMED).** `pkg/userflow/container_infra.go:8-13` imports `digital.vasic.containers/pkg/{compose,event,health,logging,runtime,serviceregistry}` with 0 `replace` directives and 0 containers entries in `go.sum`. Breaks `pkg/userflow/` (118 files) and the only binary `cmd/userflow-runner`. `helix-deps.yaml` does not declare the containers dep (CONST-054 manifest drift). The other 14 `pkg/` packages build + test clean.

3. **llms_verifier — outer/meta module not buildable offline (CONFIRMED).** From repo root `go build ./...` → `missing go.sum entry` for go-sqlite3, bcrypt, brotli, gin, quic-go, uuid, logrus, kafka-go, amqp091; Asmt 11 notes `go.sum` is **ABSENT** at that layer. The real product is the inner `llm-verifier/` module which builds + vets clean. Outer module needs `go mod download`/`go get` from a clean checkout.

4. **github_pages_website — HTML test silently validates nothing (CONFIRMED).** `test-website.sh:45-53` prints `WARNING: html5validator not installed, skipping HTML validation` and exits without failure — a "passing" HTML test that asserts nothing (anti-bluff concern). `package.json:16-18` `build`/`logs`/`status` scripts invoke `docker-compose` but no `docker-compose.yml` exists (only `Containerfile`; `start-website.sh` uses podman) — script/manifest mismatch.

5. **helix_code — `internal/memory/providers/faiss_provider.go` self-described simulation (CONFIRMED static read, gating unverified).** Struct comments say "not used in simulation", "GPU not available in simulation", "always true in simulation"; grep for `os/exec|http.Post|http.Get|net.Dial` → **0** matches (no real FAISS/IPC). Candidate CONST-050(A)/Rule-2 production-mock violation — **unverified** whether gated to unit-tests-only.

6. **helix_code — `internal/worker/consensus.go:222` multi-peer election transport not implemented.** Self-flagged §11.4/CONST-035 gap: vote-request transport "is not implemented (peers=%d). Node remains Candidate" — election can hang (also a concurrency risk, §E).

## C. New dead-code suspects

- **challenges:** `pkg/userflow/` (118 `.go` files) — effectively dead/unbuildable pending the containers dependency.
- **llms_verifier:** `llm-verifier/api/handlers.go.backup`, `llm-verifier/cmd/main.go.backup` (wrong suffix, not compiled); `backup/scoring_example.go` (`package main` outside any module — orphaned); committed `*.db.backup_*` artifacts under `cmd/`.
- **llms_verifier:** `llm-verifier/go.mod:172` `replace digital.vasic.llmprovider => ../../LLMProvider` — target submodule MISSING and no Go file imports `digital.vasic.llmprovider` → dead replace directive.
- **llm_orchestrator:** `go.mod:18` `replace digital.vasic.llmprovider => ../LLMProvider` — actual sibling is `../llm_provider` (case mismatch, CONST-052); unused by source so build unaffected — latent dependency-wiring bug. Plus stray `submodule-analysis.txt` (317 B).
- **root meta-repo:** `.gitmodules.bak.https-to-ssh`.
- **doc_processor / llm_orchestrator:** stale `submodule-analysis.txt` referencing older commit (`8b83447` vs HEAD `7750140` in doc_processor).
- Note (not dead code): `helix_agent/cmd/cognee-mock/` and `helix_code/internal/tools/lsp_fakeserver/` are named mocks/fixtures — flagged to confirm they are NOT in a production path (assessors treat them as test surface; production-path status **unverified**).

## D. New tooling gaps (Snyk / SonarQube / govulncheck / race / profiling) per module

| Module | Snyk | Sonar | govulncheck | gosec | -race wired | Notes |
|---|---|---|---|---|---|---|
| challenges | absent | absent | absent | absent | yes (`-race -p 1`) | `.go-mutesting.yml` present; `make lint` needs golangci-lint but **no `.golangci.yml`** |
| security | absent | absent | **absent** (course-exercise only) | absent | yes (`-race -p 1`) | No committed `.golangci.yml` → `make lint` env-dependent. Gap notable for a *security* module |
| llms_verifier | absent | absent | **wired** (`Makefile:90,97`) | **wired** (`Makefile:95`) | yes (`Makefile:53`) | golangci-lint wired; richest scan config of this wave (not executed) |
| helix_agent | **present** (`.snyk` v1.25.0) | **present** (`sonar-project.properties`) | wired (security-scan) | `.gosec.yml`+baseline | yes (`test-race`) | `.snyk` ignore `SNYK-GOLANG-...logr` **expired 2026-06-01** → stale/active as of 2026-06-04. Also `.semgrep.yml`/`.trivy.yaml`/`.hadolint.yaml`/`.golangci.yml` |
| doc_processor | absent | absent | absent | absent | yes (`make test-race`) | governance gate scripts present |
| llm_orchestrator | absent | absent | absent | absent | yes (`make test`/`race`) | `lint` = `go vet` only; fuzz target present |
| llm_provider | absent | absent | absent | absent | yes (every test target) | governance gates present |
| vision_engine | absent | absent | **absent** | absent | yes (`test-race`) | `lint` golangci with `go vet` fallback |
| github_pages_website | absent | absent | absent | n/a | n/a | only html5validator *referenced*, not installed; `.gitignore` ignores only `.idea` |
| helix_code | **present** (`.snyk`+sonar) | **present** | wired (`run-all-tests.sh`) | — | yes (17 `-race` mentions) | tools not installed on host |

**Host tooling (Asmt 11):** snyk, sonar-scanner, govulncheck, pprof binary all **NOT on PATH**; podman ON PATH, docker NOT FOUND (Rule-4/§11.4.76 consistent). `SONARQUBE_SNYK_IMPLEMENTATION.md` is an aspirational PLAN (status "STARTING", Phase 2-4 unchecked) — scanning is **configured-but-not-runnable** here. Root `coverage.out` (5 KB) tracked — likely stale build derivative (§11.4.30 candidate).

## E. New concurrency risks (file:func)

- **helix_code — `internal/worker/consensus.go` (Raft-style election):** when vote-request transport is absent (`:222`), node "remains Candidate" — election can hang. Highest-attention new risk. (Manager-pattern mutexes documented but **not runtime `-race`-verified**.)
- **helix_agent — `internal/background/worker_pool.go`** (worker pool + progress channels) and **`internal/streaming`**: Makefile has dedicated `test-race`/`audit-concurrency` targets implying known sensitivity; `-race` **not run** this session — none independently confirmed.
- **challenges — `pkg/runner/parallel.go`** (parallel challenge execution) and **`pkg/monitor/websocket.go`** (concurrent WS clients): highest-attention spots; 14 building pkgs passed `-short` (run **without** `-race` to stay under timeout → race-cleanliness **unverified** this session; Makefile intends `-race`).
- **No NEW defects observed** in security, llm_provider, llm_orchestrator, vision_engine — all use explicit `sync.Mutex`/`RWMutex`/`atomic`; llm_orchestrator (`pkg/agent`), vision_engine (`pkg/remote`), llm_provider (core pkgs) passed `-race` clean where run. Provider/discovery/network-gated paths **unverified** under `-race`.

## F. New docs/courses/website/diagram/SQL gaps

- **Missing root `docs/ARCHITECTURE.md`** — CLAUDE.md §11 cites it as the architecture reference but `find -iname ARCHITECTURE*.md` is empty; content lives in `implementation_guide/002_Architecture_Diagrams.md`. STALE doc pointer.
- **No source diagram files** (`.mmd`/`.puml`/`.drawio`) anywhere — only rendered `.png` exports under `docs/bluff_proofing/` and `docs/improvements/` → no regen source per §11.4.77.
- **`VIDEO_COURSE_CURRICULUM.md`** is STUB/plan only (`Status: Planned`, 71 lines, no root material). Real course assets exist only inside the `github_pages_website/` submodule (`docs/courses`, ~46 entries in `course-data.js`).
- **`website/`** is PLAN-ONLY (`WEBSITE_CONTENT_PLAN.md`, `Status: Draft`, "Hugo or Docusaurus (TBD)"). Real site is the `github_pages_website/` submodule.
- **SQL secret-in-source:** `postgres-init.sql` hardcodes DB password `helixpass` (CONST-042 concern). `init.sql` only creates extensions/grants — no application table DDL at root (tables presumably via Go migrations, **unverified**).
- **§11.4.83 evidence tree sparse:** `docs/qa/` has only 6 feature run dirs (HXC-014/016/029/030/035/036) vs 767 test files — far from fully populated; README confirms the gate is "release-gate ONLY", not wired pre-commit/pre-push.
- **llms_verifier doc sprawl / self-cert smell:** 159 root `.md` files (AGENTS.md 152 KB, CLAUDE.md 137 KB) incl. `FINAL_*`/`ULTIMATE_*`/`COMPLETE_*`/`*_SUCCESS.md`/`COMPLETION_CELEBRATION.md` — exactly the "complete/passing" wording Rule 9 / §11.4 forbids without pasted runtime evidence; vastly over-states verified state.
- **root meta-repo doc sprawl:** ~80-150 `PHASE_*`/`COMPREHENSIVE_*`/`*_REPORT.md`/`FINAL_COMPLETION` files, largely stale/aspirational and unreconciled.
- **github_pages_website staleness:** `WEBSITE_VERSION.md` v1.1.0 / 2025-11-25 — ~6 months stale vs 2026-06-04 (§11.4.99 territory). Tracked runtime logs `docs/nginx-logs/access.log` + `error.log` (CONST-053 candidate). Provider claims (15+ providers) ARE substantiated by real `helix_code/internal/llm/*_provider.go` impls — not bluffed.
- **assets:** §3.2 of root CLAUDE.md lists `assets/` as a SUBMODULE but it is NOT in `.gitmodules` (plain tracked subdir); `assets/CLAUDE.md:12` is a garbled §11.9-anchor fragment; `.gitignore` references a non-existent derivative generator.
- **helix_code manual drift:** CLAUDE.md §3.2.1 claims ~45 internal packages; actual ~60 (approval, clarification, planner, secrets, telemetry, etc. not listed).

## G. Modules that returned no data (rate-limited again) — do NOT assume clean

- **None of the 12 Wave-2 modules returned empty** — all produced assessments. However, the following surfaces remain **explicitly unverified** (not "clean"):
  - **helix_code full build/test (Asmt 12):** cold compile hit the 90-180s timeout; `go build ./cmd/cli/`, `go vet ./cmd/cli/`, even `go build ./internal/version ./internal/logo` did NOT complete. Compile success **unverified within time cap** (Asmt 5/11 reported it builds clean — treat as conflicting evidence, re-verify with warm cache).
  - **helix_qa:** built clean (Asmt 11) but **not deep-assessed** this wave (no skip/placeholder/concurrency/docs pass).
  - **containers submodule:** never directly assessed — referenced only as the missing dependency breaking `challenges`. Its own build/test state is **unverified**.
  - All infra-bound suites (Postgres/Redis/Ollama/Kafka/RabbitMQ, live provider keys) across every module were **NOT run** per read-only/no-infra constraints — runtime behavior **unverified**.
  - Static-scan execution (Snyk/Sonar/govulncheck/gosec) **NOT run** anywhere (tools absent from PATH).

## H. Updated prioritized actions to fold into the next implementation wave (top 8)

1. **Resolve doc_processor committed merge conflict** — fix `pkg/i18n/translator.go` (lines 4,14,32,37,58,75,80,89,98) + `translator_test.go` (4,38,169). Restores `go build ./...` and the binary. A committed non-compiling repo that claims §11.4 anti-bluff green is the most urgent contradiction. (§11.4.108 SOURCE-layer failure; §11.4.121 commit-hygiene relevance.)
2. **Wire challenges → containers dependency** — add `digital.vasic.containers` `replace`/require + `go.sum` entries (flat-root per CONST-051(C)) and declare it in `helix-deps.yaml` (CONST-054). Unblocks `pkg/userflow/` + `cmd/userflow-runner`.
3. **Restore llms_verifier outer-module buildability** — regenerate/commit the absent root `go.sum` (or document the `go mod download` bootstrap as a §11.4.77 regeneration mechanism). Remove dead `replace digital.vasic.llmprovider => ../../LLMProvider` (target MISSING).
4. **Investigate + classify `helix_code/internal/memory/providers/faiss_provider.go`** — confirm whether the self-described "simulation" is gated to unit-tests-only; if reachable from production, it is a CONST-050(A)/Rule-2 violation requiring a real FAISS/IPC impl or fail-loud sentinel.
5. **Fix the github_pages_website HTML soft-skip + docker-compose phantom** — make `test-website.sh` FAIL (not skip) when html5validator is absent (anti-bluff §11.4.98), and reconcile `package.json` docker-compose scripts with the actual podman/`Containerfile` workflow. Untrack `nginx-logs/*.log` (CONST-053).
6. **Refresh helix_agent stale `.snyk` ignore** — the `SNYK-GOLANG-...logr` ignore expired 2026-06-01 (now active as of 2026-06-04); re-triage or re-justify. Reconcile inner CLAUDE.md `make container-*` claims vs root §11.4.99 note.
7. **Address structural doc/hygiene debt** — create/retarget root `docs/ARCHITECTURE.md` (or fix the §11 pointer); add `.mmd`/`.puml` source diagrams (§11.4.77); rotate the hardcoded `helixpass` in `postgres-init.sql` (CONST-042); prune the llms_verifier + root "FINAL/COMPLETE/SUCCESS" self-cert doc sprawl (Rule 9). Untrack root `coverage.out`.
8. **Schedule a warm-cache `-race` + build pass** for the three timed-out/unverified surfaces — helix_code full build (resolve Asmt 5/11 vs Asmt 12 conflict), `internal/worker/consensus.go` election hang, helix_agent `internal/background`/`internal/streaming`, and challenges `pkg/runner/parallel.go`/`pkg/monitor/websocket.go` (Makefiles intend `-race` but it was not run this session).