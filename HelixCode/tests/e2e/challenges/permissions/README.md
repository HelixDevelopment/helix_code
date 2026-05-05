# Challenge — Permission Rule System (P1-F02)

End-to-end runtime evidence that the permission engine's decisions
correspond to real filesystem outcomes.

## Scenarios

1. **S1 — read auto-allowed under dontAsk**: `ls -la /tmp` resolves to `allow`.
2. **S2 — destructive denied under default**: a `Bash(rm*) deny` rule blocks
   `rm -rf $MARKER`; the marker file is verifiably **still present** after the call.
3. **S3 — smuggle via $() denied**: `echo hi $(rm -rf $MARKER)` resolves to `deny`
   even under `--permission-mode auto`; the marker is verifiably **still present**.

## Run

```bash
cd HelixCode && tests/e2e/challenges/permissions/run.sh
```

Exit 0 means PASS. Exit non-zero means at least one scenario produced the
wrong decision or the marker was tampered with.

## Mutation test (CONST-039)

To verify the Challenge actually catches a broken engine, temporarily change
`internal/tools/permissions/rule_engine.go` to skip the deny aggregation:

```go
// in aggregate(), before the switch:
hadDeny = false  // <-- mutation
```

Re-run `run.sh`. It MUST FAIL on S2 or S3. **Revert** the mutation and confirm PASS.
