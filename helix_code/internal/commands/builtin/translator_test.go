// translator_test.go — CONST-046 round-353 §11.4 anti-bluff sweep
// (2026-05-19). Unit tests for the internal/commands/builtin
// package-level translator + trc() seam, plus the call-site
// paired-mutation guards proving every migrated Description()/Usage()
// literal genuinely flows through the Translator instead of a
// hardcoded string.
//
// TestMain wires a REAL translator loaded from the on-disk
// i18n/bundles/active.en.yaml so the pre-existing
// TestXCommand_Description / TestXCommand_Usage assertions (which
// check for human-readable substrings like "/condense") keep
// asserting on the text users actually see — NOT raw message IDs.
// Without a wired translator the in-package i18n.NoopTranslator{}
// echoes message IDs, which would be a §11.4 PASS-bluff: the test
// would assert on "builtin_condense_usage", a string no end user
// ever sees.
//
// Mocks ALLOWED here per CONST-050(A) — unit-test-only file
// (_test.go, no integration build tag).
package builtin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	builtini18n "dev.helix.code/internal/commands/builtin/i18n"
	"dev.helix.code/pkg/i18n"
	"dev.helix.code/pkg/i18nadapter"
	"golang.org/x/text/language"
)

// bundleRelPath is the on-disk path of the round-353 active English
// bundle, relative to this package directory.
var bundleRelPath = filepath.Join("i18n", "bundles", "active.en.yaml")

// TestMain wires a real *i18nadapter.Translator built from the
// on-disk bundle before any test in this package runs, then restores
// the NoopTranslator default afterwards. This is the unit-test
// equivalent of the boot-time SetTranslator call helix_code performs
// in production.
func TestMain(m *testing.M) {
	bundle := i18n.NewBundle(language.English)
	if err := bundle.LoadMessageFile(bundleRelPath); err != nil {
		// A missing/broken bundle is a hard failure — running the
		// suite against NoopTranslator would silently pass on raw
		// message IDs (§11.4 PASS-bluff).
		panic("round-353 builtin i18n bundle failed to load: " + err.Error())
	}
	loc := i18n.NewLocalizer(bundle, "en")
	SetTranslator(i18nadapter.New(loc))

	code := m.Run()

	SetTranslator(nil)
	os.Exit(code)
}

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests can
// prove trc() went through Translator.T rather than returning a
// hardcoded literal that merely happens to match the bundle value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

// errTranslator always fails — exercises the loud-degrade path.
type errTranslator struct{}

func (errTranslator) T(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ context.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// withRealBundleTranslator restores the real on-disk-bundle
// translator after a test that swapped in a sentinel/err translator,
// so later tests in the package still see real text.
func withRealBundleTranslator(t *testing.T) {
	t.Helper()
	bundle := i18n.NewBundle(language.English)
	if err := bundle.LoadMessageFile(bundleRelPath); err != nil {
		t.Fatalf("reload round-353 builtin bundle: %v", err)
	}
	SetTranslator(i18nadapter.New(i18n.NewLocalizer(bundle, "en")))
}

func TestTrc_DefaultsToLoudEchoWhenNoopWired(t *testing.T) {
	SetTranslator(builtini18n.NoopTranslator{})
	defer withRealBundleTranslator(t)

	got := trc("builtin_condense_description", nil)
	if got != "builtin_condense_description" {
		t.Fatalf("trc with NoopTranslator = %q, want raw message ID (loud echo)", got)
	}
}

func TestTrc_UsesInjectedTranslator(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	defer withRealBundleTranslator(t)

	got := trc("builtin_workflows_description", nil)
	if got != "<TR:builtin_workflows_description>" {
		t.Fatalf("trc = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTrc_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (that would leave the user with blank command
	// metadata). Implementation MUST degrade to the message ID.
	SetTranslator(errTranslator{})
	defer withRealBundleTranslator(t)

	got := trc("builtin_reportbug_usage", nil)
	if got != "builtin_reportbug_usage" {
		t.Fatalf("trc on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset → NoopTranslator
	defer withRealBundleTranslator(t)

	got := trc("builtin_newtask_description", nil)
	if got != "builtin_newtask_description" {
		t.Fatalf("trc after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

// migratedIDs is the exhaustive set of CONST-046 message IDs the
// round-353 + round-355 sweeps migrated — round-353 added the
// Description/Usage metadata IDs; round-355 added the runtime
// CommandResult.Message IDs returned by Execute().
var migratedIDs = []string{
	// round-353 — context-free Description()/Usage() metadata.
	"builtin_condense_description", "builtin_condense_usage",
	"builtin_deepplanning_description", "builtin_deepplanning_usage",
	"builtin_newrule_description", "builtin_newrule_usage",
	"builtin_newtask_description", "builtin_newtask_usage",
	"builtin_reportbug_description", "builtin_reportbug_usage",
	"builtin_workflows_description", "builtin_workflows_usage",
	// round-355 — runtime CommandResult.Message text from Execute().
	"builtin_condense_no_history", "builtin_condense_not_enough_history",
	"builtin_condense_in_progress",
	"builtin_deepplanning_topic_required", "builtin_deepplanning_starting",
	"builtin_deepplanning_save_location", "builtin_deepplanning_resuming",
	"builtin_newrule_generating",
	"builtin_newtask_description_required", "builtin_newtask_created",
	"builtin_reportbug_default_description", "builtin_reportbug_prepared",
	"builtin_reportbug_submitting", "builtin_reportbug_review_manually",
}

// TestBundle_AllMigratedIDsResolveNonEmpty is the anti-bluff parity
// guard: every migrated ID MUST resolve through the real on-disk
// bundle to a non-empty, non-ID string. A missing bundle entry would
// otherwise surface only as a raw message ID in production output.
func TestBundle_AllMigratedIDsResolveNonEmpty(t *testing.T) {
	for _, id := range migratedIDs {
		got := trc(id, nil)
		if got == "" {
			t.Errorf("trc(%q) returned empty string — bundle entry missing", id)
		}
		if got == id {
			t.Errorf("trc(%q) echoed the raw ID — bundle text missing for this key", id)
		}
	}
}

// commandMeta couples a built-in command's Description()/Usage()
// accessors to the message IDs they MUST flow through and the
// human-readable substring an end user expects to see.
type commandMeta struct {
	name          string
	description   func() string
	usage         func() string
	descID        string
	usageID       string
	usageContains string
}

func allCommandMeta() []commandMeta {
	return []commandMeta{
		{"condense", (&CondenseCommand{}).Description, (&CondenseCommand{}).Usage,
			"builtin_condense_description", "builtin_condense_usage", "/condense"},
		{"deepplanning", (&DeepPlanningCommand{}).Description, (&DeepPlanningCommand{}).Usage,
			"builtin_deepplanning_description", "builtin_deepplanning_usage", "/deepplanning"},
		{"newrule", (&NewRuleCommand{}).Description, (&NewRuleCommand{}).Usage,
			"builtin_newrule_description", "builtin_newrule_usage", "/newrule"},
		{"newtask", (&NewTaskCommand{}).Description, (&NewTaskCommand{}).Usage,
			"builtin_newtask_description", "builtin_newtask_usage", "/newtask"},
		{"reportbug", (&ReportBugCommand{}).Description, (&ReportBugCommand{}).Usage,
			"builtin_reportbug_description", "builtin_reportbug_usage", "/reportbug"},
		{"workflows", (&WorkflowsCommand{}).Description, (&WorkflowsCommand{}).Usage,
			"builtin_workflows_description", "builtin_workflows_usage", "/workflows"},
	}
}

// TestDescriptionUsage_GoThroughTranslator is the call-site
// paired-mutation: with a sentinel translator wired, every command's
// Description() and Usage() MUST surface the sentinel-wrapped message
// ID — proving the literal was NOT hardcoded anywhere on the path. If
// a future refactor inlines a string, this test fails.
func TestDescriptionUsage_GoThroughTranslator(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	defer withRealBundleTranslator(t)

	for _, cm := range allCommandMeta() {
		t.Run(cm.name, func(t *testing.T) {
			gotDesc := cm.description()
			if gotDesc != "<TR:"+cm.descID+">" {
				t.Errorf("%s.Description() = %q, want sentinel-wrapped %q — literal bypassed trc()",
					cm.name, gotDesc, cm.descID)
			}
			gotUsage := cm.usage()
			if gotUsage != "<TR:"+cm.usageID+">" {
				t.Errorf("%s.Usage() = %q, want sentinel-wrapped %q — literal bypassed trc()",
					cm.name, gotUsage, cm.usageID)
			}
		})
	}
}

// TestDescriptionUsage_RealBundleProducesUserText is the positive
// runtime-evidence half: with the real on-disk bundle wired (via
// TestMain), every command's Description() is non-empty and every
// Usage() contains the expected human-readable command token. This
// is what an end user genuinely sees — proving the migration did not
// break the user-facing experience.
func TestDescriptionUsage_RealBundleProducesUserText(t *testing.T) {
	for _, cm := range allCommandMeta() {
		t.Run(cm.name, func(t *testing.T) {
			desc := cm.description()
			if strings.TrimSpace(desc) == "" {
				t.Errorf("%s.Description() empty under real bundle", cm.name)
			}
			if desc == cm.descID {
				t.Errorf("%s.Description() echoed raw ID %q — bundle entry missing", cm.name, cm.descID)
			}
			usage := cm.usage()
			if !strings.Contains(usage, cm.usageContains) {
				t.Errorf("%s.Usage() = %q, want it to contain %q (real user-facing text)",
					cm.name, usage, cm.usageContains)
			}
		})
	}
}
