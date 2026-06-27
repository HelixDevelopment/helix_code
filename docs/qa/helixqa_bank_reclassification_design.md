# HelixQA Bank Reclassification Design — 5 Over-Assert / Drift Cases

**Date:** 2026-06-24
**Submodule:** `submodules/helix_qa` (module `digital.vasic.helixqa`, go 1.26)
**Working-tree HEAD:** `26f686a` · **origin/main HEAD:** `d45bd3d` (13 commits ahead; HEAD is ancestor)
**Scope:** Design-only. READ-ONLY against the submodule — no edit/commit/push/checkout performed.

## Why these 5 banks are wrong (and the server is right)

A V&V run against the live, correct server on `:18080` found 5 HelixQA bank
cases asserting things the server does NOT do — and the server is correct,
the banks over-assert / have drifted:

- **3 OVER-ASSERT** (`HXC-ENS-001`, `HXC-LSP-001`, `HXC-SKILL-001`) assert that
  `GET /api/v1/server/info` advertises feature flags (`ensemble`,
  `lsp_enabled`, `skills_enabled`) for capabilities that are **genuinely
  unshipped** in helix_code (`internal/{ensemble,lsp,skills}` are ABSENT).
  Advertising those flags would itself be a §11.4.122 bluff, so the server
  correctly omits them — the bank is the bluff, not the server.
- **2 DRIFT** (`HXC-API-015`, `HXC-SEC-010`) assert stale response substrings;
  the server returns the correct (different) text.

## Upstream-redundancy check (FACT)

`git fetch origin main` succeeded. For all 5 bank files,
`git diff HEAD origin/main -- <file>` reports **SAME**, and the advanced
`origin/main` copies of the 5 cases are byte-identical to the working tree.

**=> The advanced main has NOT reclassified ANY of these 5.** Unlike the
challenges/containers cases, there is **no upstream redundancy** — the
over-assert / drift is still live on the advanced main, so the edits below
must be authored (they are not already done upstream).

## Schema / executor facts (FACT, from source)

- `pkg/testbank/schema.go` `TestStep`: step-level skip is
  `Skip bool \`yaml:"_skip"\`` + `SkipReason string \`yaml:"_skip_reason"\``.
- `pkg/autonomous/http_executor.go:152` honors `step.Skip` **first**, before
  the request fires, returning `ActionResult{Skipped:true}` (verdict = SKIP,
  not PASS, not FAIL). The code comment calls this "strictly more honest than
  letting the request go out and producing a confusing PASS/FAIL" — i.e. the
  built-in §11.4.3 honest-skip mechanism.
- `expect_body_contains` (`http_executor.go:326`) is a single
  `strings.Contains(body, X)` positive substring check, **case-sensitive**.
  There is **NO negation field** (`expect_body_not_contains`) and **NO OR**
  in the current schema.

**Consequence for option (b):** asserting "flag absent OR false" is **NOT
expressible** in the current schema (no negation, no OR). Option (b) would
require a runner change. Option (a) (`_skip` + reason) is the honest,
schema-supported, executor-documented path available today.

---

## Per-case reclassification

### 1. HXC-ENS-001 — `banks/helixcode-ensemble-members.yaml`
| field | value |
|---|---|
| Current assertion | step `expect_status: 200`, `expect_json_path: "$.info"`, **`expect_body_contains: "ensemble"`** |
| Redundant upstream? | **No** — origin/main identical |
| Class | OVER-ASSERT (ensemble unshipped; `internal/ensemble` absent) |

**Honest edit (recommended, option a):** add to the single step —
```yaml
        _skip: true
        _skip_reason: "SKIP-OK: ensemble provider is unshipped in helix_code (internal/ensemble absent); /server/info correctly does NOT advertise an 'ensemble' flag — asserting it would advertise a §11.4.122 bluff. Flip _skip:false when ensemble ships."
```
Keep `expect_body_contains: "ensemble"` in place (un-deleted) so the assertion
reactivates verbatim the moment `_skip` flips. Do NOT touch HXC-ENS-002/003
(recordingqa runtime-proof siblings — out of scope).

### 2. HXC-LSP-001 — `banks/helixcode-lsp.yaml`
| field | value |
|---|---|
| Current assertion | `expect_status: 200`, `expect_json_path: "$.info"`, **`expect_body_contains: "lsp_enabled"`** |
| Redundant upstream? | **No** — origin/main identical |
| Class | OVER-ASSERT (LSP unshipped; `internal/lsp` absent) |

**Honest edit (option a):**
```yaml
        _skip: true
        _skip_reason: "SKIP-OK: LSP is unshipped in helix_code (internal/lsp absent); /server/info correctly omits features.lsp_enabled — asserting it would advertise a §11.4.122 bluff. Flip _skip:false when LSP ships."
```

### 3. HXC-SKILL-001 — `banks/helixcode-skills.yaml`
| field | value |
|---|---|
| Current assertion | `expect_status: 200`, `expect_json_path: "$.info"`, **`expect_body_contains: "skills_enabled"`** |
| Redundant upstream? | **No** — origin/main identical |
| Class | OVER-ASSERT (skills unshipped; `internal/skills` absent) |

**Honest edit (option a):**
```yaml
        _skip: true
        _skip_reason: "SKIP-OK: skills are unshipped in helix_code (internal/skills absent); /server/info correctly omits features.skills_enabled — asserting it would advertise a §11.4.122 bluff. Flip _skip:false when skills ship."
```

### 4. HXC-API-015 — `banks/helixcode-full-qa-api.json`
| field | value |
|---|---|
| Current assertion | `expect_status: 401`, **`expect_body_contains: "authorization"`** (lowercase) |
| Server actual (correct) | 401 with body `"Authorization header required"` (capital A) |
| Redundant upstream? | **No** — origin/main identical |
| Class | DRIFT (case mismatch; `strings.Contains` is case-sensitive so `"authorization"` ≠ `"Authorization header required"`) |

**Honest edit (assertion-string correction):**
```json
"expect_body_contains": "Authorization header required"
```
`expect_status: 401` is already correct — leave it. The case (negative-auth)
stays a real PASS once the substring matches reality.

### 5. HXC-SEC-010 — `banks/helixcode-security-validation.json`
| field | value |
|---|---|
| Current assertion | `expect_status: 400`, **`expect_body_contains: "invalid_request"`** |
| Server actual (correct) | 400 with body `"Invalid request"` |
| Redundant upstream? | **No** — origin/main identical |
| Class | DRIFT (stale snake_case token; server returns human string) |

**Honest edit:**
```json
"expect_body_contains": "Invalid request"
```
`expect_status: 400` already correct (the server correctly 400s, not 500s).

---

## Recommended approach

1. **Over-assert trio (ENS/LSP/SKILL-001): option (a) `_skip:true` + `_skip_reason`.**
   - §11.4.122-honest: the case is preserved, never silently dropped.
   - Schema-supported + executor-documented as the honest non-execution path.
   - Produces SKIP, not a false PASS (§11.4.6) — the server is correct, the
     feature is genuinely absent.
   - Fully reversible: flip `_skip` to `false` the instant the feature ships
     (the original assertion is left intact in the step).
   - **Reject option (b)** for now — "flag absent OR false" is not expressible
     (no `expect_body_not_contains`, no OR in the schema). **Reject option (c)**
     (delete the assertion) — it degrades the case into a generic
     "server/info returns $.info" test whose name still claims "advertises X",
     which is itself misleading.
   - **Tracked follow-up (optional, stronger anti-bluff):** add an
     `expect_body_not_contains` field to `TestStep` + executor + tests, then
     convert these 3 SKIPs into positive PASSes that assert the flag is
     truthfully ABSENT (guarding against accidental future advertisement).
     This is a runner change with its own four-layer coverage — out of scope
     for the bank-only fix.

2. **Drift pair (API-015 / SEC-010): assertion-string correction** to the
   server's actual correct response (`"Authorization header required"`,
   `"Invalid request"`). Keeps both as real, passing negative/security guards.

---

## Build-test plan for the gitlink bump

helix_qa is a **direct require** of helix_code, wired in `helix_code/go.mod`:
- `require digital.vasic.helixqa v0.0.0-...` (line 13)
- `replace digital.vasic.helixqa => ../submodules/helix_qa` (line 216)

After staging the bank edits in the submodule and bumping the gitlink, from
`helix_code/`:
```bash
cd /Volumes/T7/Projects/helix_code/helix_code
go build ./...
go test ./internal/helixqa/... ./internal/server/...
```

**go.mod replace-key / llmsverifier risk (assessed):**
- helix_qa's own `go.mod` carries a `replace` block of **relative** sibling
  paths (`digital.vasic.llmsverifier => ../llms_verifier/llm-verifier`, plus
  challenges, containers, doc_processor, llm_orchestrator, llm_provider,
  security, vision_engine). Go **ignores `replace` in a non-main (dependency)
  module** — only helix_code's (main) replace block resolves these.
- **Already mitigated:** helix_code's `go.mod` line 273 already has
  `replace digital.vasic.llmsverifier => ../submodules/llms_verifier/llm-verifier`,
  and the block also covers challenges/containers/doc_processor/llm_orchestrator/
  llm_provider/security/vision_engine. So the llmsverifier path the integration
  plan flagged is covered.
- **Residual risk to watch on the bump:** if the advanced helix_qa main
  introduces a NEW transitive `digital.vasic.*` require not yet in helix_code's
  replace block, `go build ./...` will fail with an unresolved/`no required
  module provides package` error → add the matching
  `replace digital.vasic.<name> => ../submodules/<name>` and re-run. Also
  confirm the go 1.26 toolchain matches (both modules declare `go 1.26`).
- This bank-only change touches **no Go source**, so a build/test failure on
  the bump would come from the gitlink advancing other (unrelated) commits on
  helix_qa, not from these 5 YAML/JSON edits.
