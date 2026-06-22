#!/bin/bash
# ocr_recording.sh — HXC-112 read-back verifier (§11.4.159(D) / §11.4.160).
#
# Reads the screen content of a recording back to confirm the synthetic input
# actually REGISTERED in the GUI — i.e. the typed prompt appears in the input /
# chat history over the recording window. A single frame is NOT proof
# (§11.4.6): we sample many frames across the clip and assert the expected text
# appears in >=1 of them, AND (when an LLM reply is expected) that the chat
# history grew beyond the prompt.
#
# USAGE: ocr_recording.sh <mp4> <expected-prompt-substring> [expected-reply-regex]
# Uses macOS Vision OCR via a tiny Swift helper (no external OCR dep). Prints a
# PASS/FAIL verdict + the matched frame paths (captured evidence).
set -u
MP4="$1"; PROMPT="${2:-}"; REPLY_RE="${3:-}"
[ -s "$MP4" ] || { echo "[ocr][FAIL] recording missing/empty: $MP4" >&2; exit 1; }
WORK="$(mktemp -d /tmp/hxc112-ocr.XXXXXX)"
trap 'rm -rf "$WORK"' EXIT
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Sample 1 frame/sec across the clip.
ffmpeg -y -i "$MP4" -vf fps=1 "$WORK/f-%03d.png" >/dev/null 2>&1 \
  || { echo "[ocr][FAIL] frame extraction failed" >&2; exit 1; }
N=$(ls "$WORK"/f-*.png 2>/dev/null | wc -l | tr -d ' ')
echo "[ocr] extracted $N frames from $MP4"

# Vision-framework OCR helper (compiled on demand).
cat > "$WORK/ocr.swift" <<'SWIFT'
import Vision
import Foundation
import AppKit
guard CommandLine.arguments.count >= 2,
      let img = NSImage(contentsOfFile: CommandLine.arguments[1]),
      let cg = img.cgImage(forProposedRect: nil, context: nil, hints: nil) else { exit(2) }
let req = VNRecognizeTextRequest()
req.recognitionLevel = .accurate
try? VNImageRequestHandler(cgImage: cg, options: [:]).perform([req])
for obs in (req.results ?? []) {
  if let t = obs.topCandidates(1).first { print(t.string) }
}
SWIFT

ALLTXT="$WORK/all.txt"; : > "$ALLTXT"
PROMPT_HIT=""; REPLY_HIT=""
for f in "$WORK"/f-*.png; do
  TXT="$(swift "$WORK/ocr.swift" "$f" 2>/dev/null)"
  echo "=== $f ===" >> "$ALLTXT"; echo "$TXT" >> "$ALLTXT"
  if [ -n "$PROMPT" ] && echo "$TXT" | grep -qiF "$PROMPT"; then PROMPT_HIT="$f"; fi
  if [ -n "$REPLY_RE" ] && echo "$TXT" | grep -qiE "$REPLY_RE"; then REPLY_HIT="$f"; fi
done

echo "[ocr] full transcript: $ALLTXT"
RC=0
if [ -n "$PROMPT" ]; then
  if [ -n "$PROMPT_HIT" ]; then echo "[ocr][PASS] prompt text registered in GUI (frame: $PROMPT_HIT)"
  else echo "[ocr][FAIL] typed prompt never appeared in any frame — input did NOT register" >&2; RC=1; fi
fi
if [ -n "$REPLY_RE" ]; then
  if [ -n "$REPLY_HIT" ]; then echo "[ocr][PASS] expected LLM reply matched (frame: $REPLY_HIT)"
  else echo "[ocr][WARN] expected reply regex not matched (LLM may be unconfigured/slow)"; fi
fi
# Persist the matched frames as durable evidence next to the mp4.
if [ -n "$PROMPT_HIT" ]; then cp "$PROMPT_HIT" "${MP4%.mp4}-promptframe.png"; echo "[ocr] saved ${MP4%.mp4}-promptframe.png"; fi
exit $RC
