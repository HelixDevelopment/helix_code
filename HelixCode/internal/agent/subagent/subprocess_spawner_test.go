package subagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testHelperBinary is the absolute path to the compiled standalone helper
// binary, built once in TestMain by `go build ./testhelper`. It is set to a
// freshly-built artifact for the duration of the test run and removed when
// the run completes. Tests that exercise the real helper round-trip point a
// SubprocessSpawner at this path.
var testHelperBinary string

func TestMain(m *testing.M) {
	// Build the testhelper binary into a process-unique temp path so parallel
	// `go test ./...` invocations across the module don't race on the same
	// artifact.
	binPath := filepath.Join(os.TempDir(), fmt.Sprintf("helix-subagent-testhelper-%d", os.Getpid()))
	build := exec.Command("go", "build", "-o", binPath, "./testhelper")
	build.Stderr = os.Stderr
	build.Stdout = os.Stdout
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: failed to build testhelper:", err)
		os.Exit(2)
	}
	testHelperBinary = binPath
	code := m.Run()
	_ = os.Remove(binPath)
	os.Exit(code)
}

// recordingRunner returns a Runner stub that records the args / env / dir
// passed to it and returns the supplied stdout/stderr/exitCode/err. The
// recorded values are exposed via the *recorder's exported fields after the
// call returns.
type recorder struct {
	gotName     string
	gotArgs     []string
	gotEnv      []string
	gotDir      string
	wasInvoked  bool
}

func (r *recorder) makeRunner(stdout, stderr []byte, exitCode int, err error) func(ctx context.Context, name string, args []string, env []string, dir string) ([]byte, []byte, int, error) {
	return func(ctx context.Context, name string, args []string, env []string, dir string) ([]byte, []byte, int, error) {
		r.wasInvoked = true
		r.gotName = name
		r.gotArgs = append([]string(nil), args...)
		r.gotEnv = append([]string(nil), env...)
		r.gotDir = dir
		return stdout, stderr, exitCode, err
	}
}

func TestSubprocessSpawner_Kind(t *testing.T) {
	s, err := NewSubprocessSpawner("")
	if err != nil {
		t.Fatalf("NewSubprocessSpawner: %v", err)
	}
	if got := s.Kind(); got != "subprocess" {
		t.Fatalf("Kind() = %q, want %q", got, "subprocess")
	}
}

func TestSubprocessSpawner_NewWithEmptyWorkDir(t *testing.T) {
	s, err := NewSubprocessSpawner("")
	if err != nil {
		t.Fatalf("NewSubprocessSpawner with empty workDir returned error: %v", err)
	}
	if s == nil {
		t.Fatalf("NewSubprocessSpawner returned nil spawner with empty workDir")
	}
	if s.HostBinary == "" {
		t.Fatalf("HostBinary should be populated from os.Executable()")
	}
}

func TestSubprocessSpawner_BuildsEnvWithSentinel(t *testing.T) {
	rec := &recorder{}
	// Stub stdout: a valid SubagentResult JSON so parsing succeeds and
	// the test can focus on the env we passed.
	resJSON, _ := json.Marshal(SubagentResult{
		TaskID: "t-env",
		State:  StateSucceeded,
	})
	s := &SubprocessSpawner{
		HostBinary: "/dev/null/helixcode-stub",
		Runner:     rec.makeRunner(resJSON, nil, 0, nil),
	}

	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "t-env",
		Prompt: "env-test",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	res := drainOne(t, ch, 2*time.Second)
	if res.State != StateSucceeded {
		t.Fatalf("expected StateSucceeded, got %q (err=%q)", res.State, res.Error)
	}

	if !rec.wasInvoked {
		t.Fatalf("Runner was never invoked")
	}
	if rec.gotName != "/dev/null/helixcode-stub" {
		t.Errorf("Runner name = %q, want HostBinary path", rec.gotName)
	}

	hasMarker := false
	hasPayload := false
	hasNoRecurse := false
	for _, e := range rec.gotEnv {
		if e == SubagentHelperEnvVar+"=1" {
			hasMarker = true
		}
		if strings.HasPrefix(e, SubagentHelperPayloadEnvVar+"=") {
			hasPayload = true
		}
		if e == SubagentRecursionEnvVar+"=1" {
			hasNoRecurse = true
		}
	}
	if !hasMarker {
		t.Errorf("env missing %s=1 marker; env=%v", SubagentHelperEnvVar, rec.gotEnv)
	}
	if !hasPayload {
		t.Errorf("env missing %s payload; env=%v", SubagentHelperPayloadEnvVar, rec.gotEnv)
	}
	if !hasNoRecurse {
		t.Errorf("env missing %s=1 recursion guard; env=%v", SubagentRecursionEnvVar, rec.gotEnv)
	}
}

func TestSubprocessSpawner_PayloadMarshalsCorrectly(t *testing.T) {
	rec := &recorder{}
	resJSON, _ := json.Marshal(SubagentResult{TaskID: "t-pl", State: StateSucceeded})
	s := &SubprocessSpawner{
		HostBinary: "/dev/null/helixcode-stub",
		Runner:     rec.makeRunner(resJSON, nil, 0, nil),
	}

	want := SubagentTask{
		ID:           "t-pl",
		Description:  "marshal-check",
		Prompt:       "round-trip-prompt",
		Isolation:    IsolationNone,
		SubagentType: "general-purpose",
		Timeout:      30 * time.Second,
	}
	ch, err := s.Spawn(context.Background(), want, nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	drainOne(t, ch, 2*time.Second)

	var got SubagentTask
	for _, e := range rec.gotEnv {
		if strings.HasPrefix(e, SubagentHelperPayloadEnvVar+"=") {
			raw := strings.TrimPrefix(e, SubagentHelperPayloadEnvVar+"=")
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatalf("payload not JSON: %v (raw=%q)", err, raw)
			}
			break
		}
	}
	if got.ID != want.ID || got.Prompt != want.Prompt || got.Description != want.Description {
		t.Errorf("payload mismatch:\n got=%+v\nwant=%+v", got, want)
	}
	if got.Isolation != want.Isolation {
		t.Errorf("payload Isolation = %q, want %q", got.Isolation, want.Isolation)
	}
	if got.SubagentType != want.SubagentType {
		t.Errorf("payload SubagentType = %q, want %q", got.SubagentType, want.SubagentType)
	}
}

func TestSubprocessSpawner_RealHelper_RoundTrip(t *testing.T) {
	if testHelperBinary == "" {
		t.Fatalf("testHelperBinary not initialized; TestMain build failed?")
	}
	s := &SubprocessSpawner{
		HostBinary: testHelperBinary,
		Runner:     defaultSubprocessRunner,
	}

	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "t-rt",
		Prompt: "round-trip-test",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	res := drainOne(t, ch, 10*time.Second)

	if res.State != StateSucceeded {
		t.Fatalf("expected StateSucceeded, got %q (err=%q, output=%q)", res.State, res.Error, res.Output)
	}
	if res.Output != "helper-handled: round-trip-test" {
		t.Fatalf("expected real-helper output, got %q", res.Output)
	}
	if res.TaskID != "t-rt" {
		t.Fatalf("expected TaskID=t-rt, got %q", res.TaskID)
	}
}

func TestSubprocessSpawner_HelperFailsExitNonZero(t *testing.T) {
	if testHelperBinary == "" {
		t.Fatalf("testHelperBinary not initialized")
	}
	s := &SubprocessSpawner{
		HostBinary: testHelperBinary,
		Runner: func(ctx context.Context, name string, args []string, env []string, dir string) ([]byte, []byte, int, error) {
			env = append(env, "HELIXCODE_TEST_HELPER_BEHAVIOR=fail")
			return defaultSubprocessRunner(ctx, name, args, env, dir)
		},
	}

	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "t-fail",
		Prompt: "fail-me",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	res := drainOne(t, ch, 10*time.Second)

	if res.State != StateFailed {
		t.Fatalf("expected StateFailed, got %q (err=%q)", res.State, res.Error)
	}
	if !strings.Contains(res.Error, "forced failure") {
		t.Errorf("expected Error to include helper stderr 'forced failure', got %q", res.Error)
	}
}

func TestSubprocessSpawner_HelperInvalidJSON(t *testing.T) {
	if testHelperBinary == "" {
		t.Fatalf("testHelperBinary not initialized")
	}
	s := &SubprocessSpawner{
		HostBinary: testHelperBinary,
		Runner: func(ctx context.Context, name string, args []string, env []string, dir string) ([]byte, []byte, int, error) {
			env = append(env, "HELIXCODE_TEST_HELPER_BEHAVIOR=invalid-json")
			return defaultSubprocessRunner(ctx, name, args, env, dir)
		},
	}

	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "t-inv",
		Prompt: "anything",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	res := drainOne(t, ch, 10*time.Second)

	if res.State != StateFailed {
		t.Fatalf("expected StateFailed, got %q (output=%q)", res.State, res.Output)
	}
	if !strings.Contains(res.Error, "invalid helper output") {
		t.Errorf("expected Error to mention 'invalid helper output', got %q", res.Error)
	}
}

func TestSubprocessSpawner_TimeoutKillsChild(t *testing.T) {
	if testHelperBinary == "" {
		t.Fatalf("testHelperBinary not initialized")
	}
	s := &SubprocessSpawner{
		HostBinary: testHelperBinary,
		Runner: func(ctx context.Context, name string, args []string, env []string, dir string) ([]byte, []byte, int, error) {
			env = append(env, "HELIXCODE_TEST_HELPER_BEHAVIOR=slow")
			return defaultSubprocessRunner(ctx, name, args, env, dir)
		},
	}

	start := time.Now()
	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:      "t-to",
		Prompt:  "slow",
		Timeout: 200 * time.Millisecond,
	}, nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	res := drainOne(t, ch, 5*time.Second)
	elapsed := time.Since(start)

	if elapsed > 4*time.Second {
		t.Fatalf("timeout was not enforced (elapsed=%v); helper was not actually killed", elapsed)
	}
	if res.State != StateTimedOut {
		t.Fatalf("expected StateTimedOut, got %q (err=%q)", res.State, res.Error)
	}
}

func TestSubprocessSpawner_CtxCancelKillsChild(t *testing.T) {
	if testHelperBinary == "" {
		t.Fatalf("testHelperBinary not initialized")
	}
	s := &SubprocessSpawner{
		HostBinary: testHelperBinary,
		Runner: func(ctx context.Context, name string, args []string, env []string, dir string) ([]byte, []byte, int, error) {
			env = append(env, "HELIXCODE_TEST_HELPER_BEHAVIOR=slow")
			return defaultSubprocessRunner(ctx, name, args, env, dir)
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.Spawn(ctx, SubagentTask{
		ID:     "t-cnl",
		Prompt: "slow",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	// Give the child time to start, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	res := drainOne(t, ch, 5*time.Second)
	if res.State != StateCanceled && res.State != StateTimedOut {
		t.Fatalf("expected StateCanceled or StateTimedOut, got %q (err=%q)", res.State, res.Error)
	}
}

func TestSubprocessSpawner_ChannelClosesAfterResult(t *testing.T) {
	if testHelperBinary == "" {
		t.Fatalf("testHelperBinary not initialized")
	}
	s := &SubprocessSpawner{
		HostBinary: testHelperBinary,
		Runner:     defaultSubprocessRunner,
	}
	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "t-cls",
		Prompt: "x",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	first, ok := <-ch
	if !ok {
		t.Fatalf("expected first receive to succeed")
	}
	if first.State != StateSucceeded {
		t.Fatalf("expected StateSucceeded, got %q (err=%q)", first.State, first.Error)
	}
	second, ok := <-ch
	if ok {
		t.Fatalf("expected channel closed after first result, got value=%+v", second)
	}
}

func TestSubprocessSpawner_DurationPopulated(t *testing.T) {
	if testHelperBinary == "" {
		t.Fatalf("testHelperBinary not initialized")
	}
	s := &SubprocessSpawner{
		HostBinary: testHelperBinary,
		Runner:     defaultSubprocessRunner,
	}
	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "t-dur",
		Prompt: "x",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	res := drainOne(t, ch, 10*time.Second)
	if res.Duration <= 0 {
		t.Fatalf("expected positive Duration, got %v", res.Duration)
	}
}

func TestSubprocessSpawner_NilProviderIsAccepted(t *testing.T) {
	// SubprocessSpawner deliberately ignores its llm.Provider arg — the
	// child process constructs its own provider via T07/T08 wiring. Passing
	// nil therefore MUST NOT error.
	rec := &recorder{}
	resJSON, _ := json.Marshal(SubagentResult{TaskID: "t-np", State: StateSucceeded})
	s := &SubprocessSpawner{
		HostBinary: "/dev/null/helixcode-stub",
		Runner:     rec.makeRunner(resJSON, nil, 0, nil),
	}
	ch, err := s.Spawn(context.Background(), SubagentTask{
		ID:     "t-np",
		Prompt: "x",
	}, nil) // <-- nil provider
	if err != nil {
		t.Fatalf("Spawn with nil provider returned error: %v (must be accepted, see doc)", err)
	}
	res := drainOne(t, ch, 2*time.Second)
	if res.State != StateSucceeded {
		t.Fatalf("expected StateSucceeded, got %q", res.State)
	}
}

// Helper for tests below: read raw stdout bytes to sanity-check that the JSON
// round-trip flows through real OS pipes.
func TestSubprocessSpawner_RealHelper_RawStdoutIsJSON(t *testing.T) {
	if testHelperBinary == "" {
		t.Fatalf("testHelperBinary not initialized")
	}
	// Run the helper directly (bypassing SubprocessSpawner) to confirm the
	// helper itself emits valid JSON. This pins the protocol contract.
	cmd := exec.Command(testHelperBinary)
	cmd.Env = append(os.Environ(),
		SubagentHelperEnvVar+"=1",
		SubagentHelperPayloadEnvVar+`={"id":"t-direct","prompt":"hello-direct"}`,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("direct helper run failed: %v (stderr=%q)", err, stderr.String())
	}
	var res SubagentResult
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("helper stdout not valid JSON: %v (stdout=%q)", err, stdout.String())
	}
	if res.State != StateSucceeded {
		t.Fatalf("expected StateSucceeded, got %q", res.State)
	}
	if res.Output != "helper-handled: hello-direct" {
		t.Fatalf("expected output 'helper-handled: hello-direct', got %q", res.Output)
	}
}
