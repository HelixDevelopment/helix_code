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
