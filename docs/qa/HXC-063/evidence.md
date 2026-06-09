# HXC-063 — panoptic StartRecording unreachable recorder-bootstrap (§11.4.124)
**Captured:** 2026-06-09T16:13:17Z · Bug · Fixed (→ Fixed.md)
RED: go vet desktop.go:305 unreachable code — early return nil at :304 made MkdirAll+cmd.Start dead; StartRecording returned success WITHOUT recording.
§11.4.124 investigation: the dead block was a DUPLICATE of the live path above (validate→build cmd→MkdirAll video dir l281→cmd.Start). Fix: removed dead duplicate + early return; live path now stores d.recordingCmd (was missing → StopRecording can now terminate). Restore-not-delete.
GREEN: VET_CLEAN (unreachable gone), PANOPTIC_BUILD_OK, ./internal/platforms tests PASS (Factory/Mobile/Web). -4/-25 desktop.go.
Commit cd61864, pushed all remotes.
