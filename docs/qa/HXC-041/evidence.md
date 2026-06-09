# HXC-041 — helixqa standalone HTTP bank-runner (`helixqa http`) + live proof
New subcommand `helixqa http -bank <yaml> -base-url <url>` (helix_qa cmd/helixqa/http.go +281, http_test.go +285).
Drives a bank's http: cases through pkg/autonomous HTTPExecutor — NO Playwright, NO LLM.
- go build exit 0 (bin/helixqa 29.7M); go test ./pkg/autonomous/... ./cmd/helixqa/... → ok (8/8, incl. 2 §1.1 mutation tests that correctly FAIL).
- LIVE vs booted helix_code/bin/helixcode (port 18081, db=nil graceful): helixcode-auth.yaml → 16 cases: 15 PASS, 1 FAIL, exit 1 (the 1 FAIL = real bug HXC-043).
helix_qa commit d6c084d6 (3 files, no git add -A).
