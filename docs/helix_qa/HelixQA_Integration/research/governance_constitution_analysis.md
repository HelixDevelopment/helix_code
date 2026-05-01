# Governance & Constitution Analysis: Cross-Repository Anti-Bluff Mandate Coverage

**Date:** 2026-04-29
**Analyst:** Governance & Compliance Analysis Agent
**Scope:** HelixCode, HelixQA, Catalogizer + all submodules

---

## 1. EXECUTIVE SUMMARY

This analysis examines CONSTITUTION.md, CLAUDE.md, AGENTS.md, and related governance files across three primary repositories to assess coverage of two critical anti-bluff mandates:

1. **CONST-035 / Zero-Bluff Mandate** — Anti-bluff tests and challenges requirement
2. **Article XI §11.9 — User-Mandate Forensic Anchor** — Verbatim user mandate with cascade requirement (dated 2026-04-29)

**Critical Finding:** The User-Mandate Forensic Anchor (§11.9) is MISSING from HelixCode entirely — present in CONSTITUTION.md, CLAUDE.md, and AGENTS.md for both HelixQA and Catalogizer, but absent from all three HelixCode governance files. Additionally, cascade compliance to submodules is unverified across all repos.

---

## 2. REPOSITORY GOVERNANCE FILE INVENTORY

### 2.1 HelixQA (github.com/HelixDevelopment/HelixQA)

| File | Lines | CONST-035 | Article XI §11.9 | Cascade Mention |
|------|-------|-----------|-------------------|-----------------|
| CONSTITUTION.md | ~462 | ✅ Yes (Article XI §§11.1–11.9) | ✅ Yes — verbatim quote + operative rule | ✅ Yes — §11.9 "must appear in every submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md" |
| CLAUDE.md | ~1019 | ✅ Yes (Rule 10: Zero-Bluff Mandate) | ✅ Yes — verbatim quote + operative rule | ✅ Yes — "cascade to every submodule" |
| AGENTS.md | ~426 | ✅ Yes | ✅ Yes — verbatim quote + operative rule | ✅ Yes — "must appear in every submodule's governance files" |
| CHANGELOG.md | ~38 | ✅ Referenced | ✅ Referenced | N/A |

**Key HelixQA CONSTITUTION.md provisions:**
- **CONST-035** — Anti-Bluff Tests & Challenges (mandatory): "Tests, challenges, and HelixQA banks must verify real end-user behaviour... A PASS that does not exercise the feature is a critical defect."
- **CONST-033** — Host Power Management is Forbidden
- **CONST-032** — Reproduction-Before-Fix
- **Article XI §11.9** — User-Mandate Forensic Anchor (2026-04-29): Contains the verbatim user quote: *"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case..."*
- **Cascade requirement**: "This anchor section (verbatim quote + operative rule) must appear in every submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md. Non-compliance is a release blocker regardless of context."
- 100% Test Coverage, Challenge Coverage, Real Data requirement, Definition of Done with pasted output

**Key HelixQA CLAUDE.md provisions (Rule 10):**
- "Every test, challenge, or HelixQA bank entry must fail if the feature it claims to verify is removed or broken."
- "A green test suite combined with a broken feature is a worse outcome than an honest red one."
- Full §11.9 verbatim quote and operative rule present
- Cascade: "this clause must appear in every submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md"

**Key HelixQA AGENTS.md provisions:**
- "Anti-Bluff Testing — Mandatory (CONST-035)"
- Full §11.9 verbatim quote and operative rule present
- Cascade: "This anchor MUST appear in every submodule's governance files"

---

### 2.2 HelixCode (github.com/HelixDevelopment/HelixCode)

| File | Lines | CONST-035 | Article XI §11.9 | Cascade Mention |
|------|-------|-----------|-------------------|-----------------|
| CONSTITUTION.md | ~461 | ✅ Yes (labeled CONST-017) | ❌ **MISSING** | ✅ Yes (CONST-036: "propagated to all submodules") |
| CLAUDE.md | ~879 | ✅ Yes (Rule 10: Zero-Bluff Mandate) | ❌ **MISSING** | ❌ Not mentioned |
| AGENTS.md | ~426 | ✅ Yes (Rule 10: Zero-Bluff Mandate) | ❌ **MISSING** | ❌ Not mentioned |

**Key HelixCode CONSTITUTION.md provisions:**
- **CONST-017** — Anti-Bluff Tests & Challenges: "Tests must fail if the feature they claim to verify is removed or broken."
- **CONST-018** — Host Power Management is Forbidden
- **CONST-036** — Propagation: "CONST-036: The following mandatory constraints are propagated to all submodules..."
- **NO Article XI §11.9** — The verbatim user mandate forensic anchor is completely absent
- **NO verbatim user quote** — The historical user mandate text is not present anywhere

**Key HelixCode CLAUDE.md provisions:**
- "Rule 10: Zero-Bluff Mandate (CONST-035)" — operative rules present (assert concrete outcomes, run real system, matching negative, emit evidence, verify "fails when removed")
- **NO §11.9 User-Mandate Forensic Anchor** — no verbatim quote, no "2026-04-29" date
- **NO cascade requirement** for §11.9 (since §11.9 itself is missing)

**Key HelixCode AGENTS.md provisions:**
- "Rule 10: Zero-Bluff Mandate (CONST-035)" — same operative rules as CLAUDE.md
- **NO §11.9 User-Mandate Forensic Anchor**

**HelixCode Submodules:**
- `.gitmodules` lists 15 git submodules including internal/isolated_files/
- Submodules include: awesome-cpp-examples, awesome-shell-examples, cpp-learning-lab, rust-examples, rust-learning-lab, go-examples, go-learning-lab, python-examples, python-learning-lab, data-learning-lab, ml-starter-lab, mlops-learning-lab, data-engineering-lab, distributed-systems-learning-lab, internal/isolated_files/
- **Verification needed:** Whether each submodule has CONSTITUTION.md / CLAUDE.md / AGENTS.md with anti-bluff mandates

---

### 2.3 Catalogizer (github.com/vasic-digital/Catalogizer)

| File | Lines | CONST-035 | Article XI §11.9 | Cascade Mention |
|------|-------|-----------|-------------------|-----------------|
| CONSTITUTION.md | ~667 | ✅ Yes (Article XI §§11.1–11.9) | ✅ Yes — verbatim quote + operative rule | ✅ Yes — "must appear in every submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md. Non-compliance is a release blocker." |
| CLAUDE.md | ~664 | ✅ Yes (Article XI) | ✅ Yes — verbatim quote + operative rule | ✅ Yes — "this anchor MUST appear in every submodule's governance files" |
| AGENTS.md | ~286 | ✅ Yes (Article XI) | ✅ Yes — verbatim quote + operative rule | ✅ Yes — "this anchor MUST appear in every submodule's governance files" |
| MEMORY.md | N/A | N/A | N/A | N/A |

**Key Catalogizer CONSTITUTION.md provisions:**
- **Article XI §11.9** — User-Mandate Forensic Anchor (2026-04-29): Full verbatim user quote present
- **Cascade requirement (extending §11.8)**: "This anchor section (verbatim quote + operative rule) must appear in every submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md. Non-compliance is a release blocker regardless of context."
- Also contains: Article V (100% test coverage), Article VI (Open-Points brief), Article VII (Full-QA Master Cycle)
- Universal Mandatory Constraints inherited from HelixAgent root CLAUDE.md
- CONST-032 — Reproduction-Before-Fix
- CONST-033 — Host Power Management Hard Ban

**Key Catalogizer CLAUDE.md provisions:**
- "⚠️ Anti-Bluff Testing — Mandatory (Article XI)" — full contract with 5 requirements
- "⚠️ User-Mandate Forensic Anchor (Article XI §11.9 — 2026-04-29)" — verbatim quote + operative rule
- "Cascade requirement: this anchor (verbatim quote + operative rule) MUST appear in every submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md."

**Key Catalogizer AGENTS.md provisions:**
- "Anti-Bluff Testing — Mandatory (Article XI)" — full operative rules
- "⚠️ User-Mandate Forensic Anchor (Article XI §11.9 — 2026-04-29)" — verbatim quote + operative rule
- "Cascade requirement: this anchor MUST appear in every submodule's governance files."

**Catalogizer Submodules:**
- `.gitmodules` indicates 41+ independent git submodules under `digital.vasic.*` and `@vasic-digital/*`
- Go modules: 23 modules wired via `replace` in catalog-api/go.mod
- TS/React modules: 9 modules linked via `file:../` in catalog-web/package.json
- HelixQA/AI: 9 modules for QA/testing
- **Verification needed:** Whether each of the 41+ submodules has CONSTITUTION.md / CLAUDE.md / AGENTS.md with anti-bluff mandates cascaded

---

## 3. CROSS-REPOSITORY COMPARISON MATRIX

### 3.1 Anti-Bluff Mandate Presence

| Mandate | HelixQA | HelixCode | Catalogizer |
|---------|---------|-----------|-------------|
| CONST-035 / Zero-Bluff operative rules | ✅ | ✅ (as CONST-017) | ✅ |
| Article XI §11.9 verbatim user quote | ✅ | ❌ **MISSING** | ✅ |
| Article XI §11.9 operative rule ("bar for shipping is not tests pass but users can use") | ✅ | ❌ **MISSING** | ✅ |
| Article XI §11.9 date stamp (2026-04-29) | ✅ | ❌ **MISSING** | ✅ |
| Cascade requirement to submodules | ✅ | ✅ (CONST-036) | ✅ |
| §11.9 cascade in submodules | Unknown | Unknown | Unknown |

### 3.2 Shared Rules Across All Three Repos

The following rules are present in all three repositories:

1. ✅ **100% Test Coverage** requirement
2. ✅ **Challenge Coverage** requirement
3. ✅ **Real Data** requirement (no mocks outside unit tests)
4. ✅ **CONST-032 — Reproduction-Before-Fix**
5. ✅ **CONST-033 — Host Power Management is Forbidden**
6. ✅ **Definition of Done** with pasted terminal output requirement
7. ✅ **No self-certification** rule
8. ✅ **No CI/CD pipelines** rule
9. ✅ **SSH Git only** rule
10. ✅ **Resource Limits** for tests (30-40% host resources)

### 3.3 Rules Present in Only Some Repos

| Rule | Present In | Missing From |
|------|-----------|-------------|
| Article XI §11.9 User-Mandate Forensic Anchor | HelixQA, Catalogizer | **HelixCode** |
| Article VII — Full-QA Master Cycle | Catalogizer, HelixQA | HelixCode |
| Article V / VI — Open-Points Closure Brief | Catalogizer | HelixQA, HelixCode |
| Zero Warning / Zero Error | Catalogizer, HelixQA | HelixCode |
| HTTP/3 + Brotli requirement | Catalogizer | HelixQA, HelixCode |
| No sudo / root operations | Catalogizer, HelixQA | HelixCode |
| Concurrent-Safe Containers (Go-specific) | Catalogizer, HelixQA | HelixCode |

### 3.4 Contradictions

No direct contradictions were found. However, there are **naming inconsistencies**:
- HelixQA labels the anti-bluff mandate as **CONST-035**
- HelixCode labels the equivalent mandate as **CONST-017**
- Both reference the same underlying rule but with different constant identifiers
- HelixCode's CLAUDE.md and AGENTS.md reference "CONST-035" in the Rule 10 title, but the CONSTITUTION.md labels it CONST-017

---

## 4. ANTI-BLUFF MANDATE COVERAGE ANALYSIS

### 4.1 CONST-035 / Zero-Bluff Coverage

| Location | Present | Notes |
|----------|---------|-------|
| HelixQA CONSTITUTION.md | ✅ | As CONST-035 (Article XI §§11.1–11.9) |
| HelixQA CLAUDE.md | ✅ | Rule 10: Zero-Bluff Mandate |
| HelixQA AGENTS.md | ✅ | Anti-Bluff Testing — Mandatory |
| HelixCode CONSTITUTION.md | ✅ | As CONST-017 |
| HelixCode CLAUDE.md | ✅ | Rule 10: Zero-Bluff Mandate (CONST-035) |
| HelixCode AGENTS.md | ✅ | Rule 10: Zero-Bluff Mandate (CONST-035) |
| Catalogizer CONSTITUTION.md | ✅ | Article XI §§11.1–11.9 |
| Catalogizer CLAUDE.md | ✅ | Article XI |
| Catalogizer AGENTS.md | ✅ | Article XI |

**CONST-035 Coverage: 9/9 files ✅**

### 4.2 Article XI §11.9 User-Mandate Forensic Anchor Coverage

| Location | Present | Notes |
|----------|---------|-------|
| HelixQA CONSTITUTION.md | ✅ | Full verbatim quote + operative rule + cascade |
| HelixQA CLAUDE.md | ✅ | Full verbatim quote + operative rule + cascade |
| HelixQA AGENTS.md | ✅ | Full verbatim quote + operative rule + cascade |
| HelixCode CONSTITUTION.md | ❌ **MISSING** | No verbatim quote, no §11.9, no date stamp |
| HelixCode CLAUDE.md | ❌ **MISSING** | No verbatim quote, no §11.9, no date stamp |
| HelixCode AGENTS.md | ❌ **MISSING** | No verbatim quote, no §11.9, no date stamp |
| Catalogizer CONSTITUTION.md | ✅ | Full verbatim quote + operative rule + cascade |
| Catalogizer CLAUDE.md | ✅ | Full verbatim quote + operative rule + cascade |
| Catalogizer AGENTS.md | ✅ | Full verbatim quote + operative rule + cascade |

**Article XI §11.9 Coverage: 6/9 files ❌ — 3 MISSING in HelixCode**

### 4.3 Submodule Coverage (Unverified)

Both HelixCode and Catalogizer declare cascade requirements for submodules, but **no submodule was verified** to actually contain the cascaded mandates.

**HelixCode submodules requiring verification:**
- awesome-cpp-examples
- awesome-shell-examples
- cpp-learning-lab
- rust-examples
- rust-learning-lab
- go-examples
- go-learning-lab
- python-examples
- python-learning-lab
- data-learning-lab
- ml-starter-lab
- mlops-learning-lab
- data-engineering-lab
- distributed-systems-learning-lab
- internal/isolated_files/

**Catalogizer submodules requiring verification (41+ total):**
- digital.vasic.* Go modules (23 modules): Auth, Cache, Config, Concurrency, Container, Database, Discovery, EventBus, Filesystem, Lazy, LLMProvider, LLMsVerifier, Media, Memory, Middleware, Observability, RateLimiter, Recovery, ReplayBuffer, ScreenDiff, Security, Storage, Streaming, Upstreams, VisionEngine, Watcher
- @vasic-digital/* TS/React modules (9 modules): Auth-Context-React, Catalogizer-API-Client-TS, Collection-Manager-React, Dashboard-Analytics-React, Media-Browser-React, Media-Player-React, Media-Types-TS, UI-Components-React, WebSocket-Client-TS
- HelixQA/AI modules (9 modules): Build, DocProcessor, Entities, HelixQA, LLMOrchestrator, OCU-CUDA-Sidecar, TrainingCollector, VisualRegression, Website
- Plus internal modules: Assets, catalog-api, catalog-web, catalogizer-android, catalogizer-androidtv, catalogizer-api-client, catalogizer-desktop, installer-wizard, challenges, tests

---

## 5. GAP IDENTIFICATION

### 5.1 Critical Gaps (Release Blockers per Constitution)

| Gap ID | Gap Description | Severity | Affected Repo |
|--------|----------------|----------|---------------|
| GAP-001 | **Article XI §11.9 User-Mandate Forensic Anchor MISSING from HelixCode CONSTITUTION.md** | 🔴 Critical | HelixCode |
| GAP-002 | **Article XI §11.9 User-Mandate Forensic Anchor MISSING from HelixCode CLAUDE.md** | 🔴 Critical | HelixCode |
| GAP-003 | **Article XI §11.9 User-Mandate Forensic Anchor MISSING from HelixCode AGENTS.md** | 🔴 Critical | HelixCode |
| GAP-004 | **Cascade to HelixCode submodules UNVERIFIED** — no evidence that submodules contain anti-bluff mandates | 🔴 Critical | HelixCode |
| GAP-005 | **Cascade to Catalogizer submodules UNVERIFIED** — 41+ submodules, no verification performed | 🔴 Critical | Catalogizer |
| GAP-006 | **HelixQA submodules (external deps) may lack anti-bluff mandates** — external submodules like ollama, go-sqlite3, goja, etc. | 🟡 Medium | HelixQA |

### 5.2 Secondary Gaps

| Gap ID | Gap Description | Severity | Affected Repo |
|--------|----------------|----------|---------------|
| GAP-007 | **Naming inconsistency**: HelixCode uses CONST-017 while HelixQA/Catalogizer reference CONST-035 for the same rule | 🟡 Medium | HelixCode |
| GAP-008 | **HelixCode CLAUDE.md/AGENTS.md title says "CONST-035" but CONSTITUTION.md labels it "CONST-017"** — inconsistent internal reference | 🟡 Medium | HelixCode |
| GAP-009 | **Catalogizer AGENTS.md does not mention CONST-035 by number** — only references "Article XI" | 🟡 Medium | Catalogizer |
| GAP-010 | **HelixCode missing Article VII (Full-QA Master Cycle)** — present in Catalogizer and HelixQA | 🟡 Medium | HelixCode |
| GAP-011 | **HelixCode missing Article V/VI (Open-Points Closure Brief)** — present in Catalogizer | 🟡 Medium | HelixCode |

---

## 6. EXACT TEXT PROPOSALS

### 6.1 For HelixCode CONSTITUTION.md

**File:** `https://github.com/HelixDevelopment/HelixCode/blob/main/CONSTITUTION.md`
**Insertion point:** After CONST-017 (Anti-Bluff Tests & Challenges), before CONST-018

```markdown
### CONST-035 — Anti-Bluff Tests & Challenges (User-Mandate Forensic Anchor)

**§11.9 User-Mandate Forensic Anchor (2026-04-29)**

This Article exists because of an explicit, repeatedly-stated user mandate. The verbatim text:

> "We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"

This anchor is the primary authority for the entire Article. The operative rule is:

**The bar for shipping is not "tests pass" but "users can use the feature."**

Every PASS in this codebase MUST carry positive evidence captured during execution that the feature works for the end user. Metadata-only PASS, configuration-only PASS, "absence-of-error" PASS, and grep-based PASS without runtime evidence are all critical defects regardless of how green the summary line looks.

Tests and Challenges (HelixQA) are bound equally — a Challenge that scores PASS on a non-functional feature is the same class of defect as a unit test that does. Both must produce positive end-user evidence; both are subject to the anti-bluff contract.

No false-success results are tolerable. A green test suite combined with a broken feature is a worse outcome than an honest red one — it silently destroys trust in the entire suite. Anti-bluff discipline is the line between a real engineering project and a theatre of one.

**Cascade requirement (extending CONST-036):**
This anchor section (verbatim quote + operative rule) must appear in every submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md. Non-compliance is a release blocker regardless of context. Adding files to scanner allowlists to silence bluff findings without resolving the underlying defect is itself a violation.
```

---

### 6.2 For HelixCode CLAUDE.md

**File:** `https://github.com/HelixDevelopment/HelixCode/blob/main/CLAUDE.md`
**Insertion point:** After "Rule 10: Zero-Bluff Mandate (CONST-035)" section, before Rule 11

```markdown
⚠️ User-Mandate Forensic Anchor (Article XI §11.9 — 2026-04-29)

This Article exists because of an explicit user mandate, verbatim:

"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"

The operative rule: the bar for shipping is not "tests pass" but "users can use the feature."

Every PASS in this codebase MUST carry positive evidence captured during execution that the feature works for the end user. Metadata-only PASS, configuration-only PASS, "absence-of-error" PASS, and grep-based PASS without runtime evidence are all critical defects regardless of how green the summary line looks.

Tests and Challenges (HelixQA) are bound equally. A Challenge that scores PASS on a non-functional feature is the same class of defect as a unit test that does.

No false-success results are tolerable. A green test suite combined with a broken feature is a worse outcome than an honest red one — it silently destroys trust in the entire suite.

**Cascade requirement:** this anchor (verbatim quote + operative rule) MUST appear in every submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md. Non-compliance is a release blocker. Adding files to scanner allowlists to silence bluff findings without resolving the underlying defect is itself a violation.

Full text: CONSTITUTION.md Article XI §11.9.
```

---

### 6.3 For HelixCode AGENTS.md

**File:** `https://github.com/HelixDevelopment/HelixCode/blob/main/AGENTS.md`
**Insertion point:** After "Rule 10: Zero-Bluff Mandate (CONST-035)" section

```markdown
⚠️ User-Mandate Forensic Anchor (Article XI §11.9 — 2026-04-29)

This Article exists because of an explicit user mandate, verbatim:

"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"

The operative rule: the bar for shipping is not "tests pass" but "users can use the feature."

Every PASS in this codebase MUST carry positive evidence captured during execution that the feature works for the end user. No metadata-only PASS, no configuration-only PASS, no "absence-of-error" PASS, no grep-based PASS — all are critical defects regardless of how green the summary line looks.

Tests and Challenges (HelixQA) are bound equally.

No false-success results are tolerable. A green test suite combined with a broken feature is a worse outcome than an honest red one.

**Cascade requirement:** this anchor MUST appear in every submodule's governance files. Adding files to scanner allowlists to silence bluff findings without resolving the underlying defect is itself a violation.

Full text: CONSTITUTION.md Article XI §11.9.
```

---

### 6.4 For HelixCode CONSTITUTION.md — Naming Fix

**File:** `https://github.com/HelixDevelopment/HelixCode/blob/main/CONSTITUTION.md`
**Change:** Rename CONST-017 to CONST-035 for consistency across the ecosystem

Replace:
```markdown
### CONST-017 — Anti-Bluff Tests & Challenges
```

With:
```markdown
### CONST-035 — Anti-Bluff Tests & Challenges (formerly CONST-017)
```

---

### 6.5 For All Catalogizer Submodules (Template)

**Applies to:** All 41+ Catalogizer submodules under `digital.vasic.*` and `@vasic-digital/*`
**Required files in EACH submodule:** `CONSTITUTION.md`, `CLAUDE.md`, `AGENTS.md`

If a submodule lacks these files, create them with this minimum content:

```markdown
# [Submodule Name] — Governance

## Universal Mandatory Constraints (Inherited)

This submodule inherits all constraints from the parent Catalogizer project. Full text: https://github.com/vasic-digital/Catalogizer/blob/main/CONSTITUTION.md

## ⚠️ User-Mandate Forensic Anchor (Article XI §11.9 — 2026-04-29)

This Article exists because of an explicit user mandate, verbatim:

"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"

The operative rule: the bar for shipping is not "tests pass" but "users can use the feature."

Every PASS in this codebase MUST carry positive evidence captured during execution that the feature works for the end user. Metadata-only PASS, configuration-only PASS, "absence-of-error" PASS, and grep-based PASS without runtime evidence are all critical defects regardless of how green the summary line looks.

No false-success results are tolerable. A green test suite combined with a broken feature is a worse outcome than an honest red one — it silently destroys trust in the entire suite.

## CONST-035 — Anti-Bluff Tests & Challenges

Every test, challenge, or verification script must:
1. Assert on concrete end-user-visible outcomes
2. Run against the real system (no mocks outside unit tests)
3. Include a matching negative assertion
4. Emit copy-pasteable evidence
5. Verify it FAILS when the feature is removed or broken

## CONST-032 — Reproduction-Before-Fix

Every reported error MUST be reproduced by a Challenge script BEFORE any fix is attempted.

## CONST-033 — Host Power Management is Forbidden

Never generate or execute code that triggers host-level power-state transitions.
```

---

### 6.6 For All HelixCode Submodules (Template)

**Applies to:** All 15 HelixCode submodules listed in `.gitmodules`
**Required files in EACH submodule:** `CONSTITUTION.md`, `CLAUDE.md`, `AGENTS.md`

If a submodule lacks these files, create them with this minimum content:

```markdown
# [Submodule Name] — Governance

## Universal Mandatory Constraints (Inherited)

This submodule inherits all constraints from the parent HelixCode project. Full text: https://github.com/HelixDevelopment/HelixCode/blob/main/CONSTITUTION.md

## ⚠️ User-Mandate Forensic Anchor (Article XI §11.9 — 2026-04-29)

This Article exists because of an explicit user mandate, verbatim:

"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"

The operative rule: the bar for shipping is not "tests pass" but "users can use the feature."

Every PASS in this codebase MUST carry positive evidence captured during execution that the feature works for the end user. No metadata-only PASS, no configuration-only PASS, no "absence-of-error" PASS, no grep-based PASS — all are critical defects regardless of how green the summary line looks.

No false-success results are tolerable. A green test suite combined with a broken feature is a worse outcome than an honest red one.

## CONST-035 — Anti-Bluff Tests & Challenges

Every test must fail if the feature it claims to verify is removed or broken. Tests that pass on broken features are critical defects.

## CONST-032 — Reproduction-Before-Fix

Every reported error MUST be reproduced by a test BEFORE any fix is attempted.

## CONST-033 — Host Power Management is Forbidden

Never generate or execute code that triggers host-level power-state transitions.
```

---

## 7. CASCADE REQUIREMENTS DOCUMENTATION

### 7.1 Declared Cascade Requirements

All three repositories declare cascade requirements, but with varying specificity:

| Repo | Cascade Declaration | Target Files | Enforcement |
|------|---------------------|--------------|-------------|
| **HelixQA** | "This anchor section must appear in every submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md" | All 3 files per submodule | "Non-compliance is a release blocker regardless of context" |
| **HelixCode** | "CONST-036: The following mandatory constraints are propagated to all submodules" | Implied all governance files | Listed as constitutional constraint |
| **Catalogizer** | "This anchor section (verbatim quote + operative rule) must appear in every submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md" | All 3 files per submodule | "Non-compliance is a release blocker regardless of context" |

### 7.2 Actual Cascade Status

| Repo | Submodules Count | Governance Files Verified | Status |
|------|-----------------|--------------------------|--------|
| **HelixQA** | 12 (external open-source) | 0 | ❌ **UNVERIFIED** — External submodules (ollama, go-sqlite3, goja, testify, etc.) unlikely to contain project-specific mandates |
| **HelixCode** | 15 | 0 | ❌ **UNVERIFIED** — No submodule was inspected for governance file presence |
| **Catalogizer** | 41+ | 0 | ❌ **UNVERIFIED** — No submodule was inspected for governance file presence |

### 7.3 Cascade Compliance Verification Protocol

To verify cascade compliance, the following actions must be taken:

1. **For each submodule**, check for presence of:
   - `CONSTITUTION.md`
   - `CLAUDE.md`
   - `AGENTS.md`

2. **For each governance file found**, verify it contains:
   - Article XI §11.9 verbatim user mandate quote
   - Article XI §11.9 operative rule ("bar for shipping is not tests pass but users can use the feature")
   - CONST-035 / Zero-Bluff operative rules
   - CONST-032 — Reproduction-Before-Fix
   - CONST-033 — Host Power Management

3. **For submodules missing governance files**, create them using the templates in §6.5 and §6.6.

4. **For submodules with incomplete governance files**, append the missing sections.

---

## 8. SUBMODULES REQUIRING VERIFICATION

### 8.1 HelixQA External Submodules (12)

| Submodule | Path | Likely Has Governance? |
|-----------|------|----------------------|
| ollama | src/helixqa/ollama | ❌ Unlikely (external project) |
| go-sqlite3 | src/helixqa/go-sqlite3 | ❌ Unlikely (external project) |
| goja | src/helixqa/goja | ❌ Unlikely (external project) |
| testify | src/helixqa/testify | ❌ Unlikely (external project) |
| mux | src/helixqa/mux | ❌ Unlikely (external project) |
| websocket | src/helixqa/websocket | ❌ Unlikely (external project) |
| go-plugin | src/helixqa/go-plugin | ❌ Unlikely (external project) |
| testifylint | src/helixqa/testifylint | ❌ Unlikely (external project) |
| chromedp | src/helixqa/chromedp | ❌ Unlikely (external project) |
| bubbletea | src/helixqa/bubbletea | ❌ Unlikely (external project) |
| cobra | src/helixqa/cobra | ❌ Unlikely (external project) |
| bubbleteatest | src/helixqa/bubbleteatest | ❌ Unlikely (external project) |

**Note:** External open-source submodules cannot be modified directly. The parent repo must either:
- Fork and maintain governance patches
- Document that external deps are exempt from project-specific mandates
- Apply mandates only to internal/controlled submodules

### 8.2 HelixCode Submodules (15)

| Submodule | URL Pattern | Governance Verification Status |
|-----------|-------------|--------------------------------|
| awesome-cpp-examples | github.com/HelixDevelopment/awesome-cpp-examples | ❌ NOT CHECKED |
| awesome-shell-examples | github.com/HelixDevelopment/awesome-shell-examples | ❌ NOT CHECKED |
| cpp-learning-lab | github.com/HelixDevelopment/cpp-learning-lab | ❌ NOT CHECKED |
| rust-examples | github.com/HelixDevelopment/rust-examples | ❌ NOT CHECKED |
| rust-learning-lab | github.com/HelixDevelopment/rust-learning-lab | ❌ NOT CHECKED |
| go-examples | github.com/HelixDevelopment/go-examples | ❌ NOT CHECKED |
| go-learning-lab | github.com/HelixDevelopment/go-learning-lab | ❌ NOT CHECKED |
| python-examples | github.com/HelixDevelopment/python-examples | ❌ NOT CHECKED |
| python-learning-lab | github.com/HelixDevelopment/python-learning-lab | ❌ NOT CHECKED |
| data-learning-lab | github.com/HelixDevelopment/data-learning-lab | ❌ NOT CHECKED |
| ml-starter-lab | github.com/HelixDevelopment/ml-starter-lab | ❌ NOT CHECKED |
| mlops-learning-lab | github.com/HelixDevelopment/mlops-learning-lab | ❌ NOT CHECKED |
| data-engineering-lab | github.com/HelixDevelopment/data-engineering-lab | ❌ NOT CHECKED |
| distributed-systems-learning-lab | github.com/HelixDevelopment/distributed-systems-learning-lab | ❌ NOT CHECKED |
| internal/isolated_files | Internal path | ❌ NOT CHECKED |

### 8.3 Catalogizer Submodules (41+)

The following submodule categories require governance verification:

**Category A: Go Modules (digital.vasic.*)** — 23 modules
- Auth, Cache, Config, Concurrency, Container, Database, Discovery, EventBus, Filesystem, Lazy, LLMProvider, LLMsVerifier, Media, Memory, Middleware, Observability, RateLimiter, Recovery, ReplayBuffer, ScreenDiff, Security, Storage, Streaming, Upstreams, VisionEngine, Watcher

**Category B: TS/React Modules (@vasic-digital/*)** — 9 modules
- Auth-Context-React, Catalogizer-API-Client-TS, Collection-Manager-React, Dashboard-Analytics-React, Media-Browser-React, Media-Player-React, Media-Types-TS, UI-Components-React, WebSocket-Client-TS

**Category C: HelixQA/AI Modules** — 9 modules
- Build, DocProcessor, Entities, HelixQA, LLMOrchestrator, OCU-CUDA-Sidecar, TrainingCollector, VisualRegression, Website

**Category D: Application Modules** — 4 modules
- catalog-api, catalog-web, catalogizer-android, catalogizer-androidtv, catalogizer-api-client, catalogizer-desktop, installer-wizard

---

## 9. RECOMMENDATIONS

### Priority 1: Critical (Immediate Action Required)

1. **Add Article XI §11.9 to all three HelixCode governance files** using the exact text proposals in §6.1–§6.3. This is a constitutional release blocker per the cascade requirements in HelixQA and Catalogizer.

2. **Verify all HelixCode submodules** for governance file presence and anti-bluff mandate coverage. Create missing files using template §6.6.

3. **Verify all Catalogizer submodules** for governance file presence and anti-bluff mandate coverage. Create missing files using template §6.5.

### Priority 2: High (Next Sprint)

4. **Align naming**: Rename HelixCode CONST-017 to CONST-035 for consistency across the ecosystem.

5. **Document external submodule policy**: For HelixQA's 12 external open-source submodules, create a documented policy stating either (a) they are exempt from project-specific mandates, or (b) they must be forked and patched.

6. **Add Article VII (Full-QA Master Cycle) to HelixCode** if the project intends to maintain parity with Catalogizer and HelixQA.

### Priority 3: Medium (Ongoing Maintenance)

7. **Implement automated cascade verification**: Create a CI script (run manually per no-CI rule) that scans all submodules for governance file presence and required sections.

8. **Quarterly governance audits**: Schedule recurring analysis of all repos and submodules for governance compliance.

---

## 10. APPENDIX: VERBATIM MANDATE TEXT (Authoritative Source)

The following is the canonical verbatim text that MUST appear in all governance files per Article XI §11.9:

> "We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"

**Operative rule:** The bar for shipping is not "tests pass" but "users can use the feature."

---

## 11. NOTES ON UNVERIFIED ITEMS

The following items could not be verified during this analysis and require follow-up:

- [ ] HelixCode submodule governance files (15 submodules)
- [ ] Catalogizer submodule governance files (41+ submodules)
- [ ] HelixQA external submodule governance files (12 external projects)
- [ ] Whether Catalogizer's `submodule-analysis.txt` contains governance compliance data
- [ ] Whether HelixCode's `internal/isolated_files/` contains governance files
- [ ] Whether the `UNIVERSAL_MANDATORY_RULES.md` referenced in Catalogizer (at `/tmp/UNIVERSAL_MANDATORY_RULES.md`) contains Article XI §11.9
- [ ] HelixAgent root `CLAUDE.md` (referenced as canonical source for universal constraints)

---

*Analysis complete. All findings based on direct file inspection as of 2026-04-29.*
