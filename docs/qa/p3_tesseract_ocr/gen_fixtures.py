"""Generate golden-good and golden-bad OCR fixtures with KNOWN content (§11.4.107(10)).

golden-good  : black text on white, known tokens -> OCR MUST extract them at high conf.
golden-bad   : pure white + faint gaussian noise, NO text -> OCR MUST NOT return the tokens.

Deterministic (fixed RNG seed) so the self-validation is reproducible (§11.4.50).
"""
from __future__ import annotations

import os

import numpy as np
from PIL import Image, ImageDraw, ImageFont

HERE = os.path.dirname(os.path.abspath(__file__))
FIX = os.path.join(HERE, "fixtures")
os.makedirs(FIX, exist_ok=True)

# The known ground-truth tokens the golden-good image contains.
KNOWN_TEXT = "HELIX OCR TEST 42"
KNOWN_TOKENS = ["HELIX", "OCR", "TEST", "42"]


def _font(size: int) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    for path in (
        "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
        "/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf",
        "/usr/share/fonts/TTF/DejaVuSans-Bold.ttf",
    ):
        if os.path.exists(path):
            return ImageFont.truetype(path, size)
    return ImageFont.load_default()


def gen_good() -> str:
    # A genuine multi-line TEXT BLOCK (what the service's PSM 6 default is designed
    # for). A single wide line with tight kerning merged "HELIX OCR" -> "HELIXOOR"
    # under PSM 6; a 2-line block reads all 4 tokens cleanly at conf ~94-96.
    font = _font(90)
    lines = ["HELIX OCR", "TEST 42"]  # tokens == KNOWN_TOKENS
    pad, line_h = 50, 120
    probe = ImageDraw.Draw(Image.new("RGB", (10, 10)))
    W = max(probe.textbbox((0, 0), ln, font=font)[2] for ln in lines) + 2 * pad
    H = line_h * len(lines) + 2 * pad
    img = Image.new("RGB", (W, H), "white")
    d = ImageDraw.Draw(img)
    for i, ln in enumerate(lines):
        d.text((pad, pad + i * line_h), ln, fill="black", font=font)
    p = os.path.join(FIX, "golden_good.png")
    img.save(p)
    return p


def gen_bad() -> str:
    rng = np.random.default_rng(42)
    # white canvas + faint noise, NO glyphs at all
    arr = np.full((200, 640, 3), 255, dtype=np.int16)
    noise = rng.normal(0, 6, arr.shape)  # sigma 6 -> imperceptible, no letters
    arr = np.clip(arr + noise, 0, 255).astype(np.uint8)
    p = os.path.join(FIX, "golden_bad.png")
    Image.fromarray(arr).save(p)
    return p


if __name__ == "__main__":
    print("golden-good:", gen_good(), "| known tokens:", KNOWN_TOKENS)
    print("golden-bad :", gen_bad(), "| contains NO text")
