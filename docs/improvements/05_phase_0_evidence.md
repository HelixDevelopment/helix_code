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
