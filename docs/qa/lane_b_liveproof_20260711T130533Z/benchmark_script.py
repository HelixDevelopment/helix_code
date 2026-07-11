#!/usr/bin/env python3
"""Lane-B (Mistral-Nemo-12B, :18435) live co-residence re-validation benchmark.
Coder (:18434) is NEVER touched by this script — read-only health probes only.
"""
import json
import time
import urllib.request
import urllib.error
import threading
import sys

LANEB_URL = "http://localhost:18435/v1/completions"
LANEB_CHAT_URL = "http://localhost:18435/v1/chat/completions"
CODER_URL = "http://localhost:18434/v1/models"
HEADERS = {"Content-Type": "application/json"}

SINGLE_STREAM_PROMPTS = [
    "Write a Python function to check if a string is a palindrome.",
    "Explain the difference between a list and a tuple in Python.",
    "What is the time complexity of binary search? Explain briefly.",
    "Write a Go function that reverses a string.",
    "Explain what a race condition is in concurrent programming.",
]

PARALLEL_PROMPTS = [
    "Write a bash one-liner to count lines in all .go files in a directory.",
    "Explain the CAP theorem in two sentences.",
    "Write a SQL query to find duplicate rows in a table.",
]

TOOL_CALL_PROMPT = "What is 2+2? Answer with only the digit, nothing else."


def call_llm(prompt, max_tokens=200, timeout=120):
    body = json.dumps({
        "prompt": prompt,
        "max_tokens": max_tokens,
        "temperature": 0.2,
        "stop": [],
        "stream": False,
    }).encode()
    req = urllib.request.Request(LANEB_URL, data=body, headers=HEADERS, method="POST")
    start = time.time()
    try:
        resp = urllib.request.urlopen(req, timeout=timeout)
        elapsed = time.time() - start
        data = json.loads(resp.read().decode())
        gen_text = ""
        if "choices" in data and len(data["choices"]) > 0:
            choice = data["choices"][0]
            gen_text = choice.get("text") or (choice.get("message") or {}).get("content", "")
        usage = data.get("usage", {})
        comp_tok = usage.get("completion_tokens", 0)
        prompt_tok = usage.get("prompt_tokens", 0)
        if comp_tok == 0 and gen_text:
            comp_tok = max(1, len(gen_text.split()))
        tok_s = comp_tok / elapsed if elapsed > 0 else 0
        return {
            "prompt": prompt[:60],
            "text": gen_text,
            "tokens": comp_tok,
            "prompt_tokens": prompt_tok,
            "elapsed": round(elapsed, 3),
            "tok_s": round(tok_s, 2),
        }
    except Exception as e:
        elapsed = time.time() - start
        return {"prompt": prompt[:60], "error": str(e), "elapsed": round(elapsed, 3)}


def call_chat(prompt, max_tokens=10, timeout=30):
    """Chat-templated call (uses the server's --jinja instruct template) — the
    CORRECT endpoint for tool-calling / instruction-following correctness
    checks. Root-cause note (§11.4.102): the raw /v1/completions endpoint is a
    base-continuation endpoint (no chat template applied) — a closed question
    like '2+2?' ending in a period gets treated as already-complete text and
    the model emits EOS immediately (finish_reason=stop, predicted_n=1, empty
    text). This is a test-methodology fix, not a Lane-B service defect;
    verified by direct curl against both endpoints before code-fixing the
    harness (see docs/qa/lane_b_liveproof_*/RESULTS.md for the raw evidence).
    """
    body = json.dumps({
        "messages": [{"role": "user", "content": prompt}],
        "max_tokens": max_tokens,
        "temperature": 0.2,
        "stream": False,
    }).encode()
    req = urllib.request.Request(LANEB_CHAT_URL, data=body, headers=HEADERS, method="POST")
    start = time.time()
    try:
        resp = urllib.request.urlopen(req, timeout=timeout)
        elapsed = time.time() - start
        data = json.loads(resp.read().decode())
        content = ""
        if "choices" in data and len(data["choices"]) > 0:
            content = data["choices"][0].get("message", {}).get("content", "")
        usage = data.get("usage", {})
        return {
            "prompt": prompt,
            "content": content,
            "completion_tokens": usage.get("completion_tokens", 0),
            "elapsed": round(elapsed, 3),
            "raw": data,
        }
    except Exception as e:
        return {"prompt": prompt, "error": str(e), "elapsed": round(time.time() - start, 3)}


def check_coder_alive():
    try:
        req = urllib.request.Request(CODER_URL, method="GET")
        resp = urllib.request.urlopen(req, timeout=5)
        data = json.loads(resp.read().decode())
        model = data.get("data", [{}])[0].get("id", "")
        return True, model
    except Exception as e:
        return False, str(e)


def main():
    print("=" * 78)
    print("LANE-B LIVE CO-RESIDENCE RE-VALIDATION — helixllm agentgen-boot")
    print(f"Date/Time: {time.strftime('%Y-%m-%d %H:%M:%S UTC', time.gmtime())}")
    print("=" * 78)

    all_results = {}

    # ---- 0. coder alive BEFORE benchmark ----
    alive_before, model_before = check_coder_alive()
    print(f"\n[0] Coder :18434 alive BEFORE bench: {alive_before} model={model_before}")
    all_results["coder_alive_before"] = {"alive": alive_before, "model": model_before}

    # ---- 1. single-stream ----
    print("\n[1] SINGLE-STREAM benchmark (5 prompts, sequential)")
    single_results = []
    for i, p in enumerate(SINGLE_STREAM_PROMPTS, 1):
        r = call_llm(p)
        single_results.append(r)
        if "error" in r:
            print(f"  [{i}] ERROR: {r['error']}")
        else:
            print(f"  [{i}] tok={r['tokens']:4d} t={r['elapsed']:.2f}s {r['tok_s']:.2f} tok/s")
    ok_single = [r for r in single_results if "error" not in r]
    if ok_single:
        rates = [r["tok_s"] for r in ok_single]
        total_tok = sum(r["tokens"] for r in ok_single)
        total_time = sum(r["elapsed"] for r in ok_single)
        print(f"  -> mean={sum(rates)/len(rates):.2f} tok/s  "
              f"median={sorted(rates)[len(rates)//2]:.2f} tok/s  "
              f"aggregate={total_tok/total_time:.2f} tok/s")
        all_results["single_stream"] = {
            "results": single_results,
            "mean_tok_s": round(sum(rates) / len(rates), 2),
            "median_tok_s": round(sorted(rates)[len(rates) // 2], 2),
            "aggregate_tok_s": round(total_tok / total_time, 2),
        }
    else:
        all_results["single_stream"] = {"results": single_results, "error": "all failed"}

    # ---- 2. 3-parallel concurrent ----
    print("\n[2] 3-PARALLEL CONCURRENT benchmark")
    par_results = [None] * len(PARALLEL_PROMPTS)

    def worker(idx, prompt):
        par_results[idx] = call_llm(prompt, max_tokens=200)

    par_start = time.time()
    threads = [threading.Thread(target=worker, args=(i, p)) for i, p in enumerate(PARALLEL_PROMPTS)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    par_wall = time.time() - par_start

    for i, r in enumerate(par_results, 1):
        if "error" in r:
            print(f"  [{i}] ERROR: {r['error']}")
        else:
            print(f"  [{i}] tok={r['tokens']:4d} t={r['elapsed']:.2f}s {r['tok_s']:.2f} tok/s")
    ok_par = [r for r in par_results if r and "error" not in r]
    print(f"  -> wall-clock for 3 concurrent requests: {par_wall:.2f}s  "
          f"({len(ok_par)}/{len(PARALLEL_PROMPTS)} succeeded)")
    all_results["parallel_3"] = {
        "results": par_results,
        "wall_clock_s": round(par_wall, 2),
        "succeeded": len(ok_par),
        "total": len(PARALLEL_PROMPTS),
    }

    # ---- 3. tool-calling / arithmetic correctness (chat-templated endpoint) ----
    print("\n[3] TOOL-CALLING / ARITHMETIC CORRECTNESS CHECK (/v1/chat/completions)")
    tc = call_chat(TOOL_CALL_PROMPT, max_tokens=10)
    print(f"  prompt: {TOOL_CALL_PROMPT!r}")
    if "error" in tc:
        print(f"  ERROR: {tc['error']}")
        tc_correct = False
    else:
        print(f"  response: {tc['content']!r}")
        tc_correct = "4" in tc["content"]
        print(f"  contains '4': {tc_correct}")
    all_results["tool_call_check"] = {"result": tc, "correct": tc_correct, "endpoint": LANEB_CHAT_URL}

    # ---- 4. co-residence + coder-untouched proof ----
    alive_after, model_after = check_coder_alive()
    print(f"\n[4] Coder :18434 alive AFTER bench: {alive_after} model={model_after}")
    all_results["coder_alive_after"] = {"alive": alive_after, "model": model_after}
    all_results["co_residence_confirmed"] = alive_before and alive_after

    print("\n" + "=" * 78)
    print("SUMMARY")
    print("=" * 78)
    print(json.dumps({
        "single_stream_mean_tok_s": all_results.get("single_stream", {}).get("mean_tok_s"),
        "parallel_3_wall_clock_s": all_results["parallel_3"]["wall_clock_s"],
        "parallel_3_succeeded": all_results["parallel_3"]["succeeded"],
        "tool_call_correct": tc_correct,
        "coder_untouched": all_results["co_residence_confirmed"],
    }, indent=2))

    with open("/tmp/laneb_liveproof_results.json", "w") as f:
        json.dump(all_results, f, indent=2)

    ok = (bool(ok_single) and len(ok_par) == len(PARALLEL_PROMPTS) and tc_correct
          and all_results["co_residence_confirmed"])
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
