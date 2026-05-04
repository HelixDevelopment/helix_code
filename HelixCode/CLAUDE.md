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
