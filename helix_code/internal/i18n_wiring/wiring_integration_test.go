// Package i18n_wiring contains integration tests proving the
// per-submodule i18n injection pattern works end-to-end:
//
//	helix_code/pkg/i18n (real go-i18n bundle)
//	  → helix_code/pkg/i18nadapter (Translator wrapper)
//	    → submodules/lazy/pkg/i18n.Translator interface
//	      → digital.vasic.lazy/pkg/lazy.Service.Describe()
//
// The submodule (Lazy) declares its OWN Translator interface and never
// imports anything from helix_code (CONST-051(B) decoupling).
// helix_code constructs the bundle + localizer + adapter and injects
// at submodule-construction time.
//
// Anti-bluff (CONST-035 / §11.4): every assertion below pins an EXACT
// expected string. A trivial passthrough (e.g. Lazy returning the raw
// message ID) would fail every test in this file.
package i18n_wiring

import (
	"context"
	"testing"

	"dev.helix.code/pkg/i18n"
	"dev.helix.code/pkg/i18nadapter"
	"golang.org/x/text/language"

	lazy "digital.vasic.lazy/pkg/lazy"
)

// enBundleYAML maps Lazy's well-known message IDs to actual
// English strings. In production deployment these would live in
// helix_code/i18n/messages/active.en.yaml (out of scope for round
// 93 — the bundle infrastructure is what's proven here).
const enBundleYAML = `lazy.service.uninitialized:
  other: "service is not yet initialized"
lazy.service.ready:
  other: "service is ready"
lazy.service.failed:
  other: "service initialization failed"
`

const srBundleYAML = `lazy.service.uninitialized:
  other: "servis još nije inicijalizovan"
lazy.service.ready:
  other: "servis je spreman"
lazy.service.failed:
  other: "inicijalizacija servisa neuspešna"
`

func newWiredTranslator(t *testing.T, langs ...string) *i18nadapter.Translator {
	t.Helper()
	b := i18n.NewBundle(language.English)
	b.MustParseMessageFileBytes([]byte(enBundleYAML), "active.en.yaml")
	b.MustParseMessageFileBytes([]byte(srBundleYAML), "active.sr.yaml")
	loc := i18n.NewLocalizer(b, langs...)
	return i18nadapter.New(loc)
}

func TestEndToEnd_LazyService_DescribeReturnsLocalizedString_English(t *testing.T) {
	tr := newWiredTranslator(t, "en")

	svc := lazy.NewService(func() (int, error) { return 42, nil })
	svc.SetTranslator(tr)

	got, err := svc.Describe(context.Background())
	if err != nil {
		t.Fatalf("Describe(uninitialized, en) err = %v; want nil", err)
	}
	want := "service is not yet initialized"
	if got != want {
		t.Fatalf("Describe(uninitialized, en) = %q; want %q\n"+
			"If this returned the message ID 'lazy.service.uninitialized' "+
			"verbatim, the wiring is broken — the adapter is NOT being "+
			"called through, or the bundle has no English entry for the ID.",
			got, want)
	}

	// Now initialize and re-describe.
	_, _ = svc.Get()
	got, err = svc.Describe(context.Background())
	if err != nil {
		t.Fatalf("Describe(ready, en) err = %v; want nil", err)
	}
	want = "service is ready"
	if got != want {
		t.Fatalf("Describe(ready, en) = %q; want %q", got, want)
	}
}

func TestEndToEnd_LazyService_DescribeReturnsLocalizedString_Serbian(t *testing.T) {
	tr := newWiredTranslator(t, "sr-RS", "en")

	svc := lazy.NewService(func() (int, error) { return 0, nil })
	svc.SetTranslator(tr)

	got, err := svc.Describe(context.Background())
	if err != nil {
		t.Fatalf("Describe(uninitialized, sr) err = %v; want nil", err)
	}
	want := "servis još nije inicijalizovan"
	if got != want {
		t.Fatalf("Describe(uninitialized, sr) = %q; want %q\n"+
			"If this returned the English string, the locale chain isn't "+
			"being honoured — go-i18n should select sr-RS before en.",
			got, want)
	}
}

func TestEndToEnd_LazyService_DefaultNoopWhenAdapterNotInjected(t *testing.T) {
	// Without SetTranslator: Lazy uses its NoopTranslator default and
	// returns the message ID verbatim. Proves the submodule remains
	// standalone-usable (CONST-051(B)) when no i18n is wired.
	svc := lazy.NewService(func() (int, error) { return 1, nil })

	got, err := svc.Describe(context.Background())
	if err != nil {
		t.Fatalf("Describe(noop) err = %v; want nil", err)
	}
	want := "lazy.service.uninitialized"
	if got != want {
		t.Fatalf("Describe(noop) = %q; want %q (message-ID passthrough)", got, want)
	}
}
