# Challenge — Tool Result Persistence (P1-F03)

End-to-end runtime evidence that the persistence layer's behaviour
matches the spec for three boundary scenarios.

## Scenarios

1. **S1 — below threshold (49,999 bytes)**: `MaybePersist` returns inline; `.helix/tool-results/` is not created.
2. **S2 — above threshold (50,001 bytes)**: persisted; the file exists at the reported path; `wc -c` matches 50,001.
3. **S3 — hash idempotence**: two persists of identical 60,000-byte content produce filenames that share the same `sha256[:16]` hash prefix.

## Run

```bash
cd HelixCode && tests/e2e/challenges/persistence/run.sh
```

Exit 0 means PASS. Exit non-zero means at least one scenario failed.

## Mutation test (CONST-039)

To verify the Challenge actually catches a broken engine:

```go
// in internal/tools/persistence/types.go:
const PersistThreshold = 0  // <-- mutation: every output persists
```

Re-run `run.sh`. S1 MUST FAIL because every output now triggers persistence (`was_persisted=true` instead of `false`). Revert the mutation and confirm PASS.
