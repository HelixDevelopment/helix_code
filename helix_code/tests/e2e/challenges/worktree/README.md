# Challenge — Git Worktree Agent Isolation (P1-F04)

End-to-end runtime evidence that worktree isolation actually keeps a
worktree's commits OUT of the main worktree.

## Scenarios

1. **S1 — isolation preserves main**: enter `feature-x`, write a file,
   commit on `feature-x`, then verify main's HEAD is unchanged and the
   new file does NOT appear in the main worktree.
2. **S2 — clean re-entry is idempotent**: enter `feature-y` twice; both
   calls return the same path.
3. **S3 — invalid names rejected**: try `../etc`, empty string, names
   containing spaces, and 65-char names. All four must be rejected.

## Run

```bash
cd HelixCode && tests/e2e/challenges/worktree/run.sh
```

Exit 0 means PASS. Exit non-zero means at least one scenario failed.

## Mutation test (CONST-039)

To verify the Challenge actually catches a broken engine:

```go
// in internal/tools/worktree/manager.go ValidateName():
//     if !worktreeNamePattern.MatchString(name) { ... }
//
// Comment out the regex check.
```

Re-run `run.sh`. S3 MUST FAIL because invalid names are now accepted.
Revert the mutation and confirm PASS.
