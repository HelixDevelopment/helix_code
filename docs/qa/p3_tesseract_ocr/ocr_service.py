"""Unified /v1/ocr HTTP service — Tesseract 5.x, OEM 1 (LSTM), per-word conf + bbox.

Implements the stream-07 report's unified OCR contract (§2.3):
    POST /v1/ocr  { image: <b64>, roi?: [x,y,w,h], lang?: "eng", min_conf?: 60 }
    -> { engine, words: [ {text, conf, bbox:[x,y,w,h], line, block} ], full_text, mean_conf }

Real OCR only — NO simulation, NO hardcoded results (§11.4.6 / anti-bluff). Every
word/conf/bbox comes from pytesseract.image_to_data on the actual pixels supplied.
"""
from __future__ import annotations

import base64
import io
import os
import subprocess

import numpy as np
import pytesseract
from fastapi import FastAPI
from PIL import Image
from pydantic import BaseModel

app = FastAPI(title="helix-ocr", version="1.0.0")

# OEM 1 = LSTM engine (most accurate, the report's mandated mode); PSM 6 = single
# uniform block of text (good default for UI/subtitle/label frames).
TESS_CONFIG = "--oem 1 --psm 6"


class OCRRequest(BaseModel):
    image: str  # base64-encoded image bytes
    roi: list[int] | None = None  # [x, y, w, h]
    lang: str = "eng"
    min_conf: float | None = None


def _tesseract_version() -> str:
    try:
        return str(pytesseract.get_tesseract_version())
    except Exception as exc:  # pragma: no cover - surfaced via /health
        return f"error: {exc}"


@app.get("/health")
def health() -> dict:
    # Prove the real engine is present and callable (§11.4.108 runtime signature).
    return {
        "status": "ok",
        "engine": "tesseract",
        "tesseract_version": _tesseract_version(),
        "config": TESS_CONFIG,
        "tessdata_prefix": os.environ.get("TESSDATA_PREFIX", ""),
    }


@app.post("/v1/ocr")
def ocr(req: OCRRequest) -> dict:
    raw = base64.b64decode(req.image)
    img = Image.open(io.BytesIO(raw)).convert("RGB")

    if req.roi:
        x, y, w, h = req.roi
        img = img.crop((x, y, x + w, y + h))

    min_conf = req.min_conf if req.min_conf is not None else float(os.environ.get("OCR_MIN_CONF", 60))

    data = pytesseract.image_to_data(
        img, lang=req.lang, config=TESS_CONFIG, output_type=pytesseract.Output.DICT
    )

    words = []
    confs = []
    for i in range(len(data["text"])):
        text = data["text"][i].strip()
        conf = float(data["conf"][i])
        if not text or conf < 0:  # tesseract emits conf=-1 for layout-only rows
            continue
        words.append(
            {
                "text": text,
                "conf": conf,
                "bbox": [data["left"][i], data["top"][i], data["width"][i], data["height"][i]],
                "line": data["line_num"][i],
                "block": data["block_num"][i],
            }
        )
        confs.append(conf)

    kept = [w for w in words if w["conf"] >= min_conf]
    mean_conf = float(np.mean([w["conf"] for w in kept])) if kept else 0.0
    full_text = " ".join(w["text"] for w in kept)

    return {
        "engine": f"tesseract-{_tesseract_version()}",
        "config": TESS_CONFIG,
        "lang": req.lang,
        "min_conf": min_conf,
        "words": kept,
        "words_all": words,
        "full_text": full_text,
        "mean_conf": round(mean_conf, 2),
    }
