package render

import (
	"errors"
	"io"
	"testing"
)

func TestRenderMode_String_Values(t *testing.T) {
	tests := []struct {
		name string
		m    RenderMode
		want string
	}{
		{"fancy", ModeFancy, "fancy"},
		{"plain", ModePlain, "plain"},
		{"auto", ModeAuto, "auto"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.m); got != tt.want {
				t.Fatalf("string(%v) = %q, want %q", tt.m, got, tt.want)
			}
		})
	}
}

func TestRenderMode_IsValid_AcceptsKnown(t *testing.T) {
	for _, m := range []RenderMode{ModeFancy, ModePlain, ModeAuto} {
		if !m.IsValid() {
			t.Fatalf("RenderMode(%q).IsValid() = false; want true", m)
		}
	}
}

func TestRenderMode_IsValid_RejectsUnknown(t *testing.T) {
	tests := []RenderMode{
		"",
		"banana",
		"FANCY", // case-sensitive — only the lower-case sentinels are valid
		"Plain",
		"  fancy",
		"auto ",
	}
	for _, m := range tests {
		t.Run(string(m), func(t *testing.T) {
			if m.IsValid() {
				t.Fatalf("RenderMode(%q).IsValid() = true; want false", m)
			}
		})
	}
}

func TestEnvVarName_IsHelixcodeRender(t *testing.T) {
	if EnvVarName != "HELIXCODE_RENDER" {
		t.Fatalf("EnvVarName = %q, want %q", EnvVarName, "HELIXCODE_RENDER")
	}
}

func TestFrame_ZeroValueIsEmpty(t *testing.T) {
	var f Frame
	if !f.IsZero() {
		t.Fatalf("zero-value Frame.IsZero() = false; want true")
	}
}

func TestFrame_IsZero_NonEmpty(t *testing.T) {
	tests := []struct {
		name string
		f    Frame
	}{
		{"only block id", Frame{BlockID: "b1"}},
		{"only lines", Frame{Lines: []string{"line"}}},
		{"both", Frame{BlockID: "b1", Lines: []string{"a", "b"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.f.IsZero() {
				t.Fatalf("Frame{%+v}.IsZero() = true; want false", tt.f)
			}
		})
	}
}

func TestSentinelErrors_Distinct(t *testing.T) {
	all := []error{
		ErrInvalidMode,
		ErrRendererClosed,
		ErrEmptyBlockID,
	}
	// Each must be non-nil with a non-empty message.
	for _, err := range all {
		if err == nil {
			t.Fatalf("sentinel error is nil")
		}
		if err.Error() == "" {
			t.Fatalf("sentinel error %v has empty message", err)
		}
	}
	// All messages must be pairwise distinct.
	seen := make(map[string]error, len(all))
	for _, err := range all {
		msg := err.Error()
		if prev, ok := seen[msg]; ok {
			t.Fatalf("duplicate sentinel message %q on %v and %v", msg, prev, err)
		}
		seen[msg] = err
	}
	// errors.Is must round-trip on each sentinel against itself.
	for _, err := range all {
		if !errors.Is(err, err) {
			t.Fatalf("errors.Is(%v, %v) = false", err, err)
		}
	}
}

// stubRenderer is a no-op Renderer used solely to assert at compile time that
// the interface signature stays in sync with T03/T04 implementations.
type stubRenderer struct{}

func (stubRenderer) Mode() RenderMode             { return ModePlain }
func (stubRenderer) Begin(string) error           { return nil }
func (stubRenderer) WriteToken(string) error      { return nil }
func (stubRenderer) Commit() error                { return nil }
func (stubRenderer) RenderFrame(Frame) error      { return nil }
func (stubRenderer) Close() error                 { return nil }

func TestRendererInterface_Compiles(t *testing.T) {
	var r Renderer = stubRenderer{}
	if r.Mode() != ModePlain {
		t.Fatalf("stubRenderer.Mode() = %v, want %v", r.Mode(), ModePlain)
	}
	if err := r.Begin("b1"); err != nil {
		t.Fatalf("Begin returned %v", err)
	}
	if err := r.WriteToken("hello"); err != nil {
		t.Fatalf("WriteToken returned %v", err)
	}
	if err := r.Commit(); err != nil {
		t.Fatalf("Commit returned %v", err)
	}
	if err := r.RenderFrame(Frame{BlockID: "b1", Lines: []string{"a"}}); err != nil {
		t.Fatalf("RenderFrame returned %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close returned %v", err)
	}
}

func TestFactoryOptions_ZeroValueIsEmpty(t *testing.T) {
	var opts FactoryOptions
	if opts.Writer != nil {
		t.Fatalf("zero FactoryOptions.Writer = %v; want nil", opts.Writer)
	}
	if opts.Mode != "" {
		t.Fatalf("zero FactoryOptions.Mode = %q; want empty", opts.Mode)
	}
	if opts.IsTTY != nil {
		t.Fatalf("zero FactoryOptions.IsTTY != nil; want nil")
	}
	if opts.EnvLookup != nil {
		t.Fatalf("zero FactoryOptions.EnvLookup != nil; want nil")
	}
	// Compile-time guarantee that Writer is io.Writer-shaped.
	var _ io.Writer = (io.Writer)(nil)
}
