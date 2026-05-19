# CLAUDE.md — Assets

**Version**: 1.0.0
**Scope**: Assets submodule guidance for AI agents

## Asset Management

This submodule contains brand assets (logos, images). Changes should be minimal and follow brand guidelines.

## Mandatory Constraints

- We had been in position that all tests do execute.
- The bar for shipping is not green tests alone — every PASS must certify Quality, Completion, and Full usability per CONST-035.
- Reproduction-Before-Fix: Every bugfix must start with a challenge or test that reproduces the issue.
- Host Power Management is Forbidden: No scripts may shut down, reboot, or sleep the host machine.

---

## Article XI — Anti-Bluff and Quality Mandate

### §11.9 — Anti-Bluff Forensic Anchor

> Verbatim user mandate: *"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"*
>
> Operative rule: every PASS in this codebase MUST carry positive runtime evidence captured during execution. Metadata-only / configuration-only / absence-of-error / grep-based PASS without runtime evidence are critical defects regardless of how green the summary line looks.

Full text: root `CONSTITUTION.md` Article XI §11.9.

---

## Round-305 Minimal Enrichment (Template C — owned-static-content)

This directory holds brand artefacts (logos, images). The only verifiable
runtime invariants are file existence, non-zero size, and PNG magic-number
integrity. Per CONST-035 + CONST-050(B), even minimal enrichment ships an
anti-bluff Challenge with paired-mutation self-test rather than
metadata-only / file-existence-only assertions.

**Wire-evidence Challenge:** `challenges/scripts/asset_integrity_challenge.sh`
- Verifies each expected PNG is present, above 1 KiB sanity floor, and
  starts with the canonical PNG magic header `89504e470d0a1a0a` (compared
  via `xxd` to avoid bash variable-expansion mangling of `\x89`).
- Paired mutation: `MUTATION_TEST=1` writes a known-bad sentinel and the
  Challenge MUST reject it; absence of rejection is a meta-bluff regression.
- Captures runtime evidence: byte size + measured magic-number per asset.

**Governance hygiene (CONST-053):** `.gitignore` blocks build derivatives,
caches, OS detritus, secrets, and image-derivative artefacts.

