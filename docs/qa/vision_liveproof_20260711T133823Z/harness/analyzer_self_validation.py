#!/usr/bin/env python3
"""
§11.4.107(10) analyzer self-validation — golden-good/golden-bad FIXTURE pair
run directly against the grounded-assertion checker logic (no network call).

This is the strict fixture-pair variant: instead of only checking that the
real VLM response happens not to mention wrong-content tokens (done in
vision_probe_liveproof.py), we feed the checker two SYNTHETIC descriptions:

  golden-good fixture: text that DOES describe the known content (red circle)
    -> checker MUST report overall PASS.
  golden-bad fixture:  text that describes WRONG content (blue square, and
    explicitly does NOT mention red/circle) -> checker MUST report overall
    FAIL.

If the golden-bad fixture ever PASSes, the checker is a rubber stamp
(mutation-caught bluff gate, §1.1 pattern). Exit 0 only when BOTH fixtures
produce their required verdict.
"""
import json
import sys


def all_present(text, tokens):
    return all(any(v in text for v in variants) for variants in tokens)


GROUNDED = [["red"], ["circle", "round", "disc", "dot"]]
GOLDEN_BAD_TOKENS = [["blue"], ["square", "rectangle", "triangle"]]

GOLDEN_GOOD_FIXTURE = (
    "the image shows a solid red circle centered on a white background, "
    "with the word helix printed below it in black letters."
)
GOLDEN_BAD_FIXTURE = (
    "the image shows a solid blue square centered on a white background, "
    "with the word acme printed below it in black letters."
)


def verdict_for(text):
    grounded_pass = all_present(text, GROUNDED)
    golden_bad_pass = all_present(text, GOLDEN_BAD_TOKENS)
    return {
        "text": text,
        "grounded_pass": grounded_pass,
        "golden_bad_tokens_pass": golden_bad_pass,
        "overall": "PASS" if (grounded_pass and not golden_bad_pass) else "FAIL",
    }


good = verdict_for(GOLDEN_GOOD_FIXTURE)
bad = verdict_for(GOLDEN_BAD_FIXTURE)

result = {
    "golden_good_fixture": good,
    "golden_bad_fixture": bad,
    "expect": {"golden_good_fixture.overall": "PASS", "golden_bad_fixture.overall": "FAIL"},
    "self_validation": "PASS" if (good["overall"] == "PASS" and bad["overall"] == "FAIL") else "FAIL",
}
print(json.dumps(result, indent=2))
sys.exit(0 if result["self_validation"] == "PASS" else 1)
