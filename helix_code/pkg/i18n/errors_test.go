package i18n

import (
	"errors"
	"testing"
)

func TestErrors_DistinctSentinels(t *testing.T) {
	if errors.Is(ErrMessageNotFound, ErrBundleNotConfigured) {
		t.Fatalf("ErrMessageNotFound and ErrBundleNotConfigured must be distinct sentinels")
	}
	if errors.Is(ErrBundleNotConfigured, ErrMessageNotFound) {
		t.Fatalf("ErrBundleNotConfigured and ErrMessageNotFound must be distinct sentinels")
	}
}

func TestErrors_MessagesAreDeveloperFacing(t *testing.T) {
	// Sentinel messages are diagnostic strings intended for engineers reading
	// logs / panics, NOT end users — that is why they are exempt from
	// CONST-046's "no hardcoded user-facing English" rule per the design doc.
	// This test pins the prefix so a future refactor that accidentally
	// re-purposes these sentinels into user-facing output is loud about it.
	for _, e := range []error{ErrMessageNotFound, ErrBundleNotConfigured} {
		if got := e.Error(); len(got) == 0 || got[:5] != "i18n:" {
			t.Fatalf("sentinel %v lacks expected developer-facing %q prefix", e, "i18n:")
		}
	}
}
