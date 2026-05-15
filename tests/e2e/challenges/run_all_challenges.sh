#!/bin/bash
# Run all HelixCode anti-bluff challenges

cd "$(dirname "$0")/../../.." # Change to repo root

# --- Governance cascade gate (CONST-035 / Article XI §11.9) ---
echo "[PRE-CHECK] Running governance cascade verification..."
if [[ -x "scripts/verify-governance-cascade.sh" ]]; then
  ./scripts/verify-governance-cascade.sh
  if [[ $? -ne 0 ]]; then
    echo "[BLOCKED] Governance cascade verification failed. Merge prohibited."
    exit 42
  fi
else
  echo "[BLOCKED] Governance verification script not found or not executable."
  exit 43
fi
echo "[PRE-CHECK] Governance cascade verification passed."
# --- End governance cascade gate ---

echo "=========================================="
echo "  HelixCode Anti-Bluff Challenge Runner"
echo "=========================================="
echo ""

PASSED=0
FAILED=0

for phase in 1 2 3 4 5 6 7 8; do
    echo "=== Running Phase $phase Challenge ==="
    SCRIPT=$(ls tests/e2e/challenges/phase${phase}_*challenge.sh 2>/dev/null | head -1)
    if [ -z "$SCRIPT" ]; then
        echo "❌ Phase $phase: Script not found"
        ((FAILED++))
    elif bash "$SCRIPT" 2>&1; then
        echo "✅ Phase $phase: PASSED"
        ((PASSED++))
    else
        echo "❌ Phase $phase: FAILED"
        ((FAILED++))
    fi
    echo ""
done

# gptme port anti-bluff challenges (subagent Role, verifier profile, cache coldness).
# Listed explicitly so the existing phase{N}_*.sh glob is undisturbed.
GPTME_SCRIPTS=(
    "tests/e2e/challenges/gptme_subagent_role.sh"
    "tests/e2e/challenges/gptme_verifier_profile.sh"
    "tests/e2e/challenges/gptme_cache_coldness.sh"
    # Live-server anti-bluff harness — exercises HelixCode HTTP endpoints
    # against the mistborn-distributed stack with captured wire evidence.
    # SKIPs gracefully when mistborn tunnels (:15432) aren't bound.
    "tests/e2e/challenges/helix_qa_live_anti_bluff.sh"
    # Conversational REPL anti-bluff Challenge (round 41) — verifies the
    # interactive REPL accepts multi-word LLM prompts (regression-guard
    # against the round-pre-41 fmt.Scanln-one-word-truncation form) and
    # exercises a live LLM round-trip with paired structural + runtime
    # gates. Honest SKIP-OK when no provider key in env.
    "tests/e2e/challenges/conversational_repl.sh"
    # @-file-mention anti-bluff Challenge (round 41) — verifies the
    # REPL recognises `@<path>` tokens, attaches the file content to
    # the prompt sent to the LLM, surfaces the 📎-marker to the user,
    # and the LLM actually receives + quotes back a unique sentinel
    # from the file (proving the attachment isn't a bluff). Also
    # proves non-existent @-mentions stay verbatim with no crash.
    "tests/e2e/challenges/at_mention.sh"
    # DDoS Health-Flood anti-bluff Challenge (round 41) — first
    # installment of the 6 missing test types from Task #256 per
    # CONST-050(B)'s matrix. Floods /api/v1/health with concurrent
    # requests + asserts pass-rate threshold + post-flood liveness.
    # Captures p50/p95/p99 latencies as wire evidence. Honest
    # SKIP-OK when server isn't reachable.
    "tests/e2e/challenges/ddos_health_flood.sh"
    # Stress Sustained-Load anti-bluff Challenge (round 41) — 2nd
    # of the 6 missing test types per Task #266. Differs from DDoS
    # (burst): sustained load over N seconds at target rps,
    # per-second pass-rate snapshot, latency-degradation check
    # vs pre-stress baseline (catches "still-200-but-zombie-slow"
    # class that pass-rate gate would miss). Honest SKIP-OK
    # pattern.
    "tests/e2e/challenges/stress_sustained_load.sh"
    # Chaos Failure-Injection anti-bluff Challenge (round 41) — 3rd
    # of the 6 missing test types per Task #266. Operator-safe
    # client-side fault injection (no sudo / no iptables / no host
    # mutation per CONST-033): malformed HTTP, oversized headers,
    # slow-loris, abrupt connection close + concurrent legit-traffic
    # control group. Catches "chaos starves real users" + "fell-over-
    # after-chaos" zombie classes. Honest SKIP-OK pattern.
    "tests/e2e/challenges/chaos_failure_injection.sh"
    # Horizontal-Scaling anti-bluff Challenge (round 41) — 4th of
    # the 6 missing test types per Task #266. Topology-dispatch
    # to SKIP-OK on single-node dev boxes (the common case); when
    # SCALING_REPLICA_URLS resolves to ≥2 reachable replicas,
    # exercises per-replica load + load-balance fairness sanity +
    # body-identity sha256 across replicas (catches the "different
    # replicas → different upstream config" misconfiguration class).
    "tests/e2e/challenges/scaling_horizontal.sh"
)
for SCRIPT in "${GPTME_SCRIPTS[@]}"; do
    NAME=$(basename "$SCRIPT" .sh)
    echo "=== Running $NAME Challenge ==="
    if [ ! -f "$SCRIPT" ]; then
        echo "❌ $NAME: Script not found"
        ((FAILED++))
    elif bash "$SCRIPT" 2>&1; then
        echo "✅ $NAME: PASSED"
        ((PASSED++))
    else
        echo "❌ $NAME: FAILED"
        ((FAILED++))
    fi
    echo ""
done

echo "=========================================="
echo "  Results: $PASSED passed, $FAILED failed"
echo "=========================================="

if [ $FAILED -gt 0 ]; then
    exit 1
fi

echo "🎉 ALL CHALLENGES PASSED! Anti-Bluff Verification: COMPLETE"
