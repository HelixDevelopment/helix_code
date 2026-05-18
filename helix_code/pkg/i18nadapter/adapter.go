// Package i18nadapter bridges helix_code's pkg/i18n Localizer to the
// minimal Translator contracts that owned submodules declare in their
// OWN packages per CONST-051(B) (submodule decoupling). A consuming
// submodule's interface looks like:
//
//	type Translator interface {
//	    T(ctx context.Context, id string, data map[string]any) (string, error)
//	    TPlural(ctx context.Context, id string, count int, data map[string]any) (string, error)
//	}
//
// This adapter's *Translator structurally satisfies that contract.
// Submodules import nothing from helix_code; helix_code constructs the
// localizer + adapter at boot and injects via the submodule's
// constructor. The submodule remains project-not-aware and
// standalone-reusable.
package i18nadapter

import (
	"context"

	"dev.helix.code/pkg/i18n"
)

// Translator wraps a *i18n.Localizer so it can be injected into any
// owned submodule's locally-declared Translator interface (matching
// method set: T(ctx, id, data), TPlural(ctx, id, count, data)).
//
// Construction is cheap; one Translator per (Localizer, request-locale)
// pair is the typical usage. Translator is safe for concurrent use to
// the same extent the underlying *i18n.Localizer is (read-only after
// construction).
type Translator struct {
	loc *i18n.Localizer
}

// New wraps the given Localizer. Panics if loc is nil — a nil
// Localizer would silently swallow every translation request without
// surfacing the misconfiguration, which is a §11.4 PASS-bluff at the
// adapter layer.
func New(loc *i18n.Localizer) *Translator {
	if loc == nil {
		panic("i18nadapter: nil *i18n.Localizer passed to New; localizer MUST be constructed via i18n.NewLocalizer before adapter creation")
	}
	return &Translator{loc: loc}
}

// T resolves messageID through the wrapped Localizer. The ctx
// parameter is accepted for contract compatibility with the consumer's
// Translator interface but is not currently propagated to go-i18n
// (which has no context-aware Localize call). When go-i18n grows
// context support, this adapter MUST be tightened to honour it.
func (t *Translator) T(_ context.Context, id string, data map[string]any) (string, error) {
	if data == nil {
		return t.loc.T(id)
	}
	return t.loc.T(id, data)
}

// TPlural resolves messageID with CLDR plural selection through the
// wrapped Localizer. Same ctx-propagation note as T.
func (t *Translator) TPlural(_ context.Context, id string, count int, data map[string]any) (string, error) {
	if data == nil {
		return t.loc.TPlural(id, count)
	}
	return t.loc.TPlural(id, count, data)
}
