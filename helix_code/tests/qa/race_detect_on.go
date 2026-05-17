// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

//go:build race

package qa

// raceDetectorActive is true in race builds (-race / -tags=race). The
// companion file race_detect.go (build tag `!race`) sets it to false.
// Used by TestQA_BuildQuality_NoRaceConditions to detect whether
// `go test -race` was actually invoked.
const raceDetectorActive = true
