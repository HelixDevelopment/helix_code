#!/usr/bin/env bash
# Phase-0 build + acceptance-proof driver (P0-T2/T3/T1).
# ADR-0001: pinned CUDA 12.8, sm_120 (Blackwell), rootless podman.
#
# STAGES:
#   build   — build cuda-base + llamacpp images (NO GPU needed; runs before the toolkit install)
#   verify  — GPU acceptance proofs (REQUIRES nvidia-container-toolkit + CDI; run AFTER `sudo apt-get install -y nvidia-container-toolkit`)
#
# Every proof is captured to $EVID (§11.4.5/§11.4.69/§11.4.108). Build-success is NOT acceptance;
# the acceptance signature is a REAL inference response + a REAL nvidia-smi VRAM delta.
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(git -C "$HERE" rev-parse --show-toplevel)"
LLAMACPP_SRC="${LLAMACPP_SRC:-$REPO/dependencies/LLama_CPP}"
CUDA_BASE_TAG="helixllm/cuda-base:12.8.1"
LLAMACPP_TAG="helixllm/llamacpp:cuda12.8-sm120"
CUDA_RUN_BASE="docker.io/nvidia/cuda:12.8.1-base-ubuntu22.04"
EVID="${EVID:-$REPO/docs/qa/phase0_$(git -C "$REPO" rev-parse --short HEAD)}"
mkdir -p "$EVID"

stage="${1:-build}"

build() {
  echo "== [P0-T2] build cuda-base ($CUDA_BASE_TAG) — no GPU needed =="
  podman build -f "$HERE/Dockerfile.cuda-base" -t "$CUDA_BASE_TAG" "$HERE" 2>&1 | tee "$EVID/p0t2_cuda_base_build.log"
  echo "== [P0-T3] build llama.cpp for sm_120 ($LLAMACPP_TAG) from $LLAMACPP_SRC — no GPU needed =="
  podman build -f "$HERE/Dockerfile.llamacpp" --build-arg "CUDA_BASE=$CUDA_BASE_TAG" \
      -t "$LLAMACPP_TAG" "$LLAMACPP_SRC" 2>&1 | tee "$EVID/p0t3_llamacpp_build.log"
  echo "== build-layer proof (no GPU needed; §11.4.108): artifacts present + CUDA backend linked =="
  # Do NOT run the binary here — it dlopens libcuda.so.1 (driver, runtime-only via CDI).
  # ldd is the honest build-layer signature: libggml-cuda.so present = CUDA backend compiled in;
  # 'libcuda.so.1 => not found' is EXPECTED pre-GPU and itself proves the driver dep is wired.
  podman run --rm --entrypoint /bin/sh "$LLAMACPP_TAG" -c '
      echo "--- /opt/llamacpp artifacts ---"; ls -la /opt/llamacpp/ | grep -E "llama-server|llama-cli|\.so" ;
      echo "--- ldd llama-server ---"; ldd /opt/llamacpp/llama-server 2>&1 | grep -Ei "ggml|cuda|llama|not found" || true
  ' 2>&1 | tee "$EVID/p0t3_llamacpp_buildproof.log"
  echo "build stage done. Evidence: $EVID"
}

verify() {
  echo "== [P0-T1] rootless GPU passthrough proof (REQUIRES toolkit + CDI) =="
  echo "  prerequisites (run once): "
  echo "    sudo apt-get install -y nvidia-container-toolkit"
  echo "    nvidia-ctk config --set nvidia-container-cli.no-cgroups --in-place   # (may need sudo)"
  echo "    nvidia-ctk cdi generate --output=\$HOME/.config/cdi/nvidia.yaml"
  podman run --rm --device nvidia.com/gpu=all --security-opt=label=disable \
      "$CUDA_RUN_BASE" nvidia-smi 2>&1 | tee "$EVID/p0t1_gpu_passthrough_nvidia_smi.log"
  echo "== [P0-T3] REAL llama.cpp GPU proof: (needs a small GGUF mounted at /models) =="
  echo "  example (fill MODEL): "
  echo "    podman run --rm --device nvidia.com/gpu=all --security-opt=label=disable \\"
  echo "      -v \$HOME/models:/models -p 8080:8080 $LLAMACPP_TAG \\"
  echo "      -m /models/<small>.gguf -ngl 99 -c 4096 --host 0.0.0.0 --port 8080 &"
  echo "    curl -s localhost:8080/v1/completions -d '{\"prompt\":\"2+2=\",\"n_predict\":8}'"
  echo "    # acceptance = non-empty completion AND nvidia-smi shows a VRAM delta (capture both)"
  echo "verify stage: capture the above outputs to $EVID as the §11.4.108 runtime signature."
}

case "$stage" in
  build)  build ;;
  verify) verify ;;
  *) echo "usage: $0 {build|verify}"; exit 2 ;;
esac
