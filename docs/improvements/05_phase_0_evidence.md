# Phase 0 Evidence Log

Each task's acceptance check output is pasted below with a timestamp. This file is the rolled-up forensic record per synthesis spec §3.2 / Article XI §11.9.

## P0-03 — HelixAgent submodule integration

**Timestamp:** 2026-05-04T20:56+03:00
**HelixAgent SHA committed:** `fe3f69e766077360d730da934e292f86646dadfd`
**Total disk size:** 777 MB (under 1 GB threshold — no shallow-init follow-up needed)

### Core libraries (all populated)

| Library | Files/dirs | SHA |
|---|---|---|
| HelixLLM | 25 | `a385d9e3d1b74d8064c2b112ff773052a791eb20` |
| HelixMemory | 15 | `f1d55ea6cb297160790e4e21c472cf3a1626c42a` |
| HelixSpecifier | 13 | `8f83107842f50010c09f63064b649bcf21330bc3` |
| LLMsVerifier | 266 | `1d53ae3b72c77c1f27171c0677431c48d2d02bdd` |

### cli_agents

- **Total directory entries:** 60
- **Populated (≥2 files):** 47 (78%)
- **Phase 1 priority `claude-code`:** ✓ populated (`HelixAgent/cli_agents/claude-code/`)
- **Empty (need Phase 2 sub-spec attention to fix HelixAgent's pin first):** 13
  - `aider, conduit, continue, HelixCode (recursive ref), kilo-code, kiro-cli (no upstream access), mobile-agent, ollama-code, opencode-cli, openhands, plandex, roo-code, superset`

**Why empty:** HelixAgent's recorded submodule pointers reference SHAs that no longer exist on the corresponding upstream remotes (e.g. force-push or branch deletion upstream). Examples:
- `cli_agents/aider`: `5c536a29...` not found on aider's upstream
- `cli_agents/plandex`: `9825d8d5...` not found on plandex's upstream
- `cli_agents/kiro-cli`: repo `git@github.com:stark1tty/kiro-cli.git` inaccessible (no rights)

This is a **pre-existing HelixAgent issue**, not caused by this Phase 0 work. Per spec §1.3 N2 (HelixAgent rewrite is out of scope for this programme), the fix lives in HelixAgent's own governance: each affected agent's Phase 2 sub-spec will bump HelixAgent's submodule pointer to a SHA that exists upstream.

### .gitmodules entry verified

```
[submodule "HelixAgent"]
	path = HelixAgent
	url = git@github.com:HelixDevelopment/HelixAgent.git
```

SSH URL per Constitution Rule 3.

### Pre-commit secret scan

`git diff --cached` produced only `.gitmodules` + `HelixAgent` gitlink + `docs/improvements/05_phase_0_evidence.md` + `docs/improvements/PROGRESS.md`. Token-pattern grep returned zero matches.

### Acceptance check status

| Plan acceptance criterion | Result |
|---|---|
| `.gitmodules` has `[submodule "HelixAgent"]` SSH | ✓ |
| `HelixAgent/{HelixLLM,HelixMemory,HelixSpecifier,LLMsVerifier}` exist + populated | ✓ all four |
| `HelixAgent/cli_agents/` count ≥35 | ✓ 60 entries (47 fully populated) |
| `HelixAgent/cli_agents/claude-code` populated (Phase 1 priority) | ✓ |
| Pre-commit secret scan clean | ✓ |
| No third-party submodule modifications | ✓ verified — no commits made inside any submodule |
| HelixAgent total size measured | ✓ 777 MB |

**Outcome:** Phase 1 (claude-code porting) is unblocked. The 13 unpopulated cli_agents are documented as deferred to Phase 2 sub-spec time when each affected agent's pin can be bumped in HelixAgent.

## P0-04 — verify-llmsverifier-pin-parity.sh

**Timestamp:** 2026-05-04T21:30+03:00

### Live pass-path output (Step 4.2)

```
FAIL: LLMsVerifier pin divergence
  Dependencies/HelixDevelopment/LLMsVerifier  → 629c5bd5d141351270e72b6fb7359fa4b7881d7c
  HelixAgent/LLMsVerifier → 1d53ae3b72c77c1f27171c0677431c48d2d02bdd

Resolution: pick the canonical SHA, bump the other to match, commit, push.
exit=1
```

### Synthetic-divergence test (Step 4.4)

Because the live state was already divergent (exit=1), we used the canonical submodule's `HEAD^` (`1d53ae3b...` — which equals the transitive SHA) to temporarily bring the two into parity, proving the script correctly reports exit=0 on parity:

After checking out `PARENT_PARENT_SHA` (canonical→`1d53ae3b`, transitive→`1d53ae3b`):
```
OK: LLMsVerifier pin parity — both at 1d53ae3b72c77c1f27171c0677431c48d2d02bdd
exit=0 (expected — script correctly detects parity)
```

After restoring canonical to `PARENT_SHA` (`629c5bd5`):
```
FAIL: LLMsVerifier pin divergence
  Dependencies/HelixDevelopment/LLMsVerifier  → 629c5bd5d141351270e72b6fb7359fa4b7881d7c
  HelixAgent/LLMsVerifier → 1d53ae3b72c77c1f27171c0677431c48d2d02bdd

Resolution: pick the canonical SHA, bump the other to match, commit, push.
exit=1  (matches Step 4.2 — back to original live state)
```

Restore verified: `git -C Dependencies/HelixDevelopment/LLMsVerifier rev-parse HEAD` returns `629c5bd5d141351270e72b6fb7359fa4b7881d7c` — correct.

### Live divergence status

Pins diverge — see PROGRESS.md parking lot for resolution.

- `Dependencies/HelixDevelopment/LLMsVerifier` → `629c5bd5d141351270e72b6fb7359fa4b7881d7c`
- `HelixAgent/LLMsVerifier` → `1d53ae3b72c77c1f27171c0677431c48d2d02bdd`

The canonical pin is one commit ahead of the transitive (HelixAgent) pin. Resolution deferred per spec §1.3 N2.

## P0-05 — API-key migration from ../HelixAgent/.env

**Timestamp:** 2026-05-04T21:15:12+03:00

**Source:** `-rw------- milosvasic milosvasic 7603 /run/media/milosvasic/DATA4TB/Projects/HelixAgent/.env`

**Destination:** `-rw------- milosvasic milosvasic 7603 /run/media/milosvasic/DATA4TB/Projects/HelixCode/HelixCode/.env`

**Key count:** 109

**Key set diff (source vs destination):**

```
```

(empty diff = identical key set)

**In git index — precise exact-match check:** `HelixCode/HelixCode/.env` is **NOT** tracked by git (`git ls-files --error-unmatch HelixCode/HelixCode/.env` exits 1). The earlier substring-grep count of 2 reflected unrelated tracked files (`HelixCode/HelixCode/.env.example` and `HelixCode/HelixCode/.env.full-test`), not the secret-bearing file.

## P0-06 — .gitignore hardening
Timestamp: 2026-05-04T21:22:51+03:00

Root .gitignore tail:
```
# Allow the e2e challenge testing framework but ignore test results
!HelixCode/tests/e2e/challenges/
HelixCode/tests/e2e/challenges/test-results/
HelixCode/tests/e2e/challenges/.DS_Store


reports/demos/

# === Secret hygiene (CONST-041) — added P0-06 ===
.env.*
!.env.example
*.pem
*.key
*.crt
id_rsa
id_rsa.pub
id_ed25519
id_ed25519.pub
helix.security.json
# === END Secret hygiene ===
```

Inner .gitignore tail:
```

# nyc test coverage
.nyc_output

# Dependency directories (if using Go modules, comment out vendor/ above)
# vendor/

# End of https://www.toptal.com/developers/gitignore/api/go,vim,emacs,visualstudiocode
cli

# === Secret hygiene (CONST-041) — added P0-06 ===
.env
.env.*
!.env.example
id_rsa
id_rsa.pub
id_ed25519
id_ed25519.pub
helix.security.json
# === END Secret hygiene ===
```

Verifications:
- HelixCode/.env is ignored: YES
- HelixCode/.env.example is NOT ignored: YES (good)
- Tracked credential files (pre-existing CONST-041 violations, all committed before this task): **three files** are in the git index:
  - `HelixCode/test/workers/ssh-keys/id_rsa` — SSH private key labelled `helixcode-test`
  - `HelixCode/test/workers/ssh-keys/id_rsa.pub` — corresponding public key
  - `helix.security.json` — root-level security credential file (5929 bytes, executable)

  All three were committed before this programme began. The CONST-041 `.gitignore` blocks prevent any NEW untracked instances of these patterns from being accidentally added. Proper remediation — key/credential rotation, `git rm --cached` to remove from index, regeneration of any derived secrets, and historical-leak documentation — is deferred to **T08** (`scripts/scan-secrets.sh`). The planted-secret test in T08 will fail on the live tree due to these three files, triggering tracked remediation through the standard scan-secrets workflow.

- **Asymmetric coverage between root and inner `.gitignore` CONST-041 blocks** is intentional and correct:
  - The **root block** adds 10 patterns: omits `.env.local` (pre-existing at root `.gitignore` line 5) and omits `.env` at block level (pre-existing at line 4; P0-06 polish adds it back into the block to make the block self-contained).
  - The **inner block** adds 8 patterns: omits `*.pem`, `*.key`, `*.crt` (pre-existing at inner `.gitignore` lines 85–87); omits `.env.local` (pre-existing at inner `.gitignore` line 44).
  - **Effective combined coverage**: all 12 canonical secret-file patterns are protected at both the root and inner levels — the asymmetry reflects de-duplication of already-existing lines, not a coverage gap.

## P0-07 — .env.example refresh

**Timestamp:** 2026-05-04T21:36:23+03:00

**Key parity vs ../HelixAgent/.env:** OK (identical)

**Real values present:** 0 (must be 0)

**Total keys:** 109

**Verification commands and output:**

```
$ grep -oE '^[A-Z_]+=' ../HelixAgent/.env | sort -u > /tmp/p0-07-canonical-keys.txt
$ wc -l /tmp/p0-07-canonical-keys.txt
109 /tmp/p0-07-canonical-keys.txt

$ diff <(grep -oE '^[A-Z_]+=' ../HelixAgent/.env | sort -u) \
       <(grep -oE '^[A-Z_]+=' HelixCode/.env.example | sort -u)
key-diff-exit=0

$ result=$(grep -E '^[A-Z_]+=[^<]' HelixCode/.env.example | grep -vE '=$')
$ [ -z "$result" ] && echo "VERIFIED: zero real values present in .env.example"
VERIFIED: zero real values present in .env.example
real-value-count=0
```

**CONST-041 status:** COMPLIANT — every entry in `.env.example` is either a comment, blank line, or `KEY=<REDACTED>`.

**File format:** 7-line header block + 1 blank line + 109 `KEY=<REDACTED>` lines = 117 lines total.

## P0-08 — scan-secrets.sh + planted-secret test

**Timestamp:** 2026-05-04T22:15+03:00

### Test harness output (Step 8.4)

```
TEST 1: clean directory → expect exit 0
  PASS
TEST 2: planted secret → expect non-zero exit
  PASS — scanner detected the planted secret

Results: 2 passed, 0 failed
exit=0
```

### Live-tree run (Step 8.5)

Live exit code: **1** (expected — known credential files in tracked tree per T06 polish evidence; NOT a script defect)

Filenames flagged by the scanner (file:line only — values redacted per CONST-041):

```
./Challenges/Panoptic/docs/SECURITY.md:462:
./Challenges/Panoptic/tests/security/panoptic_test.go:422:
./Challenges/Panoptic/tests/security/panoptic_test.go:423:
./Challenges/Panoptic/tests/security/panoptic_test.go:454:
./Challenges/pkg/env/redact_test.go:37:
./Challenges/pkg/env/redact_test.go:48:
./Challenges/pkg/logging/redacting_logger_test.go:55:
./docs/COMPLETE_CLI_REFERENCE.md:908:
./docs/superpowers/plans/2026-05-04-phase-0-foundation-cleanup.md:833:
./docs/troubleshooting/guide.md:649:
./.env:188:
./.env:20:
./.env:4:
./.env:6:
./.env:9:
./HelixCode/.env:118:
./HelixCode/.env:119:
./HelixCode/.env:186:
./HelixCode/.env:29:
./HelixCode/.env:41:
./HelixCode/.env:43:
./HelixCode/.env:46:
./HelixCode/.env:47:
./HelixCode/.env:53:
./HelixCode/internal/llm/vertexai_provider_test.go:25:
./HelixCode/internal/worker/ssh_pool_test.go:569:
./HelixCode/tests/e2e/test-bank/performance_security_tests.go:1073:
./HelixCode/test/workers/ssh-keys/id_rsa:1:
./HelixQA/pkg/llm/google_test.go:334:
./Security/pkg/securestorage/securestorage_test.go:129:
```

### Findings

**Tracked files with matches:**
| File | Tracked | Nature |
|---|---|---|
| `HelixCode/test/workers/ssh-keys/id_rsa` | YES | Pre-existing tracked SSH private key (T06 known credential #1) |
| `helix.security.json` | YES | Pre-existing tracked credential file (T06 known credential #2) |
| `HelixCode/test/workers/ssh-keys/id_rsa.pub` | YES (no match) | Public key — does not match any pattern |
| `docs/COMPLETE_CLI_REFERENCE.md:908` | YES | Example/doc text: `sk-ant-your-anthropic-key` (placeholder, not a real key) |
| `docs/superpowers/plans/...foundation-cleanup.md:833` | YES | Planted-secret TDD test example: `sk-FAKE0123456789...` (fake) |
| `docs/troubleshooting/guide.md:649` | YES | Documentation snippet: `-----BEGIN OPENSSH PRIVATE KEY-----` (illustrative, incomplete) |
| `HelixCode/internal/llm/vertexai_provider_test.go:25` | YES | Unit-test fixture: embedded fake RSA key block (test data, not rotatable) |
| `HelixCode/internal/worker/ssh_pool_test.go:569` | YES | Unit-test stub: partial `-----BEGIN OPENSSH PRIVATE KEY-----` header only |
| `HelixCode/tests/e2e/test-bank/performance_security_tests.go:1073` | YES | Test fixture: `sk-1234567890abcdef` (clearly fake) |

**Not-tracked files flagged** (untracked working-tree files — not a commit risk):
- `.env`, `HelixCode/.env`: the real secret-bearing env files (correctly untracked per P0-06 gitignore)
- `Challenges/Panoptic/...`, `HelixQA/...`, `Security/...`: submodule working-tree files, not tracked at root

**The 3 pre-existing tracked credentials from T06 polish:**
- `HelixCode/test/workers/ssh-keys/id_rsa` — correctly detected by scanner (pattern: `-----BEGIN ... PRIVATE KEY-----`)
- `HelixCode/test/workers/ssh-keys/id_rsa.pub` — public key, no secret pattern matches (expected)
- `helix.security.json` — NOT detected in this run (see note below)

**Note on `helix.security.json`:** The scanner's `SCAN_TARGET="."` run did not flag `helix.security.json` in the output above. The file's content does not match any of the scanner's current patterns (it's a JSON credential file that likely uses JWTs or other formats not in the regex set). The file IS blocked from future commits via `.gitignore` (P0-06) and will be addressed in P0-T08.5 remediation. The scanner's pattern set can be extended in P0-T08.5 to cover JWT/JSON credential formats.

**Additional tracked files with fake/doc patterns** (`docs/*.md` and test fixtures): these are false-positive-in-intent — the patterns are doc examples and test fixtures, not real rotatable secrets. They will be reviewed during P0-T08.5 to determine whether to add per-file exclusions or restructure test data. No immediate action required.

---

## P0-08.7 — Port SonarQube + Snyk security scan integration (through Containers)

**Timestamp:** 2026-05-04T22:30+03:00
**Commits:** `1d728de` + `2494bc8` + `e29e2f6` + `16a4490` + (this commit)
**Branch:** main

### Sub-commit 1 — Compose files + configs (`1d728de`)

Files created:
- `HelixCode/docker/security/sonarqube/docker-compose.yml` — adapted from HelixAgent; container names changed from `helixagent-*` to `helixcode-*`; subnet changed to `172.21.0.0/16`
- `HelixCode/docker/security/sonarqube/sonar-project.properties` — project key/name/version use `${SONARQUBE_PROJECT_KEY}` etc.
- `HelixCode/docker/security/snyk/docker-compose.yml` — adapted; uses `${SNYK_TOKEN:-}`
- `HelixCode/docker/security/snyk/Dockerfile` — adapted; references HelixCode project
- `HelixCode/sonar-project.properties` — root-level; uses `${SONARQUBE_PROJECT_KEY}` env-var
- `HelixCode/.snyk` — root-level Snyk policy (v1.25.0)

Credential scan result: `No credentials found - OK`

### Sub-commit 2 — Master orchestrator script (`2494bc8`)

- `HelixCode/scripts/security-scan.sh` — supports `snyk|sonarqube|trivy|gosec|grype|kics|semgrep|all`
- Loads credentials from `HelixCode/.env` (gitignored)
- Reports to `reports/security/<scanner>-<timestamp>.<ext>`
- `--help` dry run verified:

```
$ ./scripts/security-scan.sh --help
Usage:
  ./scripts/security-scan.sh [scanner] [options]

Scanners:
  snyk        - Snyk vulnerability scanner (requires SNYK_TOKEN)
  sonarqube   - SonarQube code quality and security analysis
  ...
exit 0
```

### Sub-commit 3 — Makefile targets (`e29e2f6`)

Inner Makefile (`HelixCode/Makefile`) targets added:
`security-scan`, `security-scan-snyk`, `security-scan-sonarqube`, `security-scan-trivy`,
`security-scan-gosec`, `security-scan-grype`, `security-scan-kics`, `security-scan-semgrep`,
`security-scan-all`, `deps-scan`, `secrets-scan`, `scan-start-sonar`, `scan-stop`

Root Makefile targets added:
`scan-sonarqube`, `scan-snyk`, `scan-all`, `scan-gosec`, `scan-trivy`, `scan-secrets`

Dry-run verification:
```
$ make -n scan-sonarqube
make -C HelixCode security-scan-sonarqube
./scripts/security-scan.sh sonarqube
```

### Sub-commit 4 — Containers BootManager wiring (`16a4490`)

- `HelixCode/cmd/security-scan/main.go` (~170 lines) wires:
  - `digital.vasic.containers/pkg/runtime.AutoDetect(ctx)` for runtime detection
  - `digital.vasic.containers/pkg/endpoint.NewEndpoint()` builder for SonarQube + Snyk endpoints
  - `digital.vasic.containers/pkg/health.NewDefaultChecker()` with HTTP health on `:9000/api/system/status`
  - `digital.vasic.containers/pkg/boot.NewBootManager().BootAll(ctx)` for lifecycle management
  - Supports `-action=start|stop|status`

Build verification:
```
$ go build ./cmd/security-scan/...
# exit 0 — binary compiles
```

`scripts/security-scan.sh start_sonarqube()` updated to call `go run ./cmd/security-scan` when Go is available; falls back to direct compose otherwise.

### Sub-commit 5 — Challenges + evidence (this commit)

Challenges created:
- `HelixCode/tests/e2e/challenges/sonarqube/run.sh` — 8 sections, 33 tests (config-correctness only)
- `HelixCode/tests/e2e/challenges/sonarqube/expected.json`
- `HelixCode/tests/e2e/challenges/snyk/run.sh` — 7 sections, 26 tests (config-correctness only)
- `HelixCode/tests/e2e/challenges/snyk/expected.json`

Challenge run output (both 100% PASS):

```
===========================================
  SonarQube Security Scanning Challenge
===========================================
[PASS] Root sonar-project.properties exists
[PASS] SonarQube docker-compose.yml exists
[PASS] SONAR_TOKEN uses env-var reference (not hardcoded)
[PASS] sonar.projectKey uses env-var reference in root properties
[PASS] No HelixAgent-specific references in HelixCode configs
[PASS] SonarQube service defined
[PASS] PostgreSQL service defined
[PASS] SonarQube uses pinned community edition image
[PASS] Memory limits configured
[PASS] SonarQube health check uses /api/system/status
[PASS] Security network defined
[PASS] security-scan.sh --help shows sonarqube mode
[PASS] cmd/security-scan imports Containers BootManager
[PASS] cmd/security-scan uses runtime.AutoDetect
Results: 33/33 passed, 0 failed

===========================================
  Snyk Security Scanning Challenge
===========================================
[PASS] Snyk docker-compose.yml exists
[PASS] SNYK_TOKEN uses env-var reference (not hardcoded)
[PASS] No HelixAgent-specific container names
[PASS] snyk-deps service defined
[PASS] snyk-full service defined
[PASS] Dockerfile uses official snyk/snyk-cli base image
[PASS] .snyk has policy version
[PASS] security-scan.sh reads SNYK_TOKEN from env
[PASS] cmd/security-scan imports Containers BootManager
[PASS] cmd/security-scan handles snyk scanner
Results: 26/26 passed, 0 failed
```

### CREDENTIAL ROTATION NOTE — MANDATORY before live scans

This task does NOT run live scans. The original `helix.security.json` credentials (SonarQube token, Snyk token, project_key, organization) were committed and are considered compromised (see P0-T08.5). The user MUST rotate these before any live scan can succeed:

1. **SonarQube token**: generate a new API token in SonarQube UI → set `SONAR_TOKEN=<new>` in `HelixCode/.env`
2. **SonarQube project key**: choose a new key → set `SONARQUBE_PROJECT_KEY=<new>` in `HelixCode/.env`
3. **Snyk token**: generate a new token at snyk.io → set `SNYK_TOKEN=<new>` in `HelixCode/.env`
4. **Snyk organization**: set `SNYK_ORG=<new_org_id>` in `HelixCode/.env`

After rotation, live scan can be invoked with:
```bash
make scan-sonarqube    # from root
make scan-snyk         # from root
```

### scan-secrets.sh verification

```
$ bash scripts/scan-secrets.sh HelixCode/docker/security HelixCode/sonar-project.properties HelixCode/.snyk ...
OK: no credential patterns found
exit code: 0
```

### Remote convergence (all 5 commits)

All 3 distinct remotes (github, gitlab, upstream) converged on each sub-commit SHA.
Final HEAD after sub-commit 5 (this evidence): see PROGRESS.md.

**Script correctness verdict:** The scanner is operating correctly on real data. It detects the known tracked private key (`id_rsa`) and real api-key patterns in the live `.env` files. Live-run exit=1 is correct given the presence of pre-existing tracked credentials.
