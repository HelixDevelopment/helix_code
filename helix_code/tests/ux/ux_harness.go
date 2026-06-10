// Package ux is a HelixCode-LOCAL user-experience harness for the API/CLI product.
// For a no-primary-GUI product, "UX" is the set of mechanically-checkable
// interaction invariants that determine whether the product is USABLE — NOT
// subjective aesthetics (those are honestly operator-attended, §11.4.52). It is a
// real local harness writing re-read evidence, NOT a HelixQA shell delegation.
//
// The four mechanizable UX invariants (§11.4.6 honest framing):
//  1. Journey completeness — the documented CLI journey runs end-to-end and each
//     step produces real, asserted output (not a canned constant).
//  2. i18n no-leak (CONST-046) — user-facing text resolves through a wired
//     translator to locale text, never leaking the raw message ID.
//  3. Error-message clarity — a failure surfaces a real, resolved, descriptive
//     string (not a raw ID, not empty, not a bare Go error).
//  4. Response-shape consistency — sampled error responses share one envelope.
//
// Each invariant ships a §1.1 paired-mutation meta-test (canned-string step,
// leaked message ID, inconsistent error shape) proving the harness cannot bluff.
package ux

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	serveri18n "dev.helix.code/internal/server/i18n"
)

// MinErrorClarityChars is the §11.4.91-style clarity floor: a user-facing error
// message must name its subject with at least this many characters (an empty or
// terse "err"/"fail" string is not a clear, actionable message).
const MinErrorClarityChars = 12

// JourneyStep is one bidirectional line of the §11.4.83 journey transcript.
type JourneyStep struct {
	Step             string `json:"step"`
	CommandSent      string `json:"command_sent"`
	ResponseReceived string `json:"response_received"`
	Assertion        string `json:"assertion"`
	Verdict          string `json:"verdict"`
}

// I18nResolution is one row of i18n_resolution.json.
type I18nResolution struct {
	MessageID    string `json:"message_id"`
	Locale       string `json:"locale"`
	ResolvedText string `json:"resolved_text"`
	LeakedID     bool   `json:"leaked_id"`
}

// ErrorShapeRow records the top-level envelope key set of one sampled error response.
type ErrorShapeRow struct {
	Sample  string   `json:"sample"`
	TopKeys []string `json:"top_keys"`
}

var (
	runIDOnce sync.Once
	runIDVal  string
)

func runID() string {
	runIDOnce.Do(func() {
		if v := os.Getenv("UX_RUN_ID"); v != "" {
			runIDVal = v
			return
		}
		if v := os.Getenv("STRESSCHAOS_RUN_ID"); v != "" {
			runIDVal = v
			return
		}
		runIDVal = time.Now().UTC().Format("20060102T150405Z")
	})
	return runIDVal
}

// EvidenceRoot resolves qa-results/<run-id>/. Override with UX_EVIDENCE_ROOT.
func EvidenceRoot() string {
	if v := os.Getenv("UX_EVIDENCE_ROOT"); v != "" {
		return filepath.Join(v, runID())
	}
	return filepath.Join(moduleRoot(), "qa-results", runID())
}

func moduleRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "qa-results-fallback"
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}

func evidenceDir(t testing.TB, name string) string {
	t.Helper()
	dir := filepath.Join(EvidenceRoot(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("ux: cannot create evidence dir %s: %v", dir, err)
	}
	return dir
}

// writeBytes writes b then RE-READS it, failing on empty (§11.4.5/§11.4.69).
func writeBytes(t testing.TB, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("ux: write evidence %s: %v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("ux: evidence artefact missing %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("ux: evidence artefact empty (not evidence per §11.4.5) %s", path)
	}
}

func writeJSON(t testing.TB, path string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("ux: marshal evidence %s: %v", path, err)
	}
	writeBytes(t, path, b)
}

// --- Invariant 1: journey completeness -------------------------------------

// JourneyStepFn produces the real response for a journey step. In the live driver
// these shell the real bin/cli (handleListModels / handleGenerate / handleCommand);
// meta-tests substitute a canned-string step to prove the assertion catches it.
type JourneyStepFn func(ctx context.Context) (response string, err error)

// JourneyStepSpec binds a step name + command + its real producer + a predicate
// that must hold on the REAL output (canned constants fail it).
type JourneyStepSpec struct {
	Name        string
	Command     string
	Produce     JourneyStepFn
	RealOutput  func(response string) bool // must return true for genuinely-real output
	Description string                     // what RealOutput asserts (for the transcript)
}

// RunJourney drives the journey steps, writes a bidirectional journey_transcript.jsonl
// (§11.4.83), and FAILS if any step errors or its RealOutput predicate rejects the
// response (a canned/empty step fails). Returns the transcript steps.
func RunJourney(t testing.TB, name string, steps []JourneyStepSpec) []JourneyStep {
	t.Helper()
	if len(steps) == 0 {
		t.Fatalf("ux: RunJourney %q has zero steps — an empty journey is not a journey", name)
	}

	transcript := make([]JourneyStep, 0, len(steps))
	var lines strings.Builder
	for _, s := range steps {
		resp, err := s.Produce(context.Background())
		verdict := "PASS"
		assertion := s.Description
		if err != nil {
			verdict = "FAIL"
		} else if s.RealOutput != nil && !s.RealOutput(resp) {
			verdict = "FAIL"
		}
		step := JourneyStep{
			Step:             s.Name,
			CommandSent:      s.Command,
			ResponseReceived: resp,
			Assertion:        assertion,
			Verdict:          verdict,
		}
		transcript = append(transcript, step)
		b, _ := json.Marshal(step)
		lines.Write(b)
		lines.WriteByte('\n')
	}

	dir := evidenceDir(t, name)
	path := filepath.Join(dir, "journey_transcript.jsonl")
	writeBytes(t, path, []byte(lines.String()))

	for _, st := range transcript {
		if st.Verdict != "PASS" {
			t.Fatalf("ux: journey %q step %q FAILED real-output assertion (%s) — response=%q (evidence: %s)",
				name, st.Step, st.Assertion, st.ResponseReceived, path)
		}
	}
	t.Logf("ux: journey %q completed %d steps, all real-output assertions PASS -> %s", name, len(transcript), path)
	return transcript
}

// --- Invariant 2: i18n no-leak (CONST-046) ---------------------------------

// TranslateFn resolves a message ID against a locale; the live path wires a real
// serveri18n translator, meta-tests wire the NoopTranslator (the planted leak).
type TranslateFn func(ctx context.Context, messageID, locale string) (string, error)

// RealServerTranslator returns a TranslateFn backed by the REAL serveri18n bundle
// (active.en.yaml), proving resolution goes through the production i18n seam.
func RealServerTranslator(t testing.TB) TranslateFn {
	t.Helper()
	return func(ctx context.Context, messageID, locale string) (string, error) {
		tr, err := serveri18n.NewTranslator(locale)
		if err != nil {
			return "", err
		}
		return tr.T(ctx, messageID, nil)
	}
}

// AssertNoI18nLeak resolves each message ID and FAILS if any resolves to the raw
// ID (a leaked message ID shown to a user is a CONST-046 UX defect). Writes
// i18n_resolution.json. The IDs MUST exist in the real bundle (the real translator
// returns the locale text; a leak means the translator was NOT wired).
func AssertNoI18nLeak(t testing.TB, name, locale string, messageIDs []string, translate TranslateFn) []I18nResolution {
	t.Helper()
	if len(messageIDs) == 0 {
		t.Fatalf("ux: AssertNoI18nLeak %q has zero message IDs to check", name)
	}
	rows := make([]I18nResolution, 0, len(messageIDs))
	for _, id := range messageIDs {
		resolved, err := translate(context.Background(), id, locale)
		if err != nil {
			t.Fatalf("ux: i18n resolve %q failed: %v", id, err)
		}
		rows = append(rows, I18nResolution{
			MessageID:    id,
			Locale:       locale,
			ResolvedText: resolved,
			LeakedID:     resolved == id,
		})
	}

	dir := evidenceDir(t, name)
	path := filepath.Join(dir, "i18n_resolution.json")
	writeJSON(t, path, rows)

	for _, r := range rows {
		if r.LeakedID {
			t.Fatalf("ux: i18n LEAK — message ID %q resolved to itself (raw ID shown to user) — CONST-046 UX defect (evidence: %s)",
				r.MessageID, path)
		}
	}
	t.Logf("ux: i18n no-leak %q — %d IDs all resolved to locale text -> %s", name, len(rows), path)
	return rows
}

// --- Invariant 3: error-message clarity ------------------------------------

// AssertErrorClarity FAILS if msg is empty, equals its message ID (a leak), or is
// shorter than the clarity floor (§11.4.91). Returns the message for transcripting.
func AssertErrorClarity(t testing.TB, name, messageID, msg string) {
	t.Helper()
	if strings.TrimSpace(msg) == "" {
		t.Fatalf("ux: error-clarity %q — message is EMPTY (an empty error tells the user nothing)", name)
	}
	if msg == messageID {
		t.Fatalf("ux: error-clarity %q — message equals raw ID %q (leaked, not actionable)", name, messageID)
	}
	if len([]rune(strings.TrimSpace(msg))) < MinErrorClarityChars {
		t.Fatalf("ux: error-clarity %q — message %q below clarity floor (%d chars) — not descriptive enough",
			name, msg, MinErrorClarityChars)
	}
}

// --- Invariant 4: response-shape consistency -------------------------------

// AssertConsistentErrorShape FAILS if the sampled error responses do NOT share the
// same top-level JSON envelope key set (an inconsistent shape is a real UX defect
// for API consumers who parse uniformly). Writes error_shape_report.json.
func AssertConsistentErrorShape(t testing.TB, name string, samples map[string]string) []ErrorShapeRow {
	t.Helper()
	if len(samples) < 2 {
		t.Fatalf("ux: AssertConsistentErrorShape %q needs >=2 samples to compare", name)
	}
	rows := make([]ErrorShapeRow, 0, len(samples))
	var reference []string
	consistent := true

	// Deterministic order for the reference + report.
	names := make([]string, 0, len(samples))
	for n := range samples {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, sampleName := range names {
		body := samples[sampleName]
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(body), &obj); err != nil {
			t.Fatalf("ux: response-shape %q sample %q is not a JSON object: %v", name, sampleName, err)
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		rows = append(rows, ErrorShapeRow{Sample: sampleName, TopKeys: keys})
		if reference == nil {
			reference = keys
		} else if strings.Join(keys, ",") != strings.Join(reference, ",") {
			consistent = false
		}
	}

	dir := evidenceDir(t, name)
	path := filepath.Join(dir, "error_shape_report.json")
	writeJSON(t, path, rows)

	if !consistent {
		t.Fatalf("ux: response-shape %q — sampled error responses have DIVERGENT top-level envelopes (clients cannot parse uniformly) (evidence: %s)",
			name, path)
	}
	t.Logf("ux: response-shape %q — %d samples share envelope %v -> %s", name, len(rows), reference, path)
	return rows
}
