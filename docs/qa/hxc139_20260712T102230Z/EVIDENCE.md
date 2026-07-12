# HXC-139 — QA evidence (§11.4.83)

**Item:** HXC-139 (Bug / High) — A vendored reference-agent fixture breaks the helix_agent module build
**Fix commit:** helix_agent `ecbbf1e0` (pushed ff-only to all 4 mirrors: github vasic-digital + HelixDevelopment, githubhelixdevelopment, upstream — all at `ecbbf1e05fc3f450afee5fbde0ea8043e278bdce`)
**helix_code pointer bump:** `submodules/helix_agent` → `ecbbf1e0`
**Date (UTC):** 2026-07-12T10:22:30Z
**Discipline:** §11.4.102 systematic-debugging (four-phase), §11.4.115 RED-polarity, §11.4.135 regression guard, §11.4.142 independent review, §11.4.28/§11.4.51 third-party decoupling.

## Root cause (Phase 1, FACT — reproduced)

`go build ./...` / `go vet ./...` in `submodules/helix_agent` (module `dev.helix.agent`, go 1.26) failed with:

```
cli_agents/continue/core/autocomplete/context/root-path-context/test/files/file1.go:4:2:
package core/autocomplete/context/root-path-context/test/files/models is not in std
(/usr/lib/golang/src/core/autocomplete/context/root-path-context/test/files/models)
```

`cli_agents/continue` is a **third-party nested git submodule** (`url = git@github.com:continuedev/continue.git`).
It ships a Go test-fixture with a domain-less import. Because nothing on the path between the
module root and that file carried a `go.mod`, Go swept the fixture into the `dev.helix.agent`
build and the whole module failed to build/vet.

## Fix (Phase 4)

Two new files in helix_agent's OWN tree (the third-party Continue submodule and all other nested
submodules under `cli_agents/` are NOT modified):

1. `cli_agents/go.mod` — nested-module marker `module dev.helix.agent.vendored/cli_agents` (go 1.26),
   making the whole vendored `cli_agents/` reference tree a separate module excluded from
   `dev.helix.agent`'s `./...`. Consistent with the pre-existing `Makefile` `EXCLUDE_DIRS` intent.
2. `tests/regression/cli_agents_isolation_test.go` — §11.4.115 RED-polarity regression guard
   (package `regression`): `RED_MODE=1` reproduces the defect on the pre-fix artifact,
   `RED_MODE=0` (default) is the standing GREEN guard (§11.4.135).

## Captured verification (all on helix_agent @ ecbbf1e0)

```
[1] RED_MODE=1 go test ./tests/regression/ -run TestCliAgentsIsolation -v -count=1
    --- PASS: TestCliAgentsIsolation (0.03s)
    log: RED reproduced HXC-139: module build breaks with the file1.go 'is not in std'
         signature when cli_agents/go.mod is absent

[2] go test ./tests/regression/ -run TestCliAgentsIsolation -v -count=1   (GREEN)
    --- PASS: TestCliAgentsIsolation (3.36s)
    log: GREEN: 506 dev.helix.agent packages, 0 under cli_agents — HXC-139 isolation intact

[3] go build ./...   → exit 0
[4] go vet   ./...   → exit 0
[5] go list  ./... | grep -c cli_agents   → 0
```

## §1.1 anti-bluff (non-tautology) — proven by independent reviewer

Independent review (structurally-separated agent, §11.4.142) re-ran all of the above itself
and additionally moved `cli_agents/go.mod` aside and re-ran the GREEN test: it **FAILED**
(`GREEN: cli_agents/go.mod missing or has no module line — HXC-139 fix absent`), then restored
the marker and re-ran GREEN → PASS, byte-identical tree. This proves the guard is non-tautological
(it does not pass unconditionally). Reviewer verdict: **GO, zero findings** (no blocking, no nit,
no warning). Third-party Continue submodule verified untouched (`git -C cli_agents/continue status`
empty; `git submodule status` shows no dirty marker).

## Blast radius (§11.4.145)

`grep -rn 'dev\.helix\.agent/cli_agents' --include='*.go'` (outside the 2 new files) = 0 matches —
no real `dev.helix.agent` package imports anything under `cli_agents/`, so excluding the tree
breaks no import. `go.mod`/`go.sum` reference no `cli_agents` path. Module path
`dev.helix.agent.vendored/cli_agents` does not collide; `go 1.26` matches the outer module.
