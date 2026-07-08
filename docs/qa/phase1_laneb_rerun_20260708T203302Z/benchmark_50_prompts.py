#!/usr/bin/env python3
"""Benchmark Lane-B Mistral-Nemo-12B with 50 varied prompts."""
import json
import time
import urllib.request
import urllib.error
import sys

URL = "http://localhost:18435/v1/completions"
HEADERS = {"Content-Type": "application/json"}

PROMPTS = [
    # 1-5: Coding
    "Write a Python function to merge two sorted lists into one sorted list.",
    "Explain the difference between HTTP PUT and PATCH methods with examples.",
    "Write a regex to validate email addresses in Python.",
    "What is the time complexity of quicksort? Explain with code.",
    "Write a Go function to reverse a linked list in place.",
    # 6-10: Math & Reasoning
    "If f(x) = 3x^2 + 2x - 5, what is f(4)? Show your work.",
    "Solve for x: 2^(x+1) = 32. Explain each step.",
    "A train leaves Station A at 60 km/h. Another leaves Station B at 90 km/h, 100 km apart. When and where do they meet?",
    "What is the sum of all integers from 1 to 100? Show the formula.",
    "How many ways can you arrange the letters in the word 'BANANA'?",
    # 11-15: Science & Technology
    "Explain how quantum computing differs from classical computing in simple terms.",
    "What is the difference between TCP and UDP protocols?",
    "How does a hash map work internally? Explain with examples.",
    "Describe the CAP theorem in distributed systems.",
    "What is the difference between Docker and a virtual machine?",
    # 16-20: Creative Writing
    "Write a short poem about artificial intelligence in 4 lines.",
    "Describe a world where computers never existed in 100 words.",
    "Write a haiku about debugging code.",
    "Create a short dialog between two AI assistants discussing their purpose.",
    "Write a brief story about a programmer who travels back in time to 1980.",
    # 21-25: Code Review & Debugging
    "What is wrong with this code? for i in range(len(arr)): arr.remove(arr[i])",
    "Explain why this causes a deadlock: thread1 locks A then B; thread2 locks B then A.",
    "Fix this SQL injection vulnerable query: \"SELECT * FROM users WHERE name = '\" + name + \"'\"",
    "Why does this Go program panic? var m map[string]int; m['key'] = 42",
    "Optimize this: def sum_list(lst): s=0; for i in lst: s+=i; return s",
    # 26-30: General Knowledge
    "What is the meaning of 'YOLO' in computing contexts?",
    "Explain the concept of 'technical debt' to a non-technical manager.",
    "What is the origin of the term 'bug' in software engineering?",
    "What is the difference between REST and GraphQL?",
    "What does 'idempotent' mean in the context of HTTP methods?",
    # 31-35: Code Generation
    "Write a bash script to find the 10 largest files in a directory tree.",
    "Write a Python class for a simple bank account with deposit/withdraw/balance.",
    "Write a SQL query to find employees earning more than their department average.",
    "Write a JavaScript function to debounce a callback with a delay.",
    "Write a Dockerfile for a Python Flask app with minimal size.",
    # 36-40: System Design
    "How would you design a URL shortening service like TinyURL?",
    "Explain pub/sub messaging pattern in 3 sentences.",
    "What is the difference between vertical and horizontal scaling?",
    "How would you design a rate limiter for an API?",
    "Explain what a CDN does and when you would use one.",
    # 41-45: Security
    "What is XSS? How do you prevent it?",
    "Explain the difference between authentication and authorization.",
    "What is a CSRF attack and how do you defend against it?",
    "How does HTTPS/SSL work in simple terms?",
    "What is the principle of least privilege?",
    # 46-50: Mixed
    "What is a monad in functional programming? Explain simply.",
    "Explain the concept of 'event loop' in JavaScript/Node.js.",
    "What is the difference between SQL and NoSQL databases? When to use each?",
    "Explain how a blockchain works at a high level.",
    "What are the SOLID principles? List and briefly explain each.",
]

def call_llm(prompt, max_tokens=256):
    body = json.dumps({
        "prompt": prompt,
        "max_tokens": max_tokens,
        "temperature": 0.7,
        "stop": [],
        "stream": False,
    }).encode()
    req = urllib.request.Request(URL, data=body, headers=HEADERS, method="POST")
    start = time.time()
    try:
        resp = urllib.request.urlopen(req, timeout=120)
        elapsed = time.time() - start
        data = json.loads(resp.read().decode())
        # Try multiple response formats
        gen_text = ""
        if "choices" in data and len(data["choices"]) > 0:
            choice = data["choices"][0]
            if "text" in choice:
                gen_text = choice["text"]
            elif "message" in choice and "content" in choice["message"]:
                gen_text = choice["message"]["content"]
        usage = data.get("usage", {})
        prompt_tok = usage.get("prompt_tokens", 0) or usage.get("prompt_tokens", 0)
        comp_tok = usage.get("completion_tokens", 0) or usage.get("completion_tokens", 0)
        # If no token counts, estimate from text
        if comp_tok == 0 and gen_text:
            comp_tok = max(1, len(gen_text.split()))
        if prompt_tok == 0:
            prompt_tok = max(1, len(prompt.split()))
        tok_s = comp_tok / elapsed if elapsed > 0 else 0
        return {
            "tokens": comp_tok,
            "elapsed": elapsed,
            "tok_s": round(tok_s, 2),
            "text_len": len(gen_text),
            "prompt_tokens": prompt_tok,
        }
    except Exception as e:
        elapsed = time.time() - start
        return {"error": str(e), "elapsed": elapsed}

def main():
    print("=" * 72)
    print("LANE-B MISTRAL-NEMO-12B BENCHMARK — 50 PROMPTS")
    print("=" * 72)
    print(f"Endpoint: {URL}")
    print(f"Date/Time: {time.strftime('%Y-%m-%d %H:%M:%S')}")
    print()

    results = []
    for i, prompt in enumerate(PROMPTS, 1):
        print(f"[{i:02d}/50] {prompt[:60]}...", end=" ", flush=True)
        r = call_llm(prompt)
        results.append(r)
        if "error" in r:
            print(f"ERROR ({r['elapsed']:.1f}s): {r['error']}")
        else:
            print(f"OK  tok={r['tokens']}  t={r['elapsed']:.1f}s  {r['tok_s']:.1f} tok/s")
        time.sleep(0.5)  # Brief cooldown between requests

    print()
    print("=" * 72)
    print("SUMMARY")
    print("=" * 72)

    ok_results = [r for r in results if "error" not in r]
    err_results = [r for r in results if "error" in r]

    if ok_results:
        tok_rates = [r["tok_s"] for r in ok_results]
        tokens = [r["tokens"] for r in ok_results]
        times = [r["elapsed"] for r in ok_results]
        total_tokens = sum(tokens)
        total_time = sum(times)
        overall_tok_s = total_tokens / total_time if total_time > 0 else 0

        print(f"Successful requests: {len(ok_results)}/{len(results)}")
        print(f"Errors:             {len(err_results)}")
        print(f"Total tokens gen:   {total_tokens}")
        print(f"Total time:         {total_time:.1f}s")
        print()
        print(f"Per-request token rate:")
        print(f"  Mean:    {sum(tok_rates)/len(tok_rates):.2f} tok/s")
        print(f"  Median:  {sorted(tok_rates)[len(tok_rates)//2]:.2f} tok/s")
        print(f"  Min:     {min(tok_rates):.2f} tok/s")
        print(f"  Max:     {max(tok_rates):.2f} tok/s")
        print(f"  StdDev:  {(sum((x - sum(tok_rates)/len(tok_rates))**2 for x in tok_rates)/len(tok_rates))**0.5:.2f} tok/s")
        print()
        print(f"Overall (aggregate): {overall_tok_s:.2f} tok/s")
        print(f"Mean prompt latency:  {sum(times)/len(times):.2f}s")
        print(f"Mean tokens/request:  {sum(tokens)/len(tokens):.0f}")
        print()
        print("Per-prompt detail:")
        print(f"{'#':>3} {'tok':>5} {'time':>6} {'tok/s':>7}")
        for i, r in enumerate(ok_results, 1):
            print(f"{i:3d} {r['tokens']:5d} {r['elapsed']:6.1f}s {r['tok_s']:7.2f}")
    else:
        print("ALL REQUESTS FAILED")

    # Save raw results
    with open("/tmp/laneb_benchmark_results.json", "w") as f:
        json.dump({"results": results, "summary": {
            "total": len(results),
            "ok": len(ok_results),
            "errors": len(err_results),
            "mean_tok_s": round(sum(r["tok_s"] for r in ok_results) / len(ok_results), 2) if ok_results else 0,
            "overall_tok_s": round(overall_tok_s, 2) if ok_results else 0,
            "total_tokens": total_tokens if ok_results else 0,
            "total_time": round(total_time, 1) if ok_results else 0,
        }}, f)

    return 1 if len(err_results) > len(ok_results) else 0

if __name__ == "__main__":
    sys.exit(main())
