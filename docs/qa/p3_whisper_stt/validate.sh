#!/usr/bin/env bash
# Golden self-validation of the local faster-whisper STT service (§11.4.107(10)).
# golden-good: transcript MUST contain the known key tokens.
# golden-bad : silence/noise MUST NOT return the known text.
# Runs 3x to prove deterministic (§11.4.50). Real HTTP calls to the running service.
set -u
URL="${STT_URL:-http://127.0.0.1:8123}"
DIR="$(cd "$(dirname "$0")" && pwd)"
FIX="$DIR/fixtures"
RUNS="${RUNS:-3}"
# key tokens the golden-good transcript must contain (case/punct-insensitive);
# "forty two" may normalize to "42" — accept either.
norm() { tr '[:upper:]' '[:lower:]' | tr -d '[:punct:]' | tr -s ' '; }

pass=0; fail=0
declare -A gg_seen
for run in $(seq 1 "$RUNS"); do
  echo "########## RUN $run/$RUNS ##########"

  # ---- golden-good ----
  gg=$(curl -fsS -X POST "$URL/v1/audio/transcriptions" -F "file=@$FIX/golden_good.wav")
  gg_text=$(printf '%s' "$gg" | python3 -c 'import sys,json;print(json.load(sys.stdin)["text"])')
  gg_nsp=$(printf '%s' "$gg" | python3 -c 'import sys,json;print(json.load(sys.stdin)["max_no_speech_prob"])')
  n=$(printf '%s' "$gg_text" | norm)
  gg_seen["$gg_text"]=1
  ok=1
  echo "$n" | grep -q "helix code test number" || ok=0
  # "42" or "forty two" both acceptable
  { echo "$n" | grep -qE "(42|forty two)"; } || ok=0
  # no_speech low
  awk "BEGIN{exit !($gg_nsp < 0.6)}" || ok=0
  if [ "$ok" = 1 ]; then echo "  [PASS] golden-good text='$gg_text' max_no_speech=$gg_nsp"; pass=$((pass+1));
  else echo "  [FAIL] golden-good text='$gg_text' max_no_speech=$gg_nsp"; fail=$((fail+1)); fi

  # ---- golden-bad (silence + noise): MUST NOT contain the known text ----
  for bad in golden_bad_silence golden_bad_noise; do
    br=$(curl -fsS -X POST "$URL/v1/audio/transcriptions" -F "file=@$FIX/$bad.wav")
    bt=$(printf '%s' "$br" | python3 -c 'import sys,json;print(json.load(sys.stdin)["text"])')
    bn=$(printf '%s' "$bt" | norm)
    if echo "$bn" | grep -q "helix code test number"; then
      echo "  [FAIL] $bad LEAKED known text: '$bt'"; fail=$((fail+1))
    else
      echo "  [PASS] $bad returned no known text (text='$bt')"; pass=$((pass+1))
    fi
  done
done

echo "=================================="
echo "golden-good distinct transcripts across runs: ${#gg_seen[@]} (1 == deterministic)"
echo "PASS=$pass FAIL=$fail"
if [ "$fail" -eq 0 ] && [ "${#gg_seen[@]}" -eq 1 ]; then
  echo "RESULT: GOLDEN SELF-VALIDATION PASS (deterministic)"; exit 0
else
  echo "RESULT: GOLDEN SELF-VALIDATION FAIL"; exit 1
fi
