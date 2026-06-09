# HXC-056 — 7 submodules: CONST-052 capitalised replace => ../PliniusCommon
**Captured:** 2026-06-09T15:06:50Z · Bug · Fixed (→ Fixed.md)
## Defect
auto_temp, claritas, gandalf_solutions, hyper_tune, leak_hub, ouroborous, veritas each had
go.mod line 16 `replace digital.vasic.pliniuscommon => ../PliniusCommon`; the capitalised
`../PliniusCommon` dir is absent after the CONST-052 rename, the lowercase `../plinius_common`
exists → `go build ./...` failed on all 7. (3 of the 7 — auto_temp/claritas/veritas — had the
edit in their working tree from a rate-limited subagent that died before committing; finished here.)
## Fix
All 7 go.mod line 16 → `=> ../plinius_common` (path-case only).
## Captured runtime evidence (real `go build ./...` per submodule)
auto_temp_BUILD_OK  claritas_BUILD_OK  gandalf_solutions_BUILD_OK  hyper_tune_BUILD_OK
leak_hub_BUILD_OK  ouroborous_BUILD_OK  veritas_BUILD_OK
## Commits (go.mod only, pushed ff to all remotes)
auto_temp 816d47b · claritas a009751 · gandalf_solutions 141a205 · hyper_tune 34c424d · leak_hub be6c9f2 · ouroborous 6700977 · veritas d863c08
