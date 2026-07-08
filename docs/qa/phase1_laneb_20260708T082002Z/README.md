# Phase 1 — Lane-B benchmark spike (Task 2.1) — session 2 continuation — BLOCKED on download (honest, real evidence)

Continuation of `docs/qa/phase1_laneb_20260708T080428Z/` (previous session stopped
the download intentionally at a hard bound). This session resumed the SAME
partial file and let the download run **in the background** for the duration
of this session per the task's explicit instruction ("Long-running/background
is expected — downloads are ~1-2 MB/s so budget hours, but keep making
progress"). No benchmark numbers are fabricated — the model file did not
finish downloading within this session's wall-clock budget, so Lane-B was
never booted and no tok/s figures exist yet.

## Pre-flight — live nvidia-smi, coder confirmed untouched (baseline)

```
32607 MiB total, 19444 MiB used, 12677 MiB free   (coder = llama-server pid 416519, 19434 MiB)
curl :18434/v1/models -> Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf (coder identity confirmed)
```

## Harness build — real, reproducible (§11.4.108 SOURCE→ARTIFACT)

```
cd submodules/helix_llm
go build -o /dev/null ./cmd/agentgen-boot   -> exit 0 (BUILD OK)
go build -o /dev/null ./cmd/imagegen-boot   -> exit 0 (BUILD OK)  (used for Part B, see sibling dir)
```

helix_llm HEAD at session start: `c92fb16` (feat(serving): agentgen-boot Lane-B
harness, Task 2.1 spike, ClassAgent). No source changes were needed or made —
the harness built cleanly as-is.

## Download — real, resumed, verified resumable, background, honestly incomplete

Target file: `bartowski/Mistral-Nemo-Instruct-2407-GGUF` ->
`Mistral-Nemo-Instruct-2407-Q4_K_M.gguf`, confirmed via `curl -sI -L`:

```
HTTP/2 302 (redirect to signed CDN URL) -> HTTP 200, accept-ranges: bytes
x-linked-size: 7477208192   (== 6.965 GiB, matches harness's documented ~6.96 GiB)
```

Partial file at session start (inherited from prior session's stopped attempt):
`~/models/Mistral-Nemo-Instruct-2407-Q4_K_M.gguf.part` = 637,022,424 bytes (8.52%).

Resumed via `curl -sS -L -C - -o ...gguf.part <url>`, launched with
`nohup ... & disown` — verified truly detached (`ps -o pid,ppid` shows
**PPID=1**, i.e. reparented to init, survives this session/shell ending):

```
PID 1297065, PPID 1, STAT S, real background process
```

Three real, spaced size samples taken during this session (not extrapolated):

| unix time | bytes | delta bytes | delta t (s) | instantaneous rate |
|---|---|---|---|---|
| 1783498838 | 780,656,856 | — | — | — |
| 1783498863 | 843,538,648 | 62,881,792 | 25 | ~2.52 MB/s |
| 1783498886 | 886,431,960 | 42,893,312 | 23 | ~1.86 MB/s |

Sustained rate this session: **~2.0-2.5 MB/s** (network-bound, matches the
task's "~1-2 MB/s" estimate, slightly better). Remaining bytes at last sample:
7,477,208,192 - 886,431,960 = 6,590,776,232 (≈ 6.14 GiB). At the measured rate,
**estimated ETA ≈ 45-55 minutes** from the last sample — i.e. this download
will most likely complete within the hour if left running, but did NOT
complete inside this session's active work window.

**The curl process (PID 1297065) was left running, detached, in the
background** so it continues past this session's end per §11.4.89 /
§11.4.126. It writes to `~/models/Mistral-Nemo-Instruct-2407-Q4_K_M.gguf.part`
and will need a final `mv ...gguf.part ...gguf` once `ls -l` shows it reached
7,477,208,192 bytes.

## Verdict: BLOCKED-on-download (honest, no fabrication)

The Lane-B benchmark (single-stream tok/s, concurrent tok/s, tool-calling /
structured-output correctness sample, concurrent-with-coder co-residence
proof) was **NOT run** this session because the GGUF artifact was still
incomplete at session end. No numbers below are invented:

- tok/s single: NOT MEASURED (blocked)
- tok/s concurrent: NOT MEASURED (blocked)
- tool-calling correctness: NOT MEASURED (blocked)
- concurrent coder completion co-residence: NOT MEASURED (blocked)
- GO/NO-GO recommendation: **DEFERRED** — cannot be honestly issued without
  the above measurements (§11.4.6/§11.4.123 — no guessing).

## Coder untouched — confirmed post-session (§11.4.122)

```
curl :18434/health -> {"status":"ok"}
nvidia-smi: 32607 MiB total, 19444 MiB used, 12677 MiB free   <- IDENTICAL to pre-flight baseline
```

No Lane-B container was ever booted this session (download-only + admit-check
only — no `compose up` invoked against cmd/agentgen-boot's compose file), so
§11.4.119 single-resource-owner is trivially satisfied.

## Resume instructions (next session / conductor)

1. Check `stat -c '%s' ~/models/Mistral-Nemo-Instruct-2407-Q4_K_M.gguf.part`
   against 7,477,208,192. If equal (or `ls` shows no `.part` suffix because a
   prior run already renamed it), the download is complete.
2. If complete: `mv` off the `.part` suffix if still present, then:
   ```
   nvidia-smi --query-gpu=memory.free --format=csv,noheader   # re-verify live free VRAM first (§11.4.111)
   cd submodules/helix_llm
   go run ./cmd/agentgen-boot admit-check
   go run ./cmd/agentgen-boot boot cmd/agentgen-boot/compose.agent.yml laneb-spike
   # benchmark against http://localhost:18435 (single-stream, concurrent, tool-calling)
   # concurrently issue a real coder completion against :18434 to prove co-residence
   go run ./cmd/agentgen-boot down cmd/agentgen-boot/compose.agent.yml laneb-spike
   ```
3. If still incomplete, `curl -C -` will resume from the current byte offset;
   the file is untouched/uncorrupted (curl range-resume verified against
   `accept-ranges: bytes` above).
