# CLAUDE.md — HelixCode Inner Go Application Agent Manual

**Version**: 1.0.0
**Date**: 2026-05-04
**Scope**: AI agent operating manual for the inner Go application at `HelixCode/`
**Authority**: Cascaded from root `CLAUDE.md` (../CLAUDE.md) with Go-application-specific addenda

---

## Peer governance

Sister files in this directory:
- `AGENTS.md` — generic-agent manual
- `CONSTITUTION.md` — constitutional rules

Parent files (one level up, meta-repo root):
- `../CONSTITUTION.md`
- `../CLAUDE.md`
- `../AGENTS.md`
- `../CRUSH.md`
- `../QWEN.md`

Synthesis spec: `../docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md`

---

## Article XI — Anti-Bluff and Quality Mandate

### §11.9 — Anti-Bluff Forensic Anchor

> Verbatim user mandate: *"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"*
>
> Operative rule: every PASS in this codebase MUST carry positive runtime evidence captured during execution. Metadata-only / configuration-only / absence-of-error / grep-based PASS without runtime evidence are critical defects regardless of how green the summary line looks.
>
> Tests and Challenges (HelixQA) are bound equally. A Challenge that scores PASS on a non-functional feature is the same class of defect as a unit test that does.
>
> No false-success results are tolerable. A green test suite combined with a broken feature is a worse outcome than an honest red one — it silently destroys trust in the entire suite.

**Cascade requirement:** this anchor (verbatim quote + operative rule) MUST appear in every submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md. Non-compliance is a release blocker. Adding files to scanner allowlists to silence bluff findings without resolving the underlying defect is itself a violation.

---

## Article XII — Security Mandates

### §12.1 (CONST-042) — No-Secret-Leak

> No API key, token, password, certificate, or other credential may be committed to any repository owned by HelixDevelopment or vasic-digital, transitively or otherwise. All secrets live in `.env` files (mode 0600) listed in `.gitignore`. Any leak — to git, logs, build artefacts, screenshots, or external services — is a release blocker until rotated and post-mortemed.

### §12.2 (CONST-043) — No-Force-Push

> No force push, force-with-lease push, history rewrite, branch deletion of `main`/`master`, or upstream-overwriting operation may be performed without explicit, in-conversation user approval given for that specific operation. Authorization for one push does not extend to subsequent pushes. Bypassing hooks (`--no-verify`), signature verification (`--no-gpg-sign`), or protected-branch rules also requires explicit approval.

---

<!-- BEGIN: REPO-SPECIFIC ADDENDA -->

## Repo-specific addenda — Go application specifics

### Tech stack (authoritative — `go.mod`)
- Go 1.26, module `dev.helix.code`
- Gin v1.11, gorilla/websocket v1.5, gRPC v1.80
- pgx/v5, lib/pq, redis/go-redis/v9
- golang-jwt/v4, viper v1.21, cobra v1.8
- Fyne v2.7 (desktop), tview/tcell (TUI), chromedp (headless)
- testify v1.11

### Single-test invocations (memorise these)

```bash
go test -v -run TestJWTGenerate ./internal/auth                          # unit
go test -v -tags=integration -run TestAPI_CreateTask ./tests/integration/...
go test -v -count=1 ./internal/verifier/...                              # cache-busted
go test -v -race -coverprofile=cover.out ./internal/llm                  # race+cover
```

### Anti-bluff smoke (run before claiming "done")

```bash
grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/ cmd/ applications/
```

Must return zero hits.

### Build & test

```bash
make build              # → bin/helixcode
make verify-compile     # quick compile check
make test               # unit
make test-infra-up      # docker-compose stack
make test-full          # all tests, ZERO skips
make scan-sonarqube     # CONST-042 scanner (rotate creds first; see ../docs/improvements/PROGRESS.md parking lot)
make scan-snyk
```

### Integration tests must hit real infrastructure

Per Constitution Rule 5: no mocks in integration tests. Files under `tests/integration/**/*.go`
with `-tags=integration` must NOT import `internal/mocks/`. Verified by `scripts/bluff-detector.sh`
Check 4 (P3 deliverable).

### Mocks ONLY in unit tests

`internal/mocks/` is a test-only tree. Production code (anything under `cmd/`, `applications/`,
`internal/<pkg>/<file>.go` not ending `_test.go`) must NEVER import from `internal/mocks/`.

### Critical bluff areas (from architecture review)

- **BLUFF-001**: LLM generation — must make real HTTP calls to real providers. NEVER simulate responses.
- **BLUFF-002**: Model listing — must query all configured providers via their APIs. NEVER hardcode lists.
- **BLUFF-003**: Command execution — must use `os/exec`. NEVER use `fmt.Printf` + `time.Sleep`.

<!-- END: REPO-SPECIFIC ADDENDA -->

---

## Reference commands

See root `CLAUDE.md` §3.4 for the full command catalogue. Inner-module-specific commands:

```bash
# From HelixCode/ (inner Go app root):
go build ./...                              # compile all packages
go test -short ./...                        # unit tests (mocks allowed)
go test -v -tags=integration ./tests/integration/...  # integration tests (NO mocks)
go vet ./...                                # static analysis
golangci-lint run ./...                     # linter
./tests/e2e/challenges/run_all_challenges.sh # all challenges

# From meta-repo root:
make build                                  # delegates to HelixCode/Makefile
make scan-sonarqube
make scan-snyk
make ci-validate-all
```

---

## See also

- Root CLAUDE.md: `../CLAUDE.md` (§3.4 reference command catalogue)
- Root CONSTITUTION.md: `../CONSTITUTION.md`
- Synthesis spec: `../docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md`
- Progress tracker: `../docs/improvements/PROGRESS.md`

## CONST-045 — No Hardcoded Distribution Hosts (constitutional anchor)

ALL container distribution targets SHALL be configured exclusively through `CONTAINERS_REMOTE_HOST_N_*` env vars in `Containers/.env`. NO host hardcoded anywhere. Adding/removing hosts = editing `Containers/.env` only; NO code change. Tests read `.env` at runtime and skip with `SKIP-OK:` when `CONTAINERS_REMOTE_ENABLED=false`. See root `CONSTITUTION.md` §CONST-045 for full mandate.

---

## CONST-047 — Recursive Submodule Application Mandate (cascaded from root CONSTITUTION.md)

> Verbatim user mandate (2026-05-14): *"Make sure all work we do is applied ALWAYS to all Submodules we control under our organizations (vasic-digital and HelixDevelopment) fully recursively everywhere with full bluff-proofing and comprehensive documentation, user manuals and guides and full tests and Challenges coverage!"*

Every engineering deliverable produced for the main project MUST be applied — fully and recursively — to every owned submodule under the `vasic-digital` and `HelixDevelopment` GitHub organizations. Each owned submodule (including this one) MUST receive in lockstep: (1) anti-bluff posture (CONST-035 / Article XI §11.9), (2) comprehensive documentation matching actual capabilities, (3) full tests + Challenges coverage with captured runtime evidence, (4) recursive propagation through nested submodules under the same orgs, (5) synchronized commits when meta-repo state advances this surface.

See the root `CONSTITUTION.md` §CONST-047 for the full mandate. This anchor MUST remain in this submodule's CONSTITUTION.md, CLAUDE.md, and AGENTS.md.

---

## CONST-048 — Full-Automation-Coverage Mandate (cascaded from root CONSTITUTION.md)

> Verbatim user mandate (2026-05-15): *"Make sure that every feature, every functionality, every flow, every use case, every edge case, every service or application, on every platform we support is covered with full automation tests which will confirm anti-bluff policy and provide the proof of fully working capabilities, working implementation as expected, no issues, no bugs, fully documented, tests covered!"*

No feature, functionality, flow, use case, edge case, service, or application on any supported platform of this Go application is deliverable until covered by automation tests proving six invariants: (1) anti-bluff posture with captured runtime evidence (CONST-035); (2) proof of working capability end-to-end on target topology; (3) implementation matches documented promise; (4) no open issues/bugs surfaced; (5) full documentation in sync; (6) four-layer test floor.

See root `CONSTITUTION.md` §CONST-048 and constitution submodule `Constitution.md` §11.4.25 for the full mandate.

## CONST-049 — Constitution-Submodule Update Workflow Mandate (cascaded from root CONSTITUTION.md)

> Verbatim user mandate (2026-05-15): *"Every time we add something into our root (constitution Submodule) Constitution, CLAUDE.MD and AGENTS.MD we MUST FIRST fetch and pull all new changes / work from constitution Submodule first! All changes we apply MUST BE commited and pushed to all constitution Submodule upstreams!"*

7-step pipeline before any constitution-submodule edit: fetch+pull first → apply with §11.4.17 classification → validate → commit+push ALL upstreams (governance files only) → conflict resolution preserving union (force-push forbidden) → cascade verification (CONST-047) → bump `.gitmodules` pointer in SAME commit.

See root `CONSTITUTION.md` §CONST-049 and constitution submodule `Constitution.md` §11.4.26 for the full mandate.

## CONST-050 — No-Fakes-Beyond-Unit-Tests + 100%-Test-Type-Coverage Mandate (cascaded from root CONSTITUTION.md)

> Verbatim user mandate (2026-05-15): *"Mocks, stubs, placeholders, TODOs or FIXMEs are allowed to exist ONLY in Unit tests! All other test types MUST interract with real fully implemented System! No fakes, empty implementations or bluffing is allowed of any kind! All codebase of the project MUST BE 100% covered with every supported test type."*

**(A) No-fakes-beyond-unit-tests.** Mocks/stubs/fakes/placeholders/TODO/FIXME/"for now"/empty-implementation patterns PERMITTED only in `*_test.go` files invoked without the integration build tag (or under `tests/unit/`). Every other test type — integration, E2E, full automation, security, DDoS, scaling, chaos, stress, performance, benchmarking, UI, UX, Challenges, HelixQA — MUST exercise this Go application against real infrastructure (real PostgreSQL, real Redis, real LLM endpoints, real containers). Production code under `cmd/`, `applications/`, and non-`_test.go` files in `internal/<pkg>/` MUST NOT import from `internal/mocks/`.

**(B) 100% test-type coverage** with every supported test type. Required submodules incorporated recursively per CONST-047: Challenges (`../Challenges/`) + HelixQA (`../HelixQA/`).

See root `CONSTITUTION.md` §CONST-050 and constitution submodule `Constitution.md` §11.4.27 for the full mandate.


## CONST-051 — Submodules-As-Equal-Codebase + Decoupling + Dependency-Layout Mandate (cascaded from root CONSTITUTION.md)

> Verbatim user mandate (2026-05-15): *"All existing Submodules in the project that we are controlling and belong to some our organizations (vasic-digital, HelixDevelopment, red-elf, ATMOSphere1234321, Bear-Suite, BoatOS123456, Helix-Flow, Helix-Track, Server-Factory) are equal parts of the project's codebase! ... We MUST NEVER modify Submodules to bring into them any project specific context ... All Submodule dependencies that are used by Submodule MUST BE acessed from the root of the project! We MUST NOT have nested Submodule dependencies."*

**(A)** Every owned-by-us submodule is an EQUAL part of this Go application's codebase. Same engineering attention as main: analysis, extension, tests, gap-fill, bug-fix, documentation. **(B)** Submodules MUST stay fully decoupled — NEVER inject project-specific context. **(C)** Dependencies of owned submodules MUST live at parent project's root (`../<name>/` or `../submodules/<name>/`); nested own-org submodule chains FORBIDDEN. Third-party submodules exempt.

See root `CONSTITUTION.md` §CONST-051 and constitution submodule `Constitution.md` §11.4.28 for the full mandate.