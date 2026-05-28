# QA Transcript — HXC-016

**Revision:** 1
**Last modified:** 2026-05-28T00:00:00Z

| Field | Value |
|---|---|
| Run-id | HXC-016 |
| Feature | §11.4.69–97 governance cascade into all owned submodules + mechanical propagation gate (CONST-047 / §3, §11.4.32) |
| Type | Task |
| Tracker | `docs/Fixed.md` (HXC-016 row — `Completed (→ Fixed.md)`) |
| Authority | constitution submodule `Constitution.md` §11.4.83 (docs/qa/ end-user evidence mandate) |
| Status | PASS — verified end-to-end |

---

> **Seed worked example.** This transcript is a real, already-completed
> feature's end-user evidence, recorded here as the canonical format for
> the `docs/qa/` tree (see `docs/qa/README.md`). It transcribes the
> verification commands and outputs captured during the HXC-016 session
> (the same evidence summarised in the `docs/Fixed.md` HXC-016 row). No
> output is fabricated — every result below is the real captured result
> from that session.

## Feature exercised

The end-user-visible "feature" of HXC-016 is the **governance-cascade
guarantee**: every owned-org submodule carries the new constitution
anchors (§11.4.69 + §11.4.75–97), and the `verify-governance-cascade.sh`
release-gate tool — the interface a maintainer or release engineer runs
to confirm that guarantee — reports zero failures. The "user" here is the
maintainer running the gate before a release.

## Bidirectional thread

### Exchange 1 — run the cascade verifier (the user-visible gate)

**Sent (command the maintainer runs):**

```
scripts/verify-governance-cascade.sh
```

**Received (captured result, summarised from the HXC-016 session):**

```
=== Result: 0 failures === PASS
```

204 owned-submodule PASS lines were emitted, each carrying the
`+ §11.4 covenant-114` marker confirming the new anchors are present in
that submodule's CONSTITUTION / CLAUDE / AGENTS / QWEN files.

### Exchange 2 — paired §1.1 mutation (proves the gate is not a bluff)

**Sent (mutation — strip the §11.4.95 H2 heading from one submodule's CLAUDE.md, then re-run the gate):**

```
# remove the "## §11.4.95 —" heading line from a cache/CLAUDE.md fixture
scripts/verify-governance-cascade.sh
```

**Received:**

```
=== Result: 1 failures ===
```

The gate correctly FAILed when the anchor heading was removed —
demonstrating it detects real absence, not just running green
unconditionally.

### Exchange 3 — restore + confirm (returns to the verified baseline)

**Sent:**

```
git checkout -- <the mutated CLAUDE.md>
scripts/verify-governance-cascade.sh
```

**Received:**

```
=== Result: 0 failures === PASS
```

## Anti-bluff hardening recorded during the run

A loose `grep -qF "§11.4.95 —"` originally matched the §11.4.95
cross-reference *inside* the §11.4.93 block body, silently skipping the
§11.4.95 *heading* in several submodules. The gate was tightened to match
the H2 marker `## §11.4.NN —` (commit `d2165bf7`), which then exposed 201
missing-heading files; fix-ups `79478ed5` / `903b9225` / `a9a1a6a1`
restored them. A second defect — a `reset --hard origin/main` that
regressed 4 HelixDevelopment submodules off their canonical `master`
lineage — was repaired by repointing to master with the complete anchor
set (`b4b790ea`).

## Captured-evidence references

| Evidence | Where |
|---|---|
| Final gate result `0 failures === PASS` (204 submodule PASS lines) | HXC-016 session log; summarised in `docs/Fixed.md` HXC-016 row |
| Paired §1.1 mutation FAIL→restore→PASS | Exchanges 2–3 above |
| Gate-tightening + fix-up commits | `d2165bf7`, `9031368d`, `79478ed5`, `903b9225`, `a9a1a6a1`, `b4b790ea` |
| Root cascade commit + submodule batches | `27929ae1`; `ef4b3986` / `a864039d` / `e4046668` / `3adb2e63` / `464b2401` / `b4ad4f50` / `053fd731` |

## Verdict

PASS — the cascade guarantee is verifiable end-to-end through the same
gate a release engineer runs, the gate is proven non-bluff via a paired
mutation, and the baseline is restored. This is the evidence shape every
future `docs/qa/<run-id>/transcript.md` should follow.
