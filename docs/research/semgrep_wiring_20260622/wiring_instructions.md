# Semgrep wiring — apply instructions (conductor / operator)

**Constitution:** §11.4.166 (Universal Semgrep static analysis mandate).
**Date:** 2026-06-22
**Working dir:** /Volumes/T7/Projects/helix_code
**Evidence basis:** `docs/research/semgrep_onboarding_20260622/findings.md`

The onboarding agent created only DISJOINT new files. It did **NOT** touch
`.mcp.json`, `~/.zshrc`, `~/.bashrc`, or the live `.git/hooks/pre-commit`
(meta-git race avoidance + live-hook safety, §11.4.6). All four wiring points
below are for the conductor/operator to apply.

Scripts are inherited **by reference** from the constitution submodule
(§11.4.28 / §11.4.35 / §11.4.74) — wire, never copy.

---

## Files the agent created (DISJOINT, safe to commit)

| File | Purpose |
|---|---|
| `.docs_chain/contexts/semgrep_status.yaml` | docs_chain context for the semgrep Status ledger (§11.4.166(6) / §11.4.106) |
| `docs/semgrep/Status.md` | consumer-side semgrep event ledger (SOURCE for the docs_chain context) |
| `docs/research/semgrep_wiring_20260622/precommit_wiring.md` | exact pre-commit hook change |
| `docs/research/semgrep_wiring_20260622/mcp_entry.json` | exact `.mcp.json` "semgrep" entry |
| `docs/research/semgrep_wiring_20260622/wiring_instructions.md` | this file |

---

## (1) docs_chain context — DONE (file created)

`.docs_chain/contexts/semgrep_status.yaml` registers a `semgrep_status` context:
`docs/semgrep/Status.md` (markdown SOURCE) → `Status.html` (md-to-html /
pandoc) → `Status.pdf` (html-to-pdf / weasyprint). Schema matches the existing
contexts (`governance`, `fixed`, `helixcode`).

Generate the exports:

```sh
cd /Volumes/T7/Projects/helix_code
docs_chain sync   --root . semgrep_status     # regenerate Status.html + Status.pdf
docs_chain verify --root . semgrep_status     # pre-build gate (must pass)
```

(If `docs_chain` resolves to the engine per §11.4.106; the constitution exposes
it. The `.html`/`.pdf` siblings are gitignored-derivatives-regenerated, not
hand-authored.)

---

## (2) Pre-commit hook — apply per `precommit_wiring.md`

The live `.git/hooks/pre-commit` is the `guard-forbidden-commands.sh` guard
(§11.4.109), with NO semgrep reference. Wire semgrep **additively** (do not
weaken the guard). RECOMMENDED (Option A in `precommit_wiring.md`): replace the
hook's single trailing `exit 0` with:

```sh
_SEMGREP_HOOK="$(git rev-parse --show-toplevel)/constitution/scripts/hooks/semgrep_precommit.sh"
if [ -x "$_SEMGREP_HOOK" ]; then
  "$_SEMGREP_HOOK" || exit 1
fi

exit 0
```

This runs `semgrep scan --config auto --error` on staged source files and blocks
the commit (exit 1) on any finding; graceful-degrades when semgrep is absent.
Full detail + Option B (via `scripts/install_git_hooks.sh`) + an inline
one-liner alternative in `precommit_wiring.md`.

---

## (3) `.mcp.json` "semgrep" server — apply per `mcp_entry.json`

Add the `semgrep` key from `mcp_entry.json` into the existing `mcpServers`
object in `/Volumes/T7/Projects/helix_code/.mcp.json` (sibling to `codegraph`,
`media-validator`, `open-design`). Do **not** replace the file — insert the one
key. Verified for this host: `uvx` is present at `/opt/homebrew/bin/uvx`
(FACT), so `uvx semgrep-mcp` is launchable; `uvx` fetches `semgrep-mcp` on
demand. The entry pins `SEMGREP_PATH=/opt/homebrew/bin/semgrep` (the working
1.167.0 binary confirmed this session).

After insertion, validate the merged file:

```sh
python3 -m json.tool /Volumes/T7/Projects/helix_code/.mcp.json >/dev/null && echo "VALID JSON"
```

Then restart the agent session so the new MCP server loads, and confirm
`semgrep` appears in the live MCP server list (install exit 0 != loadable —
confirm by observing it live, §11.4.102(C)).

> Note: this environment also exposes the marketplace semgrep MCP plugin as
> `mcp__plugin_semgrep_semgrep__*`. The `.mcp.json` `uvx semgrep-mcp` entry is
> the project-pinned, decoupled form (§11.4.28) required by §11.4.166 so the
> wiring is reproducible from the checkout regardless of marketplace plugins.

---

## (4) Shell rc PATH line — operator applies

Add to `~/.zshrc` (and/or `~/.bashrc`) so semgrep is on PATH in every shell
(§11.4.166(2)):

```sh
source /Volumes/T7/Projects/helix_code/constitution/scripts/semgrep/semgrep_path.sh
```

(Currently moot for interactive use since semgrep is already on PATH via
Homebrew, but the rc source line is required by §11.4.166(2) for
all-shells/all-users availability and is harmless — `semgrep_path.sh` only
prepends common bin dirs, dedup-guarded, and does not require semgrep
pre-installed.)

---

## Post-apply verification (§11.4.166 / §107)

```sh
cd /Volumes/T7/Projects/helix_code
# validate (anti-bluff JSON-with-results probe)
sh constitution/scripts/semgrep/semgrep_validate.sh
# CI integration test (known-vuln detection, evidence under qa-results/)
sh constitution/scripts/semgrep/semgrep_ci_test.sh
# docs_chain export gate
docs_chain verify --root . semgrep_status
```

## Sources verified 2026-06-22
- `constitution/scripts/semgrep/{semgrep_path,semgrep_validate,semgrep_setup,semgrep_ci_test}.sh`
- `constitution/scripts/hooks/semgrep_precommit.sh`
- `.git/hooks/pre-commit`, `.mcp.json`, `.docs_chain/contexts/*.yaml` (read this session)
- host probe: `uvx`=/opt/homebrew/bin/uvx, `semgrep`=1.167.0 (this session)
- Constitution §11.4.166; §11.4.28/§11.4.35/§11.4.74/§11.4.102(C)/§11.4.106
