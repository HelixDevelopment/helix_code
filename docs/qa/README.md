# HelixCode — `docs/qa/` End-User Evidence Tree

**Revision:** 1
**Last modified:** 2026-05-28T00:00:00Z

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-05-28 |
| Last modified | 2026-05-28 |
| Status | active (convention established — advisory enforcement) |
| Status summary | Establishes the §11.4.83 per-feature end-user evidence convention + seed worked example (HXC-016). Enforcement is ADVISORY (warn-mode) via `scripts/verify_qa_evidence.sh`; promotion to a hard commit/release gate is a future operator decision. |
| Authority | constitution submodule `Constitution.md` §11.4.83 (docs/qa/ end-user evidence mandate) |

---

## Table of contents

- [What this tree is](#what-this-tree-is)
- [Run-id format](#run-id-format)
- [What a transcript must contain](#what-a-transcript-must-contain)
- [Materials are committed in-repo](#materials-are-committed-in-repo)
- [Bot / agent-driven QA](#bot--agent-driven-qa)
- [Directory layout](#directory-layout)
- [Advisory gate](#advisory-gate)
- [Worked example](#worked-example)

## What this tree is

Per constitution submodule `Constitution.md` §11.4.83 (docs/qa/ end-user
evidence mandate), **every feature that ships MUST carry a recorded
end-to-end communication transcript plus any attached materials**
(screenshots, request/response payloads, audio, file uploads) committed
under `docs/qa/<run-id>/` — one directory per feature run.

The forensic anchor (verbatim 2026-05-22 operator mandate, quoted from
§11.4.83):

> "every feature that ships MUST carry a recorded e2e communication
> transcript + any attached materials under `docs/qa/<run-id>/`
> (per-feature subdirectories). A feature with no QA transcript is itself
> a §107 PASS-bluff — it claims to work but has no auditable runtime
> evidence. Bot-driven automation MUST preserve full bidirectional
> communication threads as proof."

A feature with no QA transcript is itself a §11.4 / §107 PASS-bluff: it
claims to work but carries no auditable runtime evidence that an end user
actually exercised it through the same interface they will use in
production.

## Run-id format

`<run-id>` MUST be monotonic + greppable. Use one of:

- **The workable-item ticket id** — HelixCode's per-project prefix
  convention (`docs/Issues.md` "Prefix convention" section). For
  meta-repo / project-wide / governance / infrastructure work the prefix
  is `HXC` (e.g. `HXC-016`, `HXC-019`); submodule-scoped work uses its
  own prefix (`HXA`, etc.). This is the **preferred** form because it
  binds the QA evidence to the tracker entry in `docs/Issues.md` /
  `docs/Fixed.md`.
- **An ISO-8601 UTC timestamp** — `YYYY-MM-DDTHHMMSSZ` (e.g.
  `2026-05-28T143000Z`) when a run is not tied to a single ticket.

Once chosen, a `<run-id>` directory is append-only — never renamed,
never reused for a different feature.

## What a transcript must contain

Per §11.4.83 operative rule (2):

1. **Full bidirectional thread.** Every prompt/command sent **and** every
   response received **and** every error message **and** every state
   change observed. A one-sided record ("we sent X") is NOT a transcript;
   both halves are required.
2. The interface exercised MUST be the **same interface the end user will
   use in production** (CLI invocation, HTTP request, GUI action, etc.) —
   not an internal shortcut that bypasses the user-visible path.
3. Where the feature produces sink-side / downstream output (audio,
   video, network, rendered UI), the transcript MUST cite positive
   sink-side captured evidence per §11.4.68 / §11.4.69 — never
   config-only, never metadata-only.

The canonical transcript file is `docs/qa/<run-id>/transcript.md`.

## Materials are committed in-repo

Per §11.4.83 operative rule (3): attached materials MUST be committed
**alongside the transcript, in-repo** — screenshots in `.png`, payloads
in `.json` / `.txt`, audio in `.wav` / `.mp3`, etc. External-only links
(Slack URL, Drive URL) are a §11.4.13 sink-side violation; the evidence
MUST live in the repository so the audit replay works offline from a
fresh clone.

## Bot / agent-driven QA

Per §11.4.83 operative rule (4): bot-driven / agent-driven QA automation
MUST preserve the **full conversation thread** as the proof artefact. A
bot that runs the round-trip but stores only the final PASS/FAIL line is
itself a §107 bluff at the QA-automation layer. The bot writes the same
`transcript.md` shape a human would — both halves of every exchange.

## Directory layout

```
docs/qa/
├── README.md                  # this file (the convention)
├── <run-id>/                  # one directory per feature run
│   ├── transcript.md          # full bidirectional thread (required)
│   └── <materials...>         # screenshots / payloads / audio (as produced)
└── ...
```

## Advisory gate

`scripts/verify_qa_evidence.sh` scans recent feature-shipping commits
(those touching non-test code under `helix_code/internal/**`,
`helix_code/cmd/**`, or `helix_code/applications/**`) and **WARNs** when
a feature commit has no matching `docs/qa/<run-id>/` directory.

The gate is **ADVISORY (warn-mode)**: it always exits 0 and never blocks
a commit, push, or build. The operator has NOT yet authorised a hard
gate; promoting this scanner to a blocking commit/release gate (per
§11.4.83 operative rule (5)) is a **future operator decision**. Until
then the gate raises visibility without halting work.

See `docs/scripts/verify_qa_evidence.md` for the companion guide.

## Worked example

`docs/qa/HXC-016/` is a seed worked example: it records the §11.4.69–97
governance-cascade verification (HXC-016, closed `Completed (→ Fixed.md)`
in `docs/Fixed.md`) in the transcript format above, demonstrating the
full bidirectional thread + captured command output for a real,
already-completed feature.
