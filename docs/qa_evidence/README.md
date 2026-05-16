# helix_qa QA-Evidence Directory

Per operator mandate 2026-05-13:

> "Once all is up and running execute all existing tests and Challenges +
> helix_qa full testing and in-depth QA sessions! ... Obtain all apps and
> services we have screenshots during the helix_qa testing and put them in
> proper directories! non-bluff verification and validation MUST check
> generated screenshots to confirm System really being used and working!
> Created screenshots and video recording materials during the HelixQA
> QA sessions MUST BE Git ignored!"

## Directory layout

```
docs/qa_evidence/
├── README.md                          # this file (tracked)
└── qa_session_<UTC-timestamp>/        # one dir per helix_qa session (gitignored)
    ├── summary.json                   # session metadata + overall PASS/FAIL
    ├── platforms/                     # platform-specific evidence
    │   ├── web/                       # browser screenshots / DOM snapshots
    │   ├── desktop/                   # desktop app captures
    │   └── android/                   # adb captures + logcat
    ├── screenshots/*.qa-screenshot.png
    ├── recordings/*.qa-recording.mp4
    └── traces/*.qa-trace.json
```

## Anti-bluff role

Per CONST-035 / Article XI §11.9: every PASS in this codebase MUST carry
positive runtime evidence captured during execution. helix_qa captures
that evidence into `qa_session_<timestamp>/`; the post-session anti-bluff
verifier (a Go program under `tests/e2e/challenges/`) scans the directory
and asserts that:

- Every claimed-PASS test has at least one screenshot in the session dir
- Each screenshot has non-zero file size (per §11.4.5 "presence")
- For UI features, the screenshot dimensions match the test's claimed resolution
- For recording-tagged tests, `ffprobe -count_frames` reports a decoded-frame
  total > 0 (Bug #24 canonical PASS-bluff)

A test that scores PASS without a matched evidence artefact in its session
dir is treated as a §11.4 PASS-bluff and fails the post-session gate.

## Gitignore behaviour

Every file inside `qa_evidence/` is gitignored EXCEPT this README. Evidence
artefacts stay local — they are inherently per-run, may be large, and may
contain sensitive UI state. Do not commit them. Do not push them.

## Programmatic access

The Go helper `internal/helixqa.Engine.EvidenceCollector(platform)`
(see `HelixCode/internal/helixqa/wrapper.go:186`) returns the
`hqaEvidence.Collector` that writes into the session directory layout
described above. Test code that needs to capture extra evidence should
go through that collector — never write directly to `qa_evidence/`.

## Session-runner script (TBD when mistborn online)

`scripts/qa-session.sh` will be the one-shot driver: `mistborn-up.sh`
then `helixqa-runner --bank docs/banks/*.yaml --evidence-dir
docs/qa_evidence/qa_session_$(date -u +%Y%m%dT%H%M%SZ)/` then the
post-session anti-bluff verifier. Land that script once the operator
wakes mistborn so we can iterate on real session output instead of
guessing the helixqa-runner CLI surface.
