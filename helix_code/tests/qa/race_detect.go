// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

//go:build !race

package qa

// raceDetectorActive is false in non-race builds. The companion file
// race_detect_on.go (build tag `race`) sets it to true. Used by
// TestQA_BuildQuality_NoRaceConditions to detect whether `go test
// -race` was actually invoked.
const raceDetectorActive = false
