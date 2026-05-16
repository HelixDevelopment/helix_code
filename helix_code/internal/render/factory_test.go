// Tests for the RendererFactory (P1-F18-T06).
//
// The factory's job is precedence resolution:
//
//   1. opts.Mode (ModeFancy / ModePlain) overrides everything else.
//   2. opts.EnvLookup(EnvVarName) — "fancy" / "plain" honoured exactly;
//      "auto" or empty means "fall through to TTY detection".
//   3. opts.IsTTY() — true => fancy, false => plain.
//   4. Default: ModePlain.
//
// Garbage env values are treated as a configuration mistake: the factory
// emits a warning to stderr and falls back to plain (the safe default for
// non-TTY destinations).
package render

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

// captureStderr redirects os.Stderr for the duration of fn and returns the
// bytes that were written. Used to assert the warning emitted on garbage
// env values.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stderr
	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = wPipe

	var buf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&buf, rPipe)
	}()

	fn()

	_ = wPipe.Close()
	wg.Wait()
	os.Stderr = original
	return buf.String()
}

// staticEnv returns an EnvLookup that always returns value for EnvVarName
// and "" for any other key.
func staticEnv(value string) func(string) string {
	return func(key string) string {
		if key == EnvVarName {
			return value
		}
		return ""
	}
}

// staticTTY returns an IsTTY closure that always reports the given bool.
func staticTTY(b bool) func() bool { return func() bool { return b } }

func TestNewRenderer_OptsModeFancy(t *testing.T) {
	var buf bytes.Buffer
	r, err := NewRenderer(FactoryOptions{
		Writer:    &buf,
		Mode:      ModeFancy,
		EnvLookup: staticEnv("plain"), // would say plain — Mode wins.
		IsTTY:     staticTTY(false),   // would say plain — Mode wins.
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Mode(); got != ModeFancy {
		t.Fatalf("Mode() = %q, want %q", got, ModeFancy)
	}
}

func TestNewRenderer_OptsModePlain(t *testing.T) {
	var buf bytes.Buffer
	r, err := NewRenderer(FactoryOptions{
		Writer:    &buf,
		Mode:      ModePlain,
		EnvLookup: staticEnv("fancy"),
		IsTTY:     staticTTY(true),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Mode(); got != ModePlain {
		t.Fatalf("Mode() = %q, want %q", got, ModePlain)
	}
}

func TestNewRenderer_EnvLookupFancy(t *testing.T) {
	var buf bytes.Buffer
	r, err := NewRenderer(FactoryOptions{
		Writer:    &buf,
		Mode:      ModeAuto,
		EnvLookup: staticEnv("fancy"),
		IsTTY:     staticTTY(false), // would say plain — env overrides.
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Mode(); got != ModeFancy {
		t.Fatalf("Mode() = %q, want %q", got, ModeFancy)
	}
}

func TestNewRenderer_EnvLookupPlain(t *testing.T) {
	var buf bytes.Buffer
	r, err := NewRenderer(FactoryOptions{
		Writer:    &buf,
		Mode:      ModeAuto,
		EnvLookup: staticEnv("plain"),
		IsTTY:     staticTTY(true), // would say fancy — env overrides.
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Mode(); got != ModePlain {
		t.Fatalf("Mode() = %q, want %q", got, ModePlain)
	}
}

func TestNewRenderer_EnvAuto_TTYTrue(t *testing.T) {
	var buf bytes.Buffer
	r, err := NewRenderer(FactoryOptions{
		Writer:    &buf,
		Mode:      ModeAuto,
		EnvLookup: staticEnv("auto"),
		IsTTY:     staticTTY(true),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Mode(); got != ModeFancy {
		t.Fatalf("Mode() = %q, want %q", got, ModeFancy)
	}
}

func TestNewRenderer_EnvAuto_TTYFalse(t *testing.T) {
	var buf bytes.Buffer
	r, err := NewRenderer(FactoryOptions{
		Writer:    &buf,
		Mode:      ModeAuto,
		EnvLookup: staticEnv("auto"),
		IsTTY:     staticTTY(false),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Mode(); got != ModePlain {
		t.Fatalf("Mode() = %q, want %q", got, ModePlain)
	}
}

func TestNewRenderer_EnvUnset_TTYTrue(t *testing.T) {
	var buf bytes.Buffer
	r, err := NewRenderer(FactoryOptions{
		Writer:    &buf,
		Mode:      ModeAuto,
		EnvLookup: staticEnv(""), // unset.
		IsTTY:     staticTTY(true),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Mode(); got != ModeFancy {
		t.Fatalf("Mode() = %q, want %q", got, ModeFancy)
	}
}

func TestNewRenderer_EnvUnset_TTYFalse(t *testing.T) {
	var buf bytes.Buffer
	r, err := NewRenderer(FactoryOptions{
		Writer:    &buf,
		Mode:      ModeAuto,
		EnvLookup: staticEnv(""),
		IsTTY:     staticTTY(false),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Mode(); got != ModePlain {
		t.Fatalf("Mode() = %q, want %q", got, ModePlain)
	}
}

func TestNewRenderer_GarbageEnvFallsBackToPlain(t *testing.T) {
	var buf bytes.Buffer
	var r Renderer
	var err error
	stderr := captureStderr(t, func() {
		r, err = NewRenderer(FactoryOptions{
			Writer:    &buf,
			Mode:      ModeAuto,
			EnvLookup: staticEnv("Fancy"), // wrong case — exact match required.
			IsTTY:     staticTTY(true),
		})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Mode(); got != ModePlain {
		t.Fatalf("Mode() = %q, want %q (garbage env -> plain)", got, ModePlain)
	}
	if !strings.Contains(stderr, EnvVarName) {
		t.Fatalf("stderr warning must mention env var name; got %q", stderr)
	}
	if !strings.Contains(stderr, "Fancy") {
		t.Fatalf("stderr warning must echo the offending value; got %q", stderr)
	}
}

func TestNewRenderer_FancyOnNonTTY_AllowedWhenExplicit(t *testing.T) {
	// Operator pipes the binary into a file but explicitly asks for fancy
	// (e.g. `helixcode --render fancy > out.log` so they can replay later).
	// The factory must honour Mode=ModeFancy even when IsTTY says false.
	var buf bytes.Buffer
	r, err := NewRenderer(FactoryOptions{
		Writer:    &buf,
		Mode:      ModeFancy,
		EnvLookup: staticEnv(""),
		IsTTY:     staticTTY(false),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Mode(); got != ModeFancy {
		t.Fatalf("Mode() = %q, want %q (explicit fancy beats TTY signal)", got, ModeFancy)
	}
}

func TestNewRenderer_DefaultsAppliedToNilOpts(t *testing.T) {
	// Bare zero-value FactoryOptions: nil Writer, empty Mode, nil IsTTY,
	// nil EnvLookup. Must not panic and must produce a usable Renderer.
	r, err := NewRenderer(FactoryOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("got nil Renderer")
	}
	mode := r.Mode()
	if mode != ModeFancy && mode != ModePlain {
		t.Fatalf("Mode() = %q, want fancy or plain", mode)
	}
	// Close should not panic on the defaulted writer (os.Stdout).
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestResolveMode_PrecedenceTable(t *testing.T) {
	cases := []struct {
		name     string
		envValue string
		isTTY    bool
		want     RenderMode
		wantErr  bool
	}{
		{name: "env=fancy_tty=true", envValue: "fancy", isTTY: true, want: ModeFancy},
		{name: "env=fancy_tty=false", envValue: "fancy", isTTY: false, want: ModeFancy},
		{name: "env=plain_tty=true", envValue: "plain", isTTY: true, want: ModePlain},
		{name: "env=plain_tty=false", envValue: "plain", isTTY: false, want: ModePlain},
		{name: "env=auto_tty=true", envValue: "auto", isTTY: true, want: ModeFancy},
		{name: "env=auto_tty=false", envValue: "auto", isTTY: false, want: ModePlain},
		{name: "env=empty_tty=true", envValue: "", isTTY: true, want: ModeFancy},
		{name: "env=empty_tty=false", envValue: "", isTTY: false, want: ModePlain},
		{name: "env=garbage_tty=true", envValue: "Fancy", isTTY: true, want: ModePlain, wantErr: true},
		{name: "env=garbage_tty=false", envValue: "rich", isTTY: false, want: ModePlain, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveMode(tc.envValue, tc.isTTY)
			if got != tc.want {
				t.Fatalf("resolveMode(%q, %v) mode = %q, want %q", tc.envValue, tc.isTTY, got, tc.want)
			}
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveMode(%q, %v) expected error, got nil", tc.envValue, tc.isTTY)
				}
				if !errors.Is(err, ErrInvalidMode) {
					t.Fatalf("resolveMode error = %v, want ErrInvalidMode", err)
				}
			} else if err != nil {
				t.Fatalf("resolveMode(%q, %v) unexpected error: %v", tc.envValue, tc.isTTY, err)
			}
		})
	}
}

func TestDetectTTY_BytesBufferReturnsFalse(t *testing.T) {
	var buf bytes.Buffer
	if detectTTY(&buf) {
		t.Fatal("detectTTY(*bytes.Buffer) = true, want false")
	}
}

func TestDetectTTY_RealOSStdout(t *testing.T) {
	// Either result is valid: "go test" usually runs without a TTY (false),
	// but a developer running `go test ./...` from a terminal MAY see true.
	// The contract is "does not panic" and "returns a real bool".
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("detectTTY(os.Stdout) panicked: %v", r)
		}
	}()
	got := detectTTY(os.Stdout)
	_ = got // either bool is acceptable.
}

// Ensure the factory error path reports the right env-var name in errors so
// operators can debug misconfiguration without reading source.
func TestNewRenderer_ErrorIncludesEnvVarName(t *testing.T) {
	stderr := captureStderr(t, func() {
		_, _ = NewRenderer(FactoryOptions{
			Writer:    io.Discard,
			Mode:      ModeAuto,
			EnvLookup: staticEnv("nonsense"),
			IsTTY:     staticTTY(false),
		})
	})
	if !strings.Contains(stderr, EnvVarName) {
		t.Fatalf("warning missing env var name %q: got %q", EnvVarName, stderr)
	}
}

// Sanity: sentinel format ensures fmt'ing the error doesn't lose context.
func TestResolveMode_GarbageErrorWraps(t *testing.T) {
	_, err := resolveMode("rich", true)
	if err == nil {
		t.Fatal("expected error")
	}
	wrapped := fmt.Errorf("factory: %w", err)
	if !errors.Is(wrapped, ErrInvalidMode) {
		t.Fatalf("wrapped error does not satisfy errors.Is(_, ErrInvalidMode): %v", wrapped)
	}
}
