# HelixCode Inner Go Application — Constitution

**Version**: 1.0.0
**Date**: 2026-05-04
**Scope**: Inner Go application at `HelixCode/` within the HelixCode meta-repo
**Authority**: Cascaded from root `CONSTITUTION.md` (../CONSTITUTION.md) with Go-application-specific addenda

---

## Article I — Identity

This Constitution governs the inner Go application whose source lives at
`HelixCode/internal/`, `HelixCode/cmd/`, `HelixCode/applications/`,
`HelixCode/tests/`, etc. — the actual production code that end users invoke.
This is the most important governance node in the repository: bluffs, simulations,
and placeholder implementations would live here and cause direct user harm.

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

Full text: root `CONSTITUTION.md` Article XI §11.9.

---

## Article XII — Security Mandates

### §12.1 (CONST-042) — No-Secret-Leak

> No API key, token, password, certificate, or other credential may be committed to any repository owned by HelixDevelopment or vasic-digital, transitively or otherwise. All secrets live in `.env` files (mode 0600) listed in `.gitignore`. Any leak — to git, logs, build artefacts, screenshots, or external services — is a release blocker until rotated and post-mortemed.

### §12.2 (CONST-043) — No-Force-Push

> No force push, force-with-lease push, history rewrite, branch deletion of `main`/`master`, or upstream-overwriting operation may be performed without explicit, in-conversation user approval given for that specific operation. Authorization for one push does not extend to subsequent pushes. Bypassing hooks (`--no-verify`), signature verification (`--no-gpg-sign`), or protected-branch rules also requires explicit approval.

---

<!-- BEGIN: REPO-SPECIFIC ADDENDA -->

## Repo-specific addenda — Inner Go application

### Go module specifics
- Module: `dev.helix.code`
- Go version: 1.26
- Build: `make build` → `bin/helixcode`
- Single test invocation: `go test -v -run TestName ./internal/<pkg>`
- Integration tests: `go test -v -tags=integration -run TestX ./tests/integration/...`
- Mocks live ONLY at `internal/mocks/` and may be imported only by `*_test.go` under `internal/<pkg>/`. Per Constitution Rule 5, integration tests must NOT import `internal/mocks`.

### Anti-bluff smoke (run before declaring any feature done)

```bash
grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/ cmd/ applications/ && echo "BLUFF FOUND" || echo "clean"
```

### Per-feature DOD (Definition of Done) for Phase 1+2 ports

1. Production code at the documented path under `internal/<pkg>/`.
2. Unit test (mocks allowed).
3. Integration test (`-tags=integration`, no mocks).
4. Challenge under `tests/e2e/challenges/<feature>/` with `expected.json`.
5. Challenge runs against `make test-infra-up` and produces runtime evidence.
6. Evidence pasted into commit message body.
7. `scripts/bluff-detector.sh` exits clean on the diff.

### Universal mandatory rules (reproduced from root)

1. **No CI/CD Pipelines** — no `.github/workflows/`, `.gitlab-ci.yml`, `Jenkinsfile`, etc.
2. **No Mocks in Production** — mocks only in unit tests (`*_test.go`).
3. **No HTTPS for Git** — SSH URLs only (`git@github.com:…`).
4. **No Manual Container Commands** — use `make build` / orchestrator binary.
5. **Real Data for Non-Unit Tests** — real databases, real HTTP calls, real containers.
6. **100% Challenge Coverage** — every component needs Challenge scripts.
7. **Reproduction-Before-Fix** — every bug reproduced by Challenge before any fix.
8. **Definition of Done** — done requires pasted terminal output from real run against real artifacts.
9. **No Self-Certification** — no *verified/tested/working/complete/fixed/passing* without pasted command output.
10. **Zero-Bluff Mandate (CONST-035)** — every test must guarantee Quality + Completion + Full Usability.

<!-- END: REPO-SPECIFIC ADDENDA -->

---

## See also

- Root Constitution: `../CONSTITUTION.md`
- Root CLAUDE.md: `../CLAUDE.md`
- Root AGENTS.md: `../AGENTS.md`
- Synthesis spec: `../docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md`
- Phase 0 plan: `../docs/superpowers/plans/2026-05-04-phase-0-foundation-cleanup.md`
- Progress tracker: `../docs/improvements/PROGRESS.md`
