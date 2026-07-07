#!/usr/bin/env python3
"""Minimal NLLB-200 CTranslate2 HTTP translation shim (CPU-only, no GPU).

Design: docs/research/07.2026/00_master/TRANSLATION_PROVIDER.md §1.2/§3.1 —
the "thin LibreTranslate-shaped shim wrapping ctranslate2.Translator +
SentencePiece" primary-lane engine. This shim implements the minimal
translate(text, src, tgt) contract needed for the Phase-3 NLLB proof; it does
NOT implement the full /v1/translate gateway route, auto-detect, or the
LibreTranslate-compatible batch/HTML-format contract (those are the full
provider's job per the design doc, out of scope for this proof harness).

Exposes:
  GET  /health     -> 200 {"status":"ok",...}     once the model is loaded
                   -> 503 {"status":"loading"}     while downloading/loading
                   -> 500 {"status":"error",...}   if load failed (honest, never silently retries forever)
  POST /translate  -> body {"q": "...", "source": "eng_Latn", "target": "deu_Latn"}
                   -> {"translatedText": "..."}

Loading + inference code path is the documented CTranslate2 + transformers
usage for NLLB (§11.4.150 citations, RESULTS.md "Sources verified"):
  translator = ctranslate2.Translator(model_dir, device="cpu")
  tokenizer  = transformers.AutoTokenizer.from_pretrained(model_dir, src_lang=src)
  source     = tokenizer.convert_ids_to_tokens(tokenizer.encode(text))
  results    = translator.translate_batch([source], target_prefix=[[tgt]])
  target     = results[0].hypotheses[0][1:]          # drop the leading lang token
  text_out   = tokenizer.decode(tokenizer.convert_tokens_to_ids(target))

No hardcoded model/host/port literal (§CONST-045/046) beyond safe in-container
defaults — every value the harness cares about is env-injected by
run_proof.sh via the compose file.
"""
import json
import os
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

# BLAS/OpenMP thread caps MUST be set BEFORE numpy/ctranslate2/OpenBLAS is
# imported anywhere in this process — OpenBLAS reads these env vars at its own
# lazy-init time (first BLAS call), not at ctranslate2.Translator(intra_threads=)
# construction time, and by default tries to spin up one thread PER DETECTED
# HOST CPU (e.g. 64 on this host) regardless of CT2's own intra_threads param.
# ROOT CAUSE (§11.4.102, captured 2026-07-07 run): a first attempt at this lane
# crashed with `OpenBLAS blas_thread_init: pthread_create failed ... RLIMIT_NPROC
# 4096 current, 5120 max` while constructing ctranslate2.Translator — OpenBLAS
# tried to init ~64 threads under host-wide process-count pressure (§11.4.174:
# shared host, other work may be running) and hit the container's process
# ulimit. Capping BLAS_THREADS explicitly (independent of CT2_INTRA_THREADS)
# fixes this at the source.
_BLAS_THREADS = os.environ.get("BLAS_NUM_THREADS", os.environ.get("CT2_INTRA_THREADS", "4"))
os.environ.setdefault("OPENBLAS_NUM_THREADS", _BLAS_THREADS)
os.environ.setdefault("OMP_NUM_THREADS", _BLAS_THREADS)
os.environ.setdefault("MKL_NUM_THREADS", _BLAS_THREADS)

MODEL_REPO = os.environ.get("MODEL_REPO", "")
MODEL_DIR_BASE = os.environ.get("MODEL_DIR", "/data/model")
# Repo-keyed subdirectory (§11.4.108/§11.4.139 clean-artifact integrity): a
# BUG in an earlier revision of this shim checked only "does model.bin exist
# at MODEL_DIR" to decide whether to (re)download, sharing ONE directory
# across every MODEL_REPO lane on the SAME persistent cache volume. When a
# PRIMARY-lane download completed but the process then crashed (see above),
# a SUBSEQUENT container booted with a DIFFERENT MODEL_REPO (the fallback)
# saw model.bin already present and silently served the WRONG (primary
# lane's) model under the fallback's name — a stale-shadow bluff. Keying the
# actual working directory by the repo id makes cross-lane collision
# structurally impossible: each MODEL_REPO gets its own subdirectory.
MODEL_DIR = os.path.join(MODEL_DIR_BASE, MODEL_REPO.replace("/", "__")) if MODEL_REPO else MODEL_DIR_BASE
HOST = os.environ.get("SHIM_HOST", "0.0.0.0")
PORT = int(os.environ.get("SHIM_PORT", "8000"))
INTER_THREADS = int(os.environ.get("CT2_INTER_THREADS", "1"))
INTRA_THREADS = int(os.environ.get("CT2_INTRA_THREADS", "4"))
BEAM_SIZE = int(os.environ.get("CT2_BEAM_SIZE", "1"))  # beam=1 (greedy) => deterministic, cheap on CPU

state = {"ready": False, "error": None, "translator": None, "tokenizer": None}
translate_lock = threading.Lock()


def log(msg):
    print(f"[{time.strftime('%H:%M:%S')}] shim: {msg}", flush=True)


def load_model():
    try:
        if not MODEL_REPO:
            raise RuntimeError("MODEL_REPO env var not set")
        need_download = not os.path.exists(os.path.join(MODEL_DIR, "model.bin"))
        if need_download:
            import huggingface_hub

            log(f"downloading {MODEL_REPO} -> {MODEL_DIR} (cold cache, may take a while) ...")
            huggingface_hub.snapshot_download(repo_id=MODEL_REPO, local_dir=MODEL_DIR)
            log("download complete")
        else:
            log(f"model already cached at {MODEL_DIR} (skip download)")

        import ctranslate2
        import transformers

        log(f"loading ctranslate2.Translator(device=cpu, inter={INTER_THREADS}, intra={INTRA_THREADS}) ...")
        translator = ctranslate2.Translator(
            MODEL_DIR,
            device="cpu",
            inter_threads=INTER_THREADS,
            intra_threads=INTRA_THREADS,
        )
        log("loading transformers.AutoTokenizer ...")
        tokenizer = transformers.AutoTokenizer.from_pretrained(MODEL_DIR)

        state["translator"] = translator
        state["tokenizer"] = tokenizer
        state["ready"] = True
        log("READY")
    except Exception as exc:  # honest surfacing, never a silent hang (§11.4.6)
        state["error"] = f"{type(exc).__name__}: {exc}"
        log(f"LOAD FAILED: {state['error']}")


def translate_one(text, src, tgt):
    tokenizer = state["tokenizer"]
    translator = state["translator"]
    # NLLB tokenizer requires src_lang set BEFORE encode() so the correct
    # source-language token is prepended (§11.4.150 citation, opennmt.net
    # transformers guide).
    tokenizer.src_lang = src
    source_tokens = tokenizer.convert_ids_to_tokens(tokenizer.encode(text))
    target_prefix = [[tgt]]
    with translate_lock:  # ctranslate2.Translator is not documented safe for
        # unsynchronised concurrent translate_batch calls from multiple
        # request-handler threads; serialise (this proof is not a throughput
        # benchmark — §11.4.85 stress/concurrency is future work).
        results = translator.translate_batch(
            [source_tokens], target_prefix=target_prefix, beam_size=BEAM_SIZE
        )
    hyp_tokens = results[0].hypotheses[0][1:]  # drop the leading target-lang token
    return tokenizer.decode(tokenizer.convert_tokens_to_ids(hyp_tokens))


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def _json(self, code, obj):
        body = json.dumps(obj).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/health":
            if state["ready"]:
                self._json(200, {"status": "ok", "model": MODEL_REPO})
            elif state["error"]:
                self._json(500, {"status": "error", "error": state["error"]})
            else:
                self._json(503, {"status": "loading"})
            return
        self._json(404, {"error": {"message": "not found", "type": "invalid_request_error"}})

    def do_POST(self):
        if self.path != "/translate":
            self._json(404, {"error": {"message": "not found", "type": "invalid_request_error"}})
            return
        if not state["ready"]:
            code = 500 if state["error"] else 503
            self._json(code, {"error": {"message": state["error"] or "warming", "type": "service_unavailable"}})
            return
        length = int(self.headers.get("Content-Length", "0") or "0")
        raw = self.rfile.read(length) if length else b"{}"
        try:
            req = json.loads(raw or b"{}")
            q = req["q"]
            src = req["source"]
            tgt = req["target"]
            if not isinstance(q, str) or not q:
                raise ValueError("q must be a non-empty string")
            out = translate_one(q, src, tgt)
            self._json(200, {"translatedText": out})
        except Exception as exc:
            self._json(400, {"error": {"message": str(exc), "type": "invalid_request_error"}})

    def log_message(self, fmt, *args):  # route access log to stderr (captured by `podman logs`)
        sys.stderr.write("%s - - [%s] %s\n" % (self.address_string(), self.log_date_time_string(), fmt % args))


def main():
    if not MODEL_REPO:
        log("FATAL: MODEL_REPO env not set")
        sys.exit(2)
    loader = threading.Thread(target=load_model, daemon=True)
    loader.start()
    server = ThreadingHTTPServer((HOST, PORT), Handler)
    log(f"serving on {HOST}:{PORT} (model load running in background) ...")
    server.serve_forever()


if __name__ == "__main__":
    main()
