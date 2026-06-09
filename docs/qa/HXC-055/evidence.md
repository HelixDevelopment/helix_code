# HXC-055 — formatters cat --version + go_hello fixtures (§11.4.81)
**Captured:** 2026-06-09T14:54:03Z · Bug · Fixed (→ Fixed.md)
**Submodule:** submodules/formatters
## (a) RED: `cat --version` on Darwin → `cat: illegal option`, exit 1; TestNativeFormatter_HealthCheck_Success FAILed.
## (b) RED: challenges/fixtures/go_hello_{en,sr}.go are package main w/o func main → `go build ./...` FAILed tree-wide.
## Fix
(a) HealthCheck probe `<bin> --version` → `exec.LookPath(binaryPath)` (portable GNU/BSD/macOS/Windows; BinaryMissing negative contract preserved). (b) `git mv` both fixtures into challenges/fixtures/testdata/ (Go excludes testdata/ from build); runner embeds content as inline strings so files preserved (§11.4.124 relocate-not-delete).
## GREEN: go build ./... exit 0 (FMT_BUILD_OK); go vet 0; go test ./... all ok; HealthCheck Success/Missing/WithTrue PASS; runner builds+runs.
Commit `d205be3`, pushed ff `3d4f469..d205be3` (both remotes confirmed).
