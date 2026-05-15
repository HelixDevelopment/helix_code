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

## CONST-045 — No Hardcoded Distribution Hosts (cascaded from root CONSTITUTION.md)

ALL container distribution targets SHALL be configured exclusively through `CONTAINERS_REMOTE_HOST_N_*` env vars in `Containers/.env`. NO host hardcoded in ANY source, test, challenge, config, script, or governance document. Adding/removing hosts = editing `Containers/.env` only; NO code change. Tests SHALL read `.env` at runtime and skip with `SKIP-OK:` marker when `CONTAINERS_REMOTE_ENABLED=false`. See root `CONSTITUTION.md` §CONST-045 for full mandate and cascade requirements.

---

## CONST-046 — No Hardcoded Content (cascaded from root CONSTITUTION.md)

NO user-facing text, question template, prompt text, error message, label, helper text, or explanatory content may be hardcoded as a static literal string. All user-facing text MUST be:
1. Generated dynamically by an LLM at runtime based on context (user's language, prompt, session), OR
2. Loaded from an i18n resource file (`.yaml`, `.json`, `.toml`) configurable per locale, OR
3. Composed from verifier metadata, configuration data, or provider responses.

**Prohibition**: Static literal arrays of question text, fixed English prompt templates, hardcoded UI labels. Hardcoded English text silently breaks the product for non-English users — it is a constitutional violation. Every string visible to the user MUST adapt to the user's language and context.

See root `CONSTITUTION.md` §CONST-046 for the full mandate, examples, and cascade requirements.

---

## CONST-047 — Recursive Submodule Application Mandate (cascaded from root CONSTITUTION.md)

> Verbatim user mandate (2026-05-14): *"Make sure all work we do is applied ALWAYS to all Submodules we control under our organizations (vasic-digital and HelixDevelopment) fully recursively everywhere with full bluff-proofing and comprehensive documentation, user manuals and guides and full tests and Challenges coverage!"*

Every engineering deliverable produced for the main project MUST be applied — fully and recursively — to every owned submodule under the `vasic-digital` and `HelixDevelopment` GitHub organizations. Each owned submodule (including this one) MUST receive in lockstep: (1) anti-bluff posture (CONST-035 / Article XI §11.9), (2) comprehensive documentation matching actual capabilities, (3) full tests + Challenges coverage with captured runtime evidence, (4) recursive propagation through nested submodules under the same orgs, (5) synchronized commits when meta-repo state advances this surface.

See the root `CONSTITUTION.md` §CONST-047 for the full mandate. This anchor MUST remain in this submodule's CONSTITUTION.md, CLAUDE.md, and AGENTS.md.

## CONST-048 — Full-Automation-Coverage Mandate (cascaded from root CONSTITUTION.md)

> Verbatim user mandate (2026-05-15): *"Make sure that every feature, every functionality, every flow, every use case, every edge case, every service or application, on every platform we support is covered with full automation tests which will confirm anti-bluff policy and provide the proof of fully working capabilities, working implementation as expected, no issues, no bugs, fully documented, tests covered!"*

No feature/functionality/flow/use-case/edge-case/service/application on any supported platform of this Go application is deliverable until covered by automation tests proving six invariants. See root `CONSTITUTION.md` §CONST-048 and constitution submodule `Constitution.md` §11.4.25 for the full mandate.

## CONST-049 — Constitution-Submodule Update Workflow Mandate (cascaded from root CONSTITUTION.md)

> Verbatim user mandate (2026-05-15): *"Every time we add something into our root (constitution Submodule) Constitution, CLAUDE.MD and AGENTS.MD we MUST FIRST fetch and pull all new changes / work from constitution Submodule first! All changes we apply MUST BE commited and pushed to all constitution Submodule upstreams!"*

7-step pipeline before any constitution-submodule edit. See root `CONSTITUTION.md` §CONST-049 and constitution submodule `Constitution.md` §11.4.26 for the full mandate.

## CONST-050 — No-Fakes-Beyond-Unit-Tests + 100%-Test-Type-Coverage Mandate (cascaded from root CONSTITUTION.md)

> Verbatim user mandate (2026-05-15): *"Mocks, stubs, placeholders, TODOs or FIXMEs are allowed to exist ONLY in Unit tests! ... All codebase of the project MUST BE 100% covered with every supported test type."*

**(A)** Mocks/stubs/fakes/placeholders/TODOs/FIXMEs/"for now" patterns PERMITTED only in unit-test sources; non-unit tests MUST exercise this Go application against real infrastructure. Production code MUST NOT import `internal/mocks/`. **(B)** 100% test-type coverage: unit + integration + E2E + full-automation + security + DDoS + scaling + chaos + stress + performance + benchmarking + UI + UX + Challenges (`../Challenges/`) + HelixQA (`../HelixQA/`).

See root `CONSTITUTION.md` §CONST-050 and constitution submodule `Constitution.md` §11.4.27 for the full mandate.


## CONST-051 — Submodules-As-Equal-Codebase + Decoupling + Dependency-Layout Mandate (cascaded from root CONSTITUTION.md)

> Verbatim user mandate (2026-05-15): *"All existing Submodules in the project that we are controlling and belong to some our organizations (vasic-digital, HelixDevelopment, red-elf, ATMOSphere1234321, Bear-Suite, BoatOS123456, Helix-Flow, Helix-Track, Server-Factory) are equal parts of the project's codebase! ... We MUST NEVER modify Submodules to bring into them any project specific context ... All Submodule dependencies that are used by Submodule MUST BE acessed from the root of the project! We MUST NOT have nested Submodule dependencies."*

**(A)** Every owned-by-us submodule is an EQUAL part of this Go application's codebase. Same engineering attention as main: analysis, extension, tests, gap-fill, bug-fix, documentation. **(B)** Submodules MUST stay fully decoupled — NEVER inject project-specific context. **(C)** Dependencies of owned submodules MUST live at parent project's root (`../<name>/` or `../submodules/<name>/`); nested own-org submodule chains FORBIDDEN. Third-party submodules exempt.

See root `CONSTITUTION.md` §CONST-051 and constitution submodule `Constitution.md` §11.4.28 for the full mandate.
### CONST-052 — Lowercase-Snake_Case-Naming Mandate (cascaded from constitution submodule §11.4.29)
Every directory/submodule/file MUST use lowercase snake_case names. Existing non-compliant names MUST be renamed atomically with updates to all references (configs, docs, source-code imports, governance files). Common-sense exceptions: language-mandated case (Java/Kotlin/Android/Apple/C#/Swift) inside language-root, vendor third-party submodules, build artefacts. `Upstreams/` → `upstreams/` transition: `install_upstreams.sh` supports BOTH directory names during migration. Phased execution; each rename batch ships with (i) reference-resolution regression test, (ii) full CONST-050(B) test-type matrix run, (iii) anti-bluff wire-evidence. See root `CONSTITUTION.md` §CONST-052 and constitution submodule `Constitution.md` §11.4.29 for the full mandate.
