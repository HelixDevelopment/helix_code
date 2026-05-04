# AGENTS.md — HelixCode Inner Go Application Generic Agent Manual

**Version**: 1.0.0
**Date**: 2026-05-04
**Scope**: Generic-agent operating manual for the inner Go application at `HelixCode/`
**Authority**: Cascaded from root `AGENTS.md` (../AGENTS.md) with Go-application-specific addenda

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

## Process expectations
- Always run `make verify-foundation` (in the meta-repo root) before declaring work done. (P0-15 will define this target; until then, run `make ci-validate-all`.)
- Commit format per synthesis spec §7.2 (includes Phase + Task + Evidence fields).
- Push to all four remotes (`github`, `gitlab`, `origin`, `upstream`) at session boundaries.
- Never use `git push --force` without explicit per-operation user approval (CONST-043).
- Never commit `.env`, `.pem`, `.key`, or any other credential file (CONST-042).

## Stop/resume protocol

1. Read `../docs/improvements/PROGRESS.md`.
2. Find "Active task".
3. Run `make verify-foundation` (or `make ci-validate-all` until P0-15 lands) to confirm foundation intact.
4. Resume from where the file points.

## Go-application-specific process rules

- All production code (non-test) goes under `internal/<pkg>/`, `cmd/<app>/`, or `applications/<platform>/`.
- Every new package needs at minimum: one unit test, one integration test (when it touches I/O), and one Challenge.
- File naming: `<feature>.go`, `<feature>_test.go`. Do NOT put mocks in production files.
- Before ANY commit touching `internal/` or `cmd/`, run:

```bash
go build ./...
go vet ./...
go test -short ./...
grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/ cmd/ applications/ && echo "BLUFF FOUND" || echo "clean"
```

All four must succeed/clean before staging.

## Synthesis spec reference

Synthesis spec at: `../docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md`

Commit format from spec §7.2:
```
<type>(<scope>): <summary>

Phase: <phase>
Task:  <task-id>
Evidence: docs/improvements/05_phase_0_evidence.md § <section>

Co-Authored-By: <agent-name> <email>
```

<!-- END: REPO-SPECIFIC ADDENDA -->

---

## See also

- Sister files: `CLAUDE.md`, `CONSTITUTION.md`
- Root AGENTS.md: `../AGENTS.md`
- Root CONSTITUTION.md: `../CONSTITUTION.md`
- Synthesis spec: `../docs/superpowers/specs/2026-05-04-cli-agent-fusion-synthesis-design.md`
- Progress tracker: `../docs/improvements/PROGRESS.md`
