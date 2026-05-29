// Package i18nwiring is the central boot-time CONST-046 translator wiring
// for helix_code (HXC-036, Option A — boot-time wiring via a central
// WireTranslators helper, 2026-05-29).
//
// Background / why this package exists
// ------------------------------------
// The CONST-046 i18n migration (rounds 93..440) gave 74 internal packages
// each their OWN per-package DI seam: a local Translator interface, a
// NoopTranslator{} loud-echo default, a package-level SetTranslator(tr),
// and a bundles/active.en.yaml catalogue. That migration deliberately kept
// the packages project-NOT-aware (CONST-051(B) decoupling): a package
// declares the contract and a default, but is injected with a real
// translator from OUTSIDE.
//
// The boot-time injection was never written. `grep -rn '\.SetTranslator('`
// (non-test) across helix_code returned 0 call sites, so every package ran
// with NoopTranslator{} and users saw raw message-ID keys
// (e.g. "askuser_prompt_invalid_choice_hint") instead of resolved +
// interpolated text. This package is the missing wiring: it constructs a
// real *i18nadapter.Translator per package (from that package's embedded
// active.en.yaml) and injects it via the package's SetTranslator.
//
// WireAll() is called once at every real entry point that exercises a
// CONST-046-migrated prompter/flow, BEFORE any user-facing string is
// emitted (see cmd/cli buildSubsystems). It is safe to call more than once
// (each SetTranslator simply re-injects an equivalent translator) and is
// safe to call from tests that need the production render path.
//
// Per-package onboarding recipe (Phase 2 — the remaining 72 packages)
// -------------------------------------------------------------------
// Each migrated package already ships its own Translator interface,
// NoopTranslator default, SetTranslator(tr), and i18n/bundles/active.<lang>.yaml.
// To wire one in:
//
//  1. Add a constructor to the package's i18n subpackage (copy
//     internal/tools/askuser/i18n/bundle.go verbatim, changing only the
//     package doc + the error-prefix string):
//
//       //go:embed bundles/active.en.yaml
//       var activeBundleFS embed.FS
//       const activeBundlePath = "bundles/active.en.yaml"
//       func NewTranslator(langs ...string) (Translator, error) { ... }
//
//     The constructor body is identical for every package because the
//     pkg/i18n + pkg/i18nadapter APIs are package-agnostic.
//
//  2. Add the package + its i18n subpackage to this file's import block
//     (aliased, e.g. `fooi18n "dev.helix.code/internal/foo/i18n"`), then add
//     one block to WireAll below, copied from the askuser block verbatim:
//
//       if tr, err := fooi18n.NewTranslator(langs...); err != nil {
//           errs = append(errs, fmt.Errorf("foo: %w", err))
//       } else {
//           foo.SetTranslator(tr)
//       }
//
//     The explicit per-package block keeps the concrete Translator types
//     statically checked (no any-boxing / runtime type assertions) — each
//     SetTranslator's parameter type is verified at compile time.
//
// A failed NewTranslator (corrupt/missing embedded bundle) is collected and
// returned as a joined error — WireAll never silently leaves a package on
// NoopTranslator{}, which would be a §11.4 PASS-bluff at the i18n layer.
package i18nwiring

import (
	"errors"
	"fmt"

	"dev.helix.code/internal/approval"
	approvali18n "dev.helix.code/internal/approval/i18n"
	"dev.helix.code/internal/tools/askuser"
	askuseri18n "dev.helix.code/internal/tools/askuser/i18n"
)

// WireAll constructs a real translator for every CONST-046-migrated
// package wired so far and injects it via that package's SetTranslator.
// langs is the ordered accept-language preference chain forwarded to every
// package's NewTranslator (empty → en). It returns a joined error
// enumerating every package whose translator failed to construct; on
// success it returns nil and every wired package renders real interpolated
// text instead of raw message-ID keys.
//
// Phase 1 wires internal/tools/askuser + internal/approval. The remaining
// 72 packages slot in by adding one wire(...) entry each (see the package
// doc recipe).
func WireAll(langs ...string) error {
	var errs []error

	// internal/tools/askuser — numbered-choice CLI prompt narrative.
	if tr, err := askuseri18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("askuser: %w", err))
	} else {
		askuser.SetTranslator(tr)
	}

	// internal/approval — approval-gate prompt + mode-description narrative.
	if tr, err := approvali18n.NewTranslator(langs...); err != nil {
		errs = append(errs, fmt.Errorf("approval: %w", err))
	} else {
		approval.SetTranslator(tr)
	}

	if len(errs) > 0 {
		return fmt.Errorf("i18nwiring.WireAll: %w", errors.Join(errs...))
	}
	return nil
}

// MustWireAll is the panic-on-error variant for entry points where a failed
// translator build is unrecoverable (a binary that cannot localize its
// prompts must not silently boot into raw-key-echo mode). Boot code that
// prefers to log-and-degrade should call WireAll and handle the error.
func MustWireAll(langs ...string) {
	if err := WireAll(langs...); err != nil {
		panic(err)
	}
}
