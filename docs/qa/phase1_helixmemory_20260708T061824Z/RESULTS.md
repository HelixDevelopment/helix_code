# Phase-1 HelixMemory bring-up — end-to-end proof (§11.4.108 / §11.4.5 / §11.4.69 / §11.4.150)

**Date:** 2026-07-08 · Track `(T1/feature/helixllm-full-extension)` · Independent HelixMemory-only path
(operator-authorized "proceed HelixMemory-only," independent of the §11.4.174-blocked `cognee` re-enable)

## Verdict (honest, §11.4.6)

**CAPABILITY PROVEN — reference implementation of the mem0-style memory mechanism.** A real
store→recall→ground pipeline (embed → persist vector+text in Postgres/pgvector → embed query →
real pgvector cosine-distance recall → ground a prompt with ONLY the recalled memory → generate)
was proven end-to-end against the **live coder LLM** (`helixllm-coder`, `http://localhost:18434`,
read-only, never restarted/stopped — §11.4.122/§11.4.119) composed with a dedicated CPU embeddings
service (HF TEI `cpu-1.9`, `BAAI/bge-small-en-v1.5`, dim 384 — reused from
`docs/qa/phase3_embeddings_20260706/`) and a dedicated Postgres+pgvector store
(`pgvector/pgvector:pg16`), both booted on their own ports via the **containers submodule
orchestrator** (§11.4.76, rootless podman §11.4.161). The §11.4.108 runtime signature is
**GREEN-OK for both invented-fact recall queries**:

```
[MEMORY-RUNTIME-SIGNATURE] PASS top1=mem_region(score=0.8536, ok=true) tokenFound=true
    answer="Based on your recalled memory, your preferred deployment region for HelixLLM is
    called Emberfall-Station."
[MEMORY-RUNTIME-SIGNATURE] PASS top1=mem_alias(score=0.8781, ok=true) tokenFound=true
    answer="Your internal alias for the coder agent is Wraithloom."
```

## HONEST SCOPE NOTE (§11.4.6 / §11.4.150) — read before citing this as "mem0/Graphiti proven"

This proof does **NOT** install or invoke the upstream `mem0` Python package nor the
`graphiti-core` library / Graphiti MCP server. It is a minimal **reference implementation** of the
underlying mechanism those projects use (embed → persist → embed-query → similarity-search →
ground), built directly against Postgres+pgvector — the SAME class of backing store mem0's own
self-hosted OSS server uses for its vector layer. See
`docs/research/07.2026/04_embeddings_rag/HELIXMEMORY_PROVIDER.md` §6–§7 for the full recommended
production stack (Graphiti+FalkorDB durable spine + mem0 extraction layer, MCP-exposed) and the
explicit follow-on work items to wire the literal upstream packages.

## RED → GREEN runtime signature (§11.4.115 polarity, both queries)

Two "remember this" facts invented for this proof (the coder cannot know them from training) plus
two topic-adjacent distractor facts, so recall must genuinely discriminate:

| qkey | invented fact | fact-bearing memory id |
|------|----------------|--------------------------|
| q1 | "Remember that my preferred deployment region for HelixLLM is called **Emberfall-Station**." | `mem_region` |
| q2 | "Remember that my internal alias for the coder agent is **Wraithloom**." | `mem_alias` |

Distractors: `mem_lunch` ("I usually eat lunch at noon"), `mem_color` ("my favorite color is teal").

**RED (nothing stored/recalled, `10_red_baseline.txt`)** — the live coder is asked the bare recall
question with NOTHING stored/retrieved:
- q1: *"I don't have any information about your preferred deployment region for HelixLLM, as this
  would be specific to your configuration or settings that aren't provided in our conversation."*
  → does **NOT** contain `Emberfall-Station`. **RED-OK.**
- q2: *"I don't have access to information about any internal alias or specific identifier for a
  'coder agent' in our conversation."* → does **NOT** contain `Wraithloom`. **RED-OK.**

**GREEN (`11_green_proof_{q1,q2}.txt`)** — full pipeline (embed all 4 facts on real TEI vectors →
persist to real Postgres/pgvector → embed query → real pgvector cosine-distance top-1 recall →
ground ONLY on the recalled memory → generate on the SAME live coder):
- q1 answer: *"Based on your recalled memory, your preferred deployment region for HelixLLM is
  called Emberfall-Station."* → **contains** `Emberfall-Station`. **GREEN-OK.**
- q2 answer: *"Your internal alias for the coder agent is Wraithloom."* → **contains** `Wraithloom`.
  **GREEN-OK.**

RED-fail then GREEN-pass on the identical question, identical live model, is the unfakeable proof
that the generation is genuinely *grounded in recalled memory*, not recalled from training or
hardcoded.

## Recall top-1 evidence (real pgvector cosine distance, `retrieval_{q1,q2}.json`)

4-fact store (2 fact-bearing + 2 distractors), full ranking both queries:

```
qkey=q1  rank1=mem_region(0.8536) rank2=mem_alias(0.6216) rank3=mem_color(0.4249) rank4=mem_lunch(0.3698)
         [RETRIEVAL-CHECK] PASS: real-pgvector top1=mem_region == expected mem_region

qkey=q2  rank1=mem_alias(0.8781) rank2=mem_region(0.6140) rank3=mem_color(0.3892) rank4=mem_lunch(0.3220)
         [RETRIEVAL-CHECK] PASS: real-pgvector top1=mem_alias == expected mem_alias
```

Ranking is computed by Postgres itself (`ORDER BY embedding <=> '<query-vector>'::vector`) over
real TEI-served vectors persisted in the `memory_facts` table — never a hardcoded pick; both
fact-bearing memories rank strictly above every distractor with a wide margin (≥0.23 over the
runner-up).

## Self-validation — the analyzer is non-bluff (§11.4.107(10), `12_self_validation_{q1,q2}.txt`)

For each query, the SAME analyzer (`analyze()` in `main.go`) is run against one golden-good fixture
(the real captured recall + generation above) and three deliberately-degraded golden-bad fixtures:

| fixture | q1 verdict | q2 verdict |
|---|---|---|
| golden-good (real captured memory output) | **PASS** | **PASS** |
| golden-bad: no-fact answer ("I don't know...") | **FAIL** (correctly) | **FAIL** (correctly) |
| golden-bad: wrong memory recalled (top-1 swapped to a distractor) | **FAIL** (correctly) | **FAIL** (correctly) |
| golden-bad: empty answer | **FAIL** (correctly) | **FAIL** (correctly) |

`[SELF-VALIDATION] PASS: analyzer PASSes golden-good and FAILs all golden-bad fixtures` for both
qkeys — an analyzer that PASSed any golden-bad fixture would itself be a bluff gate (§11.4.107(10));
it does not.

## Container lifecycle (§11.4.119 single-resource-owner, §11.4.122 coder untouched)

- Booted `phase1helixmemory_pg-helixmemory_1` (`pgvector/pgvector:pg16`) on port **18450** and
  `phase1helixmemory_tei-helixmemory_1` (HF TEI `cpu-1.9`, `BAAI/bge-small-en-v1.5`) on port
  **18451**, both via the `digital.vasic.containers` compose orchestrator (rootless podman,
  `20_boot.txt`, `21_health.txt` — health OK after 2 polls, `24_container_state.txt`).
- `helixllm-coder` (the live coder at :18434) verified running before and after, **never**
  restarted/stopped (`00_preflight.txt`, `29b_post_teardown.txt`).
- The vision container (`helixllm_visiongen_visiongen_1`, `:18439`) and every sibling Phase-3 lane
  were observed but never touched (§11.4.174 — shared-host process ownership verified before any
  action; only ports 18450/18451, which this harness itself booted, were acted on).
- Torn down single-owner (`29_teardown.txt`: `DOWN-OK`); post-teardown confirms zero
  `phase1helixmemory_*` containers, coder still up, ports 18450/18451 freed (`29b_post_teardown.txt`).

## Honesty notes (§11.4.6 / §11.4.150)

1. **Zep Community Edition is deprecated** (April 2025) — the durable-spine recommendation is the
   raw **Graphiti** library/MCP-server (Apache-2.0), not a "Zep server," correcting the prior spike
   doc's implicit assumption. See `HELIXMEMORY_PROVIDER.md` §2.1 for the full citation.
2. **This proof is a reference implementation, not a literal mem0/Graphiti package boot** — see the
   scope note above and `HELIXMEMORY_PROVIDER.md` §6–§7.
3. **No GPU was used or needed** — Postgres, pgvector, and the CPU TEI build are all CPU workloads;
   the coder was already resident and read-only throughout.
4. **No API keys, no cloud calls** — every component (embeddings, LLM, vector store) ran on
   `localhost`, config-injected ports/credentials (a freshly-generated-per-run Postgres password,
   never hardcoded — §11.4.10), zero network egress beyond the one-time container-image pulls
   already cached from prior sessions plus this session's `pgvector/pgvector:pg16` pull.

## Reproduce

```
cd submodules/helix_llm/docs/qa/phase1_helixmemory_20260708T061824Z/harness && ./run_proof.sh
```

Builds the harness (containers-submodule `replace` directive), boots its OWN Postgres+pgvector +
TEI lanes (config-injected ports/credentials — `PGHM_HOST_PORT`/`TEI_HOST_PORT`/`PGHM_USER`/
`PGHM_PASSWORD`/`PGHM_DB`/`TEI_MODEL_ID`/`HEALTH_TIMEOUT` env-tunable), asks the live coder the RED
baseline for both recall queries, persists the 4-fact fixture set to Postgres/pgvector, recalls +
checks top-1 + grounds + generates + analyzes for both qkeys, self-validates the analyzer, tears
down single-owner, and leaves `helixllm-coder` untouched. `harness/phase1helixmemory.bin` is a
build artefact (gitignored §11.4.30; regenerate via `go build .` or by re-running the script).

## Composition

§11.4.76 (containers submodule) · §11.4.161 (rootless) · §11.4.108 (runtime signature) ·
§11.4.107(10) (self-validated analyzer) · §11.4.115 (RED-first polarity) · §11.4.119 (single-owner
teardown) · §11.4.122 (coder untouched) · §11.4.174 (shared-host process-ownership verified) ·
§11.4.150 (deep-research note, `01_research_note.txt` + `HELIXMEMORY_PROVIDER.md`) · §11.4.6
(Zep-CE-deprecation correction, honest reference-implementation scope note) · §11.4.10 (no
hardcoded/leaked credentials — freshly-generated ephemeral DB password).

## Sources verified (§11.4.99/§11.4.150)

See `docs/research/07.2026/04_embeddings_rag/HELIXMEMORY_PROVIDER.md` "Sources verified
2026-07-08" for the full citation set (Zep open-source-strategy blog posts, Graphiti repo + MCP
server README, FalkorDB license + blog posts, mem0 blog + LICENSE + docs + GitHub issue).
