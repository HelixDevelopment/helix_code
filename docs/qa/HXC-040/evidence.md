# HXC-040 — CLAUDE.md §9/§3.4 anti-bluff smoke command made trustworthy

## Problem (reproduced)
Original literal command `grep -rn "simulated\|for now\|TODO implement\|placeholder" helix_code/internal helix_code/cmd`
returns 527 hits → prints "BLUFF FOUND" on a GENUINELY CLEAN codebase.
Breakdown: 218 `_test.go` (mocks allowed, CONST-050), 123 i18n `tr(`/`"*_placeholder"` keys;
production term counts: simulated=0, TODO implement=0, for now=3 (all comments documenting REMOVED bluffs),
placeholder=306 (all i18n/template infra). Zero real bluffs.

## Latent bug found (bonus)
The ORIGINAL command was case-SENSITIVE → it would have MISSED the canonical BLUFF-001 marker
`// For now, simulate generation` (capital F) exactly as written in CLAUDE.md §3.3.

## Refined command (now in CLAUDE.md §3.4 + §9)
grep -rniE "\bsimulated\b|\bfor now\b|TODO implement|in production this would" \
  helix_code/internal helix_code/cmd | grep -v "_test\.go:" \
  | grep -vi 'tr(\|_placeholder"' | grep -viE ':[[:space:]]*//.*"' \
  | grep -q . && echo "BLUFF FOUND" || echo "clean"

## Proof
- Refined command on real tree → "clean" (0 hits)
- §1.1 mutation pair: planted 3 bluffs into helix_code/cmd/cli/main.go (BLUFF-001 comment + simulated-response string + TODO implement)
  → refined command reported "BLUFF FOUND" catching all three → reverted → "clean", main.go md5 byte-identical to backup.
- The quoted-string plant caught proves the //-citation exclusion does not weaken real bluff detection.
