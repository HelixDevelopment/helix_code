# CLAUDE_INDEX.md — Thin Always-Loaded Index (§11.4.141)

**Purpose:** ~5-8K token entry point for fresh sessions. Full rules in `constitution/Constitution.md`.
**Session state:** `.remember/remember.md` (§11.4.131)

## Essential Rules (one line each, literal anchors for propagation gates)

- **No CI/CD pipelines** — no .github/workflows/, .gitlab-ci.yml, Jenkinsfile (§11.4.156)
- **No mocks in production** — mocks/stubs only in unit tests (§11.4.27/CONST-050)
- **SSH-only Git** — git@github.com:… URLs, never HTTPS (CONST-043)
- **Real data for non-unit tests** — real DB, real HTTP, real containers (§11.4.27)
- **100% challenge coverage** — every component needs Challenge scripts (CONST-048)
- **Reproduction-before-fix** — reproduce bug BEFORE fixing (§11.4.43)
- **Definition of done** — pasted terminal output from real run required (§11.4.5)
- **No self-certification** — "verified/tested/working" forbidden without evidence (§11.4.6)
- **Zero-bluff mandate** — tests must guarantee Quality+Completion+Usability (CONST-035/§11.4)
- **No force-push** — merge-onto-latest-main, never force (§11.4.113)
- **No hardcoded content** — all user-facing text from LLM/i18n/metadata (CONST-046)
- **Submodule-dependency manifest** — every submodule ships helix-deps.yaml (CONST-054)
- **Fetch-before-edit** — git fetch --all FIRST in every session (CONST-060)
- **Subagent-driven default** — fresh implementer per task (§11.4.70)
- **Zero-idle** — never sit idle if work is progressable (§11.4.94)
- **CodeGraph mandatory** — codegraph init + index for all AI-agent projects (§11.4.78)

## Build & Test Commands

```bash
# Root governance gates (from repo root)
make no-silent-skips    # fail on bare t.Skip()
make demo-all           # run every submodule's demo
./setup.sh              # first-time: submodules + deps + build

# Inner application (from helix_code/)
make build              # → bin/helixcode
make verify-compile     # quick compile sanity
make test               # all unit tests
make test-coverage      # coverage with -race
make dev                # build + run with dev config

# Full integration (real PG + Redis + Ollama)
make test-infra-up      # start docker-compose stack
make test-full          # ALL tests, ZERO skips
make test-infra-down    # tear down

# Anti-bluff smoke
grep -rniE "\bsimulated\b|\bfor now\b|TODO implement" helix_code/internal helix_code/cmd | grep -v _test.go | grep -q . && echo "BLUFF" || echo "clean"
```

## Anti-Bluff Checklist (§5)

- [ ] No simulation/placeholder in production code
- [ ] Real HTTP calls (not mocked)
- [ ] Real database operations
- [ ] Real process execution (os/exec)
- [ ] Tests validate reality, not function call counts
- [ ] Challenge validates end-to-end
- [ ] No bare t.Skip() without SKIP-OK marker
- [ ] Evidence pasted in commit/PR

## Technology Stack (§3.1)

Go 1.26 (inner app), Gin v1.11.0, PostgreSQL 15+, Redis 7+, golang-jwt/v4, Viper v1.21.0,
Cobra v1.8.0, AWS Bedrock, Azure azcore, Fyne v2.7.0 (desktop), tview (TUI), stretchr/testify.

## Repo Layout (§3.2)

```
helix_code/          # meta-repo root (governance + submodules)
├── helix_code/      # inner Go application (dev.helix.code)
├── submodules/      # owned submodules (per .gitmodules)
├── constitution/    # constitution submodule (source of truth)
├── dependencies/    # third-party submodules
├── docs/            # architecture, QA evidence, research
├── scripts/         # init, governance, gates
└── .remember/       # session resumption (§11.4.131)
```

## Current State

See `.remember/remember.md` for live HEAD, branch, queue, and execution plan.
