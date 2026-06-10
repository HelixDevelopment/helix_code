# SDK-Bump Code Review — Independent Reviewer (§11.4.125)

**Date:** 2026-06-10
**Repo:** HelixCode (root `/Volumes/T7/Projects/HelixCode`); inner module `helix_code/` (`dev.helix.code`, go 1.26)
**Reviewer role:** independent code-review subagent — VERIFY only, no edits/commits/pushes.
**Change under review:** uncommitted SDK version bumps + one minimal API-adaptation fix.

---

## 1. Scope of the change (confirmed)

`git status --short` shows exactly three modified files, all part of this change:

- `helix_code/go.mod`
- `helix_code/go.sum`
- `helix_code/internal/memory/providers/zep_provider.go`

(`helix_agent` + docs appearing elsewhere in the broader tree are pre-existing and NOT part of this diff — confirmed: only the three files above are in `git status`.) No secret/credential present in the diff (api-key/secret/token/password/private-key scan over `git diff` returned empty).

### Source bumps in go.mod (direct)
- `getzep/zep-go/v3` v3.10.0 → **v3.23.0**
- `Azure/azure-sdk-for-go/sdk/azcore` v1.16.0 → **v1.22.0**
- `Azure/azure-sdk-for-go/sdk/azidentity` v1.8.0 → **v1.13.1** (security advisory GO-2024-2918 line)
- `aws/aws-sdk-go-v2` v1.32.7 → **v1.42.0** (+ config v1.28.7→v1.32.24, credentials v1.17.48→v1.19.23, smithy-go v1.22.1→v1.27.2)
- transitive aws indirect deps aligned (imds, configsources, endpoints/v2, sso, ssooidc, sts, + new internal/v4a and service/signin); azure internal v1.10→v1.12, MSAL v1.2.2→v1.6.0; `golang.org/x/net` v0.54→v0.55 (pulled by the bumps).
- `bedrockruntime` left at **v1.23.1** (step 4 correctly deferred); `gin` left at **v1.12.0**.

### Source fix in zep_provider.go (2 sites)
Lines 378 and 871: `TargetNodeName: targetName` → `TargetNodeName: zep.String(targetName)`.

---

## 2. Verification (commands run this session)

**Resolved versions — `go list -m`:**
```
github.com/getzep/zep-go/v3                       v3.23.0
github.com/Azure/azure-sdk-for-go/sdk/azidentity  v1.13.1   <-- security advisory line
github.com/Azure/azure-sdk-for-go/sdk/azcore      v1.22.0
github.com/aws/aws-sdk-go-v2                       v1.42.0
github.com/aws/aws-sdk-go-v2/service/bedrockruntime v1.23.1 <-- step 4 deferred, unchanged
github.com/gin-gonic/gin                          v1.12.0   <-- unchanged
```
All six match the claim exactly.

**`go mod verify`:** `all modules verified`.

**Build — `go build ./...`:** exit 0. (Only output is pre-existing macOS linker noise `ld: warning: ignoring duplicate libraries: '-lobjc'` on the desktop/aurora_os/harmony_os Fyne apps — unrelated to the SDK bumps, not an error.)

**Vet — `go vet ./internal/llm/... ./internal/memory/...`:** exit 0, no diagnostics.

**Tests — `go test ./internal/memory/... ./internal/llm/... -count=1`:** **ok** for every package with tests:
```
ok  dev.helix.code/internal/memory                    2.378s
ok  dev.helix.code/internal/memory/providers         31.644s
ok  dev.helix.code/internal/llm                       63.972s
ok  dev.helix.code/internal/llm/compression / compressioniface / i18n / litellm /
    promptcache / providers/cerebras / providers/replicate / routing / vision   all ok
```
No FAIL. (Packages with no test files reported `[no test files]` — not a failure.)

---

## 3. zep_provider.go fix correctness (§11.4.6 — verified as FACT, not asserted)

Inspected the resolved module source at
`~/go/pkg/mod/github.com/getzep/zep-go/v3@v3.23.0`:

- `graph.go:71  SourceNodeName *string` and `graph.go:86  TargetNodeName *string` — in v3.23 **both** node-name fields are `*string`. (In v3.10 `TargetNodeName` was a plain `string`; the bump changed the type, which is exactly why the build forced this edit.)
- `pointer.go:70  func String(s string) *string` — `zep.String` is the canonical pointer helper.
- Both call sites assign `targetName` from a plain `string` (`targetNode.GetName()` at the Update site; local `tn string` at `addFactTriple`), so `zep.String(targetName)` is type-correct.
- The edit makes `TargetNodeName` mirror the already-present `SourceNodeName: zep.String(sourceName)` sibling line at both sites.

**Conclusion:** pure type-adaptation to the v3.23 `*string` field signature. No semantic/logic change — the wire payload (`target_node_name`, omitempty) is unchanged. Correct and minimal.

---

## 4. Risk assessment

- **azidentity v1.13.1** lands the GO-2024-2918 fix — security-positive, the headline reason for the bump.
- aws-sdk core jump (v1.32→v1.42) is large but `bedrockruntime` (the only aws *service* client HelixCode consumes here) is held at v1.23.1, and the affected packages compile, vet clean, and pass tests against the new core — the deferral is sound and the surface is exercised by the passing `internal/llm` suite (Bedrock provider lives there).
- No production code imports changed beyond the single zep adaptation; no mock-from-production regression introduced.
- `go.sum` updated consistently (`go mod verify` clean).

No blocking findings. No warnings of substance (the `-lobjc` linker line is pre-existing host noise).

---

## VERDICT: **GO**

**Version-landed confirmations:**
- zep-go/v3 **v3.23.0** ✓
- azidentity **v1.13.1** ✓ (security advisory GO-2024-2918)
- azcore **v1.22.0** ✓
- aws-sdk-go-v2 core **v1.42.0** ✓
- bedrockruntime **v1.23.1** (deferred, unchanged) ✓
- gin **v1.12.0** (unchanged) ✓

**Build/vet/test:**
- `go build ./...` — **exit 0** (ok; only pre-existing `-lobjc` linker warnings)
- `go vet ./internal/llm/... ./internal/memory/...` — **exit 0** (ok)
- `go test ./internal/memory/... ./internal/llm/... -count=1` — **ok** (no FAIL)
- `go mod verify` — **all modules verified**

zep fix at lines 378/871 confirmed correct (`*string` field + `zep.String` helper + SourceNodeName sibling pattern). Scope = only go.mod/go.sum/zep_provider.go. No secrets.
