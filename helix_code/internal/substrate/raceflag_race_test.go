//go:build race

package substrate

// raceEnabled reports whether the test binary was built with -race. Used by the
// RED_MODE reproduction to avoid a blind PASS when invoked without the race
// detector (the reliable reproduction oracle for the Dispatch/Shutdown race).
const raceEnabled = true
