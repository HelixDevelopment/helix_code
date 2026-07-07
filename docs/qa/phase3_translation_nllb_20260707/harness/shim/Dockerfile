# Phase-3 NLLB-200 CTranslate2 primary-lane shim image (CPU-only, no GPU).
#
# Small python:3.11-slim base + ctranslate2 (C++ CPU int8/float kernels) +
# transformers (tokenizer only — no torch needed for AutoTokenizer) +
# sentencepiece/protobuf (NLLB tokenizer backend) + huggingface_hub (model
# download at container startup into a persistent cache volume, §11.4.77).
#
# Design: docs/research/07.2026/00_master/TRANSLATION_PROVIDER.md §3.1.
# Built + referenced by image tag (not by compose "build:") to match the
# existing embeddings/translation harness precedent of pre-built images
# referenced via ${VAR} in the compose file (§CONST-045/046).
FROM docker.io/library/python:3.11-slim

RUN pip install --no-cache-dir --root-user-action=ignore \
      ctranslate2 \
      transformers \
      sentencepiece \
      protobuf \
      huggingface_hub

WORKDIR /app
COPY server.py /app/server.py

EXPOSE 8000
ENTRYPOINT ["python", "/app/server.py"]
