//go:build !race

package substrate

// raceEnabled reports whether the test binary was built with -race. See
// raceflag_race_test.go.
const raceEnabled = false
