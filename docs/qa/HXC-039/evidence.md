# HXC-039 — G7 §11.4.83 docs/qa evidence for 8 historical commits (operator: add-after-the-fact)
Created 8 docs/qa/<run-id>/evidence.md (run-id = verbatim substring of each commit subject so verify_qa_evidence.sh matches):
unique(81f3c482) real-dial(83b2690a) Phase-2(cee5cdae) W6B(d985e3ae) round468d(5c5c44bc) round468c(c63c8963)
round468b(3ce30285) ref-hygiene(9970507d). Each cites SHA+subject+git-show-stat+in-commit test delta, banner-labeled
"RECONSTRUCTED AFTER-THE-FACT" with NO fabricated runtime output (§11.4.6/§11.4.123). The 2 pure-meta commits note a
[no-qa-evidence] opt-out would also have applied. PROOF: verify_qa_evidence.sh --enforce --since ed84f90e → 8 → 0
violations, RESULT PASS exit 0. helix_qa... no — meta commit 1c8fe168 (8 files).

## Live re-verification (2026-06-09T15:21:32Z)
`bash scripts/verify_qa_evidence.sh --enforce --since ed84f90e`:
  Feature-shipping commits evaluated : 28
  Violations                         : 0
  RESULT: PASS (enforcing — no violations in ed84f90e..HEAD), exit 0.
All 8 reconstructed dirs tracked in commit 1c8fe168 (unique/real-dial/Phase-2/W6B/round468d/round468c/round468b/ref-hygiene).
