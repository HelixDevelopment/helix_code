package llm

import "os"

// redMode reports whether the SP1 RED_MODE polarity switch (§11.4.115) is active.
// RED_MODE=1 → reproduce the defect on the broken artifact (the RED proof).
// default / RED_MODE=0 → standing GREEN regression guard asserting the defect is gone.
func redMode() bool {
	return os.Getenv("RED_MODE") == "1"
}
