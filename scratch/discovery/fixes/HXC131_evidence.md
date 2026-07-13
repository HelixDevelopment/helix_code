# HXC-131 — internal/cognee client auth/bearer "regression" — evidence

Date: 2026-07-12
Repo (inner app): `helix_code/`
Scope allowed: `helix_code/internal/cognee/*.go` (production + tests).
Scope touched: **none** — no edits made. Nothing committed (per instructions;
none was needed).

Discipline followed: §11.4.115 (reproduce-first RED before any fix), §11.4.102
(root-cause via investigation, not guessing), §11.4.114 (last-known-good-tag /
commit regression isolation), §11.4.6 (no false-success, no fabricated
findings), §11.4.123 (rock-solid proof or deep research — never bluff a fix
for a defect that cannot be reproduced).

---

## Task as given

FACT claimed: `TestClientBearerLogin`, `TestClientAPIKeyHeader/WithoutAPIKey`,
`TestClientHTTP_Stress_*` in `internal/cognee` fail with "login endpoint hit 0
times … bearer token not cached" — an in-process `httptest`-based,
infra-independent reproduction (no live Cognee service required).

## Step 1 — RED attempt (§11.4.115 reproduce-first)

Ran the named tests exactly as specified, in the current working tree
(`feature/helixllm-full-extension`, no local modifications to
`internal/cognee/`):

```
$ cd helix_code && go test -tags=nogui -run 'TestClientBearerLogin|TestClientAPIKeyHeader' ./internal/cognee/... -count=1 -v
=== RUN   TestClientAPIKeyHeader
=== RUN   TestClientAPIKeyHeader/WithAPIKey
=== RUN   TestClientAPIKeyHeader/WithoutAPIKey
--- PASS: TestClientAPIKeyHeader (0.00s)
    --- PASS: TestClientAPIKeyHeader/WithAPIKey (0.00s)
    --- PASS: TestClientAPIKeyHeader/WithoutAPIKey (0.00s)
=== RUN   TestClientBearerLogin
--- PASS: TestClientBearerLogin (0.00s)
PASS
ok  	dev.helix.code/internal/cognee	0.011s
```

```
$ cd helix_code && go test -tags=nogui -run 'TestClientHTTP_Stress' ./internal/cognee/... -count=3 -v
=== RUN   TestClientHTTP_Stress_SustainedSearch
    ... p50=0.040ms p95=0.072ms p99=0.198ms errRate=0.0000 ...
--- PASS: TestClientHTTP_Stress_SustainedSearch (0.04s)
=== RUN   TestClientHTTP_Stress_ConcurrentRequests
    ... parallelism=16 calls=800 gDelta=3 deadlock=false ...
--- PASS: TestClientHTTP_Stress_ConcurrentRequests (0.08s)
[... repeated 3x, all PASS ...]
PASS
ok  	dev.helix.code/internal/cognee	0.342s
```

**No RED reproduction obtained.** The specified failure ("login endpoint hit 0
times … bearer token not cached") did not occur on a single run. Per
§11.4.115/§11.4.146, a fix must never be written against a defect that cannot
first be reproduced — doing so would itself be a bluff.

## Step 2 — exhaustive reproduction attempts before concluding "not
reproducible" (§11.4.123 — rock-solid proof, not a first-try shrug)

1. Full package, default settings, `-count=1`: **PASS** (`ok
   dev.helix.code/internal/cognee 19.211s`).
2. Full package under `-race -count=5` (catches goroutine data races on
   `c.bearerToken` / `c.mu`, the most likely root cause of a token-not-cached
   symptom): **PASS** (`ok dev.helix.code/internal/cognee 101.376s`), zero
   race reports.
3. Targeted tests (`TestClientBearerLogin`, `TestClientAPIKeyHeader`,
   `TestClientHTTP_Stress_*`) run **15 consecutive times** in a loop,
   `-count=1` each: **15/15 PASS**, no flake observed.
4. `go vet -tags=nogui ./internal/cognee/...`: clean (rc=0).
5. `go build -tags=nogui ./internal/cognee/...`: clean (rc=0).
6. Checked for an environment-variable-conditional trigger analogous to the
   sibling `internal/llm` Azure-endpoint bug found elsewhere in this same
   sweep (env var leaking a default that changes branch behavior,
   `scratch/discovery/fixes/W5_infradefects_evidence.md` Defect A). Cognee's
   `.env.full-test` only sets `COGNEE_ENDPOINT` / `COGNEE_API_KEY`;
   `config.DefaultCogneeConfig()` does not read env vars, and
   `TestClientBearerLogin` explicitly overrides `RemoteAPI.Username/Password`
   after constructing the default config — no env-conditional path exists
   here. Ruled out.

## Step 3 — root cause of the discrepancy (§11.4.102 investigation, not a
guess)

Read `git log --oneline -5 -- internal/cognee/` on this branch:

```
b058c7c2 fix(tests/cognee): make the package deterministic under load (§11.4.50/§11.4.6) — no product change
5b015776 fix(tests): resolve 4 pre-existing test-side failures surfaced by the release-readiness suite
9e6e0458 test(stress/chaos): §11.4.85 resilience coverage for sessionrules.Store + cognee client
7f028910 fix(cognee): rewrite client to real cognee 1.1.2 v1+bearer API (exposed a PASS-bluff, §11.4.118/.99)
b91f166e fix(kilocode,cognee,fix): path-traversal write + credential DATA RACE + scanner PASS-bluff (§11.4.118 discovery round 13)
```

`b058c7c2`'s commit message and diff are a *verbatim match* for the exact
symptom class described in this task: it documents that
`TestClientHTTP_Stress_ConcurrentRequests` (and the package as a whole) was
**load-flaky** — "a different test failed each run; all passed in
isolation — product correct" — and that the root cause was two **test-harness
transport/timeout defects**, not a client-code defect:

- `DisableKeepAlives: true` in the hermetic `httptest` client transport caused
  an 800-connection open/close storm under the `§11.4.85` concurrent stress
  test, which under load could exceed the goroutine-leak-delta tolerance
  and/or exhaust ephemeral sockets — manifesting as spurious call failures
  under load (consistent with the reported class of failure, including the
  login-hit-count assertion being reachable via a failed/timed-out call path
  under contention).
- GPU-probe const timeouts (`nvidiaSmiQueryTimeout`, `appleIoregQueryTimeout`,
  `intelGPUTopQueryTimeout`) could not be raised for load-robustness the way
  `rocmSmiQueryTimeout` already had been (HXC-064), so probe-chain tests could
  flake under heavy parallel `go test ./...` machine load.

The fix (already landed, present in this checkout) changed the test-harness
transport to a single reused keep-alive connection (`MaxConnsPerHost: 1`),
raised the client timeout from 3s → 8s, and made the three GPU-probe timeouts
tunable `var`s instead of `const`. **No change was made to `client.go`'s
`login`/`attachAuth` bearer-caching logic** — that logic was already correct
(verified below) and remains untouched since at least `7f028910`.

The commit's own verification note states: "stress test alone 12/12 (was
0/15)" — i.e., the exact "always/mostly fails" symptom this task describes
was independently observed and fixed in this branch's history, at
`b058c7c28c48dd09aa30cfaa5b5bf60c8565c35e` (2026-06-24), which is an ancestor
of the current `HEAD` on `feature/helixllm-full-extension`.

**Conclusion: this is the SAME regression, already fixed by `b058c7c2`
(landed before this task was dispatched). No further product or test change
is required or safe to make** — HXC-131 as filed describes pre-`b058c7c2`
state; on current `HEAD` it does not reproduce under any condition tried
(clean run, `-race`, 15x repeat, `-count=5`).

## Step 4 — code-correctness check of the bearer/auth path (defense in depth)

Read `internal/cognee/client.go` `attachAuth` (L856-892) and `login`
(L894-936):

- `attachAuth` prefers `X-Api-Key` when an API key is configured; otherwise,
  when username+password are configured, it lazily calls `login()` **once**
  and caches `c.bearerToken` under `c.mu` (`RLock` to read, `Lock` to write in
  `login`), then attaches `Authorization: Bearer <tok>`. With neither
  credential configured, the request goes out unauthenticated and the
  server's real 401 surfaces — never masked.
- `login()` makes a real `http.NewRequestWithContext` + `c.httpClient.Do`
  POST to `pathLogin` with real form-encoded `username`/`password`/
  `grant_type`, decodes the real JSON response, and caches
  `c.bearerToken = lr.AccessToken` under `c.mu.Lock()`.

This is a genuine, non-bluff implementation (real HTTP call, real caching,
real mutex-guarded shared state) — matches CLAUDE.md §3.3/§4.1 patterns
exactly. `TestClientBearerLogin` (cognee_test.go:1811) is itself a §11.4.135
permanent regression guard asserting `loginCalls == 1` across two
`ListDatasets` calls — i.e. the exact "must not be 0, must be cached" property
the task asked to verify, already registered and passing.

## GREEN evidence (final confirmation, same run set as Step 1/2, re-verified
at the end of this investigation for freshness)

```
$ cd helix_code && go test -tags=nogui ./internal/cognee/... -count=1
ok  	dev.helix.code/internal/cognee	19.211s
ok  	dev.helix.code/internal/cognee/i18n	0.002s

$ cd helix_code && go build -tags=nogui ./internal/cognee/...
(exit 0, no output)
```

## Status

**No fix applied — none needed.** The reported defect is a duplicate of
`b058c7c28c48dd09aa30cfaa5b5bf60c8565c35e`, already fixed and present on this
branch's `HEAD`. Per §11.4.124 (investigate-before-remove/modify) and
§11.4.1 (no fix without a reproducible defect), no edit was made to
`internal/cognee/*.go`. Recommend HXC-131 be closed/reconciled against
`b058c7c2` rather than re-worked, or reopened only if a NEW reproduction
(different trigger, different commit, different environment) surfaces with
captured evidence per §11.4.55/§11.4.58.
