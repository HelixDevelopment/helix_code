# CONST-051(B) ↔ CONST-051(C) Standalone-Build Tension

## Status

**Architectural — operator decision required.** Surfaced after Tasks
#254 + #273 migrated HelixAgent's 46 and HelixLLM's 34 nested own-org
submodules to canonical parent-root locations.

## The tension

CONST-051(C) (nested-own-org chains forbidden) and CONST-051(B)
(submodules-as-standalone, project-not-aware, reusable) pull in
opposite directions for Go-module consumers like HelixAgent and
HelixLLM.

### Before Tasks #254 / #273 (CONST-051(C) violation)
```go
// HelixAgent/go.mod
replace digital.vasic.eventbus => ./EventBus
// HelixAgent/.gitmodules
[submodule "EventBus"]
    path = EventBus
    url = git@github.com:vasic-digital/EventBus.git
```

Standalone-buildable (G5-style ✓), but violates G4 (nested own-org
chain).

### After Tasks #254 / #273 (CONST-051(B) violation)
```go
// HelixAgent/go.mod
replace digital.vasic.eventbus => ../Dependencies/vasic-digital/EventBus
// HelixAgent/.gitmodules
(no EventBus entry — moved to parent root)
```

Satisfies G4 ✓, but standalone clones break:
```
$ git clone git@github.com:HelixDevelopment/HelixAgent.git
$ cd HelixAgent
$ go build ./...
go: module digital.vasic.eventbus@v0.0.0 not found
```

## Attempted fix: go.work workspace at HelixCode root

A `go.work` file with `use ./HelixAgent + ./Dependencies/vasic-digital/...`
was attempted (this round). It surfaced a SECOND conflict:

```
go: conflicting replacements for digital.vasic.docprocessor:
    .../Dependencies/HelixDevelopment/DocProcessor
    .../Dependencies/vasic-digital/DocProcessor
```

Both repos declare `module digital.vasic.docprocessor` even though
they are DIFFERENT codebases under different upstream orgs. The
workspace can't register both. Same conflict for: LLMOrchestrator,
LLMProvider, VisionEngine (HelixDevelopment vs vasic-digital
variants with shared module identifier).

`go.work` removed to restore HelixCode inner module's working
compile state.

## Real-fix options (operator decision)

### Option A: Module-path renaming
Distinguish the colliding modules by org-prefix in their module path:

```go
// Dependencies/HelixDevelopment/DocProcessor/go.mod
module dev.helix.docprocessor   // distinguish from digital.vasic.docprocessor
```

Requires upstream repo edits AND every consumer's go.mod update.
Substantial coordinated work.

### Option B: Proper Go-module versioning
Publish each Dependencies/vasic-digital/* module as a real Go module
with semantic version tags. Consumers reference by version:

```go
require digital.vasic.eventbus v1.0.0
// no `replace` needed
```

Requires each repo to be set up with `go mod tidy && git tag v1.0.0`.
Then consumers like HelixAgent become standalone-buildable using
`go.sum` to pin the dependency.

### Option C: Keep current state, accept CONST-051(B) gap
Document that HelixAgent + HelixLLM are NOT standalone-buildable
post-#254/#273 by design — they're only consumable from HelixCode
parent. This trades B↔C tension by sacrificing B for C. CONST-051(B)
would need a note documenting "Go-module consumers operating under
HelixCode parent are exempt from standalone-build expectation."

### Option D: Hybrid — ship a bootstrap script
Add a `scripts/bootstrap-standalone-build.sh` to HelixAgent + HelixLLM
that, on clone, clones the parent's Dependencies/* tree adjacent and
generates a temporary go.work pointing at them. Consumers can build
standalone after running bootstrap once.

## Recommendation

**Option B (proper versioning)** is the architecturally cleanest and
matches Go ecosystem norms. It requires the most upfront work but
permanently resolves the tension. Until then, the current state
(post-Tasks #254/#273) is documented as the working state with
standalone-build deferred to operator decision.

## Audit trail

| Date | Author | Notes |
|---|---|---|
| 2026-05-15 | Claude Opus 4.7 | Tension surfaced during honest assessment after close-out⁴¹. Attempted go.work fix during close-out⁴³, removed after `digital.vasic.docprocessor` cross-org module-name collision broke HelixCode inner compile. Documented as architectural decision point. |
