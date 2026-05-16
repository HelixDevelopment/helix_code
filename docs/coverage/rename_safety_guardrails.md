# Rename-Safety Guardrails (CONST-052 / Task #252)

**Operator safety mandate (2026-05-15):**

> "Double check that snake_case renaming is not applied to codebase
> which is by convention non-snake-case — directories of it or files!
> We MUST NOT break the System and working building process! Once done
> perform multiple checks for any signs of breaking the System,
> building process or introducing issues of any kind! Everything MUST
> BE safe, rock-solid and non-error-prone completed! Validate and
> verify this type of change with proper rebuild of the System and
> executing all tests and Challenges we have!"

This document is the operative companion to **CONST-052** (lowercase +
snake_case naming mandate / constitution submodule §11.4.29). CONST-052
already names "common-sense exceptions (technology-preserving)" for
language-mandated case; this document **enumerates every concrete
exception** so renames in Task #252 don't silently break the build.

## Hard rule

**Renaming is forbidden** for any directory or file whose name is
prescribed by:

1. **Build system / package manager** — `Makefile`, `GNUmakefile`,
   `Cargo.toml`, `Cargo.lock`, `Gemfile`, `pom.xml`, `build.gradle`,
   `gradlew`, `gradle/`, `package.json`, `package-lock.json`,
   `pnpm-lock.yaml`, `tsconfig.json`, `pyproject.toml`, `setup.py`,
   `setup.cfg`, `requirements.txt`, `go.mod`, `go.sum`, `Dockerfile`,
   `Containerfile`, `CMakeLists.txt`, `*.xcodeproj`, `*.xcworkspace`.

2. **Language / framework convention** — Java package paths (mixed-case
   reverse-DNS notation inside `src/main/java/`), Kotlin package paths
   (same), Apple/Swift framework directories, C# project directories,
   Go module paths inside `vendor/`.

3. **Android / AOSP build system** — every name mandated by the AOSP
   build system is exempt:
   - Top-level dirs: `art/`, `bionic/`, `bootable/`, `bootloader/`,
     `build/`, `cts/`, `dalvik/`, `developers/`, `development/`,
     `device/`, `docs/`, `external/`, `frameworks/`, `hardware/`,
     `kernel/`, `kernel-5.10/`, `libcore/`, `libnativehelper/`, `ndk/`,
     `out/`, `packages/`, `pdk/`, `platform_testing/`, `prebuilts/`,
     `sdk/`, `system/`, `test/`, `toolchain/`, `tools/`, `vendor/`.
   - Mandated files: `AndroidManifest.xml`, `Android.bp`, `Android.mk`,
     `AndroidTest.xml`, `bootstrap.bash`, `BUILD`, `kokoro`,
     `lk_inc.mk`, `OWNERS`, `version_defaults.mk`.

4. **Build / cache / generated artefacts** — `node_modules/`,
   `__pycache__/`, `.gradle/`, `.idea/`, `.vscode/`, `target/`, `dist/`,
   `out/`, `build/`, `bin/` (kept by tooling convention).

5. **VCS / governance state** — `.git/`, `.github/`, `.gitlab/`,
   `.svn/`, `.hg/`.

6. **Coordinated owned-project names** — the names listed below
   ARE going to be renamed eventually per Task #252 BUT only via
   a **phased migration** that also renames the upstream GitHub +
   GitLab repositories AND every consuming project's `.gitmodules`
   pointer AND every cross-reference. They CAN NOT be unilaterally
   renamed locally because the rename has to land atomically across
   all consumers:
   - `helix_code/`, `challenges/`, `containers/`, `Dependencies/`,
     `github_pages_website/`, `helix_agent/`, `helix_qa/`, `security/`,
     `panoptic/`, `upstreams/`, `assets/`, `mcp_servers/`.

## Soft target

Renames are permitted (and encouraged) for:

- Operator-controlled internal directories whose names are *not*
  mandated by any of the above categories.
- Files that are *named* by humans for human readability rather
  than for tooling consumption.

## Mechanical enforcement

The G6 gate in `scripts/verify-all-constitution-rules.sh` enumerates:

- **Protected count** — dirs that violate snake_case but match the
  never-rename allow-list. These are surfaced for forensic
  visibility but never reported as failures.
- **Unprotected rename candidates** — dirs that violate snake_case
  AND don't match any exemption. These are the deliberate scope
  of any future rename batch.

A batch that includes a protected name is a CONST-052 violation
(operator mandate at top of this doc applies). The rename script
(when Task #252's phased program writes one) MUST reject any move
whose source is in the never-rename allow-list.

## Validation requirements per rename batch

Per the operator mandate:

1. **Full rebuild** of the system after the rename batch.
2. **Execute all tests** that touch the renamed paths (unit +
   integration + e2e + Challenges).
3. **Execute `run_all_challenges.sh`** end-to-end with the renamed
   tree.
4. **Run `verify-all-constitution-rules.sh`** (post-rename
   constitution sweep per CONST-055).
5. **Run `verify-governance-cascade.sh`** (anchor cascade still
   intact across renamed paths).
6. **Reference-resolution gate** — every rename batch ships with a
   regression test verifying every reference to the renamed entity
   resolves to its new name (no stale references left).

A rename batch that ships without all six is a §11.4.29 violation
of equal severity to a §11.4 PASS-bluff at the reference-integrity
layer.

## Audit trail

| Date | Author | Round | Notes |
|---|---|---|---|
| 2026-05-15 | Claude Opus 4.7 | close-out²⁴ | Document created in response to operator's 2026-05-15 safety mandate. Mechanically backed by G6 gate's `NEVER_RENAME_PATTERNS` allow-list (~50 entries covering build systems, languages, AOSP top-level dirs, build artefacts, VCS state, and coordinated owned-project names). |
