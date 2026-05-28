package pkg

import "errors"

// SkipError is an honest, non-PASS test verdict.
//
// Anti-bluff (§11.4 / §11.4.3): a test whose precondition is genuinely
// absent (platform/arch mismatch, an honestly-unavailable real dependency)
// MUST report SKIP — NEVER PASS. Returning a *SkipError from a TestCase's
// Execute func makes the executor record pkg.StatusSkipped, which the report
// counts separately from Passed. This replaces the historical bluff of
// returning Assert(true, "...skipped...") which silently counted as PASS
// while the test exercised nothing.
type SkipError struct {
	Reason string
}

// Error implements the error interface.
func (e *SkipError) Error() string {
	return "test skipped: " + e.Reason
}

// Skip constructs a SkipError carrying an honest reason.
func Skip(reason string) error {
	return &SkipError{Reason: reason}
}

// IsSkip reports whether err is (or wraps) a SkipError.
func IsSkip(err error) bool {
	var se *SkipError
	return errors.As(err, &se)
}
