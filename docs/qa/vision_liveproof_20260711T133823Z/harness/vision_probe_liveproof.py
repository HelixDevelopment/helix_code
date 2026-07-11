#!/usr/bin/env python3
"""
r41f vision live-reproof — grounded VLM probe + golden-BAD self-validation.

Re-proves the Phase-3 (e857d59) VLM capability against the live
cmd/visiongen-boot service on :18439, without modifying submodules/helix_llm.
Modeled on submodules/helix_llm/docs/qa/phase3_vision_20260707/harness/
vision_probe.py (read-only reference, not copied verbatim).

Feeds the SAME known-content image (RED CIRCLE on WHITE, text "HELIX",
md5 b982a8295f408e93a662db916f9d543a) to the live VLM over the OpenAI vision
API, then:

  §11.4.108 runtime signature — GROUNDED assertion: the model's real
    description MUST CONTAIN the known content (color 'red' AND a
    circle-word). Grounded in the actual pixels.

  §11.4.107(10) golden-BAD — the SAME checker run with a WRONG-content
    assertion (color 'blue' AND shape 'square', NOT in the image) MUST FAIL.
    If the golden-bad passed, the checker would be a rubber stamp.

Exit 0 only when GROUNDED passes AND golden-BAD fails on the real response.
"""
import base64
import json
import os
import sys
import time
import urllib.request

vlm_url, image_path, evdir, model_id = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
os.makedirs(evdir, exist_ok=True)

with open(image_path, "rb") as f:
    b64 = base64.b64encode(f.read()).decode("ascii")
data_uri = f"data:image/png;base64,{b64}"

PROMPT = "Describe this image in detail. What shape and color is the main object, and what text (if any) appears?"

req_body = {
    "model": model_id,
    "messages": [
        {
            "role": "user",
            "content": [
                {"type": "text", "text": PROMPT},
                {"type": "image_url", "image_url": {"url": data_uri}},
            ],
        }
    ],
    "temperature": 0.0,
    "max_tokens": 256,
}

req_record = json.loads(json.dumps(req_body))
req_record["messages"][0]["content"][1]["image_url"]["url"] = (
    f"data:image/png;base64,<{len(b64)} base64 chars of {os.path.basename(image_path)}>"
)
with open(os.path.join(evdir, "03_vision_request.json"), "w") as f:
    json.dump({"endpoint": f"{vlm_url}/v1/chat/completions", "prompt": PROMPT, "body": req_record}, f, indent=2)

t0 = time.time()
r = urllib.request.Request(
    f"{vlm_url}/v1/chat/completions",
    data=json.dumps(req_body).encode(),
    headers={"Content-Type": "application/json"},
)
with urllib.request.urlopen(r, timeout=180) as resp:
    raw = resp.read().decode()
elapsed = time.time() - t0
out = json.loads(raw)
with open(os.path.join(evdir, "04_vision_response.json"), "w") as f:
    json.dump(out, f, indent=2)

description = out["choices"][0]["message"]["content"]
desc_l = description.lower()

BLUFF_MARKERS = ["simulated", "for now", "placeholder", "todo implement"]
bluff_hit = [m for m in BLUFF_MARKERS if m in desc_l]


def all_present(text, tokens):
    return all(any(v in text for v in variants) for variants in tokens)


# GROUNDED positive: color red AND a circle-word (both must be present).
GROUNDED = [["red"], ["circle", "round", "disc", "dot"]]
# GOLDEN-BAD wrong content: color blue AND shape square (NOT in the image).
GOLDEN_BAD = [["blue"], ["square", "rectangle", "triangle"]]

grounded_pass = all_present(desc_l, GROUNDED)
golden_bad_pass = all_present(desc_l, GOLDEN_BAD)  # MUST be False on the real response
ocr_helix = "helix" in desc_l

verdict = {
    "model": model_id,
    "vlm_url": vlm_url,
    "image": os.path.basename(image_path),
    "image_md5": "b982a8295f408e93a662db916f9d543a",
    "known_content": "RED CIRCLE on WHITE background, text 'HELIX'",
    "latency_seconds": round(elapsed, 2),
    "description": description,
    "bluff_markers_found": bluff_hit,
    "grounded_assertion": {
        "tokens_required_all_of": GROUNDED,
        "result": "PASS" if grounded_pass else "FAIL",
    },
    "golden_bad_assertion": {
        "tokens_required_all_of": GOLDEN_BAD,
        "must_be": "FAIL",
        "result": "PASS(BAD)-analyzer-would-be-a-bluff" if golden_bad_pass else "FAIL(as-required)-analyzer-honest",
    },
    "ocr_helix_bonus": ocr_helix,
    "overall": "PASS" if (grounded_pass and not golden_bad_pass and not bluff_hit) else "FAIL",
}
with open(os.path.join(evdir, "05_grounded_assertion.json"), "w") as f:
    f.write(json.dumps(verdict, indent=2) + "\n")

print(json.dumps(verdict, indent=2))
sys.exit(0 if verdict["overall"] == "PASS" else 1)
