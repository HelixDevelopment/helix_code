package ux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	serveri18n "dev.helix.code/internal/server/i18n"
)

// §1.1 paired-mutation meta-tests prove the ux harness cannot bluff: each plants a
// known UX defect and asserts the harness DETECTS it (detection path is t.Fatalf,
// captured via failTB).

type failTB struct {
	testing.TB
	mu     sync.Mutex
	failed bool
	msg    string
}

func (f *failTB) Helper() {}
func (f *failTB) Fatalf(format string, args ...interface{}) {
	f.mu.Lock()
	f.failed = true
	f.msg = fmt.Sprintf(format, args...)
	f.mu.Unlock()
	panic(sentinelFatal{})
}
func (f *failTB) Errorf(format string, args ...interface{}) {
	f.mu.Lock()
	f.failed = true
	f.msg = fmt.Sprintf(format, args...)
	f.mu.Unlock()
}
func (f *failTB) Logf(format string, args ...interface{}) {}

type sentinelFatal struct{}

func runWithFailTB(body func(tb testing.TB)) (failed bool, msg string) {
	f := &failTB{TB: &testing.T{}}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(sentinelFatal); !ok {
					panic(r)
				}
			}
		}()
		body(f)
	}()
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failed, f.msg
}

func isolatedEvidence(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	old := os.Getenv("UX_EVIDENCE_ROOT")
	os.Setenv("UX_EVIDENCE_ROOT", tmp)
	t.Cleanup(func() { os.Setenv("UX_EVIDENCE_ROOT", old) })
}

// TestMeta_RunJourney_DetectsCannedString plants a journey step whose handler
// returns a FIXED CONSTANT instead of real output and asserts the harness's
// real-output assertion FAILS the canned plant (SP7-plan A3).
func TestMeta_RunJourney_DetectsCannedString(t *testing.T) {
	isolatedEvidence(t)
	failed, _ := runWithFailTB(func(tb testing.TB) {
		RunJourney(tb, "meta-canned", []JourneyStepSpec{
			{
				Name:    "canned_step",
				Command: "cli -command 'echo expected-nonce-XYZ'",
				Produce: func(ctx context.Context) (string, error) {
					return "Generated response (this is a canned constant)", nil // canned!
				},
				// the journey EXPECTS the real echoed nonce; the canned constant lacks it
				RealOutput:  func(resp string) bool { return contains(resp, "expected-nonce-XYZ") },
				Description: "must echo the real nonce, not a canned constant",
			},
		})
	})
	if !failed {
		t.Fatal("meta: RunJourney did NOT detect the canned-string step — harness is a bluff")
	}
}

// TestMeta_AssertNoI18nLeak_DetectsLeakedID wires the NoopTranslator (which echoes
// the message ID verbatim — the production loud-failure default) and asserts the
// i18n-no-leak assertion catches resolved_text == message_id. The Noop path IS the
// planted defect.
func TestMeta_AssertNoI18nLeak_DetectsLeakedID(t *testing.T) {
	isolatedEvidence(t)
	noop := serveri18n.NoopTranslator{}
	leakyTranslate := func(ctx context.Context, id, locale string) (string, error) {
		return noop.T(ctx, id, nil) // returns id verbatim -> leak
	}
	failed, _ := runWithFailTB(func(tb testing.TB) {
		AssertNoI18nLeak(tb, "meta-leak", "en",
			[]string{"internal_server_qa_engine_disabled"}, leakyTranslate)
	})
	if !failed {
		t.Fatal("meta: AssertNoI18nLeak did NOT detect the leaked message ID (NoopTranslator) — harness is a bluff")
	}
}

// TestMeta_AssertErrorClarity_DetectsEmptyAndTerse plants empty + terse + leaked-ID
// messages and asserts each is rejected.
func TestMeta_AssertErrorClarity_DetectsEmptyAndTerse(t *testing.T) {
	cases := []struct {
		name, id, msg string
	}{
		{"empty", "internal_server_x", ""},
		{"terse", "internal_server_x", "err"},
		{"leaked", "internal_server_x", "internal_server_x"},
	}
	for _, c := range cases {
		failed, _ := runWithFailTB(func(tb testing.TB) {
			AssertErrorClarity(tb, "meta-clarity-"+c.name, c.id, c.msg)
		})
		if !failed {
			t.Fatalf("meta: AssertErrorClarity did NOT reject %q message %q — harness is a bluff", c.name, c.msg)
		}
	}
}

// TestMeta_AssertConsistentErrorShape_DetectsDivergence feeds two error responses
// with DIVERGENT top-level keys and asserts the consistency assertion fails.
func TestMeta_AssertConsistentErrorShape_DetectsDivergence(t *testing.T) {
	isolatedEvidence(t)
	failed, _ := runWithFailTB(func(tb testing.TB) {
		AssertConsistentErrorShape(tb, "meta-shape", map[string]string{
			"a": `{"error":"bad input"}`,
			"b": `{"message":"bad input","code":400}`, // divergent envelope
		})
	})
	if !failed {
		t.Fatal("meta: AssertConsistentErrorShape did NOT detect the divergent envelope — harness is a bluff")
	}
}

// TestMeta_PositivePathWritesEvidence proves a real journey writes a non-empty
// journey_transcript.jsonl.
func TestMeta_PositivePathWritesEvidence(t *testing.T) {
	isolatedEvidence(t)
	RunJourney(t, "meta-positive", []JourneyStepSpec{
		{
			Name:    "real_step",
			Command: "echo real-nonce-123",
			Produce: func(ctx context.Context) (string, error) {
				return "stdout: real-nonce-123\nexit code: 0", nil
			},
			RealOutput:  func(resp string) bool { return contains(resp, "real-nonce-123") },
			Description: "echoes the real nonce",
		},
	})
	path := filepath.Join(EvidenceRoot(), "meta-positive", "journey_transcript.jsonl")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("meta: journey_transcript.jsonl not written: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("meta: journey_transcript.jsonl is empty — would be a hollow PASS")
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
