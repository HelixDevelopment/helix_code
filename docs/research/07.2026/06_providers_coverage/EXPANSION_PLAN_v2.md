# Provider-Coverage Expansion Plan v2 (REWRITE) — HelixCode / HelixAgent / HelixLLM / LLMsVerifier / claude_toolkit

**Date:** 2026-07-08 (rewrite of the same-day v2 draft, following a NO-GO independent review)
**Type:** Planning document only. No product code touched by this rewrite. No `git add`/`commit`.
**Author:** T-planning subagent (read-only research + design).
**Status:** REWRITE — the prior draft of this file was reviewed and returned **NO-GO**
(`scratchpad/review_providers_plan_v2.md`, §11.4.142/§11.4.150) because its Phase 1 and most of
its Phase 2.1 re-planned work that had **already landed, been tagged, and been declared RELEASE
PUBLISHED** before the draft was written — a §11.4.74 catalogue-check failure. This rewrite
corrects every Critical/Important finding from that review and is scoped as a **delta over the
shipped baseline**, not a from-scratch re-plan.

**Anti-bluff (§11.4.6/§11.4.99/§11.4.150):** every claim below is cited either to `file:line`
from an actual read this session, to a `git log`/`git fetch` command actually run this session, or
to an external URL fetched/searched 2026-07-08 (carried forward from the reviewed draft where the
review did not dispute it). Nothing is re-derived from a stale in-tree research doc without
independently confirming the doc's underlying premise (registry/config state) against the live
source this session.

---

## §0 — What is ALREADY LANDED (the baseline this plan extends — READ FIRST)

This section did not exist in the reviewed draft's Phase-1 framing; the draft described this work
as pending/BLOCKED. It is not. All of the following is **already committed on the current branch**
(`feature/helixllm-full-extension`, this repo's `submodules/llms_verifier` HEAD), confirmed by
`git log --oneline -15` + direct file reads this session:

### 0.1 LLMsVerifier C1→C5 capability-resolver chain — LANDED, not BLOCKED

`submodules/llms_verifier/llm-verifier/capabilities/registry.go` (read in full this session,
lines 1-60+) is **already** `providerCapabilitySeeds` (not the old `providerCapabilities` map the
reviewed draft described) — the doc-comment at `registry.go:7-14` states verbatim: *"hand-authored
bootstrap DEFAULTS ONLY — they are NOT verified and MUST be overridden by a live probe ... The
fail-closed resolver (ResolveModelCapability, registry_resolve.go) prefers a fresh per-model
`database.VerificationResult` and reports `unverified` when none exists."* This IS the C3
fail-closed resolver the reviewed draft's Phase 1.4 described as "BLOCKED pending C4+C5 landing."
It has landed. Commits (this session's `git log --oneline -15` on `submodules/llms_verifier`):

| Commit | Date | What |
|---|---|---|
| `309af635` | 2026-07-06 16:41 | C1 — `Message.ToolCalls` |
| `2c507020` | 2026-07-06 17:04 | C2 — RAG/Skills/Plugins capability fields |
| `09f9533c` | 2026-07-06 21:32 | C4 — per-capability verification probes |
| `28e6625a` | 2026-07-06 21:44 | C5 — wire `Verify` |
| `ad18e91f` | 2026-07-06 21:56 | C3 — fail-closed resolver (RED→GREEN) |
| `d1f04e5c` | 2026-07-06 22:11 | review-advisory fix (detector gate + sentinel oracle) |
| `c696c5db` | 2026-07-07 12:07 | extended OpenAI-compatible provider config records + reachability proof (§0.2) |

`docs/research/07.2026/00_master/RESUME.md` (this project's own §11.4.131 session-resumption
file, timestamped 2026-07-08 02:16 — **before** the reviewed draft was written) states in plain
text: *"LLMsVerifier chain: ✅ C1 C2 C4 C5 C3 + advisories landed, combined review GO"* and that tag
`helix-code-1.0.0-dev-0.0.1` is **RELEASE PUBLISHED**, citing `llms_verifier(c696c5db)` as one of
the published submodule SHAs. **This plan does not re-propose the C1–C5 chain in any form.**

### 0.2 Extended-provider config rows — LANDED with partial live proof, not pending

`submodules/llms_verifier/llm-verifier/providers/config.go` (grep run this session,
`registerDefaultProviders()`, 1396 lines) already registers **13 provider config rows** beyond the
long-standing core set — confirmed by direct grep for `pr.providers["<id>"] =`:

`poe` (1086), `perplexity` (1109), `sakana` (1132), `hunyuan` (1155), `xai` (1179), `moonshot`
(1203), `zai` (1228), `fireworks` (1251), `deepinfra` (1274), `ai21` (1297), `reka` (1321) — plus
pre-existing `hyperbolic` (687, real endpoint `https://api.hyperbolic.xyz/v1`, NOT a stub) and
`xiaomi` (1028). **`Baseten` is genuinely absent** (confirmed — no `pr.providers["baseten"]` hit).

Commit `c696c5db` (2026-07-07, one day before the reviewed draft) — *"feat(providers): extended
OpenAI-compatible provider config records + reachability proof (CONST-036/§11.4.28)"* — landed
these rows with a live reachability test (`providers/extended_providers_test.go`) that captured
REAL evidence at `docs/qa/phase4_extended_providers_20260707/`: **poe LIVE-PROVEN (341 models),
zai LIVE-PROVEN (8 models), novita LIVE-PROVEN (143 models)**, fireworks reachable-but-billing-
blocked (HTTP 412 → honest SKIP), 9 credential-absent honest SKIPs.

**This plan does not re-propose adding these 13 config rows.** The genuinely remaining work is
narrower: (a) extend the live-proof coverage from 3 fully-proven + 1 partial to the remaining ~9
rows once API keys are available (operator-gated, §11.4.10 — tracked in §3.4 below, not
scriptable today); (b) reconcile the **separate, sibling** `claude_toolkit` repo's own
alias/key-routing files against this list (§2 below — that file lives in a different repo and was
NOT touched by `c696c5db`).

### 0.3 Operator-resolved clarifications — already answered, not open questions

`docs/research/07.2026/00_master/03_open_clarifications.md` (created 2026-07-06, last updated
2026-07-08 — same day as, but before, the reviewed draft) already records **operator-resolved**
answers for the four items the reviewed draft re-derived independently:

| Item | Operator resolution (already recorded, `03_open_clarifications.md` C1–C4) |
|---|---|
| GPT-Sol | ✅ OpenAI GPT-5.6 "Sol" — no new adapter; add model IDs to the existing OpenAI adapter at GA |
| Google OKF | ✅ RAG/Skills knowledge format via MCP `resources` server — NOT a provider, out of scope |
| ACP | ✅ Google A2A (agent↔agent) — NOT Zed ACP; a protocol thread, not a provider |
| Subquadratic | ✅ BLOCKED-until-GA — no public base URL/auth; no adapter work possible |

`PROVIDER_COVERAGE.md` (same directory) independently arrives at matching conclusions via its own
research pass and cites the same four items — this rewrite treats `03_open_clarifications.md` as
the operator-authoritative record and does not re-litigate any of the four (§11.4.74 — the
reviewed draft's independent re-derivation of the same four answers was itself flagged by the
review as duplicated effort; not repeated here).

---

## §1 — Corrections applied per the NO-GO review (traceability)

| Review finding | Correction applied in this rewrite |
|---|---|
| **C1** — Phase 1 (C1→C5) presented as pending/BLOCKED | §0.1 above states plainly it is LANDED, cites the same 6 commits the review found, and the phased plan below (§3) contains **zero** C1–C5 re-implementation tasks |
| **C2** — Phase 2.1 (Poe/Sakana/Hunyuan registration) duplicated `c696c5db`'s already-landed config rows; `PROVIDER_COVERAGE.md`/`03_open_clarifications.md` operator resolutions not discovered/cited | §0.2 cites `c696c5db` + the exact config.go line numbers + the captured evidence path; §0.3 cites the operator-resolved four items instead of re-deriving them; §2 below reconciles ONLY the claude_toolkit-side gap that genuinely remains (sakana/hunyuan/moonshot/fireworks/deepinfra/ai21/reka aliases — checked this session, see §2.1) |
| **C3** — claude_toolkit's vendored LLMsVerifier freshness claim made with no `git fetch`; wrong target branch for Phase 7 | Ran `git fetch --all --prune` in `claude_toolkit` this session (read-only) — confirmed `claude_toolkit`'s own `origin/main` has NO new commits (`HEAD..origin/main` empty, HEAD `9d12347`). Ran `git fetch origin --prune --tags` inside `claude_toolkit/submodules/LLMsVerifier` this session — confirmed local `17b4bfb6` is now 3 commits behind that remote's `origin/main` (`ae41f5a0`), AND confirmed via `git merge-base --is-ancestor c696c5db origin/main` → **NO** — `c696c5db` lives on `origin/feature/helixllm-full-extension`, not `origin/main`, on the SAME upstream (`vasic-digital/LLMsVerifier`) both this meta-repo's `submodules/llms_verifier` and `claude_toolkit`'s vendored copy point at. §3.5 below re-scopes the Phase-7-equivalent task honestly: it is **blocked on an operator-approved merge of `feature/helixllm-full-extension` → `main`** (§11.4.167 — no trunk merge without explicit approval), not a script that can run today against `origin/main` |
| **I1** — every `helix_code/helix_code/...` citation is a doubled, non-existent path | Confirmed this session: `ls helix_code/helix_code` → no such directory; the real paths are `helix_code/internal/server/*.go` and `helix_code/internal/llm/openai_provider.go` (single level, per this project's own `CLAUDE.md` §3.2.1). Every citation below uses the corrected single-level path. The underlying finding (HelixCode's own server registers no `/v1/chat/completions` or `/v1/messages` route) is unchanged — re-confirmed by `grep -rn "chat/completions\|/v1/messages\|router.POST" helix_code/internal/server/*.go` this session, zero hits |
| **I2** — "Xiaomi dual OAI+Anthropic" conflated vendor API with local adapter | Confirmed this session (`grep` on `submodules/helix_agent/internal/llm/providers/xiaomi/xiaomi.go`, 605 lines): the local adapter implements **only** `https://api.xiaomimimo.com/v1/chat/completions` (OpenAI-style) — zero occurrences of `/v1/messages`, `anthropic-version`, or `x-api-key`. Xiaomi's *vendor* API additionally documents an Anthropic-compatible surface (`03_open_clarifications.md`), but the **local HelixAgent adapter does not implement or need it** — the "already-wired" classification for Xiaomi stands (the dedicated adapter is real, substantial code), the "dual wire" framing is now scoped correctly as describing the external vendor, not local code |

**Preserved unchanged** (review confirmed these accurate — not re-derived, not altered):
the 1-net-new-adapter conclusion (dual OpenAI+Anthropic wire facade for HelixCode's own server,
§3.1 below); the `generic/generic.go:27-30` config-only seam; claude_toolkit's PATH-detection
implementation state + the R5 port-ownership hardening as a genuinely-open gap (confirmed this
session — `detect_helixagent_record()` at `claude-providers.sh:178-` accepts *any* non-empty
`data[].id` list via `jq -r '.data[].id? // empty'` rather than asserting the known
`helix-debate`/`helix-llm` ids are present, lines 185-186/207/210); GPT-Sol and Subquadratic held
at `UNCONFIRMED-needs-endpoint` (re-searched 2026-07-08, still no public GA/base-URL — see the
reviewed draft's Step-2 citations, not disputed by the review and not re-searched again here to
avoid duplicate WebSearch spend); §11.4.6 honesty markers throughout.

---

## §2 — claude_toolkit alias reconciliation (the genuinely-remaining slice of the old Phase 2.1)

Checked this session (`cat scripts/providers/key-aliases.json scripts/providers/overrides.json`
in `claude_toolkit`, a **separate sibling repo** at
`/home/milos/Factory/projects/tools_and_research/claude_toolkit` — NOT touched by `c696c5db`,
which only landed in `submodules/llms_verifier`'s `config.go`):

| Provider (from §0.2's landed config.go list) | Already in claude_toolkit's own alias files? | Action needed |
|---|---|---|
| `poe` | **YES** — `POE_API_KEY→poe` (+ `ApiKey_Poe`) in `key-aliases.json`; `poe.strong_model/fast_model` pinned in `overrides.json` | None |
| `xai` | **YES** — `xai.base_url` pinned in `overrides.json` | None |
| `zai` | **YES** — as `zai-coding-plan` (`ZAI_API_KEY→zai-coding-plan`) | None |
| `xiaomi` | **YES** — `XIAOMI_MIMO_API_KEY→xiaomi`, native transport + base_url pinned | None |
| `hunyuan` | **PARTIAL/AMBIGUOUS** — `TENCENT_CLOUD_API_KEY→tencent-tokenhub` exists, but there is no `hunyuan` alias and `tencent-tokenhub` is not confirmed to be the same endpoint as `https://api.hunyuan.cloud.tencent.com/v1` | **Genuinely open** — reconcile whether `tencent-tokenhub` already targets Hunyuan or is a distinct Tencent product, before adding a possibly-duplicate `hunyuan` alias (§11.4.122 no-silent-removal applies in reverse: don't silently ADD a duplicate either) |
| `sakana` | **NO** | **Genuinely open** — no existing alias/override; safe to add |
| `moonshot` | **AMBIGUOUS** — `kimi-for-coding` exists with `base_url: https://api.kimi.com/coding/`, distinct from `PROVIDER_COVERAGE.md`'s `https://api.moonshot.ai/v1` general-chat endpoint; `config.go`'s own comment at line 1202 notes moonshot is "distinct from the pre-existing 'kimi' record (api.moonshot.cn, China endpoint)" | **Genuinely open** — three possibly-distinct Kimi/Moonshot surfaces (China `kimi.cn`, international `moonshot.ai`, coding-specific `kimi.com/coding`) need reconciling before adding an alias, not a blind add |
| `fireworks`, `deepinfra`, `ai21`, `reka` | **NO** | **Genuinely open** — no existing alias/override; safe to add once operator confirms priority (these are lower-salience per the original Wave-2 rollout ordering, which the review did not dispute) |

**Corrected task (replaces the old Phase 2.1):** a pre-flight reconciliation pass — NOT a blind
"add three aliases" task — resolving the `hunyuan`/`tencent-tokenhub` and
`moonshot`/`kimi-for-coding` ambiguities via a live endpoint comparison (does `tencent-tokenhub`'s
pinned base_url, if any, resolve to the Hunyuan API?) before any new alias is written, then adding
`sakana` (unambiguous) + the four lower-priority providers (`fireworks`/`deepinfra`/`ai21`/`reka`)
with the SAME `transport: router` convention already used for HelixAgent (OpenAI-shaped, not
`native`).

---

## §3 — Phased plan (genuinely-remaining work only)

Every phase below is a **delta task** — nothing here re-implements §0's landed baseline.

### Phase A — HelixLLM-as-tracked-provider registration in LLMsVerifier (still open)

This is the ONE piece of the old Phase 1 that was genuinely NOT yet landed (the C1–C5 chain is a
generic capability-resolver mechanism; HelixLLM itself registering as a tracked provider instance
is a separate, still-open task). `submodules/helix_llm/README.md` (read this session) confirms
HelixLLM exposes both OpenAI-compatible (`/v1/chat/completions`, `/v1/models`) and Anthropic-
compatible (`/v1/messages`) surfaces on one server (default port `8443` TLS; proven-live plain-HTTP
`llama-server` sidecar at `:18434` per `RESUME.md`'s live-server section — base URL WITHOUT `/v1`
per the corrected `PHASE2_BLOCKERS_INVESTIGATION.md` OQ1 finding).

**A.1** Register a `helixllm` `ProviderDescriptor` in the now-landed seed/resolver mechanism
(`registry.go` seeds + `registry_resolve.go`'s fail-closed resolver), sourcing `BaseURL` from
`HELIX_LLM_LOCAL_OPENAI_ENDPOINT` (no `/v1` suffix), `Wire:"openai"`, health path
`/internal/health`.
**A.2** If the landed resolver supports only one wire per logical provider ID, register a second
ID (e.g. `helixllm-anthropic`) pointed at the same base URL with `Wire:"anthropic"` for the
`/v1/messages` surface — confirm against the landed `registry_resolve.go` implementation
(genuinely not yet checked this session — a real open sub-task, not carried forward as a guess).
**A.3** Acceptance: a real `curl {endpoint}/v1/models` (or the resolved endpoint) non-empty
`data[]`, captured to `docs/qa/<run-id>/helixllm_verifier_registration/models.json`, plus a real
`VerificationResult` row for `provider="helixllm"` from the landed C4 probe mechanism —
`Responsive=true`, at least one `Supports*` flag backed by a real probe artifact, never a
config-only "registered" claim.

### Phase B — claude_toolkit alias reconciliation (§2 above)

**B.1** Resolve the `tencent-tokenhub` vs `hunyuan` ambiguity and the `kimi-for-coding` vs
`moonshot` ambiguity by comparing live-resolved base URLs (§2 table) — a one-command check, not a
design change.
**B.2** Add `sakana` (unambiguous, no existing alias) + `fireworks`/`deepinfra`/`ai21`/`reka`
(lower priority, add after B.1 resolves) to `key-aliases.json`/`overrides.json` with
`transport: router` (OpenAI-shaped).
**B.3** Acceptance: `claude-providers sync` with real keys produces new `<id>.env` files with
correct `CMA_PROVIDER_BASE_URL`/`TRANSPORT`; `scripts/tests/verify_providers_live.sh` produces
`proof/<n>-sakana-live.txt` etc. showing a real, non-simulated round-trip — mirroring the existing
evidence pattern already in that repo.

### Phase C — claude_toolkit HelixAgent-provider R5 port-ownership hardening (still open, confirmed)

**C.1** `detect_helixagent_record()` (`claude-providers.sh:178-`) currently accepts any non-empty
`data[].id` list from a `:8100` responder. Harden it to assert the returned id set contains
`helix-debate` OR `helix-llm` (the `CMA_HELIXAGENT_STRONG`/`_FAST` env-overridable defaults already
declared at lines 185-186) before treating the port as HelixAgent's — prevents a false-positive
registration against an unrelated service coincidentally listening on the same port (a
§11.4.174-class port-ownership hazard).
**C.2** Acceptance: (1) real `/v1/models` response containing the known ids, captured; (2) a
NEGATIVE-case test proving a fake responder returning `{"data":[]}` or an unrelated JSON shape does
NOT get registered as the `helixagent` alias.

### Phase D — Dual OpenAI-style + Anthropic-style wire facade for HelixCode's own API (the one net-new adapter — unchanged from the reviewed draft's Phase 5, review-confirmed accurate)

Confirmed gap, corrected paths (I1): `grep -rn "chat/completions\|/v1/messages\|router.POST"
helix_code/internal/server/*.go` this session — zero hits; HelixCode's own server (`server.go`,
`handlers.go`, `llm_generate.go`, `qa_handlers.go`, `specify.go`) registers only its own custom
`/api/v1/llm/generate` shape, never serves an OpenAI or Anthropic wire to external callers.

**D.1** Read HelixLLM's own dual-facade implementation (not yet read this session — first
sub-task) as the worked precedent, per the fork points already enumerated in the reviewed draft's
R4 (request-shape `system`-field handling, response-shape content-block-array-vs-flat-string,
SSE-framing divergence, tool-calling schema translation, auth-header divergence) — none of that
enumeration was disputed by the review, carried forward unchanged.
**D.2** Design a shared internal `GenerateRequest`/`GenerateResult` extending
`helix_code/internal/server/llm_generate.go`'s existing generate/stream core.
**D.3** Add `POST /v1/chat/completions` + `POST /v1/messages` route groups to
`helix_code/internal/server/server.go`, both delegating to the shared core, with two distinct SSE
encoders.
**D.4** Bidirectional tool-calling schema translation (JSON-Schema wrapper-key difference only).
**D.5** Register both routes with the landed LLMsVerifier resolver as a `helixcode`/
`helixcode-anthropic` provider pair (mirrors Phase A.2).
**D.6** Acceptance: real curl round-trips on both endpoints (non-empty `choices[0].message.content`
/ `content[0].text`), streaming variants with correct SSE framing, a tool-call round-trip on both
shapes — all captured to `docs/qa/<run-id>/dual_wire_facade/`, no metadata-only pass accepted.

### Phase E — Setup/PATH-install script gap-fill (small, unchanged from reviewed draft §1.7/Phase 4 — not disputed)

`install_helix_path.sh` (304 lines, read in full in the reviewed draft's session) already
builds+installs all four binaries (`helixcode`, `helixagent`, `helixllm`, `llms-verifier`), no
sudo, idempotent rc-patching, black-box `--version` probe per binary. The review did not dispute
this. Remaining gaps:

**E.1** (optional) A fifth component row symlinking `claude_toolkit`'s own CLI entry point,
compliant with §11.4.177 (the symlink target resolves its own `SCRIPT_DIR`, never a copy).
**E.2** Extend the existing black-box test with a `helixllm --help | grep -q '\-\-mode'` sanity
assertion (confirms the mode-flag surface the README documents is actually present in the built
binary) — one additional assertion within the existing philosophy, not a new mechanism.

### Phase F — Provider-verification test extension (scoped to what Phase A/B actually add)

**F.1** Confirm whether `verify_providers_live.sh`'s provider-iteration loop is already
env-file-driven (and therefore automatically picks up new `<id>.env` files from Phase B) or has a
hardcoded provider-id list needing the new ids appended — a read-only check, not yet performed
this session.
**F.2** Extend `test_providers.sh`'s hermetic fixture set with the Phase-B-added providers'
fake catalog entries, so the resolver's classification logic is exercised without live network
calls.
**F.3** Once keys are available (operator-gated, §11.4.10), extend the `c696c5db` live-proof
harness (`providers/extended_providers_test.go`) from its current 3-fully-proven-of-13 state
toward full coverage of the already-registered §0.2 config rows — this is NOT new adapter work,
it is closing the evidence gap the tag's own §11.4.138 honesty-correction entry
(`docs/research/07.2026/00_master/PROVIDER_COVERAGE.md`, bottom section) already tracks as a
named follow-up.

### Phase G — Operator-gated / non-actionable-today items (tracked, not scheduled)

**G.1** claude_toolkit's vendored `submodules/LLMsVerifier` bump — per §1's C3 correction, this is
**honestly blocked**, not scriptable today: the C1–C5 + extended-provider commits live on
`origin/feature/helixllm-full-extension` of `vasic-digital/LLMsVerifier`, not `origin/main`. Two
honest paths, either requiring an explicit operator decision (§11.4.66):
&nbsp;&nbsp;(a) wait for `feature/helixllm-full-extension` → `main` merge (operator-approved per
§11.4.167), then run the existing `11_claude_toolkit.md` §2.3 fetch/merge/rebuild procedure
against the now-correct `origin/main`; or
&nbsp;&nbsp;(b) if the operator wants the extended-provider work in `claude_toolkit` sooner, point
its vendored submodule at the feature branch explicitly (a deviation from the "always track main"
convention that needs its own operator sign-off).
No script is authored for either path until the operator picks one — scripting path (a) against
today's `origin/main` would reproduce exactly the bug the review caught (Phase 7's own acceptance
criterion would fail on a live run, as the review demonstrated).
**G.2** Qwythos-9B training-data-provenance operator sign-off (`dev.ua`'s "trained on
Claude-distilled data" finding) — unchanged from the reviewed draft, not yet asked.
**G.3** GPT-Sol / Subquadratic — held at `UNCONFIRMED-needs-endpoint` / `BLOCKED-until-GA`, per
`03_open_clarifications.md`'s already-operator-resolved verdicts (§0.3). No adapter work; revisit
only on a GA/public-API announcement.
**G.4** Baseten — confirmed genuinely absent from `config.go` (unlike Hyperbolic, which already
has a real config row per §0.2). Per-deployment base URL, no single global endpoint — needs a
concrete deployment URL injected before any work, per the reviewed draft's original §1.2 note
(not disputed by the review).

---

## §4 — Net-new-work count (post-reconciliation)

**Zero** re-implementation of the landed C1–C5 chain or the 13 landed config rows. **Genuinely
remaining, actionable-today items: 6** (Phase A HelixLLM-registration, Phase B alias
reconciliation, Phase C R5 hardening, Phase D dual-wire facade — the one net-new adapter, Phase E
setup-script gap-fill, Phase F test extension). **Operator-gated, not scriptable today: 4** (Phase
G.1–G.4). This plan does not re-plan any shipped work; every task above is either newly-discovered
this session (Phase A.2's resolver-capacity question, §2's alias ambiguities) or a review-confirmed
still-open gap (Phase C, Phase D, Phase E).

---

## Sources verified 2026-07-08

In-tree source read/grepped THIS rewrite session (facts, no guessing — §11.4.6):
- `submodules/llms_verifier/llm-verifier/capabilities/registry.go:1-60` (full seed-map header +
  first provider block)
- `submodules/llms_verifier/llm-verifier/providers/config.go` (full `pr.providers["<id>"] =` grep,
  1396 lines)
- `git -C submodules/llms_verifier log --oneline -15`, `git -C submodules/llms_verifier branch
  --show-current`, `git -C submodules/llms_verifier remote -v`
- `docs/research/07.2026/00_master/RESUME.md` (full)
- `docs/research/07.2026/00_master/PROVIDER_COVERAGE.md` (full)
- `docs/research/07.2026/00_master/03_open_clarifications.md` (full)
- `helix_code/internal/server/` directory listing + `grep -rn "chat/completions\|/v1/messages\|
  router.POST" helix_code/internal/server/*.go` (zero hits, corrected single-level path per I1)
- `submodules/helix_agent/internal/llm/providers/xiaomi/xiaomi.go` (grep for wire-shape markers,
  605 lines total — confirmed OpenAI-only)
- `/home/milos/Factory/projects/tools_and_research/claude_toolkit`: `git fetch --all --prune`
  (read-only), `git log --oneline -1` (HEAD `9d12347`), `git log --oneline HEAD..origin/main`
  (empty)
- `claude_toolkit/submodules/LLMsVerifier`: `git fetch origin --prune --tags` (read-only),
  `git log --oneline -1` (local `17b4bfb6`), `git log --oneline origin/main -1` (`ae41f5a0`),
  `git merge-base --is-ancestor c696c5db origin/main` → NO, `git branch -r --contains c696c5db`
  → `origin/feature/helixllm-full-extension`
- `claude_toolkit/scripts/providers/key-aliases.json` + `overrides.json` (full contents, this
  session)
- `claude_toolkit/scripts/claude-providers.sh` (grep for `detect_helixagent_record`,
  `data[].id`, `helix-debate`/`helix-llm` — lines 178-210)

Carried forward, NOT re-searched this rewrite session (the review did not dispute these; re-
searching would be duplicate WebSearch spend against a same-day finding): Sakana Fugu vendor-
confirmed OpenAI-compat wire shape, Subquadratic private-beta status, Google OKF /
MCP-resources classification, GPT-5.6 "Sol" limited-preview status, Qwythos-9B self-host +
provenance flag — all originally cited in the reviewed draft's Step-2 table with URLs accessed
2026-07-08, unchanged by this rewrite's corrections (which are structural/state corrections, not
provider-identity corrections).

**Review this rewrite responds to:** `scratchpad/review_providers_plan_v2.md` (NO-GO, Critical
C1/C2/C3, Important I1/I2 — all addressed above; see §1 traceability table).
