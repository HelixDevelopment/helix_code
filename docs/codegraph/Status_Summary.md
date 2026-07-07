# CodeGraph — Status Summary

**Revision:** 2
**Last modified:** 2026-07-07T10:40:00Z

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-05-28 |
| Last modified | 2026-07-07T10:40:00Z |
| Status | active |
| Status summary | Two-audience digest of `Status.md` per §11.4.56. Page 1 = non-developer; Page 2 = software engineer. Tracks the HXC-017 CodeGraph index-configuration fix and the §11.4.80 update/sync automation. 2026-07-07: Phase-4 reindex on codegraph 1.2.0 — index refresh (sync) GREEN and our own-org code libraries proven reachable by the AI agents; one clean-up (removing stale third-party entries) is paused because the shared host had no spare process capacity. |

---

## 2026-07-07 — Phase-4 reindex (codegraph 1.2.0)

**For the team (non-developer).** The code "map" the AI agents use is current
and healthy on the latest CodeGraph (version 1.2.0): about 102,657 files and
1.78 million code symbols. We proved that the AI agents can genuinely look up
symbols that exist ONLY inside our own reusable libraries (the `HelixLLM` and
`LLMsVerifier` submodules we build under HelixDevelopment and vasic-digital) —
so the map really reaches our own code, not just the top-level app. No secrets
(passwords, keys, `.env` files) are in the map. One tidy-up remains: the map
still contains some third-party reference projects it should skip; removing
them needs a full rebuild, which we paused because the shared workstation was
temporarily out of spare capacity (other work was running on it). Tracked as
HXC-041 for a quieter moment; it does not affect our own code being reachable.

**For the engineer.** codegraph 1.2.0; `codegraph sync` GREEN (8 files, 5.6 s,
exit 0). Own-org resolution PROVEN via MCP (`codegraph_explore`) + CLI
(`codegraph query`/`node`): `admit` (unexported) →
`submodules/helix_llm/internal/vrambroker/broker.go:178`,
`ResolveModelCapability` →
`submodules/llms_verifier/llm-verifier/capabilities/registry_resolve.go:62`.
`scripts/codegraph_validate.sh`: 26 PASS, 3 FAIL. §11.4.10 credential audit
CLEAN (0 `.env`/`.pem`/`.key`). The 3 FAIL = stale third-party entries
(`cli_agents` 36,089 / `cli_agents_resources` 519 / `github_pages_website` 9):
config.json `exclude` is INERT in 1.2.0 (exclusion is `.gitignore`-driven per
§11.4.78) and these are tracked dirs; the from-scratch `codegraph index` to
purge them fork-failed on host process saturation (4069/4096 `ulimit -u`,
non-ours workloads per §11.4.174). Index verified intact after the aborted
rebuild. Full detail + evidence in `Status.md` (2026-07-07 entry) and
`docs/qa/phase4_codegraph_20260707/`.

## Page 1 — For the team (non-developer)

**What this is.** CodeGraph is a local "map" of all the source code in
HelixCode. The AI coding agents use it to instantly find where any function,
type, or file lives — much faster and more accurate than searching text.

**What works.** The CodeGraph map is built and active. As of the latest
re-index it covered the HelixCode source tree (tens of thousands of files
and hundreds of thousands of code symbols).

**What changed (HXC-017).** We found that the map was accidentally leaving
out a large group of our OWN reusable code libraries (the submodules we
build and maintain ourselves under the `vasic-digital` and
`HelixDevelopment` organizations). The configuration had a single broad
rule that skipped the entire `dependencies/` folder. We narrowed that rule
so:

- Our own libraries are now **included** in the map (the agents can see and
  navigate them).
- Only genuinely third-party libraries we did NOT write (llama.cpp, Ollama,
  HuggingFace Hub) stay **excluded** — there is no value in mapping code we
  do not maintain.
- We also made doubly sure that no passwords, keys, or secrets can ever end
  up in the map.

**What's pending.** A full rebuild of the map was started so the newly
included libraries actually appear. Its completion status is recorded on the
engineer page and in the event ledger.

**Team actions.** None required. The update/sync runs on a weekly cadence
using shared automation.

---

## Page 2 — For software engineers

**Change (HXC-017, §11.4.79).** `.codegraph/config.json` `exclude` list:
removed the blanket `dependencies/**`; added per-submodule-ownership split.

- INCLUDED (own-org): `dependencies/vasic-digital/**`,
  `dependencies/HelixDevelopment/**` (no longer matched by any exclude).
- EXCLUDED (third-party, §11.4.74 vendor path): `dependencies/LLama_CPP/**`
  (ggml-org), `dependencies/Ollama/**` (ollama),
  `dependencies/HuggingFace_Hub/**` (huggingface).
- Added §11.4.10 credential excludes: `**/.env`, `**/.env.*`, `**/*.key`,
  `**/*.pem`, `**/secrets/**`.

**Ownership classification source.** `git config -f .gitmodules
--get-regexp 'submodule\..*\.url'` — each `dependencies/<X>` classified by
URL org. Only three top-level `dependencies/<X>` entries are non-own-org
(llama.cpp, ollama, huggingface_hub); the rest live under
`dependencies/vasic-digital/` or `dependencies/HelixDevelopment/`.

**Validation.** Config JSON valid
(`python3 -c "import json;json.load(open('.codegraph/config.json'))"`).
Index status BEFORE: Files 39,024 / Nodes 624,103 / Edges 1,643,200.
AFTER + own-org-symbol probe (§11.4.79 anti-bluff): recorded in the
`Status.md` event ledger.

**Automation (§11.4.80).** Inherited by reference — invoke
`constitution/scripts/codegraph_update.sh` (weekly npm-update with
version-verify anti-bluff) and `constitution/scripts/codegraph_sync.sh`
(status → sync → status → validate, ledger-appending). Never copied into
HelixCode per §3 submodule inheritance.

**Regeneration (§11.4.77).** `.codegraph/codegraph.db` gitignored;
regenerated by `codegraph index .` (full) / `codegraph sync .`
(incremental) from the tracked `.codegraph/config.json`.

**Composes with:** §11.4.10, §11.4.45, §11.4.56, §11.4.65, §11.4.74,
§11.4.77, §11.4.78, §11.4.79, §11.4.80, §1.1.
