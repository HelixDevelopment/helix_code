# CodeGraph incorporation — Phase C close-out: caveat resolution

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-05-20 |
| Last modified | 2026-05-20 |
| Status | active |

> Verbatim 2026-05-19 operator mandate: *"all existing tests and Challenges do
> work in anti-bluff manner - they MUST confirm that all tested codebase really
> works as expected! We had been in position that all tests do execute with
> success and all Challenges as well, but in reality the most of the features
> does not work and can't be used! This MUST NOT be the case and execution of
> tests and Challenges MUST guarantee the quality, the completition and full
> usability by end users of the product!"*

This document closes the three honest caveats left open after the CodeGraph
(`@colbymchenry/codegraph` v0.7.12) incorporation into HelixCode. Authority:
HelixCode root CLAUDE.md / CONSTITUTION.md — CONST-035, CONST-050(B), §11.4.3
topology-aware dispatch, §11.4.21 operator-block details.

## Table of contents

- [Caveat 1 — Kimi CLI + Qwen Code true end-to-end](#caveat-1--kimi-cli--qwen-code-true-end-to-end)
- [Caveat 2 — helix_qa autonomous QA session](#caveat-2--helix_qa-autonomous-qa-session)
- [Caveat 3 — anti-bluff covenant in QWEN.md across submodules](#caveat-3--anti-bluff-covenant-in-qwenmd-across-submodules)

## Caveat 1 — Kimi CLI + Qwen Code true end-to-end

Both per-agent Challenges were re-driven non-interactively this session against
the real CodeGraph MCP server (624,092-node graph of the HelixCode repo). Result:
**both agents are genuinely backend-blocked** — reported HONESTLY, not faked.

### Kimi CLI — connect-only + tool-discovery PASS (end-to-end operator-blocked)

`tools/codegraph/challenges/cg-challenge-05-kimi.sh` re-run. The transcript
(`CG-CHALLENGE-05-kimi-transcript.txt`, 1935 bytes) proves Kimi's MCP loader
connected to the `codegraph` server and enumerated **all 9** `codegraph_*` tools:

```
MCPServerSnapshot(name='codegraph', status='connected', tools=(
    'codegraph_search', 'codegraph_context', 'codegraph_callers',
    'codegraph_callees', 'codegraph_impact', 'codegraph_node',
    'codegraph_explore', 'codegraph_status', 'codegraph_files'))
```

The agent step then aborts:

```
Error code: 429 - {'error': {'message': "You've reached kimi monthly usage limit
for this billing cycle. ..."}}
```

This is the strongest available proof short of end-to-end: the MCP transport is
fully functional and the tools are reachable; only the LLM backend quota blocks
the final answer. Per §11.4.3 the connect-only fallback is reported as
connect-only — NEVER as end-to-end.

### Qwen Code — connect-only PASS (end-to-end operator-blocked)

`tools/codegraph/challenges/cg-challenge-07-qwen.sh` re-run. Transcript
(`CG-CHALLENGE-07-qwen-transcript.txt`, 136 bytes):

```
Qwen OAuth free tier was discontinued on 2026-04-15. Run /auth to switch to
Coding Plan, OpenRouter, Fireworks AI, or another provider.
```

Connect-only PASS captured (config registers codegraph MCP server + binary
reachable). End-to-end NOT proven.

### Honest disposition

End-to-end Kimi/Qwen verification is **operator-blocked**. Filed as
`docs/Issues.md` → **HXC-010** with `**Status:** Operator-blocked` +
`**Operator-Block-Details:**` per constitution §11.4.21. UNBLOCK CONDITION:
operator supplies Kimi API quota/credentials and/or Qwen Code LLM credentials,
then re-runs the two Challenges. Claude Code, OpenCode, and Crush remain
true-end-to-end verified (Phase C, unchanged).

## Caveat 2 — helix_qa autonomous QA session

The codegraph Challenge bank
(`tools/codegraph/challenges/codegraph-integration.bank.yaml`) was driven through
the helix_qa runner this session.

- `helix_qa/helixqa list -banks <bank>` → loads all 7 test cases correctly.
- `helix_qa/helixqa run -banks <bank> -platform desktop` → the runner loads the
  7 definitions ("Auto-wired runner with 7 bridged definitions") but completes in
  ~700 ms and reports `0/7 passed` with every case **SKIPPED** (see
  `helixqa-session/qa-results/qa-report.md`). The runner's `run` path is built
  for Android/UI-driven `dumpsys`-style platforms and does NOT shell out to a
  bank's `action:` command on the `desktop` platform — it skips them and emits a
  hollow "Step Validation PASSED" metadata row (sub-microsecond durations). That
  hollow PASS is itself a §11.4 PASS-bluff pattern and is NOT counted as
  evidence here.

**Honest disposition.** The real, anti-bluff autonomous-session evidence for the
codegraph bank is the **direct execution of every bank `action:` Challenge
script**, which each run real `codegraph` commands / real JSON-RPC / real agent
invocations and FAIL LOUDLY on empty/zero/simulated results. Phase C captured
that run: `tools/codegraph/challenges/run-all.sh` → 7/7 PASS with per-challenge
wire evidence under this directory (`cg-challenge-01..07-*.log`,
`cg-challenge-02-jsonrpc-wire.jsonl`, transcripts, connect-proofs). The bank YAML
is registered and consumable by the helix_qa runner; what is NOT yet available
is a helix_qa `run`-path that executes shell-`action` banks on the `desktop`
platform. That runner-side gap is a helix_qa enhancement, tracked separately —
it does not weaken the codegraph anti-bluff evidence, which stands on the
direct Challenge-script run with captured wire transcripts.

Captured this session: `helixqa-session/helixqa-run.log` +
`helixqa-session/qa-results/qa-report.md` (the honest record of the runner's
desktop-platform skip behaviour).

## Caveat 3 — anti-bluff covenant in QWEN.md across submodules

Assessment of `QWEN.md` presence + anti-bluff covenant across all owned
submodules (run 2026-05-20):

| Scope | QWEN.md present | CLAUDE.md present |
|---|---|---|
| `helix_code` | no | yes |
| `helix_qa` | no | yes |
| `challenges` | no | yes |
| `containers` | no | yes |
| `security` | no | yes |
| `dependencies/HelixDevelopment/*` (10 dirs) | 1 of 10 (`LLMsVerifier`) | 10 of 10 |
| `dependencies/vasic-digital/*` (52 dirs) | 0 of 52 | 52 of 52 |
| **Total dependency submodules** | **1 of 62** | **62 of 62** |

Finding: **QWEN.md is effectively absent fleet-wide.** Only one of 67 owned
submodules (`submodules/llms_verifier`) ships a `QWEN.md`;
every owned submodule already carries a full `CLAUDE.md` with the anti-bluff
covenant + CONST-035 / §11.4 anchors.

Recommendation (NOT executed unilaterally — would mass-create 66+ new files):
QWEN.md should be made a required consumer governance file fleet-wide ONLY if
the **constitution submodule** designates it so (CONST-059 governs which files
are required consumer files — currently the canonical trio is
`CONSTITUTION.md` / `CLAUDE.md` / `AGENTS.md`, with `QWEN.md` / `CRUSH.md` named
as sibling agent manuals at the meta-repo root by the project CLAUDE.md §1.1 but
not mandated per-submodule). The single existing submodule QWEN.md
(`LLMsVerifier`) was inspected — it already carries the anti-bluff covenant
mirrored from its CLAUDE.md, so no covenant fix was required there.

Disposition: no QWEN.md covenant fix needed (the one existing file is already
compliant); fleet-wide QWEN.md creation deferred pending a constitution-submodule
decision on whether QWEN.md is a required per-submodule consumer file.
