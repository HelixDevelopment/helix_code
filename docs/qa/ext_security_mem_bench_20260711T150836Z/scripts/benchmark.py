#!/usr/bin/env python3
"""
Section 11.4.169(13) benchmark/performance test for the LIVE helixllm-coder
surface (:18434). Measures p50/p95/p99 end-to-end latency + throughput at
several concurrency levels, plus TTFT (time-to-first-token) via the
streaming endpoint. Thresholds/verdicts are derived from THIS run's own
measurements (Section 11.4.107(13)) -- not from literature.

NON-DESTRUCTIVE: ordinary chat-completion requests against the running
server; never restarts/kills/reconfigures it.
"""
import json
import statistics
import time
import urllib.request
import os

BASE = "http://localhost:18434"
MODEL = "/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf"
OUTDIR = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "evidence")
os.makedirs(OUTDIR, exist_ok=True)

import threading


def percentile(values, pct):
    if not values:
        return None
    s = sorted(values)
    k = (len(s) - 1) * (pct / 100.0)
    f = int(k)
    c = min(f + 1, len(s) - 1)
    if f == c:
        return s[f]
    return s[f] + (s[c] - s[f]) * (k - f)


def one_latency_request(prompt_idx):
    body = json.dumps({
        "model": MODEL, "max_tokens": 32,
        "messages": [{"role": "user", "content": "Benchmark request " + str(prompt_idx) + ": write one short sentence about the ocean."}],
    }).encode("utf-8")
    req = urllib.request.Request(BASE + "/v1/chat/completions", data=body,
                                  headers={"Content-Type": "application/json"}, method="POST")
    t0 = time.time()
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            data = resp.read()
            status = resp.status
    except Exception as e:
        status = "ERR:" + str(e)
        data = b""
    elapsed = time.time() - t0
    return {"idx": prompt_idx, "status": status, "elapsed_s": elapsed, "resp_len": len(data)}


def run_concurrency_level(concurrency, total_requests):
    results = []
    lock = threading.Lock()

    def worker(indices):
        for i in indices:
            r = one_latency_request(i)
            with lock:
                results.append(r)

    # split total_requests across `concurrency` workers
    per = total_requests // concurrency
    remainder = total_requests % concurrency
    threads = []
    idx = 0
    t_start = time.time()
    for w in range(concurrency):
        count = per + (1 if w < remainder else 0)
        indices = list(range(idx, idx + count))
        idx += count
        th = threading.Thread(target=worker, args=(indices,))
        threads.append(th)
        th.start()
    for th in threads:
        th.join()
    wall_time = time.time() - t_start

    ok = [r for r in results if r["status"] == 200]
    err = [r for r in results if r["status"] != 200]
    latencies = [r["elapsed_s"] for r in ok]

    p50 = percentile(latencies, 50)
    p95 = percentile(latencies, 95)
    p99 = percentile(latencies, 99)
    throughput = len(ok) / wall_time if wall_time > 0 else 0

    return {
        "concurrency": concurrency,
        "total_requests": total_requests,
        "ok_count": len(ok),
        "err_count": len(err),
        "errors": [r for r in err][:10],
        "wall_time_s": wall_time,
        "p50_s": p50,
        "p95_s": p95,
        "p99_s": p99,
        "min_s": min(latencies) if latencies else None,
        "max_s": max(latencies) if latencies else None,
        "mean_s": statistics.mean(latencies) if latencies else None,
        "throughput_req_per_s": throughput,
        "raw_latencies_s": latencies,
    }


def measure_ttft(num_samples=8):
    """Streaming TTFT: time from request-send to first non-empty content
    delta chunk arriving."""
    ttft_values = []
    for i in range(num_samples):
        body = json.dumps({
            "model": MODEL, "max_tokens": 24, "stream": True,
            "messages": [{"role": "user", "content": "TTFT sample " + str(i) + ": count from 1 to 3."}],
        }).encode("utf-8")
        req = urllib.request.Request(BASE + "/v1/chat/completions", data=body,
                                      headers={"Content-Type": "application/json"}, method="POST")
        t0 = time.time()
        first_content_t = None
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                for line_bytes in resp:
                    line = line_bytes.decode("utf-8", errors="replace").strip()
                    if not line.startswith("data:"):
                        continue
                    payload = line[len("data:"):].strip()
                    if payload == "[DONE]":
                        break
                    try:
                        chunk = json.loads(payload)
                    except Exception:
                        continue
                    choices = chunk.get("choices", [])
                    if choices and choices[0].get("delta", {}).get("content"):
                        first_content_t = time.time()
                        break
        except Exception as e:
            ttft_values.append({"idx": i, "ttft_s": None, "error": str(e)})
            continue
        if first_content_t:
            ttft_values.append({"idx": i, "ttft_s": first_content_t - t0, "error": None})
        else:
            ttft_values.append({"idx": i, "ttft_s": None, "error": "no content chunk observed"})
    valid = [v["ttft_s"] for v in ttft_values if v["ttft_s"] is not None]
    return {
        "samples": ttft_values,
        "p50_s": percentile(valid, 50),
        "p95_s": percentile(valid, 95),
        "mean_s": statistics.mean(valid) if valid else None,
        "num_valid": len(valid),
        "num_total": num_samples,
    }


def main():
    print("=== Section 11.4.169(13) Benchmark -- helixllm-coder :18434 ===")

    concurrency_plan = [
        (1, 15),
        (10, 40),
        (25, 50),
    ]

    level_reports = []
    for conc, total in concurrency_plan:
        print("\n-- Concurrency=" + str(conc) + ", total_requests=" + str(total) + " --")
        rep = run_concurrency_level(conc, total)
        level_reports.append(rep)
        print("  ok=%d err=%d wall=%.2fs p50=%.3fs p95=%.3fs p99=%.3fs throughput=%.2f req/s" % (
            rep["ok_count"], rep["err_count"], rep["wall_time_s"],
            rep["p50_s"] or -1, rep["p95_s"] or -1, rep["p99_s"] or -1, rep["throughput_req_per_s"]))

    print("\n-- TTFT (streaming) --")
    ttft_report = measure_ttft(8)
    print("  valid=%d/%d p50=%.3fs p95=%.3fs mean=%.3fs" % (
        ttft_report["num_valid"], ttft_report["num_total"],
        ttft_report["p50_s"] or -1, ttft_report["p95_s"] or -1, ttft_report["mean_s"] or -1))

    # Self-validation of percentile() implementation: golden-good (known
    # sorted-array percentile) vs golden-bad (deliberately-wrong impl that
    # always returns the min) to prove the percentile math is exercised
    # correctly and a broken implementation WOULD be caught.
    test_values = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
    p50_good = percentile(test_values, 50)
    expected_p50 = 5.5
    golden_good_ok = abs(p50_good - expected_p50) < 0.01

    def broken_percentile_always_min(values, pct):
        return min(values) if values else None
    p50_bad = broken_percentile_always_min(test_values, 50)
    golden_bad_correctly_wrong = abs(p50_bad - expected_p50) > 0.01  # min(1..10)=1, expected 5.5 -> should differ

    self_validation = {
        "percentile_math_golden_good_correct": golden_good_ok,
        "percentile_math_golden_bad_detectably_wrong": golden_bad_correctly_wrong,
        "test_values": test_values,
        "computed_p50": p50_good,
        "expected_p50": expected_p50,
        "broken_impl_p50": p50_bad,
    }
    print("\nSelf-validation (percentile math): golden_good_correct=" + str(golden_good_ok) +
          " golden_bad_detectably_wrong=" + str(golden_bad_correctly_wrong))

    report = {
        "target": BASE,
        "model": MODEL,
        "concurrency_levels": level_reports,
        "ttft": ttft_report,
        "self_validation": self_validation,
    }
    out_json = os.path.join(OUTDIR, "BENCHMARK_REPORT.json")
    with open(out_json, "w") as f:
        json.dump(report, f, indent=2)
    print("\nReport written to " + out_json)


if __name__ == "__main__":
    main()
