# Stream FLAT-REFS — stale reference + .gitmodules audit (READ-ONLY, no edits/commits)

Repo: /Users/milosvasic/Projects/HelixCode

Context: A "Phase 2 flatten" (HEAD) moved all own-org submodules from `dependencies/vasic-digital/X`, `dependencies/HelixDevelopment/X`, and several top-level dirs INTO `submodules/X`. `helix_code/go.mod` replaces already rewritten. But ~646 stale `dependencies/{org}/...` strings remain, and `.gitmodules` still has old-path stanzas. The 5 DROPPED mounts (their `.gitmodules` stanzas must be deleted): `dependencies/HelixDevelopment/models`, `dependencies/vasic-digital/doc_processor`, `dependencies/vasic-digital/llm_orchestrator`, `dependencies/vasic-digital/llm_provider`, `dependencies/vasic-digital/vision_engine`. `constitution` stays at `./constitution/` (NOT flattened, unchanged).

You have NO Edit tool — that is intentional. Return a precise text remediation map only.

Tasks:
1. `grep -rIn "dependencies/vasic-digital\|dependencies/HelixDevelopment"` across the tree. Classify EACH hit:
   - CLASS A = META-REPO file (NOT under `submodules/`, NOT under `docs/improvements/`, NOT under `cli_agents_resources/`). Needs rewrite: the `dependencies/{org}/` prefix → `submodules/` (leaf name preserved).
   - CLASS B = file under `submodules/<leaf>/...` — DECOUPLED per CONST-051, MUST NOT be rewritten to parent layout. List only; for each note if the ref is self-referential (submodule's own dep layout / its helix-deps.yaml) vs genuinely pointing at the parent.
   - CLASS C = under `docs/improvements/**` or `cli_agents_resources/**` (archived/third-party). Leave; report COUNT only.
2. For CLASS A only: produce an EXACT table grouped by file: `file : line : old-string → new-string`. Do not summarize it away — I apply these patches serially.
3. Read the FULL `.gitmodules`. Report: (a) every stanza still on a `dependencies/{org}/` path + the corrected `path = submodules/<leaf>` it should become (note if `url` needs no change); (b) the 5 dropped-mount stanzas to delete entirely; (c) confirm `constitution` stanza unchanged; (d) flag any DUPLICATE stanzas (same leaf under both old and new path).
4. Totals per class + a 1-line readiness verdict.

Output: compact markdown.
