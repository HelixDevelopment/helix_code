// helper_mode_test.go — P1-F15-T08 unit tests for the child-side helper
// dispatch.
//
// Covers:
//   - IsSubagentInvocation env-var behaviour ("1" → true; unset / "0" / other
//     values → false).
//   - RunAsSubagent's protocol round-trip: env-var decode → InProcessSpawner
//     against the supplied factory's provider → JSON SubagentResult on stdout
//     → exit code 0.
//   - Failure modes:
//       * malformed JSON payload → StateFailed, exit 1
//       * missing payload env-var → StateFailed, exit 1
//       * llmFactory returns error → StateFailed result on stdout, exit 0
//       * nil llmFactory → exit 1 (programmer error)
//
// Stdout capture goes via runAsSubagentWithWriter so tests can read into a
// strings.Builder without touching os.Stdout (which would interfere with the
// `go test` reporter).
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f15-subagent-team-design.md §4.2
// Plan: docs/superpowers/plans/2026-05-06-p1-f15-subagent-team.md T08
package subagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"dev.helix.code/internal/llm"
)

// setHelperEnv writes the helper-mode env vars for the duration of the test.
// t.Setenv handles cleanup automatically.
func setHelperEnv(t *testing.T, payload string) {
	t.Helper()
	t.Setenv(SubagentHelperEnvVar, "1")
	t.Setenv(SubagentHelperPayloadEnvVar, payload)
}

func TestIsSubagentInvocation_FalseWhenEnvUnset(t *testing.T) {
	// Defensively unset both env vars; t.Setenv("","") is invalid so use
	// t.Setenv with an empty value AFTER asserting they look unset.
	t.Setenv(SubagentHelperEnvVar, "")
	if IsSubagentInvocation() {
		t.Fatalf("expected IsSubagentInvocation=false when %s is empty", SubagentHelperEnvVar)
	}
}

func TestIsSubagentInvocation_TrueWhenEnvSetTo1(t *testing.T) {
	t.Setenv(SubagentHelperEnvVar, "1")
	if !IsSubagentInvocation() {
		t.Fatalf("expected IsSubagentInvocation=true when %s=1", SubagentHelperEnvVar)
	}
}

func TestIsSubagentInvocation_FalseForOtherValues(t *testing.T) {
	for _, v := range []string{"0", "yes", "true", "false", "no"} {
		t.Setenv(SubagentHelperEnvVar, v)
		if IsSubagentInvocation() {
			t.Fatalf("expected IsSubagentInvocation=false for %s=%q (only %q means yes)", SubagentHelperEnvVar, v, "1")
		}
	}
}

// fakeFactory returns a closure that yields a FakeLLMProvider seeded with the
// canned map. Useful for the round-trip tests below.
func fakeFactory(canned map[string]string) SubagentLLMProviderFactory {
	return func(ctx context.Context) (llm.Provider, error) {
		return NewFakeLLMProvider(canned), nil
	}
}

func TestRunAsSubagent_DecodesTaskAndEmitsResult(t *testing.T) {
	task := SubagentTask{
		ID:          "task-helper-1",
		Description: "round-trip",
		Prompt:      "hello-helper",
		Isolation:   IsolationNone,
	}
	payload, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}
	setHelperEnv(t, string(payload))

	var buf bytes.Buffer
	exitCode := runAsSubagentWithWriter(&buf, fakeFactory(map[string]string{
		"hello-helper": "helper-canned-99",
	}))

	if exitCode != 0 {
		t.Fatalf("expected exit 0 on success, got %d (stdout=%q)", exitCode, buf.String())
	}

	// Real JSON round-trip: decode the captured stdout back into a
	// SubagentResult and assert every field we care about.
	var got SubagentResult
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("decoding helper stdout failed: %v (stdout=%q)", err, buf.String())
	}

	if got.State != StateSucceeded {
		t.Fatalf("expected StateSucceeded, got %q (err=%q)", got.State, got.Error)
	}
	if got.Output != "helper-canned-99" {
		t.Fatalf("expected Output=helper-canned-99, got %q", got.Output)
	}
	if got.TaskID != "task-helper-1" {
		t.Fatalf("expected TaskID round-trip, got %q", got.TaskID)
	}
	if got.Isolation != IsolationNone {
		t.Fatalf("expected Isolation=none round-trip, got %q", got.Isolation)
	}
}

func TestRunAsSubagent_OutputIsValidJSON_RoundTripsBack(t *testing.T) {
	// Same as the previous test but asserts the JSON is parseable AS-IS
	// (no leading/trailing junk that would break the parent's json.Unmarshal).
	task := SubagentTask{
		ID:     "task-helper-roundtrip",
		Prompt: "round-trip-please",
	}
	payload, _ := json.Marshal(task)
	setHelperEnv(t, string(payload))

	var buf bytes.Buffer
	exitCode := runAsSubagentWithWriter(&buf, fakeFactory(nil))

	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatalf("expected non-empty stdout JSON")
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("stdout is not valid JSON: %q", out)
	}

	var got SubagentResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode SubagentResult: %v", err)
	}
	if got.TaskID != "task-helper-roundtrip" {
		t.Fatalf("TaskID round-trip failed: %q", got.TaskID)
	}
	// Fallback-echo path: FakeLLMProvider with no canned answer returns
	// "FAKE-LLM-ECHO: <prompt>". Asserting on this confirms the provider
	// was REALLY invoked (not bluffed).
	if !strings.Contains(got.Output, "FAKE-LLM-ECHO: round-trip-please") {
		t.Fatalf("Output did not contain FAKE-LLM-ECHO marker: %q", got.Output)
	}
}

func TestRunAsSubagent_LLMFactoryErrorEmitsFailedState(t *testing.T) {
	task := SubagentTask{ID: "task-factory-err", Prompt: "irrelevant"}
	payload, _ := json.Marshal(task)
	setHelperEnv(t, string(payload))

	factoryErr := errors.New("provider construction blew up")
	failingFactory := func(ctx context.Context) (llm.Provider, error) {
		return nil, factoryErr
	}

	var buf bytes.Buffer
	exitCode := runAsSubagentWithWriter(&buf, failingFactory)

	// Factory failure is NOT a protocol error — the parent expects a
	// SubagentResult JSON on stdout and a clean exit 0 so it can surface
	// the failure to the caller via the result channel.
	if exitCode != 0 {
		t.Fatalf("expected exit 0 on factory error (factory failure is a soft failure), got %d", exitCode)
	}

	var got SubagentResult
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("decode result: %v (stdout=%q)", err, buf.String())
	}
	if got.State != StateFailed {
		t.Fatalf("expected StateFailed, got %q", got.State)
	}
	if !strings.Contains(got.Error, "provider construction blew up") {
		t.Fatalf("expected factory error text in result.Error, got %q", got.Error)
	}
	if got.TaskID != "task-factory-err" {
		t.Fatalf("expected TaskID=task-factory-err, got %q", got.TaskID)
	}
}

func TestRunAsSubagent_MalformedPayloadEmitsFailedState(t *testing.T) {
	setHelperEnv(t, "{not json")

	var buf bytes.Buffer
	exitCode := runAsSubagentWithWriter(&buf, fakeFactory(nil))

	// Protocol error: stdout still gets a SubagentResult so the parent can
	// decode it, but exit code is 1 (this is unrecoverable on the child
	// side — there's nothing to retry).
	if exitCode != 1 {
		t.Fatalf("expected exit 1 on malformed payload, got %d", exitCode)
	}

	var got SubagentResult
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("decode result: %v (stdout=%q)", err, buf.String())
	}
	if got.State != StateFailed {
		t.Fatalf("expected StateFailed, got %q", got.State)
	}
	if !strings.Contains(got.Error, "decode") && !strings.Contains(got.Error, "payload") {
		t.Fatalf("expected payload-decode error text, got %q", got.Error)
	}
}

func TestRunAsSubagent_MissingPayloadEmitsFailedState(t *testing.T) {
	t.Setenv(SubagentHelperEnvVar, "1")
	t.Setenv(SubagentHelperPayloadEnvVar, "")

	var buf bytes.Buffer
	exitCode := runAsSubagentWithWriter(&buf, fakeFactory(nil))

	if exitCode != 1 {
		t.Fatalf("expected exit 1 on missing payload, got %d", exitCode)
	}

	var got SubagentResult
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("decode result: %v (stdout=%q)", err, buf.String())
	}
	if got.State != StateFailed {
		t.Fatalf("expected StateFailed, got %q", got.State)
	}
	if !strings.Contains(got.Error, "missing") && !strings.Contains(got.Error, "payload") {
		t.Fatalf("expected missing-payload error text, got %q", got.Error)
	}
}

func TestRunAsSubagent_NilFactoryReturnsExit1(t *testing.T) {
	task := SubagentTask{ID: "x", Prompt: "y"}
	payload, _ := json.Marshal(task)
	setHelperEnv(t, string(payload))

	var buf bytes.Buffer
	exitCode := runAsSubagentWithWriter(&buf, nil)

	if exitCode != 1 {
		t.Fatalf("expected exit 1 on nil factory, got %d", exitCode)
	}
	// We still emit a JSON SubagentResult so a hypothetical parent can
	// decode the failure (defensive).
	var got SubagentResult
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("decode result: %v (stdout=%q)", err, buf.String())
	}
	if got.State != StateFailed {
		t.Fatalf("expected StateFailed, got %q", got.State)
	}
	if !strings.Contains(got.Error, "llmFactory") {
		t.Fatalf("expected nil-llmFactory error text, got %q", got.Error)
	}
}
