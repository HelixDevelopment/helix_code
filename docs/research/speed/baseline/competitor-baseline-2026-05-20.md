<!--
Document-Metadata (constitution §11.4.44)
Revision: 1
Generated: 20260520T133654Z
Authority: HelixCode speed programme — P0-T03 competitor wall-clock baseline.
           Produced by scripts/testing/competitor_speed_baseline.sh.
Scope:     Captured measurement artefact. Numbers are real wall-clock from
           the host below; absent agents are SKIP-OK, never fabricated
           (CONST-035 anti-bluff / §11.4.6 no-guessing).
-->

# Competitor Wall-Clock Baseline — 20260520T133654Z

Speed-programme Phase 0 task **P0-T03**. Establishes the competitor
numbers HelixCode must beat (R4 §3). Captured by
`scripts/testing/competitor_speed_baseline.sh`.

| Field | Value |
|---|---|
| Host | `Linux 6.12.61-6.12-alt1 x86_64` |
| Generated (UTC) | 20260520T133654Z |
| Runs per scenario | 3 |
| Timing tool | `/usr/bin/time` |

## Measured wall-clock

| Agent | Command | Scenario | Median | Detail |
|---|---|---|---|---|
| Claude Code | `claude` | S1 cold-start | 111 ms | min 102 / mean 110 / max 117 ms (n=3) |
| Gemini CLI | `gemini` | S1 cold-start | 2015 ms | min 1920 / mean 1988 / max 2031 ms (n=3) |
| Aider | `aider` | S1 cold-start | 645 ms | min 624 / mean 651 / max 685 ms (n=3) |
| Cline | `cline` | S1 cold-start | SKIP-OK | not installed on host |
| Crush | `crush` | S1 cold-start | 128 ms | min 117 / mean 127 / max 136 ms (n=3) |
| OpenCode | `opencode` | S1 cold-start | 1025 ms | min 970 / mean 1018 / max 1060 ms (n=3) |
| Qwen Code | `qwen` | S1 cold-start | 919 ms | min 887 / mean 916 / max 943 ms (n=3) |

S1 cold-start = `<agent> --version` wall-clock — a safe, no-LLM-cost,
repeatable measurement. `SKIP-OK` rows are agents not installed on this
host; per CONST-035 / §11.4.6 no number is fabricated for them.

**S2-S4 (LLM-invoking):** skipped — they would cost real API tokens.
Set `COMPETITOR_BENCH_LLM=1` to opt in. The S1 cold-start figures above are
the default no-cost baseline (CONST-050: no fake LLM, no fabricated number).

## Raw `time` evidence (anti-bluff — independently checkable)

### Claude Code (`claude`) — S1 cold-start
binary: /home/milosvasic/.local/bin/claude
samples (ms): 111 117 102 
median: 111 ms | mean: 110 ms | min: 102 ms | max: 117 ms
```
run 1: wall=111ms rc=0
    2.1.143 (Claude Code)
    	Command being timed: "claude --version"
    	User time (seconds): 0.06
    	System time (seconds): 0.04
    	Percent of CPU this job got: 102%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.10
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 203280
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 18757
    	Voluntary context switches: 75
    	Involuntary context switches: 42
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
run 2: wall=117ms rc=0
    2.1.143 (Claude Code)
    	Command being timed: "claude --version"
    	User time (seconds): 0.07
    	System time (seconds): 0.04
    	Percent of CPU this job got: 103%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.11
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 202640
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 18551
    	Voluntary context switches: 99
    	Involuntary context switches: 164
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
run 3: wall=102ms rc=0
    2.1.143 (Claude Code)
    	Command being timed: "claude --version"
    	User time (seconds): 0.06
    	System time (seconds): 0.03
    	Percent of CPU this job got: 104%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.09
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 203252
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 18760
    	Voluntary context switches: 83
    	Involuntary context switches: 107
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 8
    	Socket messages sent: 0
```

### Gemini CLI (`gemini`) — S1 cold-start
binary: /home/milosvasic/.npm-global/bin/gemini
samples (ms): 2031 2015 1920 
median: 2015 ms | mean: 1988 ms | min: 1920 ms | max: 2031 ms
```
run 1: wall=2031ms rc=0
    0.33.2
    	Command being timed: "gemini --version"
    	User time (seconds): 2.38
    	System time (seconds): 0.28
    	Percent of CPU this job got: 131%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:02.02
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 234212
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 58960
    	Voluntary context switches: 673
    	Involuntary context switches: 299
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 96
    	Socket messages sent: 0
run 2: wall=2015ms rc=0
    0.33.2
    	Command being timed: "gemini --version"
    	User time (seconds): 2.40
    	System time (seconds): 0.27
    	Percent of CPU this job got: 132%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:02.01
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 232532
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 62428
    	Voluntary context switches: 609
    	Involuntary context switches: 144
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
run 3: wall=1920ms rc=0
    0.33.2
    	Command being timed: "gemini --version"
    	User time (seconds): 2.31
    	System time (seconds): 0.25
    	Percent of CPU this job got: 134%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:01.91
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 232264
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 59879
    	Voluntary context switches: 707
    	Involuntary context switches: 117
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 8
    	Socket messages sent: 0
```

### Aider (`aider`) — S1 cold-start
binary: /home/milosvasic/.local/bin/aider
samples (ms): 624 645 685 
median: 645 ms | mean: 651 ms | min: 624 ms | max: 685 ms
```
run 1: wall=624ms rc=0
    aider 0.86.2
    	Command being timed: "aider --version"
    	User time (seconds): 0.57
    	System time (seconds): 0.04
    	Percent of CPU this job got: 100%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.62
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 73100
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 15165
    	Voluntary context switches: 9
    	Involuntary context switches: 10
    	Swaps: 0
    	File system inputs: 256
    	File system outputs: 0
    	Socket messages sent: 0
run 2: wall=645ms rc=0
    aider 0.86.2
    	Command being timed: "aider --version"
    	User time (seconds): 0.59
    	System time (seconds): 0.04
    	Percent of CPU this job got: 100%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.64
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 73032
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 15164
    	Voluntary context switches: 9
    	Involuntary context switches: 8
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
run 3: wall=685ms rc=0
    aider 0.86.2
    	Command being timed: "aider --version"
    	User time (seconds): 0.63
    	System time (seconds): 0.04
    	Percent of CPU this job got: 99%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.68
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 73116
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 15171
    	Voluntary context switches: 9
    	Involuntary context switches: 8
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
```

### Cline (`cline`) — SKIP-OK
CONST-§11.4.3 topology-dispatch: `cline` not found on PATH; no number fabricated.

### Crush (`crush`) — S1 cold-start
binary: /home/milosvasic/.npm-global/bin/crush
samples (ms): 136 117 128 
median: 128 ms | mean: 127 ms | min: 117 ms | max: 136 ms
```
run 1: wall=136ms rc=0
    crush version v0.56.0
    	Command being timed: "crush --version"
    	User time (seconds): 0.14
    	System time (seconds): 0.03
    	Percent of CPU this job got: 134%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.13
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 75104
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 11324
    	Voluntary context switches: 301
    	Involuntary context switches: 19
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
run 2: wall=117ms rc=0
    crush version v0.56.0
    	Command being timed: "crush --version"
    	User time (seconds): 0.12
    	System time (seconds): 0.03
    	Percent of CPU this job got: 140%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.11
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 73728
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 11543
    	Voluntary context switches: 282
    	Involuntary context switches: 7
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
run 3: wall=128ms rc=0
    crush version v0.56.0
    	Command being timed: "crush --version"
    	User time (seconds): 0.13
    	System time (seconds): 0.03
    	Percent of CPU this job got: 137%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.12
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 72540
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 11430
    	Voluntary context switches: 269
    	Involuntary context switches: 10
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
```

### OpenCode (`opencode`) — S1 cold-start
binary: /home/milosvasic/.opencode/bin/opencode
samples (ms): 1060 970 1025 
median: 1025 ms | mean: 1018 ms | min: 970 ms | max: 1060 ms
```
run 1: wall=1060ms rc=0
    1.14.41
    	Command being timed: "opencode --version"
    	User time (seconds): 1.13
    	System time (seconds): 0.18
    	Percent of CPU this job got: 125%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:01.05
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 271512
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 54936
    	Voluntary context switches: 1648
    	Involuntary context switches: 883
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
run 2: wall=970ms rc=0
    1.14.41
    	Command being timed: "opencode --version"
    	User time (seconds): 1.09
    	System time (seconds): 0.13
    	Percent of CPU this job got: 127%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.96
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 272056
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 54384
    	Voluntary context switches: 1918
    	Involuntary context switches: 1091
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
run 3: wall=1025ms rc=0
    1.14.41
    	Command being timed: "opencode --version"
    	User time (seconds): 1.14
    	System time (seconds): 0.14
    	Percent of CPU this job got: 126%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:01.02
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 275816
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 55452
    	Voluntary context switches: 1722
    	Involuntary context switches: 788
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
```

### Qwen Code (`qwen`) — S1 cold-start
binary: /home/milosvasic/.npm-global/bin/qwen
samples (ms): 887 919 943 
median: 919 ms | mean: 916 ms | min: 887 ms | max: 943 ms
```
run 1: wall=887ms rc=0
    0.14.5
    	Command being timed: "qwen --version"
    	User time (seconds): 0.89
    	System time (seconds): 0.11
    	Percent of CPU this job got: 114%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.88
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 218432
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 55465
    	Voluntary context switches: 305
    	Involuntary context switches: 27
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
run 2: wall=919ms rc=0
    0.14.5
    	Command being timed: "qwen --version"
    	User time (seconds): 0.93
    	System time (seconds): 0.13
    	Percent of CPU this job got: 116%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.91
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 227436
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 57994
    	Voluntary context switches: 296
    	Involuntary context switches: 67
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
run 3: wall=943ms rc=0
    0.14.5
    	Command being timed: "qwen --version"
    	User time (seconds): 0.96
    	System time (seconds): 0.12
    	Percent of CPU this job got: 115%
    	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.93
    	Average shared text size (kbytes): 0
    	Average unshared data size (kbytes): 0
    	Average stack size (kbytes): 0
    	Average total size (kbytes): 0
    	Maximum resident set size (kbytes): 227764
    	Average resident set size (kbytes): 0
    	Major (requiring I/O) page faults: 0
    	Minor (reclaiming a frame) page faults: 57817
    	Voluntary context switches: 274
    	Involuntary context switches: 54
    	Swaps: 0
    	File system inputs: 0
    	File system outputs: 0
    	Socket messages sent: 0
```
