# HXC-132 — Evidence

Implementer scope: fix (A) `helixcode-server` build-path bug in
`helix_code/docker-compose.full-test.yml`, and (B) make hardcoded-`:8080`
test files honor `HELIXCODE_TEST_URL`. No commit made. No containers booted
(config-parse only, per instructions).

## FACT (A) — root cause investigation

Real layout, confirmed by direct filesystem search:

```
$ find /home/milos/Factory/projects/tools_and_research/helix_code -iname "Dockerfile.test*" | grep -v /.git/
/home/milos/Factory/projects/tools_and_research/helix_code/Dockerfile.test               <- meta-repo ROOT
/home/milos/Factory/projects/tools_and_research/helix_code/cli_agents/bridle/marketplace/Dockerfile.test   <- unrelated
```

`helix_code/Dockerfile.test` does **not** exist. The compose file lives at
`helix_code/docker-compose.full-test.yml` and originally declared:

```yaml
  helixcode-server:
    build:
      context: .
      dockerfile: Dockerfile.test
```

`context: .` resolves relative to the compose file's own directory, i.e.
`helix_code/`. So the build was looking for `helix_code/Dockerfile.test`,
which doesn't exist — build would fail before ever reaching the container
registry/image steps.

The real `Dockerfile.test` (repo root) confirms the intended build context is
the **meta-repo root**, not `helix_code/`:

```dockerfile
WORKDIR /app
COPY helix_code/go.mod helix_code/go.sum ./     # <- needs helix_code/ subdir in context
RUN go mod download
COPY helix_code/ .                              # <- needs helix_code/ subdir in context
COPY scripts/run-docker-tests.sh /app/test-runner   # <- needs scripts/ at context root
...
COPY scripts/run-docker-tests.sh .
COPY scripts/run-all-tests.sh .
COPY scripts/run-tests.sh .
```

Confirmed both required paths exist at the meta-repo root, matching the
Dockerfile's COPY instructions:

```
$ ls -la /home/milos/Factory/projects/tools_and_research/helix_code/scripts/run-docker-tests.sh \
         /home/milos/Factory/projects/tools_and_research/helix_code/scripts/run-all-tests.sh \
         /home/milos/Factory/projects/tools_and_research/helix_code/scripts/run-tests.sh
-rwxr-xr-x 1 milos milos 6253 Jul  5 03:30 .../scripts/run-all-tests.sh
-rwxr-xr-x 1 milos milos 6125 Jul  5 03:30 .../scripts/run-docker-tests.sh
-rwxr-xr-x 1 milos milos 3885 Jul  5 03:30 .../scripts/run-tests.sh

$ ls -la /home/milos/Factory/projects/tools_and_research/helix_code/helix_code/go.mod
-rw-r--r-- 1 milos milos 13426 Jul 12 13:47 .../helix_code/go.mod
```

### Fix applied

`helix_code/docker-compose.full-test.yml` (service `helixcode-server`):

```diff
   helixcode-server:
     build:
-      context: .
+      # Dockerfile.test lives at the meta-repo root (../Dockerfile.test relative
+      # to this compose file) and its COPY instructions (`COPY helix_code/...`,
+      # `COPY scripts/run-docker-tests.sh ...`) are written relative to that
+      # root, not to this helix_code/ subdirectory. context: . would resolve
+      # dockerfile: Dockerfile.test to a nonexistent helix_code/Dockerfile.test.
+      context: ..
       dockerfile: Dockerfile.test
```

`dockerfile: Dockerfile.test` is unchanged — it is now correctly resolved
relative to the new `context: ..` (meta-repo root), where the file actually
lives. All other services in this compose file that build from Dockerfiles
(`mock-llm-server`, `ssh-server`, `ssh-worker-*`, `mock-slack`) already use
this exact `context: <dir containing the Dockerfile>` + bare `dockerfile:
<name>` pattern — this fix brings `helixcode-server` in line with the
existing convention rather than inventing a new one.

### Verification — compose config parses (no `up`, no containers booted)

`helixcode-server` is gated behind `profiles: [server]`, so a plain
`podman-compose ... config` omits it (default profile filtering — this is
compose's normal behavior, not a defect). Verified twice: once against the
full default-profile config (no errors, `helixcode-server` correctly
omitted because it's profile-gated), and once with `--profile server` to
force-include and inspect the resolved build path.

```
$ podman-compose -f docker-compose.full-test.yml config > /tmp/compose_config_out.yml 2>/tmp/compose_config_err.txt
$ echo exit:$?
exit:0
$ cat /tmp/compose_config_err.txt
(empty — no errors/warnings)

$ podman-compose --profile server -f docker-compose.full-test.yml config > /tmp/compose_config_out2.yml 2>/tmp/compose_config_err2.txt
$ echo exit:$?
exit:0
$ cat /tmp/compose_config_err2.txt
(empty)
```

Resolved build block for `helixcode-server` (parsed via PyYAML from the
`--profile server` config output):

```
build: {'context': '/home/milos/Factory/projects/tools_and_research/helix_code', 'dockerfile': 'Dockerfile.test'}
resolved context: /home/milos/Factory/projects/tools_and_research/helix_code
resolved dockerfile path: /home/milos/Factory/projects/tools_and_research/helix_code/Dockerfile.test
dockerfile exists: True
```

Before the fix, the equivalent resolution would have been
`context=.../helix_code/`, `dockerfile path=.../helix_code/Dockerfile.test`,
which does **not** exist (confirmed above) — build would fail on `podman
build`/`docker build` with a "Dockerfile not found" error. After the fix the
resolved path exists on disk. No `up`/`build` was executed per instructions
— this is config-parse + filesystem-existence verification only.

## FACT (B) — hardcoded `:8080` test-URL audit

Full inventory of every `localhost:8080` / `:8080` literal under
`helix_code/tests/` (Go source only, `grep -rn '"http://localhost:8080"'`):

| File | Override present before fix | Env var |
|---|---|---|
| `tests/security/owasp_test.go` | yes | `HELIXCODE_TEST_URL` |
| `tests/performance/benchmark_test.go` | yes | `HELIXCODE_TEST_URL` |
| `tests/e2e/test_bank/core/tests.go` | yes | `HELIXCODE_TEST_URL` |
| `tests/e2e/test_bank/integration/tests.go` | yes | `HELIXCODE_TEST_URL` |
| `tests/e2e/test_bank/platform/tests.go` | yes | `HELIXCODE_TEST_URL` |
| `tests/e2e/test_bank/distributed/tests.go` | yes | `HELIXCODE_TEST_URL` |
| `tests/e2e/test_bank/performance/performance_security_tests.go` | yes | `HELIXCODE_TEST_URL` |
| `tests/e2e/phase2/integration_test.go` | yes | `HELIX_TEST_SERVER` (different var, out of scope — already overridable) |
| `tests/e2e/phase3/framework.go` | yes | `HELIX_PRODUCTION_SERVER` (different var, out of scope — already overridable) |
| `tests/e2e/phase2/test_helpers.go` | yes | `HELIX_TEST_SERVER_URL` (different var, out of scope — already overridable) |
| `tests/integration/integration_test.go` | yes | `TEST_BASE_URL` (different var, out of scope — already overridable) |
| `tests/integration/provider_integration_test.go` | n/a | literal `:8080` here is a spawned local `LocalAI`/`Llama.cpp` provider process port, not the HelixCode server under test — out of scope, no product-code / test-target-server behavior involved |
| **`tests/memory/memory_test.go`** | **no — pure hardcoded literal** | **fixed → `HELIXCODE_TEST_URL`** |
| **`tests/qa/qa_test.go`** | **no — pure hardcoded literal** | **fixed → `HELIXCODE_TEST_URL`** |

`tests/memory/memory_test.go` and `tests/qa/qa_test.go` were the only two Go
test files under `helix_code/tests/` with a zero-override hardcoded
`"http://localhost:8080"` literal — exactly matching the task's own example
(`tests/memory`). Every other file already had *some* override mechanism
(several already used `HELIXCODE_TEST_URL`; a few used a differently-named
var). Per the stated scope ("touch ONLY ... the test files that hardcode the
server URL"), only the two zero-override files were changed, using the exact
`HELIXCODE_TEST_URL` convention already established by
`tests/security/owasp_test.go` and the `tests/e2e/test_bank/*` files.

### Fix applied

`tests/memory/memory_test.go`:

```diff
 	"io"
 	"math"
 	"net/http"
+	"os"
 	"runtime"
 	"sync"
 	"testing"
 	"time"
 ...
+// getTestBaseURL returns the HelixCode server URL to test against, honoring
+// the HELIXCODE_TEST_URL override (same convention as tests/security and
+// tests/e2e/test_bank) so the suite can be pointed at a real running server.
+func getTestBaseURL() string {
+	if baseURL := os.Getenv("HELIXCODE_TEST_URL"); baseURL != "" {
+		return baseURL
+	}
+	return "http://localhost:8080"
+}
+
 // DefaultTestConfig returns a default test configuration
 func DefaultTestConfig() *TestConfig {
 	return &TestConfig{
-		BaseURL:     "http://localhost:8080",
+		BaseURL:     getTestBaseURL(),
 		AdminToken:  "test-admin-token",
```

`tests/qa/qa_test.go` (already imported `"os"`):

```diff
+// getTestBaseURL returns the HelixCode server URL to test against, honoring
+// the HELIXCODE_TEST_URL override (same convention as tests/security and
+// tests/e2e/test_bank) so the suite can be pointed at a real running server.
+func getTestBaseURL() string {
+	if baseURL := os.Getenv("HELIXCODE_TEST_URL"); baseURL != "" {
+		return baseURL
+	}
+	return "http://localhost:8080"
+}
+
 // DefaultTestConfig returns a default test configuration
 func DefaultTestConfig() *TestConfig {
 	// Find project root by looking for go.mod
 	projectRoot := findProjectRoot()
 	return &TestConfig{
-		BaseURL:     "http://localhost:8080",
+		BaseURL:     getTestBaseURL(),
 		AdminToken:  "test-admin-token",
```

Default behavior is unchanged (still falls back to `http://localhost:8080`
when `HELIXCODE_TEST_URL` is unset) — this is additive, not a behavior
change to product code, and no other test/product code was touched.

## Verification — Go compiles / vets clean

```
$ cd helix_code && go vet -tags=nogui ./tests/memory/... ./tests/qa/...
(no output)
$ echo exit:$?
exit:0
```

Compile-only proof the modified test files themselves parse/typecheck
(`-run '^$'` matches nothing, so nothing executes — this only proves
compilation, consistent with "do not boot containers / do not require a live
server" scope):

```
$ go test -tags=nogui -run '^$' -count=1 ./tests/memory/... ./tests/qa/...
ok  	dev.helix.code/tests/memory	0.004s [no tests to run]
ok  	dev.helix.code/tests/qa	0.068s [no tests to run]
$ echo EXIT=$?
EXIT=0
```

Full-repo build (confirms nothing else broke; per Go tooling semantics
`go build ./...` does not itself typecheck `_test.go` files — that check is
covered by the `go vet` / `go test -run '^$'` runs above):

```
$ cd helix_code && go build -tags=nogui ./...
(no output)
$ echo EXIT_CODE=$?
EXIT_CODE=0
```

Symbol-collision check (both new `getTestBaseURL()` helpers live in
different Go packages — `memory` and `qa` — so no cross-package collision;
confirmed only one definition per package):

```
$ grep -rn "func getTestBaseURL" tests/memory/ tests/qa/
tests/qa/qa_test.go:35:func getTestBaseURL() string {
tests/memory/memory_test.go:33:func getTestBaseURL() string {
```

## Diff summary (files touched — matches stated scope exactly)

```
 helix_code/docker-compose.full-test.yml |  7 ++++++-
 helix_code/tests/memory/memory_test.go  | 13 ++++++++++++-
 helix_code/tests/qa/qa_test.go          | 12 +++++++++++-
 3 files changed, 29 insertions(+), 3 deletions(-)
```

No other files touched. No product code (`internal/`, `cmd/`, etc.) changed.
No commit made — working tree left with these three files modified,
uncommitted, per instructions.
