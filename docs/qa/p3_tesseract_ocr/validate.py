"""Golden self-validation of the /v1/ocr service (§11.4.107(10) anti-bluff core).

golden-good : ASSERT extracted text CONTAINS the known tokens AND mean_conf > floor.
golden-bad  : ASSERT the known tokens are NOT returned (analyzer cannot bluff).

Exits 0 only if BOTH hold. Captures the REAL endpoint responses to evidence/.
The confidence floor is CALIBRATED from the actual golden-good output, not guessed
from literature (§11.4.6 / §11.4.107(13)) — see README.
"""
from __future__ import annotations

import base64
import json
import os
import sys
import time

import requests

from gen_fixtures import KNOWN_TOKENS

HERE = os.path.dirname(os.path.abspath(__file__))
EVID = os.path.join(HERE, "evidence")
FIX = os.path.join(HERE, "fixtures")
os.makedirs(EVID, exist_ok=True)

# Consume the fixtures generated deterministically INSIDE the container (pinned
# DejaVu font + pinned PIL). NEVER regenerate here: the host lacks the container's
# font, so a host re-render silently produces a broken tiny-font image.
GOLDEN_GOOD = os.path.join(FIX, "golden_good.png")
GOLDEN_BAD = os.path.join(FIX, "golden_bad.png")

BASE = os.environ.get("OCR_BASE_URL", "http://127.0.0.1:8080")
# Calibrated on the actual golden-good run (see README "Calibrated confidence floor").
CONF_FLOOR = float(os.environ.get("OCR_CONF_FLOOR", "80"))


def _b64(path: str) -> str:
    with open(path, "rb") as fh:
        return base64.b64encode(fh.read()).decode()


def _post(path: str) -> dict:
    r = requests.post(f"{BASE}/v1/ocr", json={"image": _b64(path), "lang": "eng"}, timeout=60)
    r.raise_for_status()
    return r.json()


def _write(name: str, obj: dict) -> None:
    with open(os.path.join(EVID, name), "w") as fh:
        json.dump(obj, fh, indent=2)


def main() -> int:
    # wait for the service to come up
    for _ in range(60):
        try:
            h = requests.get(f"{BASE}/health", timeout=5).json()
            break
        except Exception:
            time.sleep(1)
    else:
        print("FAIL: service /health never came up at", BASE)
        return 2
    _write("health.json", h)
    print("engine health:", json.dumps(h))

    if not (os.path.exists(GOLDEN_GOOD) and os.path.exists(GOLDEN_BAD)):
        print("FAIL: fixtures missing — run gen_fixtures.py INSIDE the container first")
        return 2
    good_png, bad_png = GOLDEN_GOOD, GOLDEN_BAD

    good = _post(good_png)
    _write("golden_good_response.json", good)
    bad = _post(bad_png)
    _write("golden_bad_response.json", bad)

    good_text = good["full_text"].upper()
    good_tokens_present = [t for t in KNOWN_TOKENS if t in good_text]
    bad_text = bad["full_text"].upper()
    bad_tokens_present = [t for t in KNOWN_TOKENS if t in bad_text]

    ok_good_tokens = set(good_tokens_present) == set(KNOWN_TOKENS)
    ok_good_conf = good["mean_conf"] > CONF_FLOOR
    ok_bad = len(bad_tokens_present) == 0

    verdict = {
        "conf_floor": CONF_FLOOR,
        "golden_good": {
            "full_text": good["full_text"],
            "mean_conf": good["mean_conf"],
            "tokens_expected": KNOWN_TOKENS,
            "tokens_found": good_tokens_present,
            "assert_all_tokens_present": ok_good_tokens,
            "assert_mean_conf_above_floor": ok_good_conf,
        },
        "golden_bad": {
            "full_text": bad["full_text"],
            "mean_conf": bad["mean_conf"],
            "tokens_leaked": bad_tokens_present,
            "assert_no_known_tokens": ok_bad,
        },
        "PASS": bool(ok_good_tokens and ok_good_conf and ok_bad),
    }
    _write("self_validation_verdict.json", verdict)
    print(json.dumps(verdict, indent=2))

    if verdict["PASS"]:
        print("\nSELF-VALIDATION PASS: analyzer reads real text AND cannot bluff on blank input.")
        return 0
    print("\nSELF-VALIDATION FAIL")
    return 1


if __name__ == "__main__":
    sys.exit(main())
