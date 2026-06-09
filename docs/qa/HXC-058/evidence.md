# HXC-058 — helix_agent `go build ./...` third-party fixture (out-of-scope determination)
**Captured:** 2026-06-09T15:19:18Z · Bug · Fixed (→ Fixed.md)
## Investigation (§11.4.124 — third-party, do not modify)
`cd submodules/helix_agent && go build ./...` fails on exactly ONE package:
`cli_agents/continue/core/autocomplete/context/root-path-context/test/files/file1.go:4:2: package ... is not in std`.
`cli_agents/continue` is a git submodule of continuedev/continue (.gitmodules line 154) — NOT owned
(not vasic-digital/HelixDevelopment). file1.go is upstream's intentionally-non-compiling multi-language
test fixture (siblings file1.php, base_module.py, etc.); continue has no root go.mod. Per CLAUDE.md §3.2
cli_agents/ holds third-party reference agents. Modifying it would violate §11.4.124 + §11.4.28.
## Resolution: owned build is GREEN — no owned-code defect
helix_agent's owned-build contract (Makefile): primary `go build -mod=mod ./cmd/helixagent` + TEST_PACKAGES
`./cmd/... ./internal/... ./pkg/... ./tests/... ./challenges/...`. Captured:
- `go build -mod=mod -o /tmp/helixagent_hxc058 ./cmd/helixagent` → exit 0
- `go build -mod=mod ./cmd/... ./internal/... ./pkg/... ./tests/... ./challenges/...` → exit 0 (zero stderr)
The `./...` failure is solely the wildcard walking the on-disk third-party continue submodule; it is out of
owned-build scope and NOT a defect in owned code. No code change made; no pointer bump.
