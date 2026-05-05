// testhelper/main.go is a TEST-ONLY standalone Go binary built by
// subprocess_spawner_test.go's TestMain. It mirrors the helper-mode protocol
// that T08 will implement on the real host binary: it inspects the
// HELIXCODE_SUBAGENT_HELPER env-var, decodes the SubagentTask payload from
// HELIXCODE_SUBAGENT_HELPER_PAYLOAD, and emits a SubagentResult JSON to
// stdout.
//
// The behaviour is configurable via HELIXCODE_TEST_HELPER_BEHAVIOR:
//
//	(unset) | "ok"  → exit 0, write a SubagentResult{State: succeeded,
//	                  Output: "helper-handled: " + Prompt} as JSON to stdout
//	"fail"           → exit 1, write something to stderr (no stdout JSON)
//	"invalid-json"   → exit 0, write garbage bytes to stdout (no JSON)
//	"slow"           → sleep 5s, then behave like "ok". Lets the parent kill us.
//	"panic"          → panic during decode (process exits with non-zero)
//
// This file is NOT a Go test helper (i.e., not a *_test.go file and not
// registered with testing.M); it is a real, separate Go binary at
// `internal/agent/subagent/testhelper`. The parent test compiles it via
// `go build` in TestMain and invokes it as a subprocess so the protocol
// round-trip is exercised end-to-end with actual stdin / stdout / stderr
// pipes between two real OS processes.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// These constants MUST stay in lockstep with the parent's
// SubagentHelperEnvVar / SubagentHelperPayloadEnvVar in subprocess_spawner.go.
// We re-declare them here (rather than importing from the subagent package)
// so the helper stays a pure standalone binary with no dependency on the
// rest of the module.
const (
	helperEnvVar        = "HELIXCODE_SUBAGENT_HELPER"
	helperPayloadEnvVar = "HELIXCODE_SUBAGENT_HELPER_PAYLOAD"
	behaviorEnvVar      = "HELIXCODE_TEST_HELPER_BEHAVIOR"
)

// minimalTask is a structural subset of subagent.SubagentTask sufficient to
// compute the helper's output. Keeping a local copy avoids importing the
// subagent package and keeps the binary lightweight.
type minimalTask struct {
	ID          string `json:"id"`
	Prompt      string `json:"prompt"`
	Description string `json:"description"`
}

// minimalResult mirrors the JSON shape of subagent.SubagentResult that the
// parent decodes. Only the fields the tests assert on are present; unspecified
// fields decode to zero values on the parent side.
type minimalResult struct {
	TaskID        string `json:"task_id"`
	State         string `json:"state"`
	Output        string `json:"output"`
	Error         string `json:"error,omitempty"`
	Duration      int64  `json:"duration"`
	Isolation     string `json:"isolation"`
	ToolCallCount int    `json:"tool_call_count"`
}

func main() {
	// Sanity: only run when invoked as a helper. This mirrors the real T08
	// dispatch which will gate on IsSubagentInvocation().
	if os.Getenv(helperEnvVar) != "1" {
		fmt.Fprintln(os.Stderr, "testhelper: not a helper invocation (HELIXCODE_SUBAGENT_HELPER unset)")
		os.Exit(2)
	}

	behavior := os.Getenv(behaviorEnvVar)

	// "fail" branch: emit stderr, exit non-zero, write nothing to stdout.
	if behavior == "fail" {
		fmt.Fprintln(os.Stderr, "testhelper: forced failure for HELIXCODE_TEST_HELPER_BEHAVIOR=fail")
		os.Exit(1)
	}

	// "invalid-json" branch: write garbage to stdout (the parent must report
	// "invalid helper output").
	if behavior == "invalid-json" {
		fmt.Fprint(os.Stdout, "this is not json {{{ ]]] garbage")
		os.Exit(0)
	}

	rawPayload := os.Getenv(helperPayloadEnvVar)
	if rawPayload == "" {
		fmt.Fprintln(os.Stderr, "testhelper: missing HELIXCODE_SUBAGENT_HELPER_PAYLOAD")
		os.Exit(2)
	}

	if behavior == "panic" {
		panic("testhelper: forced panic for HELIXCODE_TEST_HELPER_BEHAVIOR=panic")
	}

	var task minimalTask
	if err := json.Unmarshal([]byte(rawPayload), &task); err != nil {
		fmt.Fprintf(os.Stderr, "testhelper: invalid payload: %v\n", err)
		os.Exit(2)
	}

	// "slow" branch: sleep so the parent's timeout / ctx cancel can trip.
	// The sleep MUST be interruptible by the parent killing the process; we
	// use time.Sleep here because the parent's exec.CommandContext kills the
	// child via SIGKILL when ctx fires, which terminates the process
	// regardless of what it's doing.
	if behavior == "slow" {
		time.Sleep(5 * time.Second)
	}

	res := minimalResult{
		TaskID:        task.ID,
		State:         "succeeded",
		Output:        "helper-handled: " + task.Prompt,
		Duration:      0,
		Isolation:     "none",
		ToolCallCount: 0,
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(&res); err != nil {
		fmt.Fprintf(os.Stderr, "testhelper: encode result: %v\n", err)
		os.Exit(2)
	}
	os.Exit(0)
}
