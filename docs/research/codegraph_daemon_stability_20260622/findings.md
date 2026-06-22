# CodeGraph Daemon #850 Watchdog Stability — Findings

| Field | Value |
|-------|-------|
| Date | 2026-06-22 |
| Symptom | Recurring "Main thread unresponsive ~60s — killing wedged process (#850)" |
| Verdict | Index massively oversized because `.codegraph/config.json` is INERT |
| Query reliability | Reliable (sub-ms reads); only intermittent fail inside the ~60s wedge window |

## Root cause (FACT, evidence-backed)

CodeGraph v1.0.1 is **zero-config** and excludes **only via `.gitignore`** (official
docs, accessed 2026-06-22). The repo's `.codegraph/config.json` (legacy schema, dated
Jun 3) is **no longer read** — it is inert.

Proof — the live DB tracks `.gitignore`, NOT config.json:
- config.json excludes `cli_agents/**`, yet **36,099 cli_agents files are indexed**
  (`git check-ignore cli_agents` → NOT IGNORED; root `.gitignore` still lists the
  pre-CONST-052 name `Example_Projects/`, never the renamed `cli_agents/`).
- `cli_agents_resources/` (514) and `github_pages_website/` (9) also "excluded" in
  config but indexed (not in `.gitignore`).
- `dependencies/{LLama_CPP,Ollama,HuggingFace_Hub}` = 0 indexed only because they are
  uninitialized (empty) submodules — NOT because the config worked.

Result: **96,674 files, 1,664,519 nodes, 3,829,858 edges → 4.38 GB DB**
(`submodules/` 57,456 + `cli_agents/` 36,099 + nested vendored submodule trees with
generated `*.gen.ts` / `worker-configuration.d.ts`). Each watcher re-walk/sync over
that tree blocks the main thread >60s on an 18 GB host under heavy memory pressure
(vm_stat: ~87 MB free, ~7.6 GB compressed) → the **#850 watchdog SIGKILLs and
restarts** in a loop. Compounding: **two daemons run** (one with `--path`, one
without), doubling load.

## CRITICAL corollary — `codegraph_validate.sh` is a PASS-bluff

`codegraph_validate.sh` returned **18/18 PASS** by checking `config.json`'s
include/exclude lists — but the tool ignores that file. The validator certifies an
exclusion that does not happen. Third-party `cli_agents/**` IS indexed despite the
"✅ cli_agents/** is excluded" line. This is a §11.4 PASS-bluff at the validation
layer and partially invalidates the earlier §11.4.79 "own-org included / third-party
excluded" conclusion (own-org IS included — confirmed by live explore — but
third-party is NOT actually excluded).

## Timeline

Current `daemon.log` (40 lines, one window): **9 watchdog kills**, 13 restarts, 4
clean idle-timeouts. Lines 1–20: tight start→wedge→kill→restart loop (7× back-to-back,
~1–2 min apart); calm middle (3 idle shutdowns); then 3 more kills. Every kill = main
thread blocked ≥60s (the `node -e` sidecar's 60000 ms timer, confirmed via `ps`).

## Query reliability

**Reliable.** All 4 live `codegraph_explore` calls returned full correct results
incl. own-org submodule symbols (`submodules/helix_qa`, `submodules/database`,
`submodules/llms_verifier`, `submodules/containers`). SQLite reads sub-ms even at
4.38 GB; the stall is in the sync/index path, not the read path. Index grew
96,674→96,676 mid-session (watcher actively re-walking). Caveat: a read landing in the
~60s wedge can intermittently fail and need a retry.

## Recommended mitigation (recommendation only — NOT applied)

- **A. Fix scope via the mechanism CodeGraph honors (`.gitignore` / `.git/info/exclude`), not config.json.**
  Exclude `cli_agents/`, `cli_agents_resources/`, `github_pages_website/`, nested
  `submodules/**/submodules/**` + `submodules/**/cli_agents/**`, and generated
  `**/*.gen.ts` / `**/worker-configuration.d.ts`. Since cli_agents must stay
  git-tracked, prefer non-tracking `.git/info/exclude` over editing committed
  `.gitignore`; confirm mechanism with operator.
- **B.** Stop daemon(s), delete `codegraph.db*`, re-run `codegraph index` — DB should
  shrink to a fraction; stalls disappear.
- **C.** Reconcile MCP config to ONE `serve --mcp --path …` daemon, not two.
- **D.** Watchdog tuning (`CODEGRAPH_WATCHDOG_TIMEOUT_MS` / `CODEGRAPH_NO_WATCHDOG=1`)
  is a stop-gap only — disabling reintroduces the 100%-CPU hang #850 prevents; fix
  scope first.
- **E. Constitution-level (§11.4.78/§11.4.79):** document exclude source-of-truth =
  `.gitignore` NOT config.json; add to `codegraph_validate.sh`: (1) index-size budget
  + `SELECT COUNT(*) FROM files WHERE path LIKE '<excluded>/%'` MUST be 0 (non-zero =
  inert exclude = release-blocker, mirroring §11.4.79's "index that lies is a
  PASS-bluff"); (2) daemon-health probe that FAILs if `#850` fired in recent log;
  (3) one-daemon-per-path invariant; (4) CONST-052 renames MUST cascade into ignore
  files in the same change.

## Sources (verified 2026-06-22)

- https://github.com/colbymchenry/codegraph/releases
- https://colbymchenry.github.io/codegraph/getting-started/configuration/
- https://colbymchenry.github.io/codegraph/guides/indexing/
- https://colbymchenry.github.io/codegraph/troubleshooting/
