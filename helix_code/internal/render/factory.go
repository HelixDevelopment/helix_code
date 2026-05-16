// RendererFactory (P1-F18-T06).
//
// NewRenderer is the single entry point that turns a FactoryOptions struct
// into either an ANSI fancy renderer (T03) or a plain renderer (T04). The
// precedence ladder, in priority order:
//
//  1. opts.Mode (ModeFancy / ModePlain) overrides everything else. ModeAuto
//     and the empty string fall through.
//  2. opts.EnvLookup(EnvVarName) — "fancy" / "plain" honoured exactly;
//     "auto" or empty fall through to the next rung.
//  3. opts.IsTTY() — true => fancy, false => plain.
//  4. Default: ModePlain.
//
// Garbage env values (anything other than fancy/plain/auto/"") are treated
// as a configuration mistake: a one-line warning is written to os.Stderr
// (mentioning the env-var name and the offending value) and the factory
// falls back to ModePlain (the safe choice for non-TTY destinations and
// the choice that avoids leaking ANSI escape codes into log files).
//
// All TTY probing goes through golang.org/x/term — promoted from indirect
// to direct in go.mod by this task. detectTTY is the single chokepoint so
// tests can stub it out via the IsTTY closure on FactoryOptions.
package render

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// NewRenderer constructs a Renderer per opts. See package doc above for the
// precedence rules. The returned Renderer is always either *ansiRenderer or
// *plainRenderer; never nil unless err != nil.
func NewRenderer(opts FactoryOptions) (Renderer, error) {
	// Apply defaults to the zero-value options. Each field has a sane
	// fallback so that NewRenderer(FactoryOptions{}) is a usable call.
	if opts.Writer == nil {
		opts.Writer = os.Stdout
	}
	if opts.EnvLookup == nil {
		opts.EnvLookup = os.Getenv
	}
	if opts.IsTTY == nil {
		w := opts.Writer
		opts.IsTTY = func() bool { return detectTTY(w) }
	}
	if opts.Mode == "" {
		opts.Mode = ModeAuto
	}

	// Rung 1: explicit Mode on opts beats env and TTY probe.
	switch opts.Mode {
	case ModeFancy:
		return NewANSIRenderer(opts.Writer), nil
	case ModePlain:
		return NewPlainRenderer(opts.Writer), nil
	case ModeAuto:
		// fall through to env + TTY resolution.
	default:
		// An invalid mode at the API surface is a programmer error, not
		// a misconfiguration: surface it as ErrInvalidMode rather than
		// silently downgrading.
		return nil, fmt.Errorf("render: opts.Mode %q: %w", opts.Mode, ErrInvalidMode)
	}

	// Rung 2 + 3: env var, then TTY.
	envValue := opts.EnvLookup(EnvVarName)
	mode, err := resolveMode(envValue, opts.IsTTY())
	if err != nil {
		// Garbage env value: warn and degrade to plain. Do NOT propagate
		// the error to the caller — operators should be able to start
		// the binary even if HELIXCODE_RENDER is mistyped, and plain is
		// the safe default for log files / CI.
		fmt.Fprintf(os.Stderr,
			"render: %s=%q is not one of fancy|plain|auto; falling back to plain\n",
			EnvVarName, envValue)
		mode = ModePlain
	}

	switch mode {
	case ModeFancy:
		return NewANSIRenderer(opts.Writer), nil
	case ModePlain:
		return NewPlainRenderer(opts.Writer), nil
	default:
		// resolveMode never returns ModeAuto — defensive guard only.
		return nil, fmt.Errorf("render: resolveMode returned %q: %w", mode, ErrInvalidMode)
	}
}

// resolveMode applies the env-var + TTY precedence rules and returns either
// ModeFancy or ModePlain. ModeAuto and "" delegate to the TTY probe; any
// other value returns ModePlain plus an ErrInvalidMode-wrapped error so the
// caller can decide whether to warn-and-degrade (NewRenderer) or hard-fail
// (table tests).
func resolveMode(envValue string, isTTY bool) (RenderMode, error) {
	switch envValue {
	case string(ModeFancy):
		return ModeFancy, nil
	case string(ModePlain):
		return ModePlain, nil
	case string(ModeAuto), "":
		if isTTY {
			return ModeFancy, nil
		}
		return ModePlain, nil
	default:
		return ModePlain, fmt.Errorf("render: %s=%q: %w", EnvVarName, envValue, ErrInvalidMode)
	}
}

// detectTTY reports whether w refers to a real terminal. Only *os.File
// instances can possibly be a TTY; everything else (bytes.Buffer, pipes,
// io.Discard, custom io.Writers) is unconditionally false. The probe goes
// through golang.org/x/term.IsTerminal which handles the OS-specific
// isatty(3) plumbing.
func detectTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
